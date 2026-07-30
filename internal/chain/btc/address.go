// Package btc provides Bitcoin (BTC) chain client implementation.
//
// BTC support mirrors the BSV package (enum + parallel-switch model) with the
// deliberate differences required by Bitcoin: legacy (pre-fork) SIGHASH signing,
// a 546-satoshi dust limit, sat/vByte fees from mempool.space (Esplora), and the
// ability to pay any standard destination type (P2PKH, P2SH, native SegWit v0
// P2WPKH/P2WSH). Sigil only spends its own legacy P2PKH UTXOs; native-SegWit
// spending (witness signing) is out of scope.
package btc

import (
	"github.com/bsv-blockchain/go-sdk/script"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Network represents the BTC network.
type Network string

// Network constants.
const (
	// NetworkMainnet is the BTC mainnet.
	NetworkMainnet Network = "main"
	// NetworkTestnet is the BTC testnet (testnet4 on mempool.space).
	NetworkTestnet Network = "test"
)

// AddrType enumerates the standard Bitcoin destination script types Sigil can
// pay to. Sigil only spends P2PKH (its own receive/change addresses); the other
// types are supported only as send destinations.
type AddrType int

// Address type constants.
const (
	// AddrP2PKH is a pay-to-public-key-hash address (base58, "1…"/"m…"/"n…").
	AddrP2PKH AddrType = iota
	// AddrP2SH is a pay-to-script-hash address (base58, "3…"/"2…").
	AddrP2SH
	// AddrP2WPKH is a native SegWit v0 pay-to-witness-public-key-hash address
	// (bech32, "bc1q…"/"tb1q…", 20-byte program).
	AddrP2WPKH
	// AddrP2WSH is a native SegWit v0 pay-to-witness-script-hash address
	// (bech32, 32-byte program).
	AddrP2WSH
)

// String returns a human-readable name for the address type.
func (t AddrType) String() string {
	switch t {
	case AddrP2PKH:
		return "P2PKH"
	case AddrP2SH:
		return "P2SH"
	case AddrP2WPKH:
		return "P2WPKH"
	case AddrP2WSH:
		return "P2WSH"
	default:
		return "unknown"
	}
}

// Base58Check version bytes and bech32 HRPs, scoped by network.
const (
	versionP2PKHMainnet = 0x00 // mainnet P2PKH addresses start with "1"
	versionP2SHMainnet  = 0x05 // mainnet P2SH addresses start with "3"
	versionP2PKHTestnet = 0x6f // testnet P2PKH addresses start with "m" or "n"
	versionP2SHTestnet  = 0xc4 // testnet P2SH addresses start with "2"

	hrpMainnet = "bc" // mainnet bech32 human-readable part
	hrpTestnet = "tb" // testnet bech32 human-readable part

	// hash160Len is the length of a RIPEMD-160 hash / P2WPKH witness program.
	hash160Len = 20
	// witnessScriptHashLen is the length of a P2WSH witness program (SHA-256).
	witnessScriptHashLen = 32
)

// ErrInvalidAddress indicates the address format is invalid, has a bad checksum,
// or belongs to the wrong network. Callers receive this stable sentinel
// regardless of the underlying failure, mirroring the BSV package.
var ErrInvalidAddress = &sigilerr.SigilError{
	Code:     "BTC_INVALID_ADDRESS",
	Message:  "invalid BTC address format",
	ExitCode: sigilerr.ExitInput,
}

// hrpForNetwork returns the bech32 human-readable part for the network.
func hrpForNetwork(network Network) string {
	if network == NetworkTestnet {
		return hrpTestnet
	}
	return hrpMainnet
}

// DecodeAddress decodes a BTC address, returning its type and the underlying
// hash/witness program (20 or 32 bytes). Validation is scoped to the network:
// base58 version bytes and the bech32 HRP must match, so a mainnet address is
// rejected on testnet and vice versa. bech32m (segwit v1+/taproot) is rejected.
func DecodeAddress(addr string, network Network) (AddrType, []byte, error) {
	if addr == "" {
		return 0, nil, ErrInvalidAddress
	}

	// bech32 (native SegWit) addresses carry the network HRP followed by the '1'
	// separator. Detect them explicitly so a bech32 string is never mistaken for
	// base58 (and a mismatched-HRP segwit address is rejected, not reinterpreted).
	if isBech32Candidate(addr) {
		return decodeSegwitAddress(addr, network)
	}
	return decodeBase58Address(addr, network)
}

// decodeSegwitAddress decodes a native SegWit v0 address into its type/program.
func decodeSegwitAddress(addr string, network Network) (AddrType, []byte, error) {
	version, program, err := bitcoin.SegwitDecode(hrpForNetwork(network), addr)
	if err != nil || version != 0 {
		return 0, nil, ErrInvalidAddress
	}
	switch len(program) {
	case hash160Len:
		return AddrP2WPKH, program, nil
	case witnessScriptHashLen:
		return AddrP2WSH, program, nil
	default:
		return 0, nil, ErrInvalidAddress
	}
}

// decodeBase58Address decodes a Base58Check (P2PKH/P2SH) address, scoped to the
// network's version-byte allow-list.
func decodeBase58Address(addr string, network Network) (AddrType, []byte, error) {
	payload, err := bitcoin.Base58CheckDecode(addr)
	if err != nil || len(payload) != 1+hash160Len {
		return 0, nil, ErrInvalidAddress
	}
	version, hash := payload[0], payload[1:]

	verP2PKH, verP2SH := byte(versionP2PKHMainnet), byte(versionP2SHMainnet)
	if network == NetworkTestnet {
		verP2PKH, verP2SH = versionP2PKHTestnet, versionP2SHTestnet
	}
	switch version {
	case verP2PKH:
		return AddrP2PKH, hash, nil
	case verP2SH:
		return AddrP2SH, hash, nil
	default:
		return 0, nil, ErrInvalidAddress
	}
}

// isBech32Candidate reports whether a string looks like a native SegWit address
// (starts with a known HRP + '1'). Case-insensitive per BIP173.
func isBech32Candidate(addr string) bool {
	if len(addr) < 4 {
		return false
	}
	prefix := addr[:3]
	return prefix == "bc1" || prefix == "tb1" || prefix == "BC1" || prefix == "TB1"
}

// ValidateBTCAddressForNetwork validates a BTC address against the given network.
// It normalizes every failure (bad format, bad checksum, wrong network, taproot)
// to the stable ErrInvalidAddress sentinel.
func ValidateBTCAddressForNetwork(addr string, network Network) error {
	if _, _, err := DecodeAddress(addr, network); err != nil {
		return ErrInvalidAddress
	}
	return nil
}

// AddressToScript builds the locking script (scriptPubKey) for paying an address.
// Layouts:
//   - P2PKH:  OP_DUP OP_HASH160 <20> OP_EQUALVERIFY OP_CHECKSIG  (76a914…88ac)
//   - P2SH:   OP_HASH160 <20> OP_EQUAL                            (a914…87)
//   - P2WPKH: OP_0 <20>                                           (0014…)
//   - P2WSH:  OP_0 <32>                                           (0020…)
func AddressToScript(addr string, network Network) (*script.Script, error) {
	addrType, program, err := DecodeAddress(addr, network)
	if err != nil {
		return nil, err
	}

	switch addrType {
	case AddrP2PKH:
		return buildScript(program, []byte{script.OpDUP, script.OpHASH160}, []byte{script.OpEQUALVERIFY, script.OpCHECKSIG})
	case AddrP2SH:
		return buildScript(program, []byte{script.OpHASH160}, []byte{script.OpEQUAL})
	case AddrP2WPKH, AddrP2WSH:
		// Native SegWit v0: OP_0 followed by the 20- or 32-byte witness program.
		return buildScript(program, []byte{script.Op0}, nil)
	default:
		return nil, ErrInvalidAddress
	}
}

// buildScript assembles a locking script as: <prefix opcodes> <pushdata program>
// <suffix opcodes>. It centralizes the go-sdk append error handling.
func buildScript(program, prefix, suffix []byte) (*script.Script, error) {
	s := &script.Script{}
	if len(prefix) > 0 {
		if err := s.AppendOpcodes(prefix...); err != nil {
			return nil, err
		}
	}
	if err := s.AppendPushData(program); err != nil {
		return nil, err
	}
	if len(suffix) > 0 {
		if err := s.AppendOpcodes(suffix...); err != nil {
			return nil, err
		}
	}
	return s, nil
}
