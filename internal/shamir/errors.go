package shamir

import "errors"

var (
	// ErrThresholdInvalid is returned when k < 2.
	ErrThresholdInvalid = errors.New("threshold k must be at least 2")

	// ErrSharesInsufficient is returned when n < k.
	ErrSharesInsufficient = errors.New("total shares n must be at least k")

	// ErrSharesExceedMax is returned when n > 255.
	ErrSharesExceedMax = errors.New("total shares n cannot exceed 255")

	// ErrSecretEmpty is returned when the secret is empty.
	ErrSecretEmpty = errors.New("secret cannot be empty")

	// ErrNoShares is returned when no shares are provided to Combine.
	ErrNoShares = errors.New("no shares provided")

	// ErrInvalidShareFormat is returned when a share string is malformed.
	ErrInvalidShareFormat = errors.New("invalid share format")

	// ErrUnsupportedVersion is returned when a share has an unknown version.
	ErrUnsupportedVersion = errors.New("unsupported share version")

	// ErrInvalidThreshold is returned when a share has an invalid threshold.
	ErrInvalidThreshold = errors.New("invalid threshold in share")

	// ErrInvalidIndex is returned when a share has an invalid index.
	ErrInvalidIndex = errors.New("invalid index in share")

	// ErrInvalidHex is returned when a share has invalid hex data.
	ErrInvalidHex = errors.New("invalid hex data in share")

	// ErrThresholdMismatch is returned when shares have conflicting thresholds.
	ErrThresholdMismatch = errors.New("shares have conflicting thresholds")

	// ErrLengthMismatch is returned when shares have conflicting lengths.
	ErrLengthMismatch = errors.New("shares have conflicting lengths")

	// ErrNotEnoughShares is returned when fewer than k shares are provided.
	ErrNotEnoughShares = errors.New("insufficient shares")

	// ErrNotEnoughUniqueShares is returned when fewer than k unique shares are provided.
	ErrNotEnoughUniqueShares = errors.New("insufficient unique shares")
)
