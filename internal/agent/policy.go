package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/fileutil"
)

// counterFilePermissions is the permission mode for counter files.
const counterFilePermissions = 0o600

// DailyCounter tracks daily spending for an agent.
type DailyCounter struct {
	// Date is the UTC date string (YYYY-MM-DD) this counter is for.
	Date string `json:"date"`

	// SpentSat is the total satoshis spent today.
	SpentSat uint64 `json:"spent_sat"`

	// SpentWei is the total wei spent today (string for precision).
	SpentWei string `json:"spent_wei"`

	// HMAC is the HMAC-SHA256 of the counter data, keyed with the token.
	HMAC string `json:"hmac"`
}

// spentWeiBig returns SpentWei as a *big.Int. Returns zero if unset.
func (dc *DailyCounter) spentWeiBig() *big.Int {
	if dc.SpentWei == "" || dc.SpentWei == "0" {
		return new(big.Int)
	}
	v, ok := new(big.Int).SetString(dc.SpentWei, 10)
	if !ok {
		return new(big.Int)
	}
	return v
}

// todayDate returns today's date string in UTC.
func todayDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

// ValidateTransaction checks if a transaction is allowed by the agent policy.
// chainID is the blockchain being used.
// to is the destination address.
// amountSmallest is the amount in smallest units (satoshis for BSV, wei for ETH).
//
//nolint:gocognit,gocyclo // Multi-chain transaction validation requires conditional branches
func ValidateTransaction(cred *Credential, chainID chain.ID, to string, amountSmallest *big.Int) error {
	policy := &cred.Policy

	// Check chain authorization
	if !cred.HasChain(chainID) {
		return fmt.Errorf("%w: %q (allowed: %v)", ErrChainDenied, chainID, cred.Chains)
	}

	// Check address allowlist
	if len(policy.AllowedAddrs) > 0 {
		allowed := false
		for _, addr := range policy.AllowedAddrs {
			if addr == to {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: %q", ErrAddrDenied, to)
		}
	}

	// Check per-transaction limit
	switch chainID {
	case chain.BSV, chain.BTC, chain.BCH:
		if policy.MaxPerTxSat > 0 {
			limit := new(big.Int).SetUint64(policy.MaxPerTxSat)
			if amountSmallest.Cmp(limit) > 0 {
				return fmt.Errorf("%w: %s sat exceeds limit of %d sat",
					ErrPerTxLimit, amountSmallest.String(), policy.MaxPerTxSat)
			}
		}
	case chain.ETH:
		if maxWei := policy.MaxPerTxWeiBig(); maxWei != nil {
			if amountSmallest.Cmp(maxWei) > 0 {
				return fmt.Errorf("%w: %s wei exceeds limit of %s wei",
					ErrPerTxLimit, amountSmallest.String(), policy.MaxPerTxWei)
			}
		}
	}

	return nil
}

// CheckDailyLimit checks if the daily spending limit would be exceeded.
// counterPath is the path to the counter file.
// token is used for HMAC verification of the counter.
//
//nolint:gocognit // Multi-chain daily limit checking requires conditional branches
func CheckDailyLimit(counterPath, token string, cred *Credential, chainID chain.ID, amountSmallest *big.Int) error {
	policy := &cred.Policy

	// Load or initialize counter
	counter := loadCounter(counterPath, token)

	switch chainID {
	case chain.BSV, chain.BTC, chain.BCH:
		if policy.MaxDailySat == 0 {
			return nil // No daily limit
		}
		newTotal := counter.SpentSat + amountSmallest.Uint64()
		if newTotal < counter.SpentSat { // Overflow check
			return ErrDailyOverflow
		}
		if newTotal > policy.MaxDailySat {
			remaining := policy.MaxDailySat - counter.SpentSat
			return fmt.Errorf("%w: %s sat would exceed limit of %d sat (spent today: %d sat, remaining: %d sat)",
				ErrDailyLimitExceed, amountSmallest.String(), policy.MaxDailySat, counter.SpentSat, remaining)
		}
	case chain.ETH:
		maxDaily := policy.MaxDailyWeiBig()
		if maxDaily == nil {
			return nil // No daily limit
		}
		spentWei := counter.spentWeiBig()
		newTotal := new(big.Int).Add(spentWei, amountSmallest)
		if newTotal.Cmp(maxDaily) > 0 {
			remaining := new(big.Int).Sub(maxDaily, spentWei)
			if remaining.Sign() < 0 {
				remaining = new(big.Int)
			}
			return fmt.Errorf("%w: %s wei would exceed limit of %s wei (spent today: %s wei, remaining: %s wei)",
				ErrDailyLimitExceed, amountSmallest.String(), policy.MaxDailyWei, spentWei.String(), remaining.String())
		}
	}

	return nil
}

// RecordSpend records a completed spend in the daily counter.
func RecordSpend(counterPath, token string, chainID chain.ID, amountSmallest *big.Int) error {
	counter := loadCounter(counterPath, token)

	switch chainID {
	case chain.BSV, chain.BTC, chain.BCH:
		counter.SpentSat += amountSmallest.Uint64()
	case chain.ETH:
		spentWei := counter.spentWeiBig()
		newSpent := new(big.Int).Add(spentWei, amountSmallest)
		counter.SpentWei = newSpent.String()
	}

	return saveCounter(counterPath, token, counter)
}

// GetDailySpent returns the daily spending totals for an agent.
func GetDailySpent(counterPath, token string) (satSpent uint64, weiSpent string) {
	counter := loadCounter(counterPath, token)
	return counter.SpentSat, counter.SpentWei
}

// ErrCounterTampered indicates a counter file was found but its integrity check failed.
// This may indicate tampering and causes the counter to be treated as at-limit (deny).
var ErrCounterTampered = fmt.Errorf("daily counter integrity check failed: possible tampering")

// loadCounter loads the daily counter from disk, or returns a fresh one.
// If the counter is for a previous date, returns a fresh counter (daily reset).
//
// Security: If the counter file exists but fails integrity checks (corrupt JSON,
// HMAC mismatch), the counter is treated as at maximum spend. This prevents an
// attacker from resetting daily limits by deleting or corrupting the counter file.
// A missing file on a new day is the only case where a zero counter is returned.
func loadCounter(counterPath, token string) *DailyCounter {
	today := todayDate()

	if counterPath == "" {
		return &DailyCounter{Date: today}
	}

	//nolint:gosec // G304: Path is from validated internal store
	data, err := os.ReadFile(counterPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist — fresh start (first spend of the day)
			return &DailyCounter{Date: today}
		}
		// File exists but can't be read (permissions, I/O error) —
		// treat as tampered to deny further spending.
		return maxedCounter(today)
	}

	var counter DailyCounter
	if err := json.Unmarshal(data, &counter); err != nil {
		// Corrupted JSON — treat as tampered (deny)
		return maxedCounter(today)
	}

	// If the counter is for a different day, reset
	if counter.Date != today {
		return &DailyCounter{Date: today}
	}

	// Verify HMAC
	if !verifyCounterHMAC(&counter, token) {
		// Tampered counter — deny further spending
		return maxedCounter(today)
	}

	return &counter
}

// maxedCounter returns a counter that appears to have spent the maximum amount,
// effectively blocking further spending. Used when counter integrity is suspect.
func maxedCounter(date string) *DailyCounter {
	return &DailyCounter{
		Date:     date,
		SpentSat: ^uint64(0),                             // math.MaxUint64
		SpentWei: "999999999999999999999999999999999999", // Effectively infinite
	}
}

// saveCounter writes the daily counter to disk with HMAC.
func saveCounter(counterPath, token string, counter *DailyCounter) error {
	if counterPath == "" {
		return nil
	}

	// Compute HMAC
	counter.HMAC = computeCounterHMAC(counter, token)

	data, err := json.MarshalIndent(counter, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling counter: %w", err)
	}

	return fileutil.WriteAtomic(counterPath, data, counterFilePermissions)
}

// computeCounterHMAC computes the HMAC for a counter (excluding the HMAC field).
func computeCounterHMAC(counter *DailyCounter, token string) string {
	// Create a copy without the HMAC field for hashing
	payload := fmt.Sprintf("%s:%d:%s", counter.Date, counter.SpentSat, counter.SpentWei)
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyCounterHMAC verifies the HMAC of a counter.
func verifyCounterHMAC(counter *DailyCounter, token string) bool {
	expected := computeCounterHMAC(counter, token)
	return hmac.Equal([]byte(expected), []byte(counter.HMAC))
}
