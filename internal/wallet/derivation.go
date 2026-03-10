package wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
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
	// ChainBCH is the Bitcoin Cash chain.
	ChainBCH = chain.BCH
	// ChainLTC is the Litecoin chain.
	ChainLTC = chain.LTC
)

// BIP44 change chain constants.
const (
	// ExternalChain is for receiving addresses (BIP44 change=0).
	ExternalChain uint32 = 0
	// InternalChain is for change addresses (BIP44 change=1).
	InternalChain uint32 = 1
)

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

	// IsChange indicates if this is a change address (internal chain).
	// False for receiving addresses (external chain).
	IsChange bool `json:"is_change,omitempty"`
}

// GetDerivationPath returns the full BIP44 derivation path for a chain.
// Uses external chain (change=0) for backward compatibility.
func GetDerivationPath(chain ChainID, account, index uint32) string {
	return GetDerivationPathFull(chain, account, ExternalChain, index)
}

// GetDerivationPathFull returns the full BIP44 derivation path including change chain.
// Path format: m/44'/coin_type'/account'/change/index
func GetDerivationPathFull(chain ChainID, account, change, index uint32) string {
	coinType := chain.CoinType()
	return fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, account, change, index)
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
	case ChainBSV, ChainBTC, ChainBCH, ChainLTC:
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

// DeriveAddressWithChange derives an address for the given chain, change type, and index.
// Use ExternalChain (0) for receiving addresses, InternalChain (1) for change addresses.
func DeriveAddressWithChange(seed []byte, chain ChainID, account, change, index uint32) (*Address, error) {
	// Create master key from seed
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// Derive the key using BIP44 path with change
	key, err := deriveBIP44KeyWithChange(masterKey, chain, account, change, index)
	if err != nil {
		return nil, err
	}

	// Get public key and derive address based on chain
	var address, pubKeyHex string
	switch chain {
	case ChainETH:
		address, pubKeyHex, err = deriveETHAddress(key)
	case ChainBSV, ChainBTC, ChainBCH, ChainLTC:
		address, pubKeyHex, err = deriveBSVAddress(key)
	default:
		return nil, ErrUnsupportedChain
	}
	if err != nil {
		return nil, err
	}

	return &Address{
		Path:      GetDerivationPathFull(chain, account, change, index),
		Index:     index,
		Address:   address,
		PublicKey: pubKeyHex,
		IsChange:  change == InternalChain,
	}, nil
}

// DerivePrivateKey derives a private key for signing operations.
// Uses external chain (change=0) for backward compatibility.
// The returned key must be zeroed by the caller after use.
func DerivePrivateKey(seed []byte, chain ChainID, account, index uint32) ([]byte, error) {
	return DerivePrivateKeyWithChange(seed, chain, account, ExternalChain, index)
}

// DerivePrivateKeyWithChange derives a private key with explicit change chain.
// Use ExternalChain (0) for receiving addresses, InternalChain (1) for change addresses.
// The returned key must be zeroed by the caller after use.
func DerivePrivateKeyWithChange(seed []byte, chain ChainID, account, change, index uint32) ([]byte, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	key, err := deriveBIP44KeyWithChange(masterKey, chain, account, change, index)
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

// deriveBIP44Key derives a key following BIP44 path structure using external chain.
// Path: m / purpose' / coin_type' / account' / 0 / address_index
// For backward compatibility, uses ExternalChain (change=0).
func deriveBIP44Key(masterKey *hdkeychain.ExtendedKey, chain ChainID, account, index uint32) (*hdkeychain.ExtendedKey, error) {
	return deriveBIP44KeyWithChange(masterKey, chain, account, ExternalChain, index)
}

// deriveBIP44KeyWithChange derives a key following BIP44 path structure with explicit change chain.
// Path: m / purpose' / coin_type' / account' / change / address_index
func deriveBIP44KeyWithChange(masterKey *hdkeychain.ExtendedKey, chain ChainID, account, change, index uint32) (*hdkeychain.ExtendedKey, error) {
	return deriveBIP44KeyWithCoinType(masterKey, chain.CoinType(), account, change, index)
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
	pubKey := key.SerializedPubKey()
	address = bitcoin.Base58CheckEncode(0x00, bitcoin.Hash160(pubKey))
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

// ErrInvalidPrivKeyLength indicates a private key has incorrect byte length.
var ErrInvalidPrivKeyLength = errors.New("invalid private key length")

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

// IsValidETHAddress checks if an Ethereum address has valid structural format
// (length, "0x" prefix, hex characters). It does NOT validate the EIP-55
// mixed-case checksum. Use HasValidEIP55Checksum for full validation.
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

// eip55Nibble extracts the nibble from the keccak256 hash at position i.
// Even positions use the high nibble; odd positions use the low nibble.
func eip55Nibble(hashBytes []byte, i int) byte {
	if i%2 == 0 {
		return hashBytes[i/2] >> 4
	}

	return hashBytes[i/2] & 0x0F
}

// HasValidEIP55Checksum reports whether address has a valid EIP-55 mixed-case checksum.
// The address must first pass IsValidETHAddress (structural check).
func HasValidEIP55Checksum(address string) bool {
	if !IsValidETHAddress(address) {
		return false
	}

	addrHex := strings.ToLower(address[2:])
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(addrHex))
	hashBytes := hash.Sum(nil)

	for i, c := range []byte(address[2:]) {
		nibble := eip55Nibble(hashBytes, i)
		if c >= 'a' && c <= 'f' && nibble >= 8 {
			return false // should be uppercase
		}
		if c >= 'A' && c <= 'F' && nibble < 8 {
			return false // should be lowercase
		}
	}

	return true
}

// decompressPublicKey decompresses a 33-byte compressed public key to 65-byte uncompressed.
func decompressPublicKey(compressed []byte) ([]byte, error) {
	pk, err := secp256k1.ParsePubKey(compressed)
	if err != nil {
		return nil, fmt.Errorf("invalid compressed public key: %w", err)
	}
	return pk.SerializeUncompressed(), nil
}

// DerivePrivateKeyForChain derives a private key for a specific chain at index.
// Uses account 0, which is the default. The returned key must be zeroed after use.
func DerivePrivateKeyForChain(seed []byte, chain ChainID, index uint32) ([]byte, error) {
	return DerivePrivateKey(seed, chain, 0, index)
}

// DeriveAddressWithCoinType derives a BSV-style address using an arbitrary coin type.
// This is useful for discovering funds from wallets that use different coin types
// (e.g., MoneyButton uses 0 for BTC, Exodus uses 145 for BCH).
//
// Parameters:
//   - seed: BIP39 seed bytes
//   - coinType: BIP44 coin type (0=BTC, 145=BCH, 236=BSV)
//   - account: account index (typically 0)
//   - change: 0 for external (receiving), 1 for internal (change)
//   - index: address index
//
// Returns the address string, public key hex, and derivation path.
func DeriveAddressWithCoinType(seed []byte, coinType, account, change, index uint32) (string, string, string, error) {
	// Create master key from seed
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create master key: %w", err)
	}

	// Derive using BIP44 path with arbitrary coin type
	key, err := deriveBIP44KeyWithCoinType(masterKey, coinType, account, change, index)
	if err != nil {
		return "", "", "", err
	}

	// Derive BSV-style address (P2PKH)
	address, pubKeyHex, err := deriveBSVAddress(key)
	if err != nil {
		return "", "", "", err
	}

	path := fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, account, change, index)
	return address, pubKeyHex, path, nil
}

// MnemonicContext holds a pre-computed master key for efficient batch address derivation.
// Create one per seed using NewMnemonicContext, then call its methods for each address.
// This avoids recreating the master key (HMAC-SHA512) on every derivation call.
// Intermediate keys (m/44', coin type, account) are cached to avoid redundant EC operations.
type MnemonicContext struct {
	masterKey    *hdkeychain.ExtendedKey
	purposeKey   *hdkeychain.ExtendedKey            // cached m/44'
	coinTypeKeys map[uint32]*hdkeychain.ExtendedKey // coinType → m/44'/coinType'
	accountKeys  map[uint64]*hdkeychain.ExtendedKey // (coinType<<32|account) → m/44'/coinType'/account'
	legacyKey    *hdkeychain.ExtendedKey            // cached m/0'
}

// NewMnemonicContext creates a MnemonicContext from a BIP39 seed.
func NewMnemonicContext(seed []byte) (*MnemonicContext, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}
	return &MnemonicContext{
		masterKey:    masterKey,
		coinTypeKeys: make(map[uint32]*hdkeychain.ExtendedKey),
		accountKeys:  make(map[uint64]*hdkeychain.ExtendedKey),
	}, nil
}

// Zero clears all private key material held by the MnemonicContext.
// Call this when the context is no longer needed, typically via defer.
func (mc *MnemonicContext) Zero() {
	if mc.masterKey != nil {
		mc.masterKey.Zero()
	}
	if mc.purposeKey != nil {
		mc.purposeKey.Zero()
	}
	for _, k := range mc.coinTypeKeys {
		k.Zero()
	}
	for _, k := range mc.accountKeys {
		k.Zero()
	}
	if mc.legacyKey != nil {
		mc.legacyKey.Zero()
	}
}

// DeriveAddressWithCoinType derives a BSV-style address using an arbitrary coin type.
// Returns the address, public key hex, and derivation path.
// Uses cached intermediate keys for efficiency.
func (mc *MnemonicContext) DeriveAddressWithCoinType(coinType, account, change, index uint32) (string, string, string, error) {
	accountKey, err := mc.getAccountKey(coinType, account)
	if err != nil {
		return "", "", "", err
	}
	changeKey, err := accountKey.ChildBIP32Std(change)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive change key: %w", err)
	}
	indexKey, err := changeKey.ChildBIP32Std(index)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive index key: %w", err)
	}
	address, pubKeyHex, err := deriveBSVAddress(indexKey)
	if err != nil {
		return "", "", "", err
	}
	path := fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, account, change, index)
	return address, pubKeyHex, path, nil
}

// DeriveLegacyAddress derives an address using the legacy HandCash 1.x path (m/0'/index).
// Uses cached m/0' key for efficiency.
func (mc *MnemonicContext) DeriveLegacyAddress(index uint32) (string, string, string, error) {
	legacyKey, err := mc.getLegacyKey()
	if err != nil {
		return "", "", "", err
	}
	indexKey, err := legacyKey.ChildBIP32Std(index)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive index key: %w", err)
	}
	address, pubKeyHex, err := deriveBSVAddress(indexKey)
	if err != nil {
		return "", "", "", err
	}
	path := fmt.Sprintf("m/0'/%d", index)
	return address, pubKeyHex, path, nil
}

// GapResult holds the derived public key and path components for a BIP44 address index.
// Path formatting is deferred to avoid allocations on non-matching results.
type GapResult struct {
	PubKey   []byte
	CoinType uint32
	Account  uint32
	Change   uint32
	Index    uint32
}

// Path returns the BIP44 derivation path string.
// Computed lazily — only call on matched results to avoid unnecessary allocations.
func (r GapResult) Path() string {
	return fmt.Sprintf("m/44'/%d'/%d'/%d/%d", r.CoinType, r.Account, r.Change, r.Index)
}

// DeriveGap derives public keys for indices 0..gap-1 on a BIP44 chain.
// Uses cached intermediate keys and performs only one EC operation per index,
// making it roughly 5x faster for gap scanning.
func (mc *MnemonicContext) DeriveGap(coinType, account, change uint32, gap int) ([]GapResult, error) {
	accountKey, err := mc.getAccountKey(coinType, account)
	if err != nil {
		return nil, err
	}
	changeKey, err := accountKey.ChildBIP32Std(change)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change key: %w", err)
	}

	results := make([]GapResult, 0, gap)
	for i := uint32(0); i < uint32(gap); i++ { //nolint:gosec // gap is validated
		indexKey, iErr := changeKey.ChildBIP32Std(i)
		if iErr != nil {
			// BIP32 child derivation failure is extremely rare (degenerate key).
			// Skip this index; the GapResult slice preserves correct Index values.
			continue
		}
		results = append(results, GapResult{
			PubKey:   indexKey.SerializedPubKey(),
			CoinType: coinType,
			Account:  account,
			Change:   change,
			Index:    i,
		})
	}
	return results, nil
}

// getPurposeKey returns the cached m/44' key, deriving it on first call.
func (mc *MnemonicContext) getPurposeKey() (*hdkeychain.ExtendedKey, error) {
	if mc.purposeKey != nil {
		return mc.purposeKey, nil
	}
	key, err := mc.masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return nil, fmt.Errorf("failed to derive purpose key: %w", err)
	}
	mc.purposeKey = key
	return key, nil
}

// getCoinTypeKey returns the cached m/44'/coinType' key.
func (mc *MnemonicContext) getCoinTypeKey(coinType uint32) (*hdkeychain.ExtendedKey, error) {
	if key, ok := mc.coinTypeKeys[coinType]; ok {
		return key, nil
	}
	purposeKey, err := mc.getPurposeKey()
	if err != nil {
		return nil, err
	}
	key, err := purposeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("failed to derive coin type key: %w", err)
	}
	mc.coinTypeKeys[coinType] = key
	return key, nil
}

// getAccountKey returns the cached m/44'/coinType'/account' key.
func (mc *MnemonicContext) getAccountKey(coinType, account uint32) (*hdkeychain.ExtendedKey, error) {
	cacheKey := uint64(coinType)<<32 | uint64(account)
	if key, ok := mc.accountKeys[cacheKey]; ok {
		return key, nil
	}
	coinTypeKey, err := mc.getCoinTypeKey(coinType)
	if err != nil {
		return nil, err
	}
	key, err := coinTypeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + account)
	if err != nil {
		return nil, fmt.Errorf("failed to derive account key: %w", err)
	}
	mc.accountKeys[cacheKey] = key
	return key, nil
}

// getLegacyKey returns the cached m/0' key for HandCash legacy derivation.
func (mc *MnemonicContext) getLegacyKey() (*hdkeychain.ExtendedKey, error) {
	if mc.legacyKey != nil {
		return mc.legacyKey, nil
	}
	key, err := mc.masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive m/0' key: %w", err)
	}
	mc.legacyKey = key
	return key, nil
}

// DerivePrivateKeyWithCoinType derives a private key using an arbitrary coin type.
// This is used for signing transactions from discovered addresses.
// The returned key must be zeroed by the caller after use.
func DerivePrivateKeyWithCoinType(seed []byte, coinType, account, change, index uint32) ([]byte, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	key, err := deriveBIP44KeyWithCoinType(masterKey, coinType, account, change, index)
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

// DeriveLegacyAddress derives an address using the legacy HandCash 1.x path (m/0'/index).
// This non-standard path was used by early versions of HandCash.
func DeriveLegacyAddress(seed []byte, index uint32) (string, string, string, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create master key: %w", err)
	}

	// m/0' (hardened)
	purposeKey, err := masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive m/0' key: %w", err)
	}

	// m/0'/index (non-hardened)
	indexKey, err := purposeKey.ChildBIP32Std(index)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to derive index key: %w", err)
	}

	address, pubKeyHex, err := deriveBSVAddress(indexKey)
	if err != nil {
		return "", "", "", err
	}

	path := fmt.Sprintf("m/0'/%d", index)
	return address, pubKeyHex, path, nil
}

// DeriveLegacyPrivateKey derives a private key using the legacy HandCash 1.x path.
// The returned key must be zeroed by the caller after use.
func DeriveLegacyPrivateKey(seed []byte, index uint32) ([]byte, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// m/0' (hardened)
	purposeKey, err := masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("failed to derive m/0' key: %w", err)
	}

	// m/0'/index (non-hardened)
	indexKey, err := purposeKey.ChildBIP32Std(index)
	if err != nil {
		return nil, fmt.Errorf("failed to derive index key: %w", err)
	}

	serialized, err := indexKey.SerializedPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}
	privKey := make([]byte, 32)
	copy(privKey, serialized)
	return privKey, nil
}

// deriveBIP44KeyWithCoinType derives a key using an arbitrary coin type.
// Path: m / 44' / coin_type' / account' / change / address_index
func deriveBIP44KeyWithCoinType(masterKey *hdkeychain.ExtendedKey, coinType, account, change, index uint32) (*hdkeychain.ExtendedKey, error) {
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

	// m/44'/coin_type'/account'/change
	changeKey, err := accountKey.ChildBIP32Std(change)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change key: %w", err)
	}

	// m/44'/coin_type'/account'/change/index
	indexKey, err := changeKey.ChildBIP32Std(index)
	if err != nil {
		return nil, fmt.Errorf("failed to derive index key: %w", err)
	}

	return indexKey, nil
}

// AddressFromPrivKeyBytes derives a P2PKH address from raw private key bytes.
// This is the hot path for batch WIF/hex key processing:
// secp256k1 scalar multiplication → compressed pubkey → Hash160 → Base58Check.
func AddressFromPrivKeyBytes(privKey []byte) (string, error) {
	if len(privKey) != 32 {
		return "", fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidPrivKeyLength, len(privKey))
	}

	sk := secp256k1.PrivKeyFromBytes(privKey)
	pubKey := sk.PubKey().SerializeCompressed()
	return bitcoin.Base58CheckEncode(0x00, bitcoin.Hash160(pubKey)), nil
}

// ZeroBytes zeros out a byte slice.
// runtime.KeepAlive prevents the compiler from optimizing away the zeroing
// as a dead store when the slice is not used afterward.
func ZeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
	runtime.KeepAlive(data)
}
