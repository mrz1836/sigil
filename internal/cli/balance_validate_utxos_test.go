package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// TestValidateBSVUTXOs_CorruptStore exercises the fail-fast branch of validateBSVUTXOs:
// a malformed utxos.json makes the store Load() fail before any BSV client or network
// call is created.
func TestValidateBSVUTXOs_CorruptStore(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	walletDir := filepath.Join(home, "wallets", "corruptwallet")
	require.NoError(t, os.MkdirAll(walletDir, 0o750))
	// Invalid JSON forces utxostore.Store.Load to return a parse error.
	require.NoError(t, os.WriteFile(filepath.Join(walletDir, "utxos.json"), []byte("{ not valid json"), 0o600))

	cmdCtx := &CommandContext{ //nolint:govet // local variable, not shadowing
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: output.FormatText},
	}

	var buf bytes.Buffer
	err := validateBSVUTXOs(context.Background(), cmdCtx, "corruptwallet", "main", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading UTXO store")
}
