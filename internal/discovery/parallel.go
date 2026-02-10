package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultParallelWorkers is the default number of concurrent scheme scanners.
	DefaultParallelWorkers = 3
)

// ParallelScanner performs concurrent multi-path wallet discovery.
type ParallelScanner struct {
	client     ChainClient
	deriver    KeyDeriver
	opts       *Options
	maxWorkers int
}

// NewParallelScanner creates a new parallel discovery scanner.
func NewParallelScanner(client ChainClient, deriver KeyDeriver, opts *Options, maxWorkers int) *ParallelScanner {
	if opts == nil {
		opts = DefaultOptions()
	}
	if maxWorkers <= 0 {
		maxWorkers = DefaultParallelWorkers
	}

	return &ParallelScanner{
		client:     client,
		deriver:    deriver,
		opts:       opts,
		maxWorkers: maxWorkers,
	}
}

// scanJob represents a single scheme scan job.
type scanJob struct {
	scheme   PathScheme
	gapLimit int
	index    int // Original index for result ordering
}

// scanResult represents the result of a single scheme scan.
type scanResult struct {
	schemeName string
	addresses  []DiscoveredAddress
	balance    uint64
	utxoCount  int
	scanned    int
	err        error
	index      int // Original index for result ordering
}

// ScanParallel performs parallel discovery across all configured path schemes.
// Returns combined results from all schemes.
//
//nolint:gocognit // Parallel coordination inherently complex
func (p *ParallelScanner) ScanParallel(ctx context.Context, seed []byte) (*Result, error) {
	if err := p.opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	if len(seed) == 0 {
		return nil, ErrInvalidSeed
	}

	startTime := time.Now()

	// Create scanner for executing individual scans
	scanner := NewScanner(p.client, p.deriver, p.opts)

	// Sort schemes by priority
	schemes := SortByPriority(p.opts.PathSchemes)

	// Create job queue
	jobs := make(chan scanJob, len(schemes))
	results := make(chan scanResult, len(schemes))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < p.maxWorkers; i++ {
		wg.Add(1)
		go p.worker(ctx, seed, scanner, jobs, results, &wg)
	}

	// Submit jobs
	for i, scheme := range schemes {
		// Use extended gap limit for first (highest priority) scheme
		gapLimit := p.opts.GapLimit
		if i == 0 {
			gapLimit = p.opts.ExtendedGapLimit
		}

		jobs <- scanJob{
			scheme:   scheme,
			gapLimit: gapLimit,
			index:    i,
		}
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	allResults := make([]scanResult, 0, len(schemes))
	for result := range results {
		allResults = append(allResults, result)
	}

	// Build combined result
	combinedResult := &Result{
		FoundAddresses: make(map[string][]DiscoveredAddress),
		Duration:       time.Since(startTime),
	}

	// Sort results by original index to maintain scheme priority order
	sortResultsByIndex(allResults)

	for _, res := range allResults {
		if res.err != nil {
			combinedResult.Errors = append(combinedResult.Errors,
				fmt.Sprintf("%s: %v", res.schemeName, res.err))
			continue
		}

		if len(res.addresses) > 0 {
			combinedResult.FoundAddresses[res.schemeName] = res.addresses
			combinedResult.TotalBalance += res.balance
			combinedResult.TotalUTXOs += res.utxoCount
		}

		combinedResult.SchemesScanned = append(combinedResult.SchemesScanned, res.schemeName)
		combinedResult.AddressesScanned += res.scanned
	}

	return combinedResult, nil
}

// worker processes scan jobs from the queue.
//
//nolint:funcorder // Worker method grouped with ScanParallel
func (p *ParallelScanner) worker(
	ctx context.Context,
	seed []byte,
	scanner *Scanner,
	jobs <-chan scanJob,
	results chan<- scanResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for job := range jobs {
		// Check for cancellation
		if ctx.Err() != nil {
			results <- scanResult{
				schemeName: job.scheme.Name,
				err:        ctx.Err(),
				index:      job.index,
			}
			continue
		}

		// Execute scan
		schemeResult, err := scanner.scanScheme(ctx, seed, job.scheme, job.gapLimit)

		// Send result
		results <- scanResult{
			schemeName: job.scheme.Name,
			addresses:  schemeResult.addresses,
			balance:    schemeResult.balance,
			utxoCount:  schemeResult.utxoCount,
			scanned:    schemeResult.scanned,
			err:        err,
			index:      job.index,
		}
	}
}

// sortResultsByIndex sorts results by their original index.
func sortResultsByIndex(results []scanResult) {
	// Simple insertion sort since the list is small
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].index > key.index {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}
}

// ScanSchemesParallel scans specific schemes in parallel.
// Useful for targeted recovery when the user knows which wallets to check.
func (p *ParallelScanner) ScanSchemesParallel(ctx context.Context, seed []byte, schemeNames []string) (*Result, error) {
	// Build scheme list from names
	schemes := make([]PathScheme, 0, len(schemeNames))
	for _, name := range schemeNames {
		scheme := SchemeByName(name)
		if scheme == nil {
			return nil, fmt.Errorf("%w: %s", ErrUnknownScheme, name)
		}
		schemes = append(schemes, *scheme)
	}

	// Temporarily override path schemes
	originalSchemes := p.opts.PathSchemes
	p.opts.PathSchemes = schemes
	defer func() {
		p.opts.PathSchemes = originalSchemes
	}()

	return p.ScanParallel(ctx, seed)
}
