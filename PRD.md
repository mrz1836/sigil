# 📋 Sigil — Product Requirements Document

> Personal multi-chain wallet CLI — unlock your BSV, BTC, and ETH

<br>

## Overview

Sigil is a secure, terminal-based wallet for power users who want full control over their keys and transactions across **BSV, BTC, BCH, ETH, and USDC**.

**Read first:** [VISION.md](./VISION.md) — the *why* behind Sigil

<br>

---

<br>

## 🎯 MVP (Phase 1)

**Goal:** Usable day one for real tasks — BSVA invoice payments (ETH/USDC) and basic BSV wallet operations.

<br>

### 1.1 Core Infrastructure

| Feature | Description |
|---------|-------------|
| **Project structure** | `cmd/`, `internal/`, proper Go module |
| **Config system** | YAML config at `~/.sigil/config.yaml` |
| **Encrypted storage** | Age encryption for wallet files |
| **CLI framework** | Cobra commands, clean help text |

<br>

### 1.2 Key Management (All Chains)

| Feature | Description |
|---------|-------------|
| **Generate keys** | BIP39 mnemonic (12 or 24 words) |
| **Import WIF** | Standard Bitcoin WIF format |
| **Import mnemonic** | 12/24 word phrases with optional passphrase |
| **Import hex** | Raw 256-bit private key |
| **Encrypted storage** | Keys encrypted at rest (age) |
| **Derivation paths** | BIP44 for BSV (`m/44'/236'/0'`), BTC (`m/44'/0'/0'`), ETH (`m/44'/60'/0'`) |

<br>

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

**Config:**
```yaml
networks:
  eth:
    enabled: true
    rpc: "https://mainnet.infura.io/v3/YOUR_KEY"
    chain_id: 1
    tokens:
      - symbol: USDC
        address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
        decimals: 6
```

<br>

### 1.4 BSV Basics

| Feature | Description |
|---------|-------------|
| **Address generation** | Derive BSV addresses from HD wallet |
| **Balance check** | WhatsOnChain API integration |
| **UTXO listing** | View unspent outputs |
| **Simple send** | Basic P2PKH transaction |
| **Fee estimation** | TAAL/GorillaPool fee APIs |

<br>

### 1.5 CLI Commands (MVP)

```bash
# Wallet
sigil wallet create --name "main" --words 24
sigil wallet import --mnemonic --name "exodus"
sigil wallet list
sigil wallet show --name "main"

# Keys
sigil key generate --words 24
sigil key import --wif "L1aW4..."

# ETH/USDC
sigil eth balance --address 0x742d35Cc...
sigil eth send --from "main" --to 0x123... --amount 0.1
sigil usdc balance --address 0x742d35Cc...
sigil usdc send --from "main" --to 0x123... --amount 100

# BSV
sigil bsv balance --address 1A1zP1...
sigil bsv utxos --address 1A1zP1...
sigil bsv send --from "main" --to 1BvBMSE... --amount 0.5

# Config
sigil config init
sigil config set networks.eth.rpc "https://..."
```

<br>

### 1.6 MVP Success Criteria

- [ ] Can generate new 24-word wallet
- [ ] Can import existing mnemonic
- [ ] Can check ETH/USDC balances
- [ ] Can send USDC to another address
- [ ] Can check BSV balance
- [ ] Can send BSV transaction
- [ ] All keys encrypted at rest

<br>

---

<br>

## 🔄 Phase 2: Multi-Chain Fork Scanner

**Goal:** Detect and recover funds from old wallets across BSV/BTC/BCH forks.

<br>

### 2.1 Chain APIs

| Chain | Primary API | Fallback |
|-------|-------------|----------|
| BSV | WhatsOnChain | GorillaPool |
| BTC | mempool.space | Blockstream |
| BCH | Fullstack.cash | Bitcoin.com |

<br>

### 2.2 Fork Scanner

| Feature | Description |
|---------|-------------|
| **Address scanning** | Check first N addresses on each chain |
| **Gap limit** | Standard 20-address gap for HD wallets |
| **Balance aggregation** | Total per chain, per address |
| **UTXO discovery** | Find all spendable outputs |

```
┌─────────────────────────────────────────────────────────────┐
│ Fork Scanner                                                │
├─────────────────────────────────────────────────────────────┤
│  Wallet: main                                               │
│  Scanning addresses 0-19 on all chains...                   │
│                                                             │
│  Chain │ Address              │ Balance    │ UTXOs          │
│  ──────┼──────────────────────┼────────────┼───────         │
│  BSV   │ 1A1zP1eP5QGefi2D...  │ 1.234 BSV  │ 3              │
│  BTC   │ 1A1zP1eP5QGefi2D...  │ 0.001 BTC  │ 1              │
│  BCH   │ (none found)         │ —          │ —              │
│                                                             │
│  Total: 1.234 BSV, 0.001 BTC                                │
└─────────────────────────────────────────────────────────────┘
```

<br>

### 2.3 CLI Commands (Phase 2)

```bash
sigil scan --wallet "main"                    # Scan all chains
sigil scan --wallet "main" --chain bsv        # BSV only
sigil scan --wallet "main" --chain btc,bch    # Specific chains
```

<br>

---

<br>

## 🖥️ Phase 3: TUI Dashboard

**Goal:** Beautiful terminal UI for visual wallet management.

<br>

### 3.1 TUI Screens

| Screen | Purpose |
|--------|---------|
| **Dashboard** | Unified balance view, recent transactions |
| **Wallet Manager** | Create, import, list wallets |
| **UTXO Explorer** | Inspect, select, freeze UTXOs |
| **TX Builder** | Visual transaction construction |
| **Fork Scanner** | Interactive chain scanning |
| **Settings** | Network, fees, security config |

<br>

### 3.2 Tech Stack

- **bubbletea** — TUI framework
- **lipgloss** — Styling
- **bubbles** — UI components

<br>

### 3.3 Dashboard Mockup

```
┌─────────────────────────────────────────────────────────────┐
│ sigil — main                                         [?] [×]│
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
│  ↓ +100 USDC   BSVA Invoice    Jan 28, 2026                 │
│  ↑ -0.1 ETH    Gas refill      Jan 25, 2026                 │
│  ↓ +0.5 BSV    Consolidation   Jan 20, 2026                 │
│                                                             │
│  [Send] [Receive] [Scan] [Settings]                         │
└─────────────────────────────────────────────────────────────┘
```

<br>

---

<br>

## 💾 Phase 4: Backup System

**Goal:** Thumbdrive-ready encrypted backups for cold storage.

<br>

### 4.1 Backup Features

| Feature | Description |
|---------|-------------|
| **`.sigil` format** | Encrypted, portable backup file |
| **Manifest** | Metadata header (unencrypted) |
| **Checksum** | SHA256 integrity verification |
| **Paper backup** | Printable mnemonic format |

<br>

### 4.2 Backup File Structure

```
main-2026-01-25.sigil
└── wallet-backup/
    ├── manifest.json     # Metadata (wallet name, chains, counts)
    ├── wallet.json       # Encrypted wallet data
    └── checksum.sha256   # Integrity check
```

<br>

### 4.3 CLI Commands (Phase 4)

```bash
sigil backup create --wallet main --output /Volumes/USB/
sigil backup restore --input backup.sigil
sigil backup verify --input backup.sigil
sigil backup paper --wallet main    # Generate printable backup
```

<br>

---

<br>

## 🚀 Phase 5: Advanced Features

**Goal:** Power-user features for complex workflows.

<br>

### 5.1 Transaction Building

| Feature | Description |
|---------|-------------|
| **UTXO selection** | Manual or automatic (oldest-first, minimize-inputs) |
| **Consolidation** | Merge many UTXOs into one |
| **Sweep** | Move all funds from address |
| **OP_RETURN** | Embed data in transactions |
| **RBF** | Replace-by-fee for BTC |

<br>

### 5.2 Additional Import Formats

| Format | Description |
|--------|-------------|
| **xprv/xpub** | Extended keys (watch-only support) |
| **Keystore JSON** | Ethereum keystore files |
| **Electrum seed** | Electrum-style seeds |

<br>

### 5.3 Token Support

| Token Type | Chain |
|------------|-------|
| **1Sat Ordinals** | BSV |
| **BSV20** | BSV |
| **ERC-20** | ETH (beyond USDC) |

<br>

---

<br>

## 🛡️ Phase 6: Security Hardening

**Goal:** Production-ready security for real funds.

<br>

### 6.1 Security Features

| Feature | Description |
|---------|-------------|
| **Memory protection** | mlock to prevent swap |
| **Paranoid mode** | Dice roll entropy mixing |
| **Auto-lock** | Timeout after inactivity |
| **Confirmation threshold** | Require confirm above X amount |
| **Air-gapped workflow** | Sign offline, broadcast online |

<br>

### 6.2 Audit & Compliance

- [ ] Security audit by third party
- [ ] Reproducible builds
- [ ] Dependency review

<br>

---

<br>

## 🏗️ Architecture

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
│  ├── wallet    ├── bsv     ├── eth     ├── scan             │
│  ├── key       ├── btc     ├── usdc    ├── backup           │
│  └── config    └── bch     └── tx      └── ...              │
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

<br>

---

<br>

## 📁 Storage Layout

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

<br>

---

<br>

## 🔧 Configuration

```yaml
# ~/.sigil/config.yaml
version: 1
home: ~/.sigil

encryption:
  method: age
  identity_file: ~/.sigil/identity.age

networks:
  bsv:
    enabled: true
    api: whatsonchain
    broadcast: taal
  btc:
    enabled: true
    api: mempool
  bch:
    enabled: true
    api: fullstack
  eth:
    enabled: true
    rpc: "https://mainnet.infura.io/v3/YOUR_KEY"
    chain_id: 1
    tokens:
      - symbol: USDC
        address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
        decimals: 6

fees:
  provider: taal
  fallback_sats_per_byte: 1

security:
  auto_lock_seconds: 300
  require_confirm_above: 0.1

derivation:
  default_path: "m/44'/236'/0'"
  address_gap: 20
```

<br>

---

<br>

## 📚 References

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

<br>

---

<br>

<p align="center">
  <sub>Built with 🧠 by Z</sub>
</p>
