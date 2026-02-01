# Tasks: Sigil MVP - Multi-Chain Wallet CLI

**Input**: Design documents from `/specs/001-sigil-mvp/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: TDD-style test tasks are interleaved with implementation tasks. Each test task (T###t) comes immediately BEFORE its corresponding implementation task (T###).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Based on plan.md structure:
- Entry point: `cmd/sigil/`
- Core logic: `internal/` (cli, wallet, chain, crypto, config, output, cache, backup)
- Shared errors: `pkg/errors/`
- Test fixtures: `testdata/`

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Initialize Go project with dependencies and directory structure

- [X] T001 Initialize Go module with `go mod init sigil` in repository root
- [X] T002 Create directory structure per plan.md: cmd/sigil/, internal/{cli,wallet,chain/eth,chain/bsv,crypto,config,output,cache,backup}/, pkg/errors/, testdata/{mnemonics,wallets,config}/
- [X] T003 [P] Add primary dependencies to go.mod (cobra, viper, go-bip39, go-bip32, go-sdk, go-whatsonchain, go-ethereum, age, yaml.v3, levenshtein, x/time, x/sys, x/term)
- [X] T004 [P] Create main.go entry point in cmd/sigil/main.go with minimal cobra root command setup
- [X] T005 [P] Configure golangci-lint with .golangci.yml for project linting rules
- [X] T005b [P] Add testify and test dependencies to go.mod for assertions
- [X] T005c [P] Create test directory structure: testdata/{mnemonics,wallets,config}/ with .gitkeep files

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

### Error Handling Infrastructure

- [X] T006t Write tests for sentinel errors and exit codes in pkg/errors/errors_test.go
- [X] T006 Define sentinel errors and exit codes in pkg/errors/errors.go per FR-006 (0=success, 1=general, 2=input, 3=auth, 4=not found, 5=permission/funds)
- [X] T007 [P] Implement error details and suggestion helpers in pkg/errors/errors.go (WithDetails, WithSuggestion per FR-036/FR-037)

### Crypto Infrastructure

- [X] T008t Write tests for SecureBytes mlock/zeroing behavior in internal/crypto/secure_test.go
- [X] T008 Implement SecureBytes with mlock/munlock for secure memory handling in internal/crypto/secure.go per research.md section 8
- [X] T009 [P] Create crypto/rand wrapper for entropy generation in internal/crypto/entropy.go (FR-007)
- [X] T010t Write tests for age encryption round-trip in internal/crypto/age_test.go
- [X] T010 [P] Implement age encryption/decryption with password-based Scrypt in internal/crypto/age.go (FR-010)

### Configuration Infrastructure

- [X] T011 Define Config struct and all nested config types in internal/config/config.go per data-model.md
- [X] T012 Implement ConfigDefaults() with default values in internal/config/defaults.go per contracts/config.go
- [X] T013t Write tests for config Load/Save round-trip in internal/config/config_test.go
- [X] T013 Implement config Load/Save with YAML file I/O in internal/config/config.go (FR-002)
- [X] T014 [P] Implement environment variable overrides in internal/config/env.go (SIGIL_HOME, SIGIL_ETH_RPC, etc.)

### Output Infrastructure

- [X] T015 Define OutputFormat enum and Formatter interface in internal/output/format.go
- [X] T016t Write tests for output formatters (Text/JSON) in internal/output/format_test.go
- [X] T016 Implement TextFormatter with table rendering in internal/output/format.go (FR-004)
- [X] T017 [P] Implement JSONFormatter for machine-readable output in internal/output/format.go (FR-004)
- [X] T018 [P] Implement TTY auto-detection for format selection in internal/output/format.go (FR-005)
- [X] T019 [P] Implement error formatting with suggestions in internal/output/error.go (FR-036/FR-038)
- [X] T020 [P] Implement table renderer for text output in internal/output/table.go

### Logging Infrastructure

- [X] T020a Define log levels (off, error, debug) and Logger interface in internal/config/logging.go (FR-042)
- [X] T020b Implement file-based logger writing to ~/.sigil/sigil.log in internal/config/logging.go (FR-042)

### CLI Root Infrastructure

- [X] T021 Implement root cobra command with global flags (--home, --output, --verbose) in internal/cli/root.go (FR-001/FR-003/FR-004)
- [X] T022 Wire config loading, logger initialization, and output formatter creation in internal/cli/root.go

### Chain Infrastructure

- [X] T023 Define Chain and UTXOChain interfaces in internal/chain/chain.go per contracts/chain.go
- [X] T024 [P] Define chain constants (ChainETH, ChainBSV) and derivation paths in internal/chain/chain.go per data-model.md

### Rate Limiting Infrastructure

**Scope**: Rate limiting applies to all external chain API clients (ETH RPC, WhatsOnChain, TAAL). Backup operations are local file I/O and do not require rate limiting.

- [X] T025t Write tests for rate limiter token bucket behavior in internal/chain/ratelimit_test.go
- [X] T025 Implement RateLimitedClient with token bucket in internal/chain/ratelimit.go per research.md section 7 (FR-044)
- [X] T026t Write tests for retry with exponential backoff in internal/chain/retry_test.go
- [X] T026 [P] Implement retry with exponential backoff helper in internal/chain/retry.go per research.md section 9 (FR-039)
- [X] T027 [P] Implement Retry-After header handling in internal/chain/retry.go (FR-043)

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Create and Secure a New Wallet (Priority: P1) MVP

**Goal**: Users can create a new HD wallet with BIP39 mnemonic and encrypted storage

**Independent Test**: Run `sigil wallet create main --words 24` and verify mnemonic is generated, addresses are derived, and wallet file is encrypted at `~/.sigil/wallets/main.wallet`

### Implementation for User Story 1

- [X] T028t [US1] Write tests for BIP39 mnemonic generation with official test vectors in internal/wallet/mnemonic_test.go
- [X] T028 [P] [US1] Implement BIP39 mnemonic generation (12/24 words) in internal/wallet/mnemonic.go (FR-007)
- [X] T029t [US1] Write tests for mnemonic validation with valid/invalid inputs in internal/wallet/mnemonic_test.go
- [X] T029 [P] [US1] Implement BIP39 mnemonic validation in internal/wallet/mnemonic.go
- [X] T030t [US1] Write tests for whitespace normalization edge cases in internal/wallet/mnemonic_test.go
- [X] T030 [P] [US1] Implement mnemonic input normalization (trim, collapse spaces) in internal/wallet/mnemonic.go
- [X] T031t [US1] Write tests for BIP39 seed derivation with official test vectors in internal/wallet/mnemonic_test.go
- [X] T031 [US1] Implement BIP39 seed derivation with optional passphrase in internal/wallet/mnemonic.go (FR-008)
- [X] T032t [US1] Write tests for ETH address derivation with test vectors in internal/wallet/derivation_test.go
- [X] T032 [P] [US1] Implement BIP44 key derivation for ETH path m/44'/60'/0' in internal/wallet/derivation.go (FR-009/FR-015)
- [X] T033t [US1] Write tests for BSV address derivation with test vectors in internal/wallet/derivation_test.go
- [X] T033 [P] [US1] Implement BIP44 key derivation for BSV path m/44'/236'/0' in internal/wallet/derivation.go (FR-009/FR-022)
- [X] T034 [US1] Implement address derivation from public keys (ETH: keccak256, BSV: Base58Check) in internal/wallet/derivation.go
- [X] T035 [US1] Define Wallet and Address structs in internal/wallet/wallet.go per data-model.md
- [X] T036t [US1] Write tests for wallet encryption/decryption round-trip in internal/wallet/storage_test.go
- [X] T036 [US1] Implement wallet file storage with age encryption in internal/wallet/storage.go (FR-010)
- [X] T037 [US1] Implement file permissions enforcement (0600 for wallet files) in internal/wallet/storage.go
- [X] T038 [US1] Implement wallet existence check in internal/wallet/storage.go (FR-032)
- [X] T039t [US1] Write integration test for `sigil wallet create` command in internal/cli/wallet_test.go
- [X] T039 [US1] Implement wallet create command in internal/cli/wallet.go with `sigil wallet create <name>` (FR-029)
- [X] T040 [US1] Add --words flag (12/24 default 12) to wallet create command in internal/cli/wallet.go
- [X] T041 [US1] Add --passphrase flag for BIP39 passphrase prompt to wallet create command in internal/cli/wallet.go (FR-008)
- [X] T042 [US1] Implement secure password input (hidden) for wallet encryption in internal/cli/wallet.go
- [X] T043 [US1] Display mnemonic with formatting for secure viewing in internal/cli/wallet.go
- [X] T044 [US1] Display derived addresses for ETH and BSV after wallet creation in internal/cli/wallet.go

**Checkpoint**: User Story 1 complete - wallet creation with mnemonic display and encrypted storage works independently

---

## Phase 4: User Story 2 - Restore Wallet from Mnemonic (Priority: P1)

**Goal**: Users can restore an existing wallet from a BIP39 mnemonic phrase

**Independent Test**: Run `sigil wallet restore backup --input "abandon abandon ... about"` and verify derived addresses match expected values

### Implementation for User Story 2

- [X] T045t [US2] Write tests for Levenshtein typo detection in internal/wallet/mnemonic_test.go
- [X] T045 [P] [US2] Implement Levenshtein-based typo detection for mnemonic words in internal/wallet/mnemonic.go (FR-013)
- [X] T046 [P] [US2] Implement word suggestion for typos (Did you mean X?) in internal/wallet/mnemonic.go (FR-013)
- [X] T047t [US2] Write tests for format auto-detection (mnemonic/WIF/hex) in internal/wallet/restore_test.go
- [X] T047 [US2] Implement format auto-detection for input (mnemonic, WIF, hex) in internal/wallet/restore.go (FR-012)
- [X] T048t [US2] Write tests for WIF parsing with test vectors in internal/wallet/restore_test.go
- [X] T048 [P] [US2] Implement WIF private key parsing in internal/wallet/restore.go (FR-011)
- [X] T049 [P] [US2] Implement hex private key parsing (64-char) in internal/wallet/restore.go (FR-011)
- [X] T050t [US2] Write integration test for `sigil wallet restore` command in internal/cli/wallet_test.go
- [X] T050 [US2] Implement wallet restore command in internal/cli/wallet.go with `sigil wallet restore <name>` (FR-011)
- [X] T051 [US2] Add --input flag for seed material (mnemonic/WIF/hex) to restore command in internal/cli/wallet.go
- [X] T052 [US2] Implement interactive guided mode when no --input provided in internal/cli/wallet.go (step-by-step prompts)
- [X] T053 [US2] Display derived addresses for verification before saving restored wallet in internal/cli/wallet.go (FR-014)
- [X] T054 [US2] Implement address confirmation prompt "Do these match your expected addresses? [y/N]" in internal/cli/wallet.go (FR-014)

**Checkpoint**: User Story 2 complete - wallet restoration with typo detection and address verification works independently

---

## Phase 5: User Story 3 - Check Balances Across Chains (Priority: P2)

**Goal**: Users can check ETH, USDC, and BSV balances in a single command

**Independent Test**: Run `sigil balance show --wallet main` against live APIs and verify balances are displayed in a formatted table

### Implementation for User Story 3

- [X] T055t [US3] Write tests for ETH client with mocked RPC in internal/chain/eth/client_test.go
- [X] T055 [P] [US3] Implement ETH RPC client wrapper in internal/chain/eth/client.go (FR-016)
- [X] T056t [US3] Write tests for WhatsOnChain client with mocked API in internal/chain/bsv/client_test.go
- [X] T056 [P] [US3] Implement WhatsOnChain API client wrapper in internal/chain/bsv/client.go (FR-023)
- [X] T057 [US3] Implement ETH native balance query via eth_getBalance in internal/chain/eth/balance.go (FR-016)
- [X] T058 [US3] Implement ERC-20 balanceOf query for USDC in internal/chain/eth/balance.go (FR-017)
- [X] T059 [US3] Implement BSV balance query via WhatsOnChain API in internal/chain/bsv/balance.go (FR-023)
- [X] T060 [US3] Define BalanceCache struct in internal/cache/cache.go per data-model.md
- [X] T061t [US3] Write tests for balance cache storage/retrieval in internal/cache/file_test.go
- [X] T061 [US3] Implement file-based balance cache storage in internal/cache/file.go (FR-040)
- [X] T062 [US3] Implement cache staleness detection with age tracking in internal/cache/cache.go (FR-041)
- [X] T063t [US3] Write integration test for `sigil balance show` command in internal/cli/balance_test.go
- [X] T063 [US3] Implement balance show command in internal/cli/balance.go with `sigil balance show --wallet <name>`
- [X] T064 [US3] Add --chain flag to filter by chain (eth, bsv) in internal/cli/balance.go
- [X] T065 [US3] Format balance output as table with chain/address/balance/symbol columns in internal/cli/balance.go
- [X] T066 [US3] Display staleness warning for cached data when API fails in internal/cli/balance.go (FR-041)

**Checkpoint**: User Story 3 complete - balance checking across chains works independently

---

## Phase 6: User Story 4 - Send USDC Payment (Priority: P2)

**Goal**: Users can send USDC to another address with proper gas handling

**Independent Test**: Send a small USDC amount to a test address and verify transaction broadcasts successfully

### Implementation for User Story 4

- [ ] T067t [US4] Write tests for EIP-55 checksum validation in internal/chain/eth/address_test.go
- [ ] T067 [P] [US4] Implement EIP-55 checksum address validation for ETH in internal/chain/eth/address.go (FR-021)
- [ ] T068 [P] [US4] Implement gas price fetching (slow/medium/fast) in internal/chain/eth/gas.go (FR-020)
- [ ] T069 [US4] Implement gas estimation for ETH transfers in internal/chain/eth/gas.go (FR-018)
- [ ] T070 [US4] Implement gas estimation for ERC-20 transfers in internal/chain/eth/gas.go (FR-019)
- [ ] T071t [US4] Write tests for ETH transaction building in internal/chain/eth/tx_test.go
- [ ] T071 [US4] Implement ETH native transfer transaction building in internal/chain/eth/tx.go (FR-018)
- [ ] T072 [US4] Implement ERC-20 transfer transaction building for USDC in internal/chain/eth/tx.go (FR-019)
- [ ] T073 [US4] Implement transaction signing with private key (zeroed after use) in internal/chain/eth/tx.go
- [ ] T074 [US4] Implement transaction broadcast via RPC in internal/chain/eth/tx.go
- [ ] T075 [US4] Define TransactionResult struct in internal/chain/transaction.go per data-model.md
- [ ] T076 [US4] Implement tx send command in internal/cli/tx.go with `sigil tx send --wallet <name> --to <address> --amount <value> --chain eth --token USDC`
- [ ] T077 [US4] Implement wallet unlock with password prompt in internal/cli/tx.go
- [ ] T078 [US4] Implement insufficient funds error with required/available amounts in internal/cli/tx.go (FR-037)
- [ ] T079 [US4] Display transaction hash after successful broadcast in internal/cli/tx.go

**Checkpoint**: User Story 4 complete - USDC transactions work independently

---

## Phase 7: User Story 5 - Send ETH Transaction (Priority: P2)

**Goal**: Users can send ETH for payments or gas top-up

**Independent Test**: Send ETH to a test address and verify the transaction

### Implementation for User Story 5

- [ ] T080 [US5] Extend tx send command to handle native ETH transfers (no --token flag) in internal/cli/tx.go
- [ ] T081 [US5] Validate ETH address format in tx send command in internal/cli/tx.go (FR-021)
- [ ] T082 [US5] Display gas estimate before confirming ETH transaction in internal/cli/tx.go

**Checkpoint**: User Story 5 complete - ETH transactions work independently

---

## Phase 8: User Story 6 - Send BSV Transaction (Priority: P2)

**Goal**: Users can send BSV with UTXO-based transaction building

**Independent Test**: Send BSV to a test address and verify transaction broadcasts

### Implementation for User Story 6

- [ ] T083t [US6] Write tests for Base58Check address validation in internal/chain/bsv/address_test.go
- [ ] T083 [P] [US6] Implement Base58Check address validation for BSV in internal/chain/bsv/address.go (FR-028)
- [ ] T084 [US6] Define UTXO struct in internal/chain/bsv/utxo.go per data-model.md
- [ ] T085 [US6] Implement UTXO listing via WhatsOnChain API in internal/chain/bsv/utxo.go (FR-024)
- [ ] T086 [US6] Implement UTXO selection (simple first-fit) in internal/chain/bsv/utxo.go
- [ ] T087 [US6] Implement BSV fee estimation via TAAL API in internal/chain/bsv/fee.go (FR-027)
- [ ] T088t [US6] Write tests for P2PKH transaction building using go-sdk in internal/chain/bsv/tx_test.go
- [ ] T088 [US6] Implement P2PKH transaction building using go-sdk in internal/chain/bsv/tx.go (FR-025)
- [ ] T089 [US6] Implement BSV transaction signing in internal/chain/bsv/tx.go
- [ ] T090 [US6] Implement BSV transaction broadcast via TAAL in internal/chain/bsv/tx.go (FR-026)
- [ ] T091 [US6] Extend tx send command for BSV chain (--chain bsv) in internal/cli/tx.go
- [ ] T092 [US6] Implement utxo list command in internal/cli/utxo.go with `sigil utxo list --wallet <name> --chain bsv`
- [ ] T093 [US6] Format UTXO output as table with txid/vout/amount/confirmations columns in internal/cli/utxo.go

**Checkpoint**: User Story 6 complete - BSV transactions and UTXO listing work independently

---

## Phase 9: User Story 7 - List and Show Wallets (Priority: P3)

**Goal**: Users can list all wallets and view wallet details

**Independent Test**: Create multiple wallets and run list/show commands to verify output

### Implementation for User Story 7

- [ ] T094 [P] [US7] Implement wallet listing from storage directory in internal/wallet/storage.go (FR-030)
- [ ] T095 [US7] Define WalletSummary struct for listing in internal/wallet/wallet.go per contracts/wallet.go
- [ ] T096 [US7] Implement wallet list command in internal/cli/wallet.go with `sigil wallet list` (FR-030)
- [ ] T097 [US7] Format wallet list as table with name/created_at/chains/addresses columns in internal/cli/wallet.go
- [ ] T098 [US7] Implement wallet show command in internal/cli/wallet.go with `sigil wallet show <name>` (FR-031)
- [ ] T099 [US7] Display all derived addresses for each chain in wallet show command in internal/cli/wallet.go

**Checkpoint**: User Story 7 complete - wallet listing and details work independently

---

## Phase 10: User Story 8 - Import Key from WIF or Hex (Priority: P3)

**Goal**: Users can import raw private keys in WIF or hex format

**Independent Test**: Import a known WIF key and verify derived address matches

**Note**: `wallet import` is for single private keys (WIF/hex). `wallet restore` is for full HD wallets from mnemonic. Tasks T100-T103 MUST reuse parsing logic from T047-T049 (internal/wallet/restore.go) to avoid duplication.

### Implementation for User Story 8

- [ ] T100 [US8] Implement wallet import command in internal/cli/wallet.go with `sigil wallet import <name>` (reuses WIF/hex parsing from internal/wallet/restore.go)
- [ ] T101 [US8] Add --wif flag for WIF import mode with secure input in internal/cli/wallet.go
- [ ] T102 [US8] Validate WIF format and checksum in import command using shared validation from internal/wallet/restore.go (FR-011)
- [ ] T103 [US8] Derive address from imported key and display for verification in internal/cli/wallet.go

**Checkpoint**: User Story 8 complete - WIF/hex key import works independently

---

## Phase 11: User Story 9 - Configure Application Settings (Priority: P3)

**Goal**: Users can manage configuration via CLI commands

**Independent Test**: Run config commands and verify settings persist

### Implementation for User Story 9

- [ ] T104 [US9] Implement config init command in internal/cli/config.go with `sigil config init`
- [ ] T105 [US9] Create default config.yaml at ~/.sigil/config.yaml in config init command
- [ ] T106 [US9] Implement config show command in internal/cli/config.go with `sigil config show`
- [ ] T107 [US9] Implement config get command in internal/cli/config.go with `sigil config get <path>` (e.g., networks.eth.rpc)
- [ ] T108 [US9] Implement config set command in internal/cli/config.go with `sigil config set <path> <value>`
- [ ] T109 [US9] Validate config values before saving in config set command

**Checkpoint**: User Story 9 complete - config management works independently

---

## Phase 12: User Story 10 - Create and Verify Backup (Priority: P3)

**Goal**: Users can create encrypted backups and verify/restore them

**Independent Test**: Create a backup, verify it, and restore from it

### Implementation for User Story 10

- [ ] T110 [P] [US10] Define Backup and BackupManifest structs in internal/backup/manifest.go per data-model.md
- [ ] T111 [US10] Implement backup creation with age encryption in internal/backup/backup.go (FR-033)
- [ ] T112 [US10] Implement SHA256 checksum generation for backup integrity in internal/backup/backup.go (FR-034)
- [ ] T113 [US10] Implement backup verification (checksum + decrypt test) in internal/backup/backup.go (FR-034)
- [ ] T114 [US10] Implement backup restoration to wallet file in internal/backup/backup.go (FR-035)
- [ ] T115 [US10] Implement backup create command in internal/cli/backup.go with `sigil backup create --wallet <name>`
- [ ] T116 [US10] Create backup file at ~/.sigil/backups/<name>-<date>.sigil in backup create command
- [ ] T117 [US10] Implement backup verify command in internal/cli/backup.go with `sigil backup verify --input <path>`
- [ ] T118 [US10] Implement backup restore command in internal/cli/backup.go with `sigil backup restore --input <path>`

**Checkpoint**: User Story 10 complete - backup operations work independently

---

## Phase 13: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T119 [P] Add verbose API request/response logging to debug level in internal/config/logging.go (extends T020a/T020b)
- [ ] T120 [P] Add --help text with usage examples to all commands per FR-006/SC-006
- [ ] T121 Implement version command in internal/cli/root.go with `sigil version`
- [ ] T122 [P] Add shell completion generation (bash, zsh, fish) via cobra completions in internal/cli/root.go
- [ ] T123 Verify all exit codes match documented values per FR-006 in pkg/errors/errors.go
- [ ] T124 Run quickstart.md validation - verify all documented commands work as specified
- [ ] T124a Add fuzz tests for mnemonic parsing, address validation, transaction parsing in internal/**/fuzz_test.go
- [ ] T124b Create integration test suite running full quickstart.md workflow in tests/integration/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-12)**: All depend on Foundational phase completion
  - P1 stories (US1, US2) should complete first as they enable wallet functionality
  - P2 stories (US3-6) can proceed after P1 or in parallel
  - P3 stories (US7-10) can proceed after P1 or in parallel
- **Polish (Phase 13)**: Can run after core user stories are complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational - Shares mnemonic code with US1 but is independently testable
- **User Story 3 (P2)**: Needs wallet from US1/US2 to query, but core balance logic is independent
- **User Story 4 (P2)**: Needs wallet and ETH chain code, builds on US3 client infrastructure
- **User Story 5 (P2)**: Reuses TX infrastructure from US4 - minimal additional work
- **User Story 6 (P2)**: Needs wallet and BSV chain code, independent from ETH stories
- **User Story 7 (P3)**: Reuses wallet storage from US1 - minimal additional work
- **User Story 8 (P3)**: Builds on restoration logic from US2
- **User Story 9 (P3)**: Uses config infrastructure from Foundational - independent from wallet stories
- **User Story 10 (P3)**: Needs wallet from US1 - backup logic is self-contained

### Within Each Phase

- Setup tasks can run in parallel (T003-T005)
- Foundational tasks have internal dependencies but many [P] tasks can parallelize
- User story tasks follow: interfaces → models → services → CLI commands

### Parallel Opportunities

**Phase 1 (Setup)**:
```text
Parallel: T003 (deps) + T004 (main.go) + T005 (lint)
```

**Phase 2 (Foundational)**:
```text
Parallel: T007 (error helpers) with T006 (after sentinel errors)
Parallel: T008 (secure memory) + T009 (entropy) + T010 (age encryption)
Parallel: T014 (env vars), T016/T017/T018/T019/T020 (output formatters)
Parallel: T024 (chain constants) + T026/T027 (retry helpers)
```

**Phase 3 (User Story 1)**:
```text
Parallel: T028 (mnemonic gen) + T029 (mnemonic val) + T030 (normalize)
Parallel: T032 (ETH derivation) + T033 (BSV derivation)
```

---

## Implementation Strategy

### MVP First (User Stories 1-2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (wallet create)
4. Complete Phase 4: User Story 2 (wallet restore)
5. **STOP and VALIDATE**: Test wallet creation and restoration independently
6. Deploy/demo if ready - users can create and restore wallets

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 + US2 → Wallet management MVP
3. Add US3 → Balance checking
4. Add US4 + US5 + US6 → Transactions MVP
5. Add US7-10 → Full feature set

### Suggested MVP Scope

**MVP 1 (Core Wallet)**: Phases 1-4 (T001-T054, including T005b, T005c, T020a, T020b)
- Setup, Foundational, US1 (create), US2 (restore)
- ~58 tasks, enables basic wallet management

**MVP 2 (Add Transactions)**: + Phases 5-8 (T055-T093)
- Adds balance checking and send transactions
- 39 additional tasks, enables full transactional workflow

**MVP 3 (Complete)**: + Phases 9-13 (T094-T124b)
- Adds listing, config, backup, polish
- ~33 additional tasks, complete MVP feature set

---

## Notes

- [P] tasks = different files, no dependencies - can run in parallel
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Key material (seeds, private keys) must be zeroed after use per SecureBytes pattern
- All wallet files use 0600 permissions, directories use 0750

---

## Validation Workflow

### After Each Session
Run Go validations before ending any coding session:
```bash
magex format:fix && magex lint && magex test:race
```

### Before Commits
Run pre-commit hooks (excludes lint which magex handles):
```bash
go-pre-commit run --all-files --skip lint
```

### CI Expectations
- All tests pass with `-race` flag
- golangci-lint passes with project config
- No sensitive data in commits (pre-commit checks)

---

## Test Coverage Requirements

- Minimum 80% code coverage overall
- 100% coverage for `internal/crypto/` and `internal/wallet/mnemonic.go`
- All crypto operations must have test vectors from official specs (BIP39, BIP32, BIP44)
- Fuzz tests required for: mnemonic parsing, address validation, transaction parsing

---

## Library Safety

**Pre-Approved BSV Ecosystems** (use freely):
- `github.com/bitcoin-sv/*` - Official BSV Foundation (go-sdk, etc.)
- `github.com/libsv/*` - LibSV ecosystem (go-bt, go-bk, etc.)
- `github.com/BitcoinSchema/*` - BitcoinSchema ecosystem
- `github.com/mrz1836/go-whatsonchain` - WhatsOnChain API wrapper

**Approved Crypto Libraries**:
- `github.com/tyler-smith/go-bip39` - BIP39 (industry standard)
- `github.com/tyler-smith/go-bip32` - BIP32 (industry standard)
- `filippo.io/age` - Encryption (Go crypto team)
- `crypto/rand` from stdlib - Entropy

**DO NOT** add any other crypto/blockchain libraries without explicit approval.
