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

**Examples:**
```bash
sigil balance show --wallet main
sigil balance show --wallet main --chain eth
sigil balance show --wallet main -o json
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
| `--amount` | - | Amount to send (required) |
| `--chain` | `eth` | Blockchain: `eth`, `bsv` |
| `--token` | - | ERC-20 token symbol (e.g., `USDC`) - ETH only |
| `--gas` | `medium` | Gas speed: `slow`, `medium`, `fast` |
| `--yes` | `false` | Skip confirmation prompt |

**Examples:**
```bash
# Send ETH
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

# Send USDC
sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 100 --chain eth --token USDC

# Send BSV
sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv
```

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
| `SIGIL_ETH_RPC` | Ethereum RPC endpoint URL (default: Cloudflare gateway) |
| `SIGIL_BSV_API_KEY` | WhatsOnChain API key (optional) |
| `SIGIL_OUTPUT_FORMAT` | Default output format (`text`, `json`, `auto`) |
| `SIGIL_VERBOSE` | Enable verbose output (`true`, `yes`, `on`, `1`) |
| `SIGIL_LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |
| `SIGIL_SESSION_TTL` | Session timeout in minutes (default: 15) |
| `NO_COLOR` | Disable colored output (any value) |

**Examples:**
```bash
export SIGIL_HOME=/custom/path
export SIGIL_ETH_RPC=https://mainnet.infura.io/v3/YOUR_KEY
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

# Network settings
networks:
  eth:
    rpc: https://cloudflare-eth.com  # Default Cloudflare gateway (no API key needed)
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
| `networks.eth.rpc` | Ethereum RPC URL | Any URL |
| `networks.bsv.api_key` | WhatsOnChain API key | Any string |
