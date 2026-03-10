package wallet

import (
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// ErrInvalidPubKeyLen indicates the public key has incorrect length for multi-format derivation.
var ErrInvalidPubKeyLen = errors.New("invalid compressed public key length")

// NetworkParams holds the encoding parameters for a specific chain.
type NetworkParams struct {
	P2PKHVersion   byte
	P2SHVersion    byte
	Bech32HRP      string // non-empty for BTC/LTC (segwit)
	CashAddrPrefix string // non-empty for BCH (cashaddr)
}

// BTCMainnetParams returns network parameters for Bitcoin mainnet.
func BTCMainnetParams() NetworkParams {
	return NetworkParams{
		P2PKHVersion: 0x00,
		P2SHVersion:  0x05,
		Bech32HRP:    "bc",
	}
}

// LTCMainnetParams returns network parameters for Litecoin mainnet.
func LTCMainnetParams() NetworkParams {
	return NetworkParams{
		P2PKHVersion: 0x30,
		P2SHVersion:  0x32,
		Bech32HRP:    "ltc",
	}
}

// BCHMainnetParams returns network parameters for Bitcoin Cash mainnet.
func BCHMainnetParams() NetworkParams {
	return NetworkParams{
		P2PKHVersion:   0x00,
		P2SHVersion:    0x05,
		CashAddrPrefix: "bitcoincash",
	}
}

// DOGEMainnetParams returns network parameters for Dogecoin mainnet.
func DOGEMainnetParams() NetworkParams {
	return NetworkParams{
		P2PKHVersion: 0x1E,
		P2SHVersion:  0x16,
	}
}

// DerivedAddresses holds all address formats derived from a single public key.
type DerivedAddresses struct {
	P2PKH    string // Base58Check P2PKH address
	P2SH     string // P2SH-P2WPKH address (BTC/LTC only)
	Bech32   string // Native segwit P2WPKH address (BTC/LTC only)
	CashAddr string // CashAddr format (BCH only)
}

// AllAddressesFromPubKey derives all address formats from a compressed public key.
// The pubKeyHash (Hash160) is computed once and reused across formats.
func AllAddressesFromPubKey(compressedPubKey []byte, params NetworkParams) (*DerivedAddresses, error) {
	if len(compressedPubKey) != 33 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidPubKeyLen, len(compressedPubKey))
	}

	pubKeyHash := bitcoin.Hash160(compressedPubKey)
	result := &DerivedAddresses{}

	// P2PKH: Base58Check(version + pubKeyHash)
	result.P2PKH = bitcoin.Base58CheckEncode(params.P2PKHVersion, pubKeyHash)

	if params.Bech32HRP != "" {
		// P2SH-P2WPKH: Base58Check(P2SHVersion + Hash160(0x0014 + pubKeyHash))
		witnessScript := make([]byte, 22)
		witnessScript[0] = 0x00
		witnessScript[1] = 0x14
		copy(witnessScript[2:], pubKeyHash)
		scriptHash := bitcoin.Hash160(witnessScript)
		result.P2SH = bitcoin.Base58CheckEncode(params.P2SHVersion, scriptHash)

		// Bech32 P2WPKH: witness version 0 + pubKeyHash
		var err error
		result.Bech32, err = bitcoin.SegwitEncode(params.Bech32HRP, 0, pubKeyHash)
		if err != nil {
			return nil, fmt.Errorf("bech32 encode: %w", err)
		}
	}

	if params.CashAddrPrefix != "" {
		// CashAddr P2PKH
		var err error
		result.CashAddr, err = bitcoin.CashAddrEncodeShort(params.CashAddrPrefix, bitcoin.CashAddrTypeP2PKH, pubKeyHash)
		if err != nil {
			return nil, fmt.Errorf("cashaddr encode: %w", err)
		}
	}

	return result, nil
}

// AllAddressesFromPrivKey derives all address formats from a raw 32-byte private key.
func AllAddressesFromPrivKey(privKey []byte, params NetworkParams) (*DerivedAddresses, error) {
	if len(privKey) != 32 {
		return nil, fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidPrivKeyLength, len(privKey))
	}

	sk := secp256k1.PrivKeyFromBytes(privKey)
	pubKey := sk.PubKey().SerializeCompressed()
	return AllAddressesFromPubKey(pubKey, params)
}

// NetworkParamsForCoinType returns the NetworkParams for a given BIP44 coin type.
// Returns BTC params as default for unknown coin types (since BTC/BSV share version bytes).
func NetworkParamsForCoinType(coinType uint32) NetworkParams {
	switch coinType {
	case 2: // LTC
		return LTCMainnetParams()
	case 3: // DOGE
		return DOGEMainnetParams()
	case 145: // BCH
		return BCHMainnetParams()
	case 236: // BSV — no SegWit, P2PKH only
		return NetworkParams{P2PKHVersion: 0x00, P2SHVersion: 0x05}
	default: // BTC (0) and others
		return BTCMainnetParams()
	}
}

// NetworkLabelForCoinType returns a human-readable label for a coin type.
func NetworkLabelForCoinType(coinType uint32) string {
	switch coinType {
	case 0:
		return "BTC"
	case 2:
		return "LTC"
	case 3:
		return "DOGE"
	case 145:
		return "BCH"
	case 236:
		return "BSV"
	default:
		return "BTC"
	}
}

// Addresses returns a slice of all non-empty addresses in the DerivedAddresses.
func (d *DerivedAddresses) Addresses() []string {
	var addrs []string
	if d.P2PKH != "" {
		addrs = append(addrs, d.P2PKH)
	}
	if d.P2SH != "" {
		addrs = append(addrs, d.P2SH)
	}
	if d.Bech32 != "" {
		addrs = append(addrs, d.Bech32)
	}
	if d.CashAddr != "" {
		addrs = append(addrs, d.CashAddr)
	}
	return addrs
}

// FormatLabel returns a human-readable label for an address within a DerivedAddresses.
func (d *DerivedAddresses) FormatLabel(addr string) string {
	switch addr {
	case d.P2PKH:
		return "P2PKH"
	case d.P2SH:
		return "P2SH-P2WPKH"
	case d.Bech32:
		return "Bech32"
	case d.CashAddr:
		return "CashAddr"
	default:
		return ""
	}
}
