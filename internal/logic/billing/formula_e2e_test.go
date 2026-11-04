//go:build integration

// 公式计费端到端验证（进程内直调 logic，连真实 DB/Redis）：
// 保存 formula 定价 → 定价缓存读取（Formula 展开）→ 预扣估算 → computeCost 结算金额 →
// 非法公式保存拒绝 → 清理。覆盖除 HTTP 路由外的完整业务链路（路由与 RBAC 由 401 探测另行验证）。
package billing_test

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	_ "github.com/qianfree/team-api/internal/logic"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/service"
	rcommon "github.com/qianfree/team-api/relay/common"
)

func TestFormulaPricingE2E(t *testing.T) {
	ctx := context.Background()
	modelName := fmt.Sprintf("test-formula-e2e-%d", time.Now().UnixNano()/1e6)

	// 1. 建临时模型（status=active，测后删除）
	result, err := dao.MdlModels.Ctx(ctx).Data(g.Map{
		"model_id":           modelName,
		"model_name":         "公式计费E2E测试",
		"category":           "chat",
		"status":             "active",
		"max_context_tokens": 128000,
	}).Insert()
	if err != nil {
		t.Fatalf("建模型失败: %v", err)
	}
	modelDBID, _ := result.LastInsertId()
	defer func() {
		_, _ = dao.MdlModels.Ctx(ctx).Where("id", modelDBID).Delete()
	}()

	// 2. 保存 formula 定价（走管理端 service 全链路：BuildPricingBlob 烟测 + upsert + 缓存清理）
	const formula = "ctx <= 128000 ? p * 5 + cr * 1.25 + c * 20 : p * 10 + c * 40"
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode: "formula",
			Formula:     formula,
		}},
	})
	if err != nil {
		t.Fatalf("保存 formula 定价失败: %v", err)
	}
	defer func() {
		// 清理定价行（恢复未配价状态，模型行由外层 defer 删除）
		_, _ = dao.MdlPricing.Ctx(ctx).Where("model_id", modelDBID).Delete()
	}()

	// 3. 定价读取：Formula 展开进 PricingResult
	const tenantID int64 = 1
	pricing, err := billing.GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		t.Fatalf("读取定价失败: %v", err)
	}
	if pricing.BillingMode != billing.BillingModeFormula || pricing.Formula != formula {
		t.Fatalf("定价读取异常: mode=%s formula=%q", pricing.BillingMode, pricing.Formula)
	}

	// 4. 预扣估算：p=10000、估算输出 = max_output(0→4096 兜底)×0.8 = 3276 →
	//    公式走低档分支（ctx=10000 ≤ 128000），金额 = 系数/1e6 × 租户乘数（真实库租户可能带等级折扣）
	preDeduct, err := billing.EstimatePreDeductAmount(ctx, tenantID, modelName, 1, 10000, 0, false, nil, nil)
	if err != nil {
		t.Fatalf("预扣估算失败: %v", err)
	}
	mul := pricing.TenantMultiplier * pricing.TimeMultiplier
	if pricing.TimeMultiplier <= 0 {
		mul = pricing.TenantMultiplier
	}
	expectedPreDeduct := math.Ceil((10000*5+3276*20)/1e6*mul*1e6) / 1e6
	if preDeduct <= 0 || math.Abs(preDeduct-expectedPreDeduct) > 1e-9 {
		t.Fatalf("预扣金额异常: %v（期望 %v，租户乘数 %v）", preDeduct, expectedPreDeduct, pricing.TenantMultiplier)
	}

	// 5. 结算金额：OpenAI 口径 usage（prompt 含缓存读）→ 排除语义生效
	usage := &rcommon.Usage{
		PromptTokens:     25000, // 文本 17000 + 缓存读 8000
		CompletionTokens: 6000,
		PromptTokensDetails: &rcommon.TokenDetails{
			CachedTokens: 8000,
		},
		CacheIncludedInPrompt: true,
	}
	bd, err := billing.CalculateCostWithUsage(ctx, tenantID, modelName, usage)
	if err != nil {
		t.Fatalf("结算计算失败: %v", err)
	}
	// p = 25000-8000 = 17000（ctx=25000 ≤ 128000 低档）
	// 系数 = 17000*5 + 8000*1.25 + 6000*20 = 85000+10000+120000 = 215000 → 金额 = 0.215 × 租户乘数
	expectedSettle := 0.215 * mul
	if math.Abs(bd.TotalCost-expectedSettle) > 1e-9 {
		t.Fatalf("结算金额异常: %v（期望 %v = 0.215×乘数%v）", bd.TotalCost, expectedSettle, mul)
	}
	if bd.FormulaFactor != 215000 {
		t.Fatalf("系数异常: %v", bd.FormulaFactor)
	}
	if bd.InputTokens != 17000 || bd.CacheReadTokens != 8000 {
		t.Fatalf("token 口径异常: in=%d cr=%d", bd.InputTokens, bd.CacheReadTokens)
	}

	// 6. 非法公式保存拒绝（烟测兜底）
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode: "formula",
			Formula:     "unknown_var * 5",
		}},
	})
	if err == nil {
		t.Fatal("非法公式应被保存链路拒绝")
	}

	// 7. u() 旁路计量链路：保存按秒计费公式（STT 场景）→ 结算按真实秒数计费。
	//    预扣阶段无 facts（u 取 0 → 0.001 钳制），结算按 usage.Facts 补差
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode: "formula",
			// 每秒 0.4 本位币：金额 = u("audio_seconds") * 0.4
			Formula: `u("audio_seconds") * 400000`,
		}},
	})
	if err != nil {
		t.Fatalf("保存 u() 公式定价失败: %v", err)
	}
	bd2, err := billing.CalculateCostWithUsage(ctx, tenantID, modelName, &rcommon.Usage{
		PromptTokens: 150, CompletionTokens: 0, // token 粗估值不参与该公式计费
		Facts: map[string]float64{"audio_seconds": 90},
	})
	if err != nil {
		t.Fatalf("u() 公式结算失败: %v", err)
	}
	// 系数 = 90 × 400000 = 36,000,000 → 金额 = 36 × 租户乘数
	if math.Abs(bd2.TotalCost-36*mul) > 1e-9 {
		t.Fatalf("u() 公式结算金额异常: %v（期望 %v = 36×乘数）", bd2.TotalCost, 36*mul)
	}
	// 预扣：无 facts → u 取 0 → 0.001 钳制
	preDeduct2, err := billing.EstimatePreDeductAmount(ctx, tenantID, modelName, 1, 150, 0, false, nil, nil)
	if err != nil {
		t.Fatalf("u() 公式预扣失败: %v", err)
	}
	if preDeduct2 != 0.001 {
		t.Fatalf("纯 facts 公式预扣应为 0.001 钳制: %v", preDeduct2)
	}

	// 8. 未注册 u() key 保存拒绝（防拼写错误静默取 0 错价）
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode: "formula",
			Formula:     `u("audio_second") * 400000`, // 拼写错误（少 s）
		}},
	})
	if err == nil {
		t.Fatal("未注册 u() key 应被保存烟测拒绝")
	}

	// 9. 任务路径公式（P2）：保存视频按秒公式 → 任务预扣估算走 DB 定价（spec.duration 信号）
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode: "formula",
			Formula:     `fixed(0.4) * seconds`, // 视频每秒 0.4 本位币
		}},
	})
	if err != nil {
		t.Fatalf("保存任务公式定价失败: %v", err)
	}
	ratios := map[string]any{"spec.duration": 5.0, "spec.resolution": "720p"}
	taskProvider := billing.NewTaskBillingProvider()
	preTaskD, err := taskProvider.EstimateTaskCost(ctx, tenantID, modelName, ratios, nil, nil)
	if err != nil {
		t.Fatalf("任务公式预扣失败: %v", err)
	}
	// 预扣 = fixed(0.4)×400000×5/1e6 × 租户乘数 = 2×mul（wrapSchemeCost 的 minCost 0.01 钳制不影响）
	if math.Abs(preTaskD.InexactFloat64()-2*mul) > 1e-9 {
		t.Fatalf("任务公式预扣异常: %v（期望 %v = 2×乘数）", preTaskD, 2*mul)
	}

	// 10. 请求规则链路：基础公式 + formula_rules 分离存储 → 取价组合展开 →
	//     预扣按请求头/请求体探测放大 → 未命中走默认 1 → 任务路径探测值冻结进 ratios
	const reqRules = `(header("tier") == "pro" ? 2 : 1) * (param("service_tier") == "priority" ? 3 : 1)`
	_, err = service.Admin().SetModelPricing(ctx, &v1.PricingSetReq{
		ModelID: modelDBID,
		Items: []v1.PricingItem{{
			BillingMode:  "formula",
			Formula:      "p * 5 + c * 10",
			FormulaRules: reqRules,
		}},
	})
	if err != nil {
		t.Fatalf("保存带请求规则的 formula 定价失败: %v", err)
	}
	pricing2, err := billing.GetModelPrice(ctx, tenantID, modelName)
	if err != nil {
		t.Fatalf("读取带规则定价失败: %v", err)
	}
	combined := billing.EffectiveFormula("p * 5 + c * 10", reqRules)
	if pricing2.Formula != combined || pricing2.FormulaRules != reqRules || pricing2.FormulaBase != "p * 5 + c * 10" {
		t.Fatalf("组合式展开异常: formula=%q base=%q rules=%q", pricing2.Formula, pricing2.FormulaBase, pricing2.FormulaRules)
	}

	// 命中路径：header tier=pro ×2 + service_tier=priority ×3 → 预扣 = 基础 × 6
	preHit, err := billing.EstimatePreDeductAmount(ctx, tenantID, modelName, 1, 10000, 0, false,
		http.Header{"Tier": []string{"pro"}}, []byte(`{"service_tier":"priority"}`))
	if err != nil {
		t.Fatalf("带请求预扣失败: %v", err)
	}
	basePre, err := billing.EstimatePreDeductAmount(ctx, tenantID, modelName, 1, 10000, 0, false, nil, nil)
	if err != nil {
		t.Fatalf("无请求预扣失败: %v", err)
	}
	if math.Abs(preHit-basePre*6) > 1e-6 {
		t.Fatalf("请求规则预扣异常: hit=%v base=%v（期望 ×6）", preHit, basePre)
	}

	// 任务路径：探测值随 ratios 冻结（billing_context 持久化口径）
	taskRatios := map[string]any{"spec.duration": 5.0}
	if _, err := taskProvider.EstimateTaskCost(ctx, tenantID, modelName, taskRatios, nil,
		http.Header{"Tier": []string{"pro"}}); err != nil {
		t.Fatalf("任务带 header 估算失败: %v", err)
	}
	frozen, ok := taskRatios["formula_request"].(map[string]any)
	if !ok {
		t.Fatalf("formula_request 未冻结进 ratios: %v", taskRatios)
	}
	// 内存形态 headers 为 map[string]string；JSONB 往返后的还原由单测覆盖
	if frozen["headers"].(map[string]string)["tier"] != "pro" {
		t.Fatalf("冻结的 header 探测值异常: %v", frozen)
	}
	if _, ok := frozen["at"]; !ok {
		t.Fatalf("冻结的时间基准缺失: %v", frozen)
	}
}
