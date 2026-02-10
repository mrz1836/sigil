package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/output"
	walletservice "github.com/mrz1836/sigil/internal/service/wallet"
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

// validateWalletExists checks if a wallet exists in storage.
// Returns an error with helpful suggestion if wallet is not found.
func validateWalletExists(name string, storage *wallet.FileStorage) error {
	exists, existsErr := storage.Exists(name)
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", name),
		)
	}
	return nil
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

// loadWalletWithSession loads a wallet using the wallet service.
// Uses cached session if available, otherwise prompts for password.
// When SIGIL_AGENT_TOKEN is set, uses agent token authentication instead.
// When SIGIL_AGENT_XPUB is set, uses xpub read-only mode (no seed).
func loadWalletWithSession(name string, storage *wallet.FileStorage, cmd *cobra.Command) (*wallet.Wallet, []byte, error) {
	ctx := GetCmdContext(cmd)

	// Create wallet service
	walletService := walletservice.NewService(&walletservice.Config{
		Storage:    storage,
		SessionMgr: ctx.SessionMgr,
		Config:     ctx.Cfg,
		Logger:     ctx.Log,
	})

	// Prepare load context
	loadCtx := &walletservice.LoadContext{
		AgentStore: ctx.AgentStore,
		OnAuthMessage: func(msg string) {
			out(cmd.ErrOrStderr(), "%s\n", msg)
		},
		OnSessionInfo: func(info *walletservice.AgentSessionInfo) {
			// Store agent session info in command context for downstream policy enforcement
			if info.Credential != nil {
				ctx.AgentCred = info.Credential
				ctx.AgentToken = info.Token
				ctx.AgentCounterPath = info.CounterPath
			}
			if info.XpubReadOnly {
				ctx.AgentXpub = info.Xpub
			}
		},
	}

	// Load wallet
	result, _, err := walletService.Load(&walletservice.LoadRequest{
		Name: name,
		PasswordFunc: func(prompt string) (string, error) {
			pwd, pwdErr := promptPasswordFn(prompt)
			if pwdErr != nil {
				return "", pwdErr
			}
			return string(pwd), nil
		},
	}, loadCtx)
	if err != nil {
		return nil, nil, err
	}

	return result.Wallet, result.Seed, nil
}
