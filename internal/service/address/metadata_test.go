package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

func TestNewMetadataAdapter(t *testing.T) {
	t.Parallel()

	store := utxostore.New(t.TempDir())
	adapter := NewMetadataAdapter(store)
	require.NotNil(t, adapter)

	// A freshly constructed adapter over an empty store returns nil for any
	// address, since no metadata has been recorded yet.
	assert.Nil(t, adapter.GetAddress(chain.BSV, "1ABC"))
}

func TestMetadataAdapter_GetAddress_NilStore(t *testing.T) {
	t.Parallel()

	// Adapter wrapping a nil store must not panic and must return nil.
	adapter := NewMetadataAdapter(nil)
	require.NotNil(t, adapter)
	assert.Nil(t, adapter.GetAddress(chain.BSV, "1ABC"))
}

func TestMetadataAdapter_GetAddress_NotFound(t *testing.T) {
	t.Parallel()

	store := utxostore.New(t.TempDir())
	store.AddAddress(&utxostore.AddressMetadata{
		Address:     "1ABC",
		ChainID:     chain.BSV,
		HasActivity: true,
		Label:       "Savings",
	})

	adapter := NewMetadataAdapter(store)

	// Different address on the same chain -> nil.
	assert.Nil(t, adapter.GetAddress(chain.BSV, "1UNKNOWN"))
	// Same address string but a different chain -> nil, since the store key
	// is scoped by chain ID.
	assert.Nil(t, adapter.GetAddress(chain.ETH, "1ABC"))
}

func TestMetadataAdapter_GetAddress_Found(t *testing.T) {
	t.Parallel()

	store := utxostore.New(t.TempDir())
	store.AddAddress(&utxostore.AddressMetadata{
		Address:     "1ACTIVE",
		ChainID:     chain.BSV,
		HasActivity: true,
		Label:       "Savings",
	})
	store.AddAddress(&utxostore.AddressMetadata{
		Address:     "1FRESH",
		ChainID:     chain.BSV,
		HasActivity: false,
		Label:       "",
	})

	adapter := NewMetadataAdapter(store)

	tests := []struct {
		name            string
		address         string
		wantHasActivity bool
		wantLabel       string
	}{
		// Adapter copies HasActivity/Label from the store metadata onto the
		// service-level AddressMetadata type.
		{"active labeled address", "1ACTIVE", true, "Savings"},
		{"fresh unlabeled address", "1FRESH", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := adapter.GetAddress(chain.BSV, tt.address)
			require.NotNil(t, meta)
			assert.Equal(t, tt.wantHasActivity, meta.HasActivity)
			assert.Equal(t, tt.wantLabel, meta.Label)
		})
	}
}
