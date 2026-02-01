# sigil Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-01-31

## Active Technologies

- Go 1.24 (per PRD references) (001-sigil-mvp)

## Project Structure

```text
cmd/sigil/                    # Entry point
internal/                     # Core logic (cli, wallet, chain, crypto, config, output, cache, backup)
pkg/errors/                   # Shared error types
testdata/                     # Test fixtures
```

## Commands

```bash
# Validation - run after each session
magex format:fix && magex lint && magex test:race

# Pre-commit - run before commits
go-pre-commit run --all-files --skip lint

# Testing
go test ./... -race           # Run all tests with race detector
go test ./... -cover          # Run with coverage report
go test -fuzz=FuzzParseMnemonic ./internal/wallet/  # Example fuzz test
```

## Code Style

Go 1.24 (per PRD references): Follow standard conventions

## Recent Changes

- 001-sigil-mvp: Added Go 1.24 (per PRD references)

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
