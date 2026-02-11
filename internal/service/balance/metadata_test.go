package balance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// TestNewMetadataAdapter tests adapter creation.
func TestNewMetadataAdapter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.store)
}

// TestMetadataAdapter_GetAddress_Exists tests retrieving existing address metadata.
func TestMetadataAdapter_GetAddress_Exists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	// Add metadata to store
	now := time.Now()
	storeMeta := &utxostore.AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1ABC123",
		HasActivity: true,
		LastScanned: now,
		Label:       "Test Wallet",
	}
	store.AddAddress(storeMeta)

	// Get via adapter
	result := adapter.GetAddress(chain.BSV, "1ABC123")

	require.NotNil(t, result)
	assert.Equal(t, chain.BSV, result.ChainID)
	assert.Equal(t, "1ABC123", result.Address)
	assert.True(t, result.HasActivity)
	assert.Equal(t, now.Unix(), result.LastScanned.Unix()) // Compare Unix timestamps
}

// TestMetadataAdapter_GetAddress_NotFound tests retrieving non-existent metadata.
func TestMetadataAdapter_GetAddress_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	// Try to get non-existent address
	result := adapter.GetAddress(chain.BSV, "1NOTFOUND")

	assert.Nil(t, result)
}

// TestMetadataAdapter_GetAddress_InactiveAddress tests address with no activity.
func TestMetadataAdapter_GetAddress_InactiveAddress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	// Add inactive address
	storeMeta := &utxostore.AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1EMPTY",
		HasActivity: false,
		LastScanned: time.Now(),
	}
	store.AddAddress(storeMeta)

	// Get via adapter
	result := adapter.GetAddress(chain.BSV, "1EMPTY")

	require.NotNil(t, result)
	assert.Equal(t, "1EMPTY", result.Address)
	assert.False(t, result.HasActivity)
}

// TestMetadataAdapter_GetAddress_DifferentChains tests metadata for different chains.
func TestMetadataAdapter_GetAddress_DifferentChains(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	// Add BSV metadata
	bsvMeta := &utxostore.AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1BSV",
		HasActivity: true,
		LastScanned: time.Now(),
	}
	store.AddAddress(bsvMeta)

	// Add ETH metadata
	ethMeta := &utxostore.AddressMetadata{
		ChainID:     chain.ETH,
		Address:     "0xETH",
		HasActivity: true,
		LastScanned: time.Now(),
	}
	store.AddAddress(ethMeta)

	// Get BSV
	bsvResult := adapter.GetAddress(chain.BSV, "1BSV")
	require.NotNil(t, bsvResult)
	assert.Equal(t, chain.BSV, bsvResult.ChainID)
	assert.Equal(t, "1BSV", bsvResult.Address)

	// Get ETH
	ethResult := adapter.GetAddress(chain.ETH, "0xETH")
	require.NotNil(t, ethResult)
	assert.Equal(t, chain.ETH, ethResult.ChainID)
	assert.Equal(t, "0xETH", ethResult.Address)

	// Verify isolation - BSV address should not be found under ETH chain
	wrongChainResult := adapter.GetAddress(chain.ETH, "1BSV")
	assert.Nil(t, wrongChainResult, "should not find BSV address under ETH chain")
}

// TestMetadataAdapter_GetAddress_TypeConversion tests proper type conversion.
func TestMetadataAdapter_GetAddress_TypeConversion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewMetadataAdapter(store)

	// Add metadata with all fields
	now := time.Now()
	storeMeta := &utxostore.AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1FULL",
		HasActivity: true,
		LastScanned: now,
		Label:       "Full Metadata",
	}
	store.AddAddress(storeMeta)

	// Get and verify all fields converted
	result := adapter.GetAddress(chain.BSV, "1FULL")

	require.NotNil(t, result)
	assert.Equal(t, chain.BSV, result.ChainID)
	assert.Equal(t, "1FULL", result.Address)
	assert.True(t, result.HasActivity)
	assert.Equal(t, now.Unix(), result.LastScanned.Unix())
	// Label is not converted (not part of service AddressMetadata type)
}
