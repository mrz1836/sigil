package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// TestRunBalanceShow_CachedOnly drives runBalanceShow in --cached mode, which
// serves data from the on-disk cache without any network call. It mutates
// package-level balance flags, so it is not parallel.
func TestRunBalanceShow_CachedOnly(t *testing.T) {
	origWallet, origChain := balanceWalletName, balanceChainFilter
	origCached, origAsync, origRefresh, origValidate := balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate
	defer func() {
		balanceWalletName, balanceChainFilter = origWallet, origChain
		balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate = origCached, origAsync, origRefresh, origValidate
	}()

	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // local variable, not shadowing
	t.Cleanup(cleanup)
	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	// Read the wallet's BSV address from public metadata (no password needed).
	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	meta, err := storage.LoadMetadata("test-wallet")
	require.NoError(t, err)
	require.NotEmpty(t, meta.Addresses[chain.BSV])
	bsvAddr := meta.Addresses[chain.BSV][0].Address

	// Seed the balance cache on disk with an entry for that address.
	bc := cache.NewBalanceCache()
	bc.Set(cache.BalanceCacheEntry{
		Chain:     chain.BSV,
		Address:   bsvAddr,
		Balance:   "1.23456789",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now(),
	})
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cache"), 0o750))
	cacheStorage := cache.NewFileStorage(filepath.Join(tmpDir, "cache", "balances.json"))
	require.NoError(t, cacheStorage.Save(bc))

	balanceWalletName, balanceChainFilter = "test-wallet", "bsv"
	balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate = true, false, false, false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	require.NoError(t, runBalanceShow(cmd, nil))

	out := buf.String()
	assert.Contains(t, out, "Balances for wallet: test-wallet")
	assert.Contains(t, out, "1.23456789")
	assert.Contains(t, out, "BSV")
}

// TestRunBalanceShow_CachedNoData covers the --cached branch where no cache
// entries exist for the wallet, which returns ErrNoCachedData. Mutates
// package-level balance flags, so it is not parallel.
func TestRunBalanceShow_CachedNoData(t *testing.T) {
	origWallet, origChain := balanceWalletName, balanceChainFilter
	origCached, origAsync, origRefresh, origValidate := balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate
	defer func() {
		balanceWalletName, balanceChainFilter = origWallet, origChain
		balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate = origCached, origAsync, origRefresh, origValidate
	}()

	tmpDir, cmdCtx, cleanup := setupAgentTest(t) //nolint:govet // local variable, not shadowing
	t.Cleanup(cleanup)
	createTestWalletForAgent(t, tmpDir)
	withMockPrompts(t, []byte("testpass123"), true)

	balanceWalletName, balanceChainFilter = "test-wallet", "bsv"
	balanceCachedOnly, balanceAsync, balanceRefresh, balanceValidate = true, false, false, false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, cmdCtx)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// No cache seeded => no cached balances => ErrNoCachedData.
	err := runBalanceShow(cmd, nil)
	require.ErrorIs(t, err, ErrNoCachedData)
}
