package agent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestGenerateToken(t *testing.T) {
	t.Parallel()

	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if !strings.HasPrefix(token, tokenPrefix) {
		t.Errorf("GenerateToken() token = %q, want prefix %q", token, tokenPrefix)
	}

	// Check minimum length: prefix + base64(32 bytes)
	if len(token) <= len(tokenPrefix) {
		t.Errorf("GenerateToken() token too short: %d chars", len(token))
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	t.Parallel()

	tokens := make(map[string]bool, 100)
	for range 100 {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if tokens[token] {
			t.Errorf("GenerateToken() produced duplicate token: %s", token)
		}
		tokens[token] = true
	}
}

func TestTokenID(t *testing.T) {
	t.Parallel()

	token := "sigil_agt_dGVzdHRva2VuMTIzNDU2Nzg5MDEyMzQ1Njc4OTA" //nolint:gosec // G101: Test token, not a real credential

	id := TokenID(token)

	if !strings.HasPrefix(id, "agt_") {
		t.Errorf("TokenID() = %q, want prefix %q", id, "agt_")
	}

	if len(id) != 4+agentIDLength {
		t.Errorf("TokenID() length = %d, want %d", len(id), 4+agentIDLength)
	}

	// Deterministic
	id2 := TokenID(token)
	if id != id2 {
		t.Errorf("TokenID() not deterministic: %q != %q", id, id2)
	}
}

func TestTokenID_DifferentTokens(t *testing.T) {
	t.Parallel()

	id1 := TokenID("sigil_agt_token1AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	id2 := TokenID("sigil_agt_token2BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")

	if id1 == id2 {
		t.Errorf("TokenID() should produce different IDs for different tokens")
	}
}

func TestParseToken(t *testing.T) {
	t.Parallel()

	// Generate a valid token first
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	raw, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if len(raw) != tokenLength {
		t.Errorf("ParseToken() raw length = %d, want %d", len(raw), tokenLength)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"too short", "sigil_agt_"},
		{"wrong prefix", "wrong_prefix_AAAA"},
		{"invalid base64", "sigil_agt_not!valid!base64!!!!!!!!!!!!!!!!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseToken(tt.token)
			if err == nil {
				t.Errorf("ParseToken(%q) expected error, got nil", tt.token)
			}
		})
	}
}

func TestCredential_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "not expired - future",
			expiresAt: time.Now().Add(time.Hour),
			want:      false,
		},
		{
			name:      "expired - past",
			expiresAt: time.Now().Add(-time.Hour),
			want:      true,
		},
		{
			name:      "just expired",
			expiresAt: time.Now().Add(-time.Millisecond),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credential{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("Credential.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredential_TTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expiresAt time.Time
		wantZero  bool
	}{
		{
			name:      "valid - returns positive TTL",
			expiresAt: time.Now().Add(10 * time.Minute),
			wantZero:  false,
		},
		{
			name:      "expired - returns zero",
			expiresAt: time.Now().Add(-10 * time.Minute),
			wantZero:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credential{ExpiresAt: tt.expiresAt}
			got := c.TTL()
			if tt.wantZero && got != 0 {
				t.Errorf("Credential.TTL() = %v, want 0", got)
			}
			if !tt.wantZero && got <= 0 {
				t.Errorf("Credential.TTL() = %v, want > 0", got)
			}
		})
	}
}

func TestCredential_HasChain(t *testing.T) {
	t.Parallel()

	c := &Credential{
		Chains: []chain.ID{chain.BSV, chain.ETH},
	}

	if !c.HasChain(chain.BSV) {
		t.Error("HasChain(BSV) = false, want true")
	}
	if !c.HasChain(chain.ETH) {
		t.Error("HasChain(ETH) = false, want true")
	}
	if c.HasChain(chain.BTC) {
		t.Error("HasChain(BTC) = true, want false")
	}
}

func TestCredential_HasChain_Empty(t *testing.T) {
	t.Parallel()

	c := &Credential{}
	if c.HasChain(chain.BSV) {
		t.Error("HasChain(BSV) = true, want false for empty chains")
	}
}

//nolint:gocognit // Table-driven test with nil/non-nil big.Int comparison
func TestPolicy_MaxPerTxWeiBig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wei  string
		want *big.Int
	}{
		{"empty string", "", nil},
		{"zero string", "0", nil},
		{"valid wei", "1000000000000000", new(big.Int).SetUint64(1000000000000000)},
		{"invalid number", "not-a-number", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{MaxPerTxWei: tt.wei}
			got := p.MaxPerTxWeiBig()
			if tt.want == nil {
				if got != nil {
					t.Errorf("MaxPerTxWeiBig() = %v, want nil", got)
				}
			} else {
				if got == nil || got.Cmp(tt.want) != 0 {
					t.Errorf("MaxPerTxWeiBig() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

//nolint:gocognit // Table-driven test with nil/non-nil big.Int comparison
func TestPolicy_MaxDailyWeiBig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wei  string
		want *big.Int
	}{
		{"empty string", "", nil},
		{"zero string", "0", nil},
		{"valid wei", "10000000000000000", new(big.Int).SetUint64(10000000000000000)},
		{"invalid number", "xyz", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{MaxDailyWei: tt.wei}
			got := p.MaxDailyWeiBig()
			if tt.want == nil {
				if got != nil {
					t.Errorf("MaxDailyWeiBig() = %v, want nil", got)
				}
			} else {
				if got == nil || got.Cmp(tt.want) != 0 {
					t.Errorf("MaxDailyWeiBig() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestComputePolicyHMAC(t *testing.T) {
	t.Parallel()

	policy := &Policy{
		MaxPerTxSat: 50000,
		MaxDailySat: 500000,
	}
	token := "sigil_agt_testtoken" //nolint:gosec // G101: Test token, not a real credential

	hmacStr, err := ComputePolicyHMAC(policy, token)
	if err != nil {
		t.Fatalf("ComputePolicyHMAC() error = %v", err)
	}

	if hmacStr == "" {
		t.Error("ComputePolicyHMAC() returned empty string")
	}

	// Deterministic
	hmac2, err := ComputePolicyHMAC(policy, token)
	if err != nil {
		t.Fatalf("ComputePolicyHMAC() second call error = %v", err)
	}
	if hmacStr != hmac2 {
		t.Error("ComputePolicyHMAC() not deterministic")
	}
}

func TestComputePolicyHMAC_DifferentTokens(t *testing.T) {
	t.Parallel()

	policy := &Policy{MaxPerTxSat: 50000}

	hmac1, _ := ComputePolicyHMAC(policy, "token1")
	hmac2, _ := ComputePolicyHMAC(policy, "token2")

	if hmac1 == hmac2 {
		t.Error("ComputePolicyHMAC() should produce different HMACs for different tokens")
	}
}

func TestVerifyPolicyHMAC(t *testing.T) {
	t.Parallel()

	policy := &Policy{
		MaxPerTxSat: 50000,
		MaxDailySat: 500000,
	}
	token := "sigil_agt_testtoken" //nolint:gosec // G101: Test token

	hmacStr, _ := ComputePolicyHMAC(policy, token)

	valid, err := VerifyPolicyHMAC(policy, token, hmacStr)
	if err != nil {
		t.Fatalf("VerifyPolicyHMAC() error = %v", err)
	}
	if !valid {
		t.Error("VerifyPolicyHMAC() = false, want true")
	}
}

func TestVerifyPolicyHMAC_WrongToken(t *testing.T) {
	t.Parallel()

	policy := &Policy{MaxPerTxSat: 50000}
	hmacStr, _ := ComputePolicyHMAC(policy, "correct_token")

	valid, err := VerifyPolicyHMAC(policy, "wrong_token", hmacStr)
	if err != nil {
		t.Fatalf("VerifyPolicyHMAC() error = %v", err)
	}
	if valid {
		t.Error("VerifyPolicyHMAC() = true, want false for wrong token")
	}
}

func TestVerifyPolicyHMAC_TamperedPolicy(t *testing.T) {
	t.Parallel()

	original := &Policy{MaxPerTxSat: 50000}
	token := "sigil_agt_testtoken" //nolint:gosec // G101: Test token
	hmacStr, _ := ComputePolicyHMAC(original, token)

	// Tamper with the policy
	tampered := &Policy{MaxPerTxSat: 999999}

	valid, err := VerifyPolicyHMAC(tampered, token, hmacStr)
	if err != nil {
		t.Fatalf("VerifyPolicyHMAC() error = %v", err)
	}
	if valid {
		t.Error("VerifyPolicyHMAC() = true, want false for tampered policy")
	}
}

func TestZeroBytes(t *testing.T) {
	t.Parallel()

	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	zeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("zeroBytes() data[%d] = %d, want 0", i, b)
		}
	}
}

func TestZeroBytes_Nil(t *testing.T) {
	t.Parallel()
	// Should not panic on nil slice
	zeroBytes(nil)
}
