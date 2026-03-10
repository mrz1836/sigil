package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// Sentinel errors for keygen flag validation.
var (
	errKeygenUnknownFormat = errors.New("unknown format")
	errKeygenCountMin      = errors.New("--count must be at least 1")
	errKeygenWorkersMin    = errors.New("--workers must be at least 1")
)

// keygen command flags
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level state
var (
	keygenFormat  string
	keygenCount   int
	keygenFile    string
	keygenWorkers int
)

// keygenCmd is the keygen subcommand.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var keygenCmd = &cobra.Command{
	Use:     "keygen",
	GroupID: "utility",
	Short:   "Generate a file of private keys",
	Long: `Generate a text file of cryptographic private keys in the chosen format.

One key is written per line. The output is compatible with sigil lookup
--keys-file for batch address derivation and lookups.

Supported formats:
  hex              Raw 32-byte private key, lowercase hex
  wif              Compressed WIF (K... or L... prefix)
  wif-uncompressed Uncompressed WIF (5... prefix)
  mnemonic12       BIP39 12-word mnemonic phrase
  mnemonic24       BIP39 24-word mnemonic phrase`,
	Example: `  # Generate 1000 compressed WIF keys
  sigil keygen --format wif --count 1000 --file keys.txt

  # Generate 50000 hex keys using 8 workers
  sigil keygen --format hex --count 50000 --file hex_keys.txt --workers 8

  # Generate BIP39 12-word mnemonics
  sigil keygen --format mnemonic12 --count 500 --file mnemonics.txt`,
	RunE: runKeygen,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for flag registration
func init() {
	keygenCmd.Flags().StringVar(&keygenFormat, "format", "", "key format: hex, wif, wif-uncompressed, mnemonic12, mnemonic24 (required)")
	keygenCmd.Flags().IntVar(&keygenCount, "count", 0, "number of keys to generate (required)")
	keygenCmd.Flags().StringVar(&keygenFile, "file", "", "output file path (required)")
	keygenCmd.Flags().IntVar(&keygenWorkers, "workers", runtime.NumCPU(), "number of parallel generation goroutines")

	if err := keygenCmd.MarkFlagRequired("format"); err != nil {
		panic(err)
	}
	if err := keygenCmd.MarkFlagRequired("count"); err != nil {
		panic(err)
	}
	if err := keygenCmd.MarkFlagRequired("file"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(keygenCmd)
}

func validateKeygenFlags() error {
	switch keygenFormat {
	case "hex", "wif", "wif-uncompressed", "mnemonic12", "mnemonic24":
	default:
		return fmt.Errorf("%w %q; supported: hex, wif, wif-uncompressed, mnemonic12, mnemonic24", errKeygenUnknownFormat, keygenFormat)
	}
	if keygenCount < 1 {
		return errKeygenCountMin
	}
	if keygenWorkers < 1 {
		return errKeygenWorkersMin
	}
	return nil
}

// generateOneKey creates a single key string in the requested format.
func generateOneKey(format string) (string, error) {
	switch format {
	case "hex":
		privKey := make([]byte, 32)
		defer wallet.ZeroBytes(privKey)
		if _, err := io.ReadFull(rand.Reader, privKey); err != nil {
			return "", err
		}
		return hex.EncodeToString(privKey), nil

	case "wif":
		privKey := make([]byte, 32)
		defer wallet.ZeroBytes(privKey)
		if _, err := io.ReadFull(rand.Reader, privKey); err != nil {
			return "", err
		}
		// Compressed WIF: version 0x80 + 32 bytes + compression flag 0x01
		payload := make([]byte, 33)
		copy(payload, privKey)
		payload[32] = 0x01
		return bitcoin.Base58CheckEncode(0x80, payload), nil

	case "wif-uncompressed":
		privKey := make([]byte, 32)
		defer wallet.ZeroBytes(privKey)
		if _, err := io.ReadFull(rand.Reader, privKey); err != nil {
			return "", err
		}
		// Uncompressed WIF: version 0x80 + 32 bytes (no compression flag)
		return bitcoin.Base58CheckEncode(0x80, privKey), nil

	case "mnemonic12":
		return wallet.GenerateMnemonic(12)

	case "mnemonic24":
		return wallet.GenerateMnemonic(24)

	default:
		return "", fmt.Errorf("%w %q", errKeygenUnknownFormat, format)
	}
}

//nolint:gocognit,gocyclo // concurrent key generation with progress reporting requires coordinated goroutines
func runKeygen(cmd *cobra.Command, _ []string) (retErr error) {
	if err := validateKeygenFlags(); err != nil {
		return err
	}

	f, err := os.Create(keygenFile) //nolint:gosec // G304: path comes from --file flag
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("close output file: %w", err)
		}
	}()

	results := make(chan string, keygenWorkers*4)

	var genErr atomic.Pointer[error]
	var remaining atomic.Int64
	remaining.Store(int64(keygenCount))

	// Worker pool: each worker atomically claims keys to generate.
	var wg sync.WaitGroup
	for i := 0; i < keygenWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if remaining.Add(-1) < 0 {
					return
				}
				key, genKeyErr := generateOneKey(keygenFormat)
				if genKeyErr != nil {
					genErr.Store(&genKeyErr)
					return
				}
				results <- key
			}
		}()
	}

	// Close results once all workers finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Progress reporting — fires every second to stderr.
	var written atomic.Int64
	start := time.Now()
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				n := written.Load()
				elapsed := time.Since(start).Seconds()
				if elapsed > 0 {
					_, _ = fmt.Fprintf(os.Stderr, "\r  %s / %s keys  (%.0f/sec)",
						formatCount(int(n)), formatCount(keygenCount), float64(n)/elapsed)
				}
			}
		}
	}()

	// Writer: drain results into a buffered file writer.
	bw := bufio.NewWriterSize(f, 4<<20)
	for key := range results {
		_, _ = bw.WriteString(key)
		_ = bw.WriteByte('\n')
		written.Add(1)
	}

	// Stop progress goroutine before printing the success line.
	close(progressDone)

	if flushErr := bw.Flush(); flushErr != nil {
		return fmt.Errorf("flush output: %w", flushErr)
	}

	if p := genErr.Load(); p != nil {
		return fmt.Errorf("generating keys: %w", *p)
	}

	absPath, absErr := filepath.Abs(keygenFile)
	if absErr != nil {
		absPath = keygenFile
	}

	_, _ = fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 60))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Generated %s keys → %s\n", formatCount(keygenCount), absPath)
	return nil
}
