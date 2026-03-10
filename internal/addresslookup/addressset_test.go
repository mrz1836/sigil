package addresslookup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAddressSet_Empty(t *testing.T) {
	t.Parallel()
	set := NewAddressSet(nil)
	assert.Equal(t, 0, set.Count())
	assert.False(t, set.Contains("anything"))
}

func TestNewAddressSet_SingleAddress(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{{addr: "1ABC", bal: "100"}}
	set := NewAddressSet(pairs)

	assert.Equal(t, 1, set.Count())
	assert.True(t, set.Contains("1ABC"))
	assert.False(t, set.Contains("1DEF"))
}

func TestNewAddressSet_MultipleAddresses(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "1DEF", bal: "200"},
		{addr: "1ABC", bal: "100"},
		{addr: "1GHI", bal: "300"},
	}
	set := NewAddressSet(pairs)

	assert.Equal(t, 3, set.Count())

	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))
	assert.True(t, set.Contains("1GHI"))

	assert.False(t, set.Contains("1XYZ"))
	assert.False(t, set.Contains(""))
}

func TestAddressSet_Lookup_ReturnsBalance(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "1ABC", bal: "100.50"},
		{addr: "1DEF", bal: "200.75"},
		{addr: "1GHI", bal: "0.01"},
	}
	set := NewAddressSet(pairs)

	result := set.Lookup("1DEF")
	assert.True(t, result.Found)
	assert.Equal(t, "200.75", result.Balance)

	result = set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Equal(t, "100.50", result.Balance)

	result = set.Lookup("1GHI")
	assert.True(t, result.Found)
	assert.Equal(t, "0.01", result.Balance)

	result = set.Lookup("1XYZ")
	assert.False(t, result.Found)
	assert.Empty(t, result.Balance)
}

func TestAddressSet_Lookup_EmptySet(t *testing.T) {
	t.Parallel()
	set := &AddressSet{}

	result := set.Lookup("1ABC")
	assert.False(t, result.Found)
	assert.Empty(t, result.Balance)
}

func TestAddressSet_Contains_RealAddressFormats(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
		{addr: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"},
		{addr: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"},
	}
	set := NewAddressSet(pairs)

	assert.Equal(t, 3, set.Count())
	assert.True(t, set.Contains("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"))
	assert.True(t, set.Contains("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"))
	assert.True(t, set.Contains("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"))
	assert.False(t, set.Contains("1ABC"))
}

func TestAddressSet_MemBytes(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "AAAA", bal: "10"},
		{addr: "BBBB", bal: "20"},
	}
	set := NewAddressSet(pairs)

	assert.Equal(t, int64(12), set.MemBytes())
}

func TestAddressSet_NoBalances(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "1ABC"},
		{addr: "1DEF"},
	}
	set := NewAddressSet(pairs)

	assert.Equal(t, 2, set.Count())
	assert.True(t, set.Contains("1ABC"))

	result := set.Lookup("1ABC")
	assert.True(t, result.Found)
	assert.Empty(t, result.Balance)
}

func TestAddressSet_LargeDataset(t *testing.T) {
	t.Parallel()
	n := 10000
	pairs := make([]addrBal, n)
	for i := 0; i < n; i++ {
		pairs[i] = addrBal{
			addr: fmt.Sprintf("addr_%06d", i),
			bal:  fmt.Sprintf("%d", i*100),
		}
	}
	set := NewAddressSet(pairs)

	require.Equal(t, n, set.Count())

	assert.True(t, set.Contains("addr_000000"))
	assert.True(t, set.Contains("addr_005000"))
	assert.True(t, set.Contains("addr_009999"))
	assert.False(t, set.Contains("addr_010000"))

	result := set.Lookup("addr_000042")
	assert.True(t, result.Found)
	assert.Equal(t, "4200", result.Balance)
}

func TestAddressSet_DuplicateAddresses(t *testing.T) {
	t.Parallel()
	pairs := []addrBal{
		{addr: "1ABC", bal: "100"},
		{addr: "1ABC", bal: "200"},
		{addr: "1DEF", bal: "300"},
	}
	set := NewAddressSet(pairs)

	assert.Equal(t, 3, set.Count())
	assert.True(t, set.Contains("1ABC"))
	assert.True(t, set.Contains("1DEF"))
}

func TestPadToWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"abc", 5, "abc\x00\x00"},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "abcde"},
		{"", 3, "\x00\x00\x00"},
	}
	for _, tt := range tests {
		result := padToWidth(tt.input, tt.width)
		assert.Equal(t, tt.expected, result)
	}
}

func BenchmarkAddressSet_Contains(b *testing.B) {
	n := 100000
	pairs := make([]addrBal, n)
	for i := 0; i < n; i++ {
		pairs[i] = addrBal{addr: fmt.Sprintf("1Addr%08d", i)}
	}
	set := NewAddressSet(pairs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set.Contains(fmt.Sprintf("1Addr%08d", i%n))
	}
}

func BenchmarkLoad_And_Lookup(b *testing.B) {
	n := 100000
	dir := b.TempDir()
	path := filepath.Join(dir, "addrs.csv")

	var sb strings.Builder
	sb.WriteString("address,balance\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "1Addr%08d,%d\n", i, i)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		b.Fatal(err)
	}

	set, _, err := Load(path)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set.Lookup(fmt.Sprintf("1Addr%08d", i%n))
	}
}
