package balance

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransportManager_Get_SingletonBehavior tests that Get returns the same transport.
func TestTransportManager_Get_SingletonBehavior(t *testing.T) {
	t.Parallel()

	var tm TransportManager

	// Call Get multiple times
	transport1 := tm.Get()
	transport2 := tm.Get()
	transport3 := tm.Get()

	// Should return same instance
	require.NotNil(t, transport1)
	assert.Same(t, transport1, transport2, "second call should return same instance")
	assert.Same(t, transport1, transport3, "third call should return same instance")
}

// TestTransportManager_Get_Initialization tests that transport is properly initialized.
func TestTransportManager_Get_Initialization(t *testing.T) {
	t.Parallel()

	var tm TransportManager

	transport := tm.Get()

	require.NotNil(t, transport)
	// Verify basic transport properties from rpc.NewDefaultTransport
	assert.Equal(t, 100, transport.MaxIdleConns, "MaxIdleConns should be 100")
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost, "MaxIdleConnsPerHost should be 10")
	assert.Equal(t, 20, transport.MaxConnsPerHost, "MaxConnsPerHost should be 20")
}

// TestTransportManager_Get_ConcurrentAccess tests concurrent access is safe.
// CONCURRENCY-CRITICAL: This test MUST be run with -race flag.
func TestTransportManager_Get_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	var tm TransportManager
	const numGoroutines = 100

	// Channel to collect results from goroutines
	results := make(chan *http.Transport, numGoroutines)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch many goroutines that call Get() concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			transport := tm.Get()
			results <- transport
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect all results
	var transports []*http.Transport
	for transport := range results {
		transports = append(transports, transport)
	}

	// Verify we got all results
	require.Len(t, transports, numGoroutines)

	// Verify all goroutines got the same instance
	firstTransport := transports[0]
	for i, transport := range transports {
		assert.Same(t, firstTransport, transport, "goroutine %d got different instance", i)
	}
}

// TestTransportManager_Get_SyncOnceSemantics tests sync.Once is called exactly once.
func TestTransportManager_Get_SyncOnceSemantics(t *testing.T) {
	t.Parallel()

	var tm TransportManager

	// Track how many times the initialization would have happened
	// (We can't directly test sync.Once, but we can verify the result)

	const numCalls = 50
	var wg sync.WaitGroup
	wg.Add(numCalls)

	results := make(chan *http.Transport, numCalls)

	// Call Get() from multiple goroutines
	for i := 0; i < numCalls; i++ {
		go func() {
			defer wg.Done()
			results <- tm.Get()
		}()
	}

	wg.Wait()
	close(results)

	// All results should be the same instance (proving sync.Once worked)
	var firstTransport *http.Transport
	count := 0
	for transport := range results {
		if firstTransport == nil {
			firstTransport = transport
		}
		assert.Same(t, firstTransport, transport)
		count++
	}

	assert.Equal(t, numCalls, count)
}

// TestSharedETHTransport_Properties tests the global sharedETHTransport properties.
func TestSharedETHTransport_Properties(t *testing.T) {
	// Not parallel - uses global state

	// Call the global function multiple times
	transport1 := sharedETHTransport()
	transport2 := sharedETHTransport()

	require.NotNil(t, transport1)
	assert.Same(t, transport1, transport2, "should return same global instance")

	// Verify it's properly initialized
	assert.Equal(t, 100, transport1.MaxIdleConns)
	assert.Equal(t, 10, transport1.MaxIdleConnsPerHost)
	assert.Equal(t, 20, transport1.MaxConnsPerHost)
}

// TestSharedETHTransport_ConcurrentGlobal tests concurrent access to global transport.
// CONCURRENCY-CRITICAL: This test MUST be run with -race flag.
func TestSharedETHTransport_ConcurrentGlobal(t *testing.T) {
	// Not parallel - uses global state

	const numGoroutines = 100
	results := make(chan *http.Transport, numGoroutines)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch many goroutines that call sharedETHTransport() concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			transport := sharedETHTransport()
			results <- transport
		}()
	}

	wg.Wait()
	close(results)

	// Collect all results
	var transports []*http.Transport
	for transport := range results {
		transports = append(transports, transport)
	}

	// Verify all got the same instance
	require.Len(t, transports, numGoroutines)
	firstTransport := transports[0]
	for i, transport := range transports {
		assert.Same(t, firstTransport, transport, "goroutine %d got different instance", i)
	}
}
