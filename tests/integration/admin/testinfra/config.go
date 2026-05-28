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
	DefaultDBDSN     = getEnvOrDefault("TEST_DB_DSN", "postgres://qian:PgDB123456!@192.168.50.22:15432/team-api-3?sslmode=disable")
)

// getEnvOrDefault returns the environment variable value or the fallback.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
