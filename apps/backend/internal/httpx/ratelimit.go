package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"

	"phasma/backend/internal/cacheclient"
	"phasma/backend/internal/env"
)

// tokenBucketLua refills tokens based on elapsed time, decrements one, sets
// the key TTL, and returns {1, 0} when allowed or {0, retry_ms} when denied.
const tokenBucketLua = `
local data   = redis.call('HMGET', KEYS[1], 't', 'l')
local tokens = tonumber(data[1])
local last   = tonumber(data[2])
local now    = tonumber(ARGV[3])
local burst  = tonumber(ARGV[1])
local rate   = tonumber(ARGV[2])
if not tokens then tokens = burst end
if not last   then last   = now  end
local elapsed = math.max(0, now - last)
local refill  = math.floor(elapsed * rate / 1000)
tokens = math.min(burst, tokens + refill)
if tokens > 0 then
  redis.call('HMSET', KEYS[1], 't', tokens - 1, 'l', now)
  redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate * 1000) + 1000)
  return {1, 0}
end
return {0, math.ceil(1000 / rate)}
`

type RateLimitPolicy struct {
	Name  string
	Burst int
	Rate  float64
}

type RateLimitDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type RateLimiterStore interface {
	Allow(ctx context.Context, identifier string, policy RateLimitPolicy) (RateLimitDecision, error)
}

type CacheRateLimiterStore struct {
	client   valkey.Client
	script   *valkey.Lua
	failOpen bool
}

func NewCacheRateLimiterStore(cacheURL, password string) (*CacheRateLimiterStore, error) {
	opt, err := cacheclient.ParseOptions(cacheURL, password)
	if err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}
	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("rate limiter: connect to cache: %w", err)
	}
	return &CacheRateLimiterStore{
		client:   client,
		script:   valkey.NewLuaScript(tokenBucketLua),
		failOpen: env.Bool("RATE_LIMIT_FAIL_OPEN", false),
	}, nil
}

func (s *CacheRateLimiterStore) Allow(ctx context.Context, identifier string, policy RateLimitPolicy) (RateLimitDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	nowMs := time.Now().UnixMilli()
	res := s.script.Exec(ctx, s.client,
		[]string{identifier},
		[]string{
			strconv.Itoa(policy.Burst),
			strconv.FormatFloat(policy.Rate, 'f', -1, 64),
			strconv.FormatInt(nowMs, 10),
		},
	)
	if err := res.Error(); err != nil {
		return RateLimitDecision{Allowed: s.failOpen}, err
	}
	arr, err := res.ToArray()
	if err != nil {
		return RateLimitDecision{Allowed: s.failOpen}, err
	}
	allowed, err := arr[0].ToInt64()
	if err != nil {
		return RateLimitDecision{Allowed: s.failOpen}, err
	}
	if allowed == 1 {
		return RateLimitDecision{Allowed: true}, nil
	}
	retryMs, _ := arr[1].ToInt64()
	return RateLimitDecision{Allowed: false, RetryAfter: time.Duration(retryMs) * time.Millisecond}, nil
}

type NoopRateLimiterStore struct{}

func (NoopRateLimiterStore) Allow(_ context.Context, _ string, _ RateLimitPolicy) (RateLimitDecision, error) {
	return RateLimitDecision{Allowed: true}, nil
}

func RateLimit(store RateLimiterStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy, exempt := rateLimitPolicy(r)
			if exempt {
				next.ServeHTTP(w, r)
				return
			}

			decision, err := store.Allow(r.Context(), rateLimitKey(r, policy), policy)
			if err != nil {
				slog.Error("rate limiter storage failed", "request_id", GetRequestID(r), "policy", policy.Name, "error", err)
				if !decision.Allowed {
					WriteMessage(w, http.StatusServiceUnavailable, "Service unavailable")
					return
				}
			}
			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter.Seconds()+0.5)))
				WriteMessage(w, http.StatusTooManyRequests, "Too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitPolicy(r *http.Request) (RateLimitPolicy, bool) {
	if r.Method == http.MethodGet && (r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/health/") || r.URL.Path == "/ready") {
		return RateLimitPolicy{}, true
	}
	switch {
	case r.Method == http.MethodPost && (r.URL.Path == "/sessions" || r.URL.Path == "/users" || r.URL.Path == "/uploads"):
		return RateLimitPolicy{Name: "strict", Burst: env.Int("RATE_LIMIT_STRICT_BURST", 5), Rate: env.Float("RATE_LIMIT_STRICT_RATE", 0.2)}, false
	case r.Method == http.MethodGet && (r.URL.Path == "/users/search" || r.URL.Path == "/hashtags/search" || r.URL.Path == "/search"):
		return RateLimitPolicy{Name: "typeahead", Burst: env.Int("RATE_LIMIT_TYPEAHEAD_BURST", 20), Rate: env.Float("RATE_LIMIT_TYPEAHEAD_RATE", 5)}, false
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		return RateLimitPolicy{Name: "read", Burst: env.Int("RATE_LIMIT_READ_BURST", 120), Rate: env.Float("RATE_LIMIT_READ_RATE", 2)}, false
	default:
		return RateLimitPolicy{Name: "mutation", Burst: env.Int("RATE_LIMIT_MUTATION_BURST", 30), Rate: env.Float("RATE_LIMIT_MUTATION_RATE", 1)}, false
	}
}

func rateLimitKey(r *http.Request, policy RateLimitPolicy) string {
	if userID, ok := UserID(r); ok && userID != "" {
		return policy.Name + ":user:" + userID
	}
	// Session cookie is HttpOnly and server-issued, so it's a stable per-client
	// identifier that doesn't collapse when a proxy (e.g. nginx) sits in front.
	if sessionID, ok := GetSessionCookie(r); ok {
		return policy.Name + ":session:" + sessionID
	}
	return policy.Name + ":ip:" + ClientIP(r)
}

// trustProxy controls whether X-Forwarded-For is honored. It must only be
// enabled when the backend sits behind a proxy that overwrites the header;
// invalid forwarded values are ignored to keep spoofed strings out of rate
// limit keys.
var trustProxy = env.Bool("TRUST_PROXY", false)

// ClientIP extracts the client IP, preferring X-Forwarded-For only when the
// deployment is configured to trust an upstream proxy.
func ClientIP(r *http.Request) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if ip := net.ParseIP(strings.TrimSpace(strings.Split(forwarded, ",")[0])); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
