package addresslookup

import (
	"time"
)

// AddressSet is a hash map for O(1) address lookup.
// Addresses are stored as keys, balances as values.
type AddressSet struct {
	m        map[string]string
	count    int
	memBytes int64
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
func NewAddressSet(pairs []addrBal) *AddressSet {
	if len(pairs) == 0 {
		return &AddressSet{}
	}

	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p.addr] = p.bal
	}
	var memBytes int64
	for k, v := range m {
		memBytes += int64(len(k) + len(v) + 80) // 80 bytes approx map bucket overhead per entry
	}
	return &AddressSet{m: m, count: len(m), memBytes: memBytes}
}

// Lookup searches for an address and returns the result with balance.
func (s *AddressSet) Lookup(address string) LookupResult {
	if s.count == 0 {
		return LookupResult{}
	}
	if bal, ok := s.m[address]; ok {
		return LookupResult{Found: true, Balance: bal}
	}
	return LookupResult{}
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
// Computed once at construction time for O(1) access.
func (s *AddressSet) MemBytes() int64 {
	return s.memBytes
}
