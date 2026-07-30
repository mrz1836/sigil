package sigilcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetScryptWorkFactor_Clamping verifies the work factor is clamped into the
// valid [10, 22] range. It runs as an internal test so it can observe the
// package-level atomic, and restores the original value so other tests keep the
// secure default. It must not run in parallel (it mutates process-global state).
func TestSetScryptWorkFactor_Clamping(t *testing.T) {
	orig := scryptWorkFactor.Load()
	t.Cleanup(func() { scryptWorkFactor.Store(orig) })

	tests := []struct {
		name string
		in   int
		want int32
	}{
		{name: "below floor clamps to 10", in: 5, want: 10},
		{name: "at floor", in: 10, want: 10},
		{name: "mid range preserved", in: 15, want: 15},
		{name: "default", in: 18, want: 18},
		{name: "at ceiling", in: 22, want: 22},
		{name: "above ceiling clamps to 22", in: 30, want: 22},
	}
	for _, tc := range tests {
		SetScryptWorkFactor(tc.in)
		assert.Equal(t, tc.want, scryptWorkFactor.Load(), tc.name)
	}
}
