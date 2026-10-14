package tenant

import (
	"encoding/json"
	"sort"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// 分面统计：模型广场筛选面板的析取计数（disjunctive facets）。
// 每个维度在「应用其余维度筛选后」的剩余集合内计数，自身维度的已选值
// 不参与自身计数——勾选 OpenAI 后仍能看到 Anthropic 的真实剩余数，
// 计数为 0 的选项由前端禁用置灰，保证点任意可选项都有结果。

// marketplaceFacetRow 分面统计投影行：keyword 基础条件下（不含任何维度筛选）
// 的全量候选模型，每模型一行四列
type marketplaceFacetRow struct {
	Category     string `json:"category"`
	Vendor       string `json:"vendor"`
	Capabilities string `json:"capabilities"`
	BillingMode  string `json:"billing_mode"`
}

// marketplaceFacetFilters 当前已选维度集合（析取计数时排除自身维度）
type marketplaceFacetFilters struct {
	Categories   map[string]struct{}
	Vendors      map[string]struct{}
	Capabilities map[string]struct{}
	BillingMode  string
}

// marketplaceFacetRowLimit 分面统计行数上限（增长阀）：目录超过该规模时跳过
// 统计，计数字段返回空，前端隐藏计数只渲染选项，避免一次拉全量投影的开销
const marketplaceFacetRowLimit = 5000

// vendorFacetUncategorized 未设置厂商（空串）的归并键，前端映射为「未分类」
const vendorFacetUncategorized = "uncategorized"

// marketplaceFacetEntry 归一化后的分面统计条目
type marketplaceFacetEntry struct {
	category string
	vendor   string
	capKeys  map[string]struct{}
	billing  string
}

// normalizeMarketplaceFacetRows 原始投影行 → 归一化条目：
// 空厂商归入 uncategorized；空计费模式兜底 token（与展示侧口径一致）；
// capabilities JSONB 只保留为真值的能力键（truthy 判定与前端 getCapabilityList 一致）
func normalizeMarketplaceFacetRows(rows []marketplaceFacetRow) []marketplaceFacetEntry {
	entries := make([]marketplaceFacetEntry, 0, len(rows))
	for _, row := range rows {
		entry := marketplaceFacetEntry{
			category: row.Category,
			vendor:   row.Vendor,
			capKeys:  map[string]struct{}{},
			billing:  row.BillingMode,
		}
		if entry.vendor == "" {
			entry.vendor = vendorFacetUncategorized
		}
		if entry.billing == "" {
			entry.billing = "token"
		}
		if row.Capabilities != "" && row.Capabilities != "{}" {
			// encoding/json 解析保证值类型确定（bool/float64/string），
			// gconv 对 map 的值类型归一行为不可预期
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(row.Capabilities), &raw); err == nil {
				for key, value := range raw {
					if isTruthyCapability(value) {
						entry.capKeys[key] = struct{}{}
					}
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// isTruthyCapability 能力值为真判定：true / 数值 1 / 字符串 "true"
// （管理后台写入为布尔，数值/字符串形态做兼容兜底）
func isTruthyCapability(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v == 1
	case int:
		return v == 1
	case int64:
		return v == 1
	case string:
		return v == "true"
	default:
		return false
	}
}

// aggregateMarketplaceFacets 析取聚合四个维度的分面计数。
// 返回值第二项为厂商去重列表（计数 > 0 的键，字典序，含 uncategorized），
// 供列表响应的 vendors 字段复用。
func aggregateMarketplaceFacets(rows []marketplaceFacetRow, f marketplaceFacetFilters) (v1.MarketplaceFacets, []string) {
	facets := v1.MarketplaceFacets{
		Categories:   map[string]int{},
		Vendors:      map[string]int{},
		Capabilities: map[string]int{},
		BillingModes: map[string]int{},
	}
	entries := normalizeMarketplaceFacetRows(rows)

	// 各维度匹配谓词：已选集合为空 = 未选 = 全部匹配
	matchCategory := func(e marketplaceFacetEntry) bool {
		if len(f.Categories) == 0 {
			return true
		}
		_, ok := f.Categories[e.category]
		return ok
	}
	matchVendor := func(e marketplaceFacetEntry) bool {
		if len(f.Vendors) == 0 {
			return true
		}
		_, ok := f.Vendors[e.vendor]
		return ok
	}
	// 能力为 AND 语义：行需具备全部已选能力
	matchCapabilities := func(e marketplaceFacetEntry) bool {
		for key := range f.Capabilities {
			if _, ok := e.capKeys[key]; !ok {
				return false
			}
		}
		return true
	}
	matchBilling := func(e marketplaceFacetEntry) bool {
		return f.BillingMode == "" || f.BillingMode == e.billing
	}

	// 每个维度的计数只应用其余三个维度的谓词（排除自身）
	for _, e := range entries {
		if matchVendor(e) && matchCapabilities(e) && matchBilling(e) {
			facets.Categories[e.category]++
		}
	}
	for _, e := range entries {
		if matchCategory(e) && matchCapabilities(e) && matchBilling(e) {
			facets.Vendors[e.vendor]++
		}
	}
	for _, e := range entries {
		if matchCategory(e) && matchVendor(e) && matchBilling(e) {
			for key := range e.capKeys {
				facets.Capabilities[key]++
			}
		}
	}
	for _, e := range entries {
		if matchCategory(e) && matchVendor(e) && matchCapabilities(e) {
			facets.BillingModes[e.billing]++
		}
	}

	vendors := make([]string, 0, len(facets.Vendors))
	for vendor, count := range facets.Vendors {
		if count > 0 {
			vendors = append(vendors, vendor)
		}
	}
	sort.Strings(vendors)
	return facets, vendors
}
