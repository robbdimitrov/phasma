package cacheclient

import (
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"phasma/backend/internal/env"
)

const (
	defaultConnLifetimeMinutes = 30
	defaultConnWriteTimeoutMS  = 3000
)

func ParseOptions(cacheURL, password string) (valkey.ClientOption, error) {
	opt, err := valkey.ParseURL(cacheURL)
	if err != nil {
		return valkey.ClientOption{}, fmt.Errorf("parse cache url: %w", err)
	}
	if password != "" {
		opt.Password = password
	}
	opt.ConnLifetime = time.Duration(env.Int("CACHE_CONN_LIFETIME_MINUTES", defaultConnLifetimeMinutes)) * time.Minute
	opt.ConnWriteTimeout = time.Duration(env.Int("CACHE_CONN_WRITE_TIMEOUT_MS", defaultConnWriteTimeoutMS)) * time.Millisecond
	return opt, nil
}
