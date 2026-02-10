package eth

import "sync"

// NonceManager tracks the highest sent nonce per address to prevent
// nonce collisions when multiple transactions are sent in rapid succession
// (before the first is visible in the mempool).
type NonceManager struct {
	mu     sync.Mutex
	nonces map[string]uint64 // address -> next nonce (one past the highest used)
}

// NewNonceManager creates a new NonceManager.
func NewNonceManager() *NonceManager {
	return &NonceManager{
		nonces: make(map[string]uint64),
	}
}

// Next returns the next nonce to use for the given address.
// It takes the RPC-reported pending nonce and returns the higher of
// the RPC nonce and the locally tracked nonce. The local nonce is
// then incremented for the next call.
func (nm *NonceManager) Next(address string, rpcNonce uint64) uint64 {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	local, exists := nm.nonces[address]

	// Use the higher of RPC nonce and local nonce.
	// If RPC nonce is higher, the network has caught up or advanced past
	// our local tracking (e.g., transaction sent from another client).
	nonce := rpcNonce
	if exists && local > rpcNonce {
		nonce = local
	}

	// Track the next nonce
	nm.nonces[address] = nonce + 1

	return nonce
}

// Reset clears the local nonce tracking for an address.
// Useful after errors or when nonce state is known to be stale.
func (nm *NonceManager) Reset(address string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	delete(nm.nonces, address)
}
