package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMode_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode AuthMode
		want string
	}{
		{"session", AuthSession, "session"},
		{"agent token", AuthAgentToken, "agent_token"},
		{"xpub", AuthXpub, "xpub"},
		{"password", AuthPassword, "password"},
		// Any value outside the defined enum falls through to the default arm.
		{"unknown falls through to default", AuthMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}
