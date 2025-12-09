package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redis       *redis.Client
	maxRequests int
	window      time.Duration
}

func NewRateLimiter(redisClient *redis.Client, maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:       redisClient,
		maxRequests: maxRequests,
		window:      window,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		key := fmt.Sprintf("rate_limit:%s", ip)

		ctx := context.Background()
		count, err := rl.redis.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, allow the request but log the error
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if count == 1 {
			rl.redis.Expire(ctx, key, rl.window)
		}

		if count > int64(rl.maxRequests) {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxRequests))
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxRequests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rl.maxRequests-int(count)))

		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (format: client, proxy1, proxy2)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take only the first IP (the actual client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
