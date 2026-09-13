//go:build e2e

// 手动 E2E（需真实 DB/Redis；默认 go test 不编译本文件）。
// 跑法：go test -tags e2e -run TestE2EParamMultiplier ./internal/logic/billing/
// 使用独立的临时模型行 + 不存在的租户 ID，走纯基础定价路径，结束即清理，不影响现有数据。
// 验证链路：pricing JSONB 配置 param_multipliers → TaskBillingProvider.EstimateTaskCost
// 携归一化任务体求值 → ratios 注入 param_multiplier/param_matched → 费用连乘。

package billing

import (
	"fmt"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

func TestE2EParamMultiplier(t *testing.T) {
	// 测试 CWD 是包目录，把配置搜索路径指回仓库根的 manifest/config
	adapterFile, err := gcfg.NewAdapterFile()
	if err != nil {
		t.Fatalf("new config adapter: %v", err)
	}
	if err := adapterFile.AddPath("../../../manifest/config"); err != nil {
		t.Fatalf("add config path: %v", err)
	}
	gcfg.Instance().SetAdapter(adapterFile)
	ctx := gctx.New()

	const tmpModel = "tmp-e2e-param-multiplier"
	const tmpTenant = int64(999999998) // 不存在的租户：走纯基础定价路径

	cleanup := func() {
		var id int64
		v, _ := dao.MdlModels.Ctx(ctx).Where("model_id", tmpModel).Fields("id").Value()
		fmt.Sscanf(v.String(), "%d", &id)
		if id > 0 {
			_, _ = dao.MdlPricing.Ctx(ctx).Where("model_id", id).Delete()
			_, _ = dao.MdlModels.Ctx(ctx).Where("id", id).Delete()
		}
		ClearModelPriceCache(ctx, tmpModel)
	}
	cleanup()
	defer cleanup()

	// 临时模型 + per_second 定价（含参数倍率规则：图生视频 ×1.5，≥10 秒 ×2.0，视频输入 ×0.609）
	modelID, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(do.MdlModels{
		ModelId:   tmpModel,
		ModelName: "E2E参数倍率临时模型",
		Category:  "video",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("insert tmp model: %v", err)
	}
	pricingJSON := `{
		"unit": "second",
		"prices": {"720p": 0.5, "1080p": 1.0, "*": 0.5},
		"param_multipliers": [
			{"conditions": [{"path": "metadata.image", "match": "has_value"}], "multiplier": 1.5, "note": "图生视频"},
			{"conditions": [{"path": "seconds", "match": "gte", "value": 10}], "multiplier": 2.0, "note": "长视频"},
			{"conditions": [{"path": "metadata.content[*].video_url", "match": "has_value"}], "multiplier": 0.609, "note": "视频输入折扣"}
		]
	}`
	if _, err := dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:     modelID,
		BillingMode: "per_second",
		Pricing:     pricingJSON,
	}); err != nil {
		t.Fatalf("insert tmp pricing: %v", err)
	}

	provider := NewTaskBillingProvider()

	// 1. 命中「图生视频 ×1.5」+「长视频 ×2.0」（seconds 字符串 "12" 数字强转参与比较）：
	//    费用 = 0.5(720p) × 12s × 1.5 × 2.0 = 18.0；ratios 注入 param_multiplier=3 / param_matched
	body := []byte(`{"model":"` + tmpModel + `","prompt":"p","seconds":"12","metadata":{"image":"https://x","duration":12,"resolution":"720p"}}`)
	ratios := map[string]any{"spec.duration": 12.0, "spec.resolution": "720p"}
	cost, err := provider.EstimateTaskCost(ctx, tmpTenant, tmpModel, ratios, body)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	assertDecimal(t, cost, 18.0, "e2e param multiplier cost (0.5×12×1.5×2)")
	if m, _ := ratios["param_multiplier"].(float64); m != 3.0 {
		t.Fatalf("param_multiplier injected = %v (type %T), want 3.0", ratios["param_multiplier"], ratios["param_multiplier"])
	}
	if matched, _ := ratios["param_matched"].(string); matched != "图生视频|长视频" {
		t.Fatalf("param_matched = %q, want 图生视频|长视频", matched)
	}

	// 2. 纯文生视频（无图、5 秒）：不命中任何规则，无注入；费用 = 0.5 × 5 = 2.5
	ratios2 := map[string]any{"spec.duration": 5.0}
	body2 := []byte(`{"model":"` + tmpModel + `","prompt":"p","seconds":"5"}`)
	cost2, err := provider.EstimateTaskCost(ctx, tmpTenant, tmpModel, ratios2, body2)
	if err != nil {
		t.Fatalf("estimate2: %v", err)
	}
	assertDecimal(t, cost2, 2.5, "e2e no-match cost")
	if _, injected := ratios2["param_multiplier"]; injected {
		t.Fatalf("no-match should not inject, got %v", ratios2["param_multiplier"])
	}

	// 3. 视频输入折扣（[*] 路径）+ 1080p：费用 = 1.0 × 8 × 0.609 = 4.872
	ratios3 := map[string]any{"spec.duration": 8.0, "spec.resolution": "1080p"}
	body3 := []byte(`{"model":"` + tmpModel + `","prompt":"p","metadata":{"resolution":"1080p","duration":8,"content":[{"type":"video_url","video_url":{"url":"v"}}]}}`)
	cost3, err := provider.EstimateTaskCost(ctx, tmpTenant, tmpModel, ratios3, body3)
	if err != nil {
		t.Fatalf("estimate3: %v", err)
	}
	assertDecimal(t, cost3, 4.872, "e2e video-input discount cost (1.0×8×0.609)")

	// 4. 定价缓存命中路径同样生效（第二次调用走 600s 缓存里的 ParamMultipliers）
	ratios4 := map[string]any{"spec.duration": 12.0, "spec.resolution": "720p"}
	cost4, _ := provider.EstimateTaskCost(ctx, tmpTenant, tmpModel, ratios4, body)
	assertDecimal(t, cost4, 18.0, "e2e cached param multiplier cost")
}
