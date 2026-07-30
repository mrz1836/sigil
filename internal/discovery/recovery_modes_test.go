package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoveryScenarios_getGapLimitForMode(t *testing.T) {
	t.Parallel()

	r := &RecoveryScenarios{}
	assert.Equal(t, DefaultGapLimit, r.getGapLimitForMode(RecoveryModeStandard))
	assert.Equal(t, RecoveryGapLimit, r.getGapLimitForMode(RecoveryModeExtended))
	assert.Equal(t, ExtendedRecoveryGapLimit, r.getGapLimitForMode(RecoveryModeAggressive))
	// An unknown mode falls back to the default gap limit.
	assert.Equal(t, DefaultGapLimit, r.getGapLimitForMode(RecoveryMode(99)))
}

func TestRecoveryScenarios_getSchemes(t *testing.T) {
	t.Parallel()

	r := &RecoveryScenarios{}
	defaults := DefaultSchemes()
	require.NotEmpty(t, defaults)

	// No specific schemes requested → scan all defaults.
	assert.Equal(t, defaults, r.getSchemes(nil))

	// Only unknown names → fall back to defaults.
	assert.Equal(t, defaults, r.getSchemes([]string{"totally-unknown-scheme"}))

	// A known scheme name resolves to exactly that scheme.
	valid := defaults[0].Name
	got := r.getSchemes([]string{valid})
	require.Len(t, got, 1)
	assert.Equal(t, valid, got[0].Name)
}
