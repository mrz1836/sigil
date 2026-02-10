package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

func setupTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	return NewFileStore(agentsDir)
}

func createTestCredential(walletName, label string, chains []chain.ID) *Credential {
	return &Credential{
		ID:         "agt_abc123",
		Label:      label,
		WalletName: walletName,
		Chains:     chains,
		Policy: Policy{
			MaxPerTxSat: 50000,
			MaxDailySat: 500000,
		},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func TestNewFileStore(t *testing.T) {
	t.Parallel()

	store := NewFileStore("/tmp/test-agents")
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}
	if store.basePath != "/tmp/test-agents" {
		t.Errorf("basePath = %q, want %q", store.basePath, "/tmp/test-agents")
	}
}

func TestFileStore_CreateCredential(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	cred := createTestCredential("test-wallet", "test-agent", []chain.ID{chain.BSV})
	cred.ID = TokenID(token)

	seed := []byte("test-seed-32-bytes-long-enough!!")

	if err := store.CreateCredential(cred, token, seed); err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}

	// Verify file was created
	agentPath := store.agentPath("test-wallet", cred.ID)
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Error("agent file was not created")
	}
}

func TestFileStore_CreateCredential_InvalidWalletName(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	cred := createTestCredential("invalid wallet!!", "test", []chain.ID{chain.BSV})

	err := store.CreateCredential(cred, "token", []byte("seed"))
	if err == nil {
		t.Error("CreateCredential() expected error for invalid wallet name")
	}
}

func TestFileStore_Load(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	token, _ := GenerateToken()

	cred := createTestCredential("load-test", "loader", []chain.ID{chain.BSV, chain.ETH})
	cred.ID = TokenID(token)

	seed := []byte("test-seed-32-bytes-long-enough!!")

	if err := store.CreateCredential(cred, token, seed); err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}

	// Load it back
	decryptedSeed, loadedCred, err := store.Load("load-test", cred.ID, token)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	defer zeroBytes(decryptedSeed)

	if string(decryptedSeed) != string(seed) {
		t.Errorf("Load() seed = %q, want %q", decryptedSeed, seed)
	}

	if loadedCred.Label != "loader" {
		t.Errorf("Load() label = %q, want %q", loadedCred.Label, "loader")
	}
	if loadedCred.WalletName != "load-test" {
		t.Errorf("Load() wallet = %q, want %q", loadedCred.WalletName, "load-test")
	}
	if len(loadedCred.Chains) != 2 {
		t.Errorf("Load() chains count = %d, want 2", len(loadedCred.Chains))
	}
}

func TestFileStore_Load_WrongToken(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	token, _ := GenerateToken()

	cred := createTestCredential("wrong-token", "agent", []chain.ID{chain.BSV})
	cred.ID = TokenID(token)

	if err := store.CreateCredential(cred, token, []byte("secret-seed-material-long-enoug")); err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}

	wrongToken, _ := GenerateToken()
	_, _, err := store.Load("wrong-token", cred.ID, wrongToken)
	if err == nil {
		t.Error("Load() expected error for wrong token")
	}
}

func TestFileStore_Load_NotFound(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	_, _, err := store.Load("nonexistent", "agt_000000", "token")
	if err == nil {
		t.Error("Load() expected error for nonexistent agent")
	}
}

func TestFileStore_Load_InvalidWalletName(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	_, _, err := store.Load("bad name!", "agt_123", "token")
	if err == nil {
		t.Error("Load() expected error for invalid wallet name")
	}
}

func TestFileStore_LoadByToken(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	token, _ := GenerateToken()

	cred := createTestCredential("by-token", "agent", []chain.ID{chain.BSV})
	cred.ID = TokenID(token)

	seed := []byte("load-by-token-seed-long-enough!!")

	if err := store.CreateCredential(cred, token, seed); err != nil {
		t.Fatalf("CreateCredential() error = %v", err)
	}

	// LoadByToken should find it
	decryptedSeed, loadedCred, err := store.LoadByToken("by-token", token)
	if err != nil {
		t.Fatalf("LoadByToken() error = %v", err)
	}
	defer zeroBytes(decryptedSeed)

	if string(decryptedSeed) != string(seed) {
		t.Errorf("LoadByToken() seed mismatch")
	}
	if loadedCred.ID != cred.ID {
		t.Errorf("LoadByToken() ID = %q, want %q", loadedCred.ID, cred.ID)
	}
}

func TestFileStore_LoadByToken_NotFound(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	_, _, err := store.LoadByToken("no-agents", "sigil_agt_nonexistent")
	if err == nil {
		t.Error("LoadByToken() expected error for nonexistent token")
	}
}

func TestFileStore_List(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	// Create two agents for the same wallet
	token1, _ := GenerateToken()
	cred1 := createTestCredential("list-test", "agent-1", []chain.ID{chain.BSV})
	cred1.ID = TokenID(token1)
	_ = store.CreateCredential(cred1, token1, []byte("seed-material-one-long-enough!!"))

	token2, _ := GenerateToken()
	cred2 := createTestCredential("list-test", "agent-2", []chain.ID{chain.ETH})
	cred2.ID = TokenID(token2)
	_ = store.CreateCredential(cred2, token2, []byte("seed-material-two-long-enough!!"))

	// List agents
	agents, err := store.List("list-test")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("List() returned %d agents, want 2", len(agents))
	}
}

func TestFileStore_List_Empty(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	agents, err := store.List("empty-wallet")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("List() returned %d agents, want 0", len(agents))
	}
}

func TestFileStore_List_InvalidWalletName(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	_, err := store.List("bad wallet!")
	if err == nil {
		t.Error("List() expected error for invalid wallet name")
	}
}

func TestFileStore_Delete(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)
	token, _ := GenerateToken()

	cred := createTestCredential("delete-test", "doomed", []chain.ID{chain.BSV})
	cred.ID = TokenID(token)
	_ = store.CreateCredential(cred, token, []byte("delete-me-seed-long-enough!!!!!"))

	// Verify exists
	agents, _ := store.List("delete-test")
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent before delete, got %d", len(agents))
	}

	// Delete
	if err := store.Delete("delete-test", cred.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify gone
	agents, _ = store.List("delete-test")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(agents))
	}
}

func TestFileStore_Delete_NotFound(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	// Should not error on nonexistent agent
	err := store.Delete("no-wallet", "agt_none")
	if err != nil {
		t.Errorf("Delete() unexpected error for nonexistent: %v", err)
	}
}

func TestFileStore_DeleteAll(t *testing.T) {
	t.Parallel()

	store := setupTestStore(t)

	// Create multiple agents
	for i := range 3 {
		token, _ := GenerateToken()
		cred := createTestCredential("deleteall-test", "agent", []chain.ID{chain.BSV})
		cred.ID = TokenID(token)
		_ = i
		_ = store.CreateCredential(cred, token, []byte("seed-for-deleteall-long-enough!!"))
	}

	count, err := store.DeleteAll("deleteall-test")
	if err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	if count != 3 {
		t.Errorf("DeleteAll() count = %d, want 3", count)
	}

	// Verify all gone
	agents, _ := store.List("deleteall-test")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after DeleteAll, got %d", len(agents))
	}
}

func TestFileStore_agentPath_TraversalPrevention(t *testing.T) {
	t.Parallel()

	store := NewFileStore("/tmp/agents")

	// Normal path should work
	normalPath := store.agentPath("mywallet", "agt_abc123")
	if normalPath == "" {
		t.Error("agentPath() returned empty for valid input")
	}

	// Path traversal should return empty
	traversalPath := store.agentPath("../../../etc", "passwd")
	if traversalPath != "" {
		t.Errorf("agentPath() should return empty for traversal, got %q", traversalPath)
	}
}

func TestFileStore_counterPath_TraversalPrevention(t *testing.T) {
	t.Parallel()

	store := NewFileStore("/tmp/agents")

	normalPath := store.counterPath("wallet", "agt_abc")
	if normalPath == "" {
		t.Error("counterPath() returned empty for valid input")
	}

	traversalPath := store.counterPath("../../../etc", "passwd")
	if traversalPath != "" {
		t.Errorf("counterPath() should return empty for traversal, got %q", traversalPath)
	}
}

func TestFileStore_CounterPath(t *testing.T) {
	t.Parallel()

	store := NewFileStore("/tmp/agents")

	path := store.CounterPath("mywallet", "agt_123abc")
	if path == "" {
		t.Error("CounterPath() returned empty for valid input")
	}

	expected := filepath.Join("/tmp/agents", "mywallet-agt_123abc.counter")
	if path != expected {
		t.Errorf("CounterPath() = %q, want %q", path, expected)
	}
}
