<!--
SYNC IMPACT REPORT
==================
Version Change: 1.0.0 → 1.1.0 (minor: clarified fork scanner scope)
Modified Principles:
  - V. Fork Awareness: Changed fork scanner from MUST to SHOULD for MVP compatibility.
    Added clarification that BTC path derivation is required even without BTC operations.
Added Sections: None
Removed Sections: None
Templates Requiring Updates:
  - ✅ .specify/templates/plan-template.md (no updates needed - Constitution Check is generic)
  - ✅ .specify/templates/spec-template.md (no updates needed - compatible structure)
  - ✅ .specify/templates/tasks-template.md (no updates needed - phase structure aligns)
Follow-up TODOs: None
-->

# Sigil Constitution

## Core Principles

### I. Security First

Keys MUST never touch disk unencrypted. All wallet files MUST use Age encryption at rest.
Memory containing sensitive data (private keys, mnemonics) MUST be locked with mlock to prevent
swap and MUST be explicitly zeroed before garbage collection.

**Non-negotiables:**
- No network calls without explicit user consent
- Offline operation MUST be supported for all key operations (generation, signing)
- Cryptographic operations MUST use crypto/rand for entropy, never math/rand
- Input validation MUST occur at system boundaries (addresses, amounts, mnemonics)

**Rationale:** This is a wallet managing real funds. Security failures are catastrophic and
irreversible. Defense in depth is mandatory.

### II. Full Sovereignty

Users own their keys. No third-party custody, no phone-home telemetry, no cloud sync.
The wallet operates entirely locally with user-controlled network access.

**Non-negotiables:**
- No automatic update checks or analytics
- No external services required for core wallet operations
- Users MUST be able to export keys in standard formats (WIF, hex, mnemonic)
- Backup files MUST be self-contained and portable

**Rationale:** Sovereignty means the user has complete control. Any dependency on external
services for core operations violates this principle.

### III. Power User UX

Sigil targets users who understand cryptographic keys and blockchain fundamentals.
The interface prioritizes control and transparency over hand-holding.

**Non-negotiables:**
- CLI for scripting and automation with consistent noun-verb command pattern
- JSON output MUST be available for all commands (machine-readable)
- TUI for visual management (Phase 3+)
- No confirmation dialogs for read operations
- Verbose/debug modes MUST expose full transaction details

**Rationale:** Power users need scriptable, automatable tools. Dumbed-down interfaces
obstruct rather than help.

### IV. Transparency

Every transaction MUST be inspectable before signing. Users MUST see inputs, outputs,
and fees with no hidden operations.

**Non-negotiables:**
- Transaction building MUST expose all components before signature
- Fee estimation MUST show calculation method and allow manual override
- No "magic" operations that modify transactions without display
- Error messages MUST include actionable suggestions and context

**Rationale:** When real money is at stake, surprises are unacceptable. Users must
understand exactly what they're signing.

### V. Fork Awareness

One seed can have value across multiple chains (BSV, BTC, BCH, ETH). Sigil MUST manage
keys across all supported chains from a single wallet.

**Non-negotiables:**
- BIP44 derivation paths MUST be chain-specific (BSV: m/44'/236'/0', BTC: m/44'/0'/0', ETH: m/44'/60'/0')
- Fork scanner SHOULD check for value across all enabled chains (MVP: manual balance check per chain; automated fork scanning in Phase 2+)
- Chain-specific address formats MUST be validated and displayed correctly
- Same private key MUST work across compatible chains without re-import
- Key derivation MUST include BTC path even when BTC chain operations are not yet implemented (enables seamless Phase 2 upgrade)

**Rationale:** Users have funds scattered across forks. A multi-chain wallet that ignores
this reality fails its core purpose. MVP focuses on key management foundation; automated
fork scanning is additive and can follow in Phase 2.

### VI. CLI Design Standards

Commands follow the noun-verb pattern: `sigil <noun> <verb> [args] [flags]`.
Output format, flag conventions, and error codes are standardized.

**Non-negotiables:**
- Nouns: wallet, key, balance, tx, scan, backup, config, utxo
- Verbs: create, import, export, list, show, get, send, sign, broadcast, run, verify
- Global flags: --output/-o, --verbose/-v, --quiet/-q, --config/-c, --home, --no-color
- Exit codes: 0 (success), 1 (error), 2 (input error), 3 (auth error), 4 (not found), 5 (permission)
- Auto-detect output format: text for TTY, JSON for pipes/redirects

**Rationale:** Predictable CLI design enables scripting, reduces learning curve, and
ensures reliable automation.

### VII. Testing Discipline

Security-critical code (cryptography, key derivation, signing) requires 100% test coverage.
Fuzz testing is mandatory for all user-input parsing.

**Non-negotiables:**
- Table-driven unit tests for all core functions
- Fuzz tests for: mnemonic validation, amount parsing, address validation
- Integration tests (build-tagged) for RPC/API interactions
- Pre-commit: golangci-lint + go test -race

**Test Priority:**
| Priority | Scope | Coverage |
|----------|-------|----------|
| Critical | Crypto, key derivation, signing | 100% |
| High | Address validation, amounts, tx building | 95% |
| Medium | Config parsing, CLI flags, output formatting | 85% |
| Low | Help text, cosmetic formatting | 70% |

**Rationale:** Bugs in wallet software lose money. Testing is not optional.

## Security Requirements

These constraints apply to ALL code in the repository:

| Requirement | Implementation |
|-------------|----------------|
| Encrypted storage | Age encryption for all wallet files |
| Memory protection | mlock for sensitive data, explicit zeroing |
| Input validation | All external input validated before use |
| No hardcoded secrets | API keys from config/env only |
| Secure defaults | Fail closed, require explicit enable for network |
| Dependency review | No new deps without security audit consideration |

**File permissions:**
- Wallet files: 0600 (owner read/write only)
- Config files: 0640 (owner read/write, group read)
- Directories: 0750 (owner full, group read/execute)

## Development Workflow

### Code Quality Gates

All code MUST pass before merge:

1. **Linting**: `golangci-lint run` with project configuration
2. **Tests**: `go test -race ./...` with no failures
3. **Build**: Clean build with no warnings treated as errors
4. **Security**: gosec findings addressed or explicitly justified

### Commit Standards

- Commits MUST be atomic (one logical change per commit)
- Commit messages follow conventional commits: `type(scope): description`
- Types: feat, fix, docs, test, refactor, chore, security
- Security-sensitive changes MUST be tagged with `security` type

### Review Requirements

- All cryptographic code requires explicit review focus
- Changes to key derivation, signing, or encryption require security-aware review
- New dependencies require justification and basic audit

## Governance

This constitution supersedes all other development practices for the Sigil project.
Amendments require:

1. Written proposal with rationale
2. Impact analysis on existing code
3. Version increment per semantic versioning:
   - MAJOR: Principle removal or incompatible redefinition
   - MINOR: New principle or materially expanded guidance
   - PATCH: Clarifications, wording, non-semantic refinements
4. Update to all dependent documentation

**Compliance:** All PRs and reviews MUST verify adherence to these principles.
Deviations MUST be explicitly justified in the Complexity Tracking section of
implementation plans.

**Version**: 1.1.0 | **Ratified**: 2026-01-31 | **Last Amended**: 2026-01-31
