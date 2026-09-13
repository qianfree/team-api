//go:build e2e

// 定价 JSON 化完整性校验（需真实 DB）。默认 go test 不编译本文件（与 e2e_time_segment_test 同策略：
// 无条件引入 pgsql 驱动会让默认单测连上真实库，billing_currency 等部署配置泄漏进单元测试）。
// 跑法：go test -tags e2e -run TestPricingJSONComplete ./internal/logic/billing/

package billing

import (
	"context"
	"encoding/json"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"
)

// TestPricingJSONComplete contract 迁移（000021）后完整性校验：
// 每模型一行、pricing JSONB 非空且可解析、tiered 模式含阶梯数组。
func TestPricingJSONComplete(t *testing.T) {
	ctx := context.Background()

	// 1. 每模型恰好一行（uk 约束之外的双重确认）
	dupVal, err := g.DB().GetValue(ctx,
		`SELECT COUNT(*) FROM (SELECT model_id FROM mdl_pricing GROUP BY model_id HAVING count(*) > 1) d`)
	if err != nil {
		t.Fatalf("query dup rows: %v", err)
	}
	if n := dupVal.Int(); n > 0 {
		t.Errorf("%d 个模型存在多行定价（contract 未收敛）", n)
	}

	// 2. pricing JSONB 全部非空
	nullVal, err := g.DB().GetValue(ctx,
		`SELECT COUNT(*) FROM mdl_pricing WHERE pricing IS NULL`)
	if err != nil {
		t.Fatalf("query null pricing: %v", err)
	}
	if n := nullVal.Int(); n > 0 {
		t.Errorf("%d 行 pricing JSON 为空（唯一真相缺失，计费将按未配价 fail-closed）", n)
	}

	// 3. JSON 可解析 + 模式字段完整性（tiered 必含 tiers 数组、per_second 必含矩阵）
	rows, err := g.DB().GetAll(ctx,
		`SELECT model_id, billing_mode, pricing FROM mdl_pricing WHERE pricing IS NOT NULL`)
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	bad := 0
	for _, row := range rows {
		blob := &PricingBlob{}
		if err := json.Unmarshal([]byte(row["pricing"].String()), blob); err != nil {
			t.Errorf("model_id=%v pricing JSON 解析失败: %v", row["model_id"], err)
			bad++
			continue
		}
		switch row["billing_mode"].String() {
		case "tiered":
			if len(blob.Tiers) == 0 {
				t.Errorf("model_id=%v tiered 模式 tiers 为空", row["model_id"])
				bad++
			}
		case "per_second", BillingModeSpecial:
			if len(blob.Prices) == 0 {
				t.Errorf("model_id=%v %s 模式 prices 矩阵为空", row["model_id"], row["billing_mode"].String())
				bad++
			}
			// special 必须与方案配对（pricing JSONB 顶层 scheme 键），裸 special 会在
			// 计费入口 fail-closed 拒绝请求
			if row["billing_mode"].String() == BillingModeSpecial && blob.Scheme == "" {
				t.Errorf("model_id=%v special 模式缺少 scheme 声明", row["model_id"])
				bad++
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d 行 pricing JSON 内容异常", bad)
	}
}
