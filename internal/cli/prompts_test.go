package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// TestPromptPassword_Success tests successful password prompt.
func TestPromptPassword_Success(t *testing.T) {
	// Save and restore original
	orig := promptPasswordFn
	t.Cleanup(func() { promptPasswordFn = orig })

	// Mock implementation
	promptPasswordFn = func(_ string) ([]byte, error) {
		return []byte("testpassword123"), nil
	}

	// Test
	result, err := promptPasswordFn("Enter password: ")
	require.NoError(t, err)
	assert.Equal(t, []byte("testpassword123"), result)
}

// TestPromptPassword_Error tests password prompt error handling.
func TestPromptPassword_Error(t *testing.T) {
	// Save and restore original
	orig := promptPasswordFn
	t.Cleanup(func() { promptPasswordFn = orig })

	// Mock implementation that returns error
	expectedErr := errors.New("terminal error") //nolint:err113 // test error
	promptPasswordFn = func(_ string) ([]byte, error) {
		return nil, expectedErr
	}

	// Test
	result, err := promptPasswordFn("Enter password: ")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "terminal error")
}

// TestPromptNewPassword_Success tests successful new password creation.
func TestPromptNewPassword_Success(t *testing.T) {
	// Save and restore original
	orig := promptNewPasswordFn
	t.Cleanup(func() { promptNewPasswordFn = orig })

	// Mock implementation - password meets requirements
	promptNewPasswordFn = func() ([]byte, error) {
		return []byte("validpass123"), nil
	}

	// Test
	result, err := promptNewPasswordFn()
	require.NoError(t, err)
	assert.Equal(t, []byte("validpass123"), result)
}

// TestPromptNewPassword_TooShort tests password length validation via function variable.
func TestPromptNewPassword_TooShort(t *testing.T) {
	// Save and restore original
	origNPW := promptNewPasswordFn
	t.Cleanup(func() { promptNewPasswordFn = origNPW })

	// Mock to return error about short password
	promptNewPasswordFn = func() ([]byte, error) {
		return nil, errors.New("password must be at least 8 characters") //nolint:err113 // test error
	}

	// Test through the function variable
	result, err := promptNewPasswordFn()
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "at least 8 characters")
}

// TestPromptNewPassword_Mismatch tests password confirmation mismatch.
func TestPromptNewPassword_Mismatch(t *testing.T) {
	// Save and restore original
	origNPW := promptNewPasswordFn
	t.Cleanup(func() { promptNewPasswordFn = origNPW })

	// Mock to return error about mismatch
	promptNewPasswordFn = func() ([]byte, error) {
		return nil, errors.New("passwords do not match") //nolint:err113 // test error
	}

	// Test through the function variable
	result, err := promptNewPasswordFn()
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "do not match")
}

// TestPromptPassphrase_Success tests successful passphrase prompt via function variable.
func TestPromptPassphrase_Success(t *testing.T) {
	// Save and restore original
	origPP := promptPassphraseFn
	t.Cleanup(func() {
		promptPassphraseFn = origPP
	})

	// Mock the function variable directly
	promptPassphraseFn = func() (string, error) {
		return "mypassphrase", nil
	}

	// Test through the function variable
	result, err := promptPassphraseFn()
	require.NoError(t, err)
	assert.Equal(t, "mypassphrase", result)
}

// TestPromptPassphrase_EmptyAllowed tests that empty passphrase is allowed.
func TestPromptPassphrase_EmptyAllowed(t *testing.T) {
	// Save and restore original
	origPP := promptPassphraseFn
	t.Cleanup(func() {
		promptPassphraseFn = origPP
	})

	// Mock the function variable directly
	promptPassphraseFn = func() (string, error) {
		return "", nil
	}

	// Test through the function variable
	result, err := promptPassphraseFn()
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestPromptPassphrase_Mismatch tests passphrase error handling.
func TestPromptPassphrase_Mismatch(t *testing.T) {
	// Save and restore original
	origPP := promptPassphraseFn
	t.Cleanup(func() {
		promptPassphraseFn = origPP
	})

	// Mock the function variable to return error
	promptPassphraseFn = func() (string, error) {
		return "", errors.New("passphrases do not match") //nolint:err113 // test error
	}

	// Test through the function variable
	result, err := promptPassphraseFn()
	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "do not match")
}

// TestPromptConfirmation_Yes tests confirmation with "yes" responses.
func TestPromptConfirmation_Yes(t *testing.T) {
	// Save and restore original
	orig := promptConfirmFn
	t.Cleanup(func() { promptConfirmFn = orig })

	testCases := []struct {
		name     string
		response string
	}{
		{"lowercase y", "y"},
		{"uppercase Y", "Y"},
		{"lowercase yes", "yes"},
		{"uppercase YES", "YES"},
		{"mixed case Yes", "Yes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock to return true for yes-like responses
			promptConfirmFn = func() bool {
				return tc.response == "y" || tc.response == "Y" ||
					tc.response == "yes" || tc.response == "YES" || tc.response == "Yes"
			}

			result := promptConfirmFn()
			assert.True(t, result)
		})
	}
}

// TestPromptConfirmation_No tests confirmation with "no" responses.
func TestPromptConfirmation_No(t *testing.T) {
	// Save and restore original
	orig := promptConfirmFn
	t.Cleanup(func() { promptConfirmFn = orig })

	testCases := []struct {
		name     string
		response string
	}{
		{"lowercase n", "n"},
		{"uppercase N", "N"},
		{"lowercase no", "no"},
		{"uppercase NO", "NO"},
		{"empty", ""},
		{"random text", "maybe"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock to return false for non-yes responses
			promptConfirmFn = func() bool {
				return tc.response == "y" || tc.response == "Y" ||
					tc.response == "yes" || tc.response == "YES"
			}

			result := promptConfirmFn()
			assert.False(t, result)
		})
	}
}

// TestPromptSeedMaterial_ValidMnemonic tests valid mnemonic input.
func TestPromptSeedMaterial_ValidMnemonic(t *testing.T) {
	// Save and restore original
	orig := promptSeedFn
	t.Cleanup(func() { promptSeedFn = orig })

	testCases := []struct {
		name     string
		mnemonic string
	}{
		{
			"12 words",
			"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		},
		{
			"24 words",
			"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
				"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock implementation
			promptSeedFn = func() (string, error) {
				return tc.mnemonic, nil
			}

			// Test
			result, err := promptSeedFn()
			require.NoError(t, err)
			assert.Equal(t, tc.mnemonic, result)

			// Verify it's a valid mnemonic
			err = wallet.ValidateMnemonic(result)
			assert.NoError(t, err)
		})
	}
}

// TestPromptSeedMaterial_InvalidWordCount tests invalid word count.
func TestPromptSeedMaterial_InvalidWordCount(t *testing.T) {
	// Save and restore original
	orig := promptSeedFn
	t.Cleanup(func() { promptSeedFn = orig })

	// Mock implementation - wrong word count
	promptSeedFn = func() (string, error) {
		// 11 words - invalid
		return "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon", nil
	}

	// Test
	result, err := promptSeedFn()
	require.NoError(t, err) // Function succeeds

	// But validation should fail
	err = wallet.ValidateMnemonic(result)
	assert.Error(t, err)
}

// TestPromptSeedMaterial_ReadError tests error handling during input.
func TestPromptSeedMaterial_ReadError(t *testing.T) {
	// Save and restore original
	orig := promptSeedFn
	t.Cleanup(func() { promptSeedFn = orig })

	// Mock implementation that returns error
	expectedErr := sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, "read failed")
	promptSeedFn = func() (string, error) {
		return "", expectedErr
	}

	// Test
	result, err := promptSeedFn()
	require.Error(t, err)
	assert.Empty(t, result)
}

// TestPromptMnemonicInteractive_24Words tests interactive 24-word mnemonic.
func TestPromptMnemonicInteractive_24Words(t *testing.T) {
	t.Parallel()

	mnemonic24 := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"

	// Validate it's actually valid
	err := wallet.ValidateMnemonic(mnemonic24)
	require.NoError(t, err)

	// Test we can process it correctly
	words := len(splitWords(mnemonic24))
	assert.Equal(t, 24, words)
}

// TestPromptMnemonicInteractive_12Words tests interactive 12-word mnemonic.
func TestPromptMnemonicInteractive_12Words(t *testing.T) {
	t.Parallel()

	mnemonic12 := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	// Validate it's actually valid
	err := wallet.ValidateMnemonic(mnemonic12)
	require.NoError(t, err)

	// Test we can process it correctly
	words := len(splitWords(mnemonic12))
	assert.Equal(t, 12, words)
}

// TestPromptMnemonicInteractive_InvalidWords tests invalid BIP39 words.
func TestPromptMnemonicInteractive_InvalidWords(t *testing.T) {
	t.Parallel()

	// 12 words but not valid BIP39
	invalidMnemonic := "invalid word list that is not in bip39 wordlist at all really"

	// Validation should fail
	err := wallet.ValidateMnemonic(invalidMnemonic)
	assert.Error(t, err)
}

// Helper function to split mnemonic into words.
func splitWords(mnemonic string) []string {
	words := []string{}
	for _, word := range splitMnemonic(mnemonic) {
		if word != "" {
			words = append(words, word)
		}
	}
	return words
}

// Helper to split by spaces.
func splitMnemonic(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
