package session

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

func TestMain(m *testing.M) {
	sigilcrypto.SetScryptWorkFactor(10) // Fast for tests
	os.Exit(m.Run())
}

// MockKeyring is a mock implementation of the Keyring interface for testing.
type MockKeyring struct {
	mu      sync.Mutex
	store   map[string]string
	setErr  error
	getErr  error
	delErr  error
	getCt   int
	setCt   int
	delCt   int
	failing bool
}

// NewMockKeyring creates a new mock keyring.
func NewMockKeyring() *MockKeyring {
	return &MockKeyring{
		store: make(map[string]string),
	}
}

// Set stores a secret in the mock keyring.
func (m *MockKeyring) Set(service, user, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCt++
	if m.setErr != nil {
		return m.setErr
	}
	if m.failing {
		return ErrKeyringUnavailable
	}
	m.store[service+":"+user] = password
	return nil
}

// Get retrieves a secret from the mock keyring.
func (m *MockKeyring) Get(service, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCt++
	if m.getErr != nil {
		return "", m.getErr
	}
	if m.failing {
		return "", ErrKeyringUnavailable
	}
	val, ok := m.store[service+":"+user]
	if !ok {
		return "", ErrSessionNotFound
	}
	return val, nil
}

// Delete removes a secret from the mock keyring.
func (m *MockKeyring) Delete(service, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delCt++
	if m.delErr != nil {
		return m.delErr
	}
	if m.failing {
		return ErrKeyringUnavailable
	}
	delete(m.store, service+":"+user)
	return nil
}

// SetFailing makes the keyring fail all operations.
func (m *MockKeyring) SetFailing(failing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failing = failing
}

// Reset clears the mock keyring state.
func (m *MockKeyring) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = make(map[string]string)
	m.setErr = nil
	m.getErr = nil
	m.delErr = nil
	m.getCt = 0
	m.setCt = 0
	m.delCt = 0
	m.failing = false
}

func TestManager_Available(t *testing.T) {
	t.Parallel()
	t.Run("available with working keyring", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		if !m.Available() {
			t.Error("Expected Available() to return true with working keyring")
		}
	})

	t.Run("unavailable with failing keyring", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		mock.SetFailing(true)
		m := NewManager(tmpDir, mock)

		if m.Available() {
			t.Error("Expected Available() to return false with failing keyring")
		}
	})
}

//nolint:gocognit,gocyclo // Test functions with multiple subtests are inherently complex
func TestManager_StartSession(t *testing.T) {
	t.Parallel()
	t.Run("successful session creation", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 15*time.Minute)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		// Verify session file was created
		sessionPath := filepath.Join(tmpDir, "main.session")
		if _, statErr := os.Stat(sessionPath); os.IsNotExist(statErr) {
			t.Error("Session file was not created")
		}

		// Verify file permissions
		info, statErr := os.Stat(sessionPath)
		if statErr != nil {
			t.Fatalf("Failed to stat session file: %v", statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("Session file permissions = %v, want 0600", info.Mode().Perm())
		}

		// Verify keyring entry was created
		if mock.setCt < 1 {
			t.Error("Keyring Set() was not called")
		}
	})

	t.Run("keyring unavailable", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		mock.SetFailing(true)
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data")
		err := m.StartSession("main", seed, 15*time.Minute)
		if !errors.Is(err, ErrKeyringUnavailable) {
			t.Errorf("StartSession() error = %v, want ErrKeyringUnavailable", err)
		}
	})

	t.Run("TTL clamping - too short", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 10*time.Second) // Below MinTTL
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		// Verify session was created with clamped TTL
		_, session, err := m.GetSession("main")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}

		// Check that TTL is positive and not more than MinTTL (1 minute)
		// We can't check exact range since parallel tests may have delayed this
		ttl := session.TTL()
		if ttl <= 0 || ttl > MinTTL+time.Second {
			t.Errorf("Session TTL = %v, expected > 0 and <= %v (clamped)", ttl, MinTTL)
		}
	})

	t.Run("TTL clamping - too long", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 2*time.Hour) // Above MaxTTL
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		_, session, err := m.GetSession("main")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}

		// Check that TTL is positive and not more than MaxTTL (1 hour)
		// We can't check exact range since parallel tests may have delayed this
		ttl := session.TTL()
		if ttl <= 0 || ttl > MaxTTL+time.Second {
			t.Errorf("Session TTL = %v, expected > 0 and <= %v (clamped)", ttl, MaxTTL)
		}
	})
}

//nolint:gocognit,gocyclo // Test functions with multiple subtests are inherently complex
func TestManager_GetSession(t *testing.T) {
	t.Parallel()
	t.Run("retrieve valid session", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		originalSeed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", originalSeed, 15*time.Minute)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		seed, session, err := m.GetSession("main")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}

		if string(seed) != string(originalSeed) {
			t.Errorf("GetSession() seed = %s, want %s", string(seed), string(originalSeed))
		}

		if session.WalletName != "main" {
			t.Errorf("GetSession() wallet = %s, want main", session.WalletName)
		}

		if !session.IsValid() {
			t.Error("GetSession() returned invalid session")
		}
	})

	t.Run("session not found", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		_, _, err := m.GetSession("nonexistent")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("GetSession() error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("expired session", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		// Create session with very short TTL
		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, MinTTL)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		// Manually modify session file to expire it
		sessionPath := filepath.Join(tmpDir, "main.session")
		//nolint:gosec // G304: Test file path is from controlled test input
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			t.Fatalf("Failed to read session file: %v", err)
		}

		// Replace expiry time with past time
		modifiedData := string(data)
		// This is a bit hacky but works for testing
		now := time.Now().Add(-1 * time.Hour)
		expiredSession := `"expires_at": "` + now.Format(time.RFC3339Nano) + `"`
		// Find and replace the expires_at field
		startIdx := 0
		for i := 0; i < len(modifiedData)-12; i++ {
			if modifiedData[i:i+12] == `"expires_at"` {
				startIdx = i
				break
			}
		}
		if startIdx > 0 {
			endIdx := startIdx
			for i := startIdx; i < len(modifiedData); i++ {
				if modifiedData[i] == ',' || modifiedData[i] == '}' {
					endIdx = i
					break
				}
			}
			modifiedData = modifiedData[:startIdx] + expiredSession + modifiedData[endIdx:]
		}

		err = os.WriteFile(sessionPath, []byte(modifiedData), 0o600)
		if err != nil {
			t.Fatalf("Failed to write modified session file: %v", err)
		}

		_, _, err = m.GetSession("main")
		if !errors.Is(err, ErrSessionExpired) {
			t.Errorf("GetSession() error = %v, want ErrSessionExpired", err)
		}

		// Verify session was cleaned up
		if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
			t.Error("Expired session file was not cleaned up")
		}
	})

	t.Run("corrupted session file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		// Create a corrupted session file
		sessionPath := filepath.Join(tmpDir, "main.session")
		err := os.MkdirAll(tmpDir, 0o700)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(sessionPath, []byte("invalid json"), 0o600)
		if err != nil {
			t.Fatalf("Failed to write corrupted file: %v", err)
		}

		_, _, err = m.GetSession("main")
		if !errors.Is(err, ErrSessionCorrupted) {
			t.Errorf("GetSession() error = %v, want ErrSessionCorrupted", err)
		}
	})

	t.Run("missing keyring entry", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		// Create a valid session
		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 15*time.Minute)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		// Delete the keyring entry directly
		_ = mock.Delete(ServiceName, "wallet:main")

		_, _, err = m.GetSession("main")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("GetSession() error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestManager_HasValidSession(t *testing.T) {
	t.Parallel()
	t.Run("valid session exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 15*time.Minute)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		if !m.HasValidSession("main") {
			t.Error("HasValidSession() = false, want true")
		}
	})

	t.Run("no session exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		if m.HasValidSession("nonexistent") {
			t.Error("HasValidSession() = true, want false")
		}
	})

	t.Run("keyring unavailable", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		mock.SetFailing(true)
		m := NewManager(tmpDir, mock)

		if m.HasValidSession("main") {
			t.Error("HasValidSession() = true, want false when keyring unavailable")
		}
	})
}

func TestManager_EndSession(t *testing.T) {
	t.Parallel()
	t.Run("end existing session", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		seed := []byte("test-seed-data-32-bytes-long!!!!")
		err := m.StartSession("main", seed, 15*time.Minute)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}

		err = m.EndSession("main")
		if err != nil {
			t.Fatalf("EndSession() error = %v", err)
		}

		// Verify session is gone
		if m.HasValidSession("main") {
			t.Error("Session still exists after EndSession()")
		}

		// Verify files are cleaned up
		sessionPath := filepath.Join(tmpDir, "main.session")
		if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
			t.Error("Session file was not removed")
		}
	})

	t.Run("end nonexistent session - no error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		err := m.EndSession("nonexistent")
		if err != nil {
			t.Errorf("EndSession() error = %v, want nil for nonexistent session", err)
		}
	})
}

//nolint:gocognit // Test functions with multiple subtests are inherently complex
func TestManager_EndAllSessions(t *testing.T) {
	t.Parallel()
	t.Run("end multiple sessions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		// Create multiple sessions
		seed := []byte("test-seed-data-32-bytes-long!!!!")
		for _, name := range []string{"main", "backup", "test"} {
			err := m.StartSession(name, seed, 15*time.Minute)
			if err != nil {
				t.Fatalf("StartSession(%s) error = %v", name, err)
			}
		}

		count := m.EndAllSessions()
		if count != 3 {
			t.Errorf("EndAllSessions() = %d, want 3", count)
		}

		// Verify all sessions are gone
		for _, name := range []string{"main", "backup", "test"} {
			if m.HasValidSession(name) {
				t.Errorf("Session %s still exists after EndAllSessions()", name)
			}
		}
	})

	t.Run("no sessions to end", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		count := m.EndAllSessions()
		if count != 0 {
			t.Errorf("EndAllSessions() = %d, want 0", count)
		}
	})
}

//nolint:gocognit,gocyclo // Test functions with multiple subtests are inherently complex
func TestManager_ListSessions(t *testing.T) {
	t.Parallel()
	t.Run("list multiple sessions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		// Create multiple sessions
		seed := []byte("test-seed-data-32-bytes-long!!!!")
		wallets := []string{"main", "backup"}
		for _, name := range wallets {
			err := m.StartSession(name, seed, 15*time.Minute)
			if err != nil {
				t.Fatalf("StartSession(%s) error = %v", name, err)
			}
		}

		sessions, err := m.ListSessions()
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(sessions) != 2 {
			t.Errorf("ListSessions() returned %d sessions, want 2", len(sessions))
		}

		// Check that both wallets are listed
		found := make(map[string]bool)
		for _, s := range sessions {
			found[s.WalletName] = true
		}
		for _, name := range wallets {
			if !found[name] {
				t.Errorf("ListSessions() missing wallet %s", name)
			}
		}
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		m := NewManager(tmpDir, mock)

		sessions, err := m.ListSessions()
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(sessions) != 0 {
			t.Errorf("ListSessions() returned %d sessions, want 0", len(sessions))
		}
	})

	t.Run("keyring unavailable", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		mock := NewMockKeyring()
		mock.SetFailing(true)
		m := NewManager(tmpDir, mock)

		_, err := m.ListSessions()
		if !errors.Is(err, ErrKeyringUnavailable) {
			t.Errorf("ListSessions() error = %v, want ErrKeyringUnavailable", err)
		}
	})
}

//nolint:gocognit // Concurrent access test needs multiple goroutines
func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	mock := NewMockKeyring()
	m := NewManager(tmpDir, mock)

	seed := []byte("test-seed-data-32-bytes-long!!!!")

	// Start a session
	err := m.StartSession("main", seed, 15*time.Minute)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := m.GetSession("main")
			if err != nil {
				errCh <- err
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.HasValidSession("main")
		}()
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.ListSessions()
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

func TestManager_SessionFilePermissions(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	mock := NewMockKeyring()
	m := NewManager(tmpDir, mock)

	seed := []byte("test-seed-data-32-bytes-long!!!!")
	err := m.StartSession("main", seed, 15*time.Minute)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	sessionPath := filepath.Join(tmpDir, "main.session")
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("Failed to stat session file: %v", err)
	}

	// Check permissions are 0600
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("Session file permissions = %o, want 0600", perm)
	}
}
