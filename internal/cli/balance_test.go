package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/wallet"
)

// Test error variables for err113 compliance
var (
	errTestError = fmt.Errorf("test error")
	errReadError = fmt.Errorf("read error")
)

// mockConfigProvider implements ConfigProvider for testing.
type mockConfigProvider struct {
	home               string
	ethRPC             string
	fallbackRPCs       []string
	ethProvider        string
	ethEtherscanAPIKey string
	bsvAPIKey          string
	bsvBroadcast       string
	bsvFeeStrategy     string
	bsvMinMiners       int
	logLevel           string
	logFile            string
	outputFormat       string
	verbose            bool
	security           config.SecurityConfig
}

func (m *mockConfigProvider) GetHome() string                    { return m.home }
func (m *mockConfigProvider) GetETHRPC() string                  { return m.ethRPC }
func (m *mockConfigProvider) GetETHFallbackRPCs() []string       { return m.fallbackRPCs }
func (m *mockConfigProvider) GetBSVAPIKey() string               { return m.bsvAPIKey }
func (m *mockConfigProvider) GetBSVBroadcast() string            { return m.bsvBroadcast }
func (m *mockConfigProvider) GetLoggingLevel() string            { return m.logLevel }
func (m *mockConfigProvider) GetLoggingFile() string             { return m.logFile }
func (m *mockConfigProvider) GetOutputFormat() string            { return m.outputFormat }
func (m *mockConfigProvider) IsVerbose() bool                    { return m.verbose }
func (m *mockConfigProvider) GetSecurity() config.SecurityConfig { return m.security }

func (m *mockConfigProvider) GetETHProvider() string {
	if m.ethProvider == "" {
		return "etherscan"
	}
	return m.ethProvider
}

func (m *mockConfigProvider) GetETHEtherscanAPIKey() string {
	return m.ethEtherscanAPIKey
}

func (m *mockConfigProvider) GetBSVFeeStrategy() string {
	if m.bsvFeeStrategy == "" {
		return "normal"
	}
	return m.bsvFeeStrategy
}

func (m *mockConfigProvider) GetBSVMinMiners() int {
	if m.bsvMinMiners == 0 {
		return 3
	}
	return m.bsvMinMiners
}

func TestFormatCacheAge(t *testing.T) {
	tests := []struct {
		name     string
		age      time.Duration
		expected string
	}{
		{
			name:     "seconds ago",
			age:      30 * time.Second,
			expected: "30s ago",
		},
		{
			name:     "minutes ago",
			age:      5 * time.Minute,
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			age:      3 * time.Hour,
			expected: "3h ago",
		},
		{
			name:     "days ago",
			age:      48 * time.Hour,
			expected: "2d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp := time.Now().Add(-tt.age)
			result := formatCacheAge(timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputBalanceText(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
			{
				Chain:    "bsv",
				Address:  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				Balance:  "0.1",
				Symbol:   "BSV",
				Decimals: 8,
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "ETH")
	assert.Contains(t, output, "BSV")
	assert.Contains(t, output, "1.5")
	assert.Contains(t, output, "0.1")
}

func TestOutputBalanceJSON(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "test", parsed.Wallet)
	assert.Equal(t, "eth", parsed.Balances[0].Chain)
	assert.Equal(t, "1.5", parsed.Balances[0].Balance)
	assert.Equal(t, "ETH", parsed.Balances[0].Symbol)
}

func TestOutputBalanceTextWithUnconfirmed(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:       "bsv",
				Address:     "16DwKi833rr1PQfZw65LnHeagj1iLcVUbT",
				Balance:     "0.00070422",
				Unconfirmed: "-0.00070422",
				Symbol:      "BSV",
				Decimals:    8,
			},
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	// Should use wide table with Confirmed/Unconfirmed headers
	assert.Contains(t, output, "Confirmed")
	assert.Contains(t, output, "Unconfirmed")
	assert.Contains(t, output, "0.00070422")
	assert.Contains(t, output, "-0.00070422")
	// ETH entry without unconfirmed should show "-"
	assert.Contains(t, output, "-")
}

func TestOutputBalanceTextNoUnconfirmed(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	// Should use narrow table without Unconfirmed header
	assert.Contains(t, output, "Balance")
	assert.NotContains(t, output, "Confirmed")
	assert.NotContains(t, output, "Unconfirmed")
}

func TestOutputBalanceJSONWithUnconfirmed(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:       "bsv",
				Address:     "16DwKi833rr1PQfZw65LnHeagj1iLcVUbT",
				Balance:     "0.00070422",
				Unconfirmed: "-0.00070422",
				Symbol:      "BSV",
				Decimals:    8,
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "0.00070422", parsed.Balances[0].Balance)
	assert.Equal(t, "-0.00070422", parsed.Balances[0].Unconfirmed)
}

func TestOutputBalanceTextWithStaleData(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "Some balances could not be fetched",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
				Stale:    true,
				CacheAge: "5m ago",
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	assert.Contains(t, output, "Warning")
	assert.Contains(t, output, "1.5 *")
	assert.Contains(t, output, "Cached data")
}

func TestBalanceCacheIntegration(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "balance-cli-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)

	// Create and save cache
	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.0",
		Symbol:   "ETH",
		Decimals: 18,
	})

	err = cacheStorage.Save(balanceCache)
	require.NoError(t, err)

	// Load and verify
	loaded, err := cacheStorage.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Size())

	entry, exists, _ := loaded.Get(chain.ETH, "0x123", "")
	require.True(t, exists)
	assert.Equal(t, "1.0", entry.Balance)
}

func TestGetChainSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chainID  wallet.ChainID
		expected string
	}{
		{
			name:     "ETH chain",
			chainID:  wallet.ChainETH,
			expected: "ETH",
		},
		{
			name:     "BSV chain",
			chainID:  wallet.ChainBSV,
			expected: "BSV",
		},
		{
			name:     "BTC chain",
			chainID:  wallet.ChainBTC,
			expected: "BTC",
		},
		{
			name:     "BCH chain",
			chainID:  wallet.ChainBCH,
			expected: "BCH",
		},
		{
			name:     "unknown chain",
			chainID:  wallet.ChainID("unknown"),
			expected: "???",
		},
		{
			name:     "empty chain",
			chainID:  wallet.ChainID(""),
			expected: "???",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := getChainSymbol(tc.chainID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOutputBalanceJSON_TokenAndStale(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "Some balances could not be fetched",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x123",
				Balance:  "100.0",
				Symbol:   "USDC",
				Token:    "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
				Decimals: 6,
				Stale:    true,
				CacheAge: "5m ago",
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "Some balances could not be fetched", parsed.Warning)
	assert.Equal(t, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", parsed.Balances[0].Token)
	assert.True(t, parsed.Balances[0].Stale)
	assert.Equal(t, "5m ago", parsed.Balances[0].CacheAge)
}

func TestOutputBalanceJSON_Escaping(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test\"wallet",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "line1\nline2 \"quoted\" \u2713",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0xabc\"123",
				Balance:  "1.0",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, response.Wallet, parsed.Wallet)
	assert.Equal(t, response.Warning, parsed.Warning)
	assert.Equal(t, response.Balances[0].Address, parsed.Balances[0].Address)
}

func TestOutputBalanceText_Empty(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "empty",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances:  nil,
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)
	out := buf.String()

	assert.Contains(t, out, "No balances found")
}

func TestTruncateAddress(t *testing.T) {
	t.Parallel()

	t.Run("exactly 42 chars returned unchanged", func(t *testing.T) {
		t.Parallel()
		// 42 characters exactly
		addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
		assert.Len(t, addr, 42)
		assert.Equal(t, addr, truncateAddress(addr))
	})

	t.Run("longer than 42 chars truncated", func(t *testing.T) {
		t.Parallel()
		addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44eABCDEF"
		assert.Greater(t, len(addr), 42)
		result := truncateAddress(addr)
		assert.Equal(t, addr[:20], result[:20])
		assert.Contains(t, result, "...")
		assert.Equal(t, addr[len(addr)-17:], result[len(result)-17:])
		assert.Less(t, len(result), len(addr))
	})

	t.Run("short address returned unchanged", func(t *testing.T) {
		t.Parallel()
		addr := "1abc"
		assert.Equal(t, addr, truncateAddress(addr))
	})
}

func TestOutputBalanceJSON_NilBalancesSlice(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances:  nil, // nil slice
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	// Verify balances is [] (empty array) not null
	balances, ok := parsed["balances"].([]any)
	require.True(t, ok, "balances should be an array, not null")
	assert.Empty(t, balances)
}

// TODO: Add service-level tests in internal/service/balance/service_test.go
// The tests below were for internal implementation details that have been
// moved to the service layer. They should be re-implemented as service tests.
//
// Tests to port to service package:
// - TestGetCachedETHBalances*
// - TestGetCachedBSVBalances*
// - TestConnectETHClient*
// - TestFetchETHBalances*
// - TestFetchBSVBalances*
// - TestFetchBalancesForAddress*

func TestSortBalanceResults(t *testing.T) {
	t.Parallel()

	input := []BalanceResult{
		{Chain: "eth", Address: "0xb", Token: "t2"},
		{Chain: "eth", Address: "0xa", Token: "t1"},
		{Chain: "bsv", Address: "1a"},
		{Chain: "eth", Address: "0xb", Token: "t1"},
	}

	sortBalanceResults(input)

	// Expected order:
	// 1. bsv, 1a (chain bsv < eth)
	// 2. eth, 0xa, t1 (address 0xa < 0xb)
	// 3. eth, 0xb, t1 (token t1 < t2)
	// 4. eth, 0xb, t2

	assert.Equal(t, "bsv", input[0].Chain)
	assert.Equal(t, "1a", input[0].Address)

	assert.Equal(t, "eth", input[1].Chain)
	assert.Equal(t, "0xa", input[1].Address)

	assert.Equal(t, "eth", input[2].Chain)
	assert.Equal(t, "0xb", input[2].Address)
	assert.Equal(t, "t1", input[2].Token)

	assert.Equal(t, "eth", input[3].Chain)
	assert.Equal(t, "0xb", input[3].Address)
	assert.Equal(t, "t2", input[3].Token)
}

func TestBuildAddressList(t *testing.T) {
	t.Parallel()

	w := &wallet.Wallet{
		Addresses: map[wallet.ChainID][]wallet.Address{
			wallet.ChainETH: {
				{Address: "0x1"},
				{Address: "0x2"},
			},
			wallet.ChainBSV: {
				{Address: "1a"},
			},
		},
	}

	// Test no filter
	list := buildAddressList(w, "")
	assert.Len(t, list, 3)
	// Order is not guaranteed due to map iteration, so we check existence
	foundETH := 0
	foundBSV := 0
	for _, a := range list {
		switch a.ChainID {
		case wallet.ChainETH:
			foundETH++
		case wallet.ChainBSV:
			foundBSV++
		case wallet.ChainBTC, wallet.ChainBCH:
			// Other chains not counted in this test
		}
	}
	assert.Equal(t, 2, foundETH)
	assert.Equal(t, 1, foundBSV)

	// Test filter ETH
	listETH := buildAddressList(w, "eth")
	assert.Len(t, listETH, 2)
	for _, a := range listETH {
		assert.Equal(t, wallet.ChainETH, a.ChainID)
	}

	// Test filter BSV
	listBSV := buildAddressList(w, "bsv")
	assert.Len(t, listBSV, 1)
	assert.Equal(t, wallet.ChainBSV, listBSV[0].ChainID)
	assert.Equal(t, "1a", listBSV[0].Address)
}

func TestConvertToBalanceResponse(t *testing.T) {
	t.Parallel()

	batchResult := &balance.FetchBatchResult{
		Results: []*balance.FetchResult{
			{
				ChainID: "eth",
				Address: "0x1",
				Balances: []balance.BalanceEntry{
					{
						Chain:     wallet.ChainETH,
						Address:   "0x1",
						Balance:   "1.0",
						Symbol:    "ETH",
						UpdatedAt: time.Now(),
					},
				},
			},
			{
				ChainID: "bsv",
				Address: "1a",
				Balances: []balance.BalanceEntry{
					{
						Chain:     wallet.ChainBSV,
						Address:   "1a",
						Balance:   "0.5",
						Symbol:    "BSV",
						Stale:     true,
						UpdatedAt: time.Now().Add(-10 * time.Minute),
					},
				},
			},
		},
		Errors: []error{},
	}

	resp := convertToBalanceResponse("testwallet", batchResult)

	assert.Equal(t, "testwallet", resp.Wallet)
	assert.Len(t, resp.Balances, 2)
	assert.Empty(t, resp.Warning)

	// Check sorting (BSV first)
	assert.Equal(t, "bsv", resp.Balances[0].Chain)
	assert.Equal(t, "1a", resp.Balances[0].Address)
	assert.True(t, resp.Balances[0].Stale)
	assert.NotEmpty(t, resp.Balances[0].CacheAge)

	assert.Equal(t, "eth", resp.Balances[1].Chain)
	assert.Equal(t, "0x1", resp.Balances[1].Address)
	assert.False(t, resp.Balances[1].Stale)

	// Test with errors
	batchResultWithError := &balance.FetchBatchResult{
		Results: []*balance.FetchResult{},
		Errors:  []error{errTestError},
	}
	respErr := convertToBalanceResponse("testwallet", batchResultWithError)
	assert.NotEmpty(t, respErr.Warning)
}

func TestOutputBalanceResponse(t *testing.T) {
	// Mock cmdCtx with JSON output
	mockCfg := &mockConfigProvider{
		outputFormat: "json",
	}
	mockFmt := &mockFormatProvider{format: output.FormatJSON}

	testCmdCtx := &CommandContext{
		Cfg: mockCfg,
		Fmt: mockFmt,
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	response := BalanceShowResponse{
		Wallet: "test",
		Balances: []BalanceResult{
			{Chain: "eth", Address: "0x1", Balance: "1.0", Symbol: "ETH"},
		},
	}

	err := outputBalanceResponse(cmd, testCmdCtx, response)
	require.NoError(t, err)

	// Verify JSON output
	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "test", parsed.Wallet)

	// Test Text output
	mockFmt.format = output.FormatText
	mockCfg.outputFormat = "text"
	buf.Reset()

	err = outputBalanceResponse(cmd, testCmdCtx, response)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Balances for wallet: test")
}

func TestLoadUTXOStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sigil-test-utxo-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockCfg := &mockConfigProvider{home: tmpDir}
	testCmdCtx := &CommandContext{Cfg: mockCfg}

	// Should not panic and return store
	store := loadUTXOStore(testCmdCtx, "testwallet")
	assert.NotNil(t, store)
}

func TestHandleCacheLoadError(t *testing.T) {
	mockLog := &mockLogger{}
	testCmdCtx := &CommandContext{Log: mockLog}
	var buf bytes.Buffer

	// Test Corrupt Error
	handleCacheLoadError(testCmdCtx, &buf, cache.ErrCorruptCache)
	assert.Contains(t, buf.String(), "cache was corrupted")
	assert.NotEmpty(t, mockLog.errors)

	// Test Other Error
	buf.Reset()
	mockLog.errors = nil
	handleCacheLoadError(testCmdCtx, &buf, errReadError)
	assert.Empty(t, buf.String()) // No output to stderr for general errors
	assert.NotEmpty(t, mockLog.errors)
}

// mockLogger implements LogWriter for testing
type mockLogger struct {
	info   []string
	errors []string
	debug  []string
}

func (m *mockLogger) Info(format string, v ...interface{}) {
	m.info = append(m.info, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Error(format string, v ...interface{}) {
	m.errors = append(m.errors, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Debug(format string, v ...interface{}) {
	m.debug = append(m.debug, fmt.Sprintf(format, v...))
}

func TestLoadBalanceCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sigil-test-cache-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockCfg := &mockConfigProvider{home: tmpDir}
	// Create cache dir
	_ = os.MkdirAll(filepath.Join(tmpDir, "cache"), 0o700)

	// Test load missing file (should return new cache)
	testCmdCtx := &CommandContext{Cfg: mockCfg}
	cacheVal := loadBalanceCache(testCmdCtx, nil)
	assert.NotNil(t, cacheVal)
	assert.Equal(t, 0, cacheVal.Size())

	// Test with refresh flag
	balanceRefresh = true
	cacheVal = loadBalanceCache(testCmdCtx, nil)
	assert.NotNil(t, cacheVal)
	balanceRefresh = false
}

func TestSaveBalanceCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sigil-test-cache-save-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockCfg := &mockConfigProvider{home: tmpDir}
	// Create cache dir
	_ = os.MkdirAll(filepath.Join(tmpDir, "cache"), 0o700)

	testCmdCtx2 := &CommandContext{Cfg: mockCfg}
	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{Chain: chain.ETH, Address: "0x1", Balance: "1.0"})

	saveBalanceCache(testCmdCtx2, balanceCache)

	// Verify file exists
	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	_, err = os.Stat(cachePath)
	assert.NoError(t, err)
}

func (m *mockLogger) Close() error {
	return nil
}
