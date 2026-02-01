// Package metrics provides application-level metrics collection.
// This is a lightweight metrics foundation using atomic counters.
// For production observability, consider integrating with Prometheus or similar.
package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics holds application metrics using atomic counters for thread safety.
type Metrics struct {
	// RPC metrics
	rpcCallsTotal   atomic.Int64
	rpcErrorsTotal  atomic.Int64
	rpcLatencyNanos atomic.Int64

	// Wallet operation metrics
	walletOpsTotal  atomic.Int64
	walletOpsErrors atomic.Int64

	// Cache metrics
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64

	// Chain-specific RPC calls
	ethRPCCalls atomic.Int64
	bsvRPCCalls atomic.Int64
}

// Global is the global metrics instance.
// Use this for recording metrics throughout the application.
//
//nolint:gochecknoglobals // Intentional global for metrics access
var Global = &Metrics{}

// RecordRPCCall records an RPC call with its duration and success status.
func (m *Metrics) RecordRPCCall(chain string, duration time.Duration, err error) {
	m.rpcCallsTotal.Add(1)
	m.rpcLatencyNanos.Add(duration.Nanoseconds())

	if err != nil {
		m.rpcErrorsTotal.Add(1)
	}

	// Track per-chain calls
	switch chain {
	case "eth":
		m.ethRPCCalls.Add(1)
	case "bsv":
		m.bsvRPCCalls.Add(1)
	}
}

// RecordWalletOp records a wallet operation.
func (m *Metrics) RecordWalletOp(err error) {
	m.walletOpsTotal.Add(1)
	if err != nil {
		m.walletOpsErrors.Add(1)
	}
}

// RecordCacheHit records a cache hit.
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Add(1)
}

// RecordCacheMiss records a cache miss.
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Add(1)
}

// Snapshot returns a point-in-time copy of all metrics.
type Snapshot struct {
	RPCCallsTotal   int64
	RPCErrorsTotal  int64
	RPCLatencyNanos int64
	WalletOpsTotal  int64
	WalletOpsErrors int64
	CacheHits       int64
	CacheMisses     int64
	ETHRPCCalls     int64
	BSVRPCCalls     int64
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		RPCCallsTotal:   m.rpcCallsTotal.Load(),
		RPCErrorsTotal:  m.rpcErrorsTotal.Load(),
		RPCLatencyNanos: m.rpcLatencyNanos.Load(),
		WalletOpsTotal:  m.walletOpsTotal.Load(),
		WalletOpsErrors: m.walletOpsErrors.Load(),
		CacheHits:       m.cacheHits.Load(),
		CacheMisses:     m.cacheMisses.Load(),
		ETHRPCCalls:     m.ethRPCCalls.Load(),
		BSVRPCCalls:     m.bsvRPCCalls.Load(),
	}
}

// RPCCallsTotal returns the total number of RPC calls made.
func (m *Metrics) RPCCallsTotal() int64 {
	return m.rpcCallsTotal.Load()
}

// RPCErrorsTotal returns the total number of RPC errors.
func (m *Metrics) RPCErrorsTotal() int64 {
	return m.rpcErrorsTotal.Load()
}

// RPCLatencyAvgMs returns the average RPC latency in milliseconds.
// Returns 0 if no calls have been made.
func (m *Metrics) RPCLatencyAvgMs() float64 {
	calls := m.rpcCallsTotal.Load()
	if calls == 0 {
		return 0
	}
	nanos := m.rpcLatencyNanos.Load()
	return float64(nanos) / float64(calls) / 1e6
}

// CacheHitRate returns the cache hit rate as a percentage (0-100).
// Returns 0 if no cache operations have occurred.
func (m *Metrics) CacheHitRate() float64 {
	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// Reset resets all metrics to zero.
// Useful for testing.
func (m *Metrics) Reset() {
	m.rpcCallsTotal.Store(0)
	m.rpcErrorsTotal.Store(0)
	m.rpcLatencyNanos.Store(0)
	m.walletOpsTotal.Store(0)
	m.walletOpsErrors.Store(0)
	m.cacheHits.Store(0)
	m.cacheMisses.Store(0)
	m.ethRPCCalls.Store(0)
	m.bsvRPCCalls.Store(0)
}
