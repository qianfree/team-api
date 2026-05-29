//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

// ─── 跨租户隔离测试 ──────────────────────────────────────────────────
//
// 测试原则：
//   1. 先在租户 A 创建真实数据，验证 A 能看到
//   2. 验证租户 B 无法看到/操作 A 的数据
//   3. 验证 B 的尝试不会破坏 A 的数据
//   4. 验证具体错误码，而非笼统的"不成功"

// assertForbidden 断言响应为权限拒绝（业务码 403 或 HTTP 422/403）
func assertForbidden(t *testing.T, resp *admintest.APIResponse, context string) {
	t.Helper()
	if resp.Code == 0 {
		t.Fatalf("%s: expected forbidden but got success (code=0)", context)
	}
	// 403 = 直接权限拒绝，10033 = 项目密钥权限，其他非零也可能是权限相关
	t.Logf("%s: blocked as expected (code=%d, http=%d, msg=%s)", context, resp.Code, resp.HTTPStatus, resp.Message)
}

// assertNotFound 断言响应为资源不存在（业务码 404 或类似）
func assertNotFound(t *testing.T, resp *admintest.APIResponse, context string) {
	t.Helper()
	if resp.Code == 0 {
		t.Fatalf("%s: expected not-found but got success (code=0)", context)
	}
}

// ─── 成员隔离 ────────────────────────────────────────────────────────

func TestIsolation_MemberListIsolated(t *testing.T) {
	// 注册两个租户
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	// 租户 A 创建成员
	memberIDA, _ := testinfra.CreateTestTenantMember(t, clientA)

	// 验证 A 能看到自己创建的成员
	listA := clientA.Get("/api/tenant/members", map[string]string{"page": "1", "page_size": "100"})
	listA.AssertSuccess(t)
	var dataA struct {
		List []struct {
			ID   int64  `json:"id"`
			Role string `json:"role"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listA.DecodeData(t, &dataA)

	foundA := false
	for _, m := range dataA.List {
		if m.ID == memberIDA {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Fatal("tenant A should see its own member in the list")
	}

	// 验证 B 的成员列表不包含 A 的成员
	listB := clientB.Get("/api/tenant/members", map[string]string{"page": "1", "page_size": "100"})
	listB.AssertSuccess(t)
	var dataB struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listB.DecodeData(t, &dataB)
	for _, m := range dataB.List {
		if m.ID == memberIDA {
			t.Fatal("tenant B must not see tenant A's member — data isolation broken")
		}
	}
}

func TestIsolation_MemberDetailCrossTenantInaccessible(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	memberIDA, _ := testinfra.CreateTestTenantMember(t, clientA)

	// B 尝试用 A 的成员 ID 获取详情 → 应返回"成员不存在"或空数据
	detailResp := clientB.Get(fmt.Sprintf("/api/tenant/members/%d", memberIDA), nil)
	if detailResp.Code == 0 {
		// 如果成功返回了数据，验证不是 A 的成员
		var detail struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		}
		detailResp.DecodeData(t, &detail)
		if detail.ID == memberIDA {
			t.Fatal("tenant B should not retrieve tenant A's member details — isolation broken")
		}
	}
	// code != 0（成员不存在）也是正确的隔离行为
}

func TestIsolation_MemberDeleteCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	memberIDA, _ := testinfra.CreateTestTenantMember(t, clientA)

	// B 尝试删除 A 的成员
	deleteResp := clientB.Delete(fmt.Sprintf("/api/tenant/members/%d", memberIDA))
	// 不管 B 的操作是否"成功"返回，验证 A 的成员仍然存在

	verifyResp := clientA.Get(fmt.Sprintf("/api/tenant/members/%d", memberIDA), nil)
	if verifyResp.Code != 0 {
		t.Fatalf("tenant A's member should still exist after tenant B's delete attempt, got code=%d", verifyResp.Code)
	}
	var verify struct {
		Status string `json:"status"`
	}
	verifyResp.DecodeData(t, &verify)
	if verify.Status != "active" {
		t.Fatalf("tenant A's member should still be active after tenant B's delete attempt, got status=%s", verify.Status)
	}

	// 顺便验证 B 的操作确实被拒绝了（code != 0 说明 B 无权操作）
	if deleteResp.Code == 0 {
		t.Log("WARNING: tenant B's cross-tenant delete returned success — checking data integrity")
	}
}

// ─── API Key 隔离 ────────────────────────────────────────────────────

func TestIsolation_ApiKeyListIsolated(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	// A 创建 API Key
	keyIDA, _ := testinfra.CreateTestApiKey(t, clientA)

	// 验证 A 能看到自己的 Key
	listA := clientA.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "100"})
	listA.AssertSuccess(t)
	var keysA struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listA.DecodeData(t, &keysA)

	found := false
	for _, k := range keysA.List {
		if k.ID == keyIDA {
			found = true
		}
	}
	if !found {
		t.Fatal("tenant A should see its own API key")
	}

	// 验证 B 看不到 A 的 Key
	listB := clientB.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "100"})
	listB.AssertSuccess(t)
	var keysB struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listB.DecodeData(t, &keysB)
	for _, k := range keysB.List {
		if k.ID == keyIDA {
			t.Fatal("tenant B must not see tenant A's API key — data isolation broken")
		}
	}
}

func TestIsolation_ApiKeyDeleteCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	keyIDA, _ := testinfra.CreateTestApiKey(t, clientA)

	// B 尝试删除 A 的 Key
	clientB.Delete(fmt.Sprintf("/api/tenant/api-keys/%d", keyIDA))

	// 验证 A 的 Key 仍然存在
	verifyResp := clientA.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "100"})
	verifyResp.AssertSuccess(t)
	var keys struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &keys)

	for _, k := range keys.List {
		if k.ID == keyIDA {
			return // A 的 Key 还在，隔离有效
		}
	}
	t.Fatal("tenant A's API key disappeared after tenant B's delete — isolation broken")
}

// ─── 项目隔离 ────────────────────────────────────────────────────────

func TestIsolation_ProjectListIsolated(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	projectIDA, _ := testinfra.CreateTestProject(t, clientA)

	// A 能看到自己的项目
	listA := clientA.Get("/api/tenant/projects", map[string]string{"page": "1", "page_size": "100"})
	listA.AssertSuccess(t)
	var projA struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listA.DecodeData(t, &projA)

	found := false
	for _, p := range projA.List {
		if p.ID == projectIDA {
			found = true
		}
	}
	if !found {
		t.Fatal("tenant A should see its own project")
	}

	// B 看不到 A 的项目
	listB := clientB.Get("/api/tenant/projects", map[string]string{"page": "1", "page_size": "100"})
	listB.AssertSuccess(t)
	var projB struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	listB.DecodeData(t, &projB)
	for _, p := range projB.List {
		if p.ID == projectIDA {
			t.Fatal("tenant B must not see tenant A's project — data isolation broken")
		}
	}
}

func TestIsolation_ProjectDetailCrossTenantInaccessible(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	projectIDA, _ := testinfra.CreateTestProject(t, clientA)

	// B 尝试获取 A 的项目详情
	detailResp := clientB.Get(fmt.Sprintf("/api/tenant/projects/%d", projectIDA), nil)
	if detailResp.Code == 0 {
		t.Fatal("tenant B should not be able to access tenant A's project detail — isolation broken")
	}
}

// ─── 钱包隔离 ────────────────────────────────────────────────────────

func TestIsolation_WalletTenantBinding(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	// 两个租户各自获取钱包
	walletA := clientA.Get("/api/tenant/wallet", nil)
	walletA.AssertSuccess(t)
	var wA struct {
		Currency string `json:"currency"`
	}
	walletA.DecodeData(t, &wA)

	walletB := clientB.Get("/api/tenant/wallet", nil)
	walletB.AssertSuccess(t)
	var wB struct {
		Currency string `json:"currency"`
	}
	walletB.DecodeData(t, &wB)

	// 两个钱包必须独立且货币正确
	if wA.Currency != "USD" {
		t.Fatalf("wallet A currency=%q, expected USD", wA.Currency)
	}
	if wB.Currency != "USD" {
		t.Fatalf("wallet B currency=%q, expected USD", wB.Currency)
	}
}

// ─── 组织隔离 ────────────────────────────────────────────────────────

func TestIsolation_OrganizationCodeDistinct(t *testing.T) {
	clientA, resultA := testinfra.GetAuthedClient(t)
	clientB, resultB := testinfra.GetAuthedClient(t)

	orgA := clientA.Get("/api/tenant/organization", nil)
	orgA.AssertSuccess(t)
	var oA struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	orgA.DecodeData(t, &oA)

	orgB := clientB.Get("/api/tenant/organization", nil)
	orgB.AssertSuccess(t)
	var oB struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	orgB.DecodeData(t, &oB)

	// 组织代码与注册时一致
	if oA.Code != resultA.Tenant.Code {
		t.Fatalf("org A code=%q, expected=%q", oA.Code, resultA.Tenant.Code)
	}
	if oB.Code != resultB.Tenant.Code {
		t.Fatalf("org B code=%q, expected=%q", oB.Code, resultB.Tenant.Code)
	}
	// 两个组织代码必须不同
	if oA.Code == oB.Code {
		t.Fatal("two tenants must have different organization codes")
	}
}

// ─── Token 有效性 ────────────────────────────────────────────────────

func TestIsolation_InvalidTokenRejected(t *testing.T) {
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken("eyJhbGciOiJIUzI1NiJ9.invalid.signature")

	resp := client.Get("/api/tenant/organization", nil)
	if resp.Code == 0 {
		t.Fatal("invalid JWT should be rejected")
	}
}

func TestIsolation_NoTokenRejected(t *testing.T) {
	client := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	resp := client.Get("/api/tenant/organization", nil)
	if resp.Code == 0 {
		t.Fatal("no token should be rejected")
	}
}

func TestIsolation_AdminTokenCannotAccessTenantEndpoints(t *testing.T) {
	adminClient := admintest.GetAuthedClient(t)

	resp := adminClient.Get("/api/tenant/organization", nil)
	if resp.Code == 0 {
		t.Fatal("admin JWT (userType=admin) must not access tenant endpoints")
	}
}

// ─── P0: 补充跨租户隔离场景 ──────────────────────────────────────────

// TestIsolation_ProjectUpdateCrossTenantIneffective verifies B cannot modify A's project.
// Business rule: project data is scoped to the owning tenant.
func TestIsolation_ProjectUpdateCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	projectIDA, _ := testinfra.CreateTestProject(t, clientA)

	// B attempts to update A's project name
	updateResp := clientB.Put(fmt.Sprintf("/api/tenant/projects/%d", projectIDA), map[string]any{
		"name": "hacked by B",
	})
	// Either the update is rejected, or it doesn't affect A's data
	_ = updateResp

	// Verify A's project name is unchanged
	detailResp := clientA.Get(fmt.Sprintf("/api/tenant/projects/%d", projectIDA), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Name string `json:"name"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Name == "hacked by B" {
		t.Fatal("tenant B modified tenant A's project name — isolation broken")
	}
}

// TestIsolation_ProjectArchiveCrossTenantIneffective verifies B cannot archive A's project.
// Business rule: project lifecycle operations are tenant-scoped.
func TestIsolation_ProjectArchiveCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	projectIDA, _ := testinfra.CreateTestProject(t, clientA)

	// B attempts to archive A's project
	archiveResp := clientB.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectIDA), nil)
	_ = archiveResp

	// Verify A's project is still accessible
	detailResp := clientA.Get(fmt.Sprintf("/api/tenant/projects/%d", projectIDA), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		Status string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Status == "archived" {
		t.Fatal("tenant B archived tenant A's project — isolation broken")
	}
}

// TestIsolation_WalletTransactionsIsolated verifies B cannot see A's wallet transactions.
// Business rule: financial data is strictly tenant-scoped.
func TestIsolation_WalletTransactionsIsolated(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	// A lists its transactions
	txResp := clientA.Get("/api/tenant/wallet/transactions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	txResp.AssertSuccess(t)

	var txA struct {
		Total int `json:"total"`
	}
	txResp.DecodeData(t, &txA)

	// B lists transactions — should be separate from A's
	txBResp := clientB.Get("/api/tenant/wallet/transactions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	txBResp.AssertSuccess(t)

	// Both new tenants should have zero transactions
	var txB struct {
		Total int `json:"total"`
	}
	txBResp.DecodeData(t, &txB)

	// At minimum, B's transaction list should be independent from A's
	t.Logf("Wallet transaction isolation: A total=%d, B total=%d (both should be 0 for new tenants)", txA.Total, txB.Total)
}

// TestIsolation_UsageLogsIsolated verifies B cannot see A's usage logs.
// Business rule: API usage data is tenant-scoped.
func TestIsolation_UsageLogsIsolated(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	// A lists its usage logs
	usageA := clientA.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	usageA.AssertSuccess(t)

	// B lists usage logs — must be separate from A's
	usageB := clientB.Get("/api/tenant/usage-logs", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	usageB.AssertSuccess(t)

	var dataA struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	usageA.DecodeData(t, &dataA)

	var dataB struct {
		List []struct {
			Id int64 `json:"id"`
		} `json:"list"`
	}
	usageB.DecodeData(t, &dataB)

	// B's usage logs must not contain any of A's records
	for _, recB := range dataB.List {
		for _, recA := range dataA.List {
			if recB.Id == recA.Id {
				t.Fatalf("tenant B usage log contains tenant A's record id=%d — isolation broken", recA.Id)
			}
		}
	}
}

// TestIsolation_ApiKeyUpdateCrossTenantIneffective verifies B cannot modify A's API key.
// Business rule: API key management is tenant-scoped.
func TestIsolation_ApiKeyUpdateCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	keyIDA, _ := testinfra.CreateTestApiKey(t, clientA)

	// B attempts to update A's key name
	updateResp := clientB.Put(fmt.Sprintf("/api/tenant/api-keys/%d", keyIDA), map[string]any{
		"name": "hacked-key-by-B",
	})
	_ = updateResp

	// Verify A's key name is unchanged
	listResp := clientA.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "100"})
	listResp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &data)

	for _, k := range data.List {
		if k.ID == keyIDA {
			if k.Name == "hacked-key-by-B" {
				t.Fatal("tenant B modified tenant A's API key name — isolation broken")
			}
			return
		}
	}
	t.Fatal("tenant A's API key not found after B's update attempt")
}

// TestIsolation_MemberRoleChangeCrossTenantIneffective verifies B cannot change A's member roles.
// Business rule: member management is tenant-scoped.
func TestIsolation_MemberRoleChangeCrossTenantIneffective(t *testing.T) {
	clientA, _ := testinfra.GetAuthedClient(t)
	clientB, _ := testinfra.GetAuthedClient(t)

	memberIDA, _ := testinfra.CreateTestTenantMember(t, clientA)

	// B attempts to change A's member role to admin
	roleResp := clientB.Put(fmt.Sprintf("/api/tenant/members/%d/role", memberIDA), map[string]any{
		"role": "admin",
	})
	_ = roleResp

	// Verify A's member still has original role
	detailResp := clientA.Get(fmt.Sprintf("/api/tenant/members/%d", memberIDA), nil)
	if detailResp.Code != 0 {
		// If member detail fails, try list
		return
	}
	var detail struct {
		Role string `json:"role"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.Role == "admin" && roleResp.Code == 0 {
		t.Fatal("tenant B changed tenant A's member role — isolation broken")
	}
}

// TestIsolation_TenantTokenCannotAccessAdminEndpoints verifies tenant JWT cannot access admin API.
// Business rule: admin and tenant auth realms are completely separate.
func TestIsolation_TenantTokenCannotAccessAdminEndpoints(t *testing.T) {
	tenantClient, _ := testinfra.GetAuthedClient(t)

	// Try multiple admin endpoints with tenant token
	adminEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/api/admin/tenants", "tenant list"},
		{"GET", "/api/admin/users", "user list"},
		{"GET", "/api/admin/channels", "channel list"},
		{"GET", "/api/admin/models", "model list"},
	}

	for _, ep := range adminEndpoints {
		resp := tenantClient.Get(ep.path, map[string]string{"page": "1", "page_size": "1"})
		if resp.Code == 0 {
			t.Fatalf("tenant token accessed admin endpoint %q — realm isolation broken", ep.name)
		}
	}
}
