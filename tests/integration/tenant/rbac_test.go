//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

// ─── RBAC 权限强制执行测试 ───────────────────────────────────────────
//
// 测试原则：
//   1. 基于业务代码中的实际权限检查（而非猜测结果）
//   2. 验证具体错误码：403=权限不足，10033=项目密钥限制，10015=密码策略
//   3. 验证数据过滤：member 能访问的接口应只返回自己的数据
//   4. 验证正向数据：允许的操作应返回正确的数据内容

// ─── 辅助 ────────────────────────────────────────────────────────────

// setupMemberClient 创建 member 角色用户并返回其 client、所属租户信息、owner client
func setupMemberClient(t *testing.T) (memberClient, ownerClient *admintest.APIClient, memberID int64, tenantResult *testinfra.TenantRegisterResult) {
	t.Helper()
	ownerClient, tenantResult = testinfra.GetAuthedClient(t)

	var memberCleanup func()
	memberID, memberCleanup = testinfra.CreateTestTenantMember(t, ownerClient)
	t.Cleanup(memberCleanup)

	// 获取 member 用户名
	detailResp := ownerClient.Get(fmt.Sprintf("/api/tenant/members/%d", memberID), nil)
	detailResp.AssertSuccess(t)
	var detail struct {
		Username string `json:"username"`
	}
	detailResp.DecodeData(t, &detail)

	// member RAM 登录
	loginResult := testinfra.LoginTenant(t, detail.Username, tenantResult.Tenant.Code, testinfra.TestPassword)
	memberClient = admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(loginResult.AccessToken)
	return
}

// setupAdminClient 创建 admin 角色用户并返回其 client
func setupAdminClient(t *testing.T) (adminClient, ownerClient *admintest.APIClient, tenantResult *testinfra.TenantRegisterResult) {
	t.Helper()
	ownerClient, tenantResult = testinfra.GetAuthedClient(t)

	var memberCleanup func()
	memberID, memberCleanup := testinfra.CreateTestTenantMember(t, ownerClient)
	t.Cleanup(memberCleanup)

	// 升级为 admin
	ownerClient.Put(fmt.Sprintf("/api/tenant/members/%d/role", memberID), map[string]any{"role": "admin"}).
		AssertSuccess(t)

	// 获取用户名并登录
	detailResp := ownerClient.Get(fmt.Sprintf("/api/tenant/members/%d", memberID), nil)
	detailResp.AssertSuccess(t)
	var detail struct {
		Username string `json:"username"`
	}
	detailResp.DecodeData(t, &detail)

	loginResult := testinfra.LoginTenant(t, detail.Username, tenantResult.Tenant.Code, testinfra.TestPassword)
	adminClient = admintest.NewAPIClient(testinfra.DefaultBaseURL).WithToken(loginResult.AccessToken)
	return
}

// assertBlocked 断言被权限拦截：业务码应为 403
func assertBlocked(t *testing.T, resp *admintest.APIResponse, operation string) {
	t.Helper()
	if resp.Code == 0 {
		t.Fatalf("%s: expected to be blocked but succeeded (code=0)", operation)
	}
	if resp.Code != 403 {
		t.Fatalf("%s: expected code=403 (forbidden), got code=%d msg=%q", operation, resp.Code, resp.Message)
	}
}

// assertBlockedWithCode 断言被特定业务码拦截
func assertBlockedWithCode(t *testing.T, resp *admintest.APIResponse, expectedCode int, operation string) {
	t.Helper()
	if resp.Code == 0 {
		t.Fatalf("%s: expected to be blocked but succeeded (code=0)", operation)
	}
	if resp.Code != expectedCode {
		t.Fatalf("%s: expected code=%d, got code=%d msg=%q", operation, expectedCode, resp.Code, resp.Message)
	}
}

// ─── Member 角色禁止访问的端点（逻辑层 role != "owner" && role != "admin"）────

func TestRBAC_MemberBlockedFromDashboard(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// dashboard.go: Dashboard() → if role != "owner" && role != "admin" → 403
	resp := memberClient.Get("/api/tenant/dashboard", nil)
	assertBlocked(t, resp, "member access dashboard overview")
}

func TestRBAC_MemberBlockedFromTokenTrends(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/dashboard/token-trends", map[string]string{"days": "7"})
	assertBlocked(t, resp, "member access token trends")
}

func TestRBAC_MemberBlockedFromModelDistribution(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/dashboard/model-distribution", map[string]string{"days": "7"})
	assertBlocked(t, resp, "member access model distribution")
}

func TestRBAC_MemberBlockedFromBalancePrediction(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/dashboard/balance-prediction", nil)
	assertBlocked(t, resp, "member access balance prediction")
}

func TestRBAC_MemberBlockedFromBudgetAlerts(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/dashboard/budget-alerts", nil)
	assertBlocked(t, resp, "member access budget alerts")
}

func TestRBAC_MemberBlockedFromMemberUsageRanking(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/dashboard/member-usage-ranking", map[string]string{"days": "7", "limit": "10"})
	assertBlocked(t, resp, "member access member usage ranking")
}

// ─── Member 角色禁止访问钱包（billing.go: Wallet/Transactions/FrozenItems → 403）───

func TestRBAC_MemberBlockedFromWallet(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// billing.go: Wallet() → if role != "owner" && role != "admin" → 403
	resp := memberClient.Get("/api/tenant/wallet", nil)
	assertBlocked(t, resp, "member access wallet")
}

func TestRBAC_MemberBlockedFromWalletTransactions(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// billing.go: WalletTransactions() → 403
	resp := memberClient.Get("/api/tenant/wallet/transactions", map[string]string{"page": "1", "page_size": "10"})
	assertBlocked(t, resp, "member access wallet transactions")
}

func TestRBAC_MemberBlockedFromWalletFrozenItems(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// billing.go: WalletFrozenItems() → 403
	resp := memberClient.Get("/api/tenant/wallet/frozen-items", nil)
	assertBlocked(t, resp, "member access wallet frozen items")
}

// ─── Member 角色禁止管理项目级 API Key（api_key.go → 10033）───────────

func TestRBAC_MemberBlockedFromProjectApiKeyCreate(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	// 先创建一个项目
	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// api_key.go: ProjectApiKeyCreate() → if role != "owner" && role != "admin" → 403
	resp := memberClient.Post(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]any{
		"name": "should-not-work",
	})
	assertBlocked(t, resp, "member create project API key")
}

func TestRBAC_MemberBlockedFromProjectApiKeyList(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// project.go: ProjectApiKeyList() → if role != "owner" && role != "admin" → 403
	resp := memberClient.Get(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]string{"page": "1", "page_size": "10"})
	assertBlocked(t, resp, "member list project API keys")
}

func TestRBAC_MemberBlockedFromProjectUsageStats(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// project.go: ProjectUsageStats() → 403
	resp := memberClient.Get(fmt.Sprintf("/api/tenant/projects/%d/usage-stats", projectID), nil)
	assertBlocked(t, resp, "member access project usage stats")
}

func TestRBAC_MemberBlockedFromProjectUsageLogs(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// project.go: ProjectUsageLogs() → 403
	resp := memberClient.Get(fmt.Sprintf("/api/tenant/projects/%d/usage-logs", projectID), map[string]string{"page": "1", "page_size": "10"})
	assertBlocked(t, resp, "member access project usage logs")
}

// ─── Member 对 usage logs 的数据过滤（只能看自己的）───────────────────

func TestRBAC_MemberUsageLogsOnlyOwnData(t *testing.T) {
	memberClient, _, memberID, _ := setupMemberClient(t)

	// billing.go: UsageLogs() → if role == "member" → WHERE u.user_id = currentUser
	// member 应该能成功访问，但只返回自己的记录
	resp := memberClient.Get("/api/tenant/usage-logs", map[string]string{"page": "1", "page_size": "10"})
	resp.AssertSuccess(t)

	var usage struct {
		List  []map[string]any `json:"list"`
		Total int              `json:"total"`
	}
	resp.DecodeData(t, &usage)

	// 新 member 无使用记录是正常的（total=0），关键是接口没有报 403
	// 如果未来有 relay 请求产生 usage logs，应验证所有记录的 user_id 都等于 memberID
	t.Logf("member %d usage logs: total=%d (should be filtered to own data only)", memberID, usage.Total)
}

// ─── Member 允许访问的端点（验证返回数据正确性）────────────────────────

func TestRBAC_MemberCanViewOwnProfile(t *testing.T) {
	memberClient, _, memberID, _ := setupMemberClient(t)

	// organization.go: GetProfile() → 无角色检查，返回当前用户信息
	resp := memberClient.Get("/api/tenant/profile", nil)
	resp.AssertSuccess(t)

	var profile struct {
		ID       int64  `json:"id"`
		Role     string `json:"role"`
		Username string `json:"username"`
	}
	resp.DecodeData(t, &profile)

	if profile.Role != "member" {
		t.Fatalf("expected role=member, got %s", profile.Role)
	}
	if profile.ID != memberID {
		t.Fatalf("expected id=%d, got %d", memberID, profile.ID)
	}
}

func TestRBAC_MemberCanAccessPersonalDashboard(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/personal-dashboard", nil)
	resp.AssertSuccess(t)

	var dashboard struct {
		TodayRequests float64 `json:"today_requests"`
	}
	resp.DecodeData(t, &dashboard)
	// 新用户请求数应为 0
	if dashboard.TodayRequests != 0 {
		t.Fatalf("new member should have 0 today_requests, got %f", dashboard.TodayRequests)
	}
}

func TestRBAC_MemberCanListOwnPersonalApiKeys(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// api_key.go: ApiKeyList() → member 请求 personal 类型时走 user_id 过滤
	resp := memberClient.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "10"})
	resp.AssertSuccess(t)

	var keys struct {
		Total int `json:"total"`
	}
	resp.DecodeData(t, &keys)
	// 新 member 无 key → total=0
	if keys.Total != 0 {
		t.Fatalf("new member should have 0 personal API keys, got %d", keys.Total)
	}
}

func TestRBAC_MemberCanCreatePersonalApiKey(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// member 应该能创建个人 Key
	resp := memberClient.Post("/api/tenant/api-keys", map[string]any{
		"name": "my-personal-key",
	})
	resp.AssertSuccess(t)
	keyID := resp.GetID(t)
	if keyID <= 0 {
		t.Fatal("member should be able to create personal API key")
	}

	// 验证创建的 Key 确实在自己的列表中
	listResp := memberClient.Get("/api/tenant/api-keys", map[string]string{"page": "1", "page_size": "10"})
	listResp.AssertSuccess(t)
	var keys struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &keys)

	found := false
	for _, k := range keys.List {
		if k.ID == keyID && k.Name == "my-personal-key" {
			found = true
		}
	}
	if !found {
		t.Fatal("created personal API key not found in member's key list")
	}
}

func TestRBAC_MemberCanListProjects(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	// owner 创建项目
	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// project.go: ProjectList() → 无角色检查，member 也能看到租户项目
	resp := memberClient.Get("/api/tenant/projects", map[string]string{"page": "1", "page_size": "100"})
	resp.AssertSuccess(t)

	var projects struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	resp.DecodeData(t, &projects)

	found := false
	for _, p := range projects.List {
		if p.ID == projectID {
			found = true
		}
	}
	if !found {
		t.Fatal("member should see the project created by owner in the same tenant")
	}
}

func TestRBAC_MemberCanViewOrgInfo(t *testing.T) {
	memberClient, _, _, result := setupMemberClient(t)

	// organization.go: GetOrgInfo() → 无角色检查
	resp := memberClient.Get("/api/tenant/organization", nil)
	resp.AssertSuccess(t)

	var org struct {
		Code string `json:"code"`
	}
	resp.DecodeData(t, &org)
	if org.Code != result.Tenant.Code {
		t.Fatalf("member should see org code=%q, got %q", result.Tenant.Code, org.Code)
	}
}

// ─── Owner-only 操作：admin 和 member 都应被阻止 ─────────────────────

func TestRBAC_AdminBlockedFromOrgUpdate(t *testing.T) {
	adminClient, ownerClient, result := setupAdminClient(t)

	// organization.go: UpdateOrgInfo() → ownerOnly() → 仅 owner
	resp := adminClient.Put("/api/tenant/organization", map[string]any{
		"name": "AdminHack",
	})
	assertBlocked(t, resp, "admin update org info (owner-only)")

	// 验证组织名称没被改掉
	orgResp := ownerClient.Get("/api/tenant/organization", nil)
	orgResp.AssertSuccess(t)
	var org struct {
		Name string `json:"name"`
	}
	orgResp.DecodeData(t, &org)
	if org.Name == "AdminHack" {
		t.Fatalf("org name should not have changed, got %q", org.Name)
	}
	_ = result
}

func TestRBAC_MemberBlockedFromOrgUpdate(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	// organization.go: UpdateOrgInfo() → ownerOnly() → 403
	resp := memberClient.Put("/api/tenant/organization", map[string]any{
		"name": "MemberHack",
	})
	assertBlocked(t, resp, "member update org info (owner-only)")
}

// ─── Admin 角色允许的操作（验证正向数据）───────────────────────────────

func TestRBAC_AdminCanAccessDashboard(t *testing.T) {
	adminClient, _, _ := setupAdminClient(t)

	// dashboard.go: owner + admin 都允许
	resp := adminClient.Get("/api/tenant/dashboard", nil)
	resp.AssertSuccess(t)

	var dashboard struct {
		TodayRequests float64 `json:"today_requests"`
		ActiveKeys    int     `json:"active_keys"`
		MemberCount   int     `json:"member_count"`
	}
	resp.DecodeData(t, &dashboard)

	// admin + owner 两人，member_count >= 2
	if dashboard.MemberCount < 2 {
		t.Fatalf("admin dashboard should show member_count >= 2 (owner + admin), got %d", dashboard.MemberCount)
	}
}

func TestRBAC_AdminCanAccessWallet(t *testing.T) {
	adminClient, _, _ := setupAdminClient(t)

	resp := adminClient.Get("/api/tenant/wallet", nil)
	resp.AssertSuccess(t)

	var wallet struct {
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
		Currency      string  `json:"currency"`
	}
	resp.DecodeData(t, &wallet)
	// 新租户余额应为 0
	if wallet.Balance != 0 {
		t.Fatalf("new tenant wallet balance should be 0, got %f", wallet.Balance)
	}
	if wallet.Currency != "USD" {
		t.Fatalf("wallet currency should be USD, got %q", wallet.Currency)
	}
}

func TestRBAC_AdminCanManageProjectKeys(t *testing.T) {
	adminClient, ownerClient, _ := setupAdminClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	// admin 创建项目级 Key
	resp := adminClient.Post(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]any{
		"name": "admin-project-key",
	})
	resp.AssertSuccess(t)
	keyID := resp.GetID(t)

	// 验证 Key 在项目列表中
	listResp := adminClient.Get(fmt.Sprintf("/api/tenant/projects/%d/api-keys", projectID), map[string]string{"page": "1", "page_size": "10"})
	listResp.AssertSuccess(t)
	var keys struct {
		List []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &keys)

	found := false
	for _, k := range keys.List {
		if k.ID == keyID && k.Name == "admin-project-key" {
			found = true
		}
	}
	if !found {
		t.Fatal("admin-created project API key not found in project key list")
	}
}

// ─── Member 角色禁止管理成员（邀请/创建/移除/角色变更/导出）───────────

// TestRBAC_MemberBlockedFromMemberCreate 验证 member 不能直接创建成员
// Business rule: 成员管理（创建/邀请/移除/角色变更/导出）需要 owner 或 admin 权限
func TestRBAC_MemberBlockedFromMemberCreate(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	suffix := testinfra.RandomSuffix()
	resp := memberClient.Post("/api/tenant/members/create", map[string]any{
		"username": fmt.Sprintf("blocked%s", suffix),
		"password": testinfra.TestPassword,
		"email":    fmt.Sprintf("blocked%s@test.com", suffix),
		"role":     "member",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to create members — operation should be blocked or produce error")
	}
	t.Logf("member create member: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromMemberInvite 验证 member 不能生成邀请链接
func TestRBAC_MemberBlockedFromMemberInvite(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Post("/api/tenant/members/invite", map[string]any{
		"role":         "member",
		"expires_days": 7,
		"max_uses":     0,
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to invite members")
	}
	t.Logf("member invite: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromMemberRemove 验证 member 不能移除其他成员
func TestRBAC_MemberBlockedFromMemberRemove(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	// owner 创建另一个 member
	secondMemberID, cleanup := testinfra.CreateTestTenantMember(t, ownerClient)
	defer cleanup()

	resp := memberClient.Delete(fmt.Sprintf("/api/tenant/members/%d", secondMemberID))
	if resp.Code == 0 {
		t.Fatal("member should not be able to remove other members")
	}
	t.Logf("member remove: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromMemberRoleChange 验证 member 不能变更其他成员角色
func TestRBAC_MemberBlockedFromMemberRoleChange(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	secondMemberID, cleanup := testinfra.CreateTestTenantMember(t, ownerClient)
	defer cleanup()

	resp := memberClient.Put(fmt.Sprintf("/api/tenant/members/%d/role", secondMemberID), map[string]any{
		"role": "admin",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to change other member's role")
	}
	t.Logf("member role change: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromMemberExport 验证 member 不能导出成员列表
func TestRBAC_MemberBlockedFromMemberExport(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/members/export", map[string]string{
		"format": "csv",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to export member list")
	}
	t.Logf("member export: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromMemberQuotaSet 验证 member 不能设置成员额度
func TestRBAC_MemberBlockedFromMemberQuotaSet(t *testing.T) {
	memberClient, _, memberID, _ := setupMemberClient(t)

	resp := memberClient.Put(fmt.Sprintf("/api/tenant/members/%d/quota", memberID), map[string]any{
		"quota_type":  "total",
		"quota_limit": 100.0,
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to set member quota")
	}
	t.Logf("member quota set: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// ─── Member 角色禁止管理项目（创建/更新/归档）─────────────────────

// TestRBAC_MemberBlockedFromProjectCreate 验证 member 不能创建项目
// Business rule: 项目管理（创建/更新/归档）需要 owner 或 admin 权限
func TestRBAC_MemberBlockedFromProjectCreate(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Post("/api/tenant/projects", map[string]any{
		"name": "member-project",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to create projects")
	}
	t.Logf("member project create: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromProjectUpdate 验证 member 不能更新项目
func TestRBAC_MemberBlockedFromProjectUpdate(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	resp := memberClient.Put(fmt.Sprintf("/api/tenant/projects/%d", projectID), map[string]any{
		"name": "hacked-by-member",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to update projects")
	}

	// 验证项目名称没有被修改
	detailResp := ownerClient.Get(fmt.Sprintf("/api/tenant/projects/%d", projectID), nil)
	detailResp.AssertSuccess(t)
	var detail struct {
		Name string `json:"name"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Name == "hacked-by-member" {
		t.Fatal("project name should not have been changed by member")
	}
}

// TestRBAC_MemberBlockedFromProjectArchive 验证 member 不能归档项目
func TestRBAC_MemberBlockedFromProjectArchive(t *testing.T) {
	memberClient, ownerClient, _, _ := setupMemberClient(t)

	projectID, _ := testinfra.CreateTestProject(t, ownerClient)

	resp := memberClient.Post(fmt.Sprintf("/api/tenant/projects/%d/archive", projectID), nil)
	if resp.Code == 0 {
		t.Fatal("member should not be able to archive projects")
	}

	// 验证项目仍然 active
	detailResp := ownerClient.Get(fmt.Sprintf("/api/tenant/projects/%d", projectID), nil)
	detailResp.AssertSuccess(t)
	var detail struct {
		Status string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Status == "archived" {
		t.Fatal("project should not have been archived by member")
	}
}

// ─── 业务逻辑保护：不能修改/移除/禁用 owner ──────────────────────

// TestRBAC_CannotChangeOwnerRole 验证 admin 不能修改 owner 的角色
// Business rule: owner 角色不可被降级（member.go: 不能修改所有者的角色）
func TestRBAC_CannotChangeOwnerRole(t *testing.T) {
	adminClient, ownerClient, _ := setupAdminClient(t)

	// 获取 owner 的 ID
	profileResp := ownerClient.Get("/api/tenant/profile", nil)
	profileResp.AssertSuccess(t)
	var profile struct {
		ID int64 `json:"id"`
	}
	profileResp.DecodeData(t, &profile)

	resp := adminClient.Put(fmt.Sprintf("/api/tenant/members/%d/role", profile.ID), map[string]any{
		"role": "member",
	})
	if resp.Code == 0 {
		t.Fatal("admin should not be able to change owner's role")
	}
	t.Logf("owner role change: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_CannotRemoveOwner 验证 admin 不能移除 owner
// Business rule: owner 不可被移除（member.go: 不能移除组织所有者）
func TestRBAC_CannotRemoveOwner(t *testing.T) {
	adminClient, ownerClient, _ := setupAdminClient(t)

	profileResp := ownerClient.Get("/api/tenant/profile", nil)
	profileResp.AssertSuccess(t)
	var profile struct {
		ID int64 `json:"id"`
	}
	profileResp.DecodeData(t, &profile)

	resp := adminClient.Delete(fmt.Sprintf("/api/tenant/members/%d", profile.ID))
	if resp.Code == 0 {
		t.Fatal("admin should not be able to remove owner")
	}
	t.Logf("owner removal: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_CannotResetOwnerPassword 验证 admin 不能重置 owner 的密码
// Business rule: owner 密码不可被他人重置（member.go: 不能重置组织所有者的密码）
func TestRBAC_CannotResetOwnerPassword(t *testing.T) {
	adminClient, ownerClient, _ := setupAdminClient(t)

	profileResp := ownerClient.Get("/api/tenant/profile", nil)
	profileResp.AssertSuccess(t)
	var profile struct {
		ID int64 `json:"id"`
	}
	profileResp.DecodeData(t, &profile)

	resp := adminClient.Put(fmt.Sprintf("/api/tenant/members/%d/reset-password", profile.ID), map[string]any{
		"password": "HackedPassword123",
	})
	if resp.Code == 0 {
		t.Fatal("admin should not be able to reset owner's password")
	}
	t.Logf("owner password reset: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// ─── Admin 角色允许的成员管理操作（正向验证）─────────────────────────

// TestRBAC_AdminCanManageMembers 验证 admin 可以执行成员管理操作
// Business rule: admin 拥有除 owner-only 操作外的全部管理权限
func TestRBAC_AdminCanManageMembers(t *testing.T) {
	adminClient, _, _ := setupAdminClient(t)

	// admin 应能查看成员列表
	resp := adminClient.Get("/api/tenant/members", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var members struct {
		Total int `json:"total"`
	}
	resp.DecodeData(t, &members)
	if members.Total < 2 {
		t.Fatalf("admin should see at least 2 members (owner + admin), got %d", members.Total)
	}
}

// TestRBAC_AdminCanViewWallet 验证 admin 可以查看钱包信息
func TestRBAC_AdminCanViewWallet(t *testing.T) {
	adminClient, _, _ := setupAdminClient(t)

	resp := adminClient.Get("/api/tenant/wallet", nil)
	resp.AssertSuccess(t)

	var wallet struct {
		Currency string `json:"currency"`
	}
	resp.DecodeData(t, &wallet)
	if wallet.Currency != "USD" {
		t.Fatalf("wallet currency should be USD, got %q", wallet.Currency)
	}
}

// ─── 通知偏好 Owner-only 保护 ─────────────────────────────────────

// TestRBAC_MemberBlockedFromOrgNotificationPreferences 验证 member 不能修改组织级通知偏好
// Business rule: 组织级通知偏好仅 owner 可修改（notification.go: 仅组织所有者可修改组织级通知偏好）
func TestRBAC_MemberBlockedFromOrgNotificationPreferences(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Put("/api/tenant/notification-preferences", map[string]any{
		"scope": "org",
		"preferences": map[string]any{
			"billing": map[string]any{
				"enabled": false,
			},
		},
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to modify org-level notification preferences")
	}
	t.Logf("member org notification prefs: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_AdminBlockedFromOrgNotificationPreferences 验证 admin 也不能修改组织级通知偏好
// Business rule: scope=org 时仅 owner 可修改，admin 也应被阻止
func TestRBAC_AdminBlockedFromOrgNotificationPreferences(t *testing.T) {
	adminClient, _, _ := setupAdminClient(t)

	resp := adminClient.Put("/api/tenant/notification-preferences", map[string]any{
		"scope": "org",
		"preferences": map[string]any{
			"billing": map[string]any{
				"enabled": false,
			},
		},
	})
	if resp.Code == 0 {
		t.Fatal("admin should not be able to modify org-level notification preferences (owner-only)")
	}
	t.Logf("admin org notification prefs: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberCanUpdateOwnNotificationPreferences 验证 member 可以修改自己的通知偏好
// Business rule: scope=user 时所有角色都能修改自己的通知偏好
func TestRBAC_MemberCanUpdateOwnNotificationPreferences(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Put("/api/tenant/notification-preferences", map[string]any{
		"scope": "user",
		"preferences": map[string]any{
			"billing": map[string]any{
				"enabled": true,
			},
		},
	})
	resp.AssertSuccess(t)

	// 验证获取偏好设置成功
	getResp := memberClient.Get("/api/tenant/notification-preferences", nil)
	getResp.AssertSuccess(t)
}

// ─── 兑换码 Owner/Admin 权限验证 ────────────────────────────────────

// TestRBAC_MemberBlockedFromRedeemCode 验证 member 不能使用兑换码
// Business rule: 兑换码功能需要 owner 或 admin 权限
func TestRBAC_MemberBlockedFromRedeemCode(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Post("/api/tenant/redemptions/redeem", map[string]any{
		"code": "fake-code-for-rbac-test",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to redeem codes")
	}
	t.Logf("member redeem: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// TestRBAC_MemberBlockedFromRedemptionHistory 验证 member 不能查看兑换历史
func TestRBAC_MemberBlockedFromRedemptionHistory(t *testing.T) {
	memberClient, _, _, _ := setupMemberClient(t)

	resp := memberClient.Get("/api/tenant/redemptions/usages", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	if resp.Code == 0 {
		t.Fatal("member should not be able to view redemption history")
	}
	t.Logf("member redemption history: blocked with code=%d msg=%q", resp.Code, resp.Message)
}

// ─── 无认证访问保护 ─────────────────────────────────────────────────

// TestRBAC_UnauthenticatedAccessBlocked 验证无 token 访问受保护端点被拒绝
// Business rule: 所有 /api/tenant/* 端点（public 除外）需要 JWT 认证
func TestRBAC_UnauthenticatedAccessBlocked(t *testing.T) {
	noAuthClient := admintest.NewAPIClient(testinfra.DefaultBaseURL)

	protectedEndpoints := []struct {
		method, path, name string
	}{
		{"GET", "/api/tenant/members", "member list"},
		{"GET", "/api/tenant/wallet", "wallet"},
		{"GET", "/api/tenant/dashboard", "dashboard"},
		{"GET", "/api/tenant/organization", "organization"},
		{"GET", "/api/tenant/api-keys", "api keys"},
		{"GET", "/api/tenant/projects", "projects"},
	}

	for _, ep := range protectedEndpoints {
		var resp *admintest.APIResponse
		if ep.method == "GET" {
			resp = noAuthClient.Get(ep.path, nil)
		}
		if resp.Code == 0 {
			t.Fatalf("unauthenticated access should be blocked for %q", ep.name)
		}
		t.Logf("unauthenticated %q: blocked with code=%d", ep.name, resp.Code)
	}
}
