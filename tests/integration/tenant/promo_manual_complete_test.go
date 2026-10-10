//go:build integration

package tenant_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

// TestRechargePromoManualCompleteE2E 优惠码充值「下单不支付 → 管理后台手动完成」全链路：
// 预检 → 建单（pending，不真实支付）→ admin 手动完成 → 订单 fulfilled、钱包按订单原价入账
// （付折后价、按原价入账的折扣让利语义）、优惠码用量保持已消耗（履约不释放名额）。
// 依赖 TEST_ADMIN_USERNAME/TEST_ADMIN_PASSWORD 与已配置的支付渠道，缺失时自动跳过。
func TestRechargePromoManualCompleteE2E(t *testing.T) {
	if os.Getenv("TEST_ADMIN_USERNAME") == "" || os.Getenv("TEST_ADMIN_PASSWORD") == "" {
		t.Skip("未配置 TEST_ADMIN_USERNAME/TEST_ADMIN_PASSWORD，跳过手动完成 E2E")
	}

	client, _ := testinfra.GetAuthedClient(t)
	ctx := context.Background()

	// 支付渠道探测（建单需要；CreatePayment 仅本地构造跳转 URL，不会真实发起支付）
	info := client.Get("/api/tenant/payment-info", nil)
	info.AssertSuccess(t)
	var payInfo struct {
		Channels []struct {
			Channel    string `json:"channel"`
			PayMethods []struct {
				Type string `json:"type"`
			} `json:"pay_methods"`
		} `json:"channels"`
	}
	info.DecodeData(t, &payInfo)
	if len(payInfo.Channels) == 0 || len(payInfo.Channels[0].PayMethods) == 0 {
		t.Skip("支付渠道未配置，跳过手动完成 E2E")
	}
	channel := payInfo.Channels[0].Channel
	method := payInfo.Channels[0].PayMethods[0].Type

	// 本位币非 CNY 的部署履约走换汇入账，金额断言口径不同，仅 CNY 环境执行
	walletResp := client.Get("/api/tenant/wallet", nil)
	walletResp.AssertSuccess(t)
	var wallet struct {
		Balance  float64 `json:"balance"`
		Currency string  `json:"currency"`
	}
	walletResp.DecodeData(t, &wallet)
	if wallet.Currency != "CNY" {
		t.Skipf("本位币为 %s（非 CNY），入账金额口径不同，跳过", wallet.Currency)
	}
	balanceBefore := wallet.Balance

	// fixed 50 券 + 非整数金额 1234.56 避开档位折扣，隔离验证优惠码折扣本身
	code := fmt.Sprintf("MC-%s", testinfra.RandomSuffix())
	promoID := insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 50})
	defer admintest.HardDeletePromoCode(t, promoID)

	// 层1 预检：折扣 50、折后 1184.56
	val := callValidate(client, code, 1234.56)
	val.AssertSuccess(t)
	var valData struct {
		Discount    float64 `json:"discount"`
		FinalAmount float64 `json:"final_amount"`
	}
	val.DecodeData(t, &valData)
	admintest.AssertFloatEqual(t, 50, valData.Discount, 0.0001, "validate.discount")
	admintest.AssertFloatEqual(t, 1184.56, valData.FinalAmount, 0.0001, "validate.final_amount")

	// 层2 下单不支付：pending 订单，折扣字段落库
	createResp := client.Post("/api/tenant/recharge/create", map[string]any{
		"amount":          1234.56,
		"payment_channel": channel,
		"payment_method":  method,
		"promo_code":      code,
	})
	createResp.AssertSuccess(t)
	var created struct {
		OrderID     int64   `json:"order_id"`
		FinalAmount float64 `json:"final_amount"`
	}
	createResp.DecodeData(t, &created)
	admintest.AssertFloatEqual(t, 1184.56, created.FinalAmount, 0.0001, "create.final_amount")

	// 层3a 管理后台手动完成：pending → paid → fulfilled
	adminClient := admintest.GetAuthedClient(t)
	completeResp := adminClient.Post(fmt.Sprintf("/api/admin/orders/%d/complete", created.OrderID), nil)
	completeResp.AssertSuccess(t)

	// 订单已履约，支付流水号标记为管理员手动操作（可审计）
	order, err := g.DB().Model("ord_orders").Ctx(ctx).Where("id", created.OrderID).One()
	if err != nil {
		t.Fatalf("query order: %v", err)
	}
	if status := order["status"].String(); status != "fulfilled" {
		t.Fatalf("expected order status=fulfilled after manual complete, got %q", status)
	}
	if payNo := order["payment_no"].String(); !strings.HasPrefix(payNo, "ADMIN_") {
		t.Fatalf("expected payment_no prefixed ADMIN_ for manual complete, got %q", payNo)
	}

	// 入账流水与履约同事务落库，可同步断言：按订单原价 1234.56 入账（实付 1184.56，折扣让利通过差额体现）
	creditTx, err := g.DB().Model("bil_transactions").Ctx(ctx).
		Where("related_id", created.OrderID).
		Where("related_type", "order").
		Where("amount > 0").One()
	if err != nil {
		t.Fatalf("query credit transaction: %v", err)
	}
	if creditTx.IsEmpty() {
		t.Fatal("expected a positive credit transaction for the order")
	}
	admintest.AssertFloatEqual(t, 1234.56, creditTx["amount"].Float64(), 0.0001, "credit tx amount (original price)")

	// 钱包 API 读 DB 物化副本（Redis 权威值由物化器每 5s 覆盖），轮询等待物化后断言余额增量
	balanceCredited := false
	for i := 0; i < 20; i++ {
		afterResp := client.Get("/api/tenant/wallet", nil)
		afterResp.AssertSuccess(t)
		var walletAfter struct {
			Balance float64 `json:"balance"`
		}
		afterResp.DecodeData(t, &walletAfter)
		if walletAfter.Balance-balanceBefore >= 1234.55 {
			admintest.AssertFloatEqual(t, 1234.56, walletAfter.Balance-balanceBefore, 0.0001, "wallet credited by original amount")
			balanceCredited = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !balanceCredited {
		t.Fatal("wallet balance not materialized within 10s after fulfillment")
	}

	// 优惠码用量保持已消耗：履约不释放（名额已被真实订单占用）
	usageCount, err := g.DB().Model("ord_promo_code_usages").Ctx(ctx).
		Where("promo_code_id", promoID).Count()
	if err != nil {
		t.Fatalf("query usages: %v", err)
	}
	if usageCount != 1 {
		t.Fatalf("expected 1 promo usage retained after fulfillment, got %d", usageCount)
	}
}

// epayTestSign 复刻 epay.go 内部 epaySign 算法（内部函数不可导出，测试自带镜像实现）：
// 非 sign/sign_type 的非空参数按 key 排序拼 k=v&，末尾接商户密钥取 MD5。
func epayTestSign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" || params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
	}
	buf.WriteString(key)
	hash := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

// TestRechargePromoLateCallbackNoDoubleCredit 手动履约后渠道补发真实成功回调不得二次入账：
// fulfilled 非 ProcessCallback 的可履约状态（claimable 仅 pending / expired+成功回调），
// 晚到回调幂等吞掉并返回 success（让渠道停止重推），仅打重复收款 ERROR 告警等人工渠道侧退款；
// 支付流水号保持 ADMIN_ 前缀、入账流水仍只有一笔。
func TestRechargePromoLateCallbackNoDoubleCredit(t *testing.T) {
	if os.Getenv("TEST_ADMIN_USERNAME") == "" || os.Getenv("TEST_ADMIN_PASSWORD") == "" {
		t.Skip("未配置 TEST_ADMIN_USERNAME/TEST_ADMIN_PASSWORD，跳过晚到回调 E2E")
	}

	client, _ := testinfra.GetAuthedClient(t)
	ctx := context.Background()

	// 读取 epay 渠道配置（构造合法签名，模拟渠道真实补发的回调）
	cfgJSON, err := g.DB().Model("sys_options").Ctx(ctx).
		Where("key", "payment_channel_epay").Value("value")
	if err != nil {
		t.Fatalf("query epay config: %v", err)
	}
	var epayCfg struct {
		MerchantID  string `json:"merchant_id"`
		MerchantKey string `json:"merchant_key"`
	}
	if err := json.Unmarshal([]byte(cfgJSON.String()), &epayCfg); err != nil || epayCfg.MerchantKey == "" {
		t.Skip("epay 渠道未配置商户密钥，跳过晚到回调 E2E")
	}

	// 建单：fixed 50 券 + 非整数金额 1234.56（避开档位折扣），不支付
	code := fmt.Sprintf("LC-%s", testinfra.RandomSuffix())
	promoID := insertPromoCode(t, promoCodeFixture{code: code, typ: "fixed", discountValue: 50})
	defer admintest.HardDeletePromoCode(t, promoID)

	createResp := client.Post("/api/tenant/recharge/create", map[string]any{
		"amount":          1234.56,
		"payment_channel": "epay",
		"payment_method":  "alipay",
		"promo_code":      code,
	})
	createResp.AssertSuccess(t)
	var created struct {
		OrderID int64  `json:"order_id"`
		OrderNo string `json:"order_no"`
	}
	createResp.DecodeData(t, &created)

	countCreditTx := func() int {
		n, err := g.DB().Model("bil_transactions").Ctx(ctx).
			Where("related_id", created.OrderID).
			Where("related_type", "order").
			Where("amount > 0").Count()
		if err != nil {
			t.Fatalf("count credit tx: %v", err)
		}
		return n
	}

	// 管理后台手动完成：单次入账
	adminClient := admintest.GetAuthedClient(t)
	completeResp := adminClient.Post(fmt.Sprintf("/api/admin/orders/%d/complete", created.OrderID), nil)
	completeResp.AssertSuccess(t)
	if n := countCreditTx(); n != 1 {
		t.Fatalf("expected exactly 1 credit tx after manual complete, got %d", n)
	}

	// 模拟渠道补发成功回调（合法签名、真实流水号、金额=折后实付）
	params := map[string]string{
		"pid":          epayCfg.MerchantID,
		"type":         "alipay",
		"out_trade_no": created.OrderNo,
		"trade_no":     "LATE-TRADE-001",
		"name":         "会员服务",
		"money":        "1184.56",
		"trade_status": "TRADE_SUCCESS",
	}
	sign := epayTestSign(params, epayCfg.MerchantKey)
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("sign", sign)
	q.Set("sign_type", "MD5")
	cbResp, err := http.Get(testinfra.DefaultBaseURL + "/api/payment/callback/epay?" + q.Encode())
	if err != nil {
		t.Fatalf("send late callback: %v", err)
	}
	defer cbResp.Body.Close()
	body, _ := io.ReadAll(cbResp.Body)
	if string(body) != "success" {
		t.Fatalf("expected callback response 'success' (idempotent), got %q (http %d)", string(body), cbResp.StatusCode)
	}

	// 订单仍 fulfilled、流水号仍为管理员标记（不被晚到真实流水号覆盖）
	order, err := g.DB().Model("ord_orders").Ctx(ctx).Where("id", created.OrderID).One()
	if err != nil {
		t.Fatalf("query order: %v", err)
	}
	if status := order["status"].String(); status != "fulfilled" {
		t.Fatalf("expected order status=fulfilled after late callback, got %q", status)
	}
	if payNo := order["payment_no"].String(); !strings.HasPrefix(payNo, "ADMIN_") {
		t.Fatalf("expected payment_no still ADMIN_-prefixed after late callback, got %q", payNo)
	}

	// 核心断言：未二次入账
	if n := countCreditTx(); n != 1 {
		t.Fatalf("DOUBLE CREDIT detected: expected 1 credit tx after late callback, got %d", n)
	}
}
