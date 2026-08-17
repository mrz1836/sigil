// Package testutil provides shared fixtures and builders for Sigil's test
// suites. It lives in regular .go files (not _test.go) so that many packages'
// _test.go files can import it, yet because only test files import it, the
// package is never linked into the production sigil binary.
package testutil

import "github.com/mrz1836/sigil/internal/chain"

// TestMnemonic is the canonical all-"abandon" BIP-39 test vector: the standard
// 12-word, zero-entropy mnemonic used across the BIP-39/44 reference tests. It
// is a well-known public value, safe to hardcode, and is reused by wallet,
// derivation, and CLI tests. Never use it for a real wallet.
const TestMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// UTXOOption configures a chain.UTXO built by NewTestUTXO.
type UTXOOption func(*chain.UTXO)

// NewTestUTXO builds a chain.UTXO with valid-looking defaults (1 confirmed
// 100,000-sat P2PKH output), overridden by any options. It centralizes the
// chain.UTXO{...} literals scattered through the transaction/utxostore tests.
func NewTestUTXO(opts ...UTXOOption) chain.UTXO {
	u := chain.UTXO{
		TxID:          "0000000000000000000000000000000000000000000000000000000000000001",
		Vout:          0,
		Amount:        100_000,
		ScriptPubKey:  "76a914000000000000000000000000000000000000000088ac",
		Address:       "1TestAddress00000000000000000000000",
		Confirmations: 6,
	}
	for _, opt := range opts {
		opt(&u)
	}
	return u
}

// WithUTXOTxID sets the UTXO transaction ID.
func WithUTXOTxID(txID string) UTXOOption {
	return func(u *chain.UTXO) { u.TxID = txID }
}

// WithUTXOVout sets the UTXO output index.
func WithUTXOVout(vout uint32) UTXOOption {
	return func(u *chain.UTXO) { u.Vout = vout }
}

// WithUTXOAmount sets the UTXO amount in satoshis.
func WithUTXOAmount(amount uint64) UTXOOption {
	return func(u *chain.UTXO) { u.Amount = amount }
}

// WithUTXOAddress sets the UTXO owning address.
func WithUTXOAddress(address string) UTXOOption {
	return func(u *chain.UTXO) { u.Address = address }
}

// WithUTXOScript sets the UTXO scriptPubKey hex.
func WithUTXOScript(scriptPubKey string) UTXOOption {
	return func(u *chain.UTXO) { u.ScriptPubKey = scriptPubKey }
}

// WithUTXOConfirmations sets the UTXO confirmation count.
func WithUTXOConfirmations(confirmations uint32) UTXOOption {
	return func(u *chain.UTXO) { u.Confirmations = confirmations }
}
