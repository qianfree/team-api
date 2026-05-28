//go:build integration

package testinfra

import (
	"os"
)

var (
	DefaultBaseURL   = getEnvOrDefault("TEST_BASE_URL", "http://127.0.0.1:18888")
	DefaultUsername  = getEnvOrDefault("TEST_ADMIN_USERNAME", "")
	DefaultPassword  = getEnvOrDefault("TEST_ADMIN_PASSWORD", "")
	DefaultRedisAddr = getEnvOrDefault("TEST_REDIS_ADDR", "192.168.50.22:16380")
)

// getEnvOrDefault returns the environment variable value or the fallback.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
