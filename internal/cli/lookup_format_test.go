package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero rounds to KB", bytes: 0, want: "0 KB"},
		{name: "sub-KB truncates to 0 KB", bytes: 500, want: "0 KB"},
		{name: "exactly 1 KB", bytes: 1024, want: "1 KB"},
		{name: "KB truncates toward zero", bytes: 1536, want: "1 KB"},
		{name: "exactly 1 MB", bytes: 1 << 20, want: "1.0 MB"},
		{name: "several MB", bytes: 5 << 20, want: "5.0 MB"},
		{name: "exactly 1 GB", bytes: 1 << 30, want: "1.0 GB"},
		{name: "fractional GB", bytes: 1610612736, want: "1.5 GB"}, // 1.5 * 2^30
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatBytes(tc.bytes))
		})
	}
}
