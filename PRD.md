# Sigil — Product Requirements Document

> Personal multi-chain wallet CLI — unlock your BSV, BTC, and ETH

**Read first:** [VISION.md](./VISION.md) — the *why* behind Sigil

---

## Overview

Sigil is a secure, terminal-based wallet for power users who want full control over their keys and transactions across **BSV, BTC, BCH, ETH, and USDC**.

---

## CLI Design Principles

### Command Structure

Sigil uses a **noun-verb** command pattern for consistency and discoverability:

```
sigil <noun> <verb> [args] [flags]
```

| Pattern | Example | Description |
|---------|---------|-------------|
| `noun verb` | `sigil wallet create` | Standard command |
| `noun verb arg` | `sigil wallet show main` | Positional for required single value |
| `noun verb --flag` | `sigil tx send --to 0x...` | Flags for options |

**Nouns** (resources):
- `wallet` — HD wallet management
- `key` — Key import/export
- `balance` — Balance queries (all chains)
- `tx` — Transactions (all chains)
- `scan` — Fork scanner
- `backup` — Backup/restore
- `config` — Configuration

**Verbs** (actions):
- `create`, `import`, `export` — Lifecycle
- `list`, `show`, `get` — Read operations
- `send`, `sign`, `broadcast` — Write operations
- `run`, `verify` — Execution

### Flag Conventions

| Rule | Example |
|------|---------|
| Long flags preferred | `--wallet`, `--output`, `--chain` |
| Short for common ops | `-w` (wallet), `-o` (output), `-c` (config) |
| Boolean flags | `--verbose`, `--quiet`, `--no-color` |
| Repeatable flags | `--chain eth --chain bsv` or `--chain eth,bsv` |

### Help Text Standards

```go
// Short description: < 60 characters, no period
Short: "Create a new HD wallet"

// Long description: Full explanation with examples
Long: `Create a new HD wallet with BIP39 mnemonic.

Generates a cryptographically secure mnemonic phrase and derives
addresses for all enabled chains.

Examples:
  sigil wallet create main
  sigil wallet create main --words 24
  sigil wallet create main --words 12 --passphrase`
```

---

## Global Flags

Available on all commands:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `text` | Output format: `text`, `json` |
| `--verbose` | `-v` | `false` | Enable verbose logging |
| `--quiet` | `-q` | `false` | Suppress non-essential output |
| `--config` | `-c` | `~/.sigil/config.yaml` | Path to config file |
| `--home` | | `~/.sigil` | Sigil home directory |
| `--no-color` | | `false` | Disable colored output |

**Flag precedence:** CLI flags > Environment variables > Config file > Defaults

---

## Output Formats

### Text Format (Default)

Human-readable tables optimized for terminal viewing:

```
$ sigil balance show --wallet main

BALANCE SUMMARY
Wallet: main
───────────────────────────────────────
Chain    Address                    Balance
───────────────────────────────────────
ETH      0x742d35Cc6634...          1.2840 ETH
USDC     0x742d35Cc6634...          500.00 USDC
BSV      1A1zP1eP5QGefi...          1.234 BSV
───────────────────────────────────────
```

### JSON Format

Structured output for scripts and agents:

```
$ sigil balance show --wallet main -o json
```

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

### Auto-Detection

Output format is automatically selected based on context:

| Context | Format |
|---------|--------|
| Interactive terminal (TTY) | `text` |
| Piped output (`\| jq`) | `json` |
| Redirected (`> file`) | `json` |
| Explicit `--output` flag | As specified |

Detection logic:
```go
func DefaultOutputFormat() string {
    if !term.IsTerminal(int(os.Stdout.Fd())) {
        return "json"
    }
    return "text"
}
```

---

## Error Handling

### Exit Codes

| Code | Name | Description |
|------|------|-------------|
| `0` | Success | Command completed successfully |
| `1` | Error | General error |
| `2` | InputError | Invalid input, args, or flags |
| `3` | AuthError | Authentication/decryption failed |
| `4` | NotFoundError | Resource not found (wallet, address) |
| `5` | PermissionError | Insufficient funds, locked wallet |

### Error Messages

All errors follow a consistent structure:

**Text format:**
```
Error: insufficient funds for transaction
  Required: 0.5 ETH
  Available: 0.1 ETH
  Suggestion: Check balance with 'sigil balance show --wallet main'
```

**JSON format:**
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

### Sentinel Errors

Programmatically checkable error codes:

| Code | Meaning |
|------|---------|
| `WALLET_NOT_FOUND` | Wallet file does not exist |
| `WALLET_LOCKED` | Wallet is encrypted and locked |
| `INVALID_MNEMONIC` | Mnemonic phrase validation failed |
| `INVALID_ADDRESS` | Address format invalid for chain |
| `INVALID_AMOUNT` | Amount format or value invalid |
| `INSUFFICIENT_FUNDS` | Not enough balance for operation |
| `NETWORK_ERROR` | RPC/API communication failed |
| `TX_REJECTED` | Transaction rejected by network |
| `DECRYPTION_FAILED` | Wrong password or corrupted file |
| `CONFIG_INVALID` | Configuration file malformed |

---

## MVP (Phase 1)

**Goal:** Usable day one for real tasks — BSVA invoice payments (ETH/USDC) and basic BSV wallet operations.

### 1.1 Core Infrastructure

| Feature | Description |
|---------|-------------|
| **Project structure** | `cmd/`, `internal/`, proper Go module |
| **Config system** | YAML config at `~/.sigil/config.yaml` |
| **Encrypted storage** | Age encryption for wallet files |
| **CLI framework** | Cobra commands with noun-verb pattern |

### 1.2 Key Management (All Chains)

| Feature | Description |
|---------|-------------|
| **Generate keys** | BIP39 mnemonic (12 or 24 words) |
| **Import WIF** | Standard Bitcoin WIF format |
| **Import mnemonic** | 12/24 word phrases with optional passphrase |
| **Import hex** | Raw 256-bit private key |
| **Encrypted storage** | Keys encrypted at rest (age) |
| **Derivation paths** | BIP44 for BSV (`m/44'/236'/0'`), BTC (`m/44'/0'/0'`), ETH (`m/44'/60'/0'`) |

#### Supported Import Formats

| Format | Auto-Detect Pattern | Example |
|--------|---------------------|---------|
| BIP39 Mnemonic | 12/24 words from wordlist | `abandon abandon ... about` |
| WIF (mainnet) | 51-52 chars, starts 5/K/L | `5HueCGU8rMjxEX...` |
| Hex Private Key | 64 hex characters | `e8f32e723decf4c0...` |
| Sigil Backup | `.sigil` extension | `main-2026-01-25.sigil` |
| Ethereum Keystore | JSON with "crypto" key | `keystore-0x742d.json` |
| xprv/xpub | Starts with xprv/xpub | `xprv9s21ZrQH143K...` |

### 1.3 ETH/USDC Support (Invoice Management)

**Primary use case:** Receive BSVA invoice payments in USDC, manage ETH for gas.

| Feature | Description |
|---------|-------------|
| **ETH addresses** | Derive from same seed, different path |
| **Balance check** | ETH balance via RPC |
| **USDC balance** | ERC-20 balanceOf call |
| **Send ETH** | Transfer between addresses |
| **Send USDC** | ERC-20 transfer function |
| **Gas estimation** | Fetch current gas prices |
| **Quick transfer** | Move between own wallets easily |

### 1.4 BSV Basics

| Feature | Description |
|---------|-------------|
| **Address generation** | Derive BSV addresses from HD wallet |
| **Balance check** | WhatsOnChain API integration |
| **UTXO listing** | View unspent outputs |
| **Simple send** | Basic P2PKH transaction |
| **Fee estimation** | TAAL/GorillaPool fee APIs |

### 1.5 CLI Commands (MVP)

All commands follow the noun-verb pattern with consistent flags:

```bash
# Wallet Management
sigil wallet create <name>              # Create new wallet
sigil wallet create main --words 24     # 24-word mnemonic
sigil wallet import <name> --mnemonic   # Import from mnemonic
sigil wallet import <name> --wif        # Import from WIF
sigil wallet list                       # List all wallets
sigil wallet show <name>                # Show wallet details
sigil wallet show main -o json          # JSON output

# Key Operations
sigil key generate                      # Generate new keypair
sigil key generate --words 24           # 24-word mnemonic
sigil key export <name> --format wif    # Export as WIF

# Balance Queries (unified)
sigil balance show --wallet main                    # All chains
sigil balance show --wallet main --chain eth        # ETH only
sigil balance show --wallet main --chain eth,bsv   # Multiple chains
sigil balance show --address 0x742d35Cc...          # Single address

# Transactions (unified)
sigil tx send --wallet main --to <addr> --amount 0.1 --chain eth
sigil tx send --wallet main --to <addr> --amount 100 --chain eth --token USDC
sigil tx send --wallet main --to <addr> --amount 0.5 --chain bsv

# UTXO Management (BSV/BTC/BCH)
sigil utxo list --wallet main --chain bsv
sigil utxo list --address 1A1zP1...

# Configuration
sigil config init                               # Initialize config
sigil config show                               # Show current config
sigil config set networks.eth.rpc "https://..."
sigil config get networks.eth.rpc

# Wallet Restoration
sigil wallet restore <name>                      # Interactive restore
sigil wallet restore main --input "abandon ..."  # From mnemonic
sigil wallet restore main --file backup.sigil    # From backup

# Backup (MVP)
sigil backup create --wallet main
sigil backup restore --input backup.sigil
sigil backup verify --input backup.sigil
```

**Command-to-JSON output examples:**

```bash
$ sigil wallet list -o json
```
```json
{
  "wallets": [
    {
      "name": "main",
      "created_at": "2026-01-25T10:30:00Z",
      "chains": ["eth", "bsv", "btc"],
      "addresses": {
        "eth": "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
        "bsv": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
        "btc": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
      }
    }
  ]
}
```

```bash
$ sigil tx send --wallet main --to 0x123... --amount 100 --chain eth --token USDC -o json
```
```json
{
  "transaction": {
    "hash": "0x1234567890abcdef...",
    "chain": "eth",
    "from": "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
    "to": "0x123...",
    "amount": "100",
    "token": "USDC",
    "gas_used": "65000",
    "gas_price": "20000000000",
    "status": "pending"
  }
}
```

### 1.6 MVP Success Criteria

- [ ] Can generate new 24-word wallet
- [ ] Can import existing mnemonic
- [ ] Can check ETH/USDC balances
- [ ] Can send USDC to another address
- [ ] Can check BSV balance
- [ ] Can send BSV transaction
- [ ] All keys encrypted at rest
- [ ] JSON output works for all commands
- [ ] Exit codes are consistent
- [ ] CLI documentation complete and accurate

#### Wallet Restoration Criteria

- [ ] Can restore from 12/24-word mnemonic
- [ ] Can restore from WIF
- [ ] Can restore from hex private key
- [ ] Can restore from .sigil backup file
- [ ] Auto-detection works for all formats
- [ ] Typo suggestions for misspelled BIP39 words
- [ ] Address verification shown before saving
- [ ] Interactive mode guides users step-by-step

### 1.7 Documentation Requirements

**MVP must ship with comprehensive documentation.** All docs auto-generated where possible.

#### CLI Help System

Every command includes built-in help:

```bash
sigil --help                    # Root help with all commands
sigil wallet --help             # Noun-level help
sigil wallet create --help      # Full command help with examples
```

Help output structure:
```
$ sigil wallet create --help

Create a new HD wallet

Usage:
  sigil wallet create <name> [flags]

Examples:
  # Create wallet with 24-word mnemonic
  sigil wallet create main --words 24

  # Create with passphrase protection
  sigil wallet create vault --words 24 --passphrase

  # Create and output details as JSON
  sigil wallet create main -o json

Flags:
  -w, --words int        Mnemonic length: 12 or 24 (default 12)
  -p, --passphrase       Prompt for BIP39 passphrase
  -o, --output string    Output format: text, json (default "text")
  -h, --help             Help for create

Global Flags:
  -c, --config string    Config file (default "~/.sigil/config.yaml")
      --home string      Sigil home directory (default "~/.sigil")
      --no-color         Disable colored output
  -v, --verbose          Enable verbose logging
```

#### Generated Documentation

| Artifact | Source | Location |
|----------|--------|----------|
| **CLI Reference** | Auto-generated from Cobra | `docs/cli.md` |
| **Man Pages** | `cobra-doc` generation | `man/sigil.1` |
| **Completions** | Shell completion scripts | `completions/` |

Generation commands:
```bash
sigil completion bash > completions/sigil.bash
sigil completion zsh > completions/_sigil
sigil completion fish > completions/sigil.fish
sigil docs generate --output docs/cli.md
sigil docs man --output man/
```

#### README Structure

The project README (`README.md`) must include:

```markdown
# Sigil

> Personal multi-chain wallet CLI

## Quick Start

### Installation
[Binary downloads, go install, brew]

### First Wallet
sigil wallet create main --words 24

### Check Balances
sigil balance show --wallet main

### Send Funds
sigil tx send --wallet main --to <addr> --amount 0.1 --chain eth

## Commands

| Command | Description |
|---------|-------------|
| `sigil wallet` | Wallet management |
| `sigil balance` | Balance queries |
| `sigil tx` | Transactions |
| `sigil config` | Configuration |

## Configuration

[Config file location, env vars, examples]

## Security

[Encryption, key storage, best practices]

## Documentation

- [CLI Reference](docs/cli.md)
- [Configuration Guide](docs/config.md)
- [Security Model](docs/security.md)
```

#### Inline Code Documentation

All exported functions include godoc comments:

```go
// CreateWallet generates a new HD wallet with the specified mnemonic length.
//
// Parameters:
//   - name: Unique wallet identifier (alphanumeric, underscores)
//   - words: Mnemonic length, must be 12 or 24
//   - passphrase: Optional BIP39 passphrase for additional security
//
// Returns the wallet address for each enabled chain, or an error if
// wallet creation fails (e.g., name already exists, entropy failure).
//
// Example:
//
//	wallet, err := CreateWallet("main", 24, "")
//	if err != nil {
//	    return fmt.Errorf("create wallet: %w", err)
//	}
func CreateWallet(name string, words int, passphrase string) (*Wallet, error)
```

#### Documentation Checklist

- [ ] `sigil --help` shows all top-level commands
- [ ] Every command has `--help` with examples
- [ ] Shell completions work (bash, zsh, fish)
- [ ] `README.md` has quick start guide
- [ ] `docs/cli.md` has full command reference
- [ ] All exported Go functions have godoc comments
- [ ] Error messages include actionable suggestions
- [ ] Man page installable via `make install-man`

### 1.8 Wallet Restoration

Making wallet restoration foolproof with auto-detection, guided mode, typo correction, and verification.

#### Unified Restore Command

```bash
sigil wallet restore <name>                      # Interactive guided mode
sigil wallet restore <name> --input "words..."   # Auto-detect format
sigil wallet restore <name> --file backup.sigil  # From backup file
sigil wallet restore <name> --file keystore.json # Ethereum keystore
sigil wallet restore <name> --clipboard          # Paste from clipboard
```

#### Auto-Detection Logic

| Input | Detection | Format |
|-------|-----------|--------|
| 12/24 valid BIP39 words | Word count + wordlist | Mnemonic |
| 51-52 chars, starts 5/K/L | Length + prefix | WIF |
| 64 hex characters | Length + charset | Raw hex key |
| `.sigil` file | Extension | Sigil backup |
| JSON with "crypto" field | Structure | Ethereum keystore |

#### Interactive Guided Mode

The interactive mode walks users through restoration step-by-step:

1. **Format selection** — "What type of backup do you have?" with menu options
2. **Secure input** — Hidden input for sensitive data (mnemonic, keys)
3. **Validation** — Real-time validation with typo suggestions
4. **Address preview** — Show derived addresses before saving
5. **Confirmation** — Explicit confirmation before finalizing

#### Mnemonic Typo Correction

Validate input against the BIP39 2048-word list and suggest corrections:

- Calculate Levenshtein distance for unrecognized words
- Suggest closest matches from the wordlist
- Example: "Did you mean 'abandon' instead of 'abandn'?"

```
$ sigil wallet restore main --input "abandn ability able about ..."

Error: Invalid mnemonic - word 1 not in BIP39 wordlist
  Found: "abandn"
  Did you mean: "abandon"?

Suggestion: Correct the word and try again
```

#### Verification Step

Before saving, show derived addresses for user verification:

```
Derived addresses from your seed:
  ETH: 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0
  BSV: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
  BTC: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa

Do these match your expected addresses? [y/N]
```

This allows users to verify they entered the correct seed before committing.

---

## Phase 2: Multi-Chain Fork Scanner

**Goal:** Detect and recover funds from old wallets across BSV/BTC/BCH forks.

### 2.1 Chain APIs

| Chain | Primary API | Fallback |
|-------|-------------|----------|
| BSV | WhatsOnChain | GorillaPool |
| BTC | mempool.space | Blockstream |
| BCH | Fullstack.cash | Bitcoin.com |

### 2.2 Fork Scanner

| Feature | Description |
|---------|-------------|
| **Address scanning** | Check first N addresses on each chain |
| **Gap limit** | Standard 20-address gap for HD wallets |
| **Balance aggregation** | Total per chain, per address |
| **UTXO discovery** | Find all spendable outputs |

```
$ sigil scan run --wallet main

FORK SCANNER
Wallet: main
Scanning addresses 0-19 on all chains...

Chain   Address                  Balance      UTXOs
─────────────────────────────────────────────────────
BSV     1A1zP1eP5QGefi2D...      1.234 BSV    3
BTC     1A1zP1eP5QGefi2D...      0.001 BTC    1
BCH     (none found)             —            —

Total: 1.234 BSV, 0.001 BTC
```

### 2.3 CLI Commands (Phase 2)

```bash
sigil scan run --wallet main                    # Scan all chains
sigil scan run --wallet main --chain bsv        # BSV only
sigil scan run --wallet main --chain btc,bch    # Specific chains
sigil scan run --wallet main --gap 50           # Custom gap limit
sigil scan run --wallet main -o json            # JSON output
```

---

## Phase 3: TUI Dashboard

**Goal:** Beautiful terminal UI for visual wallet management.

### 3.1 TUI Screens

| Screen | Purpose |
|--------|---------|
| **Dashboard** | Unified balance view, recent transactions |
| **Wallet Manager** | Create, import, list wallets |
| **UTXO Explorer** | Inspect, select, freeze UTXOs |
| **TX Builder** | Visual transaction construction |
| **Fork Scanner** | Interactive chain scanning |
| **Settings** | Network, fees, security config |

### 3.2 Tech Stack

- **bubbletea** — TUI framework
- **lipgloss** — Styling
- **bubbles** — UI components

### 3.3 Dashboard Mockup

```
┌─────────────────────────────────────────────────────────────┐
│ sigil — main                                         [?] [x]│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Total Value: $4,847.32                                     │
│                                                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │     BSV     │ │     ETH     │ │    USDC     │            │
│  │  1.234 BSV  │ │  1.284 ETH  │ │   $500.00   │            │
│  │   ~$47.32   │ │  ~$4,200    │ │             │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
│                                                             │
│  Recent Activity                                            │
│  ─────────────────────────────────────────────────          │
│  + +100 USDC   BSVA Invoice    Jan 28, 2026                 │
│  - -0.1 ETH    Gas refill      Jan 25, 2026                 │
│  + +0.5 BSV    Consolidation   Jan 20, 2026                 │
│                                                             │
│  [Send] [Receive] [Scan] [Settings]                         │
└─────────────────────────────────────────────────────────────┘
```

---

## Phase 4: Advanced Backup System

**Goal:** Thumbdrive-ready encrypted backups for cold storage.

> **Note:** Basic backup/restore commands are included in MVP (see Section 1.5). This phase adds advanced features.

### 4.1 Advanced Backup Features

| Feature | Description |
|---------|-------------|
| **`.sigil` format** | Encrypted, portable backup file |
| **Manifest** | Metadata header (unencrypted) |
| **Checksum** | SHA256 integrity verification |
| **Paper backup** | Printable mnemonic format |
| **USB export** | Direct export to removable media |

### 4.2 Backup File Structure

```
main-2026-01-25.sigil
└── wallet-backup/
    ├── manifest.json     # Metadata (wallet name, chains, counts)
    ├── wallet.json       # Encrypted wallet data
    └── checksum.sha256   # Integrity check
```

### 4.3 CLI Commands (Phase 4)

```bash
# Advanced backup features (beyond MVP)
sigil backup create --wallet main --output /Volumes/USB/  # Export to USB
sigil backup paper --wallet main                          # Generate printable backup
```

---

## Phase 5: Advanced Features

**Goal:** Power-user features for complex workflows.

### 5.1 Transaction Building

| Feature | Description |
|---------|-------------|
| **UTXO selection** | Manual or automatic (oldest-first, minimize-inputs) |
| **Consolidation** | Merge many UTXOs into one |
| **Sweep** | Move all funds from address |
| **OP_RETURN** | Embed data in transactions |
| **RBF** | Replace-by-fee for BTC |

### 5.2 Additional Import Formats

| Format | Description |
|--------|-------------|
| **xprv/xpub** | Extended keys (watch-only support) |
| **Keystore JSON** | Ethereum keystore files |
| **Electrum seed** | Electrum-style seeds |

### 5.3 Token Support

| Token Type | Chain |
|------------|-------|
| **1Sat Ordinals** | BSV |
| **BSV20** | BSV |
| **ERC-20** | ETH (beyond USDC) |

---

## Phase 6: Security Hardening

**Goal:** Production-ready security for real funds.

### 6.1 Security Features

| Feature | Description |
|---------|-------------|
| **Memory protection** | mlock to prevent swap |
| **Paranoid mode** | Dice roll entropy mixing |
| **Auto-lock** | Timeout after inactivity |
| **Confirmation threshold** | Require confirm above X amount |
| **Air-gapped workflow** | Sign offline, broadcast online |

### 6.2 Audit & Compliance

- [ ] Security audit by third party
- [ ] Reproducible builds
- [ ] Dependency review

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                           sigil                             │
├─────────────────────────────────────────────────────────────┤
│  TUI Layer (bubbletea/lipgloss)                             │
│  ├── Dashboard         ├── TX Builder                       │
│  ├── Wallet Manager    ├── Fork Scanner                     │
│  └── UTXO Explorer     └── Settings                         │
├─────────────────────────────────────────────────────────────┤
│  CLI Layer (cobra)                                          │
│  ├── wallet    ├── balance   ├── tx       ├── scan          │
│  ├── key       ├── utxo      ├── backup   ├── config        │
│  └── (noun-verb pattern throughout)                         │
├─────────────────────────────────────────────────────────────┤
│  Core Library                                               │
│  ├── keystore/     — Encrypted key storage (age)            │
│  ├── wallet/       — HD wallet management                   │
│  ├── tx/           — Transaction building & signing         │
│  ├── chain/        — Multi-chain API clients                │
│  │   ├── bsv/      — WhatsOnChain, TAAL, GorillaPool        │
│  │   ├── btc/      — Mempool.space, Blockstream             │
│  │   ├── bch/      — Fullstack.cash                         │
│  │   └── eth/      — Infura, Alchemy, public RPC            │
│  ├── tokens/       — ERC-20 support (USDC)                  │
│  └── crypto/       — Key generation, derivation, entropy    │
├─────────────────────────────────────────────────────────────┤
│  Dependencies                                               │
│  ├── github.com/bitcoin-sv/go-sdk      (BSV)                │
│  ├── github.com/BitcoinSchema/go-bitcoin (HD keys)          │
│  ├── github.com/mrz1836/go-whatsonchain  (BSV API)          │
│  ├── github.com/charmbracelet/bubbletea  (TUI)              │
│  ├── github.com/ethereum/go-ethereum     (ETH)              │
│  └── filippo.io/age                      (encryption)       │
└─────────────────────────────────────────────────────────────┘
```

---

## Storage Layout

```
~/.sigil/                         # SIGIL_HOME (configurable)
├── config.yaml                   # App configuration
├── identity.age                  # Age encryption identity
├── wallets/                      # Encrypted wallet files
│   ├── main.wallet
│   ├── exodus.wallet
│   └── bsva.wallet
└── backups/                      # Portable backup files
    └── main-2026-01-25.sigil
```

---

## Configuration

### Full Schema

```yaml
# ~/.sigil/config.yaml
# All fields with validation rules and env var mappings

version: 1                          # Config schema version (required, integer >= 1)

home: ~/.sigil                      # SIGIL_HOME - Sigil data directory
                                    # Validation: valid path, writable
                                    # Env: SIGIL_HOME

encryption:
  method: age                       # Encryption method (required)
                                    # Validation: enum [age]
                                    # Env: SIGIL_ENCRYPTION_METHOD

  identity_file: ~/.sigil/identity.age
                                    # Path to age identity
                                    # Validation: valid path
                                    # Env: SIGIL_IDENTITY_FILE

  key_derivation: argon2id          # KDF for password-based encryption
                                    # Validation: enum [argon2id, scrypt]
                                    # Env: SIGIL_KEY_DERIVATION

networks:
  bsv:
    enabled: true                   # Enable BSV chain
                                    # Validation: boolean
                                    # Env: SIGIL_BSV_ENABLED

    api: whatsonchain               # Primary API provider
                                    # Validation: enum [whatsonchain, gorillapool]
                                    # Env: SIGIL_BSV_API

    broadcast: taal                 # Transaction broadcaster
                                    # Validation: enum [taal, gorillapool, whatsonchain]
                                    # Env: SIGIL_BSV_BROADCAST

    api_key: ""                     # Optional API key
                                    # Validation: string
                                    # Env: SIGIL_BSV_API_KEY

  btc:
    enabled: true                   # Env: SIGIL_BTC_ENABLED
    api: mempool                    # Validation: enum [mempool, blockstream]
                                    # Env: SIGIL_BTC_API

  bch:
    enabled: true                   # Env: SIGIL_BCH_ENABLED
    api: fullstack                  # Validation: enum [fullstack, bitcoincom]
                                    # Env: SIGIL_BCH_API

  eth:
    enabled: true                   # Env: SIGIL_ETH_ENABLED

    rpc: ""                         # Ethereum RPC URL (required if enabled)
                                    # Validation: valid URL, https preferred
                                    # Env: SIGIL_ETH_RPC

    chain_id: 1                     # Ethereum chain ID
                                    # Validation: integer >= 1
                                    # Env: SIGIL_ETH_CHAIN_ID

    tokens:                         # ERC-20 tokens to track
      - symbol: USDC                # Token symbol (required)
                                    # Validation: 1-11 uppercase chars
        address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
                                    # Contract address (required)
                                    # Validation: valid ETH address, checksummed
        decimals: 6                 # Token decimals (required)
                                    # Validation: integer 0-18

fees:
  provider: taal                    # Fee estimation provider
                                    # Validation: enum [taal, gorillapool, manual]
                                    # Env: SIGIL_FEE_PROVIDER

  fallback_sats_per_byte: 1         # Fallback fee rate
                                    # Validation: integer >= 1
                                    # Env: SIGIL_FEE_FALLBACK

  max_sats_per_byte: 100            # Maximum fee rate (safety limit)
                                    # Validation: integer >= fallback
                                    # Env: SIGIL_FEE_MAX

  eth_gas_strategy: medium          # ETH gas strategy
                                    # Validation: enum [slow, medium, fast]
                                    # Env: SIGIL_ETH_GAS_STRATEGY

derivation:
  default_account: 0                # Default BIP44 account index
                                    # Validation: integer >= 0
                                    # Env: SIGIL_DERIVATION_ACCOUNT

  address_gap: 20                   # Address gap limit for scanning
                                    # Validation: integer 1-100
                                    # Env: SIGIL_ADDRESS_GAP

  paths:                            # Custom derivation paths (optional)
    bsv: "m/44'/236'/0'"            # Validation: valid BIP44 path
    btc: "m/44'/0'/0'"
    eth: "m/44'/60'/0'"

security:
  auto_lock_seconds: 300            # Auto-lock timeout (0 = disabled)
                                    # Validation: integer >= 0
                                    # Env: SIGIL_AUTO_LOCK

  require_confirm_above: 0.1        # Confirmation threshold (in chain units)
                                    # Validation: number >= 0
                                    # Env: SIGIL_CONFIRM_THRESHOLD

  memory_lock: true                 # Use mlock for sensitive data
                                    # Validation: boolean
                                    # Env: SIGIL_MEMORY_LOCK

output:
  default_format: auto              # Default output format
                                    # Validation: enum [auto, text, json]
                                    # Env: SIGIL_OUTPUT_FORMAT

  color: auto                       # Color output
                                    # Validation: enum [auto, always, never]
                                    # Env: SIGIL_COLOR
                                    # Also: NO_COLOR (standard)

  verbose: false                    # Verbose logging
                                    # Validation: boolean
                                    # Env: SIGIL_VERBOSE
```

### Configuration Priority

Values are resolved in this order (first wins):

1. **CLI flags** — `--config`, `--output`, etc.
2. **Environment variables** — `SIGIL_*`
3. **Config file** — `~/.sigil/config.yaml`
4. **Defaults** — Built-in fallback values

### Environment Variables

All config values can be overridden via environment:

| Variable | Config Path | Example |
|----------|-------------|---------|
| `SIGIL_HOME` | `home` | `/path/to/sigil` |
| `SIGIL_ETH_RPC` | `networks.eth.rpc` | `https://...` |
| `SIGIL_OUTPUT_FORMAT` | `output.default_format` | `json` |
| `SIGIL_VERBOSE` | `output.verbose` | `true` |
| `SIGIL_BSV_API_KEY` | `networks.bsv.api_key` | `your-key` |
| `NO_COLOR` | `output.color` | `1` (standard) |

---

## Security Patterns

### Secure Memory Handling

```go
// All sensitive data uses mlock to prevent swap
type SecureBytes struct {
    data   []byte
    locked bool
}

// Lock memory on creation
func NewSecureBytes(size int) (*SecureBytes, error) {
    data := make([]byte, size)
    if err := unix.Mlock(data); err != nil {
        return nil, fmt.Errorf("mlock failed: %w", err)
    }
    return &SecureBytes{data: data, locked: true}, nil
}

// Zero and unlock on cleanup
func (s *SecureBytes) Destroy() {
    for i := range s.data {
        s.data[i] = 0
    }
    if s.locked {
        _ = unix.Munlock(s.data)
    }
}
```

### Key Lifecycle

```
┌──────────────┐
│  Generation  │  BIP39 entropy from crypto/rand
└──────┬───────┘
       │
       v
┌──────────────┐
│  Derivation  │  BIP32/BIP44 key derivation
└──────┬───────┘
       │
       v
┌──────────────┐
│  Encryption  │  Age encryption with identity
└──────┬───────┘
       │
       v
┌──────────────┐
│   Storage    │  ~/.sigil/wallets/<name>.wallet
└──────┬───────┘
       │
       v
┌──────────────┐
│    Usage     │  Decrypt to locked memory, sign, re-zero
└──────┬───────┘
       │
       v
┌──────────────┐
│   Zeroing    │  Explicit memory wipe before GC
└──────────────┘
```

### Input Validation

| Input | Validation Rules |
|-------|------------------|
| **ETH Address** | 40 hex chars, valid checksum (EIP-55) |
| **BSV Address** | Base58Check, valid prefix (1 or 3) |
| **BTC Address** | Base58Check or Bech32, valid prefix |
| **Amount** | Positive, <= balance, valid decimal places |
| **Mnemonic** | 12/24 words, valid BIP39 wordlist, valid checksum |
| **WIF** | Base58Check, valid prefix, valid key |
| **Hex Key** | 64 hex chars, valid curve point |

```go
// Address validation example
func ValidateAddress(addr string, chain Chain) error {
    switch chain {
    case ETH:
        if !common.IsHexAddress(addr) {
            return ErrInvalidAddress
        }
        if addr != common.HexToAddress(addr).Hex() {
            return ErrInvalidChecksum
        }
    case BSV, BTC:
        _, err := btcutil.DecodeAddress(addr, chainParams(chain))
        if err != nil {
            return fmt.Errorf("%w: %v", ErrInvalidAddress, err)
        }
    }
    return nil
}
```

---

## Testing Strategy

### Unit Tests

Table-driven tests for all core functions:

```go
func TestValidateAddress(t *testing.T) {
    tests := []struct {
        name    string
        addr    string
        chain   Chain
        wantErr error
    }{
        {
            name:  "valid eth address",
            addr:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
            chain: ETH,
        },
        {
            name:    "invalid eth checksum",
            addr:    "0x742d35cc6634c0532925a3b844bc454e4438f44e",
            chain:   ETH,
            wantErr: ErrInvalidChecksum,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateAddress(tt.addr, tt.chain)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

### Fuzz Tests

Security-critical inputs require fuzz testing:

```go
// go test -fuzz=FuzzValidateMnemonic
func FuzzValidateMnemonic(f *testing.F) {
    // Seed corpus
    f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
    f.Add("zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong")

    f.Fuzz(func(t *testing.T, input string) {
        // Should never panic
        _ = ValidateMnemonic(input)
    })
}

func FuzzParseAmount(f *testing.F) {
    f.Add("1.0", 8)
    f.Add("0.00000001", 8)
    f.Add("-1", 8)

    f.Fuzz(func(t *testing.T, amount string, decimals int) {
        if decimals < 0 || decimals > 18 {
            return
        }
        _ = ParseAmount(amount, decimals)
    })
}
```

### Integration Tests

Separated by build tag:

```go
//go:build integration

func TestETHTransfer(t *testing.T) {
    if os.Getenv("ETH_RPC") == "" {
        t.Skip("ETH_RPC not set")
    }
    // ... test actual RPC calls
}
```

Run with: `go test -tags=integration ./...`

### Test Priority

| Priority | What to Test | Coverage Target |
|----------|--------------|-----------------|
| **Critical** | Cryptographic operations, key derivation, signing | 100% |
| **High** | Address validation, amount parsing, transaction building | 95% |
| **Medium** | Config parsing, CLI flag handling, output formatting | 85% |
| **Low** | Help text, cosmetic formatting | 70% |

---

## Linting Configuration

### golangci-lint

```yaml
# .golangci.yml
run:
  timeout: 5m
  go: "1.24"

linters:
  enable:
    # Security
    - gosec
    - gocritic

    # Bugs
    - staticcheck
    - govet
    - errcheck
    - ineffassign
    - typecheck

    # Style
    - gofmt
    - goimports
    - misspell
    - unconvert

    # Complexity
    - gocyclo
    - gocognit
    - nestif
    - funlen

    # Performance
    - prealloc
    - bodyclose
    - noctx

    # Error handling
    - errname
    - errorlint
    - wrapcheck

    # Misc
    - exhaustive
    - exportloopref
    - godot
    - nolintlint
    - revive
    - whitespace

linters-settings:
  gocyclo:
    min-complexity: 15

  funlen:
    lines: 100
    statements: 50

  nestif:
    min-complexity: 4

  gosec:
    excludes:
      - G104  # Unhandled errors (handled by errcheck)
    config:
      G301: "0750"
      G302: "0640"

  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance
      - experimental

  errcheck:
    check-type-assertions: true
    check-blank: true

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - funlen
        - gocyclo
```

### Pre-commit Hooks

```bash
# Run before commit
golangci-lint run
go test -race ./...
```

---

## References

| Resource | Link |
|----------|------|
| BIP39 (Mnemonic) | [bitcoin/bips/bip-0039](https://github.com/bitcoin/bips/blob/master/bip-0039.mediawiki) |
| BIP32 (HD Wallets) | [bitcoin/bips/bip-0032](https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki) |
| BIP44 (Multi-Account) | [bitcoin/bips/bip-0044](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki) |
| go-sdk | [bitcoin-sv/go-sdk](https://github.com/bitcoin-sv/go-sdk) |
| go-whatsonchain | [mrz1836/go-whatsonchain](https://github.com/mrz1836/go-whatsonchain) |
| WhatsOnChain API | [developers.whatsonchain.com](https://developers.whatsonchain.com/) |
| Mempool.space API | [mempool.space/docs/api](https://mempool.space/docs/api) |
| Age encryption | [filippo.io/age](https://filippo.io/age) |
| EIP-55 (Checksums) | [eips.ethereum.org/EIPS/eip-55](https://eips.ethereum.org/EIPS/eip-55) |
| Go 1.24 Release | [go.dev/doc/go1.24](https://go.dev/doc/go1.24) |

---

<p align="center">
  <sub>Built with care by Z</sub>
</p>
