package bsv

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
)

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

// mockUTXOServer creates a test server that returns the specified UTXOs.
func mockUTXOServer(utxos []UTXO) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Convert UTXOs to WhatsOnChain format
		resp := make([]UTXOResponse, len(utxos))
		for i, u := range utxos {
			resp[i] = UTXOResponse{
				TxID:   u.TxID,
				Vout:   u.Vout,
				Value:  u.Amount,
				Height: 100,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// mockErrorServer creates a test server that returns the specified status code and error.
func mockErrorServer(statusCode int, errorMsg string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(errorMsg))
	}))
}

// mockMultiRouteServer creates a test server with configurable responses for different endpoints.
type mockServerConfig struct {
	UTXOs           []UTXO
	Balance         int64
	BroadcastTxHash string
	BroadcastFail   bool
	FeeRate         uint64
}

//nolint:gocognit // Multi-route server needs to handle multiple endpoints
func mockMultiRouteServer(cfg mockServerConfig) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/balance"):
			resp := BalanceResponse{
				Confirmed:   cfg.Balance,
				Unconfirmed: 0,
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/unspent"):
			resp := make([]UTXOResponse, len(cfg.UTXOs))
			for i, u := range cfg.UTXOs {
				resp[i] = UTXOResponse{
					TxID:   u.TxID,
					Vout:   u.Vout,
					Value:  u.Amount,
					Height: 100,
				}
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/tx/raw"):
			if cfg.BroadcastFail {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("broadcast error"))
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(cfg.BroadcastTxHash))

		case strings.Contains(r.URL.Path, "/miner/fees"):
			feeRate := cfg.FeeRate
			if feeRate == 0 {
				feeRate = DefaultFeeRate
			}
			entries := []wocMinerFeeEntry{
				{
					Timestamp: time.Now().Unix(),
					Name:      "test_miner",
					FeeRate:   float64(feeRate),
				},
			}
			_ = json.NewEncoder(w).Encode(entries)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

// testKeyPair holds a valid private key and its corresponding address for testing.
type testKeyPair struct {
	PrivateKey []byte
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

// Ensure mockServerConfig and mockMultiRouteServer are used (for godot linter).
var (
	_ = mockServerConfig{}
	_ = mockMultiRouteServer
)
