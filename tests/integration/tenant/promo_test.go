//go:build integration

package tenant_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

// promoCodeFixture 直插 DB 的优惠码测试数据（服务端查询 ord_promo_codes 无缓存，立即可见）
type promoCodeFixture struct {
	code          string
	typ           string // percentage / fixed
	discountValue float64
	minAmount     float64
	maxDiscount   float64
	totalCount    int
	usedCount     int
	perUserLimit  int
	planIDs       *string // postgres 数组字面量（如 '{1,2}'），nil 表示 NULL（不限）
	validFromExpr string  // 相对时间 SQL 表达式，空则默认「1 小时前生效」
	validToExpr   string  // 空则默认「1 天后失效」
	status        string  // 空则默认 active
}

// insertPromoCode 插入优惠码并注册清理，返回 promo id
func insertPromoCode(t *testing.T, f promoCodeFixture) int64 {
	t.Helper()
	if f.status == "" {
		f.status = "active"
	}
	if f.validFromExpr == "" {
		f.validFromExpr = "NOW() - INTERVAL '1 hour'"
	}
	if f.validToExpr == "" {
		f.validToExpr = "NOW() + INTERVAL '1 day'"
	}
	var planIDs any
	if f.planIDs != nil {
		planIDs = *f.planIDs
	}
	ctx := context.Background()
	id, err := g.DB().Model("ord_promo_codes").Ctx(ctx).Data(g.Map{
		"code":           f.code,
		"name":           "集成测试优惠码 " + f.code,
		"type":           f.typ,
		"discount_value": f.discountValue,
		"min_amount":     f.minAmount,
		"max_discount":   f.maxDiscount,
		"total_count":    f.totalCount,
		"used_count":     f.usedCount,
		"per_user_limit": f.perUserLimit,
		"plan_ids":       planIDs,
		"valid_from":     gdb.Raw(f.validFromExpr),
		"valid_to":       gdb.Raw(f.validToExpr),
		"status":         f.status,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert promo code %q: %v", f.code, err)
	}
	t.Cleanup(func() {
		g.DB().Exec(ctx, "DELETE FROM ord_promo_code_usages WHERE promo_code_id = $1", id)
		g.DB().Exec(ctx, "DELETE FROM ord_promo_codes WHERE id = $1", id)
	})
	return id
}

// validateResp 预检端点响应的业务字段
type validateResp struct {
	PromoCodeId int64   `json:"promo_code_id"`
	Type        string  `json:"type"`
	Discount    float64 `json:"discount"`
	FinalAmount float64 `json:"final_amount"`
}

func callValidate(client *admintest.APIClient, code string, amount float64) *admintest.APIResponse {
	return client.Post("/api/tenant/promo-codes/validate", map[string]any{
		"code":   code,
		"amount": amount,
	})
}

// assertBusinessError 断言业务失败且提示包含指定关键词
func assertBusinessError(t *testing.T, resp *admintest.APIResponse, msgPart string) {
	t.Helper()
	if resp.Code == 0 {
		t.Fatalf("expected business error containing %q, got success", msgPart)
	}
	if !strings.Contains(resp.Message, msgPart) {
		t.Fatalf("expected error message containing %q, got %q", msgPart, resp.Message)
	}
}

// TestPromoValidateFixed 立减券：折扣与实付金额换算
func TestPromoValidateFixed(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)
	code := fmt.Sprintf("FIX-%s", testinfra.RandomSuffix())
	insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 20, minAmount: 50})

	resp := callValidate(client, code, 100)
	resp.AssertSuccess(t)
	var data validateResp
	resp.DecodeData(t, &data)
	if data.Discount != 20 {
		t.Fatalf("expected discount=20, got %v", data.Discount)
	}
	if data.FinalAmount != 80 {
		t.Fatalf("expected final_amount=80, got %v", data.FinalAmount)
	}
	if data.Type != "fixed" {
		t.Fatalf("expected type=fixed, got %q", data.Type)
	}
}

// TestPromoValidatePercentage 百分比券（含最大折扣封顶）
func TestPromoValidatePercentage(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	t.Run("按百分比计算", func(t *testing.T) {
		code := fmt.Sprintf("PCT-%s", testinfra.RandomSuffix())
		insertPromoCode(t, promoCodeFixture{code: code, typ: "percentage", discountValue: 10})
		resp := callValidate(client, code, 200)
		resp.AssertSuccess(t)
		var data validateResp
		resp.DecodeData(t, &data)
		if data.Discount != 20 || data.FinalAmount != 180 {
			t.Fatalf("expected discount=20 final=180, got discount=%v final=%v", data.Discount, data.FinalAmount)
		}
	})

	t.Run("最大折扣封顶", func(t *testing.T) {
		code := fmt.Sprintf("CAP-%s", testinfra.RandomSuffix())
		insertPromoCode(t, promoCodeFixture{code: code, typ: "percentage", discountValue: 50, maxDiscount: 30})
		resp := callValidate(client, code, 100)
		resp.AssertSuccess(t)
		var data validateResp
		resp.DecodeData(t, &data)
		if data.Discount != 30 || data.FinalAmount != 70 {
			t.Fatalf("expected capped discount=30 final=70, got discount=%v final=%v", data.Discount, data.FinalAmount)
		}
	})
}

// TestPromoValidateNegative 各类无效优惠码的拒绝路径
func TestPromoValidateNegative(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)
	suffix := testinfra.RandomSuffix()

	t.Run("优惠码不存在", func(t *testing.T) {
		assertBusinessError(t, callValidate(client, "NO-SUCH-"+suffix, 100), "优惠码无效")
	})

	t.Run("已禁用", func(t *testing.T) {
		code := fmt.Sprintf("DIS-%s", suffix)
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, status: "disabled"})
		assertBusinessError(t, callValidate(client, code, 100), "状态异常")
	})

	t.Run("不在有效期", func(t *testing.T) {
		code := fmt.Sprintf("EXP-%s", suffix)
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, validFromExpr: "NOW() - INTERVAL '2 days'", validToExpr: "NOW() - INTERVAL '1 day'"})
		assertBusinessError(t, callValidate(client, code, 100), "有效期")
	})

	t.Run("总量已耗尽", func(t *testing.T) {
		code := fmt.Sprintf("EXH-%s", suffix)
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, totalCount: 10, usedCount: 10})
		assertBusinessError(t, callValidate(client, code, 100), "已被全部使用")
	})

	t.Run("金额不足门槛", func(t *testing.T) {
		code := fmt.Sprintf("MIN-%s", suffix)
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, minAmount: 50})
		assertBusinessError(t, callValidate(client, code, 30), "不能低于")
	})

	t.Run("套餐限定券不可用于充值", func(t *testing.T) {
		code := fmt.Sprintf("PLN-%s", suffix)
		planIDs := "{1,2}"
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, planIDs: &planIDs})
		assertBusinessError(t, callValidate(client, code, 100), "不适用于充值")
	})
}

// TestPromoValidatePerTenantLimit 每租户限用：已有用量记录后同租户再验被拒
func TestPromoValidatePerTenantLimit(t *testing.T) {
	client, reg := testinfra.GetAuthedClient(t)
	code := fmt.Sprintf("LMT-%s", testinfra.RandomSuffix())
	promoID := insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, perUserLimit: 1})

	ctx := context.Background()
	if _, err := g.DB().Exec(ctx,
		"INSERT INTO ord_promo_code_usages (promo_code_id, tenant_id, order_id, user_id, discount_amount) VALUES ($1, $2, 0, 0, 5)",
		promoID, reg.Tenant.ID); err != nil {
		t.Fatalf("seed promo usage: %v", err)
	}

	assertBusinessError(t, callValidate(client, code, 100), "上限")
}

// TestRechargeCreatePromoPrecheck 下单侧优惠码预检：无效/套餐限定优惠码在渠道配置校验前被拒，
// 且不产生订单与优惠码用量副作用（无需支付渠道配置即可覆盖此路径）
func TestRechargeCreatePromoPrecheck(t *testing.T) {
	client, reg := testinfra.GetAuthedClient(t)
	ctx := context.Background()

	countOrders := func() int64 {
		v, err := g.DB().Model("ord_orders").Ctx(ctx).Where("tenant_id", reg.Tenant.ID).Count()
		if err != nil {
			t.Fatalf("count orders: %v", err)
		}
		return int64(v)
	}
	countUsages := func() int64 {
		v, err := g.DB().Model("ord_promo_code_usages").Ctx(ctx).Where("tenant_id", reg.Tenant.ID).Count()
		if err != nil {
			t.Fatalf("count usages: %v", err)
		}
		return int64(v)
	}

	rechargePayload := func(code string) map[string]any {
		return map[string]any{
			"amount":          100,
			"payment_channel": "epay",
			"payment_method":  "alipay",
			"promo_code":      code,
		}
	}

	t.Run("无效优惠码先于渠道校验失败", func(t *testing.T) {
		beforeOrders, beforeUsages := countOrders(), countUsages()
		resp := client.Post("/api/tenant/recharge/create", rechargePayload("NO-SUCH-"+testinfra.RandomSuffix()))
		assertBusinessError(t, resp, "优惠码无效")
		if after := countOrders(); after != beforeOrders {
			t.Fatalf("no order should be created, before=%d after=%d", beforeOrders, after)
		}
		if after := countUsages(); after != beforeUsages {
			t.Fatalf("no promo usage should be recorded, before=%d after=%d", beforeUsages, after)
		}
	})

	t.Run("套餐限定券被充值下单拒绝", func(t *testing.T) {
		code := fmt.Sprintf("RPL-%s", testinfra.RandomSuffix())
		planIDs := "{1}"
		insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 5, planIDs: &planIDs})
		assertBusinessError(t, client.Post("/api/tenant/recharge/create", rechargePayload(code)), "不适用于充值")
	})
}

// TestRechargeCreateWithPromoE2E 充值下单全链路：建单（优惠码折扣落订单字段）→ 用量记账 → 取消释放。
// 需要支付渠道配置（CreatePayment 为本地 URL 构造，不会真实发起支付）；未配置的环境自动跳过。
func TestRechargeCreateWithPromoE2E(t *testing.T) {
	client, reg := testinfra.GetAuthedClient(t)
	ctx := context.Background()

	// 探测可用支付渠道与档位折扣（期望值随环境配置动态计算，避免测试耦合具体配置）
	info := client.Get("/api/tenant/payment-info", nil)
	info.AssertSuccess(t)
	var payInfo struct {
		Channels []struct {
			Channel    string `json:"channel"`
			PayMethods []struct {
				Type string `json:"type"`
			} `json:"pay_methods"`
		} `json:"channels"`
		AmountDiscount map[string]float64 `json:"amount_discount"`
	}
	info.DecodeData(t, &payInfo)
	if len(payInfo.Channels) == 0 || len(payInfo.Channels[0].PayMethods) == 0 {
		t.Skip("支付渠道未配置，跳过充值优惠码 E2E")
	}
	channel := payInfo.Channels[0].Channel
	method := payInfo.Channels[0].PayMethods[0].Type

	code := fmt.Sprintf("E2E-%s", testinfra.RandomSuffix())
	promoID := insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 50})

	queryUsedCount := func() int {
		v, err := g.DB().Model("ord_promo_codes").Ctx(ctx).Where("id", promoID).Value("used_count")
		if err != nil {
			t.Fatalf("query used_count: %v", err)
		}
		return v.Int()
	}
	usageCount := func() int {
		v, err := g.DB().Model("ord_promo_code_usages").Ctx(ctx).
			Where("promo_code_id", promoID).Where("tenant_id", reg.Tenant.ID).Count()
		if err != nil {
			t.Fatalf("query usages: %v", err)
		}
		return v
	}

	// 用非整数金额 1234.56 避开档位折扣命中，隔离验证优惠码折扣本身
	resp := client.Post("/api/tenant/recharge/create", map[string]any{
		"amount":          1234.56,
		"payment_channel": channel,
		"payment_method":  method,
		"promo_code":      code,
	})
	resp.AssertSuccess(t)
	var created struct {
		OrderID     int64   `json:"order_id"`
		PaymentURL  string  `json:"payment_url"`
		FinalAmount float64 `json:"final_amount"`
	}
	resp.DecodeData(t, &created)
	if created.OrderID == 0 || created.PaymentURL == "" {
		t.Fatalf("expected order id and payment url, got %+v", created)
	}
	admintest.AssertFloatEqual(t, 1184.56, created.FinalAmount, 0.0001, "final_amount")

	// 订单字段：amount=原价、discount_amount=优惠码折扣、final=折后
	order, err := g.DB().Model("ord_orders").Ctx(ctx).Where("id", created.OrderID).One()
	if err != nil {
		t.Fatalf("query order: %v", err)
	}
	admintest.AssertFloatEqual(t, 1234.56, order["amount"].Float64(), 0.0001, "order.amount")
	admintest.AssertFloatEqual(t, 50, order["discount_amount"].Float64(), 0.0001, "order.discount_amount")
	admintest.AssertFloatEqual(t, 1184.56, order["final_amount"].Float64(), 0.0001, "order.final_amount")
	if order["status"].String() != "pending" {
		t.Fatalf("expected order status=pending, got %q", order["status"].String())
	}
	// 优惠码用量已记账
	if n := usageCount(); n != 1 {
		t.Fatalf("expected 1 promo usage after create, got %d", n)
	}
	if n := queryUsedCount(); n != 1 {
		t.Fatalf("expected promo used_count=1 after create, got %d", n)
	}

	// 取消订单：事务内释放优惠码用量（名额归还）
	cancelResp := client.Post(fmt.Sprintf("/api/tenant/orders/%d/cancel", created.OrderID), nil)
	cancelResp.AssertSuccess(t)
	if n := usageCount(); n != 0 {
		t.Fatalf("expected 0 promo usage after cancel, got %d", n)
	}
	if n := queryUsedCount(); n != 0 {
		t.Fatalf("expected promo used_count=0 after cancel, got %d", n)
	}
	status, err := g.DB().Model("ord_orders").Ctx(ctx).Where("id", created.OrderID).Value("status")
	if err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if status.String() != "cancelled" {
		t.Fatalf("expected order status=cancelled, got %q", status.String())
	}

	// 档位折扣 + 优惠码叠加口径：若环境配置了 1000 档折扣，优惠码应作用于档位折后基数
	if ratio, ok := payInfo.AmountDiscount["1000"]; ok && ratio > 0 {
		code2 := fmt.Sprintf("E2T-%s", testinfra.RandomSuffix())
		insertPromoCode(t, promoCodeFixture{code: code2, typ: "fixed", discountValue: 50})
		resp2 := client.Post("/api/tenant/recharge/create", map[string]any{
			"amount":          1000,
			"payment_channel": channel,
			"payment_method":  method,
			"promo_code":      code2,
		})
		resp2.AssertSuccess(t)
		var created2 struct {
			OrderID     int64   `json:"order_id"`
			FinalAmount float64 `json:"final_amount"`
		}
		resp2.DecodeData(t, &created2)
		// 期望：1000 × ratio（档位折后基数）− 50（优惠码）
		admintest.AssertFloatEqual(t, 1000*ratio-50, created2.FinalAmount, 0.0001, "tier+promo final_amount")
		// 清理：取消第二单释放用量
		client.Post(fmt.Sprintf("/api/tenant/orders/%d/cancel", created2.OrderID), nil)
	}
}
