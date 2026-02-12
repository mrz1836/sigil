package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/shamir"
)

var (
	// ErrThresholdMin is returned when the threshold < 2.
	ErrThresholdMin = errors.New("threshold must be at least 2")

	// ErrSharesConfig is returned when shares < threshold.
	ErrSharesConfig = errors.New("number of shares must be greater than or equal to threshold")
)

// handleShamirCreation generates and displays Shamir shares.
func handleShamirCreation(mnemonic string, cmd *cobra.Command) error {
	if createThreshold < 2 {
		return ErrThresholdMin
	}
	if createShareCount < createThreshold {
		return ErrSharesConfig
	}

	shares, err := shamir.Split([]byte(mnemonic), createShareCount, createThreshold)
	if err != nil {
		return fmt.Errorf("failed to generate shamir shares: %w", err)
	}

	displayShamirShares(shares, createThreshold, cmd)
	return nil
}
