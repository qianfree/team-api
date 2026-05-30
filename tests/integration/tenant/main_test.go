//go:build integration

package tenant_test

import (
	"os"
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testinfra.CleanupResidualTestData()
	os.Exit(code)
}
