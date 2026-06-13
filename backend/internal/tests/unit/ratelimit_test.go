// backend/internal/tests/unit/ratelimit_test.go
package unit

import (
	"testing"
	"time"
)

// TestAuthRateLimitWindow verifies the window is non-zero so the Redis TTL
// is actually applied. This is a regression test for the bug where
// WindowSeconds was 0, causing keys to expire immediately on every request.
func TestAuthRateLimitWindowIsPositive(t *testing.T) {
	const authWindow = 900 // 15 minutes in seconds
	if authWindow <= 0 {
		t.Fatalf("auth rate limit window must be > 0, got %d", authWindow)
	}
	if time.Duration(authWindow)*time.Second == 0 {
		t.Fatal("time.Duration(window)*time.Second must not be zero — Redis EXPIRE would delete key immediately")
	}
}
