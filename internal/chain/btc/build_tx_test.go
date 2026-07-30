package btc

import (
	"encoding/hex"
	"math/big"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownAnswerRawTxHex is the deterministic serialization of the fixed-key,
// fixed-input transaction built in TestBuildRawTransaction_KnownAnswer. Pinned as
// a regression guard against signing/serialization changes in dependencies.
const knownAnswerRawTxHex = "0100000001754e4a84a89272e24d8968a37222648ced57d533e4d8cf2b24980658dd16fb6d000000006a473044022003e119281b7761ce4005560229d0132c77d5cd7d6a4fadcbe0959361fa1c040502205810769d72c809dcd35446635d6dd2ce9825fd32031c6aa04bc23884d0c80ecc01210339a36013301597daef41fbe593a02cc513d0b55527ec2df1050e2e8ff49c85c2ffffffff01905f0100000000001976a914751e76e8199196d454941c45d1b3a323f1433bd688ac00000000"

// TestBuildRawTransaction_LegacySighashByte is the load-bearing proof that BTC
// signs with legacy SIGHASH_ALL (0x01), NOT BSV's AllForkID (0x41).
func TestBuildRawTransaction_LegacySighashByte(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]
	builder := singleInputBuilder(t, to)

	tx, err := buildSignedTx(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)

	_, flag, _ := extractSigAndPubKey(t, tx, 0)
	assert.Equal(t, byte(sighash.All), flag, "sighash flag must be 0x01 (legacy ALL)")
	assert.Equal(t, byte(0x01), flag)
	assert.NotEqual(t, byte(sighash.AllForkID), flag, "must NOT be 0x41 (BSV ForkID)")
}

// TestBuildRawTransaction_LowS_StrictDER guards the BIP62/BIP66/BIP146 relay
// safety: signatures must be strict-DER and low-S. This is a regression guard
// against a future go-sdk bump that might drop low-S normalization.
func TestBuildRawTransaction_LowS_StrictDER(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]
	builder := singleInputBuilder(t, to)

	tx, err := buildSignedTx(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)

	derSig, _, _ := extractSigAndPubKey(t, tx, 0)

	// Strict-DER parse (rejects non-canonical encodings).
	sig, err := ec.ParseDERSignature(derSig)
	require.NoError(t, err, "signature must be strict DER")

	// Low-S: S <= N/2.
	halfN := new(big.Int).Rsh(ec.S256().N, 1)
	assert.LessOrEqual(t, sig.S.Cmp(halfN), 0, "S must be <= N/2 (low-S)")
}

// TestBuildRawTransaction_SignatureVerifies recomputes the legacy sighash and
// verifies the emitted signature against the input's public key — the offline
// proof that the signature is valid for the legacy algorithm.
func TestBuildRawTransaction_SignatureVerifies(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2WPKH] // pay a bech32 recipient
	builder := singleInputBuilder(t, to)

	tx, err := buildSignedTx(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)

	derSig, flag, pubKeyBytes := extractSigAndPubKey(t, tx, 0)
	require.Equal(t, byte(sighash.All), flag)

	// Recompute the sighash that was signed (source outputs are still attached).
	sh, err := tx.CalcInputSignatureHash(0, sighash.All)
	require.NoError(t, err)

	sig, err := ec.ParseDERSignature(derSig)
	require.NoError(t, err)
	pubKey, err := ec.ParsePubKey(pubKeyBytes)
	require.NoError(t, err)

	assert.True(t, ec.Verify(sh, sig, pubKey.ToECDSA()), "signature must verify against the legacy sighash")
}

// TestBuildRawTransaction_PaysAllRecipientTypes proves a P2PKH-funded tx can pay
// every standard destination type, and asserts each output's locking-script hex.
func TestBuildRawTransaction_PaysAllRecipientTypes(t *testing.T) {
	t.Parallel()

	addrs := recipientAddresses(t)
	from := p2pkhAddressForKey(t, fixedKeyHex)

	builder := NewTxBuilder()
	builder.SetNetwork(NetworkMainnet)
	builder.SetFeeRate(2)
	require.NoError(t, builder.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 1_000_000, Address: from}))

	// One output of each type.
	require.NoError(t, builder.AddOutput(addrs[AddrP2PKH], 100_000))
	require.NoError(t, builder.AddOutput(addrs[AddrP2SH], 100_000))
	require.NoError(t, builder.AddOutput(addrs[AddrP2WPKH], 100_000))
	require.NoError(t, builder.AddOutput(addrs[AddrP2WSH], 100_000))

	tx, err := buildSignedTx(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)
	require.Len(t, tx.Outputs, 4)

	wantScripts := []string{
		"76a914751e76e8199196d454941c45d1b3a323f1433bd688ac",                   // P2PKH
		"a914751e76e8199196d454941c45d1b3a323f1433bd687",                       // P2SH
		"0014751e76e8199196d454941c45d1b3a323f1433bd6",                         // P2WPKH
		"00201863143c14c5166804bd19203356da136c985678cd4d27a1b8c6329604903262", // P2WSH
	}
	for i, want := range wantScripts {
		assert.Equal(t, want, hex.EncodeToString(*tx.Outputs[i].LockingScript), "output %d script", i)
	}
}

// TestBuildRawTransaction_NonWitnessSerialization asserts standard (non-segwit)
// serialization: no witness marker/flag after the version field.
func TestBuildRawTransaction_NonWitnessSerialization(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]
	builder := singleInputBuilder(t, to)

	rawTx, err := BuildRawTransaction(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)
	require.Greater(t, len(rawTx), 10)

	// Bytes [0:4] are the version; byte [4] is the input-count varint. A segwit
	// tx would place a 0x00 marker there — assert it is the (non-zero) input count.
	assert.NotEqual(t, byte(0x00), rawTx[4], "no segwit marker byte")
	assert.Equal(t, byte(0x01), rawTx[4], "one input (varint 0x01)")

	// Re-serialization round-trips exactly.
	parsed, err := transaction.NewTransactionFromBytes(rawTx)
	require.NoError(t, err)
	assert.Equal(t, rawTx, parsed.Bytes())
	assert.Len(t, parsed.Inputs, 1)
	assert.Len(t, parsed.Outputs, 1)
}

// TestBuildRawTransaction_Deterministic proves RFC6979 determinism: identical
// inputs yield byte-identical transactions across builds.
func TestBuildRawTransaction_Deterministic(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]

	build := func() []byte {
		b := singleInputBuilder(t, to)
		raw, err := BuildRawTransaction(b, mustDecodeHex(t, fixedKeyHex))
		require.NoError(t, err)
		return raw
	}

	first, second := build(), build()
	assert.Equal(t, first, second, "signing must be deterministic (RFC6979)")
}

// TestBuildRawTransaction_KnownAnswer pins the full raw-tx hex as a regression
// guard. The value is deterministic (RFC6979 + low-S + fixed inputs); a change
// here signals a serialization or signing-algorithm change in a dependency.
func TestBuildRawTransaction_KnownAnswer(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]
	builder := singleInputBuilder(t, to)

	rawTx, err := BuildRawTransaction(builder, mustDecodeHex(t, fixedKeyHex))
	require.NoError(t, err)

	const wantHex = knownAnswerRawTxHex
	assert.Equal(t, wantHex, hex.EncodeToString(rawTx))
}

// TestBuildRawTransactionMultiKey_TwoAddresses signs inputs from two distinct
// addresses/keys in one transaction.
func TestBuildRawTransactionMultiKey_TwoAddresses(t *testing.T) {
	t.Parallel()

	addr1 := p2pkhAddressForKey(t, fixedKeyHex)
	addr2 := p2pkhAddressForKey(t, fixedKey2Hex)
	to := recipientAddresses(t)[AddrP2PKH]

	builder := NewTxBuilder()
	builder.SetNetwork(NetworkMainnet)
	builder.SetFeeRate(2)
	require.NoError(t, builder.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 60_000, Address: addr1}))
	require.NoError(t, builder.AddInput(UTXO{TxID: fixedTxID2, Vout: 1, Amount: 60_000, Address: addr2}))
	require.NoError(t, builder.AddOutput(to, 100_000))

	keyMap := map[string][]byte{
		addr1: mustDecodeHex(t, fixedKeyHex),
		addr2: mustDecodeHex(t, fixedKey2Hex),
	}
	tx, err := buildSignedTxMultiKey(builder, keyMap)
	require.NoError(t, err)
	require.Len(t, tx.Inputs, 2)

	// Both inputs signed with legacy SIGHASH_ALL and verify.
	for i := range tx.Inputs {
		derSig, flag, pubKeyBytes := extractSigAndPubKey(t, tx, i)
		assert.Equal(t, byte(sighash.All), flag)
		sh, shErr := tx.CalcInputSignatureHash(uint32(i), sighash.All)
		require.NoError(t, shErr)
		sig, pErr := ec.ParseDERSignature(derSig)
		require.NoError(t, pErr)
		pubKey, kErr := ec.ParsePubKey(pubKeyBytes)
		require.NoError(t, kErr)
		assert.True(t, ec.Verify(sh, sig, pubKey.ToECDSA()), "input %d must verify", i)
	}
}

func TestBuildRawTransaction_Errors(t *testing.T) {
	t.Parallel()

	to := recipientAddresses(t)[AddrP2PKH]
	from := p2pkhAddressForKey(t, fixedKeyHex)
	key := mustDecodeHex(t, fixedKeyHex)

	t.Run("no inputs", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddOutput(to, 1000))
		_, err := BuildRawTransaction(b, key)
		assert.ErrorIs(t, err, ErrNoInputs)
	})

	t.Run("no outputs", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 1000, Address: from}))
		_, err := BuildRawTransaction(b, key)
		assert.ErrorIs(t, err, ErrNoOutputs)
	})

	t.Run("bad key length", func(t *testing.T) {
		t.Parallel()
		b := singleInputBuilder(t, to)
		_, err := BuildRawTransaction(b, []byte{0x01, 0x02})
		assert.ErrorIs(t, err, ErrInvalidPrivateKey)
	})

	t.Run("bad txid", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddInput(UTXO{TxID: "not-hex", Vout: 0, Amount: 100_000, Address: from}))
		require.NoError(t, b.AddOutput(to, 90_000))
		_, err := BuildRawTransaction(b, key)
		assert.ErrorIs(t, err, ErrInvalidTxID)
	})

	t.Run("multikey missing key for address", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 100_000, Address: from}))
		require.NoError(t, b.AddOutput(to, 90_000))
		_, err := BuildRawTransactionMultiKey(b, map[string][]byte{"1SomeOtherAddr": key})
		assert.ErrorIs(t, err, ErrSigningFailed)
	})
}

// fuzzSignedTx builds a signed single-input tx for a fuzzed key, returning ok=false
// when the key/inputs are unusable (the fuzz contract is only "no panic").
func fuzzSignedTx(key []byte) (*transaction.Transaction, bool) {
	if len(key) != 32 {
		return nil, false
	}
	priv, pub := ec.PrivateKeyFromBytes(key)
	if priv == nil || pub == nil {
		return nil, false
	}
	from := p2pkhAddressFromPubKey(pub.Compressed())

	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)
	if b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 100_000, Address: from}) != nil {
		return nil, false
	}
	if b.AddOutput("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", 90_000) != nil {
		return nil, false
	}
	tx, err := buildSignedTx(b, key)
	if err != nil {
		return nil, false
	}
	return tx, true
}

// FuzzSign_AlwaysLowS fuzzes the signing key and asserts every emitted signature
// is strict-DER and low-S, and that signing never panics.
func FuzzSign_AlwaysLowS(f *testing.F) {
	f.Add(mustFuzzKey(fixedKeyHex))
	f.Add(mustFuzzKey(fixedKey2Hex))

	halfN := new(big.Int).Rsh(ec.S256().N, 1)

	f.Fuzz(func(t *testing.T, key []byte) {
		tx, ok := fuzzSignedTx(key)
		if !ok {
			return
		}
		derSig, flag, _ := extractSigAndPubKey(t, tx, 0)
		require.Equal(t, byte(0x01), flag)
		sig, err := ec.ParseDERSignature(derSig)
		require.NoError(t, err)
		require.LessOrEqual(t, sig.S.Cmp(halfN), 0, "S must be low-S")
	})
}
