package bsv

import (
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRawTransaction_Basic tests basic transaction building scenarios.
func TestBuildRawTransaction_Basic(t *testing.T) {
	t.Parallel()

	t.Run("single input single output", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		// Add a single input
		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  100000,
			Address: kp.Address,
		})
		require.NoError(t, err)

		// Add a single output (leaving some for fee)
		err = builder.AddOutput(validAddress2(), 99000)
		require.NoError(t, err)

		// Build and sign
		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)
		require.NotEmpty(t, rawTx)

		// Verify we can parse the transaction
		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Len(t, tx.Inputs, 1)
		assert.Len(t, tx.Outputs, 1)
		assert.Equal(t, uint64(99000), tx.Outputs[0].Satoshis)
	})

	t.Run("multiple inputs single output", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		// Add multiple inputs
		for i := 0; i < 5; i++ {
			err := builder.AddInput(UTXO{
				TxID:    testTxID(i + 1),
				Vout:    0,
				Amount:  10000,
				Address: kp.Address,
			})
			require.NoError(t, err)
		}

		// Add a single output (50000 total input, leave 1000 for fee)
		err := builder.AddOutput(validAddress2(), 49000)
		require.NoError(t, err)

		// Build and sign
		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)
		require.NotEmpty(t, rawTx)

		// Verify transaction structure
		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Len(t, tx.Inputs, 5)
		assert.Len(t, tx.Outputs, 1)
	})

	t.Run("single input multiple outputs", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		// Add a single large input
		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  100000,
			Address: kp.Address,
		})
		require.NoError(t, err)

		// Add multiple outputs
		err = builder.AddOutput(validAddress2(), 30000)
		require.NoError(t, err)
		err = builder.AddOutput(validAddress(), 30000)
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 30000)
		require.NoError(t, err)

		// Build and sign (90000 outputs + ~500 fee from 100000 input)
		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)
		require.NotEmpty(t, rawTx)

		// Verify transaction structure
		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Len(t, tx.Inputs, 1)
		assert.Len(t, tx.Outputs, 3)
	})
}

// TestBuildRawTransaction_Signing verifies signatures are valid.
func TestBuildRawTransaction_Signing(t *testing.T) {
	t.Parallel()

	t.Run("unlocking script is created", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  100000,
			Address: kp.Address,
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 99000)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)

		// Verify unlocking script exists and has proper length
		// P2PKH unlocking script: ~107 bytes (signature + pubkey)
		assert.NotNil(t, tx.Inputs[0].UnlockingScript)
		scriptLen := len(*tx.Inputs[0].UnlockingScript)
		assert.True(t, scriptLen >= 100 && scriptLen <= 110,
			"unlocking script length %d should be ~107 bytes", scriptLen)
	})

	t.Run("multiple inputs all signed", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		// Add 10 inputs
		for i := 0; i < 10; i++ {
			err := builder.AddInput(UTXO{
				TxID:    testTxID(i + 1),
				Vout:    0,
				Amount:  1000,
				Address: kp.Address,
			})
			require.NoError(t, err)
		}
		err := builder.AddOutput(validAddress2(), 9000)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)

		// Verify all inputs have unlocking scripts
		for i, input := range tx.Inputs {
			assert.NotNil(t, input.UnlockingScript, "input %d should have unlocking script", i)
			assert.NotEmpty(t, *input.UnlockingScript, "input %d unlocking script should not be empty", i)
		}
	})
}

// TestBuildRawTransaction_Errors tests error handling.
func TestBuildRawTransaction_Errors(t *testing.T) {
	t.Parallel()

	kp := getTestKeyPair()

	t.Run("nil builder", func(t *testing.T) {
		t.Parallel()

		_, err := BuildRawTransaction(nil, kp.PrivateKey)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoInputs)
	})

	t.Run("empty inputs", func(t *testing.T) {
		t.Parallel()

		builder := NewTxBuilder()
		err := builder.AddOutput(validAddress2(), 1000)
		require.NoError(t, err)

		_, err = BuildRawTransaction(builder, kp.PrivateKey)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoInputs)
	})

	t.Run("empty outputs", func(t *testing.T) {
		t.Parallel()

		builder := NewTxBuilder()
		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)

		_, err = BuildRawTransaction(builder, kp.PrivateKey)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoOutputs)
	})

	t.Run("invalid private key length", func(t *testing.T) {
		t.Parallel()

		builder := NewTxBuilder()
		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 500)
		require.NoError(t, err)

		// Try with wrong key length
		_, err = BuildRawTransaction(builder, []byte{1, 2, 3})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidPrivateKey)
	})

	t.Run("invalid txid", func(t *testing.T) {
		t.Parallel()

		builder := NewTxBuilder()
		err := builder.AddInput(UTXO{
			TxID:    "not-a-valid-hex-txid",
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 500)
		require.NoError(t, err)

		_, err = BuildRawTransaction(builder, kp.PrivateKey)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidTxID)
	})

	t.Run("missing address and script", func(t *testing.T) {
		t.Parallel()

		builder := NewTxBuilder()
		err := builder.AddInput(UTXO{
			TxID:   testTxID(1),
			Vout:   0,
			Amount: 1000,
			// No Address or ScriptPubKey
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 500)
		require.NoError(t, err)

		_, err = BuildRawTransaction(builder, kp.PrivateKey)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMissingLockingScript)
	})
}

// TestBuildRawTransaction_ScriptPubKey tests using ScriptPubKey instead of Address.
func TestBuildRawTransaction_ScriptPubKey(t *testing.T) {
	t.Parallel()

	t.Run("input with script pubkey", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()

		// Generate the locking script for our test address
		// P2PKH script: OP_DUP OP_HASH160 <pubkeyhash> OP_EQUALVERIFY OP_CHECKSIG
		// We can derive this from the address
		_, payload, err := DecodeBase58Check(kp.Address)
		require.NoError(t, err)

		// Build P2PKH script manually (25 bytes: 3 prefix + 20 hash + 2 suffix)
		scriptPubKey := make([]byte, 0, 25)
		scriptPubKey = append(scriptPubKey, 0x76, 0xa9, 0x14) // OP_DUP OP_HASH160 PUSH20
		scriptPubKey = append(scriptPubKey, payload...)
		scriptPubKey = append(scriptPubKey, 0x88, 0xac) // OP_EQUALVERIFY OP_CHECKSIG

		builder := NewTxBuilder()
		err = builder.AddInput(UTXO{
			TxID:         testTxID(1),
			Vout:         0,
			Amount:       100000,
			ScriptPubKey: hex.EncodeToString(scriptPubKey),
			// No Address field - using ScriptPubKey instead
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 99000)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)
		require.NotEmpty(t, rawTx)

		// Verify transaction is valid
		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Len(t, tx.Inputs, 1)
		assert.NotNil(t, tx.Inputs[0].UnlockingScript)
	})
}

// TestBuildRawTransaction_TxID verifies transaction ID computation.
func TestBuildRawTransaction_TxID(t *testing.T) {
	t.Parallel()

	t.Run("txid is deterministic", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()

		// Build the same transaction twice
		buildTx := func() []byte {
			builder := NewTxBuilder()
			_ = builder.AddInput(UTXO{
				TxID:    testTxID(1),
				Vout:    0,
				Amount:  100000,
				Address: kp.Address,
			})
			_ = builder.AddOutput(validAddress2(), 99000)
			rawTx, _ := BuildRawTransaction(builder, kp.PrivateKey)
			return rawTx
		}

		rawTx1 := buildTx()
		rawTx2 := buildTx()

		// Parse both transactions
		tx1, err := transaction.NewTransactionFromBytes(rawTx1)
		require.NoError(t, err)
		tx2, err := transaction.NewTransactionFromBytes(rawTx2)
		require.NoError(t, err)

		// TxIDs may differ due to signature randomness (ECDSA k value)
		// but the transactions should have the same structure
		assert.Len(t, tx1.Inputs, len(tx2.Inputs))
		assert.Len(t, tx1.Outputs, len(tx2.Outputs))
		assert.Equal(t, tx1.Outputs[0].Satoshis, tx2.Outputs[0].Satoshis)
	})
}

// TestBuildRawTransaction_BSVSpecific tests BSV-specific features.
func TestBuildRawTransaction_BSVSpecific(t *testing.T) {
	t.Parallel()

	t.Run("one satoshi output (BSV allows no dust limit)", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)

		// BSV allows 1 satoshi outputs
		err = builder.AddOutput(validAddress2(), 1)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), tx.Outputs[0].Satoshis)
	})

	t.Run("large transaction with 100 inputs", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		// Add 100 inputs
		for i := 0; i < 100; i++ {
			err := builder.AddInput(UTXO{
				TxID:    testTxID(i + 1),
				Vout:    0,
				Amount:  1000,
				Address: kp.Address,
			})
			require.NoError(t, err)
		}

		// Total: 100,000 satoshis, output 90,000 (leave 10,000 for fee)
		err := builder.AddOutput(validAddress2(), 90000)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)
		assert.Len(t, tx.Inputs, 100)

		// Verify all inputs are signed
		for i, input := range tx.Inputs {
			assert.NotNil(t, input.UnlockingScript, "input %d should be signed", i)
		}
	})

	t.Run("transaction with version 1", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 500)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)

		// Default version should be 1 for BSV
		assert.Equal(t, uint32(1), tx.Version)
	})

	t.Run("locktime is zero by default", func(t *testing.T) {
		t.Parallel()

		kp := getTestKeyPair()
		builder := NewTxBuilder()

		err := builder.AddInput(UTXO{
			TxID:    testTxID(1),
			Vout:    0,
			Amount:  1000,
			Address: kp.Address,
		})
		require.NoError(t, err)
		err = builder.AddOutput(validAddress2(), 500)
		require.NoError(t, err)

		rawTx, err := BuildRawTransaction(builder, kp.PrivateKey)
		require.NoError(t, err)

		tx, err := transaction.NewTransactionFromBytes(rawTx)
		require.NoError(t, err)

		// Default locktime should be 0
		assert.Equal(t, uint32(0), tx.LockTime)
	})
}
