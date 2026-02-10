// Package bsv provides bulk operations for efficient BSV blockchain queries.
package bsv

import (
	"context"
	"fmt"
	"sync"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"golang.org/x/time/rate"

	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// MaxBulkBatchSize is the maximum number of items per bulk API call.
	MaxBulkBatchSize = 20

	// DefaultRateLimit is the default rate limit for bulk operations (requests per second).
	DefaultRateLimit = 3

	// DefaultRateBurst allows short bursts above the rate limit.
	DefaultRateBurst = 5
)

// BulkOperations provides efficient bulk blockchain operations.
// Wraps go-whatsonchain bulk endpoints with automatic batching, rate limiting, and error recovery.
type BulkOperations struct {
	client  WOCClient
	limiter *rate.Limiter
	logger  Logger
	metrics *BulkMetrics
}

// BulkMetrics tracks performance metrics for bulk operations.
type BulkMetrics struct {
	mu             sync.Mutex
	TotalRequests  int
	TotalAddresses int
	TotalUTXOs     int
	FailedRequests int
	AverageLatency time.Duration
	latencies      []time.Duration
}

// BulkOperationsOptions configures bulk operations.
type BulkOperationsOptions struct {
	// RateLimit is the maximum requests per second (default: 3).
	RateLimit float64

	// RateBurst allows short bursts above the rate limit (default: 5).
	RateBurst int

	// Logger for debug and error output.
	Logger Logger
}

// NewBulkOperations creates a new bulk operations handler.
func NewBulkOperations(client WOCClient, opts *BulkOperationsOptions) *BulkOperations {
	if opts == nil {
		opts = &BulkOperationsOptions{
			RateLimit: DefaultRateLimit,
			RateBurst: DefaultRateBurst,
		}
	}

	rateLimit := opts.RateLimit
	if rateLimit <= 0 {
		rateLimit = DefaultRateLimit
	}

	rateBurst := opts.RateBurst
	if rateBurst <= 0 {
		rateBurst = DefaultRateBurst
	}

	return &BulkOperations{
		client:  client,
		limiter: rate.NewLimiter(rate.Limit(rateLimit), rateBurst),
		logger:  opts.Logger,
		metrics: &BulkMetrics{},
	}
}

// AddressActivity represents activity status for an address.
type AddressActivity struct {
	Address    string
	HasHistory bool
	Error      error
}

// BulkAddressActivityCheck checks if addresses have transaction history.
// Uses BulkAddressHistory for fast activity detection.
// Returns slice of activity results (one per input address).
func (b *BulkOperations) BulkAddressActivityCheck(ctx context.Context, addresses []string) ([]AddressActivity, error) {
	if len(addresses) == 0 {
		return []AddressActivity{}, nil
	}

	results := make([]AddressActivity, 0, len(addresses))

	// Process in batches of MaxBulkBatchSize
	for i := 0; i < len(addresses); i += MaxBulkBatchSize {
		end := i + MaxBulkBatchSize
		if end > len(addresses) {
			end = len(addresses)
		}
		batch := addresses[i:end]

		batchResults, err := b.checkActivityBatch(ctx, batch)
		if err != nil {
			// On batch failure, mark all addresses in batch as errors
			for _, addr := range batch {
				results = append(results, AddressActivity{
					Address: addr,
					Error:   err,
				})
			}
			continue
		}

		results = append(results, batchResults...)
	}

	return results, nil
}

// BulkAddressUTXOFetch fetches UTXOs for multiple addresses in a single or batched API call.
func (b *BulkOperations) BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]BulkUTXOResult, error) {
	if len(addresses) == 0 {
		return []BulkUTXOResult{}, nil
	}

	results := make([]BulkUTXOResult, 0, len(addresses))

	// Process in batches of MaxBulkBatchSize
	for i := 0; i < len(addresses); i += MaxBulkBatchSize {
		end := i + MaxBulkBatchSize
		if end > len(addresses) {
			end = len(addresses)
		}
		batch := addresses[i:end]

		batchResults, err := b.fetchUTXOsBatch(ctx, batch)
		if err != nil {
			// On batch failure, mark all addresses in batch as errors
			for _, addr := range batch {
				results = append(results, BulkUTXOResult{
					Address: addr,
					Error:   err,
				})
			}
			continue
		}

		results = append(results, batchResults...)
	}

	return results, nil
}

// BulkUTXOValidation validates multiple UTXOs to check if they are still unspent.
func (b *BulkOperations) BulkUTXOValidation(ctx context.Context, utxos []UTXO) ([]UTXOSpentStatus, error) {
	if len(utxos) == 0 {
		return []UTXOSpentStatus{}, nil
	}

	results := make([]UTXOSpentStatus, 0, len(utxos))

	// Process in batches of MaxBulkBatchSize
	for i := 0; i < len(utxos); i += MaxBulkBatchSize {
		end := i + MaxBulkBatchSize
		if end > len(utxos) {
			end = len(utxos)
		}
		batch := utxos[i:end]

		batchResults, err := b.validateUTXOsBatch(ctx, batch)
		if err != nil {
			// On batch failure, mark all UTXOs in batch as errors
			for _, u := range batch {
				results = append(results, UTXOSpentStatus{
					TxID:  u.TxID,
					Vout:  u.Vout,
					Error: err,
				})
			}
			continue
		}

		results = append(results, batchResults...)
	}

	return results, nil
}

// GetMetrics returns a copy of current bulk operation metrics.
func (b *BulkOperations) GetMetrics() BulkMetrics {
	b.metrics.mu.Lock()
	defer b.metrics.mu.Unlock()

	// Calculate average latency
	if len(b.metrics.latencies) > 0 {
		var total time.Duration
		for _, lat := range b.metrics.latencies {
			total += lat
		}
		b.metrics.AverageLatency = total / time.Duration(len(b.metrics.latencies))
	}

	// Return a copy of metrics (without the mutex)
	return BulkMetrics{
		TotalRequests:  b.metrics.TotalRequests,
		TotalAddresses: b.metrics.TotalAddresses,
		TotalUTXOs:     b.metrics.TotalUTXOs,
		FailedRequests: b.metrics.FailedRequests,
		AverageLatency: b.metrics.AverageLatency,
		latencies:      nil, // Don't expose internal latencies
	}
}

// recordRequest records metrics for a request.
// checkActivityBatch checks activity for a single batch.
func (b *BulkOperations) checkActivityBatch(ctx context.Context, addresses []string) ([]AddressActivity, error) {
	start := time.Now()

	// Wait for rate limiter
	if err := b.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Build address list
	addrList := &whatsonchain.AddressList{
		Addresses: addresses,
	}

	// Call bulk history endpoint
	history, err := b.client.BulkAddressHistory(ctx, addrList)
	if err != nil {
		b.recordRequest(start, true)
		b.logError("bulk activity check failed for %d addresses: %v", len(addresses), err)
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}

	b.recordRequest(start, false)
	b.debug("bulk activity check: %d addresses, %d with history", len(addresses), len(history))

	// Build results map for fast lookup
	hasHistory := make(map[string]bool)
	for _, entry := range history {
		hasHistory[entry.Address] = len(entry.History) > 0
	}

	// Create results in same order as input
	results := make([]AddressActivity, len(addresses))
	for i, addr := range addresses {
		results[i] = AddressActivity{
			Address:    addr,
			HasHistory: hasHistory[addr],
		}
	}

	return results, nil
}

// BulkUTXOResult represents UTXOs for an address.
type BulkUTXOResult struct {
	Address          string
	ConfirmedUTXOs   []UTXO
	UnconfirmedUTXOs []UTXO
	Error            error
}

// BulkAddressUTXOFetch fetches both confirmed and unconfirmed UTXOs for multiple addresses.
// Returns slice of results (one per input address).

// fetchUTXOsBatch fetches UTXOs for a single batch.
//
//nolint:gocognit,gocyclo // Parallel UTXO fetching inherently complex
func (b *BulkOperations) fetchUTXOsBatch(ctx context.Context, addresses []string) ([]BulkUTXOResult, error) {
	addrList := &whatsonchain.AddressList{
		Addresses: addresses,
	}

	// Fetch confirmed and unconfirmed in parallel
	type utxoResponse struct {
		confirmed      whatsonchain.BulkUnspentResponse
		unconfirmed    whatsonchain.BulkUnspentResponse
		errConfirmed   error
		errUnconfirmed error
	}

	respCh := make(chan utxoResponse, 1)

	go func() {
		var resp utxoResponse

		// Wait for rate limiter for confirmed
		if err := b.limiter.Wait(ctx); err != nil {
			resp.errConfirmed = err
			respCh <- resp
			return
		}

		start := time.Now()
		resp.confirmed, resp.errConfirmed = b.client.BulkAddressConfirmedUTXOs(ctx, addrList)
		b.recordRequest(start, resp.errConfirmed != nil)

		// Wait for rate limiter for unconfirmed
		if err := b.limiter.Wait(ctx); err != nil {
			resp.errUnconfirmed = err
			respCh <- resp
			return
		}

		start = time.Now()
		resp.unconfirmed, resp.errUnconfirmed = b.client.BulkAddressUnconfirmedUTXOs(ctx, addrList)
		b.recordRequest(start, resp.errUnconfirmed != nil)

		respCh <- resp
	}()

	var resp utxoResponse
	select {
	case resp = <-respCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Handle errors
	if resp.errConfirmed != nil {
		b.logError("bulk confirmed UTXO fetch failed: %v", resp.errConfirmed)
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, resp.errConfirmed)
	}
	if resp.errUnconfirmed != nil {
		b.logError("bulk unconfirmed UTXO fetch failed: %v", resp.errUnconfirmed)
		// Continue with confirmed only
	}

	// Build results map
	results := make(map[string]BulkUTXOResult)
	for _, addr := range addresses {
		results[addr] = BulkUTXOResult{Address: addr}
	}

	// Add confirmed UTXOs
	totalUTXOs := 0
	for _, entry := range resp.confirmed {
		result := results[entry.Address]
		for _, u := range entry.Utxos {
			result.ConfirmedUTXOs = append(result.ConfirmedUTXOs, UTXO{
				TxID:          u.TxHash,
				Vout:          uint32(u.TxPos), //nolint:gosec // TxPos is always non-negative
				Amount:        uint64(u.Value), //nolint:gosec // Value is always non-negative
				Address:       entry.Address,
				Confirmations: 1, // Confirmed
			})
			totalUTXOs++
		}
		results[entry.Address] = result
	}

	// Add unconfirmed UTXOs
	for _, entry := range resp.unconfirmed {
		result := results[entry.Address]
		for _, u := range entry.Utxos {
			result.UnconfirmedUTXOs = append(result.UnconfirmedUTXOs, UTXO{
				TxID:          u.TxHash,
				Vout:          uint32(u.TxPos), //nolint:gosec // TxPos is always non-negative
				Amount:        uint64(u.Value), //nolint:gosec // Value is always non-negative
				Address:       entry.Address,
				Confirmations: 0, // Unconfirmed
			})
			totalUTXOs++
		}
		results[entry.Address] = result
	}

	b.debug("bulk UTXO fetch: %d addresses, %d UTXOs", len(addresses), totalUTXOs)
	b.metrics.mu.Lock()
	b.metrics.TotalAddresses += len(addresses)
	b.metrics.TotalUTXOs += totalUTXOs
	b.metrics.mu.Unlock()

	// Convert map to slice in original order
	resultSlice := make([]BulkUTXOResult, len(addresses))
	for i, addr := range addresses {
		resultSlice[i] = results[addr]
	}

	return resultSlice, nil
}

// UTXOSpentStatus represents the spent status of a UTXO.
type UTXOSpentStatus struct {
	TxID  string
	Vout  uint32
	Spent bool
	Error error
}

// BulkUTXOValidation validates if UTXOs are still unspent.
// Uses BulkSpentOutputs endpoint.

// validateUTXOsBatch validates a single batch of UTXOs.
//
//nolint:gocognit // UTXO validation logic inherently complex
func (b *BulkOperations) validateUTXOsBatch(ctx context.Context, utxos []UTXO) ([]UTXOSpentStatus, error) {
	start := time.Now()

	// Wait for rate limiter
	if err := b.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Build bulk spent output request
	outputs := make([]whatsonchain.BulkSpentUTXO, len(utxos))
	for i, u := range utxos {
		outputs[i] = whatsonchain.BulkSpentUTXO{
			TxID: u.TxID,
			Vout: int(u.Vout),
		}
	}

	req := &whatsonchain.BulkSpentOutputRequest{
		UTXOs: outputs,
	}

	// Call bulk spent outputs endpoint
	spentResults, err := b.client.BulkSpentOutputs(ctx, req)
	if err != nil {
		b.recordRequest(start, true)
		b.logError("bulk UTXO validation failed for %d UTXOs: %v", len(utxos), err)
		return nil, fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}

	b.recordRequest(start, false)

	// Build results
	results := make([]UTXOSpentStatus, len(utxos))
	for i, u := range utxos {
		status := UTXOSpentStatus{
			TxID:  u.TxID,
			Vout:  u.Vout,
			Spent: false,
		}

		// Check if this UTXO appears in spent results
		key := fmt.Sprintf("%s:%d", u.TxID, u.Vout)
		for _, spentResult := range spentResults {
			spentKey := fmt.Sprintf("%s:%d", spentResult.TxID, spentResult.Vout)
			if key == spentKey && spentResult.Spent != nil {
				status.Spent = true
				break
			}
		}

		results[i] = status
	}

	spentCount := 0
	for _, r := range results {
		if r.Spent {
			spentCount++
		}
	}

	b.debug("bulk UTXO validation: %d UTXOs, %d spent", len(utxos), spentCount)

	return results, nil
}

func (b *BulkOperations) recordRequest(start time.Time, failed bool) {
	latency := time.Since(start)

	b.metrics.mu.Lock()
	b.metrics.TotalRequests++
	if failed {
		b.metrics.FailedRequests++
	}
	b.metrics.latencies = append(b.metrics.latencies, latency)

	// Keep only last 100 latencies to avoid unbounded memory growth
	if len(b.metrics.latencies) > 100 {
		b.metrics.latencies = b.metrics.latencies[1:]
	}
	b.metrics.mu.Unlock()

	// Record in global metrics
	var err error
	if failed {
		err = sigilerr.ErrNetworkError
	}
	metrics.Global.RecordRPCCall("bsv_bulk", latency, err)
}

// debug logs a debug message if a logger is configured.
func (b *BulkOperations) debug(format string, args ...any) {
	if b.logger != nil {
		b.logger.Debug(format, args...)
	}
}

// logError logs an error message if a logger is configured.
func (b *BulkOperations) logError(format string, args ...any) {
	if b.logger != nil {
		b.logger.Error(format, args...)
	}
}
