//go:build integration

package admin_test

import (
	"os"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testinfra.CleanupResidualTestData()
	os.Exit(code)
}
