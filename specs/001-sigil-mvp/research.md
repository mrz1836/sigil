# Research: Sigil MVP

**Date**: 2026-01-31 | **Plan**: [plan.md](./plan.md)

## 1. Go BIP39/BIP32 Libraries

### Decision
Use `github.com/tyler-smith/go-bip39` for mnemonic generation/validation and `github.com/tyler-smith/go-bip32` for HD key derivation.

### Rationale
- Most widely used Go BIP39/BIP32 implementations with active maintenance
- Compatible with standard BIP39 wordlist (English)
- Supports 12/24 word mnemonics (128/256 bits entropy)
- Well-tested against BIP39 test vectors
- Pure Go implementation (no CGO dependencies)

### Alternatives Considered
| Library | Pros | Cons | Rejected Because |
|---------|------|------|------------------|
| `btcsuite/btcutil/hdkeychain` | Part of btcsuite ecosystem | Different API style, no mnemonic support | Need mnemonic handling |
| `cosmos/go-bip39` | Cosmos ecosystem standard | Less general-purpose focus | Tyler-smith more widely adopted |
| Custom implementation | Full control | High risk for crypto code | Security critical, use proven library |

### Usage Pattern
```go
// Generation
entropy, _ := bip39.NewEntropy(256) // 24 words
mnemonic, _ := bip39.NewMnemonic(entropy)

// Validation
valid := bip39.IsMnemonicValid(mnemonic)

// Seed derivation (with optional passphrase)
seed := bip39.NewSeed(mnemonic, passphrase)

// HD key derivation
masterKey, _ := bip32.NewMasterKey(seed)
```

---

## 2. BSV SDK Integration

### Decision
Use `github.com/bitcoin-sv/go-sdk` for BSV transaction building and signing. Use `github.com/mrz1836/go-whatsonchain` for API queries.

### Rationale
- Official BSV SDK maintained by Bitcoin SV Foundation
- Supports P2PKH transaction building (MVP requirement)
- Compatible with modern BSV transaction formats
- go-whatsonchain provides clean WhatsOnChain API wrapper already referenced in PRD

### Alternatives Considered
| Library | Pros | Cons | Rejected Because |
|---------|------|------|------------------|
| Manual implementation | Full control | Complex, error-prone | BSV transactions have nuances |
| `libsv/go-bt` | Lightweight | Less maintained | Official SDK preferred |
| `bitcoinsv/bsvd` | Full node library | Heavyweight | Need lightweight client only |

### Usage Pattern
```go
// Transaction building with go-sdk
import "github.com/bitcoin-sv/go-sdk/transaction"

tx := transaction.NewTransaction()
tx.AddInput(utxo)
tx.AddOutput(recipientScript, amount)
tx.Sign(privateKey)

// API queries with go-whatsonchain
import "github.com/mrz1836/go-whatsonchain"

client := whatsonchain.NewClient()
balance, _ := client.AddressBalance(address)
utxos, _ := client.AddressUnspentTransactions(address)
```

---

## 3. Age Encryption Patterns

### Decision
Use password-based age encryption with Argon2id key derivation for wallet files. Store recipient identity in `~/.sigil/identity.age` for future multi-recipient support.

### Rationale
- Password-based encryption simplest UX for single-user wallet
- Argon2id is memory-hard, resistant to GPU attacks
- Age is modern, audited, and simple to use
- File format is portable and standardized

### Alternatives Considered
| Approach | Pros | Cons | Rejected Because |
|----------|------|------|------------------|
| Identity-based only | Standard age pattern | Requires managing identity file | Password simpler for MVP |
| NaCl secretbox | Fast | No standard file format | Age provides better tooling |
| GPG | Widely deployed | Complex key management | Overkill for single-user |

### Implementation Pattern
```go
import "filippo.io/age"

// Encrypt wallet data
func EncryptWallet(data []byte, password string) ([]byte, error) {
    recipient, _ := age.NewScryptRecipient(password)
    buf := &bytes.Buffer{}
    w, _ := age.Encrypt(buf, recipient)
    w.Write(data)
    w.Close()
    return buf.Bytes(), nil
}

// Decrypt wallet data
func DecryptWallet(encrypted []byte, password string) ([]byte, error) {
    identity, _ := age.NewScryptIdentity(password)
    r, _ := age.Decrypt(bytes.NewReader(encrypted), identity)
    return io.ReadAll(r)
}
```

### File Permissions
- Wallet files: `0600` (owner read/write only)
- Config files: `0640` (owner read/write, group read)
- Directories: `0750` (owner full, group read/execute)

---

## 4. Ethereum go-ethereum Usage

### Decision
Use `github.com/ethereum/go-ethereum` (geth) for all Ethereum operations including key derivation, transaction building, and RPC calls.

### Rationale
- Official Ethereum Go implementation
- Full EIP-55 checksum address support
- ERC-20 ABI handling included
- Well-tested transaction signing
- Active maintenance and security updates

### Alternatives Considered
| Library | Pros | Cons | Rejected Because |
|---------|------|------|------------------|
| `wealdtech/go-ens` | ENS support | Additional dependency | ENS not in MVP scope |
| `umbracle/ethgo` | Lightweight | Less ecosystem integration | Geth is standard |
| Custom RPC client | Minimal deps | Lots of work | Geth already proven |

### Usage Pattern
```go
import (
    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/ethclient"
)

// Connect to RPC
client, _ := ethclient.Dial(rpcURL)

// Check ETH balance
balance, _ := client.BalanceAt(ctx, address, nil)

// ERC-20 balance (USDC)
usdc, _ := NewUSDC(usdcAddress, client)
tokenBalance, _ := usdc.BalanceOf(nil, address)
```

### USDC Contract Details
- Mainnet address: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`
- Decimals: 6
- Standard ERC-20 interface

---

## 5. CLI Framework Selection

### Decision
Use `github.com/spf13/cobra` for CLI framework with `github.com/spf13/viper` for configuration.

### Rationale
- Industry standard for Go CLIs (kubectl, hugo, gh)
- Built-in help generation
- Supports noun-verb command pattern natively
- Shell completion generation
- Persistent flags for global options

### Alternatives Considered
| Library | Pros | Cons | Rejected Because |
|---------|------|------|------------------|
| `urfave/cli` | Simple API | Less structured for noun-verb | Cobra better for complex CLIs |
| `alecthomas/kong` | Struct-based | Less adoption | Cobra ecosystem stronger |
| Standard library | No deps | Lots of boilerplate | Cobra saves significant work |

### Command Structure
```go
// Root command
var rootCmd = &cobra.Command{Use: "sigil"}

// Noun commands
var walletCmd = &cobra.Command{Use: "wallet"}
var balanceCmd = &cobra.Command{Use: "balance"}

// Verb commands under nouns
var walletCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new HD wallet",
}

// Registration
walletCmd.AddCommand(walletCreateCmd)
rootCmd.AddCommand(walletCmd)
```

---

## 6. Levenshtein Distance for Typo Correction

### Decision
Use `github.com/agnivade/levenshtein` for mnemonic word typo detection and suggestions.

### Rationale
- Pure Go, no CGO
- Efficient O(n*m) implementation
- Well-tested
- Minimal API surface

### Alternatives Considered
| Library | Pros | Cons | Rejected Because |
|---------|------|------|------------------|
| `textdistance` | Multiple algorithms | Heavier | Only need Levenshtein |
| Custom implementation | No deps | Risk of bugs | Proven library preferred |

### Implementation Pattern
```go
import "github.com/agnivade/levenshtein"

// Find closest BIP39 word for typo
func SuggestWord(input string, wordlist []string) string {
    minDist := math.MaxInt
    var suggestion string
    for _, word := range wordlist {
        dist := levenshtein.ComputeDistance(input, word)
        if dist < minDist {
            minDist = dist
            suggestion = word
        }
    }
    if minDist <= 2 { // threshold for "close enough"
        return suggestion
    }
    return ""
}
```

---

## 7. Rate Limiting Patterns

### Decision
Implement client-side token bucket rate limiter using `golang.org/x/time/rate`. Respect `Retry-After` headers from APIs.

### Rationale
- Standard library extension, well-maintained
- Token bucket allows bursts while respecting average rate
- Simple API with `Allow()` and `Wait()` methods
- Per-endpoint rate limiting supported

### Configuration
- Default: 5 requests/second per endpoint
- Configurable via `config.yaml`
- Burst allowance: 10 requests

### Implementation Pattern
```go
import "golang.org/x/time/rate"

type RateLimitedClient struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func (c *RateLimitedClient) getLimiter(endpoint string) *rate.Limiter {
    c.mu.RLock()
    limiter, exists := c.limiters[endpoint]
    c.mu.RUnlock()
    if exists {
        return limiter
    }

    c.mu.Lock()
    defer c.mu.Unlock()
    limiter = rate.NewLimiter(rate.Limit(5), 10) // 5/sec, burst 10
    c.limiters[endpoint] = limiter
    return limiter
}

func (c *RateLimitedClient) Do(ctx context.Context, endpoint string, req *http.Request) (*http.Response, error) {
    if err := c.getLimiter(endpoint).Wait(ctx); err != nil {
        return nil, err
    }
    return http.DefaultClient.Do(req)
}
```

### Retry-After Handling
```go
func handleResponse(resp *http.Response) (bool, time.Duration) {
    if resp.StatusCode == 429 {
        if after := resp.Header.Get("Retry-After"); after != "" {
            if seconds, err := strconv.Atoi(after); err == nil {
                return true, time.Duration(seconds) * time.Second
            }
        }
        return true, 5 * time.Second // default backoff
    }
    return false, 0
}
```

---

## 8. Secure Memory Handling

### Decision
Use `golang.org/x/sys/unix` for mlock/munlock on sensitive data. Implement `SecureBytes` wrapper with explicit zeroing.

### Rationale
- Prevents sensitive data from being swapped to disk
- Explicit zeroing prevents accidental data leaks
- Standard unix syscall interface
- Works on Linux and macOS (Windows has different API)

### Limitations
- Requires sufficient mlock limit (ulimit -l)
- May fail silently on some systems
- Windows requires different implementation (VirtualLock)

### Implementation Pattern
```go
import "golang.org/x/sys/unix"

type SecureBytes struct {
    data   []byte
    locked bool
}

func NewSecureBytes(size int) (*SecureBytes, error) {
    data := make([]byte, size)

    // Try to lock memory, but don't fail if not possible
    err := unix.Mlock(data)
    locked := err == nil

    return &SecureBytes{data: data, locked: locked}, nil
}

func (s *SecureBytes) Destroy() {
    // Zero the data
    for i := range s.data {
        s.data[i] = 0
    }

    // Unlock if locked
    if s.locked {
        _ = unix.Munlock(s.data)
    }

    s.data = nil
}

func (s *SecureBytes) Bytes() []byte {
    return s.data
}
```

### Cross-Platform Consideration
```go
// +build !windows

// Unix implementation uses mlock

// +build windows

// Windows implementation uses VirtualLock
import "golang.org/x/sys/windows"

func mlock(data []byte) error {
    return windows.VirtualLock(unsafe.Pointer(&data[0]), uintptr(len(data)))
}
```

---

## 9. Retry and Exponential Backoff

### Decision
Implement retry with exponential backoff using custom implementation. Base delays: 1s, 2s, 4s (3 retries max).

### Rationale
- Simple enough to implement inline
- Matches FR-039 specification exactly
- No additional dependencies needed

### Implementation Pattern
```go
func withRetry[T any](ctx context.Context, operation func() (T, error)) (T, error) {
    var result T
    var err error

    delays := []time.Duration{
        1 * time.Second,
        2 * time.Second,
        4 * time.Second,
    }

    for i := 0; i <= len(delays); i++ {
        result, err = operation()
        if err == nil {
            return result, nil
        }

        // Check if error is retryable
        if !isRetryable(err) {
            return result, err
        }

        // Don't sleep after last attempt
        if i < len(delays) {
            select {
            case <-ctx.Done():
                return result, ctx.Err()
            case <-time.After(delays[i]):
            }
        }
    }

    return result, fmt.Errorf("operation failed after %d retries: %w", len(delays)+1, err)
}

func isRetryable(err error) bool {
    // Network errors, timeouts, 5xx responses are retryable
    // 4xx errors (except 429) are not retryable
    return errors.Is(err, context.DeadlineExceeded) ||
           errors.Is(err, io.ErrUnexpectedEOF) ||
           isNetworkError(err)
}
```

---

## 10. Balance Caching Strategy

### Decision
File-based cache at `~/.sigil/cache/balances.json` with per-chain/address timestamps.

### Rationale
- Survives CLI restarts
- Simple JSON format for debugging
- Per-entry TTL allows partial staleness
- Supports FR-041 requirement for stale data with warning

### Cache Structure
```go
type BalanceCache struct {
    Entries map[string]CacheEntry `json:"entries"` // key: "chain:address"
}

type CacheEntry struct {
    Balance   string    `json:"balance"`
    Symbol    string    `json:"symbol"`
    Decimals  int       `json:"decimals"`
    UpdatedAt time.Time `json:"updated_at"`
}

// Cache key format: "eth:0x742d35Cc..." or "bsv:1A1zP1eP..."
```

### Staleness Warning
```go
func (c *BalanceCache) Get(chain, address string) (*CacheEntry, bool, time.Duration) {
    key := fmt.Sprintf("%s:%s", chain, address)
    entry, exists := c.Entries[key]
    if !exists {
        return nil, false, 0
    }

    age := time.Since(entry.UpdatedAt)
    return &entry, true, age
}
```

---

## Summary: Dependency List

### Direct Dependencies (go.mod)

```go
require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    github.com/tyler-smith/go-bip39 v1.1.0
    github.com/tyler-smith/go-bip32 v1.0.0
    github.com/bitcoin-sv/go-sdk v1.1.0
    github.com/mrz1836/go-whatsonchain v0.14.0
    github.com/ethereum/go-ethereum v1.14.0
    filippo.io/age v1.1.1
    gopkg.in/yaml.v3 v3.0.1
    github.com/agnivade/levenshtein v1.1.1
    golang.org/x/time v0.5.0
    golang.org/x/sys v0.20.0
    golang.org/x/term v0.20.0
)
```

### Test Dependencies

```go
require (
    github.com/stretchr/testify v1.9.0
)
```

---

## Open Questions (Resolved)

All technical questions have been resolved through this research phase. No NEEDS CLARIFICATION items remain.
