# Implementation Plan: Sigil MVP - Multi-Chain Wallet CLI

**Branch**: `001-sigil-mvp` | **Date**: 2026-01-31 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-sigil-mvp/spec.md`

## Summary

Build a secure, terminal-based multi-chain wallet CLI for power users. Primary capabilities: HD wallet creation/restoration with BIP39 mnemonics, balance checking and transactions for ETH/USDC and BSV, encrypted key storage with age encryption, and consistent JSON/text output formats. Follows noun-verb CLI pattern with comprehensive error handling.

## Technical Context

**Language/Version**: Go 1.24 (per PRD references)
**Primary Dependencies**:
- `github.com/spf13/cobra` — CLI framework with noun-verb command pattern
- `github.com/bitcoin-sv/go-sdk` — BSV transaction building and signing
- `github.com/ethereum/go-ethereum` — ETH/ERC-20 interactions
- `filippo.io/age` — Encryption for wallet files at rest
- `github.com/tyler-smith/go-bip39` — BIP39 mnemonic generation/validation
- `github.com/tyler-smith/go-bip32` — BIP32 HD key derivation
- `github.com/mrz1836/go-whatsonchain` — BSV balance/UTXO queries
- `gopkg.in/yaml.v3` — Configuration file parsing

**Storage**: File-based encrypted storage at `~/.sigil/` (YAML config, age-encrypted wallet files)
**Testing**: `go test` with table-driven tests, fuzz tests for input validation, build-tagged integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform CLI)
**Project Type**: Single CLI application
**Performance Goals**: Balance queries <10s, wallet creation <30s, transaction signing <5s
**Constraints**: Offline-capable for all key operations, no plaintext keys on disk
**Scale/Scope**: Single-user local wallet, 20-address gap limit default

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design. Constitution v1.1.0.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Security First | ✅ PASS | Age encryption for all wallet files (FR-010), no network for key ops, crypto/rand for entropy (FR-007), input validation at boundaries (FR-013, FR-021, FR-028) |
| II. Full Sovereignty | ✅ PASS | No telemetry, no cloud sync, local-only storage, standard export formats (WIF, hex, mnemonic per FR-011) |
| III. Power User UX | ✅ PASS | CLI with noun-verb pattern (FR-003), JSON output for all commands (FR-004), verbose/debug modes (FR-042) |
| IV. Transparency | ✅ PASS | Transaction details exposed before signing, fee estimation shown (FR-020, FR-027), actionable error messages (FR-036) |
| V. Fork Awareness | ✅ PASS | BIP44 chain-specific paths including BTC for future compatibility (FR-009), multi-chain balance queries (FR-016, FR-023). Fork scanner is SHOULD for MVP per constitution v1.1.0. |
| VI. CLI Design Standards | ✅ PASS | Noun-verb pattern (FR-003), standard flags (FR-004), exit codes (FR-006), auto-detect output format (FR-005) |
| VII. Testing Discipline | ✅ PASS | Spec requires 100% coverage for crypto operations, fuzz tests for input parsing, integration tests for APIs |

**Pre-design Gate**: PASSED - No violations. Proceeding to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/001-sigil-mvp/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (internal Go interfaces)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
cmd/
└── sigil/
    └── main.go              # Entry point

internal/
├── cli/                     # Cobra command implementations
│   ├── root.go              # Root command, global flags
│   ├── wallet.go            # wallet create/import/list/show/restore
│   ├── balance.go           # balance show
│   ├── tx.go                # tx send
│   ├── utxo.go              # utxo list
│   ├── config.go            # config init/show/set/get
│   ├── backup.go            # backup create/verify/restore
│   └── key.go               # key generate/export
├── wallet/                  # Core wallet logic
│   ├── wallet.go            # Wallet struct, Create, Restore
│   ├── mnemonic.go          # BIP39 generation, validation, typo correction
│   ├── derivation.go        # BIP32/44 key derivation
│   └── storage.go           # Encrypted file I/O
├── chain/                   # Chain-specific implementations
│   ├── chain.go             # Chain interface
│   ├── eth/
│   │   ├── client.go        # ETH RPC client
│   │   ├── balance.go       # ETH/ERC-20 balance queries
│   │   ├── tx.go            # ETH/ERC-20 transaction building
│   │   └── gas.go           # Gas estimation
│   └── bsv/
│       ├── client.go        # WhatsOnChain client wrapper
│       ├── balance.go       # BSV balance queries
│       ├── utxo.go          # UTXO listing
│       ├── tx.go            # P2PKH transaction building
│       └── fee.go           # Fee estimation
├── crypto/                  # Cryptographic utilities
│   ├── secure.go            # SecureBytes with mlock/zeroing
│   ├── entropy.go           # crypto/rand wrapper
│   └── age.go               # Age encryption/decryption
├── config/                  # Configuration management
│   ├── config.go            # Config struct, Load, Save
│   ├── defaults.go          # Default values
│   └── env.go               # Environment variable overrides
├── output/                  # Output formatting
│   ├── format.go            # Text/JSON formatters
│   ├── table.go             # Table rendering for text output
│   └── error.go             # Error formatting with suggestions
├── cache/                   # Balance caching
│   ├── cache.go             # Cache struct and operations
│   └── file.go              # File-based cache storage
└── backup/                  # Backup file handling
    ├── backup.go            # Create, Verify, Restore
    └── manifest.go          # Manifest structure

pkg/
└── errors/                  # Shared error types
    └── errors.go            # Sentinel errors, exit codes

testdata/                    # Test fixtures
├── mnemonics/               # Valid/invalid mnemonic test cases
├── wallets/                 # Sample encrypted wallet files
└── config/                  # Sample config files
```

**Structure Decision**: Single Go application with `cmd/` for entry point, `internal/` for all application logic (not intended for external import), and `pkg/` for shared error types that could theoretically be reused.

## Complexity Tracking

> No Constitution violations. Fork scanner is SHOULD (not MUST) per constitution v1.1.0.

| Principle | Implementation Notes | Status |
|-----------|---------------------|--------|
| V. Fork Awareness | Fork scanner deferred to Phase 2+ per SHOULD clause. BTC derivation path stored for future compatibility. | ✅ Compliant |

**Note**: Constitution v1.1.0 clarifies fork scanner as SHOULD for MVP. BTC path derivation is included to enable seamless Phase 2 upgrade.

---

## Phase 0: Research Output

See [research.md](./research.md) for complete findings.

### Research Topics Investigated

1. **Go BIP39/BIP32 Libraries** — Evaluated tyler-smith/go-bip39 and go-bip32 for mnemonic and HD key derivation
2. **BSV SDK Integration** — Assessed bitcoin-sv/go-sdk for transaction building vs manual implementation
3. **Age Encryption Patterns** — Researched identity-based vs password-based encryption for wallet files
4. **Ethereum go-ethereum Usage** — Evaluated lightweight alternatives, settled on go-ethereum for mainnet compatibility
5. **CLI Framework Selection** — Confirmed Cobra as industry standard for Go CLIs
6. **Levenshtein Distance Libraries** — Evaluated options for mnemonic typo correction
7. **Rate Limiting Patterns** — Researched token bucket and sliding window approaches
8. **Secure Memory Handling** — Investigated golang.org/x/sys/unix for mlock support

---

## Phase 1: Design Output

### Data Model

See [data-model.md](./data-model.md) for complete entity definitions.

Key entities:
- **Wallet**: HD wallet container with encrypted seed
- **Address**: Chain-specific derived address
- **UTXO**: Unspent output for BSV
- **Config**: Application settings
- **Backup**: Portable encrypted backup

### Contracts

See [contracts/](./contracts/) for Go interface definitions:
- `chain.Chain` — Unified chain interface (Balance, Send, EstimateFee)
- `wallet.Storage` — Wallet persistence interface
- `output.Formatter` — Output formatting interface

### Quickstart

See [quickstart.md](./quickstart.md) for developer onboarding guide.

---

## Post-Design Constitution Re-Check

*Re-evaluated after Phase 1 design completion. Constitution v1.1.0.*

| Principle | Status | Design Evidence |
|-----------|--------|-----------------|
| I. Security First | ✅ PASS | `crypto/secure.go` implements mlock + zeroing, `crypto/age.go` handles encryption, `wallet/storage.go` enforces 0600 permissions, `crypto/entropy.go` wraps crypto/rand |
| II. Full Sovereignty | ✅ PASS | All storage local to `~/.sigil/`, no external service dependencies for core ops, `backup/backup.go` creates portable files |
| III. Power User UX | ✅ PASS | `cli/` package implements noun-verb pattern, `output/format.go` handles JSON/text, verbose flag flows through config |
| IV. Transparency | ✅ PASS | `chain/*/tx.go` exposes transaction details before signing, `output/error.go` includes suggestions |
| V. Fork Awareness | ✅ PASS | `wallet/derivation.go` implements chain-specific BIP44 paths including BTC for future compatibility, `chain/chain.go` defines multi-chain interface. Fork scanner SHOULD per v1.1.0. |
| VI. CLI Design Standards | ✅ PASS | All commands in `cli/` follow noun-verb, `pkg/errors/errors.go` defines exit codes, `output/format.go` handles auto-detection |
| VII. Testing Discipline | ✅ PASS | `testdata/` for fixtures, fuzz test targets identified in research, coverage requirements documented |

**Post-design Gate**: PASSED - Design artifacts fully align with constitution. Ready for task generation.

---

---

## Testing Discipline

### TDD Approach
Test tasks (T###t) are interleaved with implementation tasks (T###). Each test task comes immediately BEFORE its corresponding implementation task. This ensures:
- Tests define expected behavior before code is written
- Implementation is guided by test requirements
- No untested code enters the codebase

### Test Vector Sources
All crypto operations must use official test vectors:
- **BIP39**: [Trezor BIP39 test vectors](https://github.com/trezor/python-mnemonic/blob/master/vectors.json)
- **BIP32**: [BIP32 test vectors](https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki#test-vectors)
- **BIP44**: Derived from BIP32 with chain-specific coin types (ETH: 60, BSV: 236)
- **EIP-55**: [EIP-55 checksum test vectors](https://eips.ethereum.org/EIPS/eip-55)

### Coverage Requirements
| Scope | Minimum Coverage |
|-------|-----------------|
| Overall project | 80% |
| `internal/crypto/*` | 100% |
| `internal/wallet/mnemonic.go` | 100% |
| `internal/wallet/derivation.go` | 100% |

### Fuzz Testing
Fuzz tests are required for input parsing:
- Mnemonic word validation and normalization
- Address format validation (ETH, BSV)
- Transaction input parsing (amounts, addresses)
- WIF/hex private key parsing

---

## Next Steps

1. Run `/speckit.tasks` to generate implementation tasks
2. Review and prioritize tasks based on dependencies
3. Begin implementation following task order
