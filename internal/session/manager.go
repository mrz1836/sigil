package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/fileutil"
	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

// walletNameRegex mirrors wallet.walletNameRegex for input validation at the
// session boundary without importing the wallet package.
var walletNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// errInvalidWalletName is returned when a wallet name fails validation.
var errInvalidWalletName = fmt.Errorf("invalid wallet name")

const (
	// sessionFileExtension is the extension for session files.
	sessionFileExtension = ".session"

	// sessionFilePermissions is the permission mode for session files.
	sessionFilePermissions = 0o600

	// sessionDirPermissions is the permission mode for the sessions directory.
	sessionDirPermissions = 0o700

	// sessionKeyLength is the length of the random session key in bytes.
	sessionKeyLength = 32
)

// sessionFile represents the encrypted session file structure.
type sessionFile struct {
	// Session contains the session metadata.
	Session *Session `json:"session"`

	// EncryptedSeed is the session-key-encrypted seed bytes.
	EncryptedSeed []byte `json:"encrypted_seed"`
}

// FileManager implements the Manager interface using files and OS keyring.
type FileManager struct {
	basePath  string
	keyring   Keyring
	available bool
	mu        sync.RWMutex
}

// NewManager creates a new session manager.
// If keyring is nil, it uses the OS keyring.
// The manager probes the keyring on creation to determine availability.
func NewManager(basePath string, keyring Keyring) *FileManager {
	if keyring == nil {
		keyring = NewOSKeyring()
	}

	m := &FileManager{
		basePath:  basePath,
		keyring:   keyring,
		available: false,
	}

	// Probe keyring availability
	m.available = m.probeKeyring()

	return m
}

// Available returns true if session caching is available.
func (m *FileManager) Available() bool {
	return m.available
}

// StartSession creates a new session for the wallet.
//
//nolint:gocyclo // Sequential validation and error-handling steps are inherent to the operation
func (m *FileManager) StartSession(wallet string, seed []byte, ttl time.Duration) error {
	if !walletNameRegex.MatchString(wallet) {
		return errInvalidWalletName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.available {
		return ErrKeyringUnavailable
	}

	// Validate TTL
	if ttl < MinTTL {
		ttl = MinTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}

	// Generate a random session key
	sessionKey := make([]byte, sessionKeyLength)
	if _, err := rand.Read(sessionKey); err != nil {
		return fmt.Errorf("generating session key: %w", err)
	}
	defer zeroBytes(sessionKey)

	// Encrypt the seed with the session key
	encryptedSeed, err := sigilcrypto.Encrypt(seed, hex.EncodeToString(sessionKey))
	if err != nil {
		return fmt.Errorf("encrypting seed: %w", err)
	}

	// Store the session key in the keyring
	keyringKey := m.keyringKey(wallet)
	encodedKey := base64.StdEncoding.EncodeToString(sessionKey)
	if setErr := m.keyring.Set(ServiceName, keyringKey, encodedKey); setErr != nil {
		return fmt.Errorf("storing session key in keyring: %w", setErr)
	}

	// Create session metadata
	now := time.Now()
	session := &Session{
		WalletName: wallet,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
	}

	// Create session file structure
	sf := sessionFile{
		Session:       session,
		EncryptedSeed: encryptedSeed,
	}

	// Ensure sessions directory exists
	if mkdirErr := os.MkdirAll(m.basePath, sessionDirPermissions); mkdirErr != nil {
		// Clean up keyring entry on failure
		_ = m.keyring.Delete(ServiceName, keyringKey)
		return fmt.Errorf("creating sessions directory: %w", mkdirErr)
	}

	// Write session file
	data, marshalErr := json.MarshalIndent(sf, "", "  ")
	if marshalErr != nil {
		_ = m.keyring.Delete(ServiceName, keyringKey)
		return fmt.Errorf("marshaling session: %w", marshalErr)
	}

	sessionPath := m.sessionPath(wallet)
	if writeErr := fileutil.WriteAtomic(sessionPath, data, sessionFilePermissions); writeErr != nil {
		_ = m.keyring.Delete(ServiceName, keyringKey)
		return fmt.Errorf("writing session file: %w", writeErr)
	}

	return nil
}

// GetSession retrieves the decrypted seed for an active session.
func (m *FileManager) GetSession(wallet string) ([]byte, *Session, error) {
	if !walletNameRegex.MatchString(wallet) {
		return nil, nil, errInvalidWalletName
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.available {
		return nil, nil, ErrKeyringUnavailable
	}

	// Read session file
	sessionPath := m.sessionPath(wallet)
	// SECURITY: Path is safe because sessionPath uses filepath.Join
	// and wallet name is from internal session storage
	//nolint:gosec // G304: Path constructed from internal session path
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrSessionNotFound
		}
		return nil, nil, fmt.Errorf("reading session file: %w", err)
	}

	// Parse session file
	var sf sessionFile
	if unmarshalErr := json.Unmarshal(data, &sf); unmarshalErr != nil {
		// Corrupted session file - clean up
		_ = m.cleanupSession(wallet)
		return nil, nil, ErrSessionCorrupted
	}

	// Check if session has expired
	if !sf.Session.IsValid() {
		_ = m.cleanupSession(wallet)
		return nil, nil, ErrSessionExpired
	}

	// Get session key from keyring
	keyringKey := m.keyringKey(wallet)
	encodedKey, getErr := m.keyring.Get(ServiceName, keyringKey)
	if getErr != nil {
		// Keyring entry missing but session file exists - clean up
		_ = m.cleanupSession(wallet)
		return nil, nil, ErrSessionNotFound
	}

	// Decode and decrypt
	sessionKey, decodeErr := base64.StdEncoding.DecodeString(encodedKey)
	if decodeErr != nil {
		_ = m.cleanupSession(wallet)
		return nil, nil, ErrSessionCorrupted
	}
	defer zeroBytes(sessionKey)

	seed, decryptErr := sigilcrypto.Decrypt(sf.EncryptedSeed, hex.EncodeToString(sessionKey))
	if decryptErr != nil {
		_ = m.cleanupSession(wallet)
		return nil, nil, ErrSessionCorrupted
	}

	return seed, sf.Session, nil
}

// HasValidSession returns true if a valid session exists for the wallet.
func (m *FileManager) HasValidSession(wallet string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.available {
		return false
	}

	// Check if session file exists
	sessionPath := m.sessionPath(wallet)
	//nolint:gosec // G304: Path constructed from internal session path
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return false
	}

	// Parse and check expiry
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return false
	}

	return sf.Session.IsValid()
}

// EndSession removes the session for a wallet.
func (m *FileManager) EndSession(wallet string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cleanupSession(wallet)
}

// EndAllSessions removes all active sessions and returns the count.
func (m *FileManager) EndAllSessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions, err := m.listSessionsLocked()
	if err != nil {
		return 0
	}

	count := 0
	for _, sess := range sessions {
		if cleanupErr := m.cleanupSession(sess.WalletName); cleanupErr == nil {
			count++
		}
	}

	return count
}

// ListSessions returns all active sessions.
func (m *FileManager) ListSessions() ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.listSessionsLocked()
}

// probeKeyringTimeout is the maximum time to wait for a keyring probe.
// Prevents CLI startup from blocking if the OS keyring daemon is slow or hung.
const probeKeyringTimeout = 3 * time.Second

// probeKeyring tests if the keyring is available, with a timeout to prevent
// blocking CLI startup if the OS keyring daemon is unresponsive.
func (m *FileManager) probeKeyring() bool {
	ch := make(chan bool, 1)
	go func() {
		ch <- m.probeKeyringSync()
	}()

	select {
	case result := <-ch:
		return result
	case <-time.After(probeKeyringTimeout):
		return false
	}
}

// probeKeyringSync performs the actual synchronous keyring probe.
func (m *FileManager) probeKeyringSync() bool {
	const (
		testService = "sigil-probe"
		testUser    = "probe"
		testValue   = "test"
	)

	// Try to set a test value
	if err := m.keyring.Set(testService, testUser, testValue); err != nil {
		return false
	}

	// Try to get the test value
	val, err := m.keyring.Get(testService, testUser)
	if err != nil || val != testValue {
		_ = m.keyring.Delete(testService, testUser)
		return false
	}

	// Clean up the test value
	if err := m.keyring.Delete(testService, testUser); err != nil {
		return false
	}

	return true
}

// listSessionsLocked returns all active sessions (must be called with lock held).
//
//nolint:gocognit // Iterating sessions requires multiple checks
func (m *FileManager) listSessionsLocked() ([]*Session, error) {
	if !m.available {
		return nil, ErrKeyringUnavailable
	}

	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, sessionFileExtension) {
			continue
		}

		walletName := strings.TrimSuffix(name, sessionFileExtension)
		sessionPath := m.sessionPath(walletName)

		//nolint:gosec // G304: Path constructed from internal session path
		data, readErr := os.ReadFile(sessionPath)
		if readErr != nil {
			continue
		}

		var sf sessionFile
		if unmarshalErr := json.Unmarshal(data, &sf); unmarshalErr != nil {
			continue
		}

		// Only include valid (non-expired) sessions
		if sf.Session.IsValid() {
			sessions = append(sessions, sf.Session)
		}
	}

	return sessions, nil
}

// cleanupSession removes both the session file and keyring entry.
// Must be called with appropriate lock held.
func (m *FileManager) cleanupSession(wallet string) error {
	keyringKey := m.keyringKey(wallet)
	sessionPath := m.sessionPath(wallet)

	// Remove keyring entry (ignore errors)
	_ = m.keyring.Delete(ServiceName, keyringKey)

	// Remove session file
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing session file: %w", err)
	}

	return nil
}

// keyringKey returns the keyring key for a wallet.
func (m *FileManager) keyringKey(wallet string) string {
	return "wallet:" + wallet
}

// sessionPath returns the full path for a session file.
func (m *FileManager) sessionPath(wallet string) string {
	path := filepath.Join(m.basePath, wallet+sessionFileExtension)

	// Defensive check: ensure no directory traversal
	cleanPath := filepath.Clean(path)
	expectedSuffix := string(filepath.Separator) + wallet + sessionFileExtension
	if !strings.HasSuffix(cleanPath, expectedSuffix) {
		return ""
	}

	return cleanPath
}

// zeroBytes securely zeros a byte slice.
// runtime.KeepAlive prevents the compiler from optimizing away the zeroing
// as a dead store when the slice is not used afterward.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
