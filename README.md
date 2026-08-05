<div align="center">

# 🔮&nbsp;&nbsp;Sigil

**Terminal-based multi-chain cryptocurrency wallet for power users**

<br/>

<a href="https://github.com/mrz1836/sigil/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/sigil?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/sigil?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/sigil/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/sigil?style=flat-square&color=blue&v=2" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/sigil/actions"><img src="https://img.shields.io/github/actions/workflow/status/mrz1836/sigil/fortress.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/mrz1836/sigil/actions"><img src="https://img.shields.io/github/last-commit/mrz1836/sigil?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://codecov.io/gh/mrz1836/sigil"><img src="https://codecov.io/gh/mrz1836/sigil/branch/master/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/mrz1836/sigil"><img src="https://api.scorecard.dev/projects/github.com/mrz1836/sigil/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/sigil/graphs/contributors"><img src="https://img.shields.io/github/contributors/mrz1836/sigil?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://mrz1818.com/"><img src="https://img.shields.io/badge/donate-bitcoin-ff9900?style=flat-square&logo=bitcoin" alt="Bitcoin"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="33%">
       🚀&nbsp;<a href="#-installation"><code>Installation</code></a>
    </td>
    <td align="center" width="33%">
       ⚡&nbsp;<a href="#-quick-start"><code>Quick&nbsp;Start</code></a>
    </td>
    <td align="center" width="33%">
       📚&nbsp;<a href="#-documentation"><code>Documentation</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🔐&nbsp;<a href="#-security"><code>Security</code></a>
    </td>
    <td align="center">
      🛠️&nbsp;<a href="#-code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
    <td align="center">
      🧪&nbsp;<a href="#-examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       ⚖️&nbsp;<a href="#-license"><code>License</code></a>
    </td>
    <td align="center">
       🤝&nbsp;<a href="#-contributing"><code>Contributing</code></a>
    </td>
  </tr>
  <tr>
    <td align="center" colspan="3">
       👥&nbsp;<a href="#-maintainers"><code>Maintainers</code></a>
    </td>
  </tr>
</table>

<br/>

### 🎥 Demo

<img src="examples/demo.gif" alt="Sigil Demo" title="Sigil Demo"/>

<br/>

## 🚀 Installation

Install the latest prebuilt release for your platform with a single copy‑paste. It
lands in `~/.local/bin` — a user‑writable directory, so no `sudo`, and `sigil update`
can self‑update in place afterward:

```bash
# Install the latest sigil release into ~/.local/bin
VER=$(curl -fsSL https://api.github.com/repos/mrz1836/sigil/releases/latest | grep '"tag_name"' | cut -d'"' -f4 | tr -d v)
OS=$(uname -s | tr '[:upper:]' '[:lower:]'); ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
mkdir -p ~/.local/bin
curl -fsSL "https://github.com/mrz1836/sigil/releases/download/v${VER}/sigil_${VER}_${OS}_${ARCH}.tar.gz" | tar -xzf - -C ~/.local/bin sigil
sigil --version
```

If `sigil` isn't found afterward, add `~/.local/bin` to your `PATH` (put
`export PATH="$HOME/.local/bin:$PATH"` in your `~/.zshrc` or `~/.bashrc`).

<details>
<summary><strong>Build from source (contributors)</strong></summary>

Requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy).

```bash
git clone https://github.com/mrz1836/sigil.git
cd sigil
go build -o bin/sigil ./cmd/sigil
```

</details>

<br/>

## ⚡ Quick Start

Get up and running with these essential commands:

<br>

### Create a wallet

```bash
sigil wallet create main
```

Creates a new HD wallet with BIP39 mnemonic phrase.

<br>

### Check balances

```bash
sigil balance show --wallet main
```

Displays balances across all supported chains (ETH, USDC, BSV, BTC).

<br>

### Get a receiving address

```bash
sigil receive --wallet main --chain bsv --qr --label "Re-up time!"
```

Generates a new receiving address for the specified chain.

<br>

### Check for incoming funds

```bash
sigil receive --wallet main --chain bsv --check
```
Checks for incoming transactions to your wallet.

<br>

### Send a transaction

```bash
sigil tx send --wallet main --to 0x742d35Cc663... --amount 0.00001 --chain eth
sigil tx send --wallet main --to 1A1zP1eP5QGef... --amount 0.00001 --chain bsv
```

Sends cryptocurrency to the specified address.

<br>

### Use BSV testnet

```bash
# Create a testnet wallet (addresses start with m/n)
sigil wallet create tnet --testnet

# Get a testnet receive address, then fund it from the faucet
sigil receive --wallet tnet --chain bsv
# Faucet: https://faucet.bananablocks.com/  •  Explorer: https://test.bananablocks.com/

# Check the balance and send on testnet
sigil balance show --wallet tnet --refresh
sigil tx send --wallet tnet --to <testnet-address> --amount 0.00001 --chain bsv
```

The BSV network is a per-wallet setting stamped at creation (or via `--network test`,
`SIGIL_BSV_NETWORK=test`, or `networks.bsv.network` in config). See the
[CLI Documentation](docs/CLI.md#bsv-testnet) for details.

Bitcoin (BTC) uses the same per-wallet mechanism for **testnet4** (`--network test`,
`SIGIL_BTC_NETWORK=test`, or `networks.btc.network`), served by mempool.space:

```bash
# On a testnet wallet, receive a BTC testnet4 address (starts with m/n or tb1q)
sigil receive --wallet tnet --chain btc
# Fund from a testnet4 faucet, then send (pay any address type, incl. bech32)
sigil tx send --wallet tnet --to tb1q... --amount 0.00001 --chain btc
# Explorer: https://mempool.space/testnet4
```

<br>

### Back up your wallet

```bash
sigil backup create --wallet main
```

Creates an encrypted backup of your wallet.

<br>

### Keep sigil up to date

`sigil update` (alias `sigil upgrade`) downloads the latest release, verifies its
SHA‑256 checksum against the published `sigil_<ver>_checksums.txt`, and atomically
replaces the running binary — no `sudo` when it lives in `~/.local/bin`.

```bash
sigil update            # download & install the latest release
sigil update --check    # report whether a newer version is available
sigil update --force    # reinstall the latest even if already current
sigil update --verbose  # narrate each step
```

Every other command also runs a passive, cached background check and prints a one‑line
"a new version is available" notice. It never blocks or fails a command, is skipped for
development builds, and is silenced by `SIGIL_NO_UPDATE_CHECK=1` (or the shared
`NO_UPDATE_CHECK` / `CI`). A GitHub token, if you hit API rate limits, is read from
`SIGIL_GITHUB_TOKEN`, then `GITHUB_TOKEN`, then `GH_TOKEN`.

> **Heads up:** a binary that another installer owns — `go install`'s `~/go/bin`, or a
> Homebrew prefix — is **refused** by `sigil update` rather than overwritten (that would
> break the tool that owns it). Install the release binary into `~/.local/bin` as above
> to keep self‑update working.

See [`update`](docs/CLI.md#update) for flags.

<br/>

> 📖 **For complete command reference and advanced features, see the [CLI Documentation →](docs/CLI.md)**

<br/>

## 📚 Documentation

View the comprehensive documentation for Sigil:

| Document                        | Description                                          |
|---------------------------------|------------------------------------------------------|
| **[CLI.md](docs/CLI.md)**       | Complete command reference and usage guide           |

<br>

> **Heads up!** Sigil is designed with minimal dependencies and maximum security. All cryptographic operations use battle-tested libraries:
> - **filippo.io/age** for encryption
> - **golang.org/x/crypto** for cryptographic primitives
> - **cosmos/go-bip39** for BIP39 mnemonic generation

<br/>

### Supported Chains

| Chain | Status | Description |
|-------|--------|-------------|
| ✅ Bitcoin SV (BSV) | **Supported** | UTXO-based transaction support (mainnet + testnet) |
| ✅ Bitcoin (BTC) | **Supported** | UTXO-based transaction support (mainnet + testnet4) |
| ✅ Ethereum (ETH) | **Supported** | Full transaction and balance support |
| ✅ USDC | **Supported** | ERC-20 token on Ethereum network |
| 🚧 Bitcoin Cash (BCH) | **Planned** | Coming in future release |

<br/>

### Key Features

- 🔑 **HD Wallet Support** — BIP39 mnemonic phrases with BIP32/BIP44 derivation
- 🧪 **BSV Testnet** — Per-wallet mainnet/testnet with faucet-friendly `m`/`n` addresses
- 🛡️ **Shamir's Secret Sharing** — Split your wallet seed into multiple shares for enhanced security
- 💰 **Multi-Chain Balances** — Check balances across all supported networks
- 📤 **Transaction Management** — Create, sign, and broadcast transactions
- 🔐 **Secure Sessions** — Encrypted session management using OS keychain
- 🤖 **Agent Tokens** — Programmatic access for automation
- 💾 **Encrypted Backups** — Secure wallet backup and restoration
- 🧩 **UTXO Management** — Advanced coin control for Bitcoin-based chains
- 📱 **QR Code Support** — Terminal-based QR code generation and scanning

<br/>

## 🔐 Security

### Important Disclaimer

> ⚠️ **Experimental Software — Use at Your Own Risk**
>
> Sigil is experimental, open-source software provided "AS-IS" without warranty. By using Sigil, you acknowledge:
>
> - **You control your keys:** Sigil never transmits or stores your private keys. Lost mnemonics are unrecoverable.
> - **Transactions are final:** Cryptocurrency transactions are irreversible.
> - **No formal audit:** This software has not undergone professional security auditing.
> - **No liability:** Authors accept no responsibility for loss of funds or damages.
>
> **Do not use Sigil with funds you cannot afford to lose.**

For security issues, see our [Security Policy](.github/SECURITY.md) or contact: [sigil@mrz1818.com](mailto:sigil@mrz1818.com)

<br/>

### Additional Documentation & Repository Management

<details>
<summary><strong><code>Development Setup (Getting Started)</code></strong></summary>
<br/>

Install [MAGE-X](https://github.com/mrz1836/go-mage) build tool for development:

```bash
# Install MAGE-X for development and building
go install github.com/magefile/mage@latest
go install github.com/mrz1836/go-mage/magex@latest
magex update:install
```
</details>

<details>
<summary><strong><code>Wallet Discovery & Migration</code></strong></summary>
<br/>

Sigil can discover and sweep funds from other BSV wallets by scanning multiple BIP44 derivation paths. This is essential for recovering funds from defunct providers or migrating from other wallets.

### Supported Derivation Schemes

Sigil automatically scans these derivation paths to find your funds:

| Derivation Scheme | Path | Supported Wallets |
|-------------------|------|-------------------|
| **BSV Standard** | `m/44'/236'/0'/...` | [RelayX](https://relayx.com/), [RockWallet](https://rockwallet.com/), [Twetch](https://twetch.com/), [Centbee](https://www.centbee.com/) †, Trezor, Ledger, KeepKey |
| **Bitcoin Legacy** | `m/44'/0'/0'/...` | [MoneyButton](https://www.moneybutton.com/) †, [ElectrumSV](https://electrumsv.io/) |
| **Bitcoin Cash** | `m/44'/145'/0'/...` | [Exodus](https://www.exodus.com/), Simply.Cash †, BCH fork splits |
| **HandCash Legacy** | `m/0'/...` | [HandCash 1.x](https://handcash.io/) (legacy version only) |

† *Service discontinued or shut down*

### Defunct BSV Services Supported

Sigil provides a recovery path for users of these defunct BSV services:

- **[Centbee](https://www.centbee.com/)** — Popular BSV mobile wallet that ceased operations in 2026. Uses BSV Standard derivation (`m/44'/236'/...`) with 4-digit PIN as BIP39 passphrase.
- **[MoneyButton](https://www.moneybutton.com/)** — Popular BSV wallet and identity provider that shut down in 2023. Used Bitcoin Legacy derivation (`m/44'/0'/...`).
- **Simply.Cash** — Mobile BSV wallet that ceased operations. Used Bitcoin Cash derivation path (`m/44'/145'/...`).
- **[HandCash 1.x](https://handcash.io/)** — Early versions of HandCash used a non-standard legacy path (`m/0'/...`). Note: HandCash 2.0+ uses proprietary non-exportable keys and cannot be imported.

### Active Wallets Supported

Sigil also supports migrating from active BSV wallets:

- **[RelayX](https://relayx.com/)** — BSV wallet and token platform
- **[RockWallet](https://rockwallet.com/)** — Multi-chain mobile wallet with BSV support
- **[Twetch](https://twetch.com/)** — BSV social media platform with integrated wallet
- **[ElectrumSV](https://electrumsv.io/)** — Desktop BSV wallet
- **[Exodus](https://www.exodus.com/)** — Multi-chain desktop/mobile wallet

### Hardware Wallets

- **Trezor** — Hardware wallet with BSV support
- **Ledger** — Hardware wallet with BSV support
- **KeepKey** — Hardware wallet with BSV support

### Usage

Discover funds from another wallet's mnemonic:

```bash
sigil wallet discover --mnemonic "your twelve or twenty-four word phrase"
```

For Centbee wallets (uses 4-digit PIN as passphrase):

```bash
sigil wallet discover --mnemonic "your phrase" --passphrase "1234"
```

See the [CLI Documentation](docs/CLI.md#wallet-discover) for complete details on wallet discovery and fund recovery.

</details>

<details>
<summary><strong><code>Binary Deployment</code></strong></summary>
<br/>

This project uses [goreleaser](https://github.com/goreleaser/goreleaser) for streamlined binary deployment to GitHub. To get started, install it via:

```bash
brew install goreleaser
```

The release process is defined in the [.goreleaser.yml](.goreleaser.yml) configuration file.

### Supported Platforms

- **Linux:** amd64, arm64
- **macOS:** amd64, arm64
- **Windows:** amd64, arm64

### Release Process

Then create and push a new Git tag using:

```bash
magex version:bump bump=patch push=true branch=master
```

This process ensures consistent, repeatable releases with properly versioned artifacts and citation metadata.

</details>

<details>
<summary><strong><code>Build Commands</code></strong></summary>
<br/>

View all build commands

```bash script
magex help
```

Common commands:
- `magex build` — Build the binary
- `magex test` — Run test suite
- `magex lint` — Run all linters
- `magex deps:update` — Update dependencies

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

Sigil uses the **Fortress** workflow system for comprehensive CI/CD:

- **fortress-test-suite.yml** — Complete test suite across multiple Go versions
- **fortress-code-quality.yml** — Code quality checks (gofmt, golangci-lint, staticcheck)
- **fortress-security-scans.yml** — Security vulnerability scanning
- **fortress-coverage.yml** — Code coverage reporting to Codecov
- **fortress-release.yml** — Automated binary releases via GoReleaser

See all workflows in [`.github/workflows/`](.github/workflows/).

</details>

<details>
<summary><strong><code>Updating Dependencies</code></strong></summary>
<br/>

To update all dependencies (Go modules, linters, and related tools), run:

```bash
magex deps:update
```

This command ensures all dependencies are brought up to date in a single step, including Go modules and any managed tools. It is the recommended way to keep your development environment and CI in sync with the latest versions.

</details>

<br/>

## 🧪 Examples & Tests

All unit tests run via [GitHub Actions](https://github.com/mrz1836/sigil/actions) and use [Go version 1.25.6](https://go.dev/doc/go1.25). View the [configuration file](.github/workflows/fortress.yml).

Run all tests (fast):

```bash script
magex test
```

Run all tests with race detector (slower):
```bash script
magex test:race
```

### Test Coverage

View coverage report:

```bash script
magex test:coverage
```

Coverage reports are automatically uploaded to [Codecov](https://codecov.io/gh/mrz1836/sigil) on every commit.

<br/>

## 🛠️ Code Standards
Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines
Read the [AI Usage & Assistant Guidelines](.github/CLAUDE.md) for details on how AI is used in this project and how to interact with AI assistants.

<br/>

## 👥 Maintainers
| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:------------------------------------------------------------------------------------------------:|
|                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## 🤝 Contributing
View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the [code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?
All kinds of contributions are welcome :raised_hands:!
The most basic way to show your support is to star :star2: the project, or to raise issues :speech_balloon:.
You can also support this project by [becoming a sponsor on GitHub](https://github.com/sponsors/mrz1836) :clap:
or by making a [**bitcoin donation**](https://mrz1818.com/?tab=tips&utm_source=github&utm_medium=sponsor-link&utm_campaign=sigil&utm_term=sigil&utm_content=sigil) to ensure this journey continues indefinitely! :rocket:


[![Stars](https://img.shields.io/github/stars/mrz1836/sigil?label=Please%20like%20us&style=social)](https://github.com/mrz1836/sigil/stargazers)

<br/>

## 📝 License

[![License](https://img.shields.io/github/license/mrz1836/sigil.svg?style=flat&v=2)](LICENSE)
