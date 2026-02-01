package session

import (
	"os"
	"testing"
)

func TestOSKeyring_Integration(t *testing.T) {
	// Skip in CI - keyring tests require a real OS keyring
	if os.Getenv("CI") != "" {
		t.Skip("Skipping keyring integration test in CI")
	}

	keyring := NewOSKeyring()

	// Test Set
	err := keyring.Set("sigil-test", "testuser", "testpass")
	if err != nil {
		t.Skipf("Keyring not available: %v", err)
	}

	// Test Get
	pass, err := keyring.Get("sigil-test", "testuser")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if pass != "testpass" {
		t.Errorf("Get() = %v, want testpass", pass)
	}

	// Test Delete
	err = keyring.Delete("sigil-test", "testuser")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = keyring.Get("sigil-test", "testuser")
	if err == nil {
		t.Error("Expected error after deletion, got nil")
	}
}

func TestProbeKeyring(t *testing.T) {
	// Skip in CI
	if os.Getenv("CI") != "" {
		t.Skip("Skipping keyring probe test in CI")
	}

	// Just test that ProbeKeyring doesn't panic
	result := ProbeKeyring()
	t.Logf("ProbeKeyring() = %v", result)
}

func TestNewOSKeyring(t *testing.T) {
	keyring := NewOSKeyring()
	if keyring == nil {
		t.Error("NewOSKeyring() returned nil")
	}
}
