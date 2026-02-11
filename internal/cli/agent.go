package cli

import (
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Sentinel errors for agent CLI parsing.
var (
	errInvalidDuration = errors.New("invalid duration")
	errInvalidAmount   = errors.New("invalid amount")
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	agentWallet      string
	agentChains      string
	agentMaxPerTx    string
	agentMaxDaily    string
	agentMaxPerTxETH string
	agentMaxDailyETH string
	agentAllowedAddr string
	agentExpires     string
	agentLabel       string
	agentID          string
	agentRevokeAll   bool
)

// agentCmd is the parent command for agent operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent tokens for programmatic wallet access",
	Long: `Create, list, and revoke agent tokens that allow AI agents and bots
to use wallets non-interactively. Agent tokens provide policy-limited
access — spending caps, chain restrictions, address allowlists, and
expiration.

Set SIGIL_AGENT_TOKEN in the agent's environment to authenticate.
Set SIGIL_AGENT_XPUB for read-only balance/receive operations
without any secrets.

Security model:
  - Token encrypts the wallet seed in a separate agent file
  - Spending policy enforced per-transaction and per-day
  - Tokens can be revoked instantly
  - xpub mode has zero spending capability

Environment variables for agents:
  SIGIL_AGENT_TOKEN   Agent token for full wallet access (within policy)
  SIGIL_AGENT_XPUB    Extended public key for read-only operations`,
}

// agentCreateCmd creates a new agent token.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent token with spending policy",
	Long: `Create a new agent token for programmatic wallet access. You will be
prompted for the wallet password once. A random token is generated and
displayed — store it securely, it will not be shown again.

The token is used by setting SIGIL_AGENT_TOKEN in the agent's
environment. The agent then uses normal sigil commands (tx send,
balance show, receive) without password prompts.

Spending limits are enforced per-transaction and per-day. Multiple
agents can be created for the same wallet with different policies.

Amount format: use 'sat' suffix for satoshis (e.g., 50000sat),
decimal BSV (e.g., 0.0005), or 0 for unlimited.
ETH limits can be set explicitly with --max-per-tx-eth and
--max-daily-eth, or left at 0 for unlimited.`,
	Example: `  # BSV-only agent with spending limits
  sigil agent create --wallet main --chains bsv --max-per-tx 50000sat --max-daily 500000sat --expires 30d --label "payment-bot"

  # Multi-chain agent
  sigil agent create --wallet main --chains bsv,eth --max-per-tx 50000sat --max-daily 500000sat --expires 7d --label "trading-bot"

  # Agent restricted to specific addresses
  sigil agent create --wallet main --chains bsv --max-per-tx 100000sat --max-daily 1000000sat --allowed-addrs "1ABC...,1DEF..." --expires 90d --label "payroll"

  # Unlimited (use with caution)
  sigil agent create --wallet main --chains bsv,eth --expires 1d --label "test-bot"`,
	RunE: runAgentCreate,
}

// agentListCmd lists agents for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var agentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List agent tokens for a wallet",
	Long: `List all agent tokens for a wallet with their ID, label, allowed
chains, expiration status, and remaining daily spending limit.
Does not require the wallet password.`,
	Example: `  sigil agent list --wallet main
  sigil agent list --wallet main -o json`,
	RunE: runAgentList,
}

// agentInfoCmd shows detailed agent info.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var agentInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed agent token information",
	Long: `Show full details for a specific agent token including policy,
daily spending status, xpub for read-only access, and creation
metadata. Does not require the wallet password.`,
	Example: `  sigil agent info --wallet main --id agt_7f3a2b
  sigil agent info --wallet main --id agt_7f3a2b -o json`,
	RunE: runAgentInfo,
}

// agentRevokeCmd revokes agent tokens.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var agentRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke agent tokens",
	Long: `Revoke one or all agent tokens for a wallet. Revoked tokens are
immediately deleted and can no longer authenticate. This is
irreversible — a new token must be created to restore access.
Does not require the wallet password.`,
	Example: `  # Revoke a specific agent
  sigil agent revoke --wallet main --id agt_7f3a2b

  # Revoke all agents for a wallet
  sigil agent revoke --wallet main --all`,
	RunE: runAgentRevoke,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	agentCmd.GroupID = "security"
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentRevokeCmd)

	// Create flags
	agentCreateCmd.Flags().StringVar(&agentWallet, "wallet", "", "wallet name (required)")
	agentCreateCmd.Flags().StringVar(&agentChains, "chains", "", "comma-separated chain list: bsv, eth (required)")
	agentCreateCmd.Flags().StringVar(&agentMaxPerTx, "max-per-tx", "0", "max BSV per transaction (e.g., 50000sat or 0.0005)")
	agentCreateCmd.Flags().StringVar(&agentMaxDaily, "max-daily", "0", "max daily BSV spend (e.g., 500000sat or 0.005)")
	agentCreateCmd.Flags().StringVar(&agentMaxPerTxETH, "max-per-tx-eth", "0", "max ETH per transaction (e.g., 0.001)")
	agentCreateCmd.Flags().StringVar(&agentMaxDailyETH, "max-daily-eth", "0", "max daily ETH spend (e.g., 0.01)")
	agentCreateCmd.Flags().StringVar(&agentAllowedAddr, "allowed-addrs", "", "comma-separated address allowlist (empty=any)")
	agentCreateCmd.Flags().StringVar(&agentExpires, "expires", "", "token lifetime: e.g., 1d, 7d, 30d, 90d, 365d (required)")
	agentCreateCmd.Flags().StringVar(&agentLabel, "label", "", "human-readable label for this agent (required)")

	_ = agentCreateCmd.MarkFlagRequired("wallet")
	_ = agentCreateCmd.MarkFlagRequired("chains")
	_ = agentCreateCmd.MarkFlagRequired("expires")
	_ = agentCreateCmd.MarkFlagRequired("label")

	// List flags
	agentListCmd.Flags().StringVar(&agentWallet, "wallet", "", "wallet name (required)")
	_ = agentListCmd.MarkFlagRequired("wallet")

	// Info flags
	agentInfoCmd.Flags().StringVar(&agentWallet, "wallet", "", "wallet name (required)")
	agentInfoCmd.Flags().StringVar(&agentID, "id", "", "agent ID (required, e.g., agt_7f3a2b)")
	_ = agentInfoCmd.MarkFlagRequired("wallet")
	_ = agentInfoCmd.MarkFlagRequired("id")

	// Revoke flags
	agentRevokeCmd.Flags().StringVar(&agentWallet, "wallet", "", "wallet name (required)")
	agentRevokeCmd.Flags().StringVar(&agentID, "id", "", "agent ID to revoke")
	agentRevokeCmd.Flags().BoolVar(&agentRevokeAll, "all", false, "revoke all agents for this wallet")
	_ = agentRevokeCmd.MarkFlagRequired("wallet")

	// Declarative flag constraints for revoke
	agentRevokeCmd.MarkFlagsOneRequired("id", "all")
	agentRevokeCmd.MarkFlagsMutuallyExclusive("id", "all")
}

//nolint:gocognit,gocyclo // Agent creation involves multiple validation and setup steps
func runAgentCreate(cmd *cobra.Command, _ []string) error {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

	// Parse chains
	chains, err := parseChainList(agentChains)
	if err != nil {
		return err
	}

	// Parse expiry duration
	expiry, err := parseDuration(agentExpires)
	if err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid expires format: %s (use e.g., 1d, 7d, 30d, 365d)", agentExpires),
		)
	}

	// Parse spending limits
	maxPerTxSat, err := parseSatAmount(agentMaxPerTx)
	if err != nil {
		return sigilerr.WithSuggestion(sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid --max-per-tx: %s", err))
	}
	maxDailySat, err := parseSatAmount(agentMaxDaily)
	if err != nil {
		return sigilerr.WithSuggestion(sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid --max-daily: %s", err))
	}
	maxPerTxWei := parseWeiAmount(agentMaxPerTxETH)
	maxDailyWei := parseWeiAmount(agentMaxDailyETH)

	// Parse allowed addresses
	var allowedAddrs []string
	if agentAllowedAddr != "" {
		for _, addr := range strings.Split(agentAllowedAddr, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				allowedAddrs = append(allowedAddrs, addr)
			}
		}
	}

	// Load wallet to get seed
	storage := wallet.NewFileStorage(filepath.Join(cc.Cfg.GetHome(), "wallets"))
	password, promptErr := promptPasswordFn("Enter wallet password: ")
	if promptErr != nil {
		return promptErr
	}
	defer wallet.ZeroBytes(password)

	_, seed, loadErr := storage.Load(agentWallet, password)
	if loadErr != nil {
		return loadErr
	}
	defer wallet.ZeroBytes(seed)

	// Generate token
	token, err := agent.GenerateToken()
	if err != nil {
		return fmt.Errorf("generating agent token: %w", err)
	}

	// Build credential
	now := time.Now()
	cred := &agent.Credential{
		ID:         agent.TokenID(token),
		Label:      agentLabel,
		WalletName: agentWallet,
		Chains:     chains,
		Policy: agent.Policy{
			MaxPerTxSat:  maxPerTxSat,
			MaxPerTxWei:  maxPerTxWei,
			MaxDailySat:  maxDailySat,
			MaxDailyWei:  maxDailyWei,
			AllowedAddrs: allowedAddrs,
		},
		CreatedAt: now,
		ExpiresAt: now.Add(expiry),
	}

	// Derive xpubs for allowed chains
	cred.Xpubs = make(map[chain.ID]string, len(chains))
	for _, ch := range chains {
		xpub, xpubErr := wallet.DeriveAccountXpub(seed, ch, 0)
		if xpubErr != nil {
			// Non-fatal: xpub is optional (used for read-only mode)
			if cc.Log != nil {
				cc.Log.Debug("failed to derive xpub for chain %s: %v", ch, xpubErr)
			}
			continue
		}
		cred.Xpubs[ch] = xpub
	}

	// Store credential
	agentStore := agent.NewFileStore(filepath.Join(cc.Cfg.GetHome(), "agents"))
	if err := agentStore.CreateCredential(cred, token, seed); err != nil {
		return fmt.Errorf("storing agent credential: %w", err)
	}

	// Output
	if cc.Fmt.Format() == output.FormatJSON {
		return writeJSON(w, map[string]interface{}{
			"id":         cred.ID,
			"label":      cred.Label,
			"wallet":     cred.WalletName,
			"chains":     cred.Chains,
			"created_at": cred.CreatedAt.Format(time.RFC3339),
			"expires_at": cred.ExpiresAt.Format(time.RFC3339),
			"token":      token,
			"xpubs":      cred.Xpubs,
			"policy":     cred.Policy,
		})
	}

	outln(w)
	out(w, "Agent created for wallet '%s':\n", agentWallet)
	out(w, "  ID:           %s\n", cred.ID)
	out(w, "  Label:        %s\n", cred.Label)
	out(w, "  Chains:       %s\n", formatChainList(chains))
	if maxPerTxSat > 0 {
		out(w, "  Per-tx limit: %d sat\n", maxPerTxSat)
	}
	if maxDailySat > 0 {
		out(w, "  Daily limit:  %d sat\n", maxDailySat)
	}
	if maxPerTxWei != "0" && maxPerTxWei != "" {
		out(w, "  Per-tx ETH:   %s wei\n", maxPerTxWei)
	}
	if maxDailyWei != "0" && maxDailyWei != "" {
		out(w, "  Daily ETH:    %s wei\n", maxDailyWei)
	}
	if len(allowedAddrs) > 0 {
		out(w, "  Allowed:      %s\n", strings.Join(allowedAddrs, ", "))
	}
	out(w, "  Expires:      %s\n", cred.ExpiresAt.Format("2006-01-02 15:04"))
	outln(w)
	outln(w, "Token (store securely, shown once):")
	out(w, "  SIGIL_AGENT_TOKEN=%s\n", token)

	// Display xpubs for read-only mode
	hasXpub := false
	for _, xpub := range cred.Xpubs {
		if xpub != "" {
			hasXpub = true
			break
		}
	}
	if hasXpub {
		outln(w)
		outln(w, "xpub for read-only operations (balance, receive — no spending):")
		for ch, xpub := range cred.Xpubs {
			if xpub != "" {
				out(w, "  %s: %s\n", ch, xpub)
			}
		}
		outln(w)
		outln(w, "Read-only env var (pick one chain):")
		for _, ch := range chains {
			if xpub, ok := cred.Xpubs[ch]; ok && xpub != "" {
				out(w, "  SIGIL_AGENT_XPUB=%s\n", xpub)
				break
			}
		}
	}
	outln(w)

	return nil
}

//nolint:gocognit // Agent list display with format branching requires conditional logic
func runAgentList(cmd *cobra.Command, _ []string) error {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

	agentStore := agent.NewFileStore(filepath.Join(cc.Cfg.GetHome(), "agents"))
	agents, err := agentStore.List(agentWallet)
	if err != nil {
		return err
	}

	if cc.Fmt.Format() == output.FormatJSON {
		type agentJSON struct {
			ID        string     `json:"id"`
			Label     string     `json:"label"`
			Wallet    string     `json:"wallet"`
			Chains    []chain.ID `json:"chains"`
			CreatedAt string     `json:"created_at"`
			ExpiresAt string     `json:"expires_at"`
			Expired   bool       `json:"expired"`
			Policy    struct {
				MaxPerTxSat    uint64   `json:"max_per_tx_sat"`
				MaxDailySat    uint64   `json:"max_daily_sat"`
				DailySpentSat  uint64   `json:"daily_spent_sat"`
				DailyRemainSat uint64   `json:"daily_remaining_sat"`
				MaxPerTxWei    string   `json:"max_per_tx_wei"`
				MaxDailyWei    string   `json:"max_daily_wei"`
				AllowedAddrs   []string `json:"allowed_addrs"`
			} `json:"policy"`
		}

		result := make([]agentJSON, 0, len(agents))
		for _, a := range agents {
			aj := agentJSON{
				ID:        a.ID,
				Label:     a.Label,
				Wallet:    a.WalletName,
				Chains:    a.Chains,
				CreatedAt: a.CreatedAt.Format(time.RFC3339),
				ExpiresAt: a.ExpiresAt.Format(time.RFC3339),
				Expired:   a.IsExpired(),
			}
			aj.Policy.MaxPerTxSat = a.Policy.MaxPerTxSat
			aj.Policy.MaxDailySat = a.Policy.MaxDailySat
			aj.Policy.MaxPerTxWei = a.Policy.MaxPerTxWei
			aj.Policy.MaxDailyWei = a.Policy.MaxDailyWei
			aj.Policy.AllowedAddrs = a.Policy.AllowedAddrs
			if aj.Policy.AllowedAddrs == nil {
				aj.Policy.AllowedAddrs = []string{}
			}

			// Load daily counter (best effort, needs no token for list display)
			counterPath := agentStore.CounterPath(agentWallet, a.ID)
			spentSat, _ := agent.GetDailySpent(counterPath, "")
			aj.Policy.DailySpentSat = spentSat
			if a.Policy.MaxDailySat > spentSat {
				aj.Policy.DailyRemainSat = a.Policy.MaxDailySat - spentSat
			}

			result = append(result, aj)
		}
		return writeJSON(w, map[string]interface{}{"agents": result})
	}

	if len(agents) == 0 {
		out(w, "No agents found for wallet '%s'.\n", agentWallet)
		outln(w, "Create one with: sigil agent create --wallet "+agentWallet+" --chains bsv --expires 30d --label \"my-bot\"")
		return nil
	}

	out(w, "Agents for wallet '%s':\n", agentWallet)
	outln(w)
	for _, a := range agents {
		status := "active"
		if a.IsExpired() {
			status = "EXPIRED"
		}
		out(w, "  %s  %-20s  chains=%-10s  expires=%s  [%s]\n",
			a.ID, a.Label, formatChainList(a.Chains),
			a.ExpiresAt.Format("2006-01-02"), status)
	}

	return nil
}

//nolint:gocognit,gocyclo // Agent info display requires multiple conditional branches
func runAgentInfo(cmd *cobra.Command, _ []string) error {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

	agentStore := agent.NewFileStore(filepath.Join(cc.Cfg.GetHome(), "agents"))
	agents, err := agentStore.List(agentWallet)
	if err != nil {
		return err
	}

	// Find the requested agent
	var found *agent.Credential
	for _, a := range agents {
		if a.ID == agentID {
			found = a
			break
		}
	}
	if found == nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrNotFound,
			fmt.Sprintf("agent '%s' not found for wallet '%s'. List agents with: sigil agent list --wallet %s",
				agentID, agentWallet, agentWallet),
		)
	}

	if cc.Fmt.Format() == output.FormatJSON {
		return writeJSON(w, map[string]interface{}{
			"id":         found.ID,
			"label":      found.Label,
			"wallet":     found.WalletName,
			"chains":     found.Chains,
			"created_at": found.CreatedAt.Format(time.RFC3339),
			"expires_at": found.ExpiresAt.Format(time.RFC3339),
			"expired":    found.IsExpired(),
			"policy":     found.Policy,
			"xpubs":      found.Xpubs,
		})
	}

	status := "active"
	if found.IsExpired() {
		status = "EXPIRED"
	}

	outln(w)
	out(w, "Agent: %s\n", found.ID)
	out(w, "  Label:        %s\n", found.Label)
	out(w, "  Wallet:       %s\n", found.WalletName)
	out(w, "  Chains:       %s\n", formatChainList(found.Chains))
	out(w, "  Status:       %s\n", status)
	out(w, "  Created:      %s\n", found.CreatedAt.Format("2006-01-02 15:04"))
	out(w, "  Expires:      %s\n", found.ExpiresAt.Format("2006-01-02 15:04"))
	outln(w)
	outln(w, "  Policy:")
	if found.Policy.MaxPerTxSat > 0 {
		out(w, "    Per-tx BSV:   %d sat\n", found.Policy.MaxPerTxSat)
	} else {
		outln(w, "    Per-tx BSV:   unlimited")
	}
	if found.Policy.MaxDailySat > 0 {
		out(w, "    Daily BSV:    %d sat\n", found.Policy.MaxDailySat)
	} else {
		outln(w, "    Daily BSV:    unlimited")
	}
	if found.Policy.MaxPerTxWei != "" && found.Policy.MaxPerTxWei != "0" {
		out(w, "    Per-tx ETH:   %s wei\n", found.Policy.MaxPerTxWei)
	} else {
		outln(w, "    Per-tx ETH:   unlimited")
	}
	if found.Policy.MaxDailyWei != "" && found.Policy.MaxDailyWei != "0" {
		out(w, "    Daily ETH:    %s wei\n", found.Policy.MaxDailyWei)
	} else {
		outln(w, "    Daily ETH:    unlimited")
	}
	if len(found.Policy.AllowedAddrs) > 0 {
		outln(w, "    Allowed addresses:")
		for _, addr := range found.Policy.AllowedAddrs {
			out(w, "      - %s\n", addr)
		}
	} else {
		outln(w, "    Allowed addresses: any")
	}

	if len(found.Xpubs) > 0 {
		outln(w)
		outln(w, "  xpubs (read-only):")
		for ch, xpub := range found.Xpubs {
			if xpub != "" {
				out(w, "    %s: %s\n", ch, xpub)
			}
		}
	}

	outln(w)

	return nil
}

func runAgentRevoke(cmd *cobra.Command, _ []string) error { //nolint:gocognit // complexity from error handling paths
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

	// --id/--all one-required and mutual exclusivity is handled
	// by Cobra's MarkFlagsOneRequired/MarkFlagsMutuallyExclusive in init().

	agentStore := agent.NewFileStore(filepath.Join(cc.Cfg.GetHome(), "agents"))

	if agentRevokeAll {
		count, err := agentStore.DeleteAll(agentWallet)
		if err != nil {
			return err
		}
		if cc.Fmt.Format() == output.FormatJSON {
			return writeJSON(w, map[string]interface{}{
				"wallet":  agentWallet,
				"revoked": count,
			})
		}
		out(w, "Revoked %d agent(s) for wallet '%s'.\n", count, agentWallet)
		return nil
	}

	// Check if agent exists before deleting
	agents, err := agentStore.List(agentWallet)
	if err != nil {
		return err
	}
	found := false
	for _, a := range agents {
		if a.ID == agentID {
			found = true
			break
		}
	}
	if !found {
		return sigilerr.WithSuggestion(
			sigilerr.ErrNotFound,
			fmt.Sprintf("agent '%s' not found for wallet '%s'. List agents with: sigil agent list --wallet %s",
				agentID, agentWallet, agentWallet),
		)
	}

	if err := agentStore.Delete(agentWallet, agentID); err != nil {
		return err
	}

	if cc.Fmt.Format() == output.FormatJSON {
		return writeJSON(w, map[string]interface{}{
			"wallet":  agentWallet,
			"id":      agentID,
			"revoked": true,
		})
	}
	out(w, "Agent '%s' revoked for wallet '%s'.\n", agentID, agentWallet)
	return nil
}

// parseChainList parses a comma-separated list of chain IDs.
func parseChainList(s string) ([]chain.ID, error) {
	parts := strings.Split(s, ",")
	var chains []chain.ID
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		id, ok := chain.ParseChainID(p)
		if !ok || !id.IsMVP() {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid chain: %s (supported: bsv, eth)", p),
			)
		}
		chains = append(chains, id)
	}
	if len(chains) == 0 {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"at least one chain required (e.g., --chains bsv or --chains bsv,eth)",
		)
	}
	return chains, nil
}

// formatChainList formats chain IDs as a comma-separated string.
func formatChainList(chains []chain.ID) string {
	parts := make([]string, len(chains))
	for i, ch := range chains {
		parts[i] = string(ch)
	}
	return strings.Join(parts, ", ")
}

// parseDuration parses a duration string like "1d", "7d", "30d", "365d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("%w: %s", errInvalidDuration, s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "h") {
		hours, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil || hours <= 0 {
			return 0, fmt.Errorf("%w: %s", errInvalidDuration, s)
		}
		return time.Duration(hours) * time.Hour, nil
	}
	return 0, fmt.Errorf("%w: %s (use e.g., 1d, 7d, 30d)", errInvalidDuration, s)
}

// parseSatAmount parses a satoshi amount string.
// Accepts: "50000sat", "50000", "0.0005" (as BSV decimal), "0" for unlimited.
//
//nolint:gocognit // Amount parsing with multiple format support requires branching
func parseSatAmount(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	// Handle "sat" suffix
	if strings.HasSuffix(strings.ToLower(s), "sat") {
		numStr := strings.TrimSuffix(strings.ToLower(s), "sat")
		v, err := strconv.ParseUint(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s", errInvalidAmount, s)
		}
		return v, nil
	}

	// Try as integer (satoshis)
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v, nil
	}

	// Try as decimal BSV (e.g., 0.0005 = 50000 sat)
	if strings.Contains(s, ".") {
		parts := strings.SplitN(s, ".", 2)
		whole, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s", errInvalidAmount, s)
		}
		fracStr := parts[1]
		// Pad or truncate to 8 decimal places (1 BSV = 100,000,000 sat)
		const bsvDecimals = 8
		for len(fracStr) < bsvDecimals {
			fracStr += "0"
		}
		fracStr = fracStr[:bsvDecimals]
		frac, err := strconv.ParseUint(fracStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s", errInvalidAmount, s)
		}
		return whole*100_000_000 + frac, nil
	}

	return 0, fmt.Errorf("%w: %s", errInvalidAmount, s)
}

// parseWeiAmount parses an ETH amount as a wei string.
// Accepts: "0" for unlimited, decimal ETH (e.g., "0.001" → wei string).
func parseWeiAmount(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return "0"
	}

	// If it looks like a decimal ETH amount, convert to wei
	if strings.Contains(s, ".") {
		parts := strings.SplitN(s, ".", 2)
		whole := parts[0]
		frac := parts[1]
		// 1 ETH = 10^18 wei
		const ethDecimals = 18
		for len(frac) < ethDecimals {
			frac += "0"
		}
		frac = frac[:ethDecimals]

		wholeBig := new(big.Int)
		wholeBig.SetString(whole, 10)
		wholeBig.Mul(wholeBig, new(big.Int).Exp(big.NewInt(10), big.NewInt(ethDecimals), nil))

		fracBig := new(big.Int)
		fracBig.SetString(frac, 10)

		total := new(big.Int).Add(wholeBig, fracBig)
		return total.String()
	}

	// Assume it's already wei
	return s
}
