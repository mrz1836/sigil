package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/mrz1836/sigil/internal/fileutil"
	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

// ErrUseCreateCredential indicates the generic Create method should not be called directly.
var ErrUseCreateCredential = errors.New("use CreateCredential for typed access")

// File and directory permissions.
const (
	agentFilePermissions = 0o600
	agentDirPermissions  = 0o700
	agentFileExtension   = ".agent"
)

// walletNameRegex mirrors the pattern from session/manager.go.
var walletNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// FileStore provides file-based agent credential storage.
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a new agent file store.
// basePath is typically ~/.sigil/agents.
func NewFileStore(basePath string) *FileStore {
	return &FileStore{basePath: basePath}
}

// Create stores a new agent credential encrypted with the given token.
func (s *FileStore) Create(_ string, _ []byte, _ string, _ Policy,
	_ string, _ interface{ UnixNano() int64 }, _ []interface{ String() string },
) (*Credential, error) {
	// This method signature uses interface{} to avoid import cycles.
	// Use CreateCredential instead for typed access.
	return nil, ErrUseCreateCredential
}

// CreateCredential stores a new agent credential encrypted with the given token.
func (s *FileStore) CreateCredential(cred *Credential, token string, seed []byte) error {
	if !walletNameRegex.MatchString(cred.WalletName) {
		return fmt.Errorf("%w: %q", ErrInvalidWallet, cred.WalletName)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt the seed with the token
	encryptedSeed, err := sigilcrypto.Encrypt(seed, token)
	if err != nil {
		return fmt.Errorf("encrypting seed with agent token: %w", err)
	}
	cred.EncryptedSeed = encryptedSeed

	// Compute policy HMAC
	policyHMAC, err := ComputePolicyHMAC(&cred.Policy, token)
	if err != nil {
		return fmt.Errorf("computing policy HMAC: %w", err)
	}
	cred.PolicyHMAC = policyHMAC

	// Ensure directory exists
	if mkdirErr := os.MkdirAll(s.basePath, agentDirPermissions); mkdirErr != nil {
		return fmt.Errorf("creating agents directory: %w", mkdirErr)
	}

	// Marshal and write
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling agent credential: %w", err)
	}

	agentPath := s.agentPath(cred.WalletName, cred.ID)
	if agentPath == "" {
		return fmt.Errorf("%w for wallet %q, id %q", ErrInvalidAgentPath, cred.WalletName, cred.ID)
	}

	if writeErr := fileutil.WriteAtomic(agentPath, data, agentFilePermissions); writeErr != nil {
		return fmt.Errorf("writing agent file: %w", writeErr)
	}

	return nil
}

// Load retrieves and decrypts an agent credential.
// Returns the decrypted seed and credential metadata.
// The caller MUST zero the returned seed when done.
func (s *FileStore) Load(walletName, agentID, token string) ([]byte, *Credential, error) {
	if !walletNameRegex.MatchString(walletName) {
		return nil, nil, fmt.Errorf("%w: %q", ErrInvalidWallet, walletName)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	agentPath := s.agentPath(walletName, agentID)
	if agentPath == "" {
		return nil, nil, ErrInvalidAgentPath
	}

	//nolint:gosec // G304: Path constructed from validated wallet name and agent ID
	data, err := os.ReadFile(agentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: %q for wallet %q", ErrAgentNotFound, agentID, walletName)
		}
		return nil, nil, fmt.Errorf("reading agent file: %w", err)
	}

	var cred Credential
	if unmarshalErr := json.Unmarshal(data, &cred); unmarshalErr != nil {
		return nil, nil, fmt.Errorf("parsing agent file: %w", unmarshalErr)
	}

	// Verify policy HMAC
	valid, err := VerifyPolicyHMAC(&cred.Policy, token, cred.PolicyHMAC)
	if err != nil {
		return nil, nil, fmt.Errorf("verifying policy integrity: %w", err)
	}
	if !valid {
		return nil, nil, ErrPolicyTampered
	}

	// Check expiry
	if cred.IsExpired() {
		return nil, nil, fmt.Errorf("%w: %q", ErrAgentExpired, agentID)
	}

	// Decrypt seed
	seed, err := sigilcrypto.Decrypt(cred.EncryptedSeed, token)
	if err != nil {
		return nil, nil, ErrDecryptFailed
	}

	return seed, &cred, nil
}

// LoadByToken finds the agent credential for a wallet that matches the given token.
// It tries all agent files for the wallet until it finds one that decrypts successfully.
// Returns the decrypted seed and credential.
// The caller MUST zero the returned seed when done.
func (s *FileStore) LoadByToken(walletName, token string) ([]byte, *Credential, error) {
	if !walletNameRegex.MatchString(walletName) {
		return nil, nil, fmt.Errorf("%w: %q", ErrInvalidWallet, walletName)
	}

	// First, try the agent ID derived from the token (fast path).
	agentID := TokenID(token)
	seed, cred, err := s.Load(walletName, agentID, token)
	if err == nil {
		return seed, cred, nil
	}

	// If the fast path fails, scan all agent files for this wallet (slow path).
	// This handles cases where the token ID computation might have changed.
	agents, listErr := s.List(walletName)
	if listErr != nil {
		return nil, nil, fmt.Errorf("%w for wallet %q", ErrTokenNoMatch, walletName)
	}

	for _, a := range agents {
		if a.ID == agentID {
			continue // Already tried
		}
		seed, cred, err = s.Load(walletName, a.ID, token)
		if err == nil {
			return seed, cred, nil
		}
	}

	return nil, nil, fmt.Errorf("%w for wallet %q", ErrTokenNoMatch, walletName)
}

// List returns all agent credentials for a wallet (without decryption).
//
//nolint:gocognit // Iterating and filtering agent files requires multiple checks
func (s *FileStore) List(walletName string) ([]*Credential, error) {
	if !walletNameRegex.MatchString(walletName) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidWallet, walletName)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading agents directory: %w", err)
	}

	prefix := walletName + "-"
	var agents []*Credential
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, agentFileExtension) {
			continue
		}

		agentPath := filepath.Join(s.basePath, name)
		//nolint:gosec // G304: Path constructed from validated wallet name
		data, readErr := os.ReadFile(agentPath)
		if readErr != nil {
			continue
		}

		var cred Credential
		if unmarshalErr := json.Unmarshal(data, &cred); unmarshalErr != nil {
			continue
		}

		agents = append(agents, &cred)
	}

	return agents, nil
}

// Delete removes an agent credential and its counter file.
func (s *FileStore) Delete(walletName, agentID string) error {
	if !walletNameRegex.MatchString(walletName) {
		return fmt.Errorf("%w: %q", ErrInvalidWallet, walletName)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove agent file
	agentPath := s.agentPath(walletName, agentID)
	if agentPath == "" {
		return ErrInvalidAgentPath
	}
	if err := os.Remove(agentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing agent file: %w", err)
	}

	// Remove counter file (best effort)
	counterPath := s.counterPath(walletName, agentID)
	if counterPath != "" {
		_ = os.Remove(counterPath)
	}

	return nil
}

// DeleteAll removes all agent credentials for a wallet.
// Returns the number of agents removed.
func (s *FileStore) DeleteAll(walletName string) (int, error) {
	agents, err := s.List(walletName)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, a := range agents {
		if delErr := s.Delete(walletName, a.ID); delErr == nil {
			count++
		}
	}
	return count, nil
}

// CounterPath returns the counter file path for external access (policy enforcement).
func (s *FileStore) CounterPath(walletName, agentID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.counterPath(walletName, agentID)
}

// agentPath returns the full path for an agent file.
func (s *FileStore) agentPath(walletName, agentID string) string {
	filename := walletName + "-" + agentID + agentFileExtension
	path := filepath.Join(s.basePath, filename)

	// Defensive: prevent path traversal
	cleanPath := filepath.Clean(path)
	expectedSuffix := string(filepath.Separator) + filename
	if !strings.HasSuffix(cleanPath, expectedSuffix) {
		return ""
	}

	return cleanPath
}

// counterPath returns the full path for a daily spending counter file.
func (s *FileStore) counterPath(walletName, agentID string) string {
	filename := walletName + "-" + agentID + ".counter"
	path := filepath.Join(s.basePath, filename)

	cleanPath := filepath.Clean(path)
	expectedSuffix := string(filepath.Separator) + filename
	if !strings.HasSuffix(cleanPath, expectedSuffix) {
		return ""
	}

	return cleanPath
}
