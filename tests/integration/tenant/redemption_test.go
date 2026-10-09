//go:build integration

package tenant_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

// TestRedeemCodeMultiUsePerTenantOnce 多用途兑换码端到端：
// 同一租户重复兑换被拒、不同租户互不影响、额度耗尽后拒绝。
// 兑换码经 DB 直接种子（免 admin 凭据依赖），兑换链路走真实 API。
func TestRedeemCodeMultiUsePerTenantOnce(t *testing.T) {
	codeID, code := seedMultiUseRedemption(t)
	defer admintest.HardDeleteRedemption(t, codeID)

	// 租户 A 首次兑换：成功且入账面值
	clientA, _ := testinfra.GetAuthedClient(t)
	redeemResp := clientA.Post("/api/tenant/redemptions/redeem", map[string]any{"code": code})
	redeemResp.AssertSuccess(t)
	var redeemData struct {
		Credited float64 `json:"credited"`
	}
	redeemResp.DecodeData(t, &redeemData)
	if redeemData.Credited != 5.0 {
		t.Fatalf("expected credited=5.0, got %v", redeemData.Credited)
	}

	// 租户 A 重复兑换：被拒（每码每租户一次）
	dupResp := clientA.Post("/api/tenant/redemptions/redeem", map[string]any{"code": code})
	if dupResp.Code == 0 {
		t.Fatal("expected duplicate redeem to fail, got success")
	}
	if !strings.Contains(dupResp.Message, "已兑换过") {
		t.Fatalf("expected duplicate-reject message, got code=%d message=%q", dupResp.Code, dupResp.Message)
	}

	// 租户 B 兑换同一码：成功（不同租户互不影响，耗尽第二个名额）
	clientB, _ := testinfra.GetAuthedClient(t)
	redeemRespB := clientB.Post("/api/tenant/redemptions/redeem", map[string]any{"code": code})
	redeemRespB.AssertSuccess(t)

	// 租户 C 兑换：额度耗尽被拒
	clientC, _ := testinfra.GetAuthedClient(t)
	exhaustedResp := clientC.Post("/api/tenant/redemptions/redeem", map[string]any{"code": code})
	if exhaustedResp.Code == 0 {
		t.Fatal("expected redeem to fail after max_uses exhausted, got success")
	}
	if !strings.Contains(exhaustedResp.Message, "已全部使用") {
		t.Fatalf("expected exhausted message, got code=%d message=%q", exhaustedResp.Code, exhaustedResp.Message)
	}
}

// seedMultiUseRedemption 直接向数据库种子一条 max_uses=2、面值 5 的 quota 兑换码
func seedMultiUseRedemption(t *testing.T) (int64, string) {
	t.Helper()
	code := strings.ToUpper(testinfra.RandomSuffix() + testinfra.RandomSuffix())

	id, err := g.DB().Model("ord_redemptions").Ctx(context.Background()).Data(g.Map{
		"code":       code,
		"type":       "quota",
		"value":      5,
		"max_uses":   2,
		"status":     "active",
		"expires_at": gtime.Now().Add(7 * 24 * time.Hour),
		"batch_no":   "IT-TEST",
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("seed redemption code error: %v", err)
	}
	return id, code
}
