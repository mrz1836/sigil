package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/service/balance"
)

func TestCreateBalanceProgressCallback(t *testing.T) {
	t.Parallel()

	t.Run("building phase is silent", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		cb(balance.ProgressUpdate{Phase: "building", TotalAddresses: 5})
		assert.Empty(t, buf.String(), "building phase produces no output")
	})

	t.Run("bsv start and completion in one update", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		// CompletedAddresses == TotalAddresses (>0) triggers both the start line
		// (phase not yet started) and the completion line.
		cb(balance.ProgressUpdate{Phase: "fetching_bsv", TotalAddresses: 2, CompletedAddresses: 2})
		out := buf.String()
		assert.Contains(t, out, "Fetching BSV balances (2 addresses)...")
		assert.Contains(t, out, "✓ BSV complete")
	})

	t.Run("btc start and completion", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		cb(balance.ProgressUpdate{Phase: "fetching_btc", TotalAddresses: 4, CompletedAddresses: 4})
		out := buf.String()
		assert.Contains(t, out, "Fetching BTC balances (4 addresses)...")
		assert.Contains(t, out, "✓ BTC complete")
	})

	t.Run("eth reports progress at multiples of five", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		cb(balance.ProgressUpdate{Phase: "fetching_eth", TotalAddresses: 10, CompletedAddresses: 5, Message: "p1"})
		out := buf.String()
		assert.Contains(t, out, "Fetching ETH balances (10 addresses)...")
		assert.Contains(t, out, "5/10 addresses completed")
	})

	t.Run("eth reports progress at completion even when not a multiple of five", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		// 7 is not a multiple of 5, but equals the total, so it still reports.
		cb(balance.ProgressUpdate{Phase: "fetching_eth", TotalAddresses: 7, CompletedAddresses: 7})
		assert.Contains(t, buf.String(), "7/7 addresses completed")
	})

	t.Run("duplicate consecutive updates are suppressed", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cb := createBalanceProgressCallback(&buf)
		update := balance.ProgressUpdate{Phase: "fetching_bsv", TotalAddresses: 3, CompletedAddresses: 0, Message: "same"}
		cb(update)
		cb(update) // identical Phase+Message => deduplicated, no extra output
		count := bytes.Count(buf.Bytes(), []byte("Fetching BSV balances"))
		assert.Equal(t, 1, count, "start line printed exactly once")
	})
}
