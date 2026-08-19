//go:build e2e

// 手动 E2E（需真实 DB/Redis；默认 go test 不编译本文件）。
// 跑法：go test -tags e2e -run TestE2ETimeSegmentPricing ./internal/logic/billing/
// 使用独立的临时模型行 + 不存在的租户 ID，走纯基础定价路径，结束即清理，不影响现有数据。

package billing

import (
	"fmt"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

func TestE2ETimeSegmentPricing(t *testing.T) {
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

	const tmpModel = "tmp-e2e-time-segment"
	const tmpTenant = int64(999999999) // 不存在的租户：走纯基础定价路径

	// 清理（含上次失败运行的残留）
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

	// 临时模型 + 定价（时段窗口覆盖 now±1h，乘数 0.5）
	modelID, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(do.MdlModels{
		ModelId:   tmpModel,
		ModelName: "E2E时段定价临时模型",
		Category:  "chat",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("insert tmp model: %v", err)
	}
	now := time.Now()
	start := now.Add(-time.Hour).Format("15:04")
	end := now.Add(time.Hour).Format("15:04")
	segJSON := fmt.Sprintf(`[{"name":"闲时","start_time":%q,"end_time":%q,"multiplier":0.5}]`, start, end)
	if _, err := dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:      modelID,
		BillingMode:  "token",
		MinTokens:    0,
		InputPrice:   NewFromFloat(3),
		OutputPrice:  NewFromFloat(15),
		TimeSegments: segJSON,
	}); err != nil {
		t.Fatalf("insert tmp pricing: %v", err)
	}

	// 1. 首次解析（DB 路径）：命中时段
	p1, err := GetModelPrice(ctx, tmpTenant, tmpModel)
	if err != nil {
		t.Fatalf("GetModelPrice: %v", err)
	}
	assertFloat(t, p1.TimeMultiplier, 0.5, "e2e TimeMultiplier (db path)")
	if p1.TimeRuleName != "闲时" {
		t.Fatalf("TimeRuleName = %q, want 闲时", p1.TimeRuleName)
	}
	assertFloat(t, p1.InputPrice, 3.0, "e2e InputPrice")

	// 2. 费用计算：1M 输入 + 1M 输出 = 18 × 0.5 = 9
	bd, err := CalculateCost(ctx, tmpTenant, tmpModel, 1_000_000, 1_000_000)
	if err != nil {
		t.Fatalf("CalculateCost: %v", err)
	}
	assertFloat(t, bd.TotalCost, 9.0, "e2e TotalCost (18 × 0.5)")

	// 3. 缓存命中路径 + 窗口外 billAt：重评估为 1.0（受理时刻语义）
	p2, err := GetModelPriceAt(ctx, tmpTenant, tmpModel, now.Add(-3*time.Hour))
	if err != nil {
		t.Fatalf("GetModelPriceAt(out of window): %v", err)
	}
	assertFloat(t, p2.TimeMultiplier, 1.0, "e2e TimeMultiplier (cache hit, out-of-window billAt)")
	if p2.TimeRuleName != "" {
		t.Fatalf("out-of-window TimeRuleName = %q, want empty", p2.TimeRuleName)
	}

	// 4. 缓存命中路径 + 窗口内 billAt：重评估回 0.5
	p3, err := GetModelPriceAt(ctx, tmpTenant, tmpModel, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("GetModelPriceAt(in window): %v", err)
	}
	assertFloat(t, p3.TimeMultiplier, 0.5, "e2e TimeMultiplier (cache hit, in-window billAt)")

	t.Logf("E2E 通过：DB 解析/费用计算/缓存重评估均符合预期（窗口 %s~%s）", start, end)
}
