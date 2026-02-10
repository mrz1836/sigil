package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// formatEmptyWalletList formats empty wallet list based on output format.
func formatEmptyWalletList(w io.Writer, format output.Format) {
	if format == output.FormatJSON {
		_ = writeJSON(w, []string{})
	} else {
		outln(w, "No wallets found.")
		outln(w, "Create one with: sigil wallet create <name>")
	}
}

// formatWalletListJSON outputs wallet names as JSON array.
func formatWalletListJSON(w io.Writer, names []string) {
	_ = writeJSON(w, names)
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
	type addressJSON struct {
		Index   uint32 `json:"index"`
		Address string `json:"address"`
		Path    string `json:"path"`
	}
	type walletJSON struct {
		Name      string                   `json:"name"`
		CreatedAt string                   `json:"created_at"`
		Version   int                      `json:"version"`
		Addresses map[string][]addressJSON `json:"addresses"`
	}

	payload := walletJSON{
		Name:      wlt.Name,
		CreatedAt: wlt.CreatedAt.Format(time.RFC3339),
		Version:   wlt.Version,
		Addresses: make(map[string][]addressJSON, len(wlt.Addresses)),
	}
	for chainID, addresses := range wlt.Addresses {
		chainAddresses := make([]addressJSON, 0, len(addresses))
		for _, addr := range addresses {
			chainAddresses = append(chainAddresses, addressJSON{
				Index:   addr.Index,
				Address: addr.Address,
				Path:    addr.Path,
			})
		}
		payload.Addresses[string(chainID)] = chainAddresses
	}

	_ = writeJSON(cmd.OutOrStdout(), payload)
}

// loadWalletWithSession loads a wallet using cached session if available.
// If no valid session exists, it prompts for password and starts a new session.
// When SIGIL_AGENT_TOKEN is set, uses agent token authentication instead.
//
//nolint:gocognit,gocyclo,nestif // Session/agent handling requires multiple branches
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

	// Agent token authentication (non-interactive)
	if token := os.Getenv(config.EnvAgentToken); token != "" {
		return loadWalletWithAgentToken(name, token, storage, cmd)
	}

	// xpub read-only mode (no seed, no secrets)
	if xpub := os.Getenv(config.EnvAgentXpub); xpub != "" {
		return loadWalletWithXpub(name, xpub, storage, cmd)
	}

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
	password, promptErr := promptPasswordFn("Enter wallet password: ")
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

// loadWalletWithAgentToken authenticates using an agent token from SIGIL_AGENT_TOKEN.
// Finds the matching agent file, decrypts the seed, validates expiry and policy,
// and stores the agent credential in the command context for downstream policy enforcement.
func loadWalletWithAgentToken(name, token string, storage *wallet.FileStorage, cmd *cobra.Command) (*wallet.Wallet, []byte, error) {
	ctx := GetCmdContext(cmd)

	if ctx.AgentStore == nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenInvalid,
			"agent store not initialized",
		)
	}

	// Load and decrypt using the agent token
	seed, cred, err := ctx.AgentStore.LoadByToken(name, token)
	if err != nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenInvalid,
			fmt.Sprintf("agent token does not match any agent for wallet '%s'. "+
				"Create one with: sigil agent create --wallet %s", name, name),
		)
	}

	// Check expiry
	if cred.IsExpired() {
		wallet.ZeroBytes(seed)
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenExpired,
			fmt.Sprintf("agent '%s' expired at %s. Create a new agent with: sigil agent create --wallet %s",
				cred.ID, cred.ExpiresAt.Format(time.RFC3339), name),
		)
	}

	// Load wallet metadata (doesn't require password)
	wlt, loadErr := storage.LoadMetadata(name)
	if loadErr != nil {
		wallet.ZeroBytes(seed)
		return nil, nil, loadErr
	}

	// Store agent credential in context for downstream policy enforcement
	ctx.AgentCred = cred
	ctx.AgentToken = token
	ctx.AgentCounterPath = ctx.AgentStore.CounterPath(name, cred.ID)

	out(cmd.ErrOrStderr(), "[Agent '%s' (%s), expires in %s]\n", cred.Label, cred.ID, formatDuration(cred.TTL()))

	return wlt, seed, nil
}

// loadWalletWithXpub creates a read-only wallet context using an xpub from SIGIL_AGENT_XPUB.
// Returns the wallet metadata with a nil seed — only balance and receive operations are supported.
// Spending operations must detect the nil seed and return ErrAgentXpubWriteDenied.
//
//nolint:unparam // nil seed return is intentional (read-only mode, no private key access)
func loadWalletWithXpub(name, xpub string, storage *wallet.FileStorage, cmd *cobra.Command) (*wallet.Wallet, []byte, error) {
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

	// Load wallet metadata (doesn't require password)
	wlt, loadErr := storage.LoadMetadata(name)
	if loadErr != nil {
		return nil, nil, loadErr
	}

	// Store xpub in context for downstream address derivation
	ctx := GetCmdContext(cmd)
	ctx.AgentXpub = xpub

	out(cmd.ErrOrStderr(), "[xpub read-only mode — spending operations disabled]\n")

	// Return nil seed (read-only mode — no private key access)
	return wlt, nil, nil
}
