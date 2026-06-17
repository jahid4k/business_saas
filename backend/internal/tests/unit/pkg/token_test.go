// backend/internal/tests/unit/pkg/token_test.go
package pkg

import (
	"strings"
	"testing"

	"github.com/mridha/businesssaas/pkg/token"
)

func TestTokenGenerate_ReturnsNonEmpty(t *testing.T) {
	raw, hash, err := token.Generate()
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if raw == "" {
		t.Error("raw token must not be empty")
	}
	if hash == "" {
		t.Error("hash must not be empty")
	}
}

func TestTokenGenerate_RawTokenIsURLSafe(t *testing.T) {
	raw, _, _ := token.Generate()
	for _, c := range raw {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Errorf("raw token contains non-URL-safe character %q", c)
		}
	}
}

func TestTokenGenerate_Has256BitEntropy(t *testing.T) {
	raw, _, _ := token.Generate()
	if len(raw) < 40 {
		t.Errorf("raw token too short (%d chars), expected at least 40 for 256-bit entropy", len(raw))
	}
}

func TestTokenGenerate_UniqueOnEachCall(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, _, _ := token.Generate()
		if seen[raw] {
			t.Fatalf("duplicate token generated at iteration %d", i)
		}
		seen[raw] = true
	}
}

func TestTokenHash_Deterministic(t *testing.T) {
	raw := "test-token-abc123"
	h1 := token.Hash(raw)
	h2 := token.Hash(raw)
	if h1 != h2 {
		t.Errorf("Hash must be deterministic: got %q then %q", h1, h2)
	}
}

func TestTokenHash_IsHexSHA256(t *testing.T) {
	h := token.Hash("anything")
	if len(h) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %q", len(h), h)
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash contains non-hex character %q", c)
		}
	}
}

func TestTokenHash_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := token.Hash("token-A")
	h2 := token.Hash("token-B")
	if h1 == h2 {
		t.Error("different inputs must produce different hashes")
	}
}

func TestTokenEqual_Match(t *testing.T) {
	raw, hash, _ := token.Generate()
	if !token.Equal(raw, hash) {
		t.Error("Equal(raw, Hash(raw)) must return true")
	}
}

func TestTokenEqual_WrongRaw(t *testing.T) {
	_, hash, _ := token.Generate()
	if token.Equal("wrong-token", hash) {
		t.Error("Equal with wrong token must return false")
	}
}

func TestTokenEqual_EmptyRaw(t *testing.T) {
	_, hash, _ := token.Generate()
	if token.Equal("", hash) {
		t.Error("Equal with empty raw must return false")
	}
}

func TestToken_TamperedTokenFails(t *testing.T) {
	raw, hash, _ := token.Generate()
	if !token.Equal(raw, hash) {
		t.Fatal("correct token must verify")
	}
	if token.Equal(strings.ToUpper(raw), hash) {
		t.Error("tampered token must not verify against original hash")
	}
}

func TestToken_CrossTokenIsolation(t *testing.T) {
	raw1, hash1, _ := token.Generate()
	raw2, hash2, _ := token.Generate()
	if token.Equal(raw1, hash2) {
		t.Error("token-1 must not verify against token-2's hash")
	}
	if token.Equal(raw2, hash1) {
		t.Error("token-2 must not verify against token-1's hash")
	}
}
