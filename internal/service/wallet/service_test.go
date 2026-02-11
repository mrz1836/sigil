package wallet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

func TestValidateExists_Found(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test"}, nil)

	service := NewService(&Config{
		Storage: storage,
	})

	err := service.ValidateExists("test")
	assert.NoError(t, err)
}

func TestValidateExists_NotFound(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()

	service := NewService(&Config{
		Storage: storage,
	})

	err := service.ValidateExists("nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)
	assert.Contains(t, err.Error(), "wallet not found")
}

func TestValidateExists_StorageError(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.existsErr = errors.New("storage failure") //nolint:err113 // test error

	service := NewService(&Config{
		Storage: storage,
	})

	err := service.ValidateExists("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage failure")
}

func TestList_MultipleWallets(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "wallet1"}, nil)
	storage.addWallet(&wallet.Wallet{Name: "wallet2"}, nil)
	storage.addWallet(&wallet.Wallet{Name: "wallet3"}, nil)

	service := NewService(&Config{
		Storage: storage,
	})

	wallets, err := service.List()
	require.NoError(t, err)
	assert.Len(t, wallets, 3)
	assert.Contains(t, wallets, "wallet1")
	assert.Contains(t, wallets, "wallet2")
	assert.Contains(t, wallets, "wallet3")
}

func TestList_Empty(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()

	service := NewService(&Config{
		Storage: storage,
	})

	wallets, err := service.List()
	require.NoError(t, err)
	assert.Empty(t, wallets)
}

func TestList_StorageError(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.listErr = errors.New("storage failure") //nolint:err113 // test error

	service := NewService(&Config{
		Storage: storage,
	})

	wallets, err := service.List()
	require.Error(t, err)
	assert.Nil(t, wallets)
	assert.Contains(t, err.Error(), "storage failure")
}

func TestLoadMetadata_Success(t *testing.T) {
	t.Parallel()

	testWallet := &wallet.Wallet{
		Name:          "test",
		EnabledChains: []chain.ID{chain.BSV, chain.ETH},
	}

	storage := newMockStorageProvider()
	storage.addWallet(testWallet, nil)

	service := NewService(&Config{
		Storage: storage,
	})

	wlt, err := service.LoadMetadata("test")
	require.NoError(t, err)
	assert.NotNil(t, wlt)
	assert.Equal(t, "test", wlt.Name)
	assert.Equal(t, []chain.ID{chain.BSV, chain.ETH}, wlt.EnabledChains)
}

func TestLoadMetadata_NotFound(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()

	service := NewService(&Config{
		Storage: storage,
	})

	wlt, err := service.LoadMetadata("nonexistent")
	require.Error(t, err)
	assert.Nil(t, wlt)
	assert.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

func TestLoadMetadata_LoadError(t *testing.T) {
	t.Parallel()

	storage := newMockStorageProvider()
	storage.addWallet(&wallet.Wallet{Name: "test"}, nil)
	storage.loadMetaErr = errors.New("metadata load failure") //nolint:err113 // test error

	service := NewService(&Config{
		Storage: storage,
	})

	wlt, err := service.LoadMetadata("test")
	require.Error(t, err)
	assert.Nil(t, wlt)
	assert.Contains(t, err.Error(), "metadata load failure")
}

// Mock implementations for service package tests
// These are separate from the shared testutil_test.go mocks since they're in a different package

type mockStorageProvider struct {
	wallets       map[string]*wallet.Wallet
	seeds         map[string][]byte
	existsErr     error
	loadErr       error
	loadMetaErr   error
	listErr       error
	updateMetaErr error
}

func newMockStorageProvider() *mockStorageProvider {
	return &mockStorageProvider{
		wallets: make(map[string]*wallet.Wallet),
		seeds:   make(map[string][]byte),
	}
}

func (m *mockStorageProvider) Exists(name string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, exists := m.wallets[name]
	return exists, nil
}

func (m *mockStorageProvider) Load(name string, _ []byte) (*wallet.Wallet, []byte, error) {
	if m.loadErr != nil {
		return nil, nil, m.loadErr
	}
	w, exists := m.wallets[name]
	if !exists {
		return nil, nil, wallet.ErrWalletNotFound
	}
	seed := m.seeds[name]
	return w, seed, nil
}

func (m *mockStorageProvider) LoadMetadata(name string) (*wallet.Wallet, error) {
	if m.loadMetaErr != nil {
		return nil, m.loadMetaErr
	}
	w, exists := m.wallets[name]
	if !exists {
		return nil, wallet.ErrWalletNotFound
	}
	return w, nil
}

func (m *mockStorageProvider) List() ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	names := make([]string, 0, len(m.wallets))
	for name := range m.wallets {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockStorageProvider) UpdateMetadata(w *wallet.Wallet) error {
	if m.updateMetaErr != nil {
		return m.updateMetaErr
	}
	m.wallets[w.Name] = w
	return nil
}

func (m *mockStorageProvider) addWallet(w *wallet.Wallet, seed []byte) {
	m.wallets[w.Name] = w
	if seed != nil {
		m.seeds[w.Name] = seed
	}
}
