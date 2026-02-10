# Sigil CLI Reference

Sigil is a terminal-based multi-chain cryptocurrency wallet for power users. It supports HD wallet creation with BIP39 mnemonics, balance checking, and transactions across Ethereum (ETH/USDC) and Bitcoin SV (BSV) networks.

---

> ⚠️ **IMPORTANT DISCLAIMER — PLEASE READ BEFORE USE**
>
> Sigil is experimental, open-source software provided **"AS-IS" WITHOUT WARRANTY OF ANY KIND**, express or implied, including but not limited to warranties of merchantability, fitness for a particular purpose, or non-infringement.
>
> **BY USING SIGIL, YOU ACKNOWLEDGE AND AGREE THAT:**
>
> 1. **You are solely responsible for your private keys and seed phrases.** Sigil does not store, transmit, or have any access to your cryptographic keys. If you lose your mnemonic phrase, password, or backup files, your funds are permanently unrecoverable. No support service exists that can retrieve them.
>
> 2. **You bear all risk of loss.** Cryptocurrency transactions are irreversible. Sending funds to incorrect addresses, misconfiguring fees, or any other user error may result in permanent, total loss of funds.
>
> 3. **This software has not been audited.** While developed with security in mind, Sigil has not undergone formal security audits. Use at your own risk.
>
> 4. **The authors and contributors accept no liability.** Under no circumstances shall the developers, contributors, or affiliated parties be held liable for any direct, indirect, incidental, special, or consequential damages arising from the use or inability to use this software.
>
> **Do not use Sigil with funds you cannot afford to lose.**

---

<br>

## Installation

```bash
go install github.com/mrz1836/sigil/cmd/sigil@latest
```

Or build from source:

```bash
git clone https://github.com/mrz1836/sigil.git
cd sigil
go build -o bin/sigil ./cmd/sigil
```

<br>

## Quick Start

### Create a wallet

```bash
sigil wallet create main
```

### Check balances

```bash
sigil balance show --wallet main
```

### Get a receiving address

```bash
sigil receive --wallet main --chain bsv
```

### Send a transaction

```bash
# Send ETH
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

# Send BSV
sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv
```

### Back up your wallet

```bash
sigil backup create --wallet main
```

<br>

## Global Flags

These flags can be used with any command:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--home` | - | `~/.sigil` | Sigil data directory |
| `--output` | `-o` | `auto` | Output format: `text`, `json`, `auto` |
| `--verbose` | `-v` | `false` | Enable verbose output |

<br>

## Commands

### wallet

Manage wallets.

```bash
sigil wallet <subcommand>
```

#### wallet create

Create a new HD wallet with a BIP39 mnemonic phrase.

```bash
sigil wallet create <name> [flags]
```

**Arguments:**
- `<name>` - Name for the new wallet (required)

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--words` | `12` | Mnemonic word count (12 or 24) |
| `--passphrase` | `false` | Use a BIP39 passphrase |
| `--scan` | `false` | Scan for existing UTXOs after creation |

**Examples:**
```bash
sigil wallet create main
sigil wallet create main --words 24
sigil wallet create main --passphrase
sigil wallet create main --scan
```

#### wallet list

List all wallets in the sigil data directory.

```bash
sigil wallet list
sigil wallet ls    # alias
```

**Examples:**
```bash
sigil wallet list
sigil wallet list -o json
```

#### wallet show

Show details for a specific wallet including all derived addresses.

```bash
sigil wallet show <name>
```

**Arguments:**
- `<name>` - Wallet name (required)

**Example:**
```bash
sigil wallet show main
```

#### wallet restore

Restore a wallet from a BIP39 mnemonic phrase, WIF private key, or hex private key.

```bash
sigil wallet restore <name> [flags]
```

**Arguments:**
- `<name>` - Name for the restored wallet (required)

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--input` | - | Seed material (mnemonic, WIF, or hex) |
| `--passphrase` | `false` | Use a BIP39 passphrase (for mnemonic only) |
| `--scan` | `true` | Scan for existing UTXOs after restore |

**Examples:**
```bash
sigil wallet restore backup --input "abandon abandon ... about"
sigil wallet restore imported --input "5HueCGU8rMjxEXxiPuD5BDku..."
sigil wallet restore backup  # Interactive mode
sigil wallet restore backup --input "..." --scan=false  # Skip UTXO scan
```

#### wallet discover

Discover and recover funds from any BSV wallet by scanning multiple derivation paths. This is useful when you have a mnemonic phrase from another wallet (RelayX, MoneyButton, HandCash, etc.) and want to find all funds.

```bash
sigil wallet discover [flags]
```

**How It Works:**

Different BSV wallets use different BIP44 derivation paths. For example:
- **RelayX, RockWallet, Twetch** use `m/44'/236'/0'/...` (BSV standard)
- **MoneyButton, ElectrumSV** use `m/44'/0'/0'/...` (Bitcoin coin type)
- **Exodus, Simply.Cash** use `m/44'/145'/0'/...` (Bitcoin Cash coin type)
- **HandCash 1.x** uses `m/0'/...` (legacy non-BIP44)

The `discover` command scans all these paths automatically to find your funds, regardless of which wallet originally created them.

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--input` | - | Mnemonic phrase (or interactive prompt if omitted) |
| `--passphrase` | `false` | Prompt for BIP39 passphrase |
| `--gap` | `20` | Gap limit for address scanning |
| `--scheme` | - | Scan only a specific scheme (e.g., `BSV Standard`) |
| `--path` | - | Custom derivation path to scan |
| `--migrate` | `false` | Consolidate discovered funds to a sigil wallet |
| `--wallet` | - | Target wallet name for migration (required with `--migrate`) |

**Supported Wallet Schemes:**

| Scheme | Derivation Path | Wallets |
|--------|-----------------|---------|
| BSV Standard | `m/44'/236'/0'/...` | RelayX, RockWallet, Twetch, Trezor, Ledger |
| Bitcoin Legacy | `m/44'/0'/0'/...` | MoneyButton, ElectrumSV imports |
| Bitcoin Cash | `m/44'/145'/0'/...` | Exodus, Simply.Cash, BCH fork splits |
| HandCash Legacy | `m/0'/...` | HandCash 1.x only |
| Multi-Account | `m/44'/236'/1-4'/...` | Power users with multiple accounts |

**Examples:**
```bash
# Interactive discovery (prompts for mnemonic)
sigil wallet discover

# Provide mnemonic directly
sigil wallet discover --input "abandon abandon abandon ... about"

# Use with BIP39 passphrase (Centbee uses 4-digit PIN as passphrase)
sigil wallet discover --passphrase

# Increase gap limit for wallets with many addresses
sigil wallet discover --gap 50

# Scan only a specific scheme
sigil wallet discover --scheme "Bitcoin Legacy"

# Scan a custom derivation path
sigil wallet discover --path "m/44'/0'/0'/0/*"

# Output as JSON for scripting
sigil wallet discover -o json

# Discover and consolidate funds to your sigil wallet
sigil wallet discover --migrate --wallet main
```

**Sample Output:**
```
Scanning derivation paths...
  BSV Standard...
    Found: 1ABC...xyz (52340 sats)
  Bitcoin Legacy...
    Found: 1DEF...uvw (120000 sats)

═══════════════════════════════════════════════════════════════
                    DISCOVERED FUNDS
═══════════════════════════════════════════════════════════════

Scheme              Address              Path                    Balance
----------------    -----------------    --------------------    ----------
BSV Standard        1ABC...xyz           m/44'/236'/0'/0/3       0.00052340 BSV
Bitcoin Legacy      1DEF...uvw           m/44'/0'/0'/0/0         0.00120000 BSV

───────────────────────────────────────────────────────────────
Total: 0.00172340 BSV (2 addresses, 3 UTXOs)
Scan Time: 12.3s
═══════════════════════════════════════════════════════════════

Use --migrate --wallet <name> to consolidate funds.
```

**Known Limitations:**

| Wallet | Status | Notes |
|--------|--------|-------|
| HandCash 2.0+ | Not supported | Uses proprietary non-exportable keys |
| Centbee | Partial | Uses 4-digit PIN as BIP39 passphrase |

> **Tip:** If you're migrating from HandCash 2.0 or later, you'll need to use the HandCash app to transfer funds to another wallet first, as these versions don't allow mnemonic export.

<br>

---

<br>

### balance

Check cryptocurrency balances.

```bash
sigil balance <subcommand>
```

#### balance show

Show balances for all addresses in a wallet across supported chains.

```bash
sigil balance show [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--chain` | - | Filter by chain (`eth`, `bsv`) |
| `--refresh` | `false` | Force fresh fetch from network, ignoring cache |

**Examples:**
```bash
sigil balance show --wallet main
sigil balance show --wallet main --chain eth
sigil balance show --wallet main --refresh
sigil balance show --wallet main -o json
```

**Smart Caching:**

Balance fetching uses an intelligent caching strategy that balances speed with accuracy based on address activity:

- **Active addresses** (with non-zero balance): Always fetched fresh from the network for maximum accuracy
- **Inactive addresses** (used before but now empty): Cached for 30 minutes
- **Never-used addresses**: Cached for 2 hours
- **Recently created addresses** (< 24 hours old): Always fetched fresh

This approach significantly reduces API calls (typically 80% reduction) and improves command speed, especially for wallets with many addresses. Active addresses always get fresh data, while inactive addresses benefit from extended cache windows without sacrificing meaningful accuracy.

Use the `--refresh` flag to bypass caching entirely and force fresh fetches for all addresses.

**Unconfirmed Balances:**

When pending (unconfirmed) transactions are detected, an additional "Unconfirmed" column appears in the balance table showing the net change from mempool transactions. Negative values indicate outgoing funds; positive values indicate incoming funds.

- **BSV:** The WhatsOnChain API natively reports confirmed and unconfirmed balances.
- **ETH (RPC provider):** Pending balance is computed by comparing `eth_getBalance` at the `latest` vs `pending` block tags.
- **ETH (Etherscan provider):** Unconfirmed data is not available from the Etherscan API.

If no addresses have pending transactions, the table uses the standard "Balance" column header.

<br>

---

<br>

### receive

Show or generate a receiving address for your wallet.

```bash
sigil receive [flags]
```

By default, shows the first unused address. The same address is shown until it receives funds, then the next unused address is returned. Use `--new` to force generation of a new address even if the current one hasn't been used yet.

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--wallet` | `-w` | - | Wallet name (required) |
| `--chain` | `-c` | `bsv` | Blockchain: `eth`, `bsv` |
| `--new` | - | `false` | Force generation of a new address |
| `--label` | `-l` | - | Set a label for the address |
| `--qr` | - | `false` | Display QR code for the address |
| `--check` | - | `false` | Check for received funds and refresh local UTXO state |
| `--address` | - | - | Specific address to check (use with `--check`) |
| `--all` | - | `false` | Check all receiving addresses (use with `--check`) |

**Flag Constraints:**
- `--check` and `--new` are mutually exclusive (cannot use both together)
- `--address` and `--all` are mutually exclusive (cannot use both together)
- `--address` requires `--check`
- `--all` requires `--check`

**Examples:**
```bash
# Show next unused BSV receiving address
sigil receive --wallet main --chain bsv

# Label the current unused receiving address
sigil receive --wallet main --chain bsv --label "Payment from Alice"

# Generate a new address even if current is unused
sigil receive --wallet main --chain bsv --new

# Generate a new address with a label
sigil receive --wallet main --chain bsv --new --label "Payment from Alice"

# Show ETH receiving address
sigil receive --wallet main --chain eth

# Show address with QR code for mobile wallet scanning
sigil receive --wallet main --chain bsv --qr

# Check if funds have arrived at your current receive address
sigil receive --wallet main --chain bsv --check

# Check a specific address for funds
sigil receive --wallet main --chain bsv --check --address 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa

# Check all BSV receiving addresses for funds
sigil receive --wallet main --chain bsv --check --all

# Check all ETH receiving addresses for balances (requires ETHERSCAN_API_KEY)
sigil receive --wallet main --chain eth --check --all

# Check all chains at once (BSV + ETH) — omit --chain
sigil receive --wallet main --check --all

# Check all addresses with JSON output
sigil receive --wallet main --check --all -o json
```

<br>

---

<br>

### addresses

Manage and view wallet addresses.

```bash
sigil addresses <subcommand>
```

#### addresses list

List all addresses in a wallet with their status and balance.

Balances are fetched live from the network with cache fallback. When any address has pending transactions, the table shows separate "Confirmed" and "Unconfirmed" columns. An address is considered "used" if it has historical activity in the UTXO store or has a non-zero confirmed/unconfirmed balance.

```bash
sigil addresses list [flags]
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--wallet` | `-w` | - | Wallet name (required) |
| `--chain` | `-c` | - | Filter by chain (`eth`, `bsv`) |
| `--type` | `-t` | `all` | Filter: `receive`, `change`, `all` |
| `--used` | - | `false` | Show only used addresses |
| `--unused` | - | `false` | Show only unused addresses |
| `--refresh` | - | `false` | Force fresh fetch, ignore cache |

**Examples:**
```bash
# List all BSV addresses
sigil addresses list --wallet main --chain bsv

# List only receiving addresses
sigil addresses list --wallet main --type receive

# List only unused addresses
sigil addresses list --wallet main --unused

# List only change addresses that have been used
sigil addresses list --wallet main --type change --used

# Force fresh balance fetch
sigil addresses list --wallet main --refresh

# Output as JSON
sigil addresses list --wallet main -o json
```

#### addresses refresh

Refresh balance and UTXO data for wallet addresses from the blockchain.

For BSV addresses, this re-scans UTXOs via WhatsOnChain and updates the local UTXO store and balance cache. For ETH addresses, this fetches fresh balances via the configured provider and updates the balance cache.

By default, refreshes all addresses. Use `--address` to target specific addresses.

```bash
sigil addresses refresh [flags]
```

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--wallet` | `-w` | - | Wallet name (required) |
| `--chain` | `-c` | - | Filter by chain (`eth`, `bsv`) |
| `--address` | - | - | Specific address(es) to refresh (repeatable) |

**Examples:**
```bash
# Refresh all addresses
sigil addresses refresh --wallet main

# Refresh BSV addresses only
sigil addresses refresh --wallet main --chain bsv

# Refresh specific addresses
sigil addresses refresh --wallet main --address 1ABC... --address 1XYZ...

# JSON output
sigil addresses refresh --wallet main -o json
```

<br>

---

<br>

#### addresses label

Set or update the label for an address.

```bash
sigil addresses label <address> <label> [flags]
```

**Arguments:**
- `<address>` - The address to label (required)
- `<label>` - Label text, use empty string "" to clear (required)

**Flags:**
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--wallet` | `-w` | - | Wallet name (required) |

**Examples:**
```bash
# Set a label
sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "Savings" --wallet main

# Clear a label
sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "" --wallet main
```

<br>

---

<br>

### tx

Manage transactions.

```bash
sigil tx <subcommand>
```

#### tx send

Send ETH, USDC, or BSV to an address.

```bash
sigil tx send [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--to` | - | Recipient address (required) |
| `--amount` | - | Amount to send, or `all` for entire balance (required) |
| `--chain` | `eth` | Blockchain: `eth`, `bsv` |
| `--token` | - | ERC-20 token symbol (e.g., `USDC`) - ETH only |
| `--gas` | `medium` | Gas speed: `slow`, `medium`, `fast` |
| `--yes` | `false` | Skip confirmation prompt |

**Examples:**
```bash
# Send ETH
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

# Send all ETH (entire balance minus gas fees)
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount all --chain eth

# Send USDC
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 100 --chain eth --token USDC

# Send all USDC (entire token balance)
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount all --chain eth --token USDC

# Send BSV
sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv

# Send all BSV (entire balance minus mining fee)
sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount all --chain bsv
```

**Send All (`--amount all`):**

Use `--amount all` to send your entire balance. Fees are deducted automatically from the send amount, so the transaction always succeeds if you have enough to cover fees. The confirmation prompt shows the exact calculated amount before broadcast. For BSV, this consolidates all UTXOs into a single output with no change. For ETH, the send amount is `balance - gas cost`. For ERC-20 tokens, the full token balance is sent (ETH is still needed for gas).

**BSV Change Addresses:**

When sending BSV (with a specific amount, not `--amount all`), any change (remaining balance after sending the requested amount plus fees) is sent to a new change address on the BIP44 internal chain (`m/44'/236'/0'/1/x`). This improves privacy by avoiding address reuse. You can view your change addresses with `sigil addresses list --type change`.

<br>

---

<br>

### utxo

Manage unspent transaction outputs (UTXOs) for BSV wallets.

```bash
sigil utxo <subcommand>
```

#### utxo list

List all unspent transaction outputs (UTXOs) for a BSV wallet address by querying the chain directly.

```bash
sigil utxo list [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--chain` | `bsv` | Blockchain (only `bsv` supported) |

**Examples:**
```bash
sigil utxo list --wallet main
sigil utxo list --wallet main -o json
```

#### utxo refresh

Re-scan all known addresses and update the local UTXO store. New UTXOs are added; spent UTXOs are marked as spent.

```bash
sigil utxo refresh [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |

**Examples:**
```bash
sigil utxo refresh --wallet main
```

#### utxo balance

Display balance calculated from locally stored UTXOs. No network connection required after initial scan.

```bash
sigil utxo balance [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |

**Examples:**
```bash
sigil utxo balance --wallet main
sigil utxo balance --wallet main -o json
```

<br>

---

<br>

### config

View and modify Sigil configuration settings.

```bash
sigil config <subcommand>
```

#### config init

Create a default configuration file.

```bash
sigil config init [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Overwrite existing configuration |

**Examples:**
```bash
sigil config init
sigil config init --force
```

#### config show

Display the current configuration settings.

```bash
sigil config show
```

**Examples:**
```bash
sigil config show
sigil config show -o json
```

#### config get

Get a specific configuration value by its path using dot notation.

```bash
sigil config get <path>
```

**Arguments:**
- `<path>` - Configuration path in dot notation (required)

**Examples:**
```bash
sigil config get networks.eth.rpc
sigil config get output.default_format
sigil config get logging.level
```

#### config set

Set a specific configuration value by its path using dot notation.

```bash
sigil config set <path> <value>
```

**Arguments:**
- `<path>` - Configuration path in dot notation (required)
- `<value>` - New value (required)

**Examples:**
```bash
sigil config set networks.eth.rpc https://mainnet.infura.io/v3/YOUR_KEY
sigil config set output.default_format json
sigil config set logging.level debug
```

<br>

---

<br>

### session

Manage authentication sessions. When enabled, sigil caches your wallet credentials for a configurable time (default: 15 minutes) so you don't need to enter your password for every command.

Sessions use your operating system's secure keychain:
- macOS: Keychain
- Linux: Secret Service (GNOME Keyring, KWallet)
- Windows: Credential Manager

If the system keychain is unavailable, sessions are disabled and you'll be prompted for your password each time.

```bash
sigil session <subcommand>
```

#### session status

Show active sessions and remaining time.

```bash
sigil session status
```

**Example:**
```bash
$ sigil session status
Active Sessions:
  main: expires in 12m30s
  backup: expires in 8m15s
```

#### session lock

End all active sessions immediately. Use this when stepping away from your computer.

```bash
sigil session lock
```

**Example:**
```bash
$ sigil session lock
Ended 2 session(s)
```

<br>

---

<br>

### agent

Manage agent tokens for programmatic wallet access. Agent tokens allow AI agents and bots to use wallets non-interactively with policy-limited access — spending caps, chain restrictions, address allowlists, and expiration.

**Two access tiers:**

| Mode | Env Variable | Capabilities | Secrets on Disk? |
|------|-------------|--------------|------------------|
| Agent Token | `SIGIL_AGENT_TOKEN` | Send, receive, balance | Encrypted seed in agent file |
| xpub Read-Only | `SIGIL_AGENT_XPUB` | Balance, receive, addresses | None (xpub is public) |

```bash
sigil agent <subcommand>
```

#### agent create

Create a new agent token with spending policy. You will be prompted for the wallet password once. A random token is generated and displayed — store it securely, it will not be shown again.

```bash
sigil agent create [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--chains` | - | Comma-separated chain list: `bsv`, `eth` (required) |
| `--max-per-tx` | `0` | Max amount per transaction in satoshis (e.g., `50000sat` or `0.0005`) |
| `--max-daily` | `0` | Max daily aggregate spend in satoshis (e.g., `500000sat` or `0.005`) |
| `--max-per-tx-eth` | - | Max per-tx ETH in wei or decimal (e.g., `0.001`) |
| `--max-daily-eth` | - | Max daily ETH in wei or decimal (e.g., `0.01`) |
| `--allowed-addrs` | - | Comma-separated address allowlist (empty = any destination) |
| `--expires` | - | Token lifetime: e.g., `1d`, `7d`, `30d`, `90d`, `365d` (required) |
| `--label` | - | Human-readable label for this agent (required) |

**Examples:**
```bash
# BSV-only agent with spending limits
sigil agent create --wallet main --chains bsv --max-per-tx 50000sat --max-daily 500000sat --expires 30d --label "payment-bot"

# Multi-chain agent
sigil agent create --wallet main --chains bsv,eth --max-per-tx 50000sat --max-daily 500000sat --expires 7d --label "trading-bot"

# Agent restricted to specific addresses
sigil agent create --wallet main --chains bsv --max-per-tx 100000sat --max-daily 1000000sat --allowed-addrs "1ABC...,1DEF..." --expires 90d --label "payroll"

# Unlimited (use with caution)
sigil agent create --wallet main --chains bsv,eth --expires 1d --label "test-bot"
```

**Sample output:**
```
Agent created for wallet 'main':
  ID:           agt_7f3a2b
  Label:        payment-bot
  Chains:       bsv, eth
  Per-tx limit: 50,000 sat
  Daily limit:  500,000 sat
  Expires:      2026-03-11

  Token (store securely, shown once):
  SIGIL_AGENT_TOKEN=sigil_agt_K7x9mP...base64...==

  xpub (BSV): xpub6D4BDPc...
  xpub (ETH): xpub6CzR1Hq...

  Read-only env var (no spending ability):
  SIGIL_AGENT_XPUB=xpub6D4BDPc...
```

#### agent list

List all agent tokens for a wallet with their ID, label, allowed chains, and expiration status. Does not require the wallet password.

```bash
sigil agent list [flags]
sigil agent ls    # alias
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |

**Examples:**
```bash
sigil agent list --wallet main
sigil agent list --wallet main -o json
```

**JSON output schema:**
```json
{
  "agents": [
    {
      "id": "agt_7f3a2b",
      "label": "payment-bot",
      "wallet": "main",
      "chains": ["bsv", "eth"],
      "created_at": "2026-02-09T12:00:00Z",
      "expires_at": "2026-03-11T12:00:00Z",
      "expired": false,
      "policy": {
        "max_per_tx_sat": 50000,
        "max_daily_sat": 500000
      }
    }
  ]
}
```

#### agent info

Show detailed agent token information including policy, daily spending status, and xpub for read-only access. Does not require the wallet password.

```bash
sigil agent info [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--id` | - | Agent ID (required, e.g., `agt_7f3a2b`) |

**Examples:**
```bash
sigil agent info --wallet main --id agt_7f3a2b
sigil agent info --wallet main --id agt_7f3a2b -o json
```

#### agent revoke

Revoke one or all agent tokens for a wallet. Revoked tokens are immediately deleted and can no longer authenticate. This is irreversible. Does not require the wallet password.

```bash
sigil agent revoke [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |
| `--id` | - | Agent ID to revoke |
| `--all` | `false` | Revoke all agents for this wallet |

**Flag Constraints:**
- Exactly one of `--id` or `--all` is required (they are mutually exclusive)

**Examples:**
```bash
# Revoke a specific agent
sigil agent revoke --wallet main --id agt_7f3a2b

# Revoke all agents for a wallet
sigil agent revoke --wallet main --all
```

#### Agent Usage (Non-Interactive)

Once created, the agent sets the token in its environment and uses normal sigil commands:

```bash
# Spending operations (requires SIGIL_AGENT_TOKEN)
SIGIL_AGENT_TOKEN=sigil_agt_... sigil tx send --wallet main --to 1ABC... --amount 5000sat --chain bsv

# Balance check (xpub only, zero secrets)
SIGIL_AGENT_XPUB=xpub6D4BDPc... sigil balance show --wallet main --chain bsv

# Generate receive address (xpub only)
SIGIL_AGENT_XPUB=xpub6D4BDPc... sigil receive --wallet main --chain bsv
```

When `SIGIL_AGENT_TOKEN` is set, output defaults to JSON (machine-readable) and confirmation prompts are skipped automatically.

#### Spending Policy

Agent tokens enforce spending limits at two levels:

- **Per-transaction limit**: Maximum amount for a single `tx send` command
- **Daily limit**: Maximum aggregate spend per UTC day (resets at midnight UTC)

Additional restrictions:
- **Chain authorization**: Agent can only transact on chains specified at creation
- **Address allowlist**: Optional restriction to specific destination addresses
- **Expiration**: Token becomes invalid after the specified lifetime

Daily spending is tracked in `~/.sigil/agents/{wallet}-{id}.counter` with HMAC integrity protection.

#### Agent Error Codes

When policy enforcement fails, structured JSON errors are returned:

| Code | Exit | Description |
|------|------|-------------|
| `AGENT_TOKEN_INVALID` | 3 | Token doesn't match any agent file |
| `AGENT_TOKEN_EXPIRED` | 3 | Agent token has expired |
| `AGENT_POLICY_VIOLATION` | 5 | Transaction exceeds per-tx limit |
| `AGENT_DAILY_LIMIT` | 5 | Daily spending limit reached |
| `AGENT_CHAIN_DENIED` | 2 | Agent not authorized for this chain |
| `AGENT_ADDR_DENIED` | 2 | Destination address not in allowlist |
| `AGENT_XPUB_INVALID` | 2 | xpub string is malformed |
| `AGENT_XPUB_WRITE_DENIED` | 3 | Spending attempted with xpub-only auth |

**Example error response:**
```json
{
  "error": {
    "code": "AGENT_DAILY_LIMIT",
    "message": "daily spending limit reached",
    "suggestion": "Wait until tomorrow or create a new agent with a higher daily limit",
    "exit_code": 5
  }
}
```

#### Security Model

**Protects against:**
- Runaway agent spending (per-tx + daily limits)
- Agent code bugs / LLM hallucinations (policy enforcement)
- Token leak without file access (token alone is useless without the agent file on disk)
- Chain restriction violation (BSV-only agent cannot spend ETH)
- Stale credentials (tokens expire)

**Does not protect against:**
- Full machine compromise where attacker gets both token + agent file (they could decrypt the seed). Mitigated by low-value wallets + spending limits capping damage.

**xpub security:** The xpub reveals addresses (privacy impact if leaked) but has zero spending capability. Ideal for monitoring and receiving agents.

<br>

---

<br>

### backup

Create, verify, and restore encrypted wallet backups.

```bash
sigil backup <subcommand>
```

#### backup create

Create an encrypted backup of a wallet.

```bash
sigil backup create [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--wallet` | - | Wallet name (required) |

**Example:**
```bash
sigil backup create --wallet main
```

#### backup verify

Verify the integrity of a backup file.

```bash
sigil backup verify [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--input` | - | Path to backup file (required) |

**Example:**
```bash
sigil backup verify --input ~/.sigil/backups/main-2024-01-15.sigil
```

#### backup restore

Restore a wallet from an encrypted backup file.

```bash
sigil backup restore [flags]
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--input` | - | Path to backup file (required) |
| `--name` | - | New name for restored wallet (optional) |

**Examples:**
```bash
sigil backup restore --input ~/.sigil/backups/main-2024-01-15.sigil
sigil backup restore --input backup.sigil --name restored_wallet
```

#### backup list

List all backup files in the backups directory.

```bash
sigil backup list
sigil backup ls    # alias
```

**Example:**
```bash
sigil backup list
```

<br>

---

<br>

### completion

Generate shell completion scripts for sigil.

```bash
sigil completion <shell>
```

**Arguments:**
- `<shell>` - Shell type: `bash`, `zsh`, `fish`, `powershell` (required)

#### Bash

```bash
source <(sigil completion bash)

# To load completions for each session, execute once:
# Linux:
sigil completion bash > /etc/bash_completion.d/sigil
# macOS:
sigil completion bash > $(brew --prefix)/etc/bash_completion.d/sigil
```

#### Zsh

```bash
# If shell completion is not already enabled:
echo "autoload -U compinit; compinit" >> ~/.zshrc

# To load completions for each session, execute once:
sigil completion zsh > "${fpath[1]}/_sigil"
```

#### Fish

```bash
sigil completion fish | source

# To load completions for each session, execute once:
sigil completion fish > ~/.config/fish/completions/sigil.fish
```

#### PowerShell

```powershell
sigil completion powershell | Out-String | Invoke-Expression

# To load completions for every new session:
sigil completion powershell > sigil.ps1
# and source this file from your PowerShell profile.
```

<br>

---

<br>

### version

Display version, build commit, and build date.

```bash
sigil version
```

<br>

---

<br>

## Environment Variables

Environment variables override configuration file settings.

| Variable | Description |
|----------|-------------|
| `SIGIL_HOME` | Sigil data directory (default: `~/.sigil`) |
| `SIGIL_ETH_RPC` | Ethereum RPC endpoint URL (default: PublicNode gateway) |
| `SIGIL_ETH_PROVIDER` | ETH balance provider: `etherscan` (default) or `rpc` |
| `ETHERSCAN_API_KEY` | Etherscan API key (required when provider is `etherscan`) |
| `SIGIL_BSV_API_KEY` | WhatsOnChain API key (optional, fallback: `WHATS_ON_CHAIN_API_KEY`) |
| `SIGIL_BSV_FEE_STRATEGY` | BSV fee strategy: `economy`, `normal` (default), `priority` |
| `SIGIL_BSV_MIN_MINERS` | Minimum miners for normal fee strategy (default: 2) |
| `SIGIL_OUTPUT_FORMAT` | Default output format (`text`, `json`, `auto`) |
| `SIGIL_VERBOSE` | Enable verbose output (`true`, `yes`, `on`, `1`) |
| `SIGIL_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `SIGIL_SESSION_TTL` | Session timeout in minutes (default: 15) |
| `SIGIL_AGENT_TOKEN` | Agent token for non-interactive wallet access (see [Agent Mode](#agent)) |
| `SIGIL_AGENT_XPUB` | xpub for read-only balance/receive operations (see [Agent Mode](#agent)) |
| `NO_COLOR` | Disable colored output (any value) |

**Examples:**
```bash
export SIGIL_HOME=/custom/path
export SIGIL_ETH_RPC=https://mainnet.infura.io/v3/YOUR_KEY
export ETHERSCAN_API_KEY=your_etherscan_api_key
export SIGIL_OUTPUT_FORMAT=json
export SIGIL_LOG_LEVEL=debug
export SIGIL_SESSION_TTL=30  # 30 minute sessions
```

<br>

---

<br>

## Configuration Reference

Configuration is stored at `~/.sigil/config.yaml`.

```yaml
# Sigil data directory
home: ~/.sigil

# Output settings
output:
  default_format: auto    # text, json, auto
  verbose: false
  color: auto             # auto, always, never

# Logging settings
logging:
  level: error            # debug, info, warn, error
  file: ~/.sigil/sigil.log

# Security settings
security:
  session_enabled: true   # Enable session caching
  session_ttl_minutes: 15 # Session duration in minutes

# Fee settings
fees:
  bsv_fee_strategy: normal  # economy, normal, priority
  bsv_min_miners: 2         # Minimum miners for normal strategy
  eth_gas_strategy: medium  # slow, medium, fast

# Network settings
networks:
  eth:
    provider: etherscan             # "etherscan" (default) or "rpc"
    etherscan_api_key: ""           # Or set ETHERSCAN_API_KEY env var
    rpc: https://ethereum-rpc.publicnode.com  # Fallback RPC (or primary when provider=rpc)
  bsv:
    api_key: ""           # WhatsOnChain API key (optional)
```

### Configuration Paths

Use dot notation with `config get` and `config set`:

| Path | Description | Valid Values |
|------|-------------|--------------|
| `home` | Data directory | Any path |
| `output.default_format` | Default output format | `text`, `json`, `auto` |
| `output.verbose` | Verbose output | `true`, `false` |
| `output.color` | Color output | `auto`, `always`, `never` |
| `logging.level` | Log level | `debug`, `info`, `warn`, `error` |
| `logging.file` | Log file path | Any path |
| `security.session_enabled` | Enable session caching | `true`, `false` |
| `security.session_ttl_minutes` | Session duration | `1`-`60` |
| `fees.bsv_fee_strategy` | BSV fee strategy | `economy`, `normal`, `priority` |
| `fees.bsv_min_miners` | Minimum miners for normal strategy | Any integer > 0 |
| `fees.eth_gas_strategy` | ETH gas speed | `slow`, `medium`, `fast` |
| `networks.eth.provider` | ETH balance provider | `etherscan`, `rpc` |
| `networks.eth.etherscan_api_key` | Etherscan API key | Any string |
| `networks.eth.rpc` | Ethereum RPC URL | Any URL |
| `networks.bsv.api_key` | WhatsOnChain API key | Any string |
