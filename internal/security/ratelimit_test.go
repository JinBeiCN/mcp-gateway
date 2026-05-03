package security

import (
	"testing"
	"time"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(100, 200)

	if !rl.Allow("client-1") {
		t.Error("expected first request to be allowed")
	}
}

func TestRateLimiterExhaustBurst(t *testing.T) {
	rl := NewRateLimiter(1, 5)

	// Use up all burst tokens
	for i := 0; i < 5; i++ {
		if !rl.Allow("client-1") {
			t.Errorf("request %d should have been allowed", i+1)
		}
	}

	// 6th request should be denied
	if rl.Allow("client-1") {
		t.Error("6th request should have been denied (burst exhausted)")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(100, 5)

	// Use up burst
	for i := 0; i < 5; i++ {
		rl.Allow("client-1")
	}

	// Should be denied now
	if rl.Allow("client-1") {
		t.Error("should be rate limited after burst exhausted")
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should have refilled some tokens (at 100 rps, ~5 tokens in 50ms)
	if !rl.Allow("client-1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiterMultipleClients(t *testing.T) {
	rl := NewRateLimiter(1, 2)

	if !rl.Allow("client-1") {
		t.Error("client-1 first request should be allowed")
	}
	if !rl.Allow("client-2") {
		t.Error("client-2 first request should be allowed")
	}
}

func TestRateLimiterSetRate(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	rl.Allow("client-1")
	if rl.Allow("client-1") {
		t.Error("should be denied at 1 rps")
	}

	rl.SetRate("client-1", 100, 100)
	time.Sleep(20 * time.Millisecond)
	if !rl.Allow("client-1") {
		t.Error("should be allowed after rate increase and refill")
	}
}

func TestBackendRateLimiter(t *testing.T) {
	backends := []config.BackendConfig{
		{Name: "backend-1", RateLimit: &config.RateLimitConfig{RequestsPerSec: 100, BurstSize: 200}},
		{Name: "backend-2", RateLimit: &config.RateLimitConfig{RequestsPerSec: 5, BurstSize: 10}},
	}
	brl := NewBackendRateLimiter(backends)

	if !brl.Allow("client-1", "backend-1", "tool-a") {
		t.Error("expected allowed for backend-1")
	}
	if !brl.Allow("client-1", "backend-2", "tool-b") {
		t.Error("expected allowed for backend-2")
	}
}

func TestRateLimiterDefaultValues(t *testing.T) {
	rl := NewRateLimiter(0, 0)

	// Should use defaults (10 rps, 20 burst)
	for i := 0; i < 20; i++ {
		if !rl.Allow("client-1") {
			t.Errorf("request %d should have been allowed (using defaults)", i+1)
		}
	}
}

func TestGetClientKey(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	key := rl.GetClientKey("client-123", "backend-xyz")
	expected := "client-123:backend-xyz"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}
