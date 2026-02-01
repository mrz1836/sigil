package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/session"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level variables
var (
	// sessionManager is the global session manager.
	sessionManager session.Manager
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
	Use:   "status",
	Short: "Show active sessions and remaining time",
	Long: `Show all active authentication sessions and their remaining time until expiry.

Example:
  sigil session status`,
	RunE: runSessionStatus,
}

// sessionLockCmd ends all sessions.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var sessionLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "End all active sessions immediately",
	Long: `End all active authentication sessions immediately.

Use this when stepping away from your computer to ensure wallet
credentials are not cached.

Example:
  sigil session lock`,
	RunE: runSessionLock,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionStatusCmd)
	sessionCmd.AddCommand(sessionLockCmd)
}

// initSessionManager initializes the session manager.
// Should be called from initGlobals after cfg is set.
func initSessionManager() {
	if cfg == nil {
		return
	}

	sessionsPath := filepath.Join(cfg.Home, "sessions")
	sessionManager = session.NewManager(sessionsPath, nil)
}

// getSessionManager returns the session manager, initializing if needed.
func getSessionManager() session.Manager {
	if sessionManager == nil {
		initSessionManager()
	}
	return sessionManager
}

func runSessionStatus(cmd *cobra.Command, _ []string) error {
	mgr := getSessionManager()

	if !mgr.Available() {
		if formatter.Format() == output.FormatJSON {
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

	if formatter.Format() == output.FormatJSON {
		outputSessionStatusJSON(cmd, sessions)
	} else {
		outputSessionStatusText(cmd, sessions)
	}

	return nil
}

func runSessionLock(cmd *cobra.Command, _ []string) error {
	mgr := getSessionManager()

	if !mgr.Available() {
		if formatter.Format() == output.FormatJSON {
			outln(cmd.OutOrStdout(), `{"available": false, "ended": 0, "message": "Session caching is not available"}`)
		} else {
			outln(cmd.OutOrStdout(), "Session caching is not available (keyring unavailable)")
		}
		return nil
	}

	count := mgr.EndAllSessions()

	if formatter.Format() == output.FormatJSON {
		out(cmd.OutOrStdout(), `{"ended": %d}`+"\n", count)
	} else {
		out(cmd.OutOrStdout(), "Ended %d session(s)\n", count)
	}

	return nil
}

func outputSessionStatusJSON(cmd *cobra.Command, sessions []*session.Session) {
	w := cmd.OutOrStdout()

	outln(w, "{")
	outln(w, `  "available": true,`)
	out(w, `  "sessions": [`)

	for i, s := range sessions {
		if i > 0 {
			out(w, ",")
		}
		outln(w)
		out(w, `    {"wallet": "%s", "expires_in": "%s", "created_at": "%s"}`,
			s.WalletName,
			formatDuration(s.TTL()),
			s.CreatedAt.Format(time.RFC3339),
		)
	}

	if len(sessions) > 0 {
		outln(w)
		outln(w, "  ]")
	} else {
		outln(w, "]")
	}
	outln(w, "}")
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
