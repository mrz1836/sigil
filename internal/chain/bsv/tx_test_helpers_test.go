package bsv

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	whatsonchain "github.com/mrz1836/go-whatsonchain"
)

// Static test errors for err113 compliance.
// Capitalized strings match real API error messages from WhatsOnChain/ARC.
var (
	errTestBroadcast           = errors.New("broadcast error")
	errTestServerError         = errors.New("server error")
	errTestNetworkServerError  = errors.New("network: server error")
	errTestConnRefused         = errors.New("connection refused")
	errTestServiceUnavailable  = errors.New("service unavailable")
	errTestInvalidJSON         = errors.New("invalid character 'n' looking for beginning of value")
	errTestMissingInputs       = errors.New("Missing inputs")                     //nolint:staticcheck // matches real API error
	errTestAlreadyInMempool    = errors.New("Transaction already in mempool")     //nolint:staticcheck // matches real API error
	errTestAlreadyInTheMempool = errors.New("Transaction already in the mempool") //nolint:staticcheck // matches real API error
	errTestTxnAlreadyKnown     = errors.New("txn-already-known")
	errTestMempoolBase         = errors.New("already in mempool")
)

// testValidTxID is a valid 64-hex-char transaction ID for test use.
const testValidTxID = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"

// Test addresses - these are well-known Bitcoin addresses with valid checksums.
const (
	// testAddress is a valid P2PKH address (genesis block coinbase).
	testAddress = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	// testAddress2 is another valid P2PKH address.
	testAddress2 = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	// testAddress3 is a valid P2SH address.
	testAddress3 = "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"
)

// validAddress returns a well-known valid P2PKH address for testing.
func validAddress() string {
	return testAddress
}

// validAddress2 returns a second valid P2PKH address for testing.
func validAddress2() string {
	return testAddress2
}

// validP2SHAddress returns a valid P2SH address for testing.
func validP2SHAddress() string {
	return testAddress3
}

// makeUTXO creates a UTXO for testing with the given parameters.
func makeUTXO(txid string, amount uint64) UTXO {
	return UTXO{
		TxID:    txid,
		Vout:    0,
		Amount:  amount,
		Address: testAddress,
	}
}

// makeUTXOs creates multiple UTXOs with sequential txids and vout=0.
func makeUTXOs(amounts ...uint64) []UTXO {
	utxos := make([]UTXO, len(amounts))
	for i, amount := range amounts {
		utxos[i] = UTXO{
			TxID:    testTxID(i + 1), // Use valid hex txid
			Vout:    0,
			Amount:  amount,
			Address: testAddress,
		}
	}
	return utxos
}

// testTxID returns a valid-looking 64-character transaction ID.
func testTxID(n int) string {
	return fmt.Sprintf("%064x", n)
}

// mockServerConfig holds configuration for mock WOC and ARC test servers.
type mockServerConfig struct {
	UTXOs           []UTXO
	Balance         int64
	BroadcastTxHash string
	BroadcastFail   bool
	FeeRate         uint64
}

// newMockWOCFromConfig creates a mockWOCClient from a mockServerConfig.
func newMockWOCFromConfig(cfg mockServerConfig) *mockWOCClient {
	return &mockWOCClient{
		balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
			return &whatsonchain.AddressBalance{
				Confirmed:   cfg.Balance,
				Unconfirmed: 0,
			}, nil
		},
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return toHistoryRecords(cfg.UTXOs), nil
		},
		feeFunc: func(_ context.Context, _, _ int64) ([]*whatsonchain.MinerFeeStats, error) {
			feeRate := cfg.FeeRate
			if feeRate == 0 {
				feeRate = DefaultFeeRate
			}
			return []*whatsonchain.MinerFeeStats{
				{
					Miner:      "test_miner",
					MinFeeRate: float64(feeRate),
				},
			}, nil
		},
		broadcastFunc: func(_ context.Context, _ string) (string, error) {
			if cfg.BroadcastFail {
				return "", errTestBroadcast
			}
			return cfg.BroadcastTxHash, nil
		},
	}
}

// mockUTXOClient creates a mockWOCClient that returns the specified UTXOs.
func mockUTXOClient(utxos []UTXO) *mockWOCClient {
	return &mockWOCClient{
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return toHistoryRecords(utxos), nil
		},
	}
}

// mockErrorClient creates a mockWOCClient that returns errors.
func mockErrorClient() *mockWOCClient {
	return &mockWOCClient{
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return nil, errTestNetworkServerError
		},
		balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
			return nil, errTestNetworkServerError
		},
	}
}

// testKeyPair holds a valid private key and its corresponding address for testing.
type testKeyPair struct {
	PrivateKey []byte //nolint:gosec // G117: test helper struct field
	Address    string
}

// generateTestKeyPair creates a deterministic key pair for testing.
// Uses a fixed seed to ensure reproducible tests.
func generateTestKeyPair() testKeyPair {
	// Fixed 32-byte private key for deterministic testing
	// This is NOT a real wallet key - it's just for tests
	privateKeyHex := "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"
	privateKeyBytes, _ := hex.DecodeString(privateKeyHex)

	// Derive the address from the private key
	_, pubKey := ec.PrivateKeyFromBytes(privateKeyBytes)
	addr, _ := script.NewAddressFromPublicKey(pubKey, true)

	return testKeyPair{
		PrivateKey: privateKeyBytes,
		Address:    addr.AddressString,
	}
}

// getTestKeyPair returns a fresh test key pair.
// Returns a new copy each time to avoid race conditions in parallel tests.
func getTestKeyPair() testKeyPair {
	return generateTestKeyPair()
}

// makeUTXOsWithKey creates multiple UTXOs with sequential txids for a key pair.
func makeUTXOsWithKey(kp testKeyPair, amounts ...uint64) []UTXO {
	utxos := make([]UTXO, len(amounts))
	for i, amount := range amounts {
		utxos[i] = UTXO{
			TxID:    testTxID(i + 1),
			Vout:    0,
			Amount:  amount,
			Address: kp.Address,
		}
	}
	return utxos
}

// generateTestKeyPair2 creates a second deterministic key pair for multi-address testing.
func generateTestKeyPair2() testKeyPair {
	// Different fixed 32-byte private key for a second address
	privateKeyHex := "4b7a2da3bd6f891249fd81c2abc7f1fbd6dad23f08b4b77cbb01e3f7ecb4e24e"
	privateKeyBytes, _ := hex.DecodeString(privateKeyHex)

	_, pubKey := ec.PrivateKeyFromBytes(privateKeyBytes)
	addr, _ := script.NewAddressFromPublicKey(pubKey, true)

	return testKeyPair{
		PrivateKey: privateKeyBytes,
		Address:    addr.AddressString,
	}
}

// getTestKeyPair2 returns a fresh second test key pair.
func getTestKeyPair2() testKeyPair {
	return generateTestKeyPair2()
}

// makeUTXOsMultiAddr creates UTXOs spread across multiple key pairs for multi-address testing.
func makeUTXOsMultiAddr(pairs []testKeyPair, amountsPerPair [][]uint64) []UTXO {
	total := 0
	for _, amounts := range amountsPerPair {
		total += len(amounts)
	}
	utxos := make([]UTXO, 0, total)
	n := 0
	for i, kp := range pairs {
		for _, amount := range amountsPerPair[i] {
			n++
			utxos = append(utxos, UTXO{
				TxID:    testTxID(n),
				Vout:    0,
				Amount:  amount,
				Address: kp.Address,
			})
		}
	}
	return utxos
}

// Ensure mockServerConfig is used.
var _ = mockServerConfig{}
