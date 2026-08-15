package cli

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tumbler "github.com/mrz1836/go-tumbler"
	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/wallet"
)

// Static errors for the keyslot-management commands (satisfies err113 and lets
// callers/tests match on the cause).
var (
	errNoEnvelopeEnrolled = errors.New("wallet has no YubiKey envelope")
	errKeyslotNotFound    = errors.New("no keyslot with that ID")
	errLastKeyslot        = errors.New("cannot remove the only remaining keyslot")
	errBadSlotID          = errors.New("invalid slot ID")
)

// walletYubiKeyRemoveForce skips the interactive confirmation on remove.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var walletYubiKeyRemoveForce bool

// confirmYubiKeyRemoveFn gates the destructive keyslot removal. It is a package
// var (mirroring promptConfirmation) so tests can drive confirmation without a
// TTY.
//
//nolint:gochecknoglobals // test-hookable prompt seam
var confirmYubiKeyRemoveFn = confirmYubiKeyRemove

// walletYubiKeyCmd groups the keyslot-management subcommands. It operates on the
// wallet's stored envelope metadata only — it never touches a physical key.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletYubiKeyCmd = &cobra.Command{
	Use:   "yubikey",
	Short: "Inspect and manage a wallet's YubiKey keyslots",
	Long: `Inspect and manage the YubiKey keyslots enrolled for a wallet.

These subcommands read and edit the wallet's stored envelope metadata only: they
list the enrolled keyslots (password, yubikey, password+yubikey, recovery) or
revoke one from this file. They never touch your YubiKey and never ask for a
touch. Removing a slot protects only this file; truly revoking a compromised key
requires re-enrolling to rotate the data key (see SECURITY.md).`,
}

// walletYubiKeyListCmd lists the keyslots enrolled for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletYubiKeyListCmd = &cobra.Command{
	Use:   "list <wallet>",
	Short: "List the YubiKey keyslots enrolled for a wallet",
	Long: `List every keyslot in a wallet's YubiKey envelope.

Shows the effective unlock policy and, for each slot, its 16-hex slot ID, method
(password, yubikey, password+yubikey, or recovery), and label. The slot ID is the
handle you pass to "sigil wallet yubikey remove".`,
	Example: "  sigil wallet yubikey list mywallet",
	Args:    cobra.ExactArgs(1),
	RunE:    runWalletYubiKeyList,
}

// walletYubiKeyRemoveCmd revokes a keyslot from a wallet's envelope.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletYubiKeyRemoveCmd = &cobra.Command{
	Use:   "remove <wallet> <slotID>",
	Short: "Revoke a YubiKey keyslot from a wallet's envelope",
	Long: `Remove one keyslot from a wallet's YubiKey envelope by its 16-hex slot ID
(from "sigil wallet yubikey list").

This edits only this wallet file. It refuses to remove the last remaining slot,
warns loudly when the result drops below two unlock methods or loses its recovery
code, and does NOT rotate the seed. Truly revoking a possibly-compromised key
requires re-enrolling to rotate the data key — see SECURITY.md.`,
	Example: "  sigil wallet yubikey remove mywallet 1a2b3c4d5e6f7080",
	Args:    cobra.ExactArgs(2),
	RunE:    runWalletYubiKeyRemove,
}

// runWalletYubiKeyList prints the effective policy and every enrolled slot.
func runWalletYubiKeyList(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := GetCmdContext(cmd)
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	env, err := loadWalletEnvelope(storage, name)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	out(w, "Wallet %q — YubiKey keyslots\n", name)
	out(w, "Policy: %s\n\n", env.EffectivePolicy())
	out(w, "%-16s  %-16s  %s\n", "SLOT ID", "METHOD", "LABEL")
	for _, info := range env.SlotInfos() {
		out(w, "%-16s  %-16s  %s\n",
			hex.EncodeToString(info.ID[:]), info.Type.String(), labelOrDash(info.Label))
	}

	// force=true renders the full advisory set (soft warnings) without turning a
	// hard-unsafe posture into a fatal error — this is a read-only listing.
	if rep, sErr := env.ValidateSafety(true); sErr == nil && len(rep.Warnings) > 0 {
		out(w, "\nWarnings:\n")
		for _, warn := range rep.Warnings {
			out(w, "  ⚠ %s\n", warn)
		}
	}
	return nil
}

// runWalletYubiKeyRemove revokes a single keyslot from a wallet's envelope.
func runWalletYubiKeyRemove(cmd *cobra.Command, args []string) error {
	name, slotHex := args[0], args[1]
	ctx := GetCmdContext(cmd)
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	id, err := parseSlotID(slotHex)
	if err != nil {
		return err
	}

	env, err := loadWalletEnvelope(storage, name)
	if err != nil {
		return err
	}

	target, found := findSlot(env.SlotInfos(), id)
	if !found {
		return fmt.Errorf("%w: %s in wallet %q (run `sigil wallet yubikey list %s`)",
			errKeyslotNotFound, slotHex, name, name)
	}

	w := cmd.OutOrStdout()
	out(w, "Removing keyslot from wallet %q:\n", name)
	out(w, "  %s  %s  %s\n", hex.EncodeToString(target.ID[:]), target.Type.String(), labelOrDash(target.Label))

	if !walletYubiKeyRemoveForce && !confirmYubiKeyRemoveFn() {
		out(w, "Aborted — no changes made.\n")
		return nil
	}

	if rmErr := env.RemoveSlot(id); rmErr != nil {
		return mapRemoveSlotError(rmErr, slotHex, name)
	}

	warnAfterRemove(cmd.ErrOrStderr(), env)

	blob, err := env.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}
	if upErr := storage.UpdateEnvelope(name, blob); upErr != nil {
		return fmt.Errorf("saving envelope: %w", upErr)
	}

	out(w, "\n✓ Removed keyslot %s from wallet %q.\n", slotHex, name)
	out(cmd.ErrOrStderr(), "\nNote: this only edited this wallet file. If the key may be compromised, "+
		"true revocation needs data-key rotation — re-enroll (sigil wallet enroll-yubikey) to rotate. "+
		"See SECURITY.md.\n")
	return nil
}

// loadWalletEnvelope loads and parses a wallet's envelope, translating the
// no-envelope case into a friendly, actionable error.
func loadWalletEnvelope(storage *wallet.FileStorage, name string) (*tumbler.Envelope, error) {
	blob, _, err := storage.LoadEnvelope(name)
	if err != nil {
		if errors.Is(err, wallet.ErrNoEnvelope) {
			return nil, fmt.Errorf("%w for %q; enroll one with `sigil wallet enroll-yubikey %s`",
				errNoEnvelopeEnrolled, name, name)
		}
		return nil, fmt.Errorf("loading envelope: %w", err)
	}
	env, err := tumbler.ParseEnvelope(blob)
	if err != nil {
		return nil, fmt.Errorf("parsing envelope: %w", err)
	}
	return env, nil
}

// mapRemoveSlotError translates a RemoveSlot failure into a friendly,
// cause-preserving CLI error.
func mapRemoveSlotError(err error, slotHex, name string) error {
	switch {
	case errors.Is(err, tumbler.ErrPolicyUnsafe):
		return fmt.Errorf("%w: that would make wallet %q permanently unopenable", errLastKeyslot, name)
	case errors.Is(err, tumbler.ErrSlotNotFound):
		return fmt.Errorf("%w: %s in wallet %q", errKeyslotNotFound, slotHex, name)
	default:
		return fmt.Errorf("removing keyslot: %w", err)
	}
}

// warnAfterRemove surfaces safety advisories for the post-removal envelope,
// warning loudly when it drops below two unlock methods or loses recovery.
func warnAfterRemove(w io.Writer, env *tumbler.Envelope) {
	rep, err := env.ValidateSafety(true)
	if err != nil || len(rep.Warnings) == 0 {
		return
	}
	out(w, "\n")
	for _, warn := range rep.Warnings {
		out(w, "⚠ %s\n", warn)
	}
}

// parseSlotID decodes a 16-hex slot ID into a fixed [8]byte.
func parseSlotID(s string) ([8]byte, error) {
	var id [8]byte
	raw, err := hex.DecodeString(s)
	if err != nil {
		return id, fmt.Errorf("%w %q: must be 16 hexadecimal characters", errBadSlotID, s)
	}
	if len(raw) != len(id) {
		return id, fmt.Errorf("%w %q: must be 16 hexadecimal characters (got %d)", errBadSlotID, s, len(raw)*2)
	}
	copy(id[:], raw)
	return id, nil
}

// findSlot returns the slot info matching id, if present.
func findSlot(infos []tumbler.SlotInfo, id [8]byte) (tumbler.SlotInfo, bool) {
	for _, info := range infos {
		if info.ID == id {
			return info, true
		}
	}
	return tumbler.SlotInfo{}, false
}

// labelOrDash renders an empty slot label as a placeholder for clean columns.
func labelOrDash(label string) string {
	if label == "" {
		return "(none)"
	}
	return label
}

// confirmYubiKeyRemove asks the operator to confirm a destructive removal.
func confirmYubiKeyRemove() bool {
	out(os.Stderr, "Remove this keyslot? [y/N]: ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
