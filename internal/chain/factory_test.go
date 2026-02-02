package chain

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

// mockChain implements the Chain interface for testing.
type mockChain struct {
	id ID
}

func (m *mockChain) ID() ID                                               { return m.id }
func (m *mockChain) GetBalance(context.Context, string) (*big.Int, error) { return big.NewInt(0), nil }
func (m *mockChain) ValidateAddress(string) error                         { return nil }
func (m *mockChain) EstimateFee(context.Context, string, string, *big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (m *mockChain) Send(context.Context, SendRequest) (*TransactionResult, error) {
	return &TransactionResult{}, nil
}
func (m *mockChain) FormatAmount(*big.Int) string         { return "0" }
func (m *mockChain) ParseAmount(string) (*big.Int, error) { return big.NewInt(0), nil }

func TestConfigurableFactory_Register(t *testing.T) {
	factory := NewConfigurableFactory()

	called := false
	creator := func(_ context.Context, _ string) (Chain, error) {
		called = true
		return &mockChain{id: ETH}, nil
	}

	factory.Register(ETH, creator)

	chain, err := factory.NewChain(context.Background(), ETH, "http://localhost:8545")
	if err != nil {
		t.Fatalf("NewChain() error = %v", err)
	}
	if !called {
		t.Error("creator was not called")
	}
	if chain.ID() != ETH {
		t.Errorf("chain.ID() = %q, want %q", chain.ID(), ETH)
	}
}

func TestConfigurableFactory_NewChain(t *testing.T) {
	factory := NewConfigurableFactory()
	factory.Register(ETH, func(_ context.Context, _ string) (Chain, error) {
		return &mockChain{id: ETH}, nil
	})

	t.Run("registered chain succeeds", func(t *testing.T) {
		chain, err := factory.NewChain(context.Background(), ETH, "http://localhost")
		if err != nil {
			t.Errorf("NewChain() error = %v", err)
		}
		if chain == nil {
			t.Error("NewChain() returned nil chain")
		}
	})

	t.Run("unregistered chain fails", func(t *testing.T) {
		_, err := factory.NewChain(context.Background(), BSV, "http://localhost")
		if err == nil {
			t.Error("NewChain() expected error for unregistered chain")
		}
		if !errors.Is(err, ErrUnsupportedChain) {
			t.Errorf("NewChain() error = %v, want %v", err, ErrUnsupportedChain)
		}
	})
}

func TestConfigurableFactory_IsSupported(t *testing.T) {
	factory := NewConfigurableFactory()
	factory.Register(ETH, func(_ context.Context, _ string) (Chain, error) {
		return &mockChain{id: ETH}, nil
	})

	if !factory.IsSupported(ETH) {
		t.Error("IsSupported(ETH) = false, want true")
	}
	if factory.IsSupported(BSV) {
		t.Error("IsSupported(BSV) = true, want false")
	}
}

func TestConfigurableFactory_SupportedChains(t *testing.T) {
	factory := NewConfigurableFactory()
	factory.Register(ETH, func(_ context.Context, _ string) (Chain, error) {
		return &mockChain{id: ETH}, nil
	})
	factory.Register(BSV, func(_ context.Context, _ string) (Chain, error) {
		return &mockChain{id: BSV}, nil
	})

	chains := factory.SupportedChains()
	if len(chains) != 2 {
		t.Errorf("SupportedChains() returned %d chains, want 2", len(chains))
	}

	found := make(map[ID]bool)
	for _, c := range chains {
		found[c] = true
	}
	if !found[ETH] || !found[BSV] {
		t.Errorf("SupportedChains() = %v, want [ETH, BSV]", chains)
	}
}

func TestDefaultFactory_NewChain(t *testing.T) {
	factory := NewDefaultFactory()

	t.Run("valid chain returns ErrValidationOnly", func(t *testing.T) {
		_, err := factory.NewChain(context.Background(), ETH, "http://localhost")
		if !errors.Is(err, ErrValidationOnly) {
			t.Errorf("NewChain() error = %v, want %v", err, ErrValidationOnly)
		}
	})

	t.Run("invalid chain returns ErrUnsupportedChain", func(t *testing.T) {
		_, err := factory.NewChain(context.Background(), ID("invalid"), "http://localhost")
		if !errors.Is(err, ErrUnsupportedChain) {
			t.Errorf("NewChain() error = %v, want %v", err, ErrUnsupportedChain)
		}
	})

	t.Run("future chain BTC returns ErrUnsupportedChain", func(t *testing.T) {
		_, err := factory.NewChain(context.Background(), BTC, "http://localhost")
		if !errors.Is(err, ErrUnsupportedChain) {
			t.Errorf("NewChain() error = %v, want %v", err, ErrUnsupportedChain)
		}
	})
}

func TestIsSupportedChain(t *testing.T) {
	tests := []struct {
		name string
		id   ID
		want bool
	}{
		{"ETH", ETH, true},
		{"BSV", BSV, true},
		{"BTC", BTC, false},
		{"BCH", BCH, false},
		{"unknown", ID("unknown"), false},
		{"empty", ID(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedChain(tt.id); got != tt.want {
				t.Errorf("IsSupportedChain() = %v, want %v", got, tt.want)
			}
		})
	}
}
