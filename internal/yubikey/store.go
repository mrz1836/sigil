// Package yubikey wires sigil's wallet seed handling to the go-tumbler
// envelope: it builds keyslot envelopes at enroll time and recovers the seed
// at unlock time, delegating raw YubiKey I/O to a tumbler transport (the
// official ykman CLI in production, a deterministic fake in tests).
//
// The seed crosses this boundary as []byte to match sigil's existing wallet
// plumbing; inside tumbler every secret lives in mlocked securebytes.
package yubikey

import (
	"context"
	"errors"
	"fmt"

	tumbler "github.com/mrz1836/go-tumbler"
	"github.com/mrz1836/go-tumbler/securebytes"
	"github.com/mrz1836/go-tumbler/transport"
)

// scryptLogN pins the password factor's scrypt cost to age's secure default
// (logN=18), preserving sigil's existing password-cracking resistance.
const scryptLogN = 18

// defaultOTPSlot is the YubiKey OTP slot used when none is configured.
const defaultOTPSlot uint8 = 2

// Sentinel errors.
var (
	// ErrPasswordRequired indicates a password-and-yubikey operation was
	// attempted without a password.
	ErrPasswordRequired = errors.New("yubikey: password required for password-and-yubikey policy")

	// ErrUnsupportedPolicy indicates a policy this store does not handle
	// (password-only wallets never reach the yubikey store).
	ErrUnsupportedPolicy = errors.New("yubikey: unsupported policy")
)

// Store builds and unlocks tumbler envelopes that wrap a wallet seed.
type Store struct {
	transport transport.Transport
	slot      uint8
}

// NewStore builds a store from an explicit transport. Tests inject a
// transport.FakeTransport; production wiring uses NewStoreFromConfig.
func NewStore(t transport.Transport, slot uint8) *Store {
	if slot == 0 {
		slot = defaultOTPSlot
	}
	return &Store{transport: t, slot: slot}
}

// NewStoreFromConfig builds a store backed by the real ykman CLI. It returns
// an error if ykman is missing or too old (surfaced to the user at enroll).
func NewStoreFromConfig(ykmanPath string, slot uint8) (*Store, error) {
	t, err := transport.NewYkmanTransport(ykmanPath)
	if err != nil {
		return nil, err
	}
	return NewStore(t, slot), nil
}

// EnrollOptions controls the primary factor and any extra slots.
type EnrollOptions struct {
	// Password is required for PolicyPasswordAndYubiKey and ignored for
	// PolicyYubiKeyOnly. The caller zeroes it after Enroll returns.
	Password []byte
	// WithRecovery also enrolls a printed 256-bit recovery code.
	WithRecovery bool
}

// EnrollResult carries the marshaled envelope and, if requested, the one-time
// recovery code to display (never persisted).
type EnrollResult struct {
	Envelope     []byte
	RecoveryCode string
}

// Enroll wraps seed under policy and returns the marshaled envelope. The
// caller retains ownership of seed and opts.Password and must zero them.
func (s *Store) Enroll(ctx context.Context, seed []byte, policy tumbler.Policy, opts EnrollOptions) (*EnrollResult, error) {
	dek, err := securebytes.New(cloneBytes(seed))
	if err != nil {
		return nil, fmt.Errorf("yubikey: wrap seed: %w", err)
	}
	defer func() { _ = dek.Destroy() }()

	method, pwSB, err := s.primaryMethod(policy, opts.Password)
	if err != nil {
		return nil, err
	}
	if pwSB != nil {
		defer func() { _ = pwSB.Destroy() }()
	}

	env, err := tumbler.NewEnvelope(ctx, dek, policy, method)
	if err != nil {
		return nil, fmt.Errorf("yubikey: build envelope: %w", err)
	}

	res := &EnrollResult{}
	if opts.WithRecovery {
		code, genErr := tumbler.GenerateRecoveryCode()
		if genErr != nil {
			return nil, genErr
		}
		defer func() { _ = code.Destroy() }()
		if addErr := env.AddSlot(ctx, dek, tumbler.NewRecoveryMethod(code)); addErr != nil {
			return nil, addErr
		}
		printed, fmtErr := tumbler.FormatRecoveryCode(code)
		if fmtErr != nil {
			return nil, fmtErr
		}
		res.RecoveryCode = printed
	}

	blob, err := env.Marshal()
	if err != nil {
		return nil, fmt.Errorf("yubikey: marshal envelope: %w", err)
	}
	res.Envelope = blob
	return res, nil
}

// AddBackupKey enrolls an additional YubiKey slot (a spare key) into an
// existing envelope, returning the updated marshaled envelope. seed must be
// the wallet seed recovered via an existing method.
func (s *Store) AddBackupKey(ctx context.Context, envelope, seed, password []byte, label string) ([]byte, error) {
	env, err := tumbler.ParseEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	dek, err := securebytes.New(cloneBytes(seed))
	if err != nil {
		return nil, err
	}
	defer func() { _ = dek.Destroy() }()

	var labelOpts []tumbler.Option
	if label != "" {
		labelOpts = append(labelOpts, tumbler.WithLabel(label))
	}
	method, pwSB, err := s.primaryMethod(env.EffectivePolicy(), password, labelOpts...)
	if err != nil {
		return nil, err
	}
	if pwSB != nil {
		defer func() { _ = pwSB.Destroy() }()
	}
	if addErr := env.AddSlot(ctx, dek, method); addErr != nil {
		return nil, addErr
	}
	return env.Marshal()
}

// Unlock parses envelope and recovers the seed using the primary factor for
// the envelope's effective policy. passwordFn is called only when the policy
// needs a password. The returned seed is a fresh []byte the caller must zero.
func (s *Store) Unlock(ctx context.Context, envelope []byte, passwordFn func() ([]byte, error)) ([]byte, error) {
	env, err := tumbler.ParseEnvelope(envelope)
	if err != nil {
		return nil, err
	}

	method, pwSB, err := s.unlockMethod(env.EffectivePolicy(), passwordFn)
	if err != nil {
		return nil, err
	}
	if pwSB != nil {
		defer func() { _ = pwSB.Destroy() }()
	}

	return unlockToSeed(ctx, env, method)
}

// UnlockWithRecovery recovers the seed using a printed recovery code, for the
// lockout escape hatch. The returned seed is a fresh []byte the caller zeroes.
func (s *Store) UnlockWithRecovery(ctx context.Context, envelope []byte, recoveryCode string) ([]byte, error) {
	env, err := tumbler.ParseEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	code, err := tumbler.ParseRecoveryCode(recoveryCode)
	if err != nil {
		return nil, err
	}
	defer func() { _ = code.Destroy() }()

	return unlockToSeed(ctx, env, tumbler.NewRecoveryMethod(code))
}

// EffectivePolicy parses the envelope and returns its enforced policy.
func (s *Store) EffectivePolicy(envelope []byte) (tumbler.Policy, error) {
	env, err := tumbler.ParseEnvelope(envelope)
	if err != nil {
		return tumbler.PolicyInvalid, err
	}
	return env.EffectivePolicy(), nil
}

// primaryMethod builds the primary enroll/unlock method for a policy, plus the
// owned password SecureBytes (nil for yubikey-only) the caller must Destroy.
func (s *Store) primaryMethod(policy tumbler.Policy, password []byte, opts ...tumbler.Option) (tumbler.Method, *securebytes.SecureBytes, error) {
	switch policy {
	case tumbler.PolicyYubiKeyOnly:
		return tumbler.NewYubiKeyMethod(s.transport, tumbler.YubiKeyConfig{Slot: s.slot}, nil, opts...), nil, nil
	case tumbler.PolicyPasswordAndYubiKey:
		if len(password) == 0 {
			return nil, nil, ErrPasswordRequired
		}
		pw, err := securebytes.New(cloneBytes(password))
		if err != nil {
			return nil, nil, err
		}
		m := tumbler.NewYubiKeyMethod(s.transport, tumbler.YubiKeyConfig{
			Slot: s.slot,
			KDF:  tumbler.NewScryptKDF(scryptLogN, 8, 1),
		}, pw, opts...)
		return m, pw, nil
	default:
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedPolicy, policy)
	}
}

// unlockMethod builds the unlock method for a policy. Unlike enroll, the KDF
// parameters and OTP slot come from the envelope's slot, so the config here is
// intentionally minimal.
func (s *Store) unlockMethod(policy tumbler.Policy, passwordFn func() ([]byte, error)) (tumbler.Method, *securebytes.SecureBytes, error) {
	switch policy {
	case tumbler.PolicyYubiKeyOnly:
		return tumbler.NewYubiKeyMethod(s.transport, tumbler.YubiKeyConfig{}, nil), nil, nil
	case tumbler.PolicyPasswordAndYubiKey:
		if passwordFn == nil {
			return nil, nil, ErrPasswordRequired
		}
		pw, err := passwordFn()
		if err != nil {
			return nil, nil, err
		}
		pwSB, err := securebytes.New(cloneBytes(pw))
		zeroBytes(pw)
		if err != nil {
			return nil, nil, err
		}
		return tumbler.NewYubiKeyMethod(s.transport, tumbler.YubiKeyConfig{}, pwSB), pwSB, nil
	default:
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedPolicy, policy)
	}
}

// unlockToSeed unlocks env with method and returns the seed as a fresh []byte.
func unlockToSeed(ctx context.Context, env *tumbler.Envelope, method tumbler.Method) ([]byte, error) {
	seedSB, err := env.Unlock(ctx, method)
	if err != nil {
		return nil, err
	}
	defer func() { _ = seedSB.Destroy() }()

	var out []byte
	if useErr := seedSB.Use(func(b []byte) { out = cloneBytes(b) }); useErr != nil {
		return nil, useErr
	}
	return out, nil
}

// cloneBytes returns a copy of b.
func cloneBytes(b []byte) []byte {
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// zeroBytes overwrites b with zeros.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
