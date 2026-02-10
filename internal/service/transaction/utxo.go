package transaction

import (
	"context"
	"fmt"
	"sync"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// aggregateBSVUTXOs fetches UTXOs from all wallet addresses concurrently and merges them.
// Migrated from cli/tx.go lines 998-1041
func aggregateBSVUTXOs(ctx context.Context, client *bsv.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	type result struct {
		utxos []chain.UTXO
		err   error
	}

	results := make([]result, len(addresses))
	var wg sync.WaitGroup

	for i, addr := range addresses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			utxos, err := client.ListUTXOs(ctx, addr.Address)
			if err != nil {
				results[i] = result{err: fmt.Errorf("listing UTXOs for %s: %w", addr.Address, err)}
				return
			}
			converted := make([]chain.UTXO, len(utxos))
			for j, u := range utxos {
				converted[j] = chain.UTXO{
					TxID:          u.TxID,
					Vout:          u.Vout,
					Amount:        u.Amount,
					ScriptPubKey:  u.ScriptPubKey,
					Address:       u.Address,
					Confirmations: u.Confirmations,
				}
			}
			results[i] = result{utxos: converted}
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

// AggregateBSVUTXOs is the exported version for external use.
func AggregateBSVUTXOs(ctx context.Context, client *bsv.Client, addresses []wallet.Address) ([]chain.UTXO, error) {
	return aggregateBSVUTXOs(ctx, client, addresses)
}

// filterSpentBSVUTXOs removes UTXOs that are marked as spent in the local store.
// UTXOs not present in the store are kept (unknown is not known-spent).
// Migrated from cli/tx.go lines 1101-1111
func filterSpentBSVUTXOs(utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	if store == nil {
		return utxos
	}

	filtered := make([]chain.UTXO, 0, len(utxos))
	for _, u := range utxos {
		if !store.IsSpent(chain.BSV, u.TxID, u.Vout) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// FilterSpentBSVUTXOs is the exported version for external use.
func FilterSpentBSVUTXOs(utxos []chain.UTXO, store UTXOProvider) []chain.UTXO {
	return filterSpentBSVUTXOs(utxos, store)
}

// markSpentBSVUTXOs records spent UTXOs in the local store after a successful broadcast.
// Errors are logged but never returned — the broadcast already succeeded.
// Migrated from cli/tx.go lines 1113-1138
func markSpentBSVUTXOs(logger LogWriter, store UTXOProvider, utxos []chain.UTXO, spentTxID string) {
	if store == nil {
		return
	}

	for _, u := range utxos {
		// Ensure the UTXO exists in the store before marking it spent.
		// The API may return UTXOs not yet tracked locally.
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID:       chain.BSV,
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		})
		store.MarkSpent(chain.BSV, u.TxID, u.Vout, spentTxID)
	}

	if err := store.Save(); err != nil {
		if logger != nil {
			logger.Error("bsv send: failed to save utxo store: %v", err)
		}
	}
}

// MarkSpentBSVUTXOs is the exported version for external use.
func MarkSpentBSVUTXOs(logger LogWriter, store UTXOProvider, utxos []chain.UTXO, spentTxID string) {
	markSpentBSVUTXOs(logger, store, utxos, spentTxID)
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
