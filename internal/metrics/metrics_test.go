package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestMetrics_RecordRPCCall(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	// Record successful call
	m.RecordRPCCall("eth", 100*time.Millisecond, nil)
	assert.Equal(t, int64(1), m.RPCCallsTotal())
	assert.Equal(t, int64(0), m.RPCErrorsTotal())
	assert.Equal(t, int64(1), m.ethRPCCalls.Load())

	// Record failed call
	m.RecordRPCCall("bsv", 50*time.Millisecond, sigilerr.ErrNetworkError)
	assert.Equal(t, int64(2), m.RPCCallsTotal())
	assert.Equal(t, int64(1), m.RPCErrorsTotal())
	assert.Equal(t, int64(1), m.bsvRPCCalls.Load())
}

func TestMetrics_RecordWalletOp(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	m.RecordWalletOp(nil)
	m.RecordWalletOp(sigilerr.ErrGeneral)

	snap := m.Snapshot()
	assert.Equal(t, int64(2), snap.WalletOpsTotal)
	assert.Equal(t, int64(1), snap.WalletOpsErrors)
}

func TestMetrics_CacheHitRate(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	// No operations
	assert.InDelta(t, 0.0, m.CacheHitRate(), 0.001)

	// 3 hits, 1 miss = 75%
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	assert.InDelta(t, 75.0, m.CacheHitRate(), 0.001)
}

func TestMetrics_RPCLatencyAvg(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	// No calls
	assert.InDelta(t, 0.0, m.RPCLatencyAvgMs(), 0.001)

	// Two calls: 100ms and 200ms = 150ms avg
	m.RecordRPCCall("eth", 100*time.Millisecond, nil)
	m.RecordRPCCall("eth", 200*time.Millisecond, nil)

	avg := m.RPCLatencyAvgMs()
	assert.InDelta(t, 150.0, avg, 1.0)
}

func TestMetrics_Snapshot(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	m.RecordRPCCall("eth", time.Millisecond, nil)
	m.RecordCacheHit()
	m.RecordWalletOp(nil)

	snap := m.Snapshot()
	assert.Equal(t, int64(1), snap.RPCCallsTotal)
	assert.Equal(t, int64(1), snap.CacheHits)
	assert.Equal(t, int64(1), snap.WalletOpsTotal)
	assert.Equal(t, int64(1), snap.ETHRPCCalls)
}

func TestMetrics_Reset(t *testing.T) {
	t.Parallel()
	m := &Metrics{}

	m.RecordRPCCall("eth", time.Millisecond, nil)
	m.RecordCacheHit()
	m.RecordWalletOp(nil)

	m.Reset()

	snap := m.Snapshot()
	assert.Equal(t, int64(0), snap.RPCCallsTotal)
	assert.Equal(t, int64(0), snap.CacheHits)
	assert.Equal(t, int64(0), snap.WalletOpsTotal)
}

func TestGlobal(t *testing.T) {
	// Test that Global is initialized
	assert.NotNil(t, Global)

	// Reset to not affect other tests
	Global.Reset()
}
