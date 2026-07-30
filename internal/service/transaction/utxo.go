package transaction

import (
	"context"
	"fmt"
	"sync"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// btcAggregateConcurrency bounds concurrent per-address UTXO fetches for BTC.
// Unlike BSV's unbounded fan-out, BTC uses the Esplora API (mempool.space) which
// rate-limits harder, so a small worker pool keeps request bursts polite (the
// btc client's own provider rate-limiter throttles the actual HTTP calls).
const btcAggregateConcurrency = 5

// utxoLister is the ListUTXOs surface shared by the BSV and BTC clients (both
// return []chain.UTXO).
type utxoLister interface {
	ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error)
}

// aggregateUTXOs fetches UTXOs from all wallet addresses concurrently and merges
// them in address order. A positive concurrency bounds the fan-out with a worker
// pool (for rate-limited providers such as BTC's Esplora); concurrency <= 0 fans
// out unbounded (BSV, whose WhatsOnChain provider handles its own rate limiting).
func aggregateUTXOs(ctx context.Context, client utxoLister, addresses []wallet.Address, concurrency int) ([]chain.UTXO, error) {
	type result struct {
		utxos []chain.UTXO
		err   error
	}

	// A positive concurrency bounds the fan-out; otherwise size the semaphore to
	// the address count so every goroutine runs immediately (unbounded).
	limit := concurrency
	if limit <= 0 {
		limit = len(addresses)
	}

	results := make([]result, len(addresses))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, addr := range addresses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			utxos, err := client.ListUTXOs(ctx, addr.Address)
			if err != nil {
				results[i] = result{err: fmt.Errorf("listing UTXOs for %s: %w", addr.Address, err)}
				return
			}
			results[i] = result{utxos: utxos}
		}()
	}
	wg.Wait()

	var allUTXOs []chain.UTXO
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		allUTXOs = append(allUTXOs, r.utxos...)
	}
	return allUTXOs, nil
}

// aggregateBSVUTXOs fetches UTXOs from all wallet addresses with an unbounded
// fan-out (the WhatsOnChain provider handles rate limiting).
func aggregateBSVUTXOs(ctx context.Context, client *bsv.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	return aggregateUTXOs(ctx, client, addresses, 0)
}

// AggregateBSVUTXOs is the exported version for external use.
func AggregateBSVUTXOs(ctx context.Context, client *bsv.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	return aggregateBSVUTXOs(ctx, client, addresses)
}

// aggregateBTCUTXOs fetches UTXOs from all wallet addresses using a bounded worker
// pool to stay within the Esplora provider's rate limits.
func aggregateBTCUTXOs(ctx context.Context, client *btc.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	return aggregateUTXOs(ctx, client, addresses, btcAggregateConcurrency)
}

// AggregateBTCUTXOs is the exported version for external use.
func AggregateBTCUTXOs(ctx context.Context, client *btc.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	return aggregateBTCUTXOs(ctx, client, addresses)
}

// filterSpentUTXOs removes UTXOs marked as spent in the local store for the given
// chain. UTXOs not present in the store are kept (unknown is not known-spent).
func filterSpentUTXOs(chainID chain.ID, utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	if store == nil {
		return utxos
	}

	filtered := make([]chain.UTXO, 0, len(utxos))
	for _, u := range utxos {
		if !store.IsSpent(chainID, u.TxID, u.Vout) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// markSpentUTXOs records spent UTXOs in the local store for the given chain after
// a successful broadcast. Errors are logged but never returned — the broadcast
// already succeeded.
func markSpentUTXOs(logger LogWriter, store UTXOProvider, chainID chain.ID, utxos []chain.UTXO, spentTxID string) {
	if store == nil {
		return
	}

	for _, u := range utxos {
		// Ensure the UTXO exists in the store before marking it spent.
		// The API may return UTXOs not yet tracked locally.
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID:       chainID,
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		})
		store.MarkSpent(chainID, u.TxID, u.Vout, spentTxID)
	}

	if err := store.Save(); err != nil {
		if logger != nil {
			logger.Error("send: failed to save utxo store: %v", err)
		}
	}
}

// filterSpentBSVUTXOs removes BSV UTXOs marked as spent in the local store.
// Migrated from cli/tx.go lines 1101-1111
func filterSpentBSVUTXOs(utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	return filterSpentUTXOs(chain.BSV, utxos, store)
}

// FilterSpentBSVUTXOs is the exported version for external use.
func FilterSpentBSVUTXOs(utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	return filterSpentUTXOs(chain.BSV, utxos, store)
}

// FilterSpentBTCUTXOs removes BTC UTXOs marked as spent in the local store.
func FilterSpentBTCUTXOs(utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	return filterSpentUTXOs(chain.BTC, utxos, store)
}

// markSpentBSVUTXOs records spent BSV UTXOs in the local store after a broadcast.
// Migrated from cli/tx.go lines 1113-1138
func markSpentBSVUTXOs(logger LogWriter, store UTXOProvider, utxos []chain.UTXO, spentTxID string) {
	markSpentUTXOs(logger, store, chain.BSV, utxos, spentTxID)
}

// MarkSpentBSVUTXOs is the exported version for external use.
func MarkSpentBSVUTXOs(logger LogWriter, store UTXOProvider, utxos []chain.UTXO, spentTxID string) {
	markSpentUTXOs(logger, store, chain.BSV, utxos, spentTxID)
}

// MarkSpentBTCUTXOs is the exported version for external use.
func MarkSpentBTCUTXOs(logger LogWriter, store UTXOProvider, utxos []chain.UTXO, spentTxID string) {
	markSpentUTXOs(logger, store, chain.BTC, utxos, spentTxID)
}

// uniqueUTXOAddrs returns the unique set of addresses that appear in a UTXO slice.
// Migrated from cli/tx.go lines 1092-1099
func uniqueUTXOAddrs(utxos []chain.UTXO) map[string]struct{} {
	addrs := make(map[string]struct{})
	for _, u := range utxos {
		addrs[u.Address] = struct{}{}
	}
	return addrs
}

// UniqueUTXOAddrs is the exported version for external use.
func UniqueUTXOAddrs(utxos []chain.UTXO) map[string]struct{} {
	return uniqueUTXOAddrs(utxos)
}
