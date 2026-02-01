package session

import (
	"github.com/zalando/go-keyring"
)

// OSKeyring implements the Keyring interface using the OS keychain.
type OSKeyring struct{}

// NewOSKeyring creates a new OS keyring wrapper.
func NewOSKeyring() *OSKeyring {
	return &OSKeyring{}
}

// Set stores a secret in the OS keyring.
func (k *OSKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

// Get retrieves a secret from the OS keyring.
func (k *OSKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

// Delete removes a secret from the OS keyring.
func (k *OSKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// ProbeKeyring tests if the OS keyring is available.
// It attempts to set, get, and delete a test value.
func ProbeKeyring() bool {
	const (
		testService = "sigil-probe"
		testUser    = "probe"
		testValue   = "test"
	)

	// Try to set a test value
	if err := keyring.Set(testService, testUser, testValue); err != nil {
		return false
	}

	// Try to get the test value
	val, err := keyring.Get(testService, testUser)
	if err != nil || val != testValue {
		// Clean up on failure
		_ = keyring.Delete(testService, testUser)
		return false
	}

	// Clean up the test value
	if err := keyring.Delete(testService, testUser); err != nil {
		return false
	}

	return true
}
