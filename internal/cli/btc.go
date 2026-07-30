package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/transaction"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// btcConfirmationDetails holds computed details for BTC transaction confirmation.
type btcConfirmationDetails struct {
	To              string
	AmountSats      uint64         // Actual satoshi amount (computed for sweep)
	IsSweep         bool           // Whether this is a sweep-all transaction
	EstimatedFee    uint64         // Estimated fee in satoshis
	FeeRate         uint64         // Fee rate in sat/vByte
	TotalUTXOs      int            // Total number of UTXOs being spent
	AddressUTXOs    map[string]int // Address -> UTXO count
	SourceAddresses []string       // Ordered list of addresses with UTXOs
}

// promptBTCConfirmation handles BTC transaction confirmation prompt.
func promptBTCConfirmation(ctx context.Context, cmd *cobra.Command, req *transaction.SendRequest, addresses []wallet.Address) (bool, error) {
	details, err := prepareBTCConfirmation(ctx, cmd, req, addresses)
	if err != nil {
		return false, err
	}

	displayBTCTxDetailsEnhanced(cmd, details)

	return promptConfirmFn(), nil
}

// prepareBTCConfirmation fetches UTXOs and calculates actual amounts for confirmation
// display. This runs BEFORE the confirmation prompt so the user sees accurate values.
//
//nolint:gocognit,gocyclo // BTC confirmation flow inherently complex with multiple validations
func prepareBTCConfirmation(
	ctx context.Context,
	cmd *cobra.Command,
	req *transaction.SendRequest,
	addresses []wallet.Address,
) (*btcConfirmationDetails, error) {
	cc := GetCmdContext(cmd)

	network := btcClientNetwork(req.Network)
	btcClient := btc.NewClient(ctx, &btc.ClientOptions{
		APIKey:      cc.Cfg.GetBTCAPIKey(),
		Network:     network,
		Logger:      cc.Log,
		FeeStrategy: btc.FeeStrategy(cc.Cfg.GetBTCFeeStrategy()),
	})

	feeRate := btcClient.FeeRate(ctx)

	// Load UTXO store for spent filtering
	walletPath := filepath.Join(cc.Cfg.GetHome(), "wallets", req.Wallet)
	utxoStore := utxostore.New(walletPath)
	if loadErr := utxoStore.Load(); loadErr != nil {
		if cc.Log != nil {
			cc.Log.Error("failed to load utxo store for confirmation: %v", loadErr)
		}
		utxoStore = nil // Non-fatal
	}

	allUTXOs, err := transaction.AggregateBTCUTXOs(ctx, btcClient, addresses)
	if err != nil {
		return nil, fmt.Errorf("fetching UTXOs for confirmation: %w", err)
	}
	if utxoStore != nil {
		allUTXOs = transaction.FilterSpentBTCUTXOs(allUTXOs, utxoStore)
	}

	// Group UTXOs by address
	addressUTXOs := make(map[string]int)
	var sourceAddresses []string
	for _, u := range allUTXOs {
		if _, exists := addressUTXOs[u.Address]; !exists {
			sourceAddresses = append(sourceAddresses, u.Address)
		}
		addressUTXOs[u.Address]++
	}

	details := &btcConfirmationDetails{
		To:              req.To,
		FeeRate:         feeRate,
		IsSweep:         req.SweepAll(),
		AddressUTXOs:    addressUTXOs,
		SourceAddresses: sourceAddresses,
	}

	//nolint:nestif // Sweep vs normal send have distinct display paths
	if req.SweepAll() {
		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found for sweep transaction")
		}

		var totalInputs uint64
		for _, u := range allUTXOs {
			totalInputs += u.Amount
		}

		recipientScript, scriptErr := btc.AddressToScript(req.To, network)
		if scriptErr != nil {
			return nil, scriptErr
		}
		sweepAmount, sweepErr := btc.CalculateSweepAmount(totalInputs, len(allUTXOs), len(*recipientScript), feeRate)
		if sweepErr != nil {
			return nil, sweepErr
		}

		details.AmountSats = sweepAmount
		details.EstimatedFee = totalInputs - sweepAmount
		details.TotalUTXOs = len(allUTXOs)
	} else {
		amount, amtErr := btcClient.ParseAmount(req.AmountStr)
		if amtErr != nil {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid amount: %s", req.AmountStr),
			)
		}

		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found for transaction")
		}

		selected, _, selErr := btcClient.SelectUTXOs(allUTXOs, amount.Uint64(), feeRate)
		if selErr != nil {
			return nil, selErr
		}

		selectedAddresses := make(map[string]int)
		var selectedSourceAddrs []string
		for _, u := range selected {
			if _, exists := selectedAddresses[u.Address]; !exists {
				selectedSourceAddrs = append(selectedSourceAddrs, u.Address)
			}
			selectedAddresses[u.Address]++
		}

		details.AmountSats = amount.Uint64()
		details.EstimatedFee = btc.EstimateFeeForTx(len(selected), 2, feeRate)
		details.TotalUTXOs = len(selected)
		details.AddressUTXOs = selectedAddresses
		details.SourceAddresses = selectedSourceAddrs
	}

	return details, nil
}

// displayBTCTxDetailsEnhanced shows BTC transaction details with computed values.
func displayBTCTxDetailsEnhanced(cmd *cobra.Command, details *btcConfirmationDetails) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    TRANSACTION DETAILS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	if len(details.SourceAddresses) == 1 {
		out(w, "  From:      %s\n", details.SourceAddresses[0])
	} else {
		out(w, "  From:      %d addresses with UTXOs:\n", len(details.SourceAddresses))
		for _, addr := range details.SourceAddresses {
			count := details.AddressUTXOs[addr]
			out(w, "             • %s (%d UTXO%s)\n", addr, count, pluralize(count))
		}
	}

	out(w, "  To:        %s\n", details.To)

	if details.IsSweep {
		out(w, "  Amount:    %s sats (sweep all) BTC\n", formatSatsWithCommas(details.AmountSats))
	} else {
		out(w, "  Amount:    %s sats BTC\n", formatSatsWithCommas(details.AmountSats))
	}

	if len(details.SourceAddresses) > 1 {
		out(w, "  Total:     %d UTXO%s\n", details.TotalUTXOs, pluralize(details.TotalUTXOs))
	} else {
		out(w, "  UTXOs:     %d\n", details.TotalUTXOs)
	}

	out(w, "  Fee Rate:  %d sat/vB\n", details.FeeRate)
	out(w, "  Est. Fee:  %s satoshis\n", formatSatsWithCommas(details.EstimatedFee))

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
}

// convertToBTCTransactionResult converts a service result to chain.TransactionResult.
func convertToBTCTransactionResult(result *transaction.SendResult) *chain.TransactionResult {
	return &chain.TransactionResult{
		Hash:   result.Hash,
		From:   result.From,
		To:     result.To,
		Amount: result.Amount,
		Fee:    result.Fee,
		Status: result.Status,
	}
}

// displayBTCTxResult shows the BTC transaction result.
func displayBTCTxResult(cmd *cobra.Command, result *chain.TransactionResult, network string) {
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

	if cc.Fmt.Format() == output.FormatJSON {
		displayBTCTxResultJSON(w, result)
	} else {
		displayBTCTxResultText(w, result, network)
	}
}

// displayBTCTxResultText shows the BTC transaction result in text format.
func displayBTCTxResultText(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult, network string,
) {
	outln(w, "\nTransaction broadcast successfully!")
	outln(w)
	out(w, "  Hash:   %s\n", result.Hash)
	out(w, "  Status: %s\n", result.Status)
	out(w, "  Amount: %s BTC\n", result.Amount)
	out(w, "  Fee:    %s BTC\n", result.Fee)
	outln(w)
	outln(w, "Track your transaction:")
	for _, link := range btcExplorerTxLinks(network, result.Hash) {
		out(w, "  %s\n", link)
	}
}

// displayBTCTxResultJSON shows the BTC transaction result in JSON format.
func displayBTCTxResultJSON(w interface {
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
