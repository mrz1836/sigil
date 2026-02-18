package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/utxostore"
)

func TestRefreshBatch_BSV_UpdatesUTXOs(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ABC123"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh was successful
	assert.True(t, results[0].Success)
	require.NoError(t, results[0].Error)
	assert.Equal(t, "1ABC123", results[0].Address)

	// Verify UTXO store was called
	assert.NotNil(t, utxoProvider.addresses[string(chain.BSV)+":1ABC123"])
}

func TestRefreshBatch_ETH_UpdatesBalance(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up balance provider to return a balance
	balanceProvider.setBalance(chain.ETH, "0x123", "1.5", "ETH", 18)

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0x123"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh was successful
	assert.True(t, results[0].Success)
	require.NoError(t, results[0].Error)
	assert.Equal(t, "0x123", results[0].Address)
}

func TestRefreshBatch_UnknownChain(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   "UNKNOWN",
		Addresses: []string{"someaddress"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh failed with unknown chain error
	assert.False(t, results[0].Success)
	require.Error(t, results[0].Error)
	assert.ErrorIs(t, results[0].Error, ErrUnknownChain)
}

func TestRefreshBatch_UnsupportedChain(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BTC,
		Addresses: []string{"1BTCADDRESS"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh failed with unsupported chain error
	assert.False(t, results[0].Success)
	require.Error(t, results[0].Error)
	assert.ErrorIs(t, results[0].Error, ErrUnsupportedChain)
}

func TestRefreshBatch_NetworkError(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up error in balance provider
	balanceProvider.fetchErr = errors.New("network timeout") //nolint:err113 // test error

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.ETH,
		Addresses: []string{"0x123"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify refresh failed
	assert.False(t, results[0].Success)
	require.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "network timeout")
}

func TestRefreshBatch_MultipleAddresses(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ABC", "1DEF", "1GHI"},
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify all refreshes were successful
	for i, result := range results {
		assert.True(t, result.Success, "address %d failed", i)
		assert.NoError(t, result.Error, "address %d has error", i)
	}
}

func TestCheckAddress_BSV_WithUTXOs(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Add address with UTXOs
	utxoProvider.addAddress(chain.BSV, "1ABC123", 100000)

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: chain.BSV,
		Address: "1ABC123",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "1ABC123", result.Address)
	assert.Equal(t, chain.BSV, result.ChainID)
	assert.Equal(t, uint64(100000), result.Balance)
	assert.Len(t, result.UTXOs, 1)
	assert.True(t, result.HasActivity)
}

func TestCheckAddress_BSV_InactiveNoUTXOs(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Add address with no UTXOs
	utxoProvider.addAddress(chain.BSV, "1EMPTY", 0)

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: chain.BSV,
		Address: "1EMPTY",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "1EMPTY", result.Address)
	assert.Equal(t, uint64(0), result.Balance)
	assert.Empty(t, result.UTXOs)
}

func TestCheckAddress_ETH_BalanceOnly(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: chain.ETH,
		Address: "0x123",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "0x123", result.Address)
	assert.Equal(t, chain.ETH, result.ChainID)
	assert.Nil(t, result.UTXOs) // ETH has no UTXOs
}

func TestCheckAddress_UnknownChain(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: "UNKNOWN",
		Address: "someaddress",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnknownChain)
}

func TestCheckAddress_UnsupportedChain(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	tests := []struct {
		name    string
		chainID chain.ID
		address string
	}{
		{
			name:    "BTC unsupported",
			chainID: chain.BTC,
			address: "1BTCAddress",
		},
		{
			name:    "BCH unsupported",
			chainID: chain.BCH,
			address: "1BCHAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := &CheckRequest{
				ChainID: tt.chainID,
				Address: tt.address,
			}

			result, err := service.CheckAddress(context.Background(), req)
			require.Error(t, err)
			assert.Nil(t, result)
			require.ErrorIs(t, err, ErrUnsupportedChain)
			assert.Contains(t, err.Error(), string(tt.chainID))
		})
	}
}

func TestCheckAddress_BSV_RefreshError(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Set up refresh error
	utxoProvider.refreshErr = errors.New("network timeout") //nolint:err113 // Test error

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: chain.BSV,
		Address: "1FAIL",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "refreshing BSV address")
	assert.Contains(t, err.Error(), "network timeout")
}

func TestCheckAddress_BSV_WithLabel(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	// Add address with label
	key := string(chain.BSV) + ":1LABELED"
	utxoProvider.addresses[key] = &utxostore.AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1LABELED",
		HasActivity: true,
		Label:       "My Wallet",
	}
	utxoProvider.utxos[key] = []*utxostore.StoredUTXO{
		{
			ChainID: chain.BSV,
			Address: "1LABELED",
			TxID:    "tx1",
			Vout:    0,
			Amount:  50000,
			Spent:   false,
		},
	}

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &CheckRequest{
		ChainID: chain.BSV,
		Address: "1LABELED",
	}

	result, err := service.CheckAddress(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "1LABELED", result.Address)
	assert.Equal(t, uint64(50000), result.Balance)
	assert.Equal(t, "My Wallet", result.Label)
	assert.True(t, result.HasActivity)
}

func TestRefreshBatch_ContextCancellation(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ABC"},
	}

	results, err := service.RefreshBatch(ctx, req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Should detect context cancellation
	assert.False(t, results[0].Success)
	assert.Error(t, results[0].Error)
}

func TestRefreshBatch_WithTimeout(t *testing.T) {
	t.Parallel()

	utxoProvider := newMockUTXOProvider()
	balanceProvider := newMockBalanceProvider()
	configProvider := newMockConfigProvider()

	service := NewService(&Config{
		UTXOStore:      utxoProvider,
		BalanceService: balanceProvider,
		Config:         configProvider,
	})

	req := &RefreshRequest{
		ChainID:   chain.BSV,
		Addresses: []string{"1ABC"},
		Timeout:   5 * time.Second,
	}

	results, err := service.RefreshBatch(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Should succeed within timeout
	assert.True(t, results[0].Success)
}

// TestGetLabel_Nil tests getLabel with nil metadata.
func TestGetLabel_Nil(t *testing.T) {
	t.Parallel()

	label := getLabel(nil)
	assert.Empty(t, label)
}

// TestGetLabel_WithLabel tests getLabel with non-nil metadata.
func TestGetLabel_WithLabel(t *testing.T) {
	t.Parallel()

	meta := &utxostore.AddressMetadata{
		ChainID: chain.BSV,
		Address: "1ABC",
		Label:   "Test Label",
	}

	label := getLabel(meta)
	assert.Equal(t, "Test Label", label)
}

// TestGetLabel_EmptyLabel tests getLabel with empty label in metadata.
func TestGetLabel_EmptyLabel(t *testing.T) {
	t.Parallel()

	meta := &utxostore.AddressMetadata{
		ChainID: chain.BSV,
		Address: "1ABC",
		Label:   "",
	}

	label := getLabel(meta)
	assert.Empty(t, label)
}

// Mock implementations

type mockUTXOProvider struct {
	addresses  map[string]*utxostore.AddressMetadata
	utxos      map[string][]*utxostore.StoredUTXO
	refreshErr error
}

func newMockUTXOProvider() *mockUTXOProvider {
	return &mockUTXOProvider{
		addresses: make(map[string]*utxostore.AddressMetadata),
		utxos:     make(map[string][]*utxostore.StoredUTXO),
	}
}

func (m *mockUTXOProvider) RefreshAddress(_ context.Context, address string, chainID chain.ID, _ ChainClient) error {
	if m.refreshErr != nil {
		return m.refreshErr
	}
	key := string(chainID) + ":" + address
	if m.addresses[key] == nil {
		m.addresses[key] = &utxostore.AddressMetadata{
			ChainID:     chainID,
			Address:     address,
			LastScanned: time.Now(),
		}
	}
	m.addresses[key].LastScanned = time.Now()
	return nil
}

func (m *mockUTXOProvider) GetAddressBalance(chainID chain.ID, address string) uint64 {
	key := string(chainID) + ":" + address
	var total uint64
	for _, utxo := range m.utxos[key] {
		if !utxo.Spent {
			total += utxo.Amount
		}
	}
	return total
}

func (m *mockUTXOProvider) GetUTXOs(chainID chain.ID, address string) []*utxostore.StoredUTXO {
	key := string(chainID) + ":" + address
	return m.utxos[key]
}

func (m *mockUTXOProvider) GetAddress(chainID chain.ID, address string) *utxostore.AddressMetadata {
	key := string(chainID) + ":" + address
	return m.addresses[key]
}

func (m *mockUTXOProvider) addAddress(chainID chain.ID, address string, satoshis uint64) {
	key := string(chainID) + ":" + address
	m.addresses[key] = &utxostore.AddressMetadata{
		ChainID:     chainID,
		Address:     address,
		LastScanned: time.Now(),
		HasActivity: satoshis > 0,
	}
	if satoshis > 0 {
		m.utxos[key] = []*utxostore.StoredUTXO{
			{
				ChainID: chainID,
				Address: address,
				TxID:    "mock-txid",
				Vout:    0,
				Amount:  satoshis,
				Spent:   false,
			},
		}
	}
}

type mockBalanceProvider struct {
	balances       map[string]*balance.FetchResult
	fetchErr       error
	fetchDelay     time.Duration
	fetchDelayFunc func(address string) time.Duration
}

func newMockBalanceProvider() *mockBalanceProvider {
	return &mockBalanceProvider{
		balances: make(map[string]*balance.FetchResult),
	}
}

func (m *mockBalanceProvider) FetchBalance(ctx context.Context, req *balance.FetchRequest) (*balance.FetchResult, error) {
	// Apply delay if specified
	delay := m.fetchDelay
	if m.fetchDelayFunc != nil {
		delay = m.fetchDelayFunc(req.Address)
	}

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	key := string(req.ChainID) + ":" + req.Address
	if result, exists := m.balances[key]; exists {
		return result, nil
	}
	return &balance.FetchResult{
		ChainID:  req.ChainID,
		Address:  req.Address,
		Balances: []balance.BalanceEntry{},
	}, nil
}

func (m *mockBalanceProvider) setBalance(chainID chain.ID, address, amount, symbol string, decimals int) {
	key := string(chainID) + ":" + address
	m.balances[key] = &balance.FetchResult{
		ChainID: chainID,
		Address: address,
		Balances: []balance.BalanceEntry{
			{
				Balance:  amount,
				Symbol:   symbol,
				Decimals: decimals,
			},
		},
	}
}

type mockConfigProvider struct {
	bsvAPIKey          string
	ethEtherscanAPIKey string
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{ //nolint:gosec // G101: test data, not real credentials
		bsvAPIKey:          "test-bsv-key",
		ethEtherscanAPIKey: "test-etherscan-key",
	}
}

func (m *mockConfigProvider) GetBSVAPIKey() string {
	return m.bsvAPIKey
}

func (m *mockConfigProvider) GetETHEtherscanAPIKey() string {
	return m.ethEtherscanAPIKey
}
