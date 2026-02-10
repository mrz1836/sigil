package balance

import (
	"net/http"
	"sync"

	"github.com/mrz1836/sigil/internal/chain/eth/rpc"
)

// TransportManager manages a singleton HTTP transport for ETH clients.
// This ensures proper connection pooling across concurrent requests.
type TransportManager struct {
	once      sync.Once
	transport *http.Transport
}

// Get returns the shared HTTP transport, creating it on first call.
func (tm *TransportManager) Get() *http.Transport {
	tm.once.Do(func() {
		tm.transport = rpc.NewDefaultTransport()
	})
	return tm.transport
}

// globalTransportManager is the singleton instance used by the package.
//
//nolint:gochecknoglobals // Singleton for connection pooling
var globalTransportManager TransportManager

// sharedETHTransport returns the shared HTTP transport for ETH clients.
func sharedETHTransport() *http.Transport {
	return globalTransportManager.Get()
}
