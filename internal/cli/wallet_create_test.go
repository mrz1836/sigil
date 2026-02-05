package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/sigil/internal/utxostore"
)

var (
	errConnectionTimeout = errors.New("connection timeout")
	errRateLimited       = errors.New("rate limited")
)

func TestDisplayScanResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      *utxostore.ScanResult
		contains    []string
		notContains []string
	}{
		{
			name: "no errors, with UTXOs",
			result: &utxostore.ScanResult{
				AddressesScanned: 5,
				UTXOsFound:       3,
				TotalBalance:     150000000, // 1.5 BSV
			},
			contains: []string{
				"Scan Results",
				"5",
				"3",
				"150000000 satoshis",
				"1.50000000 BSV",
			},
		},
		{
			name: "with errors",
			result: &utxostore.ScanResult{
				AddressesScanned: 2,
				UTXOsFound:       0,
				TotalBalance:     0,
				Errors:           []error{errConnectionTimeout, errRateLimited},
			},
			contains: []string{
				"Scan completed with errors",
				"connection timeout",
				"rate limited",
			},
		},
		{
			name: "zero balance",
			result: &utxostore.ScanResult{
				AddressesScanned: 3,
				UTXOsFound:       0,
				TotalBalance:     0,
			},
			contains: []string{
				"0 satoshis",
				"0.00000000 BSV",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			displayScanResults(tc.result, cmd)

			output := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, output, s, "output should contain %q", s)
			}
			for _, s := range tc.notContains {
				assert.NotContains(t, output, s, "output should not contain %q", s)
			}
		})
	}
}
