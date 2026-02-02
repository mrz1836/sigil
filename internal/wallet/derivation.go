package wallet

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/decred/dcrd/hdkeychain/v3"
	"golang.org/x/crypto/sha3"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// ChainID is an alias for chain.ID for use in wallet operations.
// This unifies the chain identifier type across the codebase.
type ChainID = chain.ID

// Chain ID constants - aliases to chain package constants for convenience.
const (
	// ChainETH is the Ethereum chain.
	ChainETH = chain.ETH
	// ChainBSV is the Bitcoin SV chain.
	ChainBSV = chain.BSV
	// ChainBTC is the Bitcoin chain (future).
	ChainBTC = chain.BTC
	// ChainBCH is the Bitcoin Cash chain (future).
	ChainBCH = chain.BCH
)

// secp256k1 curve parameters for public key decompression
//
//nolint:gochecknoglobals // Cryptographic constants for secp256k1 elliptic curve
var (
	secp256k1P *big.Int
	secp256k1B *big.Int
)

//nolint:gochecknoinits // Required for cryptographic constant initialization
func init() {
	var ok bool
	secp256k1P, ok = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	if !ok {
		panic("failed to initialize secp256k1P curve parameter")
	}
	secp256k1B = big.NewInt(7)
}

// hdNetParams satisfies hdkeychain.NetworkParams for BIP32 key derivation.
// Uses standard Bitcoin mainnet HD version bytes.
type hdNetParams struct{}

func (hdNetParams) HDPrivKeyVersion() [4]byte { return [4]byte{0x04, 0x88, 0xAD, 0xE4} }
func (hdNetParams) HDPubKeyVersion() [4]byte  { return [4]byte{0x04, 0x88, 0xB2, 0x1E} }

var (
	// ErrUnsupportedChain indicates the chain is not supported.
	ErrUnsupportedChain = errors.New("unsupported chain")

	// ErrInvalidAddress indicates the address format is invalid.
	ErrInvalidAddress = errors.New("invalid address format")

	// ErrInvalidPublicKeyLength indicates the public key has wrong length.
	ErrInvalidPublicKeyLength = errors.New("invalid compressed public key length")

	// ErrInvalidPublicKeyPrefix indicates the public key has invalid prefix.
	ErrInvalidPublicKeyPrefix = errors.New("invalid public key prefix")
)

// Address represents a derived blockchain address.
type Address struct {
	// Path is the BIP44 derivation path used.
	Path string `json:"path"`

	// Index is the address index within the derivation path.
	Index uint32 `json:"index"`

	// Address is the chain-formatted address string.
	Address string `json:"address"`

	// PublicKey is the public key in hex format.
	PublicKey string `json:"public_key"`
}

// GetDerivationPath returns the full BIP44 derivation path for a chain.
func GetDerivationPath(chain ChainID, account, index uint32) string {
	coinType := chain.CoinType()
	return fmt.Sprintf("m/44'/%d'/%d'/0/%d", coinType, account, index)
}

// DeriveAddress derives an address for the given chain and index from a BIP39 seed.
func DeriveAddress(seed []byte, chain ChainID, account, index uint32) (*Address, error) {
	// Create master key from seed
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// Derive the key using BIP44 path
	key, err := deriveBIP44Key(masterKey, chain, account, index)
	if err != nil {
		return nil, err
	}

	// Get public key and derive address based on chain
	var address, pubKeyHex string
	switch chain {
	case ChainETH:
		address, pubKeyHex, err = deriveETHAddress(key)
	case ChainBSV, ChainBTC, ChainBCH:
		address, pubKeyHex, err = deriveBSVAddress(key)
	default:
		return nil, ErrUnsupportedChain
	}
	if err != nil {
		return nil, err
	}

	return &Address{
		Path:      GetDerivationPath(chain, account, index),
		Index:     index,
		Address:   address,
		PublicKey: pubKeyHex,
	}, nil
}

// DerivePrivateKey derives a private key for signing operations.
// The returned key must be zeroed by the caller after use.
func DerivePrivateKey(seed []byte, chain ChainID, account, index uint32) ([]byte, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	key, err := deriveBIP44Key(masterKey, chain, account, index)
	if err != nil {
		return nil, err
	}

	// Return a copy of the private key bytes
	serialized, err := key.SerializedPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}
	privKey := make([]byte, 32)
	copy(privKey, serialized)
	return privKey, nil
}

// deriveBIP44Key derives a key following BIP44 path structure.
// Path: m / purpose' / coin_type' / account' / change / address_index
func deriveBIP44Key(masterKey *hdkeychain.ExtendedKey, chain ChainID, account, index uint32) (*hdkeychain.ExtendedKey, error) {
	coinType := chain.CoinType()

	// m/44' (purpose)
	purposeKey, err := masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return nil, fmt.Errorf("failed to derive purpose key: %w", err)
	}

	// m/44'/coin_type'
	coinTypeKey, err := purposeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("failed to derive coin type key: %w", err)
	}

	// m/44'/coin_type'/account'
	accountKey, err := coinTypeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + account)
	if err != nil {
		return nil, fmt.Errorf("failed to derive account key: %w", err)
	}

	// m/44'/coin_type'/account'/0 (external chain)
	changeKey, err := accountKey.ChildBIP32Std(0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change key: %w", err)
	}

	// m/44'/coin_type'/account'/0/index
	indexKey, err := changeKey.ChildBIP32Std(index)
	if err != nil {
		return nil, fmt.Errorf("failed to derive index key: %w", err)
	}

	return indexKey, nil
}

// deriveETHAddress derives an Ethereum address from a BIP32 key.
func deriveETHAddress(key *hdkeychain.ExtendedKey) (address, pubKeyHex string, err error) {
	// Get the public key (BIP32 gives us compressed 33-byte key)
	pubKeyCompressed := key.SerializedPubKey()

	// For ETH, we need the uncompressed public key (65 bytes)
	pubKeyUncompressed, err := decompressPublicKey(pubKeyCompressed)
	if err != nil {
		return "", "", fmt.Errorf("failed to decompress public key: %w", err)
	}

	// ETH address is keccak256(pubkey[1:])[12:] - skip the 04 prefix
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKeyUncompressed[1:]) // Skip 0x04 prefix
	addrBytes := hash.Sum(nil)[12:]    // Take last 20 bytes

	// Apply EIP-55 checksum
	address, err = toChecksumAddress(addrBytes)
	if err != nil {
		return "", "", fmt.Errorf("checksum address: %w", err)
	}

	// Return uncompressed public key without the 04 prefix for storage
	pubKeyHex = hex.EncodeToString(pubKeyUncompressed[1:])

	return address, pubKeyHex, nil
}

// deriveBSVAddress derives a Bitcoin SV (or BTC/BCH) address from a BIP32 key.
//
//nolint:unparam // error return is for interface consistency with deriveETHAddress
func deriveBSVAddress(key *hdkeychain.ExtendedKey) (address, pubKeyHex string, _ error) {
	// Get compressed public key (33 bytes)
	pubKey := key.SerializedPubKey()

	// P2PKH address: Base58Check(0x00 + RIPEMD160(SHA256(pubkey)))
	pubKeyHash := bitcoin.Hash160(pubKey)

	// Add version byte (0x00 for mainnet P2PKH)
	versionedPayload := append([]byte{0x00}, pubKeyHash...)

	// Calculate checksum
	checksum := doubleSHA256(versionedPayload)[:4]

	// Combine and encode
	fullPayload := append(versionedPayload, checksum...)
	address = base58Encode(fullPayload)

	pubKeyHex = hex.EncodeToString(pubKey)

	return address, pubKeyHex, nil
}

// checksumChar applies EIP-55 checksum to a single hex character.
func checksumChar(c, hashByte byte, isOddPosition bool) byte {
	if c >= '0' && c <= '9' {
		return c
	}

	nibble := hashByte >> 4
	if isOddPosition {
		nibble = hashByte & 0x0F
	}

	if nibble >= 8 {
		return c - 32 // Uppercase
	}
	return c // Lowercase
}

// ErrInvalidAddressLength indicates the address byte slice has incorrect length.
var ErrInvalidAddressLength = errors.New("invalid address length")

// toChecksumAddress converts a 20-byte address to EIP-55 checksummed hex string.
func toChecksumAddress(addr []byte) (string, error) {
	const ethAddressBytes = 20

	// Input validation - return error for malformed input
	if len(addr) != ethAddressBytes {
		return "", fmt.Errorf("%w: expected %d bytes, got %d",
			ErrInvalidAddressLength, ethAddressBytes, len(addr))
	}

	addrHex := hex.EncodeToString(addr) // Always 40 chars for 20 bytes

	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(addrHex))
	hashBytes := hash.Sum(nil) // Always 32 bytes

	// Build checksummed result
	const hexLen = ethAddressBytes * 2 // 40
	result := make([]byte, hexLen)

	for i := 0; i < hexLen; i++ { // Explicit bounds: i < 40
		// SAFETY: i < 40, so i/2 < 20, which is < 32 (len(hashBytes))
		result[i] = checksumChar(addrHex[i], hashBytes[i/2], i%2 == 1)
	}

	return "0x" + string(result), nil
}

// isHexChar checks if a rune is a valid hexadecimal character.
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// IsValidETHAddress checks if an Ethereum address is valid.
func IsValidETHAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(address, "0x") {
		return false
	}

	// Check if all characters after 0x are valid hex
	addrHex := address[2:]
	for _, c := range addrHex {
		if !isHexChar(c) {
			return false
		}
	}

	return true
}

// doubleSHA256 computes SHA256(SHA256(data)).
func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// base58Encode encodes bytes to base58.
func base58Encode(input []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Count leading zeros
	leadingZeros := 0
	for _, b := range input {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// Convert to big integer
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, alphabet[mod.Int64()])
	}

	// Add leading '1's for each leading zero byte
	for i := 0; i < leadingZeros; i++ {
		result = append(result, alphabet[0])
	}

	// Reverse the result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// decompressPublicKey decompresses a 33-byte compressed public key to 65-byte uncompressed.
func decompressPublicKey(compressed []byte) ([]byte, error) {
	if len(compressed) != 33 {
		return nil, ErrInvalidPublicKeyLength
	}

	prefix := compressed[0]
	if prefix != 0x02 && prefix != 0x03 {
		return nil, ErrInvalidPublicKeyPrefix
	}

	// X coordinate
	x := new(big.Int).SetBytes(compressed[1:33])

	// Compute y^2 = x^3 + 7 (mod p)
	x3 := new(big.Int).Exp(x, big.NewInt(3), secp256k1P)
	y2 := new(big.Int).Add(x3, secp256k1B)
	y2.Mod(y2, secp256k1P)

	// Compute y = sqrt(y2) mod p using Tonelli-Shanks
	// For secp256k1, p ≡ 3 (mod 4), so y = y2^((p+1)/4) mod p
	exp := new(big.Int).Add(secp256k1P, big.NewInt(1))
	exp.Div(exp, big.NewInt(4))
	y := new(big.Int).Exp(y2, exp, secp256k1P)

	// Choose correct y based on parity
	isOdd := y.Bit(0) == 1
	wantOdd := prefix == 0x03
	if isOdd != wantOdd {
		y.Sub(secp256k1P, y)
	}

	// Build uncompressed key: 04 + X + Y
	uncompressed := make([]byte, 65)
	uncompressed[0] = 0x04

	xBytes := x.Bytes()
	yBytes := y.Bytes()

	// Pad to 32 bytes each
	copy(uncompressed[1+(32-len(xBytes)):33], xBytes)
	copy(uncompressed[33+(32-len(yBytes)):65], yBytes)

	return uncompressed, nil
}

// DerivePrivateKeyForChain derives a private key for a specific chain at index.
// Uses account 0, which is the default. The returned key must be zeroed after use.
func DerivePrivateKeyForChain(seed []byte, chain ChainID, index uint32) ([]byte, error) {
	return DerivePrivateKey(seed, chain, 0, index)
}

// ZeroBytes zeros out a byte slice.
func ZeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
