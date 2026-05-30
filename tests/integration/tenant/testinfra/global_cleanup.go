//go:build integration

package testinfra

import (
	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

// CleanupResidualTestData delegates to admin testinfra's global cleanup.
func CleanupResidualTestData() {
	admintest.CleanupResidualTestData()
}
