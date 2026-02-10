package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/session"
)

// sessionCmd is the parent command for session operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage authentication sessions",
	Long: `Manage authentication sessions for wallet operations.

When enabled, sigil caches your wallet credentials for a configurable time
(default: 15 minutes) so you don't need to enter your password for every command.

Sessions use your operating system's secure keychain:
- macOS: Keychain
- Linux: Secret Service (GNOME Keyring, KWallet)
- Windows: Credential Manager

If the system keychain is unavailable, sessions are disabled and you'll be
prompted for your password each time.`,
}

// sessionStatusCmd shows active sessions.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var sessionStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show active sessions and remaining time",
	Long:    `Show all active authentication sessions and their remaining time until expiry.`,
	Example: `  sigil session status`,
	RunE:    runSessionStatus,
}

// sessionLockCmd ends all sessions.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var sessionLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "End all active sessions immediately",
	Long: `End all active authentication sessions immediately.

Use this when stepping away from your computer to ensure wallet
credentials are not cached.`,
	Example: `  sigil session lock`,
	RunE:    runSessionLock,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	sessionCmd.GroupID = "security"
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionStatusCmd)
	sessionCmd.AddCommand(sessionLockCmd)
}

func runSessionStatus(cmd *cobra.Command, _ []string) error {
	ctx := GetCmdContext(cmd)
	mgr := ctx.SessionMgr
	fmtr := ctx.Fmt

	if mgr == nil || !mgr.Available() {
		if fmtr.Format() == output.FormatJSON {
			outln(cmd.OutOrStdout(), `{"available": false, "message": "Session caching is not available (keyring unavailable)"}`)
		} else {
			outln(cmd.OutOrStdout(), "Session caching is not available (keyring unavailable)")
		}
		return nil
	}

	sessions, err := mgr.ListSessions()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if fmtr.Format() == output.FormatJSON {
		outputSessionStatusJSON(cmd, sessions)
	} else {
		outputSessionStatusText(cmd, sessions)
	}

	return nil
}

func runSessionLock(cmd *cobra.Command, _ []string) error {
	ctx := GetCmdContext(cmd)
	mgr := ctx.SessionMgr
	fmtr := ctx.Fmt

	if mgr == nil || !mgr.Available() {
		if fmtr.Format() == output.FormatJSON {
			outln(cmd.OutOrStdout(), `{"available": false, "ended": 0, "message": "Session caching is not available"}`)
		} else {
			outln(cmd.OutOrStdout(), "Session caching is not available (keyring unavailable)")
		}
		return nil
	}

	count := mgr.EndAllSessions()

	if fmtr.Format() == output.FormatJSON {
		out(cmd.OutOrStdout(), `{"ended": %d}`+"\n", count)
	} else {
		out(cmd.OutOrStdout(), "Ended %d session(s)\n", count)
	}

	return nil
}

func outputSessionStatusJSON(cmd *cobra.Command, sessions []*session.Session) {
	type sessionEntry struct {
		Wallet    string `json:"wallet"`
		ExpiresIn string `json:"expires_in"`
		CreatedAt string `json:"created_at"`
	}

	entries := make([]sessionEntry, len(sessions))
	for i, s := range sessions {
		entries[i] = sessionEntry{
			Wallet:    s.WalletName,
			ExpiresIn: formatDuration(s.TTL()),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		}
	}

	payload := struct {
		Available bool           `json:"available"`
		Sessions  []sessionEntry `json:"sessions"`
	}{
		Available: true,
		Sessions:  entries,
	}

	_ = writeJSON(cmd.OutOrStdout(), payload)
}

func outputSessionStatusText(cmd *cobra.Command, sessions []*session.Session) {
	w := cmd.OutOrStdout()

	if len(sessions) == 0 {
		outln(w, "No active sessions")
		return
	}

	outln(w, "Active Sessions:")
	for _, s := range sessions {
		out(w, "  %s: expires in %s\n", s.WalletName, formatDuration(s.TTL()))
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
