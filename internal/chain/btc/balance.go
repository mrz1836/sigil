package btc

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/metrics"
)

// Balance is an alias for chain.Balance (identical fields) so the BTC balance
// client and the service layer share one type.
type Balance = chain.Balance

// GetNativeBalance retrieves the native BTC balance including unconfirmed data.
// Confirmed balance is chain funded − spent; the unconfirmed delta is the mempool
// funded − spent (negative when a pending spend outweighs pending receipts).
func (c *Client) GetNativeBalance(ctx context.Context, address string) (*Balance, error) {
	start := time.Now()
	stats, err := c.doGetStats(ctx, address)
	metrics.Global.RecordRPCCall("btc", time.Since(start), err)
	if err != nil {
		return nil, err
	}

	confirmed := stats.ChainStats.FundedTxoSum - stats.ChainStats.SpentTxoSum
	unconfirmed := stats.MempoolStats.FundedTxoSum - stats.MempoolStats.SpentTxoSum

	bal := &Balance{
		Address:  address,
		Amount:   big.NewInt(confirmed),
		Symbol:   "BTC",
		Decimals: decimals,
	}
	if unconfirmed != 0 {
		bal.Unconfirmed = big.NewInt(unconfirmed)
	}

	return bal, nil
}

// doGetStats validates the address and fetches its Esplora stats.
func (c *Client) doGetStats(ctx context.Context, address string) (*AddressStats, error) {
	if err := c.ValidateAddress(address); err != nil {
		return nil, err
	}
	return c.provider.AddressStats(ctx, address)
}

// btcBulkConcurrency bounds concurrent per-address balance fetches. The provider's
// own rate limiter throttles request rate; this just caps in-flight goroutines.
const btcBulkConcurrency = 5

// GetBulkNativeBalance fetches balances for multiple BTC addresses. Esplora has no
// bulk balance endpoint, so this fans out to per-address requests with bounded
// concurrency (the provider rate-limits the actual HTTP calls). Addresses whose
// fetch fails are omitted from the result map, which triggers the balance
// fetcher's per-address fallback (matching the BSV bulk contract). An error is
// returned only when every address fails (so the caller can fall back to cache).
func (c *Client) GetBulkNativeBalance(ctx context.Context, addresses []string) (map[string]*Balance, error) {
	results := make(map[string]*Balance, len(addresses))
	if len(addresses) == 0 {
		return results, nil
	}

	type balanceResult struct {
		addr string
		bal  *Balance
		err  error
	}

	sem := make(chan struct{}, btcBulkConcurrency)
	resCh := make(chan balanceResult, len(addresses))
	var wg sync.WaitGroup

	for _, addr := range addresses {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			bal, err := c.GetNativeBalance(ctx, a)
			resCh <- balanceResult{addr: a, bal: bal, err: err}
		}(addr)
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	var firstErr error
	for r := range resCh {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			c.debug("bulk balance: fetch failed for %s: %v", r.addr, r.err)
			continue
		}
		results[r.addr] = r.bal
	}

	// Only surface an error if nothing succeeded; partial results trigger the
	// fetcher's per-address fallback for the missing addresses.
	if len(results) == 0 && firstErr != nil {
		return results, firstErr
	}
	return results, nil
}
