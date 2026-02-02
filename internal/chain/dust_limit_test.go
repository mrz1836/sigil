package chain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDustLimit_ChainSpecific tests the DustLimit method returns correct values for each chain.
func TestDustLimit_ChainSpecific(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chainID  ID
		expected uint64
	}{
		{
			name:     "BSV has 1 satoshi minimum (no dust limit)",
			chainID:  BSV,
			expected: 1,
		},
		{
			name:     "BTC has 546 satoshi dust limit",
			chainID:  BTC,
			expected: 546,
		},
		{
			name:     "BCH has 546 satoshi dust limit",
			chainID:  BCH,
			expected: 546,
		},
		{
			name:     "ETH has no dust limit (uses gas)",
			chainID:  ETH,
			expected: 0,
		},
		{
			name:     "unknown chain returns 0",
			chainID:  ID("unknown"),
			expected: 0,
		},
		{
			name:     "empty chain ID returns 0",
			chainID:  ID(""),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.chainID.DustLimit())
		})
	}
}

// TestDustLimit_BSV_OneSatoshi verifies BSV accepts 1 satoshi outputs.
func TestDustLimit_BSV_OneSatoshi(t *testing.T) {
	t.Parallel()

	dustLimit := BSV.DustLimit()
	assert.Equal(t, uint64(1), dustLimit, "BSV should have 1 satoshi dust limit")

	// Verify 1 satoshi is at the limit
	assert.Equal(t, uint64(1), dustLimit)

	// Verify amounts above 1 are valid
	assert.Greater(t, uint64(2), dustLimit-1)
}

// TestDustLimit_BSV_ZeroReject verifies BSV rejects zero-value outputs.
func TestDustLimit_BSV_ZeroReject(t *testing.T) {
	t.Parallel()

	dustLimit := BSV.DustLimit()

	// Zero is below the dust limit
	assert.Positive(t, dustLimit, "BSV dust limit should be positive")
}

// TestDustLimit_BTC_Below546 verifies BTC rejects outputs below 546 satoshis.
func TestDustLimit_BTC_Below546(t *testing.T) {
	t.Parallel()

	dustLimit := BTC.DustLimit()
	assert.Equal(t, uint64(546), dustLimit)

	// 545 satoshis is below dust limit
	assert.Less(t, uint64(545), dustLimit)

	// 1 satoshi is below dust limit
	assert.Less(t, uint64(1), dustLimit)

	// Dust limit should be positive (0 is below it)
	assert.Positive(t, dustLimit)
}

// TestDustLimit_BTC_Exact546 verifies BTC accepts exactly 546 satoshi outputs.
func TestDustLimit_BTC_Exact546(t *testing.T) {
	t.Parallel()

	dustLimit := BTC.DustLimit()

	// Exactly 546 satoshis is at the limit
	assert.Equal(t, uint64(546), dustLimit)

	// 546 is valid (at the limit)
	assert.GreaterOrEqual(t, uint64(546), dustLimit)

	// 547 is valid (above the limit)
	assert.GreaterOrEqual(t, uint64(547), dustLimit)
}

// TestDustLimit_BCH_MatchesBTC verifies BCH has the same dust limit as BTC.
func TestDustLimit_BCH_MatchesBTC(t *testing.T) {
	t.Parallel()

	assert.Equal(t, BTC.DustLimit(), BCH.DustLimit(),
		"BCH and BTC should have the same dust limit")
}

// TestDustLimit_ETH_NoLimit verifies ETH has no dust limit (uses gas instead).
func TestDustLimit_ETH_NoLimit(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint64(0), ETH.DustLimit(),
		"ETH should have no dust limit (uses gas)")
}

// TestDustLimit_UTXO_vs_AccountChains verifies the distinction between UTXO and account chains.
func TestDustLimit_UTXO_vs_AccountChains(t *testing.T) {
	t.Parallel()

	// UTXO chains have non-zero dust limits (except BSV which has 1)
	utxoChains := []ID{BSV, BTC, BCH}
	for _, chainID := range utxoChains {
		assert.Positive(t, chainID.DustLimit(),
			"%s should have positive dust limit", chainID)
	}

	// Account-based chains have zero dust limit
	accountChains := []ID{ETH}
	for _, chainID := range accountChains {
		assert.Equal(t, uint64(0), chainID.DustLimit(),
			"%s should have dust limit = 0", chainID)
	}
}

// TestDustLimit_OutputValidation tests practical output validation scenarios.
func TestDustLimit_OutputValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chainID ID
		amount  uint64
		valid   bool
	}{
		// BSV scenarios
		{"BSV: 1 satoshi valid", BSV, 1, true},
		{"BSV: 0 satoshi invalid", BSV, 0, false},
		{"BSV: 546 satoshi valid", BSV, 546, true},
		{"BSV: 1 BSV valid", BSV, 100000000, true},

		// BTC scenarios
		{"BTC: 1 satoshi invalid", BTC, 1, false},
		{"BTC: 545 satoshi invalid", BTC, 545, false},
		{"BTC: 546 satoshi valid", BTC, 546, true},
		{"BTC: 547 satoshi valid", BTC, 547, true},
		{"BTC: 0 satoshi invalid", BTC, 0, false},

		// BCH scenarios (same as BTC)
		{"BCH: 1 satoshi invalid", BCH, 1, false},
		{"BCH: 546 satoshi valid", BCH, 546, true},

		// ETH scenarios (no dust limit, but 0 is still conceptually invalid)
		{"ETH: 0 wei technically valid (no dust)", ETH, 0, true},
		{"ETH: 1 wei valid", ETH, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dustLimit := tt.chainID.DustLimit()
			isValid := tt.amount >= dustLimit

			assert.Equal(t, tt.valid, isValid,
				"amount %d should be %s for %s (dust limit: %d)",
				tt.amount, map[bool]string{true: "valid", false: "invalid"}[tt.valid],
				tt.chainID, dustLimit)
		})
	}
}
