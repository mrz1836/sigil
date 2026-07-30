package chain

import "math/big"

// Balance is the native-coin balance of an address on a Bitcoin-family chain
// (BTC, BSV). The BTC and BSV clients both return this shape; ETH uses its own
// balance type because it additionally carries a token contract field.
type Balance struct {
	// Address is the queried address.
	Address string
	// Amount is the confirmed balance in the chain's smallest unit (satoshis).
	Amount *big.Int
	// Unconfirmed is the mempool balance delta in the smallest unit (may be negative).
	Unconfirmed *big.Int
	// Symbol is the ticker symbol (e.g. "BTC", "BSV").
	Symbol string
	// Decimals is the number of decimal places for the smallest unit.
	Decimals int
}
