# Quickstart: Sigil MVP Development

**Date**: 2026-01-31 | **Plan**: [plan.md](./plan.md)

## Prerequisites

- Go 1.24+
- Git
- Make (optional, for convenience targets)

## Initial Setup

### 1. Clone and Initialize

```bash
cd /Users/mrz/projects/sigil
go mod init sigil
```

### 2. Install Dependencies

```bash
go get github.com/spf13/cobra@v1.8.0
go get github.com/spf13/viper@v1.18.0
go get github.com/tyler-smith/go-bip39@v1.1.0
go get github.com/tyler-smith/go-bip32@v1.0.0
go get github.com/bitcoin-sv/go-sdk@v1.1.0
go get github.com/mrz1836/go-whatsonchain@v0.14.0
go get github.com/ethereum/go-ethereum@v1.14.0
go get filippo.io/age@v1.1.1
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/agnivade/levenshtein@v1.1.1
go get golang.org/x/time@v0.5.0
go get golang.org/x/sys@v0.20.0
go get golang.org/x/term@v0.20.0

# Test dependencies
go get github.com/stretchr/testify@v1.9.0
```

### 3. Create Directory Structure

```bash
mkdir -p cmd/sigil
mkdir -p internal/{cli,wallet,chain/eth,chain/bsv,crypto,config,output,cache,backup}
mkdir -p pkg/errors
mkdir -p testdata/{mnemonics,wallets,config}
```

## Building

### Basic Build

```bash
go build -o bin/sigil ./cmd/sigil
```

### Development Build (with race detection)

```bash
go build -race -o bin/sigil ./cmd/sigil
```

### Release Build

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/sigil ./cmd/sigil
```

## Running

### First Run

```bash
# Initialize configuration
./bin/sigil config init

# Create a test wallet
./bin/sigil wallet create test --words 12
```

### Common Commands

```bash
# List wallets
./bin/sigil wallet list

# Check balances
./bin/sigil balance show --wallet test

# Send transaction (BSV)
./bin/sigil tx send --wallet test --to 1abc... --amount 0.001 --chain bsv

# JSON output
./bin/sigil wallet list -o json
```

## Testing

### Run All Tests

```bash
go test ./...
```

### Run Tests with Race Detection

```bash
go test -race ./...
```

### Run Specific Package Tests

```bash
go test ./internal/wallet/...
go test ./internal/chain/bsv/...
```

### Run Integration Tests (requires network)

```bash
go test -tags=integration ./...
```

### Run Fuzz Tests

```bash
# Run for 30 seconds
go test -fuzz=FuzzValidateMnemonic -fuzztime=30s ./internal/wallet/

# Run until failure
go test -fuzz=FuzzParseAmount ./internal/chain/eth/
```

### Test Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Linting

### Run Linter

```bash
golangci-lint run
```

### Fix Auto-fixable Issues

```bash
golangci-lint run --fix
```

## Development Workflow

### 1. Start a New Feature

```bash
git checkout -b feature/xyz
```

### 2. Make Changes

Edit code in `internal/` or `cmd/`.

### 3. Format and Lint

```bash
go fmt ./...
golangci-lint run
```

### 4. Test

```bash
go test -race ./...
```

### 5. Commit

```bash
git add -A
git commit -m "feat(wallet): add XYZ functionality"
```

## Code Patterns

### Adding a New CLI Command

1. Create command file in `internal/cli/`:

```go
// internal/cli/example.go
package cli

import "github.com/spf13/cobra"

var exampleCmd = &cobra.Command{
    Use:   "example",
    Short: "Example noun for operations",
}

var exampleDoCmd = &cobra.Command{
    Use:   "do <arg>",
    Short: "Do something with example",
    Args:  cobra.ExactArgs(1),
    RunE:  runExampleDo,
}

func init() {
    exampleCmd.AddCommand(exampleDoCmd)
    rootCmd.AddCommand(exampleCmd)
}

func runExampleDo(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}
```

### Adding Chain Support

1. Create package in `internal/chain/<chain>/`
2. Implement the `Chain` interface from `contracts/chain.go`
3. Register in chain factory

### Error Handling

```go
import "sigil/pkg/errors"

// Return sentinel errors
if notFound {
    return errors.ErrWalletNotFound
}

// Wrap with context
if err != nil {
    return fmt.Errorf("loading wallet %s: %w", name, err)
}

// Add details for user display
return errors.WithDetails(errors.ErrInsufficientFunds, map[string]string{
    "required":  "0.5",
    "available": "0.1",
    "symbol":    "ETH",
})
```

### Secure Memory Pattern

```go
import "sigil/internal/crypto"

// Create secure buffer
buf, err := crypto.NewSecureBytes(32)
if err != nil {
    return err
}
defer buf.Destroy() // Always defer cleanup

// Use the buffer
copy(buf.Bytes(), sensitiveData)

// Process...

// Destruction happens automatically on return
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SIGIL_HOME` | Data directory | `~/.sigil` |
| `SIGIL_ETH_RPC` | Ethereum RPC URL | (none) |
| `SIGIL_BSV_API_KEY` | WhatsOnChain API key | (none) |
| `SIGIL_OUTPUT_FORMAT` | Default output format | `auto` |
| `SIGIL_VERBOSE` | Enable verbose output | `false` |
| `NO_COLOR` | Disable colored output | (not set) |

## Debugging

### Enable Debug Logging

```bash
# In config.yaml
logging:
  level: debug
  file: ~/.sigil/sigil.log

# Or via environment
SIGIL_VERBOSE=true ./bin/sigil wallet list
```

### Inspect Encrypted Wallet

```bash
# View metadata only (not encrypted content)
cat ~/.sigil/wallets/test.wallet | head -c 200
```

### Test API Connectivity

```bash
# BSV (WhatsOnChain)
curl https://api.whatsonchain.com/v1/bsv/main/chain/info

# ETH (your configured RPC)
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  YOUR_ETH_RPC_URL
```

## Common Issues

### "mlock failed" Warning

The secure memory system tries to lock memory pages. This may fail on systems with low mlock limits:

```bash
# Check current limit
ulimit -l

# Increase limit (may require sudo)
ulimit -l unlimited
```

The application continues working even if mlock fails, but keys may be swappable.

### "Config file not found"

Run `sigil config init` to create the default configuration.

### ETH RPC Not Configured

Edit `~/.sigil/config.yaml` and set `networks.eth.rpc` to your Infura/Alchemy/local node URL.

## Resources

- [PRD](../../docs/PRD.md) — Product requirements
- [Spec](./spec.md) — Feature specification
- [Data Model](./data-model.md) — Entity definitions
- [Research](./research.md) — Technical decisions
- [Contracts](./contracts/) — Interface definitions
