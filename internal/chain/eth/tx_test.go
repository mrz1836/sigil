package eth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTransferData(t *testing.T) {
	tests := []struct {
		name      string
		to        string
		amount    *big.Int
		expectLen int
	}{
		{
			name:      "standard transfer",
			to:        "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			amount:    big.NewInt(1000000), // 1 USDC
			expectLen: 68,                  // 4 bytes selector + 32 bytes address + 32 bytes amount
		},
		{
			name:      "zero amount",
			to:        "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
			amount:    big.NewInt(0),
			expectLen: 68,
		},
		{
			name:      "large amount",
			to:        "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
			amount:    new(big.Int).Mul(big.NewInt(1000000000), big.NewInt(1000000)), // 1B USDC
			expectLen: 68,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := BuildERC20TransferData(tc.to, tc.amount)
			assert.Len(t, data, tc.expectLen)

			// Verify selector is correct (transfer(address,uint256))
			assert.Equal(t, byte(0xa9), data[0])
			assert.Equal(t, byte(0x05), data[1])
			assert.Equal(t, byte(0x9c), data[2])
			assert.Equal(t, byte(0xbb), data[3])
		})
	}
}

func TestBuildERC20TransferData_Selector(t *testing.T) {
	// transfer(address,uint256) = keccak256("transfer(address,uint256)")[0:4] = 0xa9059cbb
	data := BuildERC20TransferData("0x0000000000000000000000000000000000000001", big.NewInt(1))

	expectedSelector := []byte{0xa9, 0x05, 0x9c, 0xbb}
	assert.Equal(t, expectedSelector, data[:4])
}

func TestBuildERC20TransferData_AddressPadding(t *testing.T) {
	// Address should be right-padded to 32 bytes (left-padded with zeros)
	to := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	data := BuildERC20TransferData(to, big.NewInt(1))

	// First 12 bytes after selector should be zeros (padding)
	for i := 4; i < 16; i++ {
		assert.Equal(t, byte(0), data[i], "byte at position %d should be zero", i)
	}
}

func TestBuildERC20TransferData_AmountEncoding(t *testing.T) {
	// Amount is encoded as uint256 (32 bytes, big-endian)
	amount := big.NewInt(1000000) // 1 USDC (6 decimals)
	data := BuildERC20TransferData("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", amount)

	// Extract amount from data (last 32 bytes)
	amountBytes := data[36:68]
	decodedAmount := new(big.Int).SetBytes(amountBytes)
	assert.Equal(t, 0, amount.Cmp(decodedAmount))
}

func TestTxParams(t *testing.T) {
	params := &TxParams{
		From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		GasLimit: 21000,
		GasPrice: big.NewInt(20000000000), // 20 Gwei
		Nonce:    0,
		ChainID:  big.NewInt(1),
	}

	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", params.From)
	assert.Equal(t, "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", params.To)
	assert.Equal(t, uint64(21000), params.GasLimit)
}

func TestTxParamsForERC20(t *testing.T) {
	// ERC-20 transfer should have zero value and data
	params := &TxParams{
		From:         "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		To:           USDCMainnet, // USDC contract
		Value:        big.NewInt(0),
		GasLimit:     65000,
		GasPrice:     big.NewInt(20000000000),
		Nonce:        5,
		ChainID:      big.NewInt(1),
		Data:         BuildERC20TransferData("0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", big.NewInt(1000000)),
		TokenAddress: USDCMainnet,
	}

	assert.Equal(t, big.NewInt(0).Int64(), params.Value.Int64())
	assert.NotEmpty(t, params.Data)
	assert.Equal(t, USDCMainnet, params.TokenAddress)
	assert.Len(t, params.Data, 68)
}

func TestNewETHTransferParams(t *testing.T) {
	params := NewETHTransferParams(
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		big.NewInt(1000000000000000000),
	)

	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", params.From)
	assert.Equal(t, "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", params.To)
	assert.Equal(t, int64(1000000000000000000), params.Value.Int64())
	assert.Empty(t, params.Data)
	assert.Empty(t, params.TokenAddress)
}

func TestNewERC20TransferParams(t *testing.T) {
	params := NewERC20TransferParams(
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		USDCMainnet,
		big.NewInt(1000000),
	)

	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", params.From)
	assert.Equal(t, USDCMainnet, params.To) // To is the token contract
	assert.Equal(t, int64(0), params.Value.Int64())
	assert.NotEmpty(t, params.Data)
	assert.Equal(t, USDCMainnet, params.TokenAddress)
}

func TestValidateTxParams(t *testing.T) {
	tests := []struct {
		name    string
		params  *TxParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid ETH transfer",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 21000,
				GasPrice: big.NewInt(20000000000),
				ChainID:  big.NewInt(1),
			},
			wantErr: false,
		},
		{
			name: "invalid from address",
			params: &TxParams{
				From:     "invalid",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 21000,
				GasPrice: big.NewInt(20000000000),
				ChainID:  big.NewInt(1),
			},
			wantErr: true,
			errMsg:  "from",
		},
		{
			name: "invalid to address",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "invalid",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 21000,
				GasPrice: big.NewInt(20000000000),
				ChainID:  big.NewInt(1),
			},
			wantErr: true,
			errMsg:  "to",
		},
		{
			name: "nil value",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    nil,
				GasLimit: 21000,
				GasPrice: big.NewInt(20000000000),
				ChainID:  big.NewInt(1),
			},
			wantErr: true,
			errMsg:  "value",
		},
		{
			name: "nil gas price",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 21000,
				GasPrice: nil,
				ChainID:  big.NewInt(1),
			},
			wantErr: true,
			errMsg:  "gas price",
		},
		{
			name: "nil chain ID",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 21000,
				GasPrice: big.NewInt(20000000000),
				ChainID:  nil,
			},
			wantErr: true,
			errMsg:  "chain ID",
		},
		{
			name: "zero gas limit",
			params: &TxParams{
				From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				Value:    big.NewInt(1000000000000000000),
				GasLimit: 0,
				GasPrice: big.NewInt(20000000000),
				ChainID:  big.NewInt(1),
			},
			wantErr: true,
			errMsg:  "gas limit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestZeroPrivateKey(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]byte, len(key))
	copy(original, key)

	ZeroPrivateKey(key)

	// Verify all bytes are zeroed
	for i, b := range key {
		assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
	}
}
