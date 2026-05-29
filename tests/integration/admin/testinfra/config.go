//go:build integration

package testinfra

import (
	"os"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"

	"github.com/gogf/gf/v2/frame/g"
)

var (
	DefaultBaseURL  = getEnvOrDefault("TEST_BASE_URL", "http://127.0.0.1:18888")
	DefaultUsername = getEnvOrDefault("TEST_ADMIN_USERNAME", "")
	DefaultPassword = getEnvOrDefault("TEST_ADMIN_PASSWORD", "")
)

// getEnvOrDefault returns the environment variable value or the fallback.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetRedisAddr returns the Redis address from system config, or falls back to env/empty.
func GetRedisAddr() string {
	if v := os.Getenv("TEST_REDIS_ADDR"); v != "" {
		return v
	}
	if v, err := g.Cfg().Get(nil, "redis.default.address"); err == nil && !v.IsNil() {
		return v.String()
	}
	return ""
}
