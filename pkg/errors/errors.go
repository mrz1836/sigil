// Package errors provides structured error handling for Sigil.
// It defines sentinel errors, exit codes, and helpers for adding
// context, details, and suggestions to errors.
//
//nolint:revive // Package name intentionally shadows stdlib for domain-specific error handling
package errors

import (
	"errors"
	"fmt"
	"sort"
)

// Exit codes per FR-006.
const (
	ExitSuccess    = 0 // Successful execution
	ExitGeneral    = 1 // General/unknown error
	ExitInput      = 2 // Invalid input
	ExitAuth       = 3 // Authentication failed
	ExitNotFound   = 4 // Resource not found
	ExitPermission = 5 // Permission denied or insufficient funds
)

// SigilError is the structured error type for Sigil.
type SigilError struct {
	Code       string            // Machine-readable error code
	Message    string            // Human-readable message
	Details    map[string]string // Additional context
	Suggestion string            // Actionable suggestion for user
	Cause      error             // Underlying error
	ExitCode   int               // Exit code for CLI
}

func (e *SigilError) Error() string {
	msg := e.Message

	// Include details in error message (sorted for deterministic output)
	if len(e.Details) > 0 {
		keys := make([]string, 0, len(e.Details))
		for k := range e.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			msg = fmt.Sprintf("%s (%s: %s)", msg, k, e.Details[k])
		}
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}

func (e *SigilError) Unwrap() error {
	return e.Cause
}

// Is implements errors.Is for SigilError.
func (e *SigilError) Is(target error) bool {
	var t *SigilError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// Sentinel errors.
var (
	ErrGeneral = &SigilError{
		Code:     "GENERAL_ERROR",
		Message:  "an error occurred",
		ExitCode: ExitGeneral,
	}

	ErrInvalidInput = &SigilError{
		Code:     "INVALID_INPUT",
		Message:  "invalid input",
		ExitCode: ExitInput,
	}

	ErrAuthentication = &SigilError{
		Code:     "AUTHENTICATION_FAILED",
		Message:  "authentication failed",
		ExitCode: ExitAuth,
	}

	ErrNotFound = &SigilError{
		Code:     "NOT_FOUND",
		Message:  "resource not found",
		ExitCode: ExitNotFound,
	}

	ErrPermission = &SigilError{
		Code:     "PERMISSION_DENIED",
		Message:  "permission denied",
		ExitCode: ExitPermission,
	}

	ErrInsufficientFunds = &SigilError{
		Code:     "INSUFFICIENT_FUNDS",
		Message:  "insufficient funds for transaction",
		ExitCode: ExitPermission,
	}

	// Wallet-specific errors.
	ErrWalletNotFound = &SigilError{
		Code:     "WALLET_NOT_FOUND",
		Message:  "wallet not found",
		ExitCode: ExitNotFound,
	}

	ErrWalletExists = &SigilError{
		Code:     "WALLET_EXISTS",
		Message:  "wallet already exists",
		ExitCode: ExitInput,
	}

	ErrInvalidMnemonic = &SigilError{
		Code:     "INVALID_MNEMONIC",
		Message:  "invalid mnemonic phrase",
		ExitCode: ExitInput,
	}

	ErrDecryptionFailed = &SigilError{
		Code:     "DECRYPTION_FAILED",
		Message:  "decryption failed - wrong password or corrupted file",
		ExitCode: ExitAuth,
	}

	// Chain-specific errors.
	ErrInvalidAddress = &SigilError{
		Code:     "INVALID_ADDRESS",
		Message:  "invalid address format",
		ExitCode: ExitInput,
	}

	ErrNetworkError = &SigilError{
		Code:     "NETWORK_ERROR",
		Message:  "network communication failed",
		ExitCode: ExitGeneral,
	}

	ErrTxRejected = &SigilError{
		Code:     "TX_REJECTED",
		Message:  "transaction rejected by network",
		ExitCode: ExitGeneral,
	}

	// Config-specific errors.
	ErrConfigNotFound = &SigilError{
		Code:     "CONFIG_NOT_FOUND",
		Message:  "configuration file not found",
		ExitCode: ExitNotFound,
	}

	ErrConfigInvalid = &SigilError{
		Code:     "CONFIG_INVALID",
		Message:  "configuration file is invalid",
		ExitCode: ExitInput,
	}

	// Backup-specific errors.
	ErrBackupNotFound = &SigilError{
		Code:     "BACKUP_NOT_FOUND",
		Message:  "backup file not found",
		ExitCode: ExitNotFound,
	}

	ErrBackupCorrupted = &SigilError{
		Code:     "BACKUP_CORRUPTED",
		Message:  "backup file is corrupted - checksum mismatch",
		ExitCode: ExitInput,
	}

	// Chain-specific operation errors.
	ErrNotImplemented = &SigilError{
		Code:     "NOT_IMPLEMENTED",
		Message:  "operation not implemented yet",
		ExitCode: ExitGeneral,
	}

	ErrCacheNotFound = &SigilError{
		Code:     "CACHE_NOT_FOUND",
		Message:  "no cached data available",
		ExitCode: ExitNotFound,
	}

	ErrNotSupported = &SigilError{
		Code:     "NOT_SUPPORTED",
		Message:  "operation not supported for this chain",
		ExitCode: ExitInput,
	}

	// Transaction-specific errors.
	ErrAmountRequired = &SigilError{
		Code:     "AMOUNT_REQUIRED",
		Message:  "amount is required",
		ExitCode: ExitInput,
	}

	ErrInvalidChecksum = &SigilError{
		Code:     "INVALID_CHECKSUM",
		Message:  "invalid address checksum",
		ExitCode: ExitInput,
	}

	ErrUnsupportedVersion = &SigilError{
		Code:     "UNSUPPORTED_VERSION",
		Message:  "unsupported address version",
		ExitCode: ExitInput,
	}

	ErrInvalidGasSpeed = &SigilError{
		Code:     "INVALID_GAS_SPEED",
		Message:  "invalid gas speed",
		ExitCode: ExitInput,
	}

	ErrInvalidFeeStrategy = &SigilError{
		Code:     "INVALID_FEE_STRATEGY",
		Message:  "invalid fee strategy",
		ExitCode: ExitInput,
	}

	ErrInvalidValue = &SigilError{
		Code:     "INVALID_VALUE",
		Message:  "invalid value",
		ExitCode: ExitInput,
	}

	ErrInvalidGasPrice = &SigilError{
		Code:     "INVALID_GAS_PRICE",
		Message:  "gas price cannot be nil",
		ExitCode: ExitInput,
	}

	ErrInvalidChainID = &SigilError{
		Code:     "INVALID_CHAIN_ID",
		Message:  "chain ID cannot be nil",
		ExitCode: ExitInput,
	}

	ErrInvalidGasLimit = &SigilError{
		Code:     "INVALID_GAS_LIMIT",
		Message:  "gas limit cannot be zero",
		ExitCode: ExitInput,
	}

	ErrInvalidPublicKey = &SigilError{
		Code:     "INVALID_PUBLIC_KEY",
		Message:  "error casting public key to ECDSA",
		ExitCode: ExitInput,
	}

	ErrUnknownConfigKey = &SigilError{
		Code:     "UNKNOWN_CONFIG_KEY",
		Message:  "unknown config key",
		ExitCode: ExitInput,
	}

	ErrInvalidAmount = &SigilError{
		Code:     "INVALID_AMOUNT",
		Message:  "invalid amount format",
		ExitCode: ExitInput,
	}

	ErrTransactionNotFound = &SigilError{
		Code:     "TRANSACTION_NOT_FOUND",
		Message:  "transaction not found",
		ExitCode: ExitNotFound,
	}

	ErrTokenNotFound = &SigilError{
		Code:     "TOKEN_NOT_FOUND",
		Message:  "token not found",
		ExitCode: ExitNotFound,
	}

	ErrInvalidTransaction = &SigilError{
		Code:     "INVALID_TRANSACTION",
		Message:  "invalid transaction",
		ExitCode: ExitInput,
	}

	ErrDataTooLarge = &SigilError{
		Code:     "DATA_TOO_LARGE",
		Message:  "data exceeds maximum size",
		ExitCode: ExitInput,
	}

	ErrInvalidFormat = &SigilError{
		Code:     "INVALID_FORMAT",
		Message:  "invalid format",
		ExitCode: ExitInput,
	}

	ErrInvalidMethod = &SigilError{
		Code:     "INVALID_METHOD",
		Message:  "invalid method",
		ExitCode: ExitInput,
	}

	ErrInvalidRecipient = &SigilError{
		Code:     "INVALID_RECIPIENT",
		Message:  "invalid recipient",
		ExitCode: ExitInput,
	}

	ErrNoUTXOs = &SigilError{
		Code:     "NO_UTXOS",
		Message:  "no UTXOs available",
		ExitCode: ExitInput,
	}
)

// New creates a new SigilError with the given code and message.
func New(code, message string) *SigilError {
	return &SigilError{
		Code:     code,
		Message:  message,
		ExitCode: ExitGeneral,
	}
}

// Wrap wraps an error with additional context.
func Wrap(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	msg := fmt.Sprintf(format, args...)

	var se *SigilError
	if errors.As(err, &se) {
		return &SigilError{
			Code:       se.Code,
			Message:    fmt.Sprintf("%s: %s", msg, se.Message),
			Details:    se.Details,
			Suggestion: se.Suggestion,
			Cause:      err,
			ExitCode:   se.ExitCode,
		}
	}

	return &SigilError{
		Code:     "GENERAL_ERROR",
		Message:  msg,
		Cause:    err,
		ExitCode: ExitGeneral,
	}
}

// WithDetails adds details to an error.
func WithDetails(err error, details map[string]string) error {
	if err == nil {
		return nil
	}

	var se *SigilError
	if errors.As(err, &se) {
		return &SigilError{
			Code:       se.Code,
			Message:    se.Message,
			Details:    details,
			Suggestion: se.Suggestion,
			Cause:      se.Cause,
			ExitCode:   se.ExitCode,
		}
	}

	return &SigilError{
		Code:     "GENERAL_ERROR",
		Message:  err.Error(),
		Details:  details,
		Cause:    err,
		ExitCode: ExitGeneral,
	}
}

// WithSuggestion adds a suggestion to an error.
func WithSuggestion(err error, suggestion string) error {
	if err == nil {
		return nil
	}

	var se *SigilError
	if errors.As(err, &se) {
		return &SigilError{
			Code:       se.Code,
			Message:    se.Message,
			Details:    se.Details,
			Suggestion: suggestion,
			Cause:      se.Cause,
			ExitCode:   se.ExitCode,
		}
	}

	return &SigilError{
		Code:       "GENERAL_ERROR",
		Message:    err.Error(),
		Suggestion: suggestion,
		Cause:      err,
		ExitCode:   ExitGeneral,
	}
}

// ExitCode returns the appropriate exit code for an error.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var se *SigilError
	if errors.As(err, &se) {
		return se.ExitCode
	}

	return ExitGeneral
}

// Code returns the error code for an error.
func Code(err error) string {
	var se *SigilError
	if errors.As(err, &se) {
		return se.Code
	}
	return "GENERAL_ERROR"
}

// Is wraps errors.Is for convenience.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As wraps errors.As for convenience.
func As(err error, target any) bool {
	return errors.As(err, target)
}
