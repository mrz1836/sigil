package addresslookup

import (
	"bytes"
	"sort"
	"strings"
	"sync"
	"time"
)

// AddressSet is a sorted flat byte buffer for O(log n) address lookup.
// Addresses are stored in fixed-width slots in sorted order.
// An optional parallel balance buffer stores balances at matching indices.
type AddressSet struct {
	buf       []byte
	slotWidth int
	count     int
	balBuf    []byte
	balWidth  int
	bufPool   sync.Pool
}

// Stats contains loading statistics for the address set.
type Stats struct {
	Count     int           `json:"count"`
	SlotWidth int           `json:"slot_width"`
	MemBytes  int64         `json:"mem_bytes"`
	LoadTime  time.Duration `json:"load_time"`
}

// LookupResult contains the result of an address lookup.
type LookupResult struct {
	Found   bool   `json:"found"`
	Balance string `json:"balance,omitempty"`
}

// addrBal is a temporary struct used during construction.
type addrBal struct {
	addr string
	bal  string
}

// NewAddressSet builds an AddressSet from address+balance pairs.
// Addresses are sorted and packed into a flat byte buffer.
// Balances are stored in a parallel buffer at the same indices.
func NewAddressSet(pairs []addrBal) *AddressSet {
	if len(pairs) == 0 {
		return &AddressSet{}
	}

	// Sort by address
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].addr < pairs[j].addr
	})

	// Determine slot widths
	addrWidth := 0
	balWidth := 0
	for i := range pairs {
		if len(pairs[i].addr) > addrWidth {
			addrWidth = len(pairs[i].addr)
		}
		if len(pairs[i].bal) > balWidth {
			balWidth = len(pairs[i].bal)
		}
	}

	// Pack into flat buffers
	buf := make([]byte, len(pairs)*addrWidth)
	var balBuf []byte
	if balWidth > 0 {
		balBuf = make([]byte, len(pairs)*balWidth)
	}

	for i, p := range pairs {
		copy(buf[i*addrWidth:], p.addr)
		if balWidth > 0 {
			copy(balBuf[i*balWidth:], p.bal)
		}
	}

	as := &AddressSet{
		buf:       buf,
		slotWidth: addrWidth,
		count:     len(pairs),
		balBuf:    balBuf,
		balWidth:  balWidth,
	}
	slotW := addrWidth
	as.bufPool.New = func() any {
		b := make([]byte, slotW)
		return &b
	}
	return as
}

// Lookup searches for an address and returns the result with balance.
func (s *AddressSet) Lookup(address string) LookupResult {
	if s.count == 0 || s.slotWidth == 0 {
		return LookupResult{}
	}

	// Reuse a pooled buffer to avoid per-call allocations in the hot path.
	bp := s.bufPool.Get().(*[]byte)
	padded := *bp
	copy(padded, address)
	for i := len(address); i < s.slotWidth; i++ {
		padded[i] = 0
	}

	idx := sort.Search(s.count, func(i int) bool {
		off := i * s.slotWidth
		return bytes.Compare(s.buf[off:off+s.slotWidth], padded) >= 0
	})

	var result LookupResult
	if idx < s.count {
		off := idx * s.slotWidth
		if bytes.Equal(s.buf[off:off+s.slotWidth], padded) {
			result.Found = true
			if s.balWidth > 0 && s.balBuf != nil {
				balOff := idx * s.balWidth
				result.Balance = strings.TrimRight(string(s.balBuf[balOff:balOff+s.balWidth]), "\x00")
			}
		}
	}

	s.bufPool.Put(bp)
	return result
}

// Contains returns true if the address is in the set.
func (s *AddressSet) Contains(address string) bool {
	return s.Lookup(address).Found
}

// Count returns the number of addresses in the set.
func (s *AddressSet) Count() int {
	return s.count
}

// MemBytes returns the approximate memory usage in bytes.
func (s *AddressSet) MemBytes() int64 {
	return int64(len(s.buf)) + int64(len(s.balBuf))
}

// padToWidth right-pads a string with null bytes to the given width,
// or truncates if longer.
func padToWidth(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	padded := make([]byte, width)
	copy(padded, s)
	return string(padded)
}
