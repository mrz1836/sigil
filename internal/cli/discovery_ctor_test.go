package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// TestCreateDiscoveryService_NonNil verifies the pure constructor wires a balance
// service and UTXO/cache adapters into a non-nil discovery service. No network.
func TestCreateDiscoveryService_NonNil(t *testing.T) {
	t.Parallel()

	cmdCtx := &CommandContext{Cfg: &mockConfigProvider{home: t.TempDir()}} //nolint:govet // local variable, not shadowing
	store := utxostore.New(t.TempDir())
	balanceCache := cache.NewBalanceCache()

	svc := createDiscoveryService(cmdCtx, store, balanceCache)
	require.NotNil(t, svc)

	// A second call with the same inputs yields an independent, non-nil instance.
	svc2 := createDiscoveryService(cmdCtx, store, balanceCache)
	require.NotNil(t, svc2)
	assert.NotSame(t, svc, svc2)
}
