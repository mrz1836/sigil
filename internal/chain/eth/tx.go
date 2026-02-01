package eth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

// ERC-20 transfer function selector: keccak256("transfer(address,uint256)")[0:4]
//
//nolint:gochecknoglobals // ERC-20 constant
var erc20TransferSelector = []byte{0xa9, 0x05, 0x9c, 0xbb}

// TxParams contains parameters for building a transaction.
type TxParams struct {
	From         string   // Sender address
	To           string   // Recipient address (or contract for ERC-20)
	Value        *big.Int // Value in wei (0 for ERC-20 transfers)
	GasLimit     uint64   // Gas limit
	GasPrice     *big.Int // Gas price in wei
	Nonce        uint64   // Transaction nonce
	ChainID      *big.Int // Network chain ID
	Data         []byte   // Transaction data (for contract calls)
	TokenAddress string   // ERC-20 token address (empty for native ETH)
}

// Validate checks that the transaction parameters are valid.
func (p *TxParams) Validate() error {
	if !IsValidAddress(p.From) {
		return sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
			"field":   "from",
			"address": p.From,
		})
	}
	if !IsValidAddress(p.To) {
		return sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
			"field":   "to",
			"address": p.To,
		})
	}
	if p.Value == nil {
		return sigilerrors.WithDetails(sigilerrors.ErrInvalidValue, map[string]string{
			"reason": "value cannot be nil",
		})
	}
	if p.GasPrice == nil {
		return sigilerrors.ErrInvalidGasPrice
	}
	if p.ChainID == nil {
		return sigilerrors.ErrInvalidChainID
	}
	if p.GasLimit == 0 {
		return sigilerrors.ErrInvalidGasLimit
	}
	return nil
}

// NewETHTransferParams creates parameters for a native ETH transfer.
//
//nolint:funcorder // Constructor grouped with related constructor
func NewETHTransferParams(from, to string, value *big.Int) *TxParams {
	return &TxParams{
		From:  from,
		To:    to,
		Value: value,
	}
}

// NewERC20TransferParams creates parameters for an ERC-20 token transfer.
//
//nolint:funcorder // Constructor grouped with related constructor
func NewERC20TransferParams(from, recipient, tokenAddress string, amount *big.Int) *TxParams {
	return &TxParams{
		From:         from,
		To:           tokenAddress, // Transaction is sent to the token contract
		Value:        big.NewInt(0),
		Data:         BuildERC20TransferData(recipient, amount),
		TokenAddress: tokenAddress,
	}
}

// BuildERC20TransferData builds the call data for an ERC-20 transfer.
// transfer(address,uint256) = 0xa9059cbb
func BuildERC20TransferData(to string, amount *big.Int) []byte {
	// Function selector: transfer(address,uint256)
	data := make([]byte, 68) // 4 + 32 + 32
	copy(data[:4], erc20TransferSelector)

	// Pad address to 32 bytes (left-pad with zeros)
	toAddr := common.HexToAddress(to)
	copy(data[16:36], toAddr.Bytes())

	// Pad amount to 32 bytes (left-pad with zeros)
	amountBytes := amount.Bytes()
	copy(data[68-len(amountBytes):68], amountBytes)

	return data
}

// BuildTransaction creates an unsigned transaction from parameters.
func (c *Client) BuildTransaction(ctx context.Context, params *TxParams) (*types.Transaction, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Get nonce if not set
	if params.Nonce == 0 {
		nonce, err := c.GetNonce(ctx, params.From)
		if err != nil {
			return nil, fmt.Errorf("getting nonce: %w", err)
		}
		params.Nonce = nonce
	}

	// Get chain ID if not set
	if params.ChainID == nil {
		chainID, err := c.GetChainID(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting chain ID: %w", err)
		}
		params.ChainID = chainID
	}

	toAddr := common.HexToAddress(params.To)

	// Create transaction
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    params.Nonce,
		To:       &toAddr,
		Value:    params.Value,
		Gas:      params.GasLimit,
		GasPrice: params.GasPrice,
		Data:     params.Data,
	})

	return tx, nil
}

// SignTransaction signs a transaction with the provided private key.
// The private key bytes are zeroed after signing for security.
func SignTransaction(tx *types.Transaction, privateKey []byte, chainID *big.Int) (*types.Transaction, error) {
	// Ensure we zero the key when done
	defer ZeroPrivateKey(privateKey)

	// Parse private key
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	// Create EIP-155 signer
	signer := types.NewEIP155Signer(chainID)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		return nil, fmt.Errorf("signing transaction: %w", err)
	}

	return signedTx, nil
}

// BroadcastTransaction sends a signed transaction to the network.
func (c *Client) BroadcastTransaction(ctx context.Context, tx *types.Transaction) error {
	if err := c.connect(ctx); err != nil {
		return err
	}

	if err := c.ethClient.SendTransaction(ctx, tx); err != nil {
		return fmt.Errorf("broadcasting transaction: %w", err)
	}

	return nil
}

// Send implements the chain.Chain interface - builds, signs, and broadcasts a transaction.
//
//nolint:gocognit,gocyclo // Transaction building involves multiple steps
func (c *Client) Send(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error) {
	// Validate addresses
	if err := ValidateChecksumAddress(req.From); err != nil {
		if !IsValidAddress(req.From) {
			return nil, sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
				"field":   "from",
				"address": req.From,
			})
		}
	}
	if err := ValidateChecksumAddress(req.To); err != nil {
		if !IsValidAddress(req.To) {
			return nil, sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
				"field":   "to",
				"address": req.To,
			})
		}
	}

	// Determine if this is an ERC-20 or native transfer
	var params *TxParams
	var tokenSymbol string

	if req.Token != "" {
		// ERC-20 transfer
		params = NewERC20TransferParams(req.From, req.To, req.Token, req.Amount)
		tokenSymbol = "USDC" // Assume USDC for now, can be extended
	} else {
		// Native ETH transfer
		params = NewETHTransferParams(req.From, req.To, req.Amount)
		tokenSymbol = ""
	}

	// Get gas estimate
	speed := GasSpeedMedium
	var estimate *GasEstimate
	var err error

	if req.Token != "" {
		estimate, err = c.EstimateGasForERC20Transfer(ctx, speed)
	} else {
		estimate, err = c.EstimateGasForETHTransfer(ctx, speed)
	}
	if err != nil {
		return nil, fmt.Errorf("estimating gas: %w", err)
	}

	// Set gas params
	params.GasLimit = estimate.GasLimit
	params.GasPrice = estimate.GasPrice

	// Override gas limit if specified
	if req.GasLimit > 0 {
		params.GasLimit = req.GasLimit
	}

	// Build transaction
	tx, err := c.BuildTransaction(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("building transaction: %w", err)
	}

	// Sign transaction (this zeros the private key)
	signedTx, err := SignTransaction(tx, req.PrivateKey, c.chainID)
	if err != nil {
		return nil, fmt.Errorf("signing transaction: %w", err)
	}

	// Broadcast transaction
	if err := c.BroadcastTransaction(ctx, signedTx); err != nil {
		return nil, err
	}

	// Build result
	result := &chain.TransactionResult{
		Hash:     signedTx.Hash().Hex(),
		From:     req.From,
		To:       req.To,
		Amount:   c.FormatAmount(req.Amount),
		Token:    tokenSymbol,
		Fee:      c.FormatAmount(estimate.Total),
		GasUsed:  params.GasLimit,
		GasPrice: FormatGasPrice(params.GasPrice),
		Status:   "pending",
	}

	return result, nil
}

// ZeroPrivateKey zeros out a private key byte slice for security.
func ZeroPrivateKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}

// DeriveAddress derives an Ethereum address from a private key.
func DeriveAddress(privateKey []byte) (string, error) {
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	publicKey := key.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", sigilerrors.ErrInvalidPublicKey
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return ToChecksumAddress(address.Hex()), nil
}

// HashMessage hashes a message according to EIP-191 personal_sign.
func HashMessage(message []byte) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(prefix))
	hasher.Write(message)
	return hasher.Sum(nil)
}
