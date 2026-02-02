package bsv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
			TxID:    fmt.Sprintf("tx%064d", i+1)[:64], // Ensure 64-char txid
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

		case strings.Contains(r.URL.Path, "/mapi/tx"):
			if cfg.BroadcastFail {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"txid": cfg.BroadcastTxHash})

		case strings.Contains(r.URL.Path, "/feeQuote"):
			feeRate := cfg.FeeRate
			if feeRate == 0 {
				feeRate = DefaultFeeRate
			}
			payload := TAALPayload{
				Fees: []struct {
					FeeType   string `json:"feeType"`
					MiningFee struct {
						Satoshis int64 `json:"satoshis"`
						Bytes    int64 `json:"bytes"`
					} `json:"miningFee"`
					RelayFee struct {
						Satoshis int64 `json:"satoshis"`
						Bytes    int64 `json:"bytes"`
					} `json:"relayFee"`
				}{
					{
						FeeType: "standard",
						MiningFee: struct {
							Satoshis int64 `json:"satoshis"`
							Bytes    int64 `json:"bytes"`
						}{
							//nolint:gosec // Test code - safe conversion for test fee rates
							Satoshis: int64(feeRate),
							Bytes:    1,
						},
					},
				},
			}
			payloadBytes, _ := json.Marshal(payload)
			resp := TAALFeeQuoteResponse{Payload: string(payloadBytes)}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

// Ensure mockServerConfig and mockMultiRouteServer are used (for godot linter).
var (
	_ = mockServerConfig{}
	_ = mockMultiRouteServer
)
