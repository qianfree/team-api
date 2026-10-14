package tenant

import (
	"reflect"
	"testing"
)

func setOf(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func facetRowFor(category, vendor, capabilities, billing string) marketplaceFacetRow {
	return marketplaceFacetRow{
		Category:     category,
		Vendor:       vendor,
		Capabilities: capabilities,
		BillingMode:  billing,
	}
}

func TestAggregateMarketplaceFacetsBasicCounts(t *testing.T) {
	rows := []marketplaceFacetRow{
		facetRowFor("chat", "openai", `{"function_calling":true,"vision":true}`, "token"),
		facetRowFor("chat", "anthropic", `{"vision":true}`, "token"),
		facetRowFor("image", "", ``, "per_request"),
	}

	facets, vendors := aggregateMarketplaceFacets(rows, marketplaceFacetFilters{})

	if !reflect.DeepEqual(facets.Categories, map[string]int{"chat": 2, "image": 1}) {
		t.Errorf("Categories = %v, want chat:2 image:1", facets.Categories)
	}
	if !reflect.DeepEqual(facets.Vendors, map[string]int{"openai": 1, "anthropic": 1, vendorFacetUncategorized: 1}) {
		t.Errorf("Vendors = %v", facets.Vendors)
	}
	if !reflect.DeepEqual(facets.Capabilities, map[string]int{"function_calling": 1, "vision": 2}) {
		t.Errorf("Capabilities = %v", facets.Capabilities)
	}
	if !reflect.DeepEqual(facets.BillingModes, map[string]int{"token": 2, "per_request": 1}) {
		t.Errorf("BillingModes = %v", facets.BillingModes)
	}
	if !reflect.DeepEqual(vendors, []string{"anthropic", "openai", vendorFacetUncategorized}) {
		t.Errorf("vendors = %v", vendors)
	}
}

// 析取语义：每个维度的计数只排除自身维度的筛选，其余维度照常应用——
// 勾选厂商后，类别计数反映「该厂商下各类别还剩多少」，厂商自身计数仍是全集
func TestAggregateMarketplaceFacetsDisjunctive(t *testing.T) {
	rows := []marketplaceFacetRow{
		facetRowFor("chat", "openai", `{}`, "token"),
		facetRowFor("chat", "openai", `{}`, "token"),
		facetRowFor("image", "openai", `{}`, "per_request"),
		facetRowFor("chat", "anthropic", `{}`, "token"),
	}

	facets, _ := aggregateMarketplaceFacets(rows, marketplaceFacetFilters{
		Vendors: setOf("openai"),
	})

	// 类别维度：应用厂商筛选（厂商是其余维度）→ openai 名下 chat 2 / image 1
	if !reflect.DeepEqual(facets.Categories, map[string]int{"chat": 2, "image": 1}) {
		t.Errorf("Categories = %v, want chat:2 image:1（应用厂商筛选）", facets.Categories)
	}
	// 厂商维度：排除自身筛选 → openai 3 / anthropic 1（换选别的厂商可知剩余量）
	if !reflect.DeepEqual(facets.Vendors, map[string]int{"openai": 3, "anthropic": 1}) {
		t.Errorf("Vendors = %v", facets.Vendors)
	}
	// 计费维度：应用厂商筛选 → token 2（anthropic 聊天被排除）/ per_request 1
	if !reflect.DeepEqual(facets.BillingModes, map[string]int{"token": 2, "per_request": 1}) {
		t.Errorf("BillingModes = %v, want token:2 per_request:1", facets.BillingModes)
	}
}

// 能力筛选为 AND 语义：行需具备全部已选能力
func TestAggregateMarketplaceFacetsCapabilityAndSemantics(t *testing.T) {
	rows := []marketplaceFacetRow{
		facetRowFor("chat", "openai", `{"function_calling":true,"vision":true}`, "token"),
		facetRowFor("chat", "anthropic", `{"vision":true}`, "token"),
		facetRowFor("chat", "google", `{"function_calling":true}`, "token"),
	}

	facets, _ := aggregateMarketplaceFacets(rows, marketplaceFacetFilters{
		Capabilities: setOf("function_calling", "vision"),
	})

	// 厂商维度应用能力筛选：同时具备两项的只有 openai；零计数键不进 map（前端按缺失=0 处理）
	if !reflect.DeepEqual(facets.Vendors, map[string]int{"openai": 1}) {
		t.Errorf("Vendors = %v, want 仅 openai:1", facets.Vendors)
	}
	// 能力维度排除自身筛选：function_calling 2、vision 2（全集）
	if !reflect.DeepEqual(facets.Capabilities, map[string]int{"function_calling": 2, "vision": 2}) {
		t.Errorf("Capabilities = %v, want function_calling:2 vision:2", facets.Capabilities)
	}
}

func TestNormalizeMarketplaceFacetRows(t *testing.T) {
	rows := []marketplaceFacetRow{
		{Category: "chat", Vendor: "", Capabilities: "", BillingMode: ""},
		{Category: "chat", Vendor: "", Capabilities: `{"streaming":1,"reasoning":"true","json_mode":false,"audio":0}`, BillingMode: "per_second"},
	}

	entries := normalizeMarketplaceFacetRows(rows)

	if entries[0].vendor != vendorFacetUncategorized {
		t.Errorf("空厂商应归入 %s，got %s", vendorFacetUncategorized, entries[0].vendor)
	}
	if entries[0].billing != "token" {
		t.Errorf("空计费模式应兜底 token，got %s", entries[0].billing)
	}
	// truthy 判定：true/1/"true" 为真，false/0 为假
	want := setOf("streaming", "reasoning")
	if !reflect.DeepEqual(entries[1].capKeys, want) {
		t.Errorf("capKeys = %v, want %v", entries[1].capKeys, want)
	}
}

func TestAggregateMarketplaceFacetsEmptyRows(t *testing.T) {
	facets, vendors := aggregateMarketplaceFacets(nil, marketplaceFacetFilters{
		Categories: setOf("chat"),
	})

	if facets.Categories == nil || facets.Vendors == nil || facets.Capabilities == nil || facets.BillingModes == nil {
		t.Fatal("空输入时各维度应为空 map 而非 nil（JSON 序列化为 {}）")
	}
	if len(facets.Categories) != 0 || len(vendors) != 0 {
		t.Errorf("空输入应无计数，got %v %v", facets.Categories, vendors)
	}
}
