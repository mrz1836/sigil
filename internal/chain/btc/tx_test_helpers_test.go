package btc

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// Deterministic fixed-hex private keys reused across tests (no wallet derivation).
const (
	fixedKeyHex  = "e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35"
	fixedKey2Hex = "0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d"

	// fixedTxID is a fixed 64-hex previous-output txid.
	fixedTxID  = "6dfb16dd580698242bcfd8e433d557ed8c642272a368894de27292a8844a4e75"
	fixedTxID2 = "0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098"

	// Known BIP173 witness programs (20- and 32-byte).
	program20Hex = "751e76e8199196d454941c45d1b3a323f1433bd6"
	program32Hex = "1863143c14c5166804bd19203356da136c985678cd4d27a1b8c6329604903262"
)

// Package-level test sentinels.
var errTestProvider = errors.New("test provider error")

// mustDecodeHex decodes a hex string, failing the test on error.
func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// p2pkhAddressFromPubKey encodes a compressed pubkey as a network P2PKH address.
func p2pkhAddressFromPubKey(compressed []byte) string {
	return bitcoin.Base58CheckEncode(versionP2PKHMainnet, bitcoin.Hash160(compressed))
}

// p2pkhAddressForKey returns the mainnet P2PKH address for a private key.
func p2pkhAddressForKey(t *testing.T, keyHex string) string {
	t.Helper()
	_, pub := ec.PrivateKeyFromBytes(mustDecodeHex(t, keyHex))
	return p2pkhAddressFromPubKey(pub.Compressed())
}

// mustFuzzKey decodes a fixed hex key for fuzz seeding (ignores errors).
func mustFuzzKey(hexStr string) []byte {
	b, _ := hex.DecodeString(hexStr)
	return b
}

// recipientAddresses returns one mainnet address of each standard type, built
// from the fixed known witness programs / hashes.
func recipientAddresses(t *testing.T) map[AddrType]string {
	t.Helper()
	hash20 := mustDecodeHex(t, program20Hex)
	hash32 := mustDecodeHex(t, program32Hex)

	p2wpkh, err := bitcoin.SegwitEncode(hrpMainnet, 0, hash20)
	require.NoError(t, err)
	p2wsh, err := bitcoin.SegwitEncode(hrpMainnet, 0, hash32)
	require.NoError(t, err)

	return map[AddrType]string{
		AddrP2PKH:  bitcoin.Base58CheckEncode(versionP2PKHMainnet, hash20),
		AddrP2SH:   bitcoin.Base58CheckEncode(versionP2SHMainnet, hash20),
		AddrP2WPKH: p2wpkh,
		AddrP2WSH:  p2wsh,
	}
}

// extractSigAndPubKey parses a signed input's unlocking script into its DER
// signature (sans the trailing sighash flag byte), the flag byte, and the pubkey.
func extractSigAndPubKey(t *testing.T, tx *transaction.Transaction, inputIdx int) (derSig []byte, flag byte, pubKey []byte) {
	t.Helper()
	us := tx.Inputs[inputIdx].UnlockingScript
	require.NotNil(t, us)
	chunks, err := us.Chunks()
	require.NoError(t, err)
	require.Len(t, chunks, 2, "P2PKH scriptSig must be <sig> <pubkey>")

	sigWithFlag := chunks[0].Data
	require.NotEmpty(t, sigWithFlag)
	flag = sigWithFlag[len(sigWithFlag)-1]
	derSig = sigWithFlag[:len(sigWithFlag)-1]
	pubKey = chunks[1].Data
	return derSig, flag, pubKey
}

// mockEsplora is a Pattern-A func-field fake EsploraProvider for client tests.
type mockEsplora struct {
	statsFn func(ctx context.Context, address string) (*AddressStats, error)
	utxosFn func(ctx context.Context, address string) ([]EsploraUTXO, error)
	feesFn  func(ctx context.Context) (*FeeEstimates, error)
}

func (m *mockEsplora) AddressStats(ctx context.Context, address string) (*AddressStats, error) {
	return m.statsFn(ctx, address)
}

func (m *mockEsplora) AddressUTXOs(ctx context.Context, address string) ([]EsploraUTXO, error) {
	return m.utxosFn(ctx, address)
}

func (m *mockEsplora) FeeEstimates(ctx context.Context) (*FeeEstimates, error) {
	return m.feesFn(ctx)
}

// recordingBroadcaster captures the last raw tx hex and returns a fixed result.
type recordingBroadcaster struct {
	name      string
	lastHex   string
	returnID  string
	returnErr error
}

func (b *recordingBroadcaster) Name() string { return b.name }

func (b *recordingBroadcaster) Broadcast(_ context.Context, rawTxHex string) (string, error) {
	b.lastHex = rawTxHex
	if b.returnErr != nil {
		return "", b.returnErr
	}
	return b.returnID, nil
}

// singleInputBuilder builds a one-input mainnet TxBuilder funded by the fixed
// key's P2PKH address (100k-sat input), paying 90k sats to `to` at 2 sat/vB.
func singleInputBuilder(t *testing.T, to string) *TxBuilder {
	t.Helper()
	from := p2pkhAddressForKey(t, fixedKeyHex)

	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)
	b.SetFeeRate(2)
	require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 100_000, Address: from}))
	require.NoError(t, b.AddOutput(to, 90_000))
	return b
}
