package admin

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/api/admin/v1"
)

// TestToExportPricing_TieredPerTierCachePrices tiered 逐档缓存价随导出行携带
// （回归：旧导出展开逻辑丢档内缓存价，导入后逐档配置丢失）
func TestToExportPricing_TieredPerTierCachePrices(t *testing.T) {
	upper := int64(128000)
	items := []v1.PricingItem{
		{BillingMode: "tiered", MinTokens: 0, MaxTokens: &upper, InputPrice: 1, OutputPrice: 2, CacheReadPrice: 0.1, CacheCreationPrice: 1.25},
		{BillingMode: "tiered", MinTokens: 128000, InputPrice: 0.5, OutputPrice: 1, CacheReadPrice: 0.05},
	}
	out := toExportPricing(items)
	if len(out) != 2 {
		t.Fatalf("应导出两档，got %d", len(out))
	}
	if out[0].CacheReadPrice != 0.1 || out[0].CacheCreationPrice != 1.25 {
		t.Fatalf("首档缓存价丢失: %+v", out[0])
	}
	if out[1].CacheReadPrice != 0.05 || out[1].CacheCreationPrice != 0 {
		t.Fatalf("次档缓存价不符: %+v", out[1])
	}
}

// TestImportItemOfficialNilSemantics 官方定价 nil/非 nil 的 JSON 区分：
// 缺字段或 null → nil（导入不动库内值，旧文件兼容）；显式空数组 → 非 nil（导入=清除）
func TestImportItemOfficialNilSemantics(t *testing.T) {
	// 旧导出文件：无 official_items 字段
	var old v1.ModelImportItem
	if err := json.Unmarshal([]byte(`{"model_id":"gpt-x","pricing":[]}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.OfficialItems != nil {
		t.Fatalf("缺字段应保持 nil，got %#v", old.OfficialItems)
	}

	// 模拟前端回传链路：nil 序列化为 null，再反序列化仍为 nil
	b, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip v1.ModelImportItem
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.OfficialItems != nil {
		t.Fatalf("null 回传后应仍为 nil，got %#v", roundTrip.OfficialItems)
	}

	// 源环境未配置官方定价的新导出文件：显式空数组 → 非 nil → 导入清除目标库值
	var empty v1.ModelImportItem
	if err := json.Unmarshal([]byte(`{"model_id":"gpt-x","official_items":[]}`), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.OfficialItems == nil {
		t.Fatal("显式空数组应为非 nil（导入=清除官方定价）")
	}
}

// TestOfficialPricingExportImportRoundTrip 官方定价导出→导入往返等价：
// 落库 JSONB → 导出展开（officialBlobFromJSON + expandPricingBlob）→
// 导入重建（buildOfficialBlob），重建结果应与落库 blob 等价
func TestOfficialPricingExportImportRoundTrip(t *testing.T) {
	items := []v1.PricingItem{{
		BillingMode:        "token",
		InputPrice:         3,
		OutputPrice:        15,
		CacheReadPrice:     0.3,
		CacheCreationPrice: 3.75,
	}}
	segments := []v1.TimeSegmentItem{{
		Name:       "夜间半价",
		StartTime:  "22:00",
		EndTime:    "06:00",
		Multiplier: 0.5,
	}}

	// 落库形态（同 writeOfficialPricingForModel 的构建与序列化路径）
	stored, err := buildOfficialBlob(items, segments, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("输入含正价，不应判为未配置")
	}
	raw, err := json.Marshal(officialPricingBlob{BillingMode: "token", PricingBlob: *stored})
	if err != nil {
		t.Fatal(err)
	}

	// 导出还原路径（ExportModelsJson 同一调用链）
	mode, blob := officialBlobFromJSON(string(raw))
	if mode != "token" || blob == nil {
		t.Fatalf("官方定价解析失败: mode=%q blob=%v", mode, blob)
	}
	expItems, expSegs, _ := expandPricingBlob(mode, blob)

	// 导入重建路径（writeOfficialPricingForModel 同一调用链）
	rebuilt, err := buildOfficialBlob(expItems, expSegs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt == nil {
		t.Fatal("往返后不应退化为未配置")
	}
	want, _ := json.Marshal(stored)
	got, _ := json.Marshal(rebuilt)
	if string(got) != string(want) {
		t.Fatalf("往返不等价:\nwant %s\ngot  %s", want, got)
	}
}
