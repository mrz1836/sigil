# Feature Specification: Sigil MVP - Multi-Chain Wallet CLI

**Feature Branch**: `001-sigil-mvp`
**Created**: 2026-01-31
**Status**: Draft
**Input**: User description: "Sigil MVP - Multi-chain wallet CLI with core infrastructure, key management, ETH/USDC support, and BSV basics per PRD Phase 1"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Secure a New Wallet (Priority: P1)

A power user wants to create a new HD wallet with a BIP39 mnemonic phrase so they can securely manage keys across multiple blockchains from a single seed.

**Why this priority**: Without wallet creation, no other functionality is usable. This is the foundational capability that everything else builds upon.

**Independent Test**: Can be fully tested by running `sigil wallet create main --words 24` and verifying a mnemonic is generated, addresses are derived for all enabled chains, and the wallet file is encrypted at rest.

**Acceptance Scenarios**:

1. **Given** no wallet named "main" exists, **When** user runs `sigil wallet create main`, **Then** a 12-word mnemonic is generated and displayed, addresses for ETH and BSV are shown, and an encrypted wallet file is saved to `~/.sigil/wallets/main.wallet`.
2. **Given** no wallet named "vault" exists, **When** user runs `sigil wallet create vault --words 24 --passphrase`, **Then** user is prompted for a BIP39 passphrase, a 24-word mnemonic is generated, and the wallet is created with passphrase-derived keys.
3. **Given** a wallet named "main" already exists, **When** user runs `sigil wallet create main`, **Then** an error is displayed indicating the wallet already exists.

---

### User Story 2 - Restore Wallet from Mnemonic (Priority: P1)

A user who has an existing mnemonic phrase (from a previous backup or another wallet) wants to restore their wallet in Sigil so they can access their funds across all supported chains.

**Why this priority**: Equal to wallet creation - users with existing funds need to import their keys before they can use Sigil for anything meaningful.

**Independent Test**: Can be fully tested by running `sigil wallet restore backup --input "abandon abandon ... about"` and verifying the derived addresses match expected values.

**Acceptance Scenarios**:

1. **Given** a valid 12-word mnemonic, **When** user runs `sigil wallet restore mykeys --input "word1 word2 ... word12"`, **Then** the wallet is created with the correct derived addresses for all chains displayed for verification.
2. **Given** a mnemonic with a typo, **When** user enters "abandn ability able ..." (invalid word), **Then** an error is shown with a suggestion: "Did you mean 'abandon' instead of 'abandn'?"
3. **Given** no input provided, **When** user runs `sigil wallet restore mykeys`, **Then** interactive guided mode prompts user step-by-step for their backup format and seed phrase with secure hidden input.
4. **Given** wallet restoration completes, **When** derived addresses are displayed, **Then** user must confirm "Do these match your expected addresses? [y/N]" before the wallet is saved.

---

### User Story 3 - Check Balances Across Chains (Priority: P2)

A user wants to check their current balances for ETH, USDC, and BSV in a single command so they can see their total holdings without switching between tools.

**Why this priority**: After creating/restoring a wallet, users need to see what funds they have before transacting. This enables the core use case of monitoring holdings.

**Independent Test**: Can be fully tested by running `sigil balance show --wallet main` against live APIs and verifying balances are displayed in a formatted table.

**Acceptance Scenarios**:

1. **Given** a wallet "main" exists with ETH, USDC, and BSV addresses, **When** user runs `sigil balance show --wallet main`, **Then** a formatted table shows balances for each chain with address and balance columns.
2. **Given** user wants machine-readable output, **When** user runs `sigil balance show --wallet main -o json`, **Then** JSON output includes wallet name, array of balances with chain, address, balance, symbol, and decimals.
3. **Given** user wants only ETH balance, **When** user runs `sigil balance show --wallet main --chain eth`, **Then** only ETH and USDC balances are displayed (tokens on ETH chain).
4. **Given** network is unavailable, **When** user checks balance, **Then** a clear error indicates which chain API failed with a suggestion to check network connectivity.

---

### User Story 4 - Send USDC Payment (Priority: P2)

A user receives BSVA invoice payments in USDC and wants to send USDC to another address for payments or transfers.

**Why this priority**: This is the primary use case mentioned in the PRD - paying BSVA invoices. It's the main transactional workflow for the target user.

**Independent Test**: Can be fully tested by sending a small USDC amount to a test address on mainnet or testnet and verifying the transaction broadcasts successfully.

**Acceptance Scenarios**:

1. **Given** wallet "main" has 100 USDC and sufficient ETH for gas, **When** user runs `sigil tx send --wallet main --to 0x123... --amount 50 --chain eth --token USDC`, **Then** user is prompted for wallet password, transaction is built, signed, and broadcast, and transaction hash is displayed.
2. **Given** wallet has insufficient USDC, **When** user attempts to send more than balance, **Then** error displays "Insufficient funds: Required 150 USDC, Available 100 USDC" with exit code 5.
3. **Given** wallet has insufficient ETH for gas, **When** user attempts USDC transfer, **Then** error indicates insufficient ETH for gas fees with the required and available amounts.
4. **Given** successful transaction, **When** output format is JSON, **Then** response includes transaction hash, from/to addresses, amount, token, gas used, gas price, and pending status.

---

### User Story 5 - Send ETH Transaction (Priority: P2)

A user needs to send ETH to another address, either for payments or to top up gas for USDC transactions.

**Why this priority**: Required for gas management to enable USDC transactions. Secondary to USDC but essential for the workflow.

**Independent Test**: Can be fully tested by sending ETH to a test address and verifying the transaction.

**Acceptance Scenarios**:

1. **Given** wallet "main" has 1.5 ETH, **When** user runs `sigil tx send --wallet main --to 0xabc... --amount 0.1 --chain eth`, **Then** transaction is signed and broadcast with hash displayed.
2. **Given** wallet has insufficient ETH, **When** user attempts to send more than balance, **Then** error displays "Insufficient funds" with required vs available amounts.
3. **Given** invalid ETH address provided, **When** user attempts transaction, **Then** error indicates "Invalid address format for ETH chain".

---

### User Story 6 - Send BSV Transaction (Priority: P2)

A user wants to send BSV to another address for basic BSV operations.

**Why this priority**: BSV is a core supported chain and basic send functionality is required for MVP completeness.

**Independent Test**: Can be fully tested by sending BSV to a test address and verifying the transaction broadcasts.

**Acceptance Scenarios**:

1. **Given** wallet "main" has BSV balance with available UTXOs, **When** user runs `sigil tx send --wallet main --to 1abc... --amount 0.5 --chain bsv`, **Then** transaction is built from UTXOs, signed, and broadcast.
2. **Given** wallet has insufficient BSV, **When** user attempts to send more than balance, **Then** appropriate insufficient funds error is displayed.
3. **Given** user wants to see UTXOs, **When** user runs `sigil utxo list --wallet main --chain bsv`, **Then** list of unspent transaction outputs is displayed with txid, vout, amount, and confirmations.

---

### User Story 7 - List and Show Wallets (Priority: P3)

A user with multiple wallets wants to list all wallets and view details of a specific wallet.

**Why this priority**: Supporting feature for wallet management. Less critical than core create/restore/transact flows.

**Independent Test**: Can be fully tested by creating multiple wallets and running list/show commands.

**Acceptance Scenarios**:

1. **Given** wallets "main" and "backup" exist, **When** user runs `sigil wallet list`, **Then** a table shows wallet names, creation dates, enabled chains, and primary addresses.
2. **Given** wallet "main" exists, **When** user runs `sigil wallet show main`, **Then** detailed wallet info is displayed including all derived addresses for each chain.
3. **Given** JSON output requested, **When** user runs `sigil wallet list -o json`, **Then** JSON array of wallet objects with name, created_at, chains, and addresses is returned.

---

### User Story 8 - Import Key from WIF or Hex (Priority: P3)

A user has a private key in WIF or hexadecimal format and wants to import it into Sigil.

**Why this priority**: Alternative import path for users with raw keys rather than mnemonics. Less common but important for flexibility.

**Independent Test**: Can be fully tested by importing a known WIF key and verifying the derived address matches.

**Acceptance Scenarios**:

1. **Given** a valid WIF private key, **When** user runs `sigil wallet import legacy --wif`, **Then** user is prompted for the WIF key, key is validated and imported, and the wallet is created.
2. **Given** a 64-character hex private key, **When** user runs `sigil wallet import hexwallet --input <64-char-hex>`, **Then** format is auto-detected as hex, single key is imported (note: `import` for single keys, `restore` for full HD wallet from mnemonic).
3. **Given** invalid WIF format, **When** user attempts import, **Then** error "Invalid WIF format" is displayed.

---

### User Story 9 - Configure Application Settings (Priority: P3)

A user wants to configure Sigil settings like RPC endpoints, fee providers, and default options.

**Why this priority**: Required for customization but most users will start with defaults. Not blocking for basic usage.

**Independent Test**: Can be fully tested by running config commands and verifying settings persist.

**Acceptance Scenarios**:

1. **Given** fresh Sigil installation, **When** user runs `sigil config init`, **Then** default config file is created at `~/.sigil/config.yaml`.
2. **Given** config exists, **When** user runs `sigil config show`, **Then** current configuration is displayed in readable format.
3. **Given** user wants custom ETH RPC, **When** user runs `sigil config set networks.eth.rpc "https://..."`, **Then** the setting is persisted to config file.
4. **Given** user queries a setting, **When** user runs `sigil config get networks.eth.rpc`, **Then** the current value is displayed.

---

### User Story 10 - Create and Verify Backup (Priority: P3)

A user wants to create an encrypted backup of their wallet that they can store securely and later verify.

**Why this priority**: Important for security best practices but users can also manually backup mnemonic. Basic backup included in MVP per PRD.

**Independent Test**: Can be fully tested by creating a backup, verifying it, and restoring from it.

**Acceptance Scenarios**:

1. **Given** wallet "main" exists, **When** user runs `sigil backup create --wallet main`, **Then** encrypted `.sigil` backup file is created in `~/.sigil/backups/` with timestamp in filename.
2. **Given** backup file exists, **When** user runs `sigil backup verify --input main-2026-01-31.sigil`, **Then** backup integrity is verified via checksum and confirmation is displayed.
3. **Given** backup file exists, **When** user runs `sigil backup restore --input backup.sigil`, **Then** wallet is restored from the backup file.

---

### Edge Cases

- What happens when user enters mnemonic with extra whitespace? System should normalize input (trim and collapse spaces).
- What happens when RPC endpoint is unreachable? System should show clear network error with retry suggestion.
- What happens when wallet file is corrupted? System should detect corruption via checksum and display DECRYPTION_FAILED error.
- What happens when user enters wrong password? System should display "Decryption failed - wrong password" with exit code 3.
- What happens when transaction fee exceeds safety limit? System should warn user and require confirmation.
- What happens when output is piped to another command? System should auto-detect non-TTY and output JSON format.
- What happens when config file is malformed? System should display CONFIG_INVALID error with specific parsing issue.
- What happens when BSV UTXO is too small (dust)? System should warn about dust outputs and suggest consolidation.

## Requirements *(mandatory)*

### Functional Requirements

**Core Infrastructure**:
- **FR-001**: System MUST use `~/.sigil/` as the default home directory for all data storage (configurable via `--home` flag or `SIGIL_HOME` environment variable).
- **FR-002**: System MUST load configuration from YAML file at `~/.sigil/config.yaml` with support for environment variable overrides.
- **FR-003**: System MUST implement noun-verb command pattern (`sigil <noun> <verb> [args] [flags]`) for all CLI commands.
- **FR-004**: System MUST support `--output` flag with values `text` (default) and `json` for all commands that produce output.
- **FR-005**: System MUST auto-detect output format based on TTY - use `text` for interactive terminals, `json` when piped or redirected.
- **FR-006**: System MUST return consistent exit codes: 0 (success), 1 (general error), 2 (input error), 3 (auth error), 4 (not found), 5 (permission/funds error).
- **FR-040**: System MUST cache balance query results locally with timestamps for each chain/address.
- **FR-041**: System MUST return cached balance data with staleness warning (including cache age) when a chain API fails after retry exhaustion, allowing other chains to display fresh data.
- **FR-042**: System MUST write debug logs to `~/.sigil/sigil.log` with configurable verbosity levels: `off` (no logging), `error` (errors only), `debug` (verbose including API requests/responses).

**Key Management**:
- **FR-007**: System MUST generate BIP39 mnemonic phrases with 12 or 24 words using cryptographically secure random entropy.
- **FR-008**: System MUST support BIP39 optional passphrase for additional seed protection.
- **FR-009**: System MUST derive keys using BIP44 paths: `m/44'/236'/0'` (BSV), `m/44'/60'/0'` (ETH). System SHOULD store derived keys for `m/44'/0'/0'` (BTC) for future fork scanner compatibility, but BTC chain operations are out of scope for MVP.
- **FR-010**: System MUST encrypt all wallet files at rest using age encryption.
- **FR-011**: System MUST support importing keys from: BIP39 mnemonic (12/24 words), WIF format, and 64-character hex format.
- **FR-012**: System MUST auto-detect import format based on input pattern (word count, character length, prefix).
- **FR-013**: System MUST validate mnemonic words against BIP39 wordlist and suggest corrections for typos using Levenshtein distance.
- **FR-014**: System MUST display derived addresses for user verification before saving restored wallets.

**ETH/USDC Support**:
- **FR-015**: System MUST derive ETH addresses from HD wallet seed using BIP44 ETH derivation path.
- **FR-016**: System MUST query ETH balance via configured RPC endpoint.
- **FR-017**: System MUST query USDC balance via ERC-20 (USDC) `balanceOf` contract call.
- **FR-018**: System MUST build and sign ETH transfer transactions with proper gas estimation.
- **FR-019**: System MUST build and sign ERC-20 (USDC) transfer transactions.
- **FR-020**: System MUST fetch current gas prices for transaction fee estimation.
- **FR-021**: System MUST validate ETH addresses including EIP-55 checksum verification.

**BSV Support**:
- **FR-022**: System MUST derive BSV addresses from HD wallet seed using BIP44 BSV derivation path.
- **FR-023**: System MUST query BSV balance via WhatsOnChain API.
- **FR-024**: System MUST list UTXOs for BSV addresses.
- **FR-025**: System MUST build P2PKH transactions for BSV sends.
- **FR-026**: System MUST sign and broadcast BSV transactions.
- **FR-027**: System MUST estimate BSV transaction fees via TAAL or GorillaPool APIs.
- **FR-028**: System MUST validate BSV addresses using Base58Check encoding.

**Wallet Management**:
- **FR-029**: System MUST create new named wallets with unique alphanumeric names.
- **FR-030**: System MUST list all wallets with name, creation date, chains, and primary addresses.
- **FR-031**: System MUST show detailed wallet information including all derived addresses.
- **FR-032**: System MUST prevent creating wallets with duplicate names.

**Backup**:
- **FR-033**: System MUST create encrypted `.sigil` backup files with manifest and checksum.
- **FR-034**: System MUST verify backup file integrity via SHA256 checksum.
- **FR-035**: System MUST restore wallets from `.sigil` backup files.

**Error Handling**:
- **FR-036**: System MUST display errors with consistent structure: error code, message, details, and actionable suggestion.
- **FR-037**: System MUST use sentinel error codes (WALLET_NOT_FOUND, INSUFFICIENT_FUNDS, etc.) for programmatic error handling.
- **FR-038**: System MUST provide JSON-formatted errors when `--output json` is specified.
- **FR-039**: System MUST retry failed network requests (API calls, RPC queries, transaction broadcasts) up to 3 times with exponential backoff (1s, 2s, 4s delays) before returning a network error.
- **FR-043**: System MUST respect rate limit headers (e.g., `Retry-After`) from external APIs by pausing before retry attempts.
- **FR-044**: System MUST implement client-side request throttling to limit API calls to a configurable maximum rate per service (default: 5 requests/second per endpoint).

### Key Entities

- **Wallet**: Named container for cryptographic keys derived from a single HD seed. Contains mnemonic (encrypted), derived addresses per chain, creation timestamp, and enabled chains.
- **Address**: A chain-specific public address derived from the wallet's HD seed. Associated with a derivation path and chain identifier.
- **Transaction**: A signed blockchain operation. Contains chain, from address, to address, amount, token (if applicable), gas/fee information, and hash. Status is limited to broadcast success/failure; confirmation tracking is out of scope (users check block explorers for confirmation).
- **UTXO**: Unspent Transaction Output for UTXO-based chains (BSV/BTC). Contains transaction ID, output index, amount, and script.
- **Backup**: Encrypted portable wallet file. Contains manifest (unencrypted metadata), encrypted wallet data, and integrity checksum.
- **Config**: Application settings including network endpoints (RPC URLs per chain), fee preferences (gas price strategy, fee rate), log verbosity level, and output format preferences. Advanced security options (auto-lock, memory protection) are deferred to post-MVP.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `sigil wallet create <name>` generates a valid BIP39 mnemonic, derives addresses for all enabled chains, displays the mnemonic once, and writes an encrypted wallet file to `~/.sigil/wallets/<name>.wallet`.
- **SC-002**: `sigil wallet restore <name>` accepts a 12 or 24-word mnemonic, derives identical addresses to the original wallet, displays addresses for verification, requires user confirmation, and writes an encrypted wallet file.
- **SC-003**: `sigil balance show --wallet <name>` queries all enabled chains in a single invocation and displays a formatted table with chain, address, balance, and symbol columns.
- **SC-004**: `sigil tx send --chain eth --token USDC` prompts for wallet password, builds and signs an ERC-20 transfer, broadcasts to the network, and returns a transaction hash.
- **SC-005**: `sigil tx send --chain bsv` selects UTXOs, builds a P2PKH transaction, signs, broadcasts, and returns a transaction hash.
- **SC-006**: All commands provide meaningful help text via `--help` flag with usage examples.
- **SC-007**: JSON output is valid and parseable for all commands when `-o json` flag is used (validated by `jq` without errors).
- **SC-008**: Error messages include: error code, human-readable message, and actionable suggestion for resolution.
- **SC-009**: 100% of wallet files are encrypted at rest using age encryption - no plaintext keys stored on disk.
- **SC-010**: Typo detection correctly suggests the intended BIP39 word for all 2048 words when a single character is substituted, deleted, inserted, or transposed (tested against BIP39 English wordlist).
- **SC-011**: All CLI commands follow noun-verb pattern: `sigil <noun> <verb> [args] [flags]`.
- **SC-012**: Exit codes match FR-006: 0 (success), 1 (general error), 2 (input error), 3 (auth error), 4 (not found), 5 (permission/funds error).

## Assumptions

- Users have internet connectivity for balance checks and transaction broadcasting.
- Users are comfortable with command-line interfaces and terminal operations.
- ETH mainnet (chain ID 1) is the default Ethereum network; users will configure RPC endpoints.
- Default USDC contract address is the mainnet USDC contract (0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48).
- WhatsOnChain provides reliable BSV balance and UTXO queries without rate limiting for normal usage.
- Age encryption with Argon2id key derivation provides sufficient security for encrypted wallet storage.
- Standard 20-address gap limit is sufficient for most users' address discovery needs.
- Users will backup their mnemonic phrase separately; the application displays it once on creation.

## Scope Boundaries

**In Scope (MVP)**:
- Wallet creation with 12/24 word mnemonics
- Wallet restoration from mnemonic, WIF, hex
- Balance checking for ETH, USDC, BSV
- Sending ETH, USDC, BSV transactions
- UTXO listing for BSV
- Basic config management
- Encrypted wallet storage
- Basic backup/restore
- Text and JSON output formats
- CLI help system

**Out of Scope (Future Phases)**:
- TUI dashboard interface
- Fork scanner for BTC/BCH
- Advanced UTXO selection strategies
- 1Sat Ordinals / BSV20 tokens
- Hardware wallet integration
- Watch-only wallets (xpub import)
- Paper backup generation
- Air-gapped signing workflow
- Auto-lock functionality
- Memory protection (mlock)
- Additional ERC-20 tokens beyond USDC

## Clarifications

### Session 2026-01-31

- Q: When an API call fails due to network timeout or transient error, what retry behavior should Sigil use? → A: Retry 3 times with exponential backoff (1s, 2s, 4s) before failing
- Q: When checking balances across multiple chains, if one chain's API fails after retries but others succeed, how should the command behave? → A: Return cached/stale data for failed chains with staleness warning
- Q: What logging/observability approach should Sigil use for debugging failed transactions or API issues? → A: File-based debug log at `~/.sigil/sigil.log` with configurable verbosity (off/error/debug)
- Q: After broadcasting a transaction, should Sigil track confirmation status or just return the transaction hash? → A: Display tx hash only; user checks explorer for confirmation status
- Q: How should Sigil handle API rate limiting from external services? → A: Both respect rate limit headers (Retry-After) and implement client-side request throttling
