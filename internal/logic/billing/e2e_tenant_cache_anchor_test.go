//go:build e2e

// 手动 E2E（需真实 DB/Redis；默认 go test 不编译本文件）。
// 跑法：go test -tags e2e -run TestE2ETenantCacheAnchorTiered ./internal/logic/billing/
// 场景：平台 tiered 模型（档内带缓存价）+ 租户仅覆盖锚点级缓存价（无自有阶梯）——
// 验证租户覆盖价生效（平台档内缓存价被剥离回退锚点），而非被平台档内价盖掉。

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

func TestE2ETenantCacheAnchorTiered(t *testing.T) {
	adapterFile, err := gcfg.NewAdapterFile()
	if err != nil {
		t.Fatalf("new config adapter: %v", err)
	}
	if err := adapterFile.AddPath("../../../manifest/config"); err != nil {
		t.Fatalf("add config path: %v", err)
	}
	gcfg.Instance().SetAdapter(adapterFile)
	ctx := gctx.New()

	const tmpModel = "tmp-e2e-cache-anchor"
	const tmpTenant = int64(999999998)

	cleanup := func() {
		var id int64
		v, _ := dao.MdlModels.Ctx(ctx).Where("model_id", tmpModel).Fields("id").Value()
		fmt.Sscanf(v.String(), "%d", &id)
		if id > 0 {
			_, _ = dao.MdlTenantModels.Ctx(ctx).Where("model_id", id).Where("tenant_id", tmpTenant).Delete()
			_, _ = dao.MdlPricing.Ctx(ctx).Where("model_id", id).Delete()
			_, _ = dao.MdlModels.Ctx(ctx).Where("id", id).Delete()
		}
		ClearModelPriceCache(ctx, tmpModel)
	}
	cleanup()
	defer cleanup()

	modelID, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(do.MdlModels{
		ModelId:   tmpModel,
		ModelName: "E2E缓存锚点临时模型",
		Category:  "chat",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("insert tmp model: %v", err)
	}
	// 平台阶梯：两档均带档内缓存价（读 0.1 / 5m 3.75），1h 未配按锚点推导
	tiersJSON := `[
		{"min_tokens":0,"max_tokens":100000,"input_price":5,"output_price":15,"cache_read_price":0.1,"cache_creation_price":3.75},
		{"min_tokens":100000,"max_tokens":null,"input_price":3,"output_price":10,"cache_read_price":0.1,"cache_creation_price":3.75}
	]`
	if _, err := dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:     modelID,
		BillingMode: "tiered",
		Pricing:     fmt.Sprintf(`{"cache_read_price":0.1,"cache_creation_price":3.75,"tiers":%s}`, tiersJSON),
	}); err != nil {
		t.Fatalf("insert tmp pricing: %v", err)
	}

	// 租户行：仅锚点级缓存价覆盖（无 billing_mode、无自有阶梯）——存量数据常见形态
	if _, err := dao.MdlTenantModels.Ctx(ctx).Insert(do.MdlTenantModels{
		TenantId:      tmpTenant,
		ModelId:       modelID,
		Enabled:       true,
		Multiplier:    1.0,
		CustomPricing: `{"input_price":5,"cache_read_price":0.02,"cache_creation_price":1.5}`,
	}); err != nil {
		t.Fatalf("insert tmp tenant model: %v", err)
	}

	p, err := GetModelPrice(ctx, tmpTenant, tmpModel)
	if err != nil {
		t.Fatalf("GetModelPrice: %v", err)
	}
	if p.BillingMode != "tiered" {
		t.Fatalf("BillingMode = %q, want tiered", p.BillingMode)
	}

	// 兜底平台阶梯的档内缓存价必须已剥离（回退租户锚点）
	for i, tier := range p.CustomTiers {
		if tier.CacheReadPrice != nil || tier.CacheCreationPrice != nil || tier.CacheCreation1hPrice != nil {
			t.Fatalf("tier[%d] 档内缓存价未剥离: %+v", i, tier)
		}
	}

	// 命中第二档（200K 输入）：缓存三元组 = 租户锚点（读 0.02 / 5m 1.5 / 1h 1.5×1.6）
	read, c5m, c1h := effectiveCachePrices(p, 200_000)
	assertFloat(t, read, 0.02, "e2e tenant cache read (anchored)")
	assertFloat(t, c5m, 1.5, "e2e tenant cache creation 5m (anchored)")
	assertFloat(t, c1h, 2.4, "e2e tenant cache creation 1h (1.5×1.6 derived)")

	// 输入/输出仍按平台档位价（未覆盖键继承）
	assertFloat(t, p.CustomTiers[1].InputPrice, 3.0, "e2e tier2 input price (platform)")
}
