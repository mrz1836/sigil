package btc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestVarIntLen exercises every CompactSize size class boundary.
func TestVarIntLen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    uint64
		want uint64
	}{
		{name: "zero", n: 0, want: 1},
		{name: "one", n: 1, want: 1},
		{name: "max 1-byte (0xfc)", n: 0xfc, want: 1},
		{name: "min 3-byte (0xfd)", n: 0xfd, want: 3},
		{name: "max 3-byte (0xffff)", n: 0xffff, want: 3},
		{name: "min 5-byte (0x10000)", n: 0x10000, want: 5},
		{name: "max 5-byte (0xffffffff)", n: 0xffffffff, want: 5},
		{name: "min 9-byte (0x100000000)", n: 0x100000000, want: 9},
		{name: "max uint64", n: ^uint64(0), want: 9},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, varIntLen(tc.n))
		})
	}
}
