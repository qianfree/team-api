package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// ToolCallPriceOptionKey 工具调用计费的 sys_options 键（billing 分类，JSON 类型）。
// 值为「工具名 → 每次调用单价（本位币）」映射，key 支持「工具名:模型前缀*」按模型覆盖。
//
// 配置是唯一价源：程序不内置任何工具价格，未配置的工具/模型前缀一律免费（解析为 0）。
// 故意不设内置价——厂商列表价随时调整，写死在二进制里既无法跟价，也会让「后台配置」
// 变成一层不可见的覆盖关系（改了配置还要猜底层是不是有种子价兜着）。
// 与本系统 per_request 单价/全部 bil_ 层金额同为本位币口径：无积分换算、无汇率折算，
// 管理员填什么就收什么，调整支付汇率不会让已生效的价格漂移
const ToolCallPriceOptionKey = "tool_call_prices"

// MaxBillableToolCallCount 单工具单请求的计费次数上限：上游异常 usage（脏数据/恶意放大）
// 不得直接变成天价账单，超出部分免费。
const MaxBillableToolCallCount = 10_000

// toolPrefixEntry 模型前缀覆盖条目（prefix 已去掉尾部 *，纯前缀匹配）
type toolPrefixEntry struct {
	prefix string
	price  float64
}

// toolPriceIndex 工具价格查找索引（仅由管理端配置构建）。
// defaults 为无模型维度的工具默认价；prefixes 按工具分组、前缀长度降序（最长前缀优先）
type toolPriceIndex struct {
	defaults map[string]float64
	prefixes map[string][]toolPrefixEntry
}

// toolPriceSnapshot 缓存键 → 索引的缓存对：键 = 配置串（配置变更即重建）。
// ConfigService 自带 L1/L2 缓存与 pub/sub 失效，外层仅需挡住「同一键反复建索引」
type toolPriceSnapshot struct {
	key string
	idx *toolPriceIndex
}

var toolPriceCache atomic.Pointer[toolPriceSnapshot]

// toolPriceKeyPattern 合法价格键格式：工具名（小写字母/数字/下划线），
// 可选「:模型前缀*」（模型名允许大小写/数字/点/横线/下划线；尾部 * 为通配语义可省略——
// 省略时按纯前缀匹配；* 出现在中部会被当字面量，模型名不含 * 故只会不命中，无安全影响）
var toolPriceKeyPattern = regexp.MustCompile(`^[a-z0-9_]+(:[A-Za-z0-9._*\-]+)?$`)

// maxToolCallPrice 单工具单价上限（本位币）：防手滑配置出天文数字（每次调用 100 本位币已远超任何云端工具列表价）
const maxToolCallPrice = 100.0

// isValidToolCallPrice 价格合法性：非负、有限
func isValidToolCallPrice(price float64) bool {
	return price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

// ValidateToolCallPricesJSON 管理端保存校验：必须是 JSON object，key 格式合法，价格在 [0, maxToolCallPrice]。
// 数值 0 合法（显式关闭该工具计费）。整份校验不忽略坏条目——保存入口把错误顶回给管理员
func ValidateToolCallPricesJSON(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	var rawPrices map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &rawPrices); err != nil {
		return errInvalidToolPrice("必须是合法的 JSON 对象，如 {\"web_search\": 0.01}")
	}
	for key, rawPrice := range rawPrices {
		if !toolPriceKeyPattern.MatchString(key) {
			return errInvalidToolPrice("key %q 格式非法：应为 工具名 或 工具名:模型前缀*", key)
		}
		var price float64
		if err := json.Unmarshal(rawPrice, &price); err != nil {
			return errInvalidToolPrice("key %q 的价格必须是数字", key)
		}
		if !isValidToolCallPrice(price) || price > maxToolCallPrice {
			return errInvalidToolPrice("key %q 的价格必须在 0~%g 之间", key, maxToolCallPrice)
		}
	}
	return nil
}

// buildToolPriceIndex 从原始配置串构建索引——配置串即唯一价源，没有内置种子价打底，
// 未出现的工具/前缀一律解析为 0（免费）。
// 读侧容错（对齐 ParseTenantPricingPatch 的坏数据不阻断计费原则）：
// 非法条目逐个忽略并保留合法兄弟条目，整份非法时等价于未配置（全部免费）
func buildToolPriceIndex(raw string) *toolPriceIndex {
	idx := &toolPriceIndex{
		defaults: make(map[string]float64),
		prefixes: make(map[string][]toolPrefixEntry),
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return idx
	}
	var prices map[string]float64
	if err := json.Unmarshal([]byte(trimmed), &prices); err != nil {
		return idx // 整份非法：等价于未配置
	}
	for key, price := range prices {
		// key 格式、非负有限、上限三重校验与保存侧 ValidateToolCallPricesJSON 同口径，
		// 防止绕过接口直接改 DB 时配出负价或天文数字
		if !toolPriceKeyPattern.MatchString(key) || !isValidToolCallPrice(price) || price > maxToolCallPrice {
			continue
		}
		before, after, hasColon := strings.Cut(key, ":")
		if !hasColon {
			idx.defaults[before] = price
			continue
		}
		prefix := strings.TrimSuffix(after, "*")
		idx.prefixes[before] = append(idx.prefixes[before], toolPrefixEntry{prefix: prefix, price: price})
	}
	// 最长前缀优先（同长度按字典序保证跨次稳定）
	for tool, entries := range idx.prefixes {
		sort.Slice(entries, func(i, j int) bool {
			if len(entries[i].prefix) == len(entries[j].prefix) {
				return entries[i].prefix < entries[j].prefix
			}
			return len(entries[i].prefix) > len(entries[j].prefix)
		})
		idx.prefixes[tool] = entries
	}
	return idx
}

// loadToolPriceIndex 取当前生效的工具价格索引（缓存键 = 配置串，未变直接复用）。
// 计费热路径每请求至多一次；配置更新经 ConfigService 缓存失效后串变化触发重建
func loadToolPriceIndex(ctx context.Context) *toolPriceIndex {
	raw := strings.TrimSpace(lcommon.Config().GetOption(ctx, ToolCallPriceOptionKey))
	if snap := toolPriceCache.Load(); snap != nil && snap.key == raw {
		return snap.idx
	}
	idx := buildToolPriceIndex(raw)
	toolPriceCache.Store(&toolPriceSnapshot{key: raw, idx: idx})
	return idx
}

// ResolveToolPrice 查工具单价：最长模型前缀匹配 → 工具默认价 → 0（免费，未配置/显式关闭）。
// modelName 为计费模型名（结算入参的 modelName，非渠道上游映射名）
func ResolveToolPrice(ctx context.Context, toolName, modelName string) float64 {
	return loadToolPriceIndex(ctx).resolve(toolName, modelName)
}

// resolve 索引内查找（纯函数，便于单测）
func (idx *toolPriceIndex) resolve(toolName, modelName string) float64 {
	if entries, ok := idx.prefixes[toolName]; ok && modelName != "" {
		for _, e := range entries {
			if strings.HasPrefix(modelName, e.prefix) {
				return e.price
			}
		}
	}
	if p, ok := idx.defaults[toolName]; ok {
		return p
	}
	return 0
}

// errInvalidToolPrice 校验错误构造（统一前缀，便于管理端原样展示）
func errInvalidToolPrice(format string, args ...any) error {
	return fmt.Errorf("工具调用价格表配置非法: "+format, args...)
}
