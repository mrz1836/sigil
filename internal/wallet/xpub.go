package wallet

import (
	"errors"
	"fmt"

	"github.com/decred/dcrd/hdkeychain/v3"

	"github.com/mrz1836/sigil/internal/chain"
)

// ErrXpubIsPrivate is returned when an xprv (private key) is provided where an xpub is expected.
var ErrXpubIsPrivate = errors.New("expected xpub but got xprv (private key)")

// DeriveAccountXpub derives the extended public key (xpub) for a BIP44 account.
// Path: m/44'/coinType'/account' → Neuter() → xpub string.
// The xpub can be shared with agents for read-only address derivation
// without exposing the seed or any private key material.
func DeriveAccountXpub(seed []byte, chainID chain.ID, account uint32) (string, error) {
	masterKey, err := hdkeychain.NewMaster(seed, hdNetParams{})
	if err != nil {
		return "", fmt.Errorf("failed to create master key: %w", err)
	}

	coinType := chainID.CoinType()

	// m/44' (purpose)
	purposeKey, err := masterKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		return "", fmt.Errorf("failed to derive purpose key: %w", err)
	}

	// m/44'/coin_type'
	coinTypeKey, err := purposeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return "", fmt.Errorf("failed to derive coin type key: %w", err)
	}

	// m/44'/coin_type'/account'
	accountKey, err := coinTypeKey.ChildBIP32Std(hdkeychain.HardenedKeyStart + account)
	if err != nil {
		return "", fmt.Errorf("failed to derive account key: %w", err)
	}

	// Neuter to get the extended public key (removes private key material)
	xpub := accountKey.Neuter()

	return xpub.String(), nil
}

// DeriveAddressFromXpub derives a chain-specific address from an xpub string.
// This allows address generation without access to the seed or any private keys.
// change: 0 for external (receiving), 1 for internal (change).
func DeriveAddressFromXpub(xpubStr string, chainID chain.ID, change, index uint32) (*Address, error) {
	// Parse the xpub string back into an extended key
	xpub, err := hdkeychain.NewKeyFromString(xpubStr, hdNetParams{})
	if err != nil {
		return nil, fmt.Errorf("invalid xpub: %w", err)
	}

	// Ensure this is actually a public key (not accidentally a private key)
	if xpub.IsPrivate() {
		return nil, ErrXpubIsPrivate
	}

	// m/44'/coinType'/account'/change
	changeKey, err := xpub.ChildBIP32Std(change)
	if err != nil {
		return nil, fmt.Errorf("failed to derive change key from xpub: %w", err)
	}

	// m/44'/coinType'/account'/change/index
	indexKey, err := changeKey.ChildBIP32Std(index)
	if err != nil {
		return nil, fmt.Errorf("failed to derive index key from xpub: %w", err)
	}

	// Derive address based on chain type
	var address, pubKeyHex string
	switch chainID {
	case ChainETH:
		address, pubKeyHex, err = deriveETHAddress(indexKey)
	case ChainBSV, ChainBTC, ChainBCH:
		address, pubKeyHex, err = deriveBSVAddress(indexKey)
	default:
		return nil, ErrUnsupportedChain
	}
	if err != nil {
		return nil, err
	}

	return &Address{
		Path:      GetDerivationPathFull(chainID, 0, change, index),
		Index:     index,
		Address:   address,
		PublicKey: pubKeyHex,
		IsChange:  change == InternalChain,
	}, nil
}
