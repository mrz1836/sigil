package cli

import (
	"context"
	"fmt"
	"path/filepath"

	tumbler "github.com/mrz1836/go-tumbler"
	walletservice "github.com/mrz1836/sigil/internal/service/wallet"
	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/mrz1836/sigil/internal/yubikey"
	"github.com/spf13/cobra"
)

// YubiKey enroll flags.
var (
	enrollYubiKeyPolicy   string
	enrollYubiKeyBackup   bool
	enrollYubiKeyRecovery bool
	enrollYubiKeyForce    bool
)

// maybeYubiKeyStore returns a YubiKey unlocker for a wallet only when the
// wallet actually has an enrolled envelope, so password-only wallets never
// invoke ykman. Returns nil (untyped) when there is no envelope or ykman
// cannot be constructed; the loader then surfaces a clear error for enrolled
// wallets and is unaffected for password-only ones.
func maybeYubiKeyStore(name string, storage *wallet.FileStorage, cmd *cobra.Command) walletservice.YubiKeyUnlocker {
	_, hasEnv, err := storage.LoadAuthPolicy(name)
	if err != nil || !hasEnv {
		return nil
	}
	sec := GetCmdContext(cmd).Cfg.GetSecurity()
	store, err := yubikey.NewStoreFromConfig(sec.YkmanPath, uint8(sec.YubiKeySlot))
	if err != nil {
		return nil
	}
	store.SetTouchPrompt(func() {
		out(cmd.ErrOrStderr(), "\n👆  Touch your YubiKey now — it's blinking...\n")
	})
	return store
}

// parseYubiKeyPolicy maps the --policy flag to a tumbler policy.
func parseYubiKeyPolicy(s string) (tumbler.Policy, error) {
	switch s {
	case "", "password-and-yubikey":
		return tumbler.PolicyPasswordAndYubiKey, nil
	case "yubikey-only":
		return tumbler.PolicyYubiKeyOnly, nil
	default:
		return tumbler.PolicyInvalid, fmt.Errorf(
			"unknown policy %q (use \"password-and-yubikey\" or \"yubikey-only\")", s)
	}
}

// walletEnrollYubiKeyCmd protects an existing password wallet with a YubiKey.
var walletEnrollYubiKeyCmd = &cobra.Command{
	Use:   "enroll-yubikey <wallet>",
	Short: "Protect a wallet with a YubiKey (password+yubikey or yubikey-only)",
	Long: "Enroll a YubiKey as the unlock factor for a wallet. This re-wraps the\n" +
		"wallet seed under a hardware keyslot and removes the standalone-password\n" +
		"copy, so the chosen policy is enforced. Your BIP39 mnemonic remains the\n" +
		"ultimate backup (sigil wallet restore).",
	Example: "  # Two-factor (password + YubiKey), with a printed recovery code\n" +
		"  sigil wallet enroll-yubikey mywallet --policy password-and-yubikey --recovery-code\n\n" +
		"  # YubiKey-only, plus a backup key so a lost key is not a lockout\n" +
		"  sigil wallet enroll-yubikey mywallet --policy yubikey-only --backup",
	Args: cobra.ExactArgs(1),
	RunE: runEnrollYubiKey,
}

//nolint:gocyclo // sequential enroll flow with clear guardrails
func runEnrollYubiKey(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := GetCmdContext(cmd)
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	policy, err := parseYubiKeyPolicy(enrollYubiKeyPolicy)
	if err != nil {
		return err
	}

	// Refuse to double-enroll.
	if _, hasEnv, polErr := storage.LoadAuthPolicy(name); polErr == nil && hasEnv {
		return fmt.Errorf("wallet %q already has a YubiKey envelope; remove it first", name)
	}

	// Safety: yubikey-only with no second factor is a lockout + no-PIN risk.
	if policy == tumbler.PolicyYubiKeyOnly && !enrollYubiKeyBackup && !enrollYubiKeyRecovery && !enrollYubiKeyForce {
		return fmt.Errorf("yubikey-only without a backup key or recovery code is unsafe " +
			"(a lost key means permanent lockout, and the CR path has no PIN); " +
			"re-run with --recovery-code, --backup, or --force")
	}

	// Prompt the current password: it unlocks the seed and, for
	// password-and-yubikey, is the second factor.
	password, err := promptPasswordFn("Enter wallet password: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(password)

	// Recover the seed via the legacy password path.
	_, seed, err := storage.Load(name, password)
	if err != nil {
		return fmt.Errorf("unlocking wallet: %w", err)
	}
	defer wallet.ZeroBytes(seed)

	store, err := yubikey.NewStoreFromConfig(ctx.Cfg.GetSecurity().YkmanPath, uint8(ctx.Cfg.GetSecurity().YubiKeySlot))
	if err != nil {
		return fmt.Errorf("initializing YubiKey (is ykman installed?): %w", err)
	}
	store.SetTouchPrompt(func() {
		out(cmd.ErrOrStderr(), "\n👆  Touch your YubiKey now — it's blinking...\n")
	})

	opts := yubikey.EnrollOptions{WithRecovery: enrollYubiKeyRecovery}
	if policy == tumbler.PolicyPasswordAndYubiKey {
		opts.Password = password
	}

	out(cmd.OutOrStdout(), "Enrolling — you'll be asked to touch your key in a moment...\n")
	res, err := store.Enroll(context.Background(), seed, policy, opts)
	if err != nil {
		return fmt.Errorf("enrolling YubiKey: %w", err)
	}

	// Optionally enroll a backup key (a second physical touch).
	if enrollYubiKeyBackup {
		out(cmd.OutOrStdout(), "Now insert your BACKUP YubiKey and touch it...\n")
		updated, addErr := store.AddBackupKey(context.Background(), res.Envelope, seed, opts.Password, "backup")
		if addErr != nil {
			return fmt.Errorf("enrolling backup key: %w", addErr)
		}
		res.Envelope = updated
	}

	if err = storage.EnrollEnvelope(name, res.Envelope, policy.String()); err != nil {
		return fmt.Errorf("saving envelope: %w", err)
	}

	out(cmd.OutOrStdout(), "\n✓ YubiKey enrolled for wallet %q (policy: %s)\n", name, policy)
	if res.RecoveryCode != "" {
		out(cmd.OutOrStdout(), "\n⚠ RECOVERY CODE — write this down now, it is shown only once:\n\n    %s\n\n", res.RecoveryCode)
	}
	out(cmd.ErrOrStderr(), "Your BIP39 mnemonic remains the ultimate backup (sigil wallet restore).\n")
	return nil
}

// walletRecoveryCodeCmd unlocks and re-displays a wallet using a recovery code
// (the lockout escape hatch), by starting a session so subsequent commands
// work normally.
var walletRecoveryCodeCmd = &cobra.Command{
	Use:   "recovery-code <wallet>",
	Short: "Unlock a YubiKey wallet using its printed recovery code",
	Long: "Unlock a YubiKey-protected wallet with the one-time recovery code that\n" +
		"was printed at enrollment. Use this as the lockout escape hatch when the\n" +
		"enrolled YubiKey is unavailable, then re-enroll a fresh key.",
	Example: "  sigil wallet recovery-code mywallet",
	Args:    cobra.ExactArgs(1),
	RunE:    runRecoveryCode,
}

func runRecoveryCode(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := GetCmdContext(cmd)
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	envelope, _, err := storage.LoadEnvelope(name)
	if err != nil {
		return fmt.Errorf("loading envelope: %w", err)
	}

	code, err := promptPasswordFn("Enter recovery code: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(code)

	store, err := yubikey.NewStoreFromConfig(ctx.Cfg.GetSecurity().YkmanPath, uint8(ctx.Cfg.GetSecurity().YubiKeySlot))
	if err != nil {
		return fmt.Errorf("initializing YubiKey store: %w", err)
	}

	seed, err := store.UnlockWithRecovery(context.Background(), envelope, string(code))
	if err != nil {
		return fmt.Errorf("recovery unlock failed: %w", err)
	}
	defer wallet.ZeroBytes(seed)

	out(cmd.OutOrStdout(), "✓ Recovery code accepted for wallet %q.\n", name)
	out(cmd.ErrOrStderr(), "Consider re-enrolling a fresh YubiKey and rotating the seed if the key was lost.\n")
	return nil
}
