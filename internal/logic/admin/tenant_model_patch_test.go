package admin

import (
	"testing"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// pptrOf 双层指针快捷构造（模拟 API 请求里「键存在」的传参形态）
func pptrOf[T any](v *T) **T { return &v }

// TestBuildTenantPricingPatchUpdate 补丁读改写合并：四态语义（省略=不动 / null=清除恢复继承 /
// 空数组=显式关闭 / 非空=整体替换）与旧补丁部分更新
// TestDisplayColValue 展示字段列值口径：trim 后空串=清除（NULL Raw），非空=覆盖原值
func TestDisplayColValue(t *testing.T) {
	if v, ok := displayColValue("  夜间特惠  ").(string); !ok || v != "夜间特惠" {
		t.Errorf("应 trim 后返回字符串, got %v", displayColValue("  夜间特惠  "))
	}
	// 空串/纯空白返回 gdb.Raw（NULL），类型不同于 string——两种输入产生不同类型即可验证口径分叉
	_, emptyIsStr := displayColValue("   ").(string)
	_, nonEmptyIsStr := displayColValue("x").(string)
	if emptyIsStr || !nonEmptyIsStr {
		t.Errorf("空串应返回非 string 的 NULL 哨兵，非空应返回 string")
	}
}

// TestNullKeysFromBody 「null=清除」键提取：JSON null 键保留存在性、省略键与
// 非 null 值不命中、嵌套对象内的 null 不影响顶层判断、非 JSON body 返回 nil。
// 该函数是 **指针三态契约（省略=不动 / null=清除 / 值=设置）的还原基础——
// gconv 把 null 与省略都折叠为 nil 指针，语义只能在此还原
func TestNullKeysFromBody(t *testing.T) {
	keys := nullKeysFromBody([]byte(`{"discount_ratio":null,"price_note":null,"enabled":true,"max_concurrency":5,"pricing":{"time_segments":null}}`))
	if !keys["discount_ratio"] || !keys["price_note"] {
		t.Errorf("显式 null 键应命中: %v", keys)
	}
	if keys["enabled"] || keys["max_concurrency"] || keys["pricing"] {
		t.Errorf("非 null 值键不应命中: %v", keys)
	}
	// 省略键：不存在于 map，天然不命中
	if keys["discount_label"] {
		t.Errorf("省略键不应命中")
	}
	// 空对象 / 非 JSON
	if k := nullKeysFromBody([]byte(`{}`)); len(k) != 0 {
		t.Errorf("空对象应返回空集合, got %v", k)
	}
	if k := nullKeysFromBody([]byte(`not-json`)); k != nil {
		t.Errorf("非 JSON 应返回 nil, got %v", k)
	}
	// 带 null 的 JSON 数组（非对象）也应安全返回 nil
	if k := nullKeysFromBody([]byte(`[null,1]`)); k != nil {
		t.Errorf("数组 body 应返回 nil, got %v", k)
	}
}

// TestPricingOverrideRoundTrip API 覆盖对象 ↔ PricingBlob 往返转换：
// 价格键保真、阶梯缓存价 0↔nil 归一、时段/参数三态（继承/关闭/替换）保真
func TestPricingOverrideRoundTrip(t *testing.T) {
	input := &v1.TenantPricingOverride{
		BillingMode:          strPtrOf("tiered"),
		InputPrice:           floatPtrOf(1.5),
		CacheReadPrice:       floatPtrOf(0.1),
		CacheCreation1hPrice: floatPtrOf(0.4),
		Price:                floatPtrOf(0.02),
		Tiers: []*v1.PricingTier{
			{MinTokens: 0, MaxTokens: nil, InputPrice: 5, OutputPrice: 15, CacheCreationPrice: 0.3},
		},
	}
	segs := []v1.TimeSegmentItem{{Name: "夜间", Multiplier: 0.8}}
	input.TimeSegments = &segs

	blob, err := pricingOverrideToBlob(input)
	if err != nil {
		t.Fatalf("pricingOverrideToBlob: %v", err)
	}
	if blob.BillingMode == nil || *blob.BillingMode != "tiered" || blob.InputPrice == nil || *blob.InputPrice != 1.5 {
		t.Fatalf("价格/模式键转换失真: %+v", blob)
	}
	if len(blob.Tiers) != 1 || blob.Tiers[0].CacheCreationPrice == nil || *blob.Tiers[0].CacheCreationPrice != 0.3 {
		t.Fatalf("阶梯转换失真: %+v", blob.Tiers)
	}
	if blob.Tiers[0].CacheReadPrice != nil {
		t.Fatalf("阶梯零值缓存价应归一为 nil（未配置回退锚点）")
	}

	// 回显往返：值语义等价（时段数组三态保真）
	echo := blobToPricingOverride(blob)
	if echo.InputPrice == nil || *echo.InputPrice != 1.5 || echo.TimeSegments == nil || len(*echo.TimeSegments) != 1 {
		t.Fatalf("回显失真: %+v", echo)
	}
	if len(echo.Tiers) != 1 || echo.Tiers[0].CacheReadPrice != 0 || echo.Tiers[0].CacheCreationPrice != 0.3 {
		t.Fatalf("阶梯回显失真: %+v", echo.Tiers)
	}

	// 显式关闭三态：空时段数组 → blob 非 nil 空切片 → 回显仍为空数组
	off := &v1.TenantPricingOverride{}
	emptySegs := []v1.TimeSegmentItem{}
	off.TimeSegments = &emptySegs
	blobOff, err := pricingOverrideToBlob(off)
	if err != nil {
		t.Fatalf("pricingOverrideToBlob(off): %v", err)
	}
	if blobOff.TimeSegments == nil || len(*blobOff.TimeSegments) != 0 {
		t.Fatalf("空数组应保住「显式关闭」三态: %+v", blobOff.TimeSegments)
	}
	if blobOff.HasAnyOverride() != true {
		t.Fatalf("显式关闭算覆盖键（整源判定）")
	}
}

func floatPtrOf(v float64) *float64 { return &v }

func strPtrOf(s string) *string { return &s }
