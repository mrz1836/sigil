package cli

import "testing"

// withMockPrompts replaces prompt functions for testing and restores on cleanup.
func withMockPrompts(t *testing.T, password []byte, confirm bool) {
	t.Helper()
	origPW := promptPasswordFn
	origNewPW := promptNewPasswordFn
	origConfirm := promptConfirmFn
	origPassphrase := promptPassphraseFn
	origSeed := promptSeedFn
	t.Cleanup(func() {
		promptPasswordFn = origPW
		promptNewPasswordFn = origNewPW
		promptConfirmFn = origConfirm
		promptPassphraseFn = origPassphrase
		promptSeedFn = origSeed
	})
	promptPasswordFn = func(_ string) ([]byte, error) {
		cp := make([]byte, len(password))
		copy(cp, password)
		return cp, nil
	}
	promptNewPasswordFn = func() ([]byte, error) {
		cp := make([]byte, len(password))
		copy(cp, password)
		return cp, nil
	}
	promptConfirmFn = func() bool { return confirm }
	promptPassphraseFn = func() (string, error) {
		return "testpassphrase", nil
	}
	promptSeedFn = func() (string, error) {
		return "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", nil
	}
}
