package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// Test mnemonic for generating test seeds
const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

func getTestSeed(t *testing.T) []byte {
	t.Helper()
	seed, err := wallet.MnemonicToSeed(testMnemonic, "")
	require.NoError(t, err)
	return seed
}

func TestDeriveNext_FromSeed_BSV(t *testing.T) {
	t.Parallel()

	// Create wallet with BSV enabled
	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}

	seed := getTestSeed(t)

	service := NewService(nil)
	req := &DerivationRequest{
		Wallet:  w,
		Seed:    seed,
		ChainID: chain.BSV,
	}

	addr, err := service.DeriveNext(req)
	require.NoError(t, err)
	assert.NotNil(t, addr)
	assert.NotEmpty(t, addr.Address)
	assert.Equal(t, uint32(0), addr.Index)
	assert.Contains(t, addr.Path, "m/44'/236'/0'/0/0")

	// Verify address was added to wallet
	assert.Len(t, w.Addresses[chain.BSV], 1)
	assert.Equal(t, addr.Address, w.Addresses[chain.BSV][0].Address)
}

func TestDeriveNext_FromSeed_ETH(t *testing.T) {
	t.Parallel()

	// Create wallet with ETH enabled
	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.ETH},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}

	seed := getTestSeed(t)

	service := NewService(nil)
	req := &DerivationRequest{
		Wallet:  w,
		Seed:    seed,
		ChainID: chain.ETH,
	}

	addr, err := service.DeriveNext(req)
	require.NoError(t, err)
	assert.NotNil(t, addr)
	assert.NotEmpty(t, addr.Address)
	assert.Equal(t, uint32(0), addr.Index)
	assert.Contains(t, addr.Path, "m/44'/60'/0'/0/0")

	// Verify address was added to wallet
	assert.Len(t, w.Addresses[chain.ETH], 1)
}

func TestDeriveNext_FromXpub(t *testing.T) {
	t.Parallel()

	// First derive with seed to get xpub
	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}
	seed := getTestSeed(t)

	// Get xpub for BSV account 0
	xpub, err := wallet.DeriveAccountXpub(seed, chain.BSV, 0)
	require.NoError(t, err)

	service := NewService(nil)
	req := &DerivationRequest{
		Wallet:  w,
		ChainID: chain.BSV,
		Xpub:    xpub,
	}

	addr, err := service.DeriveNext(req)
	require.NoError(t, err)
	assert.NotNil(t, addr)
	assert.NotEmpty(t, addr.Address)
	assert.Equal(t, uint32(0), addr.Index)

	// Verify address was added to wallet
	assert.Len(t, w.Addresses[chain.BSV], 1)
}

func TestDeriveNext_InvalidSeed(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}

	service := NewService(nil)
	req := &DerivationRequest{
		Wallet:  w,
		Seed:    nil, // Explicitly no seed
		ChainID: chain.BSV,
		Xpub:    "", // Explicitly empty xpub
	}

	addr, err := service.DeriveNext(req)
	require.Error(t, err)
	assert.Nil(t, addr)
	// Error should indicate agent xpub is invalid or missing
	assert.Error(t, err)
}

func TestDeriveNext_ChangeAddress(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}
	seed := getTestSeed(t)

	// Derive a change address directly through wallet
	changeAddr, err := w.DeriveNextChangeAddress(seed, chain.BSV)
	require.NoError(t, err)
	assert.NotNil(t, changeAddr)
	assert.Contains(t, changeAddr.Path, "/1/") // Change addresses use index 1
}

func TestCollect_AllAddresses(t *testing.T) {
	t.Parallel()

	// Create wallet with both BSV and ETH addresses
	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV, chain.ETH},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
				{Address: "1DEF", Index: 1, Path: "m/44'/236'/0'/0/1"},
			},
			chain.ETH: {
				{Address: "0x123", Index: 0, Path: "m/44'/60'/0'/0/0"},
			},
		},
		ChangeAddresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1CHANGE", Index: 0, Path: "m/44'/236'/0'/1/0"},
			},
		},
	}

	service := NewService(nil)
	req := &CollectionRequest{
		Wallet:     w,
		TypeFilter: AllTypes,
	}

	results := service.Collect(req)
	assert.Len(t, results, 4) // 2 BSV receive + 1 BSV change + 1 ETH receive

	// Verify we have addresses from both chains
	bsvCount := 0
	ethCount := 0
	changeCount := 0
	for _, info := range results {
		if info.ChainID == chain.BSV {
			bsvCount++
		}
		if info.ChainID == chain.ETH {
			ethCount++
		}
		if info.Type == Change {
			changeCount++
		}
	}
	assert.Equal(t, 3, bsvCount)
	assert.Equal(t, 1, ethCount)
	assert.Equal(t, 1, changeCount)
}

func TestCollect_ReceiveOnly(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
			},
		},
		ChangeAddresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1CHANGE", Index: 0, Path: "m/44'/0'/0'/1/0"},
			},
		},
	}

	service := NewService(nil)
	req := &CollectionRequest{
		Wallet:     w,
		TypeFilter: Receive,
	}

	results := service.Collect(req)
	assert.Len(t, results, 1)
	assert.Equal(t, Receive, results[0].Type)
	assert.Equal(t, "1ABC", results[0].Address)
}

func TestCollect_ChangeOnly(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
			},
		},
		ChangeAddresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1CHANGE", Index: 0, Path: "m/44'/0'/0'/1/0"},
			},
		},
	}

	service := NewService(nil)
	req := &CollectionRequest{
		Wallet:     w,
		TypeFilter: Change,
	}

	results := service.Collect(req)
	assert.Len(t, results, 1)
	assert.Equal(t, Change, results[0].Type)
	assert.Equal(t, "1CHANGE", results[0].Address)
}

func TestCollect_WithMetadata(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
				{Address: "1DEF", Index: 1, Path: "m/44'/0'/0'/0/1"},
			},
		},
	}

	// Create mock metadata provider
	metadata := &mockMetadataProvider{
		metadata: map[string]*AddressMetadata{
			string(chain.BSV) + ":1ABC": {
				HasActivity: true,
				Label:       "Savings",
			},
		},
	}

	service := NewService(metadata)
	req := &CollectionRequest{
		Wallet:     w,
		TypeFilter: AllTypes,
	}

	results := service.Collect(req)
	assert.Len(t, results, 2)

	// First address should have metadata
	assert.Equal(t, "1ABC", results[0].Address)
	assert.True(t, results[0].HasActivity)
	assert.Equal(t, "Savings", results[0].Label)

	// Second address should have no metadata
	assert.Equal(t, "1DEF", results[1].Address)
	assert.False(t, results[1].HasActivity)
	assert.Empty(t, results[1].Label)
}

func TestCollect_EmptyWallet(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}

	service := NewService(nil)
	req := &CollectionRequest{
		Wallet:     w,
		TypeFilter: AllTypes,
	}

	results := service.Collect(req)
	assert.Empty(t, results)
}

func TestCollect_ChainFilter(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV, chain.ETH},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/236'/0'/0/0"},
			},
			chain.ETH: {
				{Address: "0x123", Index: 0, Path: "m/44'/60'/0'/0/0"},
			},
		},
	}

	service := NewService(nil)
	req := &CollectionRequest{
		Wallet:      w,
		ChainFilter: chain.BSV,
		TypeFilter:  AllTypes,
	}

	results := service.Collect(req)
	assert.Len(t, results, 1)
	assert.Equal(t, chain.BSV, results[0].ChainID)
	assert.Equal(t, "1ABC", results[0].Address)
}

func TestFindUnused_HasUnused(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
				{Address: "1DEF", Index: 1, Path: "m/44'/0'/0'/0/1"},
			},
		},
	}

	// Mark first address as used
	metadata := &mockMetadataProvider{
		metadata: map[string]*AddressMetadata{
			string(chain.BSV) + ":1ABC": {HasActivity: true},
		},
	}

	service := NewService(metadata)
	req := &FindRequest{
		Wallet:  w,
		ChainID: chain.BSV,
	}

	addr := service.FindUnused(req)
	require.NotNil(t, addr)
	assert.Equal(t, "1DEF", addr.Address)
	assert.Equal(t, uint32(1), addr.Index)
}

func TestFindUnused_AllUsed(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
			},
		},
	}

	// Mark all addresses as used
	metadata := &mockMetadataProvider{
		metadata: map[string]*AddressMetadata{
			string(chain.BSV) + ":1ABC": {HasActivity: true},
		},
	}

	service := NewService(metadata)
	req := &FindRequest{
		Wallet:  w,
		ChainID: chain.BSV,
	}

	addr := service.FindUnused(req)
	assert.Nil(t, addr)
}

func TestFindUnused_EmptyWallet(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses:     make(map[chain.ID][]wallet.Address),
	}

	service := NewService(nil)
	req := &FindRequest{
		Wallet:  w,
		ChainID: chain.BSV,
	}

	addr := service.FindUnused(req)
	assert.Nil(t, addr)
}

func TestFindUnused_NoMetadata(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV},
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "1ABC", Index: 0, Path: "m/44'/0'/0'/0/0"},
			},
		},
	}

	// No metadata provider - should return first address
	service := NewService(nil)
	req := &FindRequest{
		Wallet:  w,
		ChainID: chain.BSV,
	}

	addr := service.FindUnused(req)
	require.NotNil(t, addr)
	assert.Equal(t, "1ABC", addr.Address)
}

func TestFilterUsage_OnlyUsed(t *testing.T) {
	t.Parallel()

	addresses := []AddressInfo{
		{Address: "1ABC", HasActivity: true},
		{Address: "1DEF", HasActivity: false},
		{Address: "1GHI", HasActivity: true},
	}

	filtered := FilterUsage(addresses, true, false)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "1ABC", filtered[0].Address)
	assert.Equal(t, "1GHI", filtered[1].Address)
}

func TestFilterUsage_OnlyUnused(t *testing.T) {
	t.Parallel()

	addresses := []AddressInfo{
		{Address: "1ABC", HasActivity: true},
		{Address: "1DEF", HasActivity: false},
		{Address: "1GHI", HasActivity: true},
	}

	filtered := FilterUsage(addresses, false, true)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "1DEF", filtered[0].Address)
}

func TestFilterUsage_NoFilter(t *testing.T) {
	t.Parallel()

	addresses := []AddressInfo{
		{Address: "1ABC", HasActivity: true},
		{Address: "1DEF", HasActivity: false},
	}

	filtered := FilterUsage(addresses, false, false)
	assert.Len(t, filtered, 2)
}

// Mock implementation for testing
type mockMetadataProvider struct {
	metadata map[string]*AddressMetadata
}

func (m *mockMetadataProvider) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	key := string(chainID) + ":" + address
	return m.metadata[key]
}
