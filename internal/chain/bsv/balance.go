package bsv

import (
	"context"
	"math/big"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"

	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Balance represents a balance result with metadata.
type Balance struct {
	Address     string
	Amount      *big.Int
	Unconfirmed *big.Int // Unconfirmed balance delta in satoshis (can be negative)
	Symbol      string
	Decimals    int
}

// GetNativeBalance retrieves the native BSV balance including unconfirmed data.
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	start := time.Now()
	resp, err := c.doGetFullBalance(ctx, address)
	metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
	if err != nil {
		return nil, err
	}

	bal := &Balance{
		Address:  address,
		Amount:   big.NewInt(resp.Confirmed),
		Symbol:   "BSV",
		Decimals: decimals,
	}
	if resp.Unconfirmed != 0 {
		bal.Unconfirmed = big.NewInt(resp.Unconfirmed)
	} else {
		// The WhatsOnChain balance endpoint does not always reflect mempool
		// (unconfirmed) funds — notably on testnet, where it can report 0 while
		// a pending UTXO exists. When the endpoint reports no unconfirmed delta,
		// reconcile from the unspent UTXO set so incoming pending funds appear.
		if mempool := c.mempoolValue(ctx, address); mempool > 0 {
			bal.Unconfirmed = big.NewInt(mempool)
		}
	}

	return bal, nil
}

// mempoolValue returns the total value (satoshis) of an address's unconfirmed
// (mempool, height == 0) unspent outputs. Returns 0 on any error — this is a
// best-effort enrichment of the balance, never a hard failure.
func (c *Client) mempoolValue(ctx context.Context, address string) int64 {
	utxos, err := c.woc.AddressUnspentTransactions(ctx, address)
	if err != nil {
		c.debug("mempool reconcile: unspent fetch failed for %s: %v", address, err)
		return 0
	}
	var total int64
	for _, u := range utxos {
		if u == nil || u.Height > 0 {
			continue // skip confirmed UTXOs
		}
		total += u.Value
	}
	return total
}

// bulkMempoolValues returns per-address total value (satoshis) of unconfirmed
// (mempool) unspent outputs for a batch of addresses. Best-effort: returns an
// empty map on error so balance reconciliation never fails the balance fetch.
func (c *Client) bulkMempoolValues(ctx context.Context, addresses []string) map[string]int64 {
	out := make(map[string]int64, len(addresses))
	resp, err := c.woc.BulkAddressUnconfirmedUTXOs(ctx, &whatsonchain.AddressList{Addresses: addresses})
	if err != nil {
		c.debug("bulk mempool reconcile: unconfirmed UTXO fetch failed: %v", err)
		return out
	}
	for _, rec := range resp {
		if rec == nil {
			continue
		}
		if total := sumHistoryValues(rec.Utxos); total > 0 {
			out[rec.Address] = total
		}
	}
	return out
}

// sumHistoryValues sums the (nil-safe) values of a set of unspent records.
func sumHistoryValues(recs []*whatsonchain.HistoryRecord) int64 {
	var total int64
	for _, u := range recs {
		if u != nil {
			total += u.Value
		}
	}
	return total
}

// GetAllBalances retrieves all BSV balances (just native for BSV).
func (c *Client) GetAllBalances(ctx context.Context, address string) ([]*Balance, error) {
	balance, err := c.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, err
	}

	return []*Balance{balance}, nil
}

// GetBulkNativeBalance fetches balances for multiple BSV addresses in batches.
// Uses bulk API endpoints (max 20 addresses per call) for improved performance.
// Returns a map of address -> Balance, with both confirmed and unconfirmed amounts.
//
// Important: When the bulk API returns an address with nil Balance pointer,
// the address is excluded from results. This triggers the fallback mechanism in
// the balance fetcher (fetcher.go:430-444), which retries with individual fetch.
// This handles a known issue where WhatsOnChain bulk API occasionally returns
// nil Balance for valid addresses.
//
//nolint:gocognit,gocyclo // Complex business logic for bulk balance fetching with batching and merging
func (c *Client) GetBulkNativeBalance(ctx context.Context, addresses []string) (map[string]*Balance, error) {
	if len(addresses) == 0 {
		return make(map[string]*Balance), nil
	}

	// Split into batches of 20 (SDK limit)
	const batchSize = 20
	results := make(map[string]*Balance, len(addresses))

	for i := 0; i < len(addresses); i += batchSize {
		end := i + batchSize
		if end > len(addresses) {
			end = len(addresses)
		}
		batch := addresses[i:end]

		start := time.Now()

		// Fetch confirmed balances
		confirmedList := &whatsonchain.AddressList{Addresses: batch}
		confirmed, err := c.woc.BulkAddressConfirmedBalance(ctx, confirmedList)
		if err != nil {
			metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
			return nil, sigilerr.Wrap(err, "fetching bulk confirmed balances")
		}

		// Fetch unconfirmed balances
		unconfirmedList := &whatsonchain.AddressList{Addresses: batch}
		unconfirmed, err := c.woc.BulkAddressUnconfirmedBalance(ctx, unconfirmedList)
		if err != nil {
			metrics.Global.RecordRPCCall("bsv", time.Since(start), err)
			return nil, sigilerr.Wrap(err, "fetching bulk unconfirmed balances")
		}

		// The balance endpoint under-reports mempool funds (notably on testnet),
		// so also fetch mempool UTXOs to reconcile the unconfirmed amount below.
		mempool := c.bulkMempoolValues(ctx, batch)

		metrics.Global.RecordRPCCall("bsv", time.Since(start), nil)

		// Merge results
		for _, addr := range batch {
			confirmedBalance := int64(0)
			unconfirmedBalance := int64(0)
			addressInResponse := false

			// Get confirmed balance from response
			for _, result := range confirmed {
				if result.Address == addr {
					if result.Balance != nil {
						addressInResponse = true
						confirmedBalance = result.Balance.Confirmed
					} else {
						c.debug("Address %s returned nil Balance in bulk confirmed API, will use individual fetch fallback", addr)
					}
					break
				}
			}

			// Get unconfirmed balance from response
			for _, result := range unconfirmed {
				if result.Address == addr {
					if result.Balance != nil {
						addressInResponse = true
						unconfirmedBalance = result.Balance.Unconfirmed
					} else {
						c.debug("Address %s returned nil Balance in bulk unconfirmed API, will use individual fetch fallback", addr)
					}
					break
				}
			}

			// Only add to results if address was found with valid balance data.
			// If address appears in response but with nil Balance, it's excluded to trigger
			// the fallback mechanism in fetcher.go which retries with individual fetch.
			if !addressInResponse {
				continue
			}

			bal := &Balance{
				Address:  addr,
				Amount:   big.NewInt(confirmedBalance),
				Symbol:   "BSV",
				Decimals: decimals,
			}
			if unconfirmedBalance != 0 {
				bal.Unconfirmed = big.NewInt(unconfirmedBalance)
			} else if mp := mempool[addr]; mp > 0 {
				// Endpoint reported no unconfirmed delta but the address has
				// pending mempool UTXOs — surface them as unconfirmed.
				bal.Unconfirmed = big.NewInt(mp)
			}

			results[addr] = bal
		}
	}

	return results, nil
}
