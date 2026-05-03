package security

import (
	"sync"
	"time"

	"github.com/JinBeiCN/mcp-gateway/internal/config"
)

type RateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*tokenBucket
	defaultRPS int
	defaultBurst int
}

type tokenBucket struct {
	tokens    float64
	lastFill  time.Time
	rate      float64
	burst     float64
}

func NewRateLimiter(defaultRPS, defaultBurst int) *RateLimiter {
	if defaultRPS <= 0 {
		defaultRPS = 10
	}
	if defaultBurst <= 0 {
		defaultBurst = 20
	}
	return &RateLimiter{
		buckets:     make(map[string]*tokenBucket),
		defaultRPS:  defaultRPS,
		defaultBurst: defaultBurst,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	return rl.AllowN(key, 1)
}

func (rl *RateLimiter) AllowN(key string, n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = &tokenBucket{
			tokens:   float64(rl.defaultBurst),
			lastFill: time.Now(),
			rate:     float64(rl.defaultRPS),
			burst:    float64(rl.defaultBurst),
		}
		rl.buckets[key] = bucket
	}

	rl.cleanupExpired()

	now := time.Now()
	elapsed := now.Sub(bucket.lastFill).Seconds()
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > bucket.burst {
		bucket.tokens = bucket.burst
	}
	bucket.lastFill = now

	if bucket.tokens >= float64(n) {
		bucket.tokens -= float64(n)
		return true
	}
	return false
}

func (rl *RateLimiter) cleanupExpired() {
	threshold := time.Now().Add(-5 * time.Minute)
	for key, bucket := range rl.buckets {
		if bucket.lastFill.Before(threshold) {
			delete(rl.buckets, key)
		}
	}
}

func (rl *RateLimiter) SetRate(key string, rps, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = &tokenBucket{}
		rl.buckets[key] = bucket
	}
	bucket.rate = float64(rps)
	bucket.burst = float64(burst)
	if bucket.tokens > bucket.burst {
		bucket.tokens = bucket.burst
	}
}

func (rl *RateLimiter) GetClientKey(clientID, backend string) string {
	return clientID + ":" + backend
}

type BackendRateLimiter struct {
	clientLimiter *RateLimiter
	toolLimiter   *RateLimiter
	perBackend    map[string]*RateLimiter
	backends      map[string]*config.BackendConfig
}

func NewBackendRateLimiter(backends []config.BackendConfig) *BackendRateLimiter {
	brl := &BackendRateLimiter{
		clientLimiter: NewRateLimiter(10, 20),
		toolLimiter:   NewRateLimiter(60, 120),
		perBackend:    make(map[string]*RateLimiter),
		backends:      make(map[string]*config.BackendConfig),
	}
	for i := range backends {
		b := &backends[i]
		brl.backends[b.Name] = b
		rps := 10
		burst := 20
		if b.RateLimit != nil {
			if b.RateLimit.RequestsPerSec > 0 {
				rps = b.RateLimit.RequestsPerSec
			}
			if b.RateLimit.BurstSize > 0 {
				burst = b.RateLimit.BurstSize
			}
		}
		brl.perBackend[b.Name] = NewRateLimiter(rps, burst)
	}
	return brl
}

func (brl *BackendRateLimiter) Allow(clientID, backend, tool string) bool {
	if !brl.clientLimiter.Allow(clientID) {
		return false
	}
	if !brl.toolLimiter.Allow(backend + ":" + tool) {
		return false
	}
	if limiter, ok := brl.perBackend[backend]; ok {
		return limiter.Allow(clientID)
	}
	return true
}
