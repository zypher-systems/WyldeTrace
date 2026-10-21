package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

// Token-bucket rate limiter keyed by authenticated user/token.
type userBucket struct {
	tokens float64
	last   time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	byKey    map[string]*userBucket
	rate     float64 // tokens per second
	capacity float64
}

func newRateLimiter(perMinute int) *rateLimiter {
	if perMinute < 1 {
		perMinute = 30
	}
	cap := float64(perMinute)
	return &rateLimiter{
		byKey:    map[string]*userBucket{},
		rate:     cap / 60.0,
		capacity: cap,
	}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.byKey[key]
	if b == nil {
		l.byKey[key] = &userBucket{tokens: l.capacity - 1, last: now}
		return true
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func rateLimitKey(r *http.Request) string {
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		if claims.IsAPIToken && claims.TokenID != uuid.Nil {
			return "token:" + claims.TokenID.String()
		}
		if claims.UserID != uuid.Nil {
			return "user:" + claims.UserID.String()
		}
		if claims.Email != "" {
			return "email:" + claims.Email
		}
	}
	return "anon:" + r.RemoteAddr
}

// RateLimit returns middleware that allows roughly perMinute requests per user/token.
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	lim := newRateLimiter(perMinute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !lim.allow(rateLimitKey(r)) {
				httpx.Error(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
