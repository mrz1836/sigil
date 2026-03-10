package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mrz1836/sigil/internal/addresslookup"
	"github.com/mrz1836/sigil/internal/discovery"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// Sentinel errors for lookup flag validation.
var (
	errFlagsMutuallyExclusive = errors.New("--input and --keys-file are mutually exclusive")
	errFlagInputRequired      = errors.New("one of --input or --keys-file is required")
	errWorkersMin             = errors.New("--workers must be at least 1")
	errGapRange               = errors.New("--gap must be between 1 and 10000")
	errUnknownScheme          = errors.New("unknown scheme")
)

// lookupCmd flags
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level state
var (
	lookupInput      string
	lookupKeysFile   string
	lookupFormat     string
	lookupPassphrase bool
	lookupFile       string
	lookupGap        int
	lookupScheme     string
	lookupWorkers    int
)

// lookupCmd is the top-level lookup command.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var lookupCmd = &cobra.Command{
	Use:     "lookup",
	GroupID: "utility",
	Short:   "Derive addresses from keys and search address dataset",
	Long: `Derive addresses from private keys (WIF, hex) or mnemonic phrases and search
a large address dataset for matches.

Supports multiple address formats per key:
  BTC: P2PKH (1...), P2SH-P2WPKH (3...), Bech32 (bc1q...)
  LTC: P2PKH (L...), P2SH (M...), Bech32 (ltc1q...)
  BCH: CashAddr (q..., p...)
  DOGE: P2PKH (D...)

Two modes:
  Batch mode (--keys-file): Stream through a file of keys, one per line.
  Single mode (--input): Look up a single key.

The --file flag accepts a single file or a directory. When given a directory,
all .tsv/.csv/.txt/.gz files are loaded recursively into a unified set.

The address dataset is loaded once into memory as a hash map for
O(1) lookup per address. Keys are processed in parallel.`,
	Example: `  # Single key lookup
  sigil lookup --input "5HueCGU8rMjxEXxiPuD5BDku..." --file addresses.tsv

  # Batch mode with a keys file
  sigil lookup --keys-file ~/old_keys.txt --file addresses.tsv

  # Load all chain files from a directory
  sigil lookup --input "5HueCGU8rMjxEXxiPuD5BDku..." --file ./lost_addresses/

  # Batch with explicit format and custom gap limit
  sigil lookup --keys-file ~/mnemonics.txt --format mnemonic --gap 50

  # JSON output for scripting
  sigil lookup --keys-file keys.txt --file addresses.tsv -o json`,
	RunE: runLookup,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for flag registration
func init() {
	lookupCmd.Flags().StringVar(&lookupInput, "input", "", "single key (mnemonic, WIF, or hex)")
	lookupCmd.Flags().StringVar(&lookupKeysFile, "keys-file", "", "path to file of keys (one per line)")
	lookupCmd.Flags().StringVar(&lookupFormat, "format", "auto", "key format: auto, wif, mnemonic, hex")
	lookupCmd.Flags().BoolVar(&lookupPassphrase, "passphrase", false, "prompt for BIP39 passphrase (single mode)")
	lookupCmd.Flags().StringVar(&lookupFile, "file", "", "path to address data file or directory (required)")
	lookupCmd.Flags().IntVar(&lookupGap, "gap", 20, "gap limit per derivation path (for mnemonics)")
	lookupCmd.Flags().StringVar(&lookupScheme, "scheme", "", "scan only a specific scheme (e.g., \"BSV Standard\")")
	lookupCmd.Flags().IntVar(&lookupWorkers, "workers", runtime.NumCPU(), "number of parallel workers for batch mode")

	if err := lookupCmd.MarkFlagRequired("file"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(lookupCmd)
}

// lookupMatch represents a matched address found in the dataset.
type lookupMatch struct {
	Address string `json:"address"`
	Balance string `json:"balance,omitempty"`
	Format  string `json:"format,omitempty"`
	KeyLine int    `json:"key_line"`
	Scheme  string `json:"scheme,omitempty"`
	Path    string `json:"path,omitempty"`
}

// lookupOutput is the JSON output structure.
type lookupOutput struct {
	Results []lookupMatch `json:"results"`
	Stats   lookupStats   `json:"stats"`
}

type lookupStats struct {
	KeysProcessed int64  `json:"keys_processed"`
	MatchesFound  int    `json:"matches_found"`
	Duration      string `json:"duration"`
	AddressCount  int    `json:"address_count"`
}

// keyJob represents a single key to process.
type keyJob struct {
	line  int
	input string
}

func validateLookupFlags() error {
	if lookupInput != "" && lookupKeysFile != "" {
		return errFlagsMutuallyExclusive
	}
	if lookupInput == "" && lookupKeysFile == "" {
		return errFlagInputRequired
	}
	if lookupWorkers <= 0 {
		return errWorkersMin
	}
	if lookupGap <= 0 || lookupGap > 10000 {
		return errGapRange
	}
	return nil
}

func runLookup(cmd *cobra.Command, _ []string) error {
	if err := validateLookupFlags(); err != nil {
		return err
	}

	isJSON := isJSONOutput(cmd)

	// Load address dataset (file or directory)
	addrSet, stats, err := loadAddressData(lookupFile, isJSON)
	if err != nil {
		return err
	}
	if !isJSON {
		_, _ = fmt.Fprintf(os.Stderr, "  Loaded %s addresses in %s (%s)\n",
			formatCount(stats.Count), stats.LoadTime.Round(time.Millisecond), formatBytes(stats.MemBytes))
	}

	// Select derivation schemes
	schemes, err := getSchemes()
	if err != nil {
		return err
	}

	if lookupInput != "" {
		return runSingleLookup(cmd, addrSet, schemes, isJSON)
	}
	return runBatchLookup(cmd, addrSet, schemes, isJSON)
}

func isJSONOutput(cmd *cobra.Command) bool {
	if ctx := GetCmdContext(cmd); ctx != nil {
		if ctx.Fmt != nil && ctx.Fmt.Format() == output.FormatJSON {
			return true
		}
	}
	return false
}

func loadAddressData(path string, isJSON bool) (*addresslookup.AddressSet, addresslookup.Stats, error) {
	if !isJSON {
		_, _ = fmt.Fprintf(os.Stderr, "Loading address data from %s...\n", path)
	}

	var cb addresslookup.LoadProgressCallback
	if !isJSON {
		cb = createLoadProgressCallback()
	}

	fi, statErr := os.Stat(path)
	if statErr != nil {
		return nil, addresslookup.Stats{}, fmt.Errorf("failed to access address file: %w", statErr)
	}
	if fi.IsDir() {
		addrSet, stats, err := addresslookup.LoadDirWithProgress(path, cb)
		if err != nil {
			return nil, stats, fmt.Errorf("failed to load address data: %w", err)
		}
		return addrSet, stats, nil
	}
	addrSet, stats, err := addresslookup.LoadWithProgress(path, cb)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to load address data: %w", err)
	}
	return addrSet, stats, nil
}

func createLoadProgressCallback() addresslookup.LoadProgressCallback {
	return func(p addresslookup.LoadProgress) {
		switch p.Phase {
		case "loading_file":
			_, _ = fmt.Fprintf(os.Stderr, "\r  Loading file %d/%d: %s",
				p.FilesLoaded+1, p.FilesTotal, filepath.Base(p.FileName))
		case "building_index":
			// Clear the loading line and print the building index message
			_, _ = fmt.Fprintf(os.Stderr, "\r%-60s\r", "")
			_, _ = fmt.Fprintf(os.Stderr, "  Building address index (%s entries)...\n",
				formatCount(p.PairsLoaded))
		}
	}
}

func runSingleLookup(cmd *cobra.Command, addrSet *addresslookup.AddressSet, schemes []discovery.PathScheme, isJSON bool) error {
	input := strings.TrimSpace(lookupInput)

	var passphrase []byte
	if lookupPassphrase {
		fmt.Fprint(os.Stderr, "Enter BIP39 passphrase: ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // G115: stdin fd is always a small value
		if err != nil {
			return fmt.Errorf("read passphrase: %w", err)
		}
		passphrase = raw
		defer wallet.ZeroBytes(passphrase)
		fmt.Fprintln(os.Stderr) // newline after hidden input
	}

	start := time.Now()
	matches := deriveAndLookup(input, addrSet, schemes, lookupGap, passphrase)
	duration := time.Since(start)

	if isJSON {
		out := lookupOutput{
			Results: matches,
			Stats: lookupStats{
				KeysProcessed: 1,
				MatchesFound:  len(matches),
				Duration:      duration.Round(time.Millisecond).String(),
				AddressCount:  addrSet.Count(),
			},
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}

	if len(matches) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No matches found.")
	} else {
		for _, m := range matches {
			printMatch(cmd, m)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nFound %d match(es).\n", len(matches))
	}
	return nil
}

//nolint:gocognit,gocyclo // batch processing requires coordinated goroutines
func runBatchLookup(cmd *cobra.Command, addrSet *addresslookup.AddressSet, schemes []discovery.PathScheme, isJSON bool) error {
	// Disable GC during batch processing. The address map is read-only and
	// per-mnemonic allocations are small, so GC pauses are pure overhead.
	runtime.GC()
	prev := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(prev)

	f, err := os.Open(lookupKeysFile) //nolint:gosec // G304: path comes from validated --keys-file flag
	if err != nil {
		return fmt.Errorf("open keys file: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(os.Stderr, "Processing keys from %s (%d workers)...\n", lookupKeysFile, lookupWorkers)

	ctx := cmd.Context()
	start := time.Now()

	// Progress reporting — fires every second to stderr.
	var keysProcessed atomic.Int64
	progressDone := make(chan struct{})
	if !isJSON {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-progressDone:
					return
				case <-ticker.C:
					n := keysProcessed.Load()
					elapsed := time.Since(start).Seconds()
					if elapsed > 0 {
						_, _ = fmt.Fprintf(os.Stderr, "\r  %s keys processed  (%.0f/sec)",
							formatCount(int(n)), float64(n)/elapsed)
					}
				}
			}
		}()
	}
	jobs := make(chan keyJob, lookupWorkers*2)
	results := make(chan lookupMatch, lookupWorkers*2)

	var scanErr atomic.Pointer[error]

	// Producer: stream keys file
	go func() {
		defer close(jobs)
		scanner := bufio.NewScanner(f)
		const maxLineBytes = 1024 // no crypto key line exceeds this
		scanner.Buffer(make([]byte, maxLineBytes), maxLineBytes)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			select {
			case jobs <- keyJob{line: lineNum, input: line}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			scanErr.Store(&err)
		}
	}()

	// Workers: derive and lookup
	var wg sync.WaitGroup
	for i := 0; i < lookupWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				matches := deriveAndLookup(job.input, addrSet, schemes, lookupGap, nil)
				for i := range matches {
					matches[i].KeyLine = job.line
				}
				for _, m := range matches {
					select {
					case results <- m:
					case <-ctx.Done():
						return
					}
				}
				keysProcessed.Add(1)
			}
		}()
	}

	// Close results when all workers done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allMatches []lookupMatch
	for m := range results {
		allMatches = append(allMatches, m)
		if !isJSON {
			printMatch(cmd, m)
		}
	}

	// Stop progress ticker and clear the line
	close(progressDone)
	if !isJSON {
		_, _ = fmt.Fprintf(os.Stderr, "\r%-60s\r", "")
	}

	// Check for scanner errors after all goroutines have completed
	if p := scanErr.Load(); p != nil {
		return fmt.Errorf("reading keys file: %w", *p)
	}

	duration := time.Since(start)

	if isJSON {
		out := lookupOutput{
			Results: allMatches,
			Stats: lookupStats{
				KeysProcessed: keysProcessed.Load(),
				MatchesFound:  len(allMatches),
				Duration:      duration.Round(time.Millisecond).String(),
				AddressCount:  addrSet.Count(),
			},
		}
		return writeJSON(cmd.OutOrStdout(), out)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nDone. Processed %s keys in %s. Found %d match(es).\n",
		formatCount(int(keysProcessed.Load())), duration.Round(time.Millisecond), len(allMatches))
	return nil
}

// deriveAndLookup derives addresses from a key and checks the address set.
func deriveAndLookup(input string, addrSet *addresslookup.AddressSet, schemes []discovery.PathScheme, gap int, passphrase []byte) []lookupMatch {
	format := detectFormat(input)
	if format == wallet.FormatUnknown {
		return nil
	}

	switch format { //nolint:exhaustive // FormatUnknown handled above
	case wallet.FormatWIF:
		return lookupWIF(input, addrSet)
	case wallet.FormatHex:
		return lookupHex(input, addrSet)
	case wallet.FormatMnemonic:
		return lookupMnemonic(input, addrSet, schemes, gap, passphrase)
	default:
		return nil
	}
}

func detectFormat(input string) wallet.InputFormat {
	switch lookupFormat {
	case "wif":
		return wallet.FormatWIF
	case "hex":
		return wallet.FormatHex
	case "mnemonic":
		return wallet.FormatMnemonic
	default:
		return wallet.DetectInputFormat(input)
	}
}

func lookupWIF(input string, addrSet *addresslookup.AddressSet) []lookupMatch {
	privKey, err := wallet.ParseWIF(input)
	if err != nil {
		return nil
	}
	defer wallet.ZeroBytes(privKey)
	return lookupPrivKey(privKey, addrSet)
}

func lookupHex(input string, addrSet *addresslookup.AddressSet) []lookupMatch {
	privKey, err := wallet.ParseHexKey(input)
	if err != nil {
		return nil
	}
	defer wallet.ZeroBytes(privKey)
	return lookupPrivKey(privKey, addrSet)
}

// lookupNetworks defines the networks to check for each private key.
//
//nolint:gochecknoglobals // Static lookup table
var lookupNetworks = []struct {
	label  string
	params wallet.NetworkParams
}{
	{"BTC", wallet.BTCMainnetParams()},
	{"LTC", wallet.LTCMainnetParams()},
	{"BCH", wallet.BCHMainnetParams()},
	{"DOGE", wallet.DOGEMainnetParams()},
}

func lookupPrivKey(privKey []byte, addrSet *addresslookup.AddressSet) []lookupMatch {
	var matches []lookupMatch
	for _, net := range lookupNetworks {
		addrs, err := wallet.AllAddressesFromPrivKey(privKey, net.params)
		if err != nil {
			continue
		}
		for _, addr := range addrs.Addresses() {
			result := addrSet.Lookup(addr)
			if result.Found {
				matches = append(matches, lookupMatch{
					Address: addr,
					Balance: result.Balance,
					Format:  net.label + " " + addrs.FormatLabel(addr),
				})
			}
		}
	}
	return matches
}

//nolint:gocognit // mnemonic derivation requires nested loops across schemes/accounts/chains/indices
func lookupMnemonic(input string, addrSet *addresslookup.AddressSet, schemes []discovery.PathScheme, gap int, passphrase []byte) []lookupMatch {
	seed, err := wallet.MnemonicToSeed(input, string(passphrase))
	if err != nil {
		return nil
	}
	defer wallet.ZeroBytes(seed)

	mc, err := wallet.NewMnemonicContext(seed)
	if err != nil {
		return nil
	}
	defer mc.Zero()

	var matches []lookupMatch

	for _, scheme := range schemes {
		if scheme.IsLegacy {
			// HandCash legacy: m/0'/index — derive all formats
			for i := uint32(0); i < uint32(gap); i++ { //nolint:gosec // gap is validated
				addr, _, path, legacyErr := mc.DeriveLegacyAddress(i)
				if legacyErr != nil {
					continue
				}
				result := addrSet.Lookup(addr)
				if result.Found {
					matches = append(matches, lookupMatch{
						Address: addr,
						Balance: result.Balance,
						Scheme:  scheme.Name,
						Path:    path,
					})
				}
			}
			continue
		}

		netParams := wallet.NetworkParamsForCoinType(scheme.CoinType)
		netLabel := wallet.NetworkLabelForCoinType(scheme.CoinType)

		for _, account := range scheme.Accounts {
			matches = append(matches, lookupMnemonicChain(mc, addrSet, scheme, netParams, netLabel, account, wallet.ExternalChain, gap)...)
			if scheme.ScanChange {
				matches = append(matches, lookupMnemonicChain(mc, addrSet, scheme, netParams, netLabel, account, wallet.InternalChain, gap)...)
			}
		}
	}

	return matches
}

// lookupMnemonicChain derives all address formats for a single chain of a mnemonic scheme.
func lookupMnemonicChain(
	mc *wallet.MnemonicContext,
	addrSet *addresslookup.AddressSet,
	scheme discovery.PathScheme,
	netParams wallet.NetworkParams,
	netLabel string,
	account, change uint32,
	gap int,
) []lookupMatch {
	// DeriveGap uses cached intermediate keys and does one EC op per index.
	gapResults, err := mc.DeriveGap(scheme.CoinType, account, change, gap)
	if err != nil {
		return nil
	}

	var matches []lookupMatch
	for _, r := range gapResults {
		addrs, fmtErr := wallet.AllAddressesFromPubKey(r.PubKey, netParams)
		if fmtErr != nil {
			continue
		}
		// Inline address checks to avoid slice allocation from Addresses().
		// Path() is only called on matches (lazy formatting).
		checkAddr := func(addr, label string) {
			if addr == "" {
				return
			}
			if result := addrSet.Lookup(addr); result.Found {
				matches = append(matches, lookupMatch{
					Address: addr,
					Balance: result.Balance,
					Format:  netLabel + " " + label,
					Scheme:  scheme.Name,
					Path:    r.Path(),
				})
			}
		}
		checkAddr(addrs.P2PKH, "P2PKH")
		checkAddr(addrs.P2SH, "P2SH-P2WPKH")
		checkAddr(addrs.Bech32, "Bech32")
		checkAddr(addrs.CashAddr, "CashAddr")
	}
	return matches
}

func getSchemes() ([]discovery.PathScheme, error) {
	if lookupScheme != "" {
		scheme := discovery.SchemeByName(lookupScheme)
		if scheme == nil {
			return nil, fmt.Errorf("%w %q; available: %s",
				errUnknownScheme, lookupScheme, strings.Join(allSchemeNames(), ", "))
		}
		return []discovery.PathScheme{*scheme}, nil
	}
	return discovery.DefaultSchemes(), nil
}

func allSchemeNames() []string {
	schemes := discovery.DefaultSchemes()
	names := make([]string, len(schemes))
	for i, s := range schemes {
		names[i] = s.Name
	}
	return names
}

func printMatch(cmd *cobra.Command, m lookupMatch) {
	parts := []string{fmt.Sprintf("  MATCH: %s", m.Address)}
	if m.Balance != "" {
		parts = append(parts, fmt.Sprintf("balance=%s", m.Balance))
	}
	if m.Format != "" {
		parts = append(parts, fmt.Sprintf("format=%q", m.Format))
	}
	if m.KeyLine > 0 {
		parts = append(parts, fmt.Sprintf("key_line=%d", m.KeyLine))
	}
	if m.Scheme != "" {
		parts = append(parts, fmt.Sprintf("scheme=%q", m.Scheme))
	}
	if m.Path != "" {
		parts = append(parts, fmt.Sprintf("path=%s", m.Path))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(parts, "  "))
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	default:
		return fmt.Sprintf("%d KB", b/1024)
	}
}

func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c)) //nolint:gosec // G115: c is a digit rune (0-9), safe to convert to byte
	}
	return string(result)
}
