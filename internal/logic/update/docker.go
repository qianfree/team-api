package update

import (
	"os"
)

// IsDocker detects if the application is running inside a Docker container
func IsDocker() bool {
	// Check /.dockerenv file (standard Docker indicator)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check environment variable (set by our docker-compose)
	if os.Getenv("TEAM_API_DEPLOYMENT") == "docker" {
		return true
	}

	// Check for Docker-specific cgroup indicator
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if containsDockerIndicator(content) {
			return true
		}
	}

	return false
}

// containsDockerIndicator checks if cgroup content indicates Docker
func containsDockerIndicator(content string) bool {
	dockerIndicators := []string{
		"/docker/",
		"/docker-",
		"docker",
	}
	for _, indicator := range dockerIndicators {
		if len(content) > 0 && contains(content, indicator) {
			return true
		}
	}
	return false
}

// contains is a simple string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
