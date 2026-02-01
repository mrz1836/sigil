# Data Model: Sigil MVP

**Date**: 2026-01-31 | **Plan**: [plan.md](./plan.md)

## Overview

Sigil's data model centers on the **Wallet** entity, which contains an encrypted HD seed from which all addresses are derived. The model is designed for local file storage with no external database dependencies.

---

## Entity Definitions

### Wallet

The primary entity representing an HD wallet with addresses across multiple chains.

```go
// internal/wallet/wallet.go

// Wallet represents an HD wallet with multi-chain address derivation.
type Wallet struct {
    // Name is the unique identifier for this wallet (alphanumeric + underscore)
    Name string `json:"name"`

    // CreatedAt is the wallet creation timestamp
    CreatedAt time.Time `json:"created_at"`

    // EncryptedSeed contains the age-encrypted BIP39 seed bytes
    // The mnemonic is NOT stored - only the derived seed
    EncryptedSeed []byte `json:"encrypted_seed"`

    // Addresses contains derived addresses per chain
    Addresses map[Chain][]Address `json:"addresses"`

    // EnabledChains lists which chains are active for this wallet
    EnabledChains []Chain `json:"enabled_chains"`

    // DerivationConfig holds chain-specific derivation settings
    DerivationConfig DerivationConfig `json:"derivation_config"`

    // Version is the wallet file format version (for migrations)
    Version int `json:"version"`
}
```

**Validation Rules:**
- `Name`: 1-64 characters, `^[a-zA-Z0-9_]+$`, unique across all wallets
- `EncryptedSeed`: Non-empty, valid age-encrypted data
- `EnabledChains`: At least one chain enabled
- `Version`: Must be supported version (currently: 1)

**File Location:** `~/.sigil/wallets/<name>.wallet`

**File Permissions:** `0600` (owner read/write only)

---

### Address

A chain-specific address derived from the wallet's HD seed.

```go
// internal/wallet/address.go

// Address represents a derived blockchain address.
type Address struct {
    // Path is the BIP44 derivation path used
    Path string `json:"path"` // e.g., "m/44'/60'/0'/0/0"

    // Index is the address index within the derivation path
    Index uint32 `json:"index"`

    // Address is the chain-formatted address string
    Address string `json:"address"`

    // PublicKey is the compressed public key (hex)
    PublicKey string `json:"public_key"`
}
```

**Validation Rules:**
- `Path`: Valid BIP44 path format
- `Index`: 0 to 2^31-1 (non-hardened)
- `Address`: Chain-specific format validation
  - ETH: 40 hex chars with 0x prefix, EIP-55 checksum
  - BSV: Base58Check, prefix 1 or 3

---

### Chain

Enumeration of supported blockchain networks.

```go
// internal/chain/chain.go

// Chain represents a supported blockchain.
type Chain string

const (
    ChainETH Chain = "eth"
    ChainBSV Chain = "bsv"
    ChainBTC Chain = "btc" // Future: Phase 2
    ChainBCH Chain = "bch" // Future: Phase 2
)

// DerivationPath returns the BIP44 derivation path for this chain.
func (c Chain) DerivationPath() string {
    switch c {
    case ChainETH:
        return "m/44'/60'/0'"
    case ChainBSV:
        return "m/44'/236'/0'"
    case ChainBTC:
        return "m/44'/0'/0'"
    case ChainBCH:
        return "m/44'/145'/0'"
    default:
        return ""
    }
}

// CoinType returns the BIP44 coin type for this chain.
func (c Chain) CoinType() uint32 {
    switch c {
    case ChainETH:
        return 60
    case ChainBSV:
        return 236
    case ChainBTC:
        return 0
    case ChainBCH:
        return 145
    default:
        return 0
    }
}
```

---

### UTXO

Unspent Transaction Output for UTXO-based chains (BSV/BTC/BCH).

```go
// internal/chain/bsv/utxo.go

// UTXO represents an unspent transaction output.
type UTXO struct {
    // TxID is the transaction hash containing this output
    TxID string `json:"txid"`

    // Vout is the output index within the transaction
    Vout uint32 `json:"vout"`

    // Amount is the value in satoshis
    Amount uint64 `json:"amount"`

    // ScriptPubKey is the locking script (hex)
    ScriptPubKey string `json:"script_pubkey"`

    // Address is the address this UTXO belongs to
    Address string `json:"address"`

    // Confirmations is the number of block confirmations
    Confirmations uint32 `json:"confirmations"`
}
```

**Validation Rules:**
- `TxID`: 64 hex characters
- `Vout`: 0 to transaction output count
- `Amount`: > 0 (dust threshold checked at transaction building time)
- `ScriptPubKey`: Valid hex-encoded script

---

### Transaction (Output)

Result of a broadcast transaction operation.

```go
// internal/chain/transaction.go

// TransactionResult represents the outcome of a transaction broadcast.
type TransactionResult struct {
    // Hash is the transaction hash/ID
    Hash string `json:"hash"`

    // Chain is the blockchain this transaction is on
    Chain Chain `json:"chain"`

    // From is the sender address
    From string `json:"from"`

    // To is the recipient address
    To string `json:"to"`

    // Amount is the transferred amount (human-readable string)
    Amount string `json:"amount"`

    // Token is the token symbol if applicable (e.g., "USDC")
    Token string `json:"token,omitempty"`

    // Fee is the transaction fee paid
    Fee string `json:"fee"`

    // GasUsed is ETH-specific gas consumption
    GasUsed uint64 `json:"gas_used,omitempty"`

    // GasPrice is ETH-specific gas price in wei
    GasPrice string `json:"gas_price,omitempty"`

    // Status is "pending" immediately after broadcast
    Status string `json:"status"`
}
```

**Note:** Sigil only tracks broadcast success. Confirmation status is checked via block explorers.

---

### Config

Application configuration structure.

```go
// internal/config/config.go

// Config represents the application configuration.
type Config struct {
    Version int `yaml:"version"` // Schema version

    Home string `yaml:"home"` // Sigil data directory

    Encryption EncryptionConfig `yaml:"encryption"`
    Networks   NetworksConfig   `yaml:"networks"`
    Fees       FeesConfig       `yaml:"fees"`
    Derivation DerivationConfig `yaml:"derivation"`
    Security   SecurityConfig   `yaml:"security"`
    Output     OutputConfig     `yaml:"output"`
    Logging    LoggingConfig    `yaml:"logging"`
}

type EncryptionConfig struct {
    Method        string `yaml:"method"`         // "age"
    IdentityFile  string `yaml:"identity_file"`  // Path to age identity
    KeyDerivation string `yaml:"key_derivation"` // "argon2id"
}

type NetworksConfig struct {
    ETH ETHNetworkConfig `yaml:"eth"`
    BSV BSVNetworkConfig `yaml:"bsv"`
    BTC BTCNetworkConfig `yaml:"btc"`
    BCH BCHNetworkConfig `yaml:"bch"`
}

type ETHNetworkConfig struct {
    Enabled bool          `yaml:"enabled"`
    RPC     string        `yaml:"rpc"`
    ChainID int           `yaml:"chain_id"`
    Tokens  []TokenConfig `yaml:"tokens"`
}

type TokenConfig struct {
    Symbol   string `yaml:"symbol"`
    Address  string `yaml:"address"`
    Decimals int    `yaml:"decimals"`
}

type BSVNetworkConfig struct {
    Enabled   bool   `yaml:"enabled"`
    API       string `yaml:"api"`       // "whatsonchain", "gorillapool"
    Broadcast string `yaml:"broadcast"` // "taal", "gorillapool", "whatsonchain"
    APIKey    string `yaml:"api_key"`
}

type BTCNetworkConfig struct {
    Enabled bool   `yaml:"enabled"`
    API     string `yaml:"api"` // "mempool", "blockstream"
}

type BCHNetworkConfig struct {
    Enabled bool   `yaml:"enabled"`
    API     string `yaml:"api"` // "fullstack", "bitcoincom"
}

type FeesConfig struct {
    Provider           string `yaml:"provider"`             // "taal", "gorillapool", "manual"
    FallbackSatsPerByte int   `yaml:"fallback_sats_per_byte"`
    MaxSatsPerByte     int    `yaml:"max_sats_per_byte"`
    ETHGasStrategy     string `yaml:"eth_gas_strategy"`     // "slow", "medium", "fast"
}

type DerivationConfig struct {
    DefaultAccount int               `yaml:"default_account"`
    AddressGap     int               `yaml:"address_gap"`
    Paths          map[string]string `yaml:"paths"` // Chain -> path override
}

// SecurityConfig contains security settings.
// MVP: memory_lock only (via internal/crypto/secure.go mlock).
// Post-MVP: auto_lock_seconds, require_confirm_above.
type SecurityConfig struct {
    AutoLockSeconds      int     `yaml:"auto_lock_seconds"`      // Post-MVP
    RequireConfirmAbove  float64 `yaml:"require_confirm_above"`  // Post-MVP
    MemoryLock           bool    `yaml:"memory_lock"`            // MVP (default: true)
}

type OutputConfig struct {
    DefaultFormat string `yaml:"default_format"` // "auto", "text", "json"
    Color         string `yaml:"color"`          // "auto", "always", "never"
    Verbose       bool   `yaml:"verbose"`
}

type LoggingConfig struct {
    Level string `yaml:"level"` // "off", "error", "debug"
    File  string `yaml:"file"`  // Log file path
}
```

**File Location:** `~/.sigil/config.yaml`

**File Permissions:** `0640` (owner read/write, group read)

---

### Backup

Encrypted portable backup structure.

```go
// internal/backup/backup.go

// Backup represents a portable encrypted wallet backup.
type Backup struct {
    // Manifest contains unencrypted metadata
    Manifest BackupManifest `json:"manifest"`

    // EncryptedData contains the age-encrypted wallet data
    EncryptedData []byte `json:"encrypted_data"`

    // Checksum is SHA256 of EncryptedData for integrity verification
    Checksum string `json:"checksum"`
}

// BackupManifest contains backup metadata (not encrypted).
type BackupManifest struct {
    // Version is the backup format version
    Version int `json:"version"`

    // WalletName is the original wallet name
    WalletName string `json:"wallet_name"`

    // CreatedAt is the backup creation timestamp
    CreatedAt time.Time `json:"created_at"`

    // Chains lists the chains included in the backup
    Chains []Chain `json:"chains"`

    // AddressCount is the number of addresses per chain
    AddressCount map[Chain]int `json:"address_count"`
}
```

**File Extension:** `.sigil`

**File Location:** `~/.sigil/backups/<name>-<date>.sigil`

---

### BalanceCache

Cached balance data for offline/failure scenarios.

```go
// internal/cache/cache.go

// BalanceCache stores cached balance information.
type BalanceCache struct {
    Entries map[string]BalanceCacheEntry `json:"entries"`
}

// BalanceCacheEntry represents a single cached balance.
type BalanceCacheEntry struct {
    Chain     Chain     `json:"chain"`
    Address   string    `json:"address"`
    Balance   string    `json:"balance"`
    Symbol    string    `json:"symbol"`
    Decimals  int       `json:"decimals"`
    Token     string    `json:"token,omitempty"` // For ERC-20 tokens
    UpdatedAt time.Time `json:"updated_at"`
}
```

**File Location:** `~/.sigil/cache/balances.json`

**Cache Key Format:** `<chain>:<address>` or `<chain>:<address>:<token>`

---

## Entity Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                           Config                                 │
│  (singleton, loaded at startup)                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ references paths
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Wallet                                 │
│  - name (unique identifier)                                      │
│  - encrypted_seed                                                │
│  - enabled_chains[]                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ 1:N per chain
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Address                                 │
│  - derivation path                                               │
│  - chain-specific format                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┴────────────────────┐
         │                                         │
         ▼ (UTXO chains)                           ▼ (runtime)
┌─────────────────────┐                 ┌─────────────────────────┐
│        UTXO         │                 │    TransactionResult    │
│  - txid:vout        │                 │  - broadcast outcome    │
│  - amount (sats)    │                 └─────────────────────────┘
└─────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                          Backup                                  │
│  - manifest (unencrypted metadata)                              │
│  - encrypted wallet data                                         │
│  - checksum                                                      │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                       BalanceCache                               │
│  - per-address cached balances                                   │
│  - timestamps for staleness detection                            │
└─────────────────────────────────────────────────────────────────┘
```

---

## State Transitions

### Wallet Lifecycle

```
              ┌─────────────┐
              │   (none)    │
              └──────┬──────┘
                     │ sigil wallet create
                     ▼
              ┌─────────────┐
              │   Created   │ ─────────────────────────────────┐
              └──────┬──────┘                                   │
                     │ mnemonic displayed                      │
                     │ user backs up                           │
                     ▼                                         │
              ┌─────────────┐                                   │
              │   Active    │ ◄──── sigil wallet restore ──────┘
              └──────┬──────┘
                     │ sigil backup create
                     ▼
              ┌─────────────┐
              │  Backed Up  │ (wallet file + backup file exist)
              └─────────────┘
```

### Transaction Flow

```
              ┌─────────────┐
              │   Pending   │  User initiates tx
              └──────┬──────┘
                     │ password entered
                     │ wallet decrypted
                     ▼
              ┌─────────────┐
              │  Building   │  UTXOs selected (BSV) / nonce fetched (ETH)
              └──────┬──────┘
                     │ transaction constructed
                     ▼
              ┌─────────────┐
              │   Signing   │  Private key used in memory
              └──────┬──────┘
                     │ signature applied
                     │ key material zeroed
                     ▼
              ┌─────────────┐
              │ Broadcasting│  Sent to network
              └──────┬──────┘
                     │
         ┌──────────┴──────────┐
         ▼                     ▼
┌─────────────┐         ┌─────────────┐
│  Broadcast  │         │   Failed    │
│  (pending)  │         │   (error)   │
└─────────────┘         └─────────────┘
```

---

## Storage Layout

```
~/.sigil/                              # SIGIL_HOME
├── config.yaml                        # Application configuration (0640)
├── sigil.log                          # Debug log file (0640)
├── wallets/                           # Encrypted wallet files (0750)
│   ├── main.wallet                    # Wallet file (0600)
│   └── backup.wallet                  # Wallet file (0600)
├── backups/                           # Portable backup files (0750)
│   └── main-2026-01-31.sigil          # Backup file (0600)
└── cache/                             # Cached data (0750)
    └── balances.json                  # Balance cache (0640)
```

---

## JSON Schemas

### Wallet File (decrypted content)

```json
{
  "name": "main",
  "created_at": "2026-01-31T10:30:00Z",
  "addresses": {
    "eth": [
      {
        "path": "m/44'/60'/0'/0/0",
        "index": 0,
        "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
        "public_key": "04a1b2c3..."
      }
    ],
    "bsv": [
      {
        "path": "m/44'/236'/0'/0/0",
        "index": 0,
        "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
        "public_key": "02a1b2c3..."
      }
    ]
  },
  "enabled_chains": ["eth", "bsv"],
  "derivation_config": {
    "default_account": 0,
    "address_gap": 20,
    "paths": {}
  },
  "version": 1
}
```

### Balance Response (JSON output)

```json
{
  "wallet": "main",
  "balances": [
    {
      "chain": "eth",
      "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
      "balance": "1.284",
      "symbol": "ETH",
      "decimals": 18
    },
    {
      "chain": "eth",
      "token": "USDC",
      "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
      "balance": "500.00",
      "symbol": "USDC",
      "decimals": 6
    },
    {
      "chain": "bsv",
      "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
      "balance": "1.234",
      "symbol": "BSV",
      "decimals": 8
    }
  ],
  "timestamp": "2026-01-31T12:00:00Z"
}
```

### Error Response (JSON output)

```json
{
  "error": {
    "code": "INSUFFICIENT_FUNDS",
    "message": "insufficient funds for transaction",
    "details": {
      "required": "0.5",
      "available": "0.1",
      "symbol": "ETH"
    },
    "suggestion": "Check balance with 'sigil balance show --wallet main'",
    "exit_code": 5
  }
}
```
