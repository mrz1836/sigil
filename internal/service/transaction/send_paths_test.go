package transaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

const (
	validETHAddress = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	validBSVAddress = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
)

func TestSend_Dispatch_ETH_InvalidAddress(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{
		Config:  newMockConfigProvider(),
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.ETH,
		To:        "not-an-address",
		AmountStr: "1.0",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSend_Dispatch_ETH_MissingRPCConfig(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethRPC = ""

	service := NewService(&Config{
		Config:  cfg,
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.ETH,
		To:        validETHAddress,
		AmountStr: "1.0",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSend_Dispatch_ETH_InvalidGasSpeed(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethRPC = "http://localhost:8545"

	service := NewService(&Config{
		Config:  cfg,
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.ETH,
		To:        validETHAddress,
		AmountStr: "1.0",
		GasSpeed:  "warp",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSend_Dispatch_ETH_InvalidAmountBeforeNetwork(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethRPC = "http://localhost:8545"

	service := NewService(&Config{
		Config:  cfg,
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.ETH,
		To:        validETHAddress,
		AmountStr: "abc",
		GasSpeed:  "medium",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSend_Dispatch_BSV_InvalidAddress(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{
		Config:  newMockConfigProvider(),
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.BSV,
		To:        "not-a-bsv-address",
		AmountStr: "1.0",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSend_Dispatch_BSV_InvalidAmount(t *testing.T) {
	t.Parallel()

	service := NewService(&Config{
		Config:  newMockConfigProvider(),
		Storage: newMockStorageProvider(),
		Logger:  newMockLogWriter(),
	})

	result, err := service.Send(context.Background(), &SendRequest{
		ChainID:   chain.BSV,
		To:        validBSVAddress,
		AmountStr: "invalid-amount",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestExportedWrapperFunctions(t *testing.T) {
	t.Parallel()

	logger := newMockLogWriter()

	provider := newMockCacheProvider()
	InvalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.0")
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 1, provider.saveCalled)

	store := newMockUTXOProvider()
	utxos := []chain.UTXO{{TxID: "tx1", Vout: 0, Amount: 5000, Address: validBSVAddress}}
	MarkSpentBSVUTXOs(logger, store, utxos, "spendtx")
	assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))

	mockWOC := &mockWOCClient{}
	client := bsv.NewClient(context.Background(), &bsv.ClientOptions{WOCClient: mockWOC})
	addresses := []wallet.Address{{Address: validBSVAddress, Index: 0, Path: "m/44'/236'/0'/0/0"}}
	aggregated, err := AggregateBSVUTXOs(context.Background(), client, addresses)
	require.NoError(t, err)
	assert.Empty(t, aggregated)

	RecordAgentSpend(logger, "", "", chain.BSV, nil)
}
