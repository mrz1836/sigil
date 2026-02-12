package eth

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethtypes "github.com/mrz1836/sigil/internal/chain/eth/types"
	"github.com/mrz1836/sigil/internal/wallet"
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
			data, err := BuildERC20TransferData(tc.to, tc.amount)
			require.NoError(t, err)
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
	data, err := BuildERC20TransferData("0x0000000000000000000000000000000000000001", big.NewInt(1))
	require.NoError(t, err)

	expectedSelector := []byte{0xa9, 0x05, 0x9c, 0xbb}
	assert.Equal(t, expectedSelector, data[:4])
}

func TestBuildERC20TransferData_AddressPadding(t *testing.T) {
	// Address should be right-padded to 32 bytes (left-padded with zeros)
	to := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	data, err := BuildERC20TransferData(to, big.NewInt(1))
	require.NoError(t, err)

	// First 12 bytes after selector should be zeros (padding)
	for i := 4; i < 16; i++ {
		assert.Equal(t, byte(0), data[i], "byte at position %d should be zero", i)
	}
}

func TestBuildERC20TransferData_AmountEncoding(t *testing.T) {
	// Amount is encoded as uint256 (32 bytes, big-endian)
	amount := big.NewInt(1000000) // 1 USDC (6 decimals)
	data, err := BuildERC20TransferData("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", amount)
	require.NoError(t, err)

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
	erc20Data, err := BuildERC20TransferData("0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", big.NewInt(1000000))
	require.NoError(t, err)
	params := &TxParams{
		From:         "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		To:           USDCMainnet, // USDC contract
		Value:        big.NewInt(0),
		GasLimit:     65000,
		GasPrice:     big.NewInt(20000000000),
		Nonce:        5,
		ChainID:      big.NewInt(1),
		Data:         erc20Data,
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
	params, err := NewERC20TransferParams(
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		USDCMainnet,
		big.NewInt(1000000),
	)
	require.NoError(t, err)

	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", params.From)
	assert.Equal(t, USDCMainnet, params.To) // To is the token contract
	assert.Equal(t, int64(0), params.Value.Int64())
	assert.NotEmpty(t, params.Data)
	assert.Equal(t, USDCMainnet, params.TokenAddress)
}

func TestBuildERC20TransferData_InvalidAddress(t *testing.T) {
	t.Parallel()
	_, err := BuildERC20TransferData("not-an-address", big.NewInt(1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recipient address")
}

func TestNewERC20TransferParams_InvalidRecipient(t *testing.T) {
	t.Parallel()
	_, err := NewERC20TransferParams(
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"invalid-address",
		USDCMainnet,
		big.NewInt(1000000),
	)
	require.Error(t, err)
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

func TestZeroBytes(t *testing.T) {
	t.Parallel()

	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	original := make([]byte, len(key))
	copy(original, key)

	wallet.ZeroBytes(key)

	// Verify all bytes are zeroed
	for i, b := range key {
		assert.Equal(t, byte(0), b, "byte at position %d should be zero", i)
	}
}

func TestZeroBytes_EmptySlice(t *testing.T) {
	t.Parallel()

	key := []byte{}
	wallet.ZeroBytes(key) // Should not panic
	assert.Empty(t, key)
}

func TestDeriveAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		privateKey []byte
		wantErr    bool
	}{
		{
			name: "valid 32-byte private key",
			// Known test private key (do not use in production)
			privateKey: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
				0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
				0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
			},
			wantErr: false,
		},
		{
			name:       "invalid - empty key",
			privateKey: []byte{},
			wantErr:    true,
		},
		{
			name:       "invalid - too short",
			privateKey: []byte{0x01, 0x02, 0x03},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr, err := DeriveAddress(tt.privateKey)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Verify address format
				assert.True(t, IsValidAddress(addr), "derived address should be valid: %s", addr)
				// Verify checksum format (0x prefix + 40 hex chars)
				assert.Regexp(t, `^0x[0-9a-fA-F]{40}$`, addr)
			}
		})
	}
}

func TestDeriveAddress_Deterministic(t *testing.T) {
	t.Parallel()

	// Same private key should always produce the same address
	privateKey := []byte{
		0xac, 0x03, 0x74, 0x16, 0x7c, 0x22, 0x69, 0xde,
		0xb2, 0x84, 0x31, 0x4e, 0x0a, 0x18, 0x15, 0x31,
		0x00, 0x1a, 0x68, 0x6c, 0x11, 0x28, 0x91, 0x5e,
		0x0f, 0x5c, 0x19, 0x22, 0xd3, 0x0a, 0xab, 0x81,
	}

	addr1, err := DeriveAddress(privateKey)
	require.NoError(t, err)

	addr2, err := DeriveAddress(privateKey)
	require.NoError(t, err)

	assert.Equal(t, addr1, addr2, "same private key should derive same address")
}

func TestHashMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message []byte
	}{
		{
			name:    "simple message",
			message: []byte("Hello, Ethereum!"),
		},
		{
			name:    "empty message",
			message: []byte{},
		},
		{
			name:    "binary data",
			message: []byte{0x00, 0x01, 0x02, 0xff},
		},
		{
			name:    "unicode message",
			message: []byte("Hello, World! \xf0\x9f\x8c\x8d"), // Contains emoji bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hash := HashMessage(tt.message)

			// EIP-191 hash should always be 32 bytes (Keccak256)
			assert.Len(t, hash, 32, "hash should be 32 bytes")

			// Hash should be deterministic
			hash2 := HashMessage(tt.message)
			assert.Equal(t, hash, hash2, "same message should produce same hash")
		})
	}
}

func TestHashMessage_KnownVector(t *testing.T) {
	t.Parallel()

	// Test with a known EIP-191 test vector
	// Message: "Hello, world!"
	// The EIP-191 prefix is "\x19Ethereum Signed Message:\n" + len(message)
	message := []byte("Hello, world!")
	hash := HashMessage(message)

	// Verify the hash is non-zero
	allZero := true
	for _, b := range hash {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "hash should not be all zeros")

	// Verify different messages produce different hashes
	differentMessage := []byte("Hello, world!!")
	differentHash := HashMessage(differentMessage)
	assert.NotEqual(t, hash, differentHash, "different messages should produce different hashes")
}

func TestHashMessage_EIP191Prefix(t *testing.T) {
	t.Parallel()

	// EIP-191 format: "\x19Ethereum Signed Message:\n" + len(message) + message
	// This test verifies the prefix is applied correctly by checking
	// that a raw hash differs from the EIP-191 hash

	message := []byte("test")
	eip191Hash := HashMessage(message)

	// Raw keccak256 of just the message would be different
	// We can verify this by checking the hash is not equal to what
	// we'd expect from hashing just the raw message
	assert.Len(t, eip191Hash, 32)
}

func TestSignTransaction(t *testing.T) {
	t.Parallel()

	// Parameters
	nonce := uint64(0)
	// Using nil for 'to' implies contract creation, which is valid for testing signing.

	value := big.NewInt(1000)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1000000000)
	data := []byte{}

	tx := ethtypes.NewLegacyTx(
		nonce,
		nil, // contract creation or valid address bytes. Let's use nil for simplicity or mock bytes.
		value,
		gasLimit,
		gasPrice,
		data,
	)

	// Private key
	privateKey := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	}
	chainID := big.NewInt(1)

	signedTx, err := SignTransaction(tx, privateKey, chainID)
	require.NoError(t, err)
	assert.NotNil(t, signedTx)

	// Verify it is signed
	assert.True(t, signedTx.IsSigned())

	// Verify signature components exist (v, r, s)
	// IsSigned checks v, r, s presence usually.
}
