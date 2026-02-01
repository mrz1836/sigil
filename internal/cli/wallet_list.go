package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// formatEmptyWalletList formats empty wallet list based on output format.
func formatEmptyWalletList(w io.Writer, format output.Format) {
	if format == output.FormatJSON {
		outln(w, "[]")
	} else {
		outln(w, "No wallets found.")
		outln(w, "Create one with: sigil wallet create <name>")
	}
}

// formatWalletListJSON outputs wallet names as JSON array.
func formatWalletListJSON(w io.Writer, names []string) {
	out(w, "[")
	for i, name := range names {
		if i > 0 {
			out(w, ",")
		}
		out(w, `"%s"`, name)
	}
	outln(w, "]")
}

// formatWalletListText outputs wallet names as text list.
func formatWalletListText(w io.Writer, names []string) {
	outln(w, "Wallets:")
	for _, name := range names {
		out(w, "  - %s\n", name)
	}
}

func runWalletList(cmd *cobra.Command, _ []string) error {
	ctx := GetCmdContext(cmd)
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	names, err := storage.List()
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	format := ctx.Fmt.Format()

	if len(names) == 0 {
		formatEmptyWalletList(w, format)
		return nil
	}

	if format == output.FormatJSON {
		formatWalletListJSON(w, names)
	} else {
		formatWalletListText(w, names)
	}

	return nil
}

func runWalletShow(cmd *cobra.Command, args []string) error {
	ctx := GetCmdContext(cmd)
	name := args[0]

	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	// Load wallet (using session if available)
	w, seed, err := loadWalletWithSession(name, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Display wallet info
	if ctx.Fmt.Format() == output.FormatJSON {
		displayWalletJSON(w, cmd)
	} else {
		displayWalletText(w, cmd)
	}

	return nil
}

// displayWalletText shows wallet details in text format.
func displayWalletText(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	out(w, "Wallet: %s\n", wlt.Name)
	out(w, "Created: %s\n", wlt.CreatedAt.Format("2006-01-02 15:04:05"))
	out(w, "Version: %d\n", wlt.Version)
	outln(w)
	outln(w, "Addresses:")
	for chainID, addresses := range wlt.Addresses {
		out(w, "  %s:\n", strings.ToUpper(string(chainID)))
		for _, addr := range addresses {
			out(w, "    [%d] %s\n", addr.Index, addr.Address)
			out(w, "        Path: %s\n", addr.Path)
		}
	}
}

// displayWalletJSON shows wallet details in JSON format.
func displayWalletJSON(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	// Build JSON manually to control format
	outln(w, "{")
	out(w, `  "name": "%s",`+"\n", wlt.Name)
	out(w, `  "created_at": "%s",`+"\n", wlt.CreatedAt.Format("2006-01-02T15:04:05Z"))
	out(w, `  "version": %d,`+"\n", wlt.Version)
	outln(w, `  "addresses": {`)

	chainCount := 0
	for chainID, addresses := range wlt.Addresses {
		if chainCount > 0 {
			outln(w, ",")
		}
		out(w, `    "%s": [`, chainID)
		for i, addr := range addresses {
			if i > 0 {
				out(w, ",")
			}
			out(w, `{"index": %d, "address": "%s", "path": "%s"}`, addr.Index, addr.Address, addr.Path)
		}
		out(w, "]")
		chainCount++
	}
	outln(w)
	outln(w, "  }")
	outln(w, "}")
}

// loadWalletWithSession loads a wallet using cached session if available.
// If no valid session exists, it prompts for password and starts a new session.
//
//nolint:gocognit,gocyclo,nestif // Session handling requires multiple branches
func loadWalletWithSession(name string, storage *wallet.FileStorage, cmd *cobra.Command) (*wallet.Wallet, []byte, error) {
	// Check if wallet exists
	exists, existsErr := storage.Exists(name)
	if existsErr != nil {
		return nil, nil, existsErr
	}
	if !exists {
		return nil, nil, sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", name),
		)
	}

	ctx := GetCmdContext(cmd)
	mgr := ctx.SessionMgr
	cfgProvider := ctx.Cfg
	log := ctx.Log

	// Check if sessions are enabled in config
	sessionEnabled := cfgProvider != nil && cfgProvider.GetSecurity().SessionEnabled

	// Try to use existing session
	if sessionEnabled && mgr != nil && mgr.Available() && mgr.HasValidSession(name) {
		seed, sess, getErr := mgr.GetSession(name)
		if getErr == nil {
			// Load wallet metadata (doesn't require password)
			wlt, loadErr := storage.LoadMetadata(name)
			if loadErr != nil {
				wallet.ZeroBytes(seed)
				return nil, nil, loadErr
			}
			out(cmd.ErrOrStderr(), "[Using cached session, expires in %s]\n", formatDuration(sess.TTL()))
			return wlt, seed, nil
		}
		// Session invalid or error - fall through to password prompt
	}

	// Prompt for password
	password, promptErr := promptPassword("Enter wallet password: ")
	if promptErr != nil {
		return nil, nil, promptErr
	}
	defer wallet.ZeroBytes(password)

	// Load wallet with password
	wlt, seed, loadErr := storage.Load(name, password)
	if loadErr != nil {
		return nil, nil, loadErr
	}

	// Start a new session if sessions are enabled
	if sessionEnabled && mgr != nil && mgr.Available() {
		ttl := time.Duration(cfgProvider.GetSecurity().SessionTTLMinutes) * time.Minute
		if ttl < session.MinTTL {
			ttl = session.DefaultTTL
		}

		if startErr := mgr.StartSession(name, seed, ttl); startErr != nil {
			// Log warning but don't fail - user can still proceed without session
			if log != nil {
				log.Debug("failed to start session: %v", startErr)
			}
		} else {
			out(cmd.ErrOrStderr(), "[Session started, expires in %s]\n", formatDuration(ttl))
		}
	}

	return wlt, seed, nil
}
