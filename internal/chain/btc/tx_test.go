package btc

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestGetLockingScript(t *testing.T) {
	t.Parallel()

	// Explicit scriptPubKey hex is used verbatim.
	const p2pkhHex = "76a914751e76e8199196d454941c45d1b3a323f1433bd688ac"
	s, err := getLockingScript(UTXO{ScriptPubKey: p2pkhHex}, NetworkMainnet)
	require.NoError(t, err)
	assert.Equal(t, p2pkhHex, hex.EncodeToString(*s))

	// Absent a scriptPubKey, the P2PKH script is rebuilt from the address.
	addr, _ := encodeAddr(AddrP2PKH, NetworkMainnet)
	s2, err := getLockingScript(UTXO{Address: addr}, NetworkMainnet)
	require.NoError(t, err)
	assert.Equal(t, p2pkhHex, hex.EncodeToString(*s2))

	// Neither present → error.
	_, err = getLockingScript(UTXO{}, NetworkMainnet)
	require.ErrorIs(t, err, ErrMissingLockingScript)
}

func TestTxBuilder_AddOutput_DustRejected(t *testing.T) {
	t.Parallel()

	to, _ := encodeAddr(AddrP2PKH, NetworkMainnet)
	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)

	err := b.AddOutput(to, chain.BTC.DustLimit()-1) // 545 < 546
	require.ErrorIs(t, err, ErrDustOutput)

	require.NoError(t, b.AddOutput(to, chain.BTC.DustLimit())) // exactly 546 is OK
}

func TestTxBuilder_AddOutput_CrossNetworkRejected(t *testing.T) {
	t.Parallel()

	testnetAddr, _ := encodeAddr(AddrP2PKH, NetworkTestnet)
	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)

	err := b.AddOutput(testnetAddr, 100_000)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidAddress)
}

func TestTxBuilder_Validate(t *testing.T) {
	t.Parallel()

	to, _ := encodeAddr(AddrP2PKH, NetworkMainnet)
	from := p2pkhAddressForKey(t, fixedKeyHex)

	t.Run("no inputs", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddOutput(to, 1000))
		assert.ErrorIs(t, b.Validate(), ErrNoInputs)
	})

	t.Run("no outputs", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 1000, Address: from}))
		assert.ErrorIs(t, b.Validate(), ErrNoOutputs)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		b.SetFeeRate(10)
		require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 1000, Address: from}))
		require.NoError(t, b.AddOutput(to, 999)) // fee pushes past inputs
		assert.ErrorIs(t, b.Validate(), ErrInsufficientFunds)
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		b := NewTxBuilder()
		b.SetNetwork(NetworkMainnet)
		b.SetFeeRate(1)
		require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 100_000, Address: from}))
		require.NoError(t, b.AddOutput(to, 90_000))
		assert.NoError(t, b.Validate())
	})
}

func TestTxBuilder_TotalOverflow(t *testing.T) {
	t.Parallel()

	from := p2pkhAddressForKey(t, fixedKeyHex)
	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)
	require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: math.MaxUint64, Address: from}))
	require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID2, Vout: 1, Amount: 1, Address: from}))

	_, err := b.TotalInputAmount()
	assert.ErrorIs(t, err, ErrAmountOverflow)
}

func TestTxBuilder_CalculateFee_ExactPerOutputType(t *testing.T) {
	t.Parallel()

	from := p2pkhAddressForKey(t, fixedKeyHex)
	p2wsh, _ := encodeAddr(AddrP2WSH, NetworkMainnet)
	change, _ := encodeAddr(AddrP2PKH, NetworkMainnet)

	b := NewTxBuilder()
	b.SetNetwork(NetworkMainnet)
	require.NoError(t, b.AddInput(UTXO{TxID: fixedTxID, Vout: 0, Amount: 1_000_000, Address: from}))
	require.NoError(t, b.AddOutput(p2wsh, 500_000))  // 43-vByte output
	require.NoError(t, b.AddOutput(change, 400_000)) // 34-vByte output

	const rate = 25
	// 10 overhead + 148 input + 43 (P2WSH) + 34 (P2PKH change) = 235 vB.
	assert.Equal(t, uint64(235*rate), b.CalculateFee(rate))
}

func TestComputeChange(t *testing.T) {
	t.Parallel()

	const rate = 10
	const p2pkhRecipient = 25

	t.Run("normal change above dust", func(t *testing.T) {
		t.Parallel()
		// fee = (10 + 148 + 34 + 34) * 10 = 2260. change = 100000 - 50000 - 2260.
		change, err := computeChange(100_000, 50_000, 1, p2pkhRecipient, rate)
		require.NoError(t, err)
		assert.Equal(t, uint64(100_000-50_000-2260), change)
	})

	t.Run("change below dust dropped to zero", func(t *testing.T) {
		t.Parallel()
		// Pick totals so the change would be < 546.
		feeWithChange := estimateFeeExact(1, []int{p2pkhRecipient, p2pkhScriptLen}, rate)
		total := 50_000 + feeWithChange + 100 // change would be 100 (< 546)
		change, err := computeChange(total, 50_000, 1, p2pkhRecipient, rate)
		require.NoError(t, err)
		assert.Zero(t, change)
	})

	t.Run("insufficient", func(t *testing.T) {
		t.Parallel()
		_, err := computeChange(50_100, 50_000, 1, p2pkhRecipient, rate)
		assert.ErrorIs(t, err, ErrInsufficientFunds)
	})
}

func TestCalculateSweepAmount(t *testing.T) {
	t.Parallel()

	const rate = 5
	const recipientScriptLen = 25 // P2PKH

	t.Run("normal", func(t *testing.T) {
		t.Parallel()
		total := uint64(1_000_000)
		fee := estimateFeeExact(2, []int{recipientScriptLen}, rate)
		amount, err := CalculateSweepAmount(total, 2, recipientScriptLen, rate)
		require.NoError(t, err)
		assert.Equal(t, total-fee, amount)
	})

	t.Run("fee exceeds total", func(t *testing.T) {
		t.Parallel()
		_, err := CalculateSweepAmount(10, 1, recipientScriptLen, rate)
		assert.ErrorIs(t, err, ErrSweepInsufficientFunds)
	})

	t.Run("remaining below dust", func(t *testing.T) {
		t.Parallel()
		fee := estimateFeeExact(1, []int{recipientScriptLen}, rate)
		total := fee + 100 // leaves 100 < 546
		_, err := CalculateSweepAmount(total, 1, recipientScriptLen, rate)
		assert.ErrorIs(t, err, ErrSweepInsufficientFunds)
	})

	t.Run("p2wsh recipient sized larger", func(t *testing.T) {
		t.Parallel()
		total := uint64(1_000_000)
		feeP2PKH := estimateFeeExact(1, []int{25}, rate)
		feeP2WSH := estimateFeeExact(1, []int{34}, rate)
		amtP2PKH, err := CalculateSweepAmount(total, 1, 25, rate)
		require.NoError(t, err)
		amtP2WSH, err := CalculateSweepAmount(total, 1, 34, rate)
		require.NoError(t, err)
		// Larger recipient script → larger fee → smaller sweepable amount.
		assert.Equal(t, feeP2WSH-feeP2PKH, amtP2PKH-amtP2WSH)
	})
}
