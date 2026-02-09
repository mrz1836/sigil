package bsv

import (
	"context"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
)

// mockWOCClient implements WOCClient for testing.
type mockWOCClient struct {
	balanceFunc   func(ctx context.Context, address string) (*whatsonchain.AddressBalance, error)
	utxoFunc      func(ctx context.Context, address string) (whatsonchain.AddressHistory, error)
	feeFunc       func(ctx context.Context, from, to int64) ([]*whatsonchain.MinerFeeStats, error)
	broadcastFunc func(ctx context.Context, txHex string) (string, error)
}

func (m *mockWOCClient) AddressBalance(ctx context.Context, address string) (*whatsonchain.AddressBalance, error) {
	if m.balanceFunc != nil {
		return m.balanceFunc(ctx, address)
	}
	return &whatsonchain.AddressBalance{}, nil
}

func (m *mockWOCClient) AddressUnspentTransactions(ctx context.Context, address string) (whatsonchain.AddressHistory, error) {
	if m.utxoFunc != nil {
		return m.utxoFunc(ctx, address)
	}
	return whatsonchain.AddressHistory{}, nil
}

func (m *mockWOCClient) GetMinerFeesStats(ctx context.Context, from, to int64) ([]*whatsonchain.MinerFeeStats, error) {
	if m.feeFunc != nil {
		return m.feeFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *mockWOCClient) BroadcastTx(ctx context.Context, txHex string) (string, error) {
	if m.broadcastFunc != nil {
		return m.broadcastFunc(ctx, txHex)
	}
	return "", nil
}

// toHistoryRecords converts a slice of UTXO to whatsonchain.AddressHistory for test mocks.
func toHistoryRecords(utxos []UTXO) whatsonchain.AddressHistory {
	records := make(whatsonchain.AddressHistory, len(utxos))
	for i, u := range utxos {
		records[i] = &whatsonchain.HistoryRecord{
			TxHash: u.TxID,
			TxPos:  int64(u.Vout),
			Value:  int64(u.Amount), //nolint:gosec // Test values are small and safe
			Height: 100,
		}
	}
	return records
}
