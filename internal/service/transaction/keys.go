package transaction

import (
	"errors"
	"fmt"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// errAddressNotInWallet indicates a UTXO references an address not found in the wallet.
// Migrated from cli/tx.go line 1044
var errAddressNotInWallet = errors.New("address not found in wallet")

// deriveKeysForUTXOs derives private keys for each unique address that appears in the UTXO set.
// Returns a map of address → private key. The caller must zero all keys after use.
// The chainID selects the BIP44 coin type, so BSV (236) and BTC (0) derive
// distinct keys for the same seed/index.
// Migrated from cli/tx.go lines 1046-1070
func deriveKeysForUTXOs(chainID chain.ID, utxos []chain.UTXO, addresses []wallet.Address, seed []byte) (map[string][]byte, error) {
	// Build address → index lookup
	addrIndex := make(map[string]uint32, len(addresses))
	for _, addr := range addresses {
		addrIndex[addr.Address] = addr.Index
	}

	// Collect unique addresses from UTXOs
	needed := uniqueUTXOAddrs(utxos)

	// Derive private key for each unique address
	keys := make(map[string][]byte, len(needed))
	for addr := range needed {
		key, err := deriveKeyForAddress(chainID, addr, addrIndex, seed)
		if err != nil {
			zeroKeyMap(keys)
			return nil, err
		}
		keys[addr] = key
	}

	return keys, nil
}

// DeriveKeysForUTXOs is the exported version for external use.
func DeriveKeysForUTXOs(chainID chain.ID, utxos []chain.UTXO, addresses []wallet.Address, seed []byte) (map[string][]byte, error) {
	return deriveKeysForUTXOs(chainID, utxos, addresses, seed)
}

// deriveKeyForAddress derives a private key for a single address using the index lookup.
// The chainID selects the BIP44 coin type for derivation.
// Migrated from cli/tx.go lines 1072-1083
func deriveKeyForAddress(chainID chain.ID, addr string, addrIndex map[string]uint32, seed []byte) ([]byte, error) {
	index, ok := addrIndex[addr]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errAddressNotInWallet, addr)
	}
	privKey, err := wallet.DerivePrivateKeyForChain(seed, chainID, index)
	if err != nil {
		return nil, fmt.Errorf("deriving key for address %s (index %d): %w", addr, index, err)
	}
	return privKey, nil
}

// zeroKeyMap zeros all private keys in the map.
// Migrated from cli/tx.go lines 1085-1090
func zeroKeyMap(keys map[string][]byte) {
	for _, k := range keys {
		wallet.ZeroBytes(k)
	}
}

// ZeroKeyMap is the exported version for external use.
func ZeroKeyMap(keys map[string][]byte) {
	zeroKeyMap(keys)
}
