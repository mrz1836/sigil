package wallet

// Network identifies which Bitcoin SV network a wallet or key operates on.
// It is the single source of truth for the version bytes used when encoding
// addresses, WIF private keys, and BIP32 extended keys.
//
// The zero value is Mainnet, so any struct or call site that does not
// explicitly set a network retains the historical mainnet behavior.
type Network uint8

const (
	// Mainnet is the BSV production network (zero value — default).
	Mainnet Network = iota
	// Testnet is the BSV test network.
	Testnet
)

// Network string identifiers, matching the config/CLI representation.
const (
	// NetworkMainnet is the canonical string for the mainnet network.
	NetworkMainnet = "main"
	// NetworkTestnet is the canonical string for the testnet network.
	NetworkTestnet = "test"
)

// NetworkFromString maps a config/CLI network string to a Network.
// It accepts "test"/"testnet" (case-insensitive is handled by callers that
// normalize first); anything else — including the empty string used by legacy
// wallet files — resolves to Mainnet. This makes Mainnet the fail-safe default.
func NetworkFromString(s string) Network {
	switch s {
	case NetworkTestnet, "testnet":
		return Testnet
	default:
		return Mainnet
	}
}

// String returns the canonical string identifier ("main" or "test").
func (n Network) String() string {
	if n == Testnet {
		return NetworkTestnet
	}
	return NetworkMainnet
}

// IsTestnet reports whether the network is testnet.
func (n Network) IsTestnet() bool { return n == Testnet }

// P2PKHVersion returns the Base58Check version byte for P2PKH addresses.
// Mainnet 0x00 ("1..."); testnet 0x6f ("m..."/"n...").
func (n Network) P2PKHVersion() byte {
	if n == Testnet {
		return 0x6f
	}
	return 0x00
}

// P2SHVersion returns the Base58Check version byte for P2SH addresses.
// Mainnet 0x05 ("3..."); testnet 0xc4 ("2...").
func (n Network) P2SHVersion() byte {
	if n == Testnet {
		return 0xc4
	}
	return 0x05
}

// WIFVersion returns the Base58Check version byte for WIF private keys.
// Mainnet 0x80 ("5"/"K"/"L"); testnet 0xef ("9"/"c").
func (n Network) WIFVersion() byte {
	if n == Testnet {
		return 0xef
	}
	return 0x80
}

// HDPrivVersion returns the BIP32 extended private key ("xprv"/"tprv") version bytes.
func (n Network) HDPrivVersion() [4]byte {
	if n == Testnet {
		return [4]byte{0x04, 0x35, 0x83, 0x94} // tprv
	}
	return [4]byte{0x04, 0x88, 0xAD, 0xE4} // xprv
}

// HDPubVersion returns the BIP32 extended public key ("xpub"/"tpub") version bytes.
func (n Network) HDPubVersion() [4]byte {
	if n == Testnet {
		return [4]byte{0x04, 0x35, 0x87, 0xCF} // tpub
	}
	return [4]byte{0x04, 0x88, 0xB2, 0x1E} // xpub
}
