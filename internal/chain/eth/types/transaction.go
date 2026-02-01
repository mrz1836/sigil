// Package ethtypes provides Ethereum transaction types without go-ethereum.
package ethtypes

import (
	"encoding/hex"
	"math/big"

	ethcrypto "github.com/mrz1836/sigil/internal/chain/eth/crypto"
	"github.com/mrz1836/sigil/internal/chain/eth/rlp"
)

// LegacyTx represents a legacy (pre-EIP-1559) Ethereum transaction.
type LegacyTx struct {
	Nonce    uint64
	GasPrice *big.Int
	GasLimit uint64
	To       []byte // 20 bytes, nil for contract creation
	Value    *big.Int
	Data     []byte

	// Signature values (set after signing)
	V *big.Int
	R *big.Int
	S *big.Int
}

// NewLegacyTx creates a new legacy transaction.
func NewLegacyTx(nonce uint64, to []byte, value *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *LegacyTx {
	return &LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		GasLimit: gasLimit,
		To:       to,
		Value:    value,
		Data:     data,
	}
}

// SigningHash returns the hash to be signed for EIP-155 replay protection.
func (tx *LegacyTx) SigningHash(chainID *big.Int) []byte {
	encoded := rlp.EncodeTransactionForSigning(
		tx.Nonce,
		tx.GasPrice,
		tx.GasLimit,
		tx.To,
		tx.Value,
		tx.Data,
		chainID,
	)
	return ethcrypto.Keccak256(encoded)
}

// Sign signs the transaction with the given private key and chain ID.
// Uses EIP-155 signature format for replay protection.
func (tx *LegacyTx) Sign(privateKey []byte, chainID *big.Int) error {
	hash := tx.SigningHash(chainID)

	sig, err := ethcrypto.Sign(hash, privateKey)
	if err != nil {
		return err
	}

	// Extract R, S, V from signature
	tx.R = new(big.Int).SetBytes(sig[0:32])
	tx.S = new(big.Int).SetBytes(sig[32:64])

	// EIP-155: v = recovery_id + chainID * 2 + 35
	v := int64(sig[64]) + chainID.Int64()*2 + 35
	tx.V = big.NewInt(v)

	return nil
}

// RawBytes returns the RLP-encoded signed transaction, ready for broadcast.
func (tx *LegacyTx) RawBytes() []byte {
	return rlp.EncodeTransaction(
		tx.Nonce,
		tx.GasPrice,
		tx.GasLimit,
		tx.To,
		tx.Value,
		tx.Data,
		tx.V,
		tx.R,
		tx.S,
	)
}

// Hash returns the transaction hash (keccak256 of the RLP-encoded signed tx).
func (tx *LegacyTx) Hash() []byte {
	return ethcrypto.Keccak256(tx.RawBytes())
}

// HashHex returns the transaction hash as a hex string with 0x prefix.
func (tx *LegacyTx) HashHex() string {
	return "0x" + hex.EncodeToString(tx.Hash())
}

// IsSigned returns true if the transaction has been signed.
func (tx *LegacyTx) IsSigned() bool {
	return tx.V != nil && tx.R != nil && tx.S != nil
}
