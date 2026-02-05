package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestDisplayDetectedTypos_NoTypos(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// Valid 12-word mnemonic
	displayDetectedTypos("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	assert.Empty(t, buf.String(), "expected no output for valid mnemonic")
}

func TestDisplayDetectedTypos_EmptyMnemonic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	displayDetectedTypos("", cmd)

	assert.Empty(t, buf.String(), "expected no output for empty mnemonic")
}

func TestDisplayDetectedTypos_WithTypo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// "abandn" is a typo of "abandon"
	displayDetectedTypos("abandn abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	output := buf.String()
	assert.NotEmpty(t, output, "expected output for typo")
	assert.Contains(t, output, "Possible typos detected")
	assert.Contains(t, output, "Word 1:")
	assert.Contains(t, output, "abandn")
}

func TestDisplayDetectedTypos_InvalidWord(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// "zzzzzzz" is not close to any BIP39 word
	displayDetectedTypos("zzzzzzz abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	output := buf.String()
	assert.NotEmpty(t, output, "expected output for invalid word")
	assert.Contains(t, output, "Possible typos detected")
	assert.Contains(t, output, "zzzzzzz")
	assert.Contains(t, output, "not a valid BIP39 word")
}

func TestDisplayDetectedTypos_ValidZooMnemonic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// All valid BIP39 words
	displayDetectedTypos("zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong", cmd)

	assert.Empty(t, buf.String(), "expected no output for valid mnemonic")
}

func TestValidateRestoreTarget(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))

	// Create an existing wallet for the "already exists" test case
	w, err := wallet.NewWallet("existing", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("password")))
	wallet.ZeroBytes(seed)

	tests := []struct {
		name    string
		target  string
		wantErr bool
		errIs   error
	}{
		{
			name:    "valid name, wallet doesn't exist",
			target:  "new_wallet",
			wantErr: false,
		},
		{
			name:    "valid name, wallet exists",
			target:  "existing",
			wantErr: true,
			errIs:   wallet.ErrWalletExists,
		},
		{
			name:    "invalid name with dashes",
			target:  "my-wallet",
			wantErr: true,
		},
		{
			name:    "empty name",
			target:  "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRestoreTarget(tc.target, storage)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateWalletWithAddresses(t *testing.T) {
	// Generate a real seed from a mnemonic
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	tests := []struct {
		name       string
		walletName string
		seed       []byte
	}{
		{
			name:       "valid name and seed",
			walletName: "test_restore",
			seed:       seed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := createWalletWithAddresses(tc.walletName, tc.seed)
			require.NoError(t, err)
			require.NotNil(t, w)

			assert.Equal(t, tc.walletName, w.Name)
			assert.Contains(t, w.EnabledChains, wallet.ChainETH)
			assert.Contains(t, w.EnabledChains, wallet.ChainBSV)
			assert.NotEmpty(t, w.Addresses[wallet.ChainETH], "should have ETH addresses")
			assert.NotEmpty(t, w.Addresses[wallet.ChainBSV], "should have BSV addresses")
		})
	}
}

func TestProcessSeedInput_WIF(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// Known valid WIF key
	wif := "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ"
	key, err := processSeedInput(wif, false, cmd)
	require.NoError(t, err)
	assert.Len(t, key, 32, "WIF should decode to 32-byte private key")
}

func TestProcessSeedInput_Hex(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key, err := processSeedInput(hexKey, false, cmd)
	require.NoError(t, err)
	assert.Len(t, key, 32, "hex key should decode to 32-byte private key")
}

func TestProcessSeedInput_Unknown(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	_, err := processSeedInput("random gibberish that is not a valid format", false, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
}

func TestDisplayAddressVerification(t *testing.T) {
	// Generate a real wallet with addresses
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)

	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	wlt, err := createWalletWithAddresses("verify_test", seed)
	require.NoError(t, err)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	displayAddressVerification(wlt, cmd)

	output := buf.String()
	assert.Contains(t, output, "VERIFY YOUR ADDRESSES")

	// Should contain chain labels and addresses
	for chainID, addresses := range wlt.Addresses {
		if len(addresses) > 0 {
			assert.Contains(t, output, strings.ToUpper(string(chainID)))
			assert.Contains(t, output, addresses[0].Address)
		}
	}
}
