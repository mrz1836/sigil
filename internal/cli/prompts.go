package cli

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// promptPassword prompts for a password with hidden input.
// The caller is responsible for zeroing the returned bytes after use.
func promptPassword(prompt string) ([]byte, error) {
	out(os.Stderr, "%s", prompt)

	password, err := term.ReadPassword(syscall.Stdin)
	outln(os.Stderr) // Add newline after hidden input

	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}

	return password, nil
}

// promptNewPassword prompts for a new password with confirmation.
// The caller is responsible for zeroing the returned bytes after use.
func promptNewPassword() ([]byte, error) {
	password, err := promptPassword("Enter encryption password: ")
	if err != nil {
		return nil, err
	}

	if len(password) < 8 {
		wallet.ZeroBytes(password)
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"password must be at least 8 characters",
		)
	}

	confirm, err := promptPassword("Confirm password: ")
	if err != nil {
		wallet.ZeroBytes(password)
		return nil, err
	}
	defer wallet.ZeroBytes(confirm)

	if string(password) != string(confirm) {
		wallet.ZeroBytes(password)
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"passwords do not match",
		)
	}

	return password, nil
}

// promptPassphrase prompts for an optional BIP39 passphrase.
// The caller is responsible for zeroing the returned string's backing data if needed.
func promptPassphrase() (string, error) {
	outln(os.Stderr, "\nBIP39 Passphrase (optional extra security layer):")
	outln(os.Stderr, "WARNING: If you lose this passphrase, you cannot recover your wallet!")

	passphrase, err := promptPassword("Enter passphrase: ")
	if err != nil {
		return "", err
	}

	if len(passphrase) == 0 {
		return "", nil
	}

	confirm, err := promptPassword("Confirm passphrase: ")
	if err != nil {
		wallet.ZeroBytes(passphrase)
		return "", err
	}
	defer wallet.ZeroBytes(confirm)

	if string(passphrase) != string(confirm) {
		wallet.ZeroBytes(passphrase)
		return "", sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"passphrases do not match",
		)
	}

	// Convert to string for BIP39 API - passphrase is less sensitive than password
	result := string(passphrase)
	wallet.ZeroBytes(passphrase)
	return result, nil
}

// promptConfirmation asks user to confirm addresses are correct.
func promptConfirmation() bool {
	out(os.Stderr, "\nDo these addresses match your expected addresses? [y/N]: ")

	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// promptSeedMaterial prompts for seed material interactively.
func promptSeedMaterial() (string, error) {
	outln(os.Stderr, "Enter your seed material (mnemonic phrase, WIF, or hex key):")
	outln(os.Stderr, "For mnemonic, enter all words separated by spaces.")
	outln(os.Stderr)

	// Read from stdin
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		// Try reading a full line for mnemonic
		return promptMnemonicInteractive()
	}
	return input, nil
}

// promptMnemonicInteractive prompts for a multi-word mnemonic.
func promptMnemonicInteractive() (string, error) {
	out(os.Stderr, "Enter mnemonic (all words on one line): ")

	var words []string
	for i := 0; i < 24; i++ {
		var word string
		_, err := fmt.Scan(&word)
		if err != nil {
			break
		}
		words = append(words, word)

		// Check if we have a valid mnemonic
		mnemonic := strings.Join(words, " ")
		if (len(words) == 12 || len(words) == 24) && wallet.ValidateMnemonic(mnemonic) == nil {
			return mnemonic, nil
		}
	}

	if len(words) > 0 {
		return strings.Join(words, " "), nil
	}
	return "", sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, "no input provided")
}
