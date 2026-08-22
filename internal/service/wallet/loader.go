package wallet

import (
	"context"
	"fmt"
	"time"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/session"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// LoadContext provides context needed for wallet loading operations.
// This is typically populated from the CLI CommandContext.
type LoadContext struct {
	AgentStore *agent.FileStore
	// YubiKeyStore recovers a seed from a tumbler envelope. Nil disables the
	// YubiKey unlock path (enrolled wallets then fail to load with a clear
	// message rather than silently falling back).
	YubiKeyStore YubiKeyUnlocker
	// OnAuthMessage is called with user-facing messages about authentication.
	// The service calls this instead of writing directly to output.
	OnAuthMessage func(string)
	// OnSessionInfo is called to communicate session info to the caller.
	// This allows the CLI to store agent credentials in CommandContext.
	OnSessionInfo func(*AgentSessionInfo)
}

// AgentSessionInfo contains information about an agent authentication session.
type AgentSessionInfo struct {
	Credential   *agent.Credential
	Token        string
	CounterPath  string
	XpubReadOnly bool   // True for xpub mode
	Xpub         string // Set for xpub mode
}

// Load loads a wallet using the best available authentication method.
// It tries authentication methods in this order:
// 1. Agent token (SIGIL_AGENT_TOKEN environment variable)
// 2. Agent xpub (SIGIL_AGENT_XPUB environment variable - read-only mode)
// 3. Cached session (if sessions are enabled and a valid session exists)
// 4. Password prompt (via req.PasswordFunc)
//
// The caller must zero the seed after use: wallet.ZeroBytes(result.Seed)
//
//nolint:gocognit,gocyclo // Wallet loading requires checking multiple auth methods
func (s *Service) Load(req *LoadRequest, ctx *LoadContext) (*LoadResult, *SessionInfo, error) {
	// Validate wallet exists
	if err := s.ValidateExists(req.Name); err != nil {
		return nil, nil, err
	}

	// Try agent token authentication first
	if token := CheckAgentToken(); token != "" {
		return s.loadWithAgentToken(req.Name, token, ctx)
	}

	// Try xpub read-only mode
	if xpub := CheckAgentXpub(); xpub != "" {
		return s.loadWithXpub(req.Name, xpub, ctx)
	}

	// Try session-based authentication
	sessionEnabled := s.config != nil && s.config.GetSecurity().SessionEnabled
	//nolint:nestif // Session authentication flow requires nested conditionals
	if sessionEnabled && s.sessionMgr != nil && s.sessionMgr.Available() && s.sessionMgr.HasValidSession(req.Name) {
		seed, sess, getErr := s.sessionMgr.GetSession(req.Name)
		if getErr == nil {
			// Load wallet metadata (doesn't require password)
			wlt, loadErr := s.storage.LoadMetadata(req.Name)
			if loadErr != nil {
				wallet.ZeroBytes(seed)
				return nil, nil, loadErr
			}

			if ctx != nil && ctx.OnAuthMessage != nil {
				ctx.OnAuthMessage(fmt.Sprintf("[Using cached session, expires in %s]", formatDuration(sess.TTL())))
			}

			result := &LoadResult{
				Wallet: wlt,
				Seed:   seed,
			}
			sessionInfo := &SessionInfo{
				Mode:      AuthSession,
				ExpiresIn: sess.TTL(),
				Message:   fmt.Sprintf("Using cached session, expires in %s", formatDuration(sess.TTL())),
			}
			return result, sessionInfo, nil
		}
		// Session invalid or error - fall through to password prompt
	}

	// Try YubiKey envelope authentication (after the session cache so a cached
	// session unlocks with no touch, before the plain-password fallback).
	if policy, hasEnv, polErr := s.storage.LoadAuthPolicy(req.Name); polErr == nil && hasEnv {
		return s.loadWithYubiKey(req.Name, policy, req, ctx, sessionEnabled)
	}

	// Fall back to password-based authentication
	if req.PasswordFunc == nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"no password function provided for authentication",
		)
	}

	password, promptErr := req.PasswordFunc("Enter wallet password: ")
	if promptErr != nil {
		return nil, nil, promptErr
	}
	defer wallet.ZeroBytes([]byte(password))

	// Load wallet with password
	wlt, seed, loadErr := s.storage.Load(req.Name, []byte(password))
	if loadErr != nil {
		return nil, nil, loadErr
	}

	// Start a new session if sessions are enabled
	//nolint:nestif // Session creation flow requires nested conditionals
	if sessionEnabled && s.sessionMgr != nil && s.sessionMgr.Available() {
		ttl := time.Duration(s.config.GetSecurity().SessionTTLMinutes) * time.Minute
		if ttl < session.MinTTL {
			ttl = session.DefaultTTL
		}

		if startErr := s.sessionMgr.StartSession(req.Name, seed, ttl); startErr != nil {
			// Log warning but don't fail - user can still proceed without session
			if s.logger != nil {
				s.logger.Debug("failed to start session: %v", startErr)
			}
		} else {
			if ctx != nil && ctx.OnAuthMessage != nil {
				ctx.OnAuthMessage(fmt.Sprintf("[Session started, expires in %s]", formatDuration(ttl)))
			}
		}
	}

	result := &LoadResult{
		Wallet: wlt,
		Seed:   seed,
	}
	sessionInfo := &SessionInfo{
		Mode:    AuthPassword,
		Message: "Authenticated with password",
	}
	return result, sessionInfo, nil
}

// loadWithAgentToken authenticates using an agent token from SIGIL_AGENT_TOKEN.
// Finds the matching agent file, decrypts the seed, validates expiry and policy.
func (s *Service) loadWithAgentToken(name, token string, ctx *LoadContext) (*LoadResult, *SessionInfo, error) {
	if ctx == nil || ctx.AgentStore == nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenInvalid,
			"agent store not initialized",
		)
	}

	// Load and decrypt using the agent token
	seed, cred, err := ctx.AgentStore.LoadByToken(name, token)
	if err != nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenInvalid,
			fmt.Sprintf("agent token does not match any agent for wallet '%s'. "+
				"Create one with: sigil agent create --wallet %s", name, name),
		)
	}

	// Check expiry
	if cred.IsExpired() {
		wallet.ZeroBytes(seed)
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrAgentTokenExpired,
			fmt.Sprintf("agent '%s' expired at %s. Create a new agent with: sigil agent create --wallet %s",
				cred.ID, cred.ExpiresAt.Format(time.RFC3339), name),
		)
	}

	// Load wallet metadata (doesn't require password)
	wlt, loadErr := s.storage.LoadMetadata(name)
	if loadErr != nil {
		wallet.ZeroBytes(seed)
		return nil, nil, loadErr
	}

	// Notify caller about agent session info (for policy enforcement)
	if ctx.OnSessionInfo != nil {
		ctx.OnSessionInfo(&AgentSessionInfo{
			Credential:  cred,
			Token:       token,
			CounterPath: ctx.AgentStore.CounterPath(name, cred.ID),
		})
	}

	if ctx.OnAuthMessage != nil {
		ctx.OnAuthMessage(fmt.Sprintf("[Agent '%s' (%s), expires in %s]", cred.Label, cred.ID, formatDuration(cred.TTL())))
	}

	result := &LoadResult{
		Wallet: wlt,
		Seed:   seed,
	}
	sessionInfo := &SessionInfo{
		Mode:      AuthAgentToken,
		ExpiresIn: cred.TTL(),
		Message:   fmt.Sprintf("Agent '%s' (%s), expires in %s", cred.Label, cred.ID, formatDuration(cred.TTL())),
	}
	return result, sessionInfo, nil
}

// loadWithYubiKey recovers the seed via the wallet's tumbler envelope. It
// prompts for the password only if the envelope's policy requires it (the
// store decides), then touch happens inside the transport. On success it
// starts a session so subsequent commands need neither password nor touch
// until the session expires.
func (s *Service) loadWithYubiKey(name, policy string, req *LoadRequest, ctx *LoadContext, sessionEnabled bool) (*LoadResult, *SessionInfo, error) {
	if ctx == nil || ctx.YubiKeyStore == nil {
		return nil, nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("wallet '%s' requires a YubiKey but YubiKey support is not initialized "+
				"(install ykman and enable security.yubikey_enabled)", name),
		)
	}

	envelope, _, envErr := s.storage.LoadEnvelope(name)
	if envErr != nil {
		return nil, nil, envErr
	}

	// The store invokes this only when the policy needs a password.
	passwordFn := func() ([]byte, error) {
		if req.PasswordFunc == nil {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput, "no password function provided for authentication",
			)
		}
		pw, promptErr := req.PasswordFunc("Enter wallet password: ")
		if promptErr != nil {
			return nil, promptErr
		}
		return []byte(pw), nil
	}

	if ctx.OnAuthMessage != nil {
		ctx.OnAuthMessage("[Unlocking with YubiKey…]")
	}

	seed, unlockErr := ctx.YubiKeyStore.Unlock(context.Background(), envelope, passwordFn)
	if unlockErr != nil {
		return nil, nil, unlockErr
	}

	wlt, loadErr := s.storage.LoadMetadata(name)
	if loadErr != nil {
		wallet.ZeroBytes(seed)
		return nil, nil, loadErr
	}

	s.maybeStartSession(name, seed, sessionEnabled, ctx)

	result := &LoadResult{
		Wallet: wlt,
		Seed:   seed,
	}
	sessionInfo := &SessionInfo{
		Mode:    AuthYubiKey,
		Message: fmt.Sprintf("Authenticated with YubiKey (%s)", policy),
	}
	return result, sessionInfo, nil
}

// maybeStartSession caches the seed for the wallet if sessions are enabled and
// available, so subsequent commands avoid re-authentication. Best-effort:
// failures are logged, not fatal.
func (s *Service) maybeStartSession(name string, seed []byte, sessionEnabled bool, ctx *LoadContext) {
	if !sessionEnabled || s.sessionMgr == nil || !s.sessionMgr.Available() {
		return
	}
	ttl := time.Duration(s.config.GetSecurity().SessionTTLMinutes) * time.Minute
	if ttl < session.MinTTL {
		ttl = session.DefaultTTL
	}
	if startErr := s.sessionMgr.StartSession(name, seed, ttl); startErr != nil {
		if s.logger != nil {
			s.logger.Debug("failed to start session: %v", startErr)
		}
		return
	}
	if ctx != nil && ctx.OnAuthMessage != nil {
		ctx.OnAuthMessage(fmt.Sprintf("[Session started, expires in %s]", formatDuration(ttl)))
	}
}

// loadWithXpub creates a read-only wallet context using an xpub from SIGIL_AGENT_XPUB.
// Returns the wallet metadata with a nil seed — only balance and receive operations are supported.
// Spending operations must detect the nil seed and return ErrAgentXpubWriteDenied.
func (s *Service) loadWithXpub(name, xpub string, ctx *LoadContext) (*LoadResult, *SessionInfo, error) {
	// Load wallet metadata (doesn't require password)
	wlt, loadErr := s.storage.LoadMetadata(name)
	if loadErr != nil {
		return nil, nil, loadErr
	}

	// Notify caller about xpub mode (for context storage)
	if ctx != nil && ctx.OnSessionInfo != nil {
		ctx.OnSessionInfo(&AgentSessionInfo{
			XpubReadOnly: true,
			Xpub:         xpub,
		})
	}

	if ctx != nil && ctx.OnAuthMessage != nil {
		ctx.OnAuthMessage("[xpub read-only mode — spending operations disabled]")
	}

	// Return nil seed (read-only mode — no private key access)
	result := &LoadResult{
		Wallet: wlt,
		Seed:   nil,
	}
	sessionInfo := &SessionInfo{
		Mode:    AuthXpub,
		Message: "xpub read-only mode — spending operations disabled",
	}
	return result, sessionInfo, nil
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	days := hours / 24
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
