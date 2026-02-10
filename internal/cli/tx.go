package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/transaction"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// txWallet is the wallet name for transactions.
	txWallet string
	// txTo is the recipient address.
	txTo string
	// txAmount is the amount to send.
	txAmount string
	// txChain is the blockchain to use.
	txChain string
	// txToken is the ERC-20 token to transfer (e.g., "USDC").
	txToken string
	// txGasSpeed is the gas speed preference (slow/medium/fast).
	txGasSpeed string
	// txConfirm skips confirmation prompt if false.
	txConfirm bool
)

// txCmd is the parent command for transaction operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Manage transactions",
	Long: `Send cryptocurrency transactions across supported chains.

Supports native ETH, ERC-20 tokens (USDC), and BSV.
Use --amount all to sweep the entire balance.`,
}

// txSendCmd sends a transaction.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var txSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a transaction",
	Long: `Send ETH, USDC, or BSV to an address.

For Ethereum transactions, you can send native ETH or ERC-20 tokens like USDC.
For BSV transactions, only native BSV is supported.

Use --amount all to send the entire balance (fees are deducted automatically).`,
	Example: `  # Send ETH
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

  # Send all ETH
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount all --chain eth

  # Send USDC
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 100 --chain eth --token USDC

  # Send BSV
  sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv

  # Send all BSV
  sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount all --chain bsv`,
	RunE: runTxSend,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	txCmd.GroupID = "wallet"
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(txSendCmd)

	txSendCmd.Flags().StringVar(&txWallet, "wallet", "", "wallet name (required)")
	txSendCmd.Flags().StringVar(&txTo, "to", "", "recipient address (required)")
	txSendCmd.Flags().StringVar(&txAmount, "amount", "", "amount to send, or 'all' for entire balance (required)")
	txSendCmd.Flags().StringVar(&txChain, "chain", "eth", "blockchain: eth, bsv")
	txSendCmd.Flags().StringVar(&txToken, "token", "", "ERC-20 token symbol (e.g., USDC) - ETH only")
	txSendCmd.Flags().StringVar(&txGasSpeed, "gas", "medium", "gas speed: slow, medium, fast")
	txSendCmd.Flags().BoolVar(&txConfirm, "yes", false, "skip confirmation prompt")

	_ = txSendCmd.MarkFlagRequired("wallet")
	_ = txSendCmd.MarkFlagRequired("to")
	_ = txSendCmd.MarkFlagRequired("amount")
}

//nolint:gocyclo,gocognit // CLI flow involves validation and routing
func runTxSend(cmd *cobra.Command, _ []string) error {
	cc := GetCmdContext(cmd)
	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	// Validate chain
	chainID, ok := chain.ParseChainID(txChain)
	if !ok || !chainID.IsMVP() {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid chain: %s (use eth or bsv)", txChain),
		)
	}

	// Token validation
	if txToken != "" && chainID != chain.ETH {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--token flag is only supported for ETH chain",
		)
	}

	// Load wallet and get private key (using session if available)
	storage := wallet.NewFileStorage(filepath.Join(cc.Cfg.GetHome(), "wallets"))
	wlt, seed, err := loadWalletWithSession(txWallet, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// xpub read-only mode: deny spending operations
	if cc.AgentXpub != "" {
		return sigilerr.WithSuggestion(
			sigilerr.ErrAgentXpubWriteDenied,
			"SIGIL_AGENT_XPUB provides read-only access. Use SIGIL_AGENT_TOKEN for spending operations",
		)
	}

	// Get the addresses for this chain
	addresses, ok := wlt.Addresses[chainID]
	if !ok || len(addresses) == 0 {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("wallet '%s' has no addresses for chain %s", txWallet, chainID),
		)
	}

	// Agent mode: enforce chain authorization
	if cc.AgentCred != nil {
		if !cc.AgentCred.HasChain(chainID) {
			return sigilerr.WithSuggestion(
				sigilerr.ErrAgentChainDenied,
				fmt.Sprintf("agent '%s' is not authorized for chain %s (allowed: %v)",
					cc.AgentCred.ID, chainID, cc.AgentCred.Chains),
			)
		}
	}

	// Agent mode: skip confirmation prompt (non-interactive)
	if cc.AgentCred != nil {
		txConfirm = true
	}

	// Execute transaction via service
	return runTxSendWithService(ctx, cmd, chainID, wlt, addresses, seed, storage)
}

// runTxSendWithService executes a transaction using the transaction service.
func runTxSendWithService(ctx context.Context, cmd *cobra.Command, chainID chain.ID, _ *wallet.Wallet, addresses []wallet.Address, seed []byte, storage *wallet.FileStorage) error {
	cc := GetCmdContext(cmd)

	// Create transaction service
	txService := cc.TransactionService
	if txService == nil {
		// Initialize on-demand if not set
		txService = transaction.NewService(&transaction.Config{
			Config:  cc.Cfg,
			Storage: storage,
			Logger:  cc.Log,
		})
	}

	// Build send request
	req := &transaction.SendRequest{
		ChainID:     chainID,
		To:          txTo,
		AmountStr:   txAmount,
		Wallet:      txWallet,
		FromAddress: addresses[0].Address,
		Token:       txToken,
		GasSpeed:    txGasSpeed,
		Addresses:   addresses, // For BSV multi-address
		Confirm:     txConfirm,
		Seed:        seed,
	}

	// Set agent fields if in agent mode
	if cc.AgentCred != nil {
		req.AgentCredID = cc.AgentCred.ID
		req.AgentToken = cc.AgentToken
		req.AgentCounterPath = cc.AgentCounterPath
	}

	// Send transaction
	result, err := txService.Send(ctx, req)
	if err != nil {
		return err
	}

	// Display result
	if chainID == chain.BSV {
		displayBSVTxResult(cmd, convertToBSVTransactionResult(result))
	} else {
		displayTxResult(cmd, convertToETHTransactionResult(result))
	}

	return nil
}

// convertToETHTransactionResult converts service result to chain.TransactionResult for display.
func convertToETHTransactionResult(result *transaction.SendResult) *chain.TransactionResult {
	return &chain.TransactionResult{
		Hash:     result.Hash,
		From:     result.From,
		To:       result.To,
		Amount:   result.Amount,
		Fee:      result.Fee,
		Token:    result.Token,
		Status:   result.Status,
		GasUsed:  result.GasUsed,
		GasPrice: result.GasPrice,
	}
}

// convertToBSVTransactionResult converts service result to chain.TransactionResult for display.
func convertToBSVTransactionResult(result *transaction.SendResult) *chain.TransactionResult {
	return &chain.TransactionResult{
		Hash:   result.Hash,
		From:   result.From,
		To:     result.To,
		Amount: result.Amount,
		Fee:    result.Fee,
		Status: result.Status,
	}
}

// runETHSend has been replaced by runTxSendWithService
// runBSVSend has been replaced by runTxSendWithService

// The following functions have been migrated to internal/service/transaction/:
//   - enforceAgentPolicy
//   - recordAgentSpend
//   - resolveToken
//   - isAmountAll, SanitizeAmount, parseDecimalAmount
//   - checkETHBalance
//   - invalidateBalanceCache, buildPostSendEntry
//   - aggregateBSVUTXOs
//   - deriveKeysForUTXOs, deriveKeyForAddress, zeroKeyMap
//   - uniqueUTXOAddrs, filterSpentBSVUTXOs, markSpentBSVUTXOs

// Display functions retained in CLI (unchanged):

// displayBSVTxDetails shows BSV transaction details before confirmation.
func displayBSVTxDetails(cmd *cobra.Command, from, to, amount string, fee, feeRate uint64) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    TRANSACTION DETAILS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	out(w, "  From:      %s\n", from)
	out(w, "  To:        %s\n", to)
	out(w, "  Amount:    %s BSV\n", amount)
	out(w, "  Fee Rate:  %d sat/KB\n", feeRate)
	out(w, "  Est. Fee:  %d satoshis\n", fee)

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
}

// displayBSVTxResult shows the BSV transaction result.
func displayBSVTxResult(cmd *cobra.Command, result *chain.TransactionResult) {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()
	format := cc.Fmt.Format()

	if format == output.FormatJSON {
		displayBSVTxResultJSON(w, result)
	} else {
		displayBSVTxResultText(w, result)
	}
}

// displayBSVTxResultText shows BSV transaction result in text format.
func displayBSVTxResultText(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	outln(w, "\nTransaction broadcast successfully!")
	outln(w)
	out(w, "  Hash:   %s\n", result.Hash)
	out(w, "  Status: %s\n", result.Status)
	out(w, "  Amount: %s BSV\n", result.Amount)
	out(w, "  Fee:    %s BSV\n", result.Fee)
	outln(w)
	outln(w, "Track your transaction on WhatsOnChain:")
	out(w, "  https://whatsonchain.com/tx/%s\n", result.Hash)
}

// displayBSVTxResultJSON shows BSV transaction result in JSON format.
func displayBSVTxResultJSON(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	payload := struct {
		Hash   string `json:"hash"`
		From   string `json:"from"`
		To     string `json:"to"`
		Amount string `json:"amount"`
		Fee    string `json:"fee"`
		Status string `json:"status"`
	}{
		Hash:   result.Hash,
		From:   result.From,
		To:     result.To,
		Amount: result.Amount,
		Fee:    result.Fee,
		Status: result.Status,
	}

	_ = writeJSON(w, payload)
}

// resolveToken resolves a token symbol to its contract address and decimals.
func resolveToken(symbol string) (address string, decimals int, err error) {
	switch strings.ToUpper(symbol) {
	case "USDC":
		return eth.USDCMainnet, eth.USDCDecimals, nil
	default:
		return "", 0, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("unsupported token: %s (only USDC is supported)", symbol),
		)
	}
}

// amountAll is the special value for sending the entire balance.
const amountAll = "all"

// isAmountAll returns true if the user specified "all" as the amount.
func isAmountAll(amount string) bool {
	return strings.EqualFold(strings.TrimSpace(amount), amountAll)
}

// SanitizeAmount trims whitespace from the amount string without altering content.
// Amount parsing performs strict validation and rejects non-numeric characters.
func SanitizeAmount(amount string) string {
	return strings.TrimSpace(amount)
}

// parseDecimalAmount parses a decimal string to big.Int with specified decimals.
func parseDecimalAmount(amount string, decimals int) (*big.Int, error) {
	amount = SanitizeAmount(amount)
	if amount == "" {
		return nil, sigilerr.ErrAmountRequired
	}

	if amount == "." {
		return nil, sigilerr.WithDetails(
			sigilerr.ErrInvalidAmount,
			map[string]string{"amount": amount},
		)
	}

	dotSeen := false
	for _, c := range amount {
		switch {
		case c == '.':
			if dotSeen {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInvalidAmount,
					map[string]string{"amount": amount},
				)
			}
			dotSeen = true
		case c < '0' || c > '9':
			return nil, sigilerr.WithDetails(
				sigilerr.ErrInvalidAmount,
				map[string]string{"amount": amount},
			)
		}
	}

	return chain.ParseDecimalAmount(amount, decimals, sigilerr.ErrInvalidAmount)
}

// displayTxDetails shows transaction details before confirmation.
func displayTxDetails(cmd *cobra.Command, from, to, amount, token string, estimate *eth.GasEstimate) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    TRANSACTION DETAILS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	out(w, "  From:      %s\n", from)
	out(w, "  To:        %s\n", to)

	if token != "" {
		out(w, "  Amount:    %s %s\n", amount, token)
	} else {
		out(w, "  Amount:    %s ETH\n", amount)
	}

	out(w, "  Gas Limit: %d\n", estimate.GasLimit)
	out(w, "  Gas Price: %s\n", eth.FormatGasPrice(estimate.GasPrice))
	out(w, "  Est. Fee:  %s ETH\n", chain.FormatDecimalAmount(estimate.Total, 18))

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
}

// displayTxResult shows the transaction result.
func displayTxResult(cmd *cobra.Command, result *chain.TransactionResult) {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()
	format := cc.Fmt.Format()

	if format == output.FormatJSON {
		displayTxResultJSON(w, result)
	} else {
		displayTxResultText(w, result)
	}
}

// displayTxResultText shows transaction result in text format.
func displayTxResultText(w io.Writer, result *chain.TransactionResult) {
	outln(w, "\nTransaction broadcast successfully!")
	outln(w)
	out(w, "  Hash:   %s\n", result.Hash)
	out(w, "  Status: %s\n", result.Status)

	if result.Token != "" {
		out(w, "  Amount: %s %s\n", result.Amount, result.Token)
	} else {
		out(w, "  Amount: %s ETH\n", result.Amount)
	}

	out(w, "  Fee:    %s\n", result.Fee)
	outln(w)
	outln(w, "Track your transaction on Etherscan:")
	out(w, "  https://etherscan.io/tx/%s\n", result.Hash)
}

// displayTxResultJSON shows transaction result in JSON format.
func displayTxResultJSON(w io.Writer, result *chain.TransactionResult) {
	payload := struct {
		Hash     string `json:"hash"`
		From     string `json:"from"`
		To       string `json:"to"`
		Amount   string `json:"amount"`
		Token    string `json:"token,omitempty"`
		Fee      string `json:"fee"`
		GasUsed  uint64 `json:"gas_used"`
		GasPrice string `json:"gas_price"`
		Status   string `json:"status"`
	}{
		Hash:     result.Hash,
		From:     result.From,
		To:       result.To,
		Amount:   result.Amount,
		Token:    result.Token,
		Fee:      result.Fee,
		GasUsed:  result.GasUsed,
		GasPrice: result.GasPrice,
		Status:   result.Status,
	}

	_ = writeJSON(w, payload)
}

// invalidateBalanceCache updates the on-disk balance cache after a successful
// transaction broadcast. If expectedBalance is non-empty, the cached entry is
// updated with that value (e.g., "0.0" for sweep-all). Otherwise the entry is
// deleted, forcing the next balance query to fetch from the network.
// Errors are logged but never returned — cache invalidation is best-effort.
func invalidateBalanceCache(cc *CommandContext, chainID chain.ID, address, token, expectedBalance string) {
	cachePath := filepath.Join(cc.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)

	balanceCache, err := cacheStorage.Load()
	if err != nil {
		logCacheError(cc, "failed to load balance cache for post-send update: %v", err)
		return
	}

	if expectedBalance == "" {
		// Unknown expected balance — delete to force a fresh network fetch.
		balanceCache.Delete(chainID, address, token)
	} else {
		// Known expected balance (e.g., sweep-all → "0.0").
		// Preserve symbol/decimals from the existing entry if available.
		entry := buildPostSendEntry(balanceCache, chainID, address, token, expectedBalance)
		balanceCache.Set(entry)
	}

	if err := cacheStorage.Save(balanceCache); err != nil {
		logCacheError(cc, "failed to save balance cache after send: %v", err)
	}
}

// buildPostSendEntry creates a cache entry with the expected post-send balance,
// preserving symbol and decimals from any existing entry.
func buildPostSendEntry(bc *cache.BalanceCache, chainID chain.ID, address, token, balance string) cache.BalanceCacheEntry {
	if existing, exists, _ := bc.Get(chainID, address, token); exists {
		existing.Balance = balance
		existing.Unconfirmed = "" // Clear stale unconfirmed data after send
		return *existing
	}
	return cache.BalanceCacheEntry{
		Chain:   chainID,
		Address: address,
		Token:   token,
		Balance: balance,
	}
}

func logCacheError(cc *CommandContext, format string, args ...any) {
	if cc.Log != nil {
		cc.Log.Error(format, args...)
	}
}

func logTxError(cc *CommandContext, format string, args ...any) {
	if cc.Log != nil {
		cc.Log.Error(format, args...)
	}
}

// errAddressNotInWallet indicates a UTXO references an address not found in the wallet.
var errAddressNotInWallet = errors.New("address not found in wallet")

// deriveKeysForUTXOs derives private keys for each unique address that appears in the UTXO set.
// Returns a map of address → private key. The caller must zero all keys after use.
func deriveKeysForUTXOs(utxos []chain.UTXO, addresses []wallet.Address, seed []byte) (map[string][]byte, error) {
	// Build address → index lookup
	addrIndex := make(map[string]uint32, len(addresses))
	for _, addr := range addresses {
		addrIndex[addr.Address] = addr.Index
	}

	// Collect unique addresses from UTXOs
	needed := uniqueUTXOAddrs(utxos)

	// Derive private key for each unique address
	keys := make(map[string][]byte, len(needed))
	for addr := range needed {
		key, err := deriveKeyForAddress(addr, addrIndex, seed)
		if err != nil {
			zeroKeyMap(keys)
			return nil, err
		}
		keys[addr] = key
	}

	return keys, nil
}

// deriveKeyForAddress derives a private key for a single address using the index lookup.
func deriveKeyForAddress(addr string, addrIndex map[string]uint32, seed []byte) ([]byte, error) {
	index, ok := addrIndex[addr]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errAddressNotInWallet, addr)
	}
	privKey, err := wallet.DerivePrivateKeyForChain(seed, wallet.ChainBSV, index)
	if err != nil {
		return nil, fmt.Errorf("deriving key for address %s (index %d): %w", addr, index, err)
	}
	return privKey, nil
}

// zeroKeyMap zeros all private keys in the map.
func zeroKeyMap(keys map[string][]byte) {
	for _, k := range keys {
		wallet.ZeroBytes(k)
	}
}

// uniqueUTXOAddrs returns the unique set of addresses that appear in a UTXO slice.
func uniqueUTXOAddrs(utxos []chain.UTXO) map[string]struct{} {
	addrs := make(map[string]struct{})
	for _, u := range utxos {
		addrs[u.Address] = struct{}{}
	}
	return addrs
}

// filterSpentBSVUTXOs removes UTXOs that are marked as spent in the local store.
// UTXOs not present in the store are kept (unknown is not known-spent).
func filterSpentBSVUTXOs(utxos []chain.UTXO, store *utxostore.Store) []chain.UTXO {
	filtered := make([]chain.UTXO, 0, len(utxos))
	for _, u := range utxos {
		if !store.IsSpent(chain.BSV, u.TxID, u.Vout) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// markSpentBSVUTXOs records spent UTXOs in the local store after a successful broadcast.
// Errors are logged but never returned — the broadcast already succeeded.
// This function has been migrated to internal/service/transaction/ for production use.
// This version is retained for legacy test compatibility only.
func markSpentBSVUTXOs(cc *CommandContext, store *utxostore.Store, utxos []chain.UTXO, _ string) {
	if store == nil {
		return
	}

	const spentTxID = "broadcast-txid"
	for _, u := range utxos {
		// Ensure the UTXO exists in the store before marking it spent.
		// The API may return UTXOs not yet tracked locally.
		store.AddUTXO(&utxostore.StoredUTXO{
			ChainID:       chain.BSV,
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		})
		store.MarkSpent(chain.BSV, u.TxID, u.Vout, spentTxID)
	}

	if err := store.Save(); err != nil {
		logTxError(cc, "bsv send: failed to save utxo store: %v", err)
	}
}
