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
// 与本系统 per_request 单价/全部 bil_ 层金额同为本位币口径，无 new-api 的积分换算；
// 仅内置种子价以 USD 厂商列表价记账，构建索引时按当前本位币折算（见 seedToolPricesUSD）
const ToolCallPriceOptionKey = "tool_call_prices"

// MaxBillableToolCallCount 单工具单请求的计费次数上限：上游异常 usage（脏数据/恶意放大）
// 不得直接变成天价账单，超出部分免费。对齐 new-api MaxBillableToolCallCount。
const MaxBillableToolCallCount = 10_000

// seedToolPricesUSD 内置工具默认价（USD 口径，各厂商列表价 $/1K calls ÷ 1000）。
// 计费生效价 = 种子价 × 当前本位币折算率（本位币=USD 时为 1；CNY 时取支付配置
// 汇率的倒数，汇率调整后随索引缓存重建生效）——解析出的单价始终是本位币，
// 与快照/摘要/钱包扣款同币种。管理端 tool_call_prices 配置值本身是本位币，
// 覆盖时不二次折算；置 0 = 显式关闭该工具计费（数值 0 是终结值）。
// 部署即生效（工具计费开启的默认意图）。
//
// P0 采集的 key（有渠道填充 usage.ToolCalls）：web_search（Claude server_tool_use /
// OpenAI Responses web_search_call）、google_search（Gemini grounding 查询数）。
// 其余 key 为 P1 渠道接入预留——未采集的渠道不产生对应计数，配置价格自然不生效。
// 智谱/阿里系（CNY 列表价）不设种子：汇率随时间漂移，接入采集时由管理员按当期价格配置。
var seedToolPricesUSD = map[string]float64{
	// Anthropic Web Search / OpenAI web_search：$10 / 1K searches
	"web_search": 0.01,
	// OpenAI Responses web_search_preview：$10 / 1K；gpt-4o / 4.1 系 $25 / 1K
	"web_search_preview":               0.01,
	"web_search_preview:gpt-4o*":       0.025,
	"web_search_preview:gpt-4.1*":      0.025,
	"web_search_preview:gpt-4o-mini*":  0.025,
	"web_search_preview:gpt-4.1-mini*": 0.025,
	// Anthropic Web Fetch Tool：$10 / 1K fetches（P1 采集，DTO 需扩字段）
	"web_fetch": 0.01,
	// OpenAI file_search：$2.5 / 1K calls（P1 采集）
	"file_search": 0.0025,
	// Gemini grounding（ai.google.dev/gemini-api/docs/pricing）：
	// Gemini 3 按 search query 计 $14/1K；2.5 及更早按 grounded prompt 计 $35/1K、query 免费
	"google_search":                             0.014,
	"google_search:gemini-2.5*":                 0,
	"google_search:gemini-2.0*":                 0,
	"google_search:gemini-1.5*":                 0,
	"google_search_grounded_prompt:gemini-2.5*": 0.035,
	"google_search_grounded_prompt:gemini-2.0*": 0.035,
	"google_search_grounded_prompt:gemini-1.5*": 0.035,
}

// toolPrefixEntry 模型前缀覆盖条目（prefix 已去掉尾部 *，纯前缀匹配）
type toolPrefixEntry struct {
	prefix string
	price  float64
}

// toolPriceIndex 工具价格查找索引：种子价 + 管理端覆盖合并构建（构建路径非热路径）。
// defaults 为无模型维度的工具默认价；prefixes 按工具分组、前缀长度降序（最长前缀优先）
type toolPriceIndex struct {
	defaults map[string]float64
	prefixes map[string][]toolPrefixEntry
}

// toolPriceSnapshot 缓存键 → 索引的缓存对：键 = 配置串 + 折算率（任一变化触发重建，
// 支付汇率调整后新价格即时生效）。ConfigService 自带 L1/L2 缓存与 pub/sub 失效，
// 外层仅需挡住「同一键反复建索引」
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

// buildToolPriceIndex 从原始配置串构建索引：种子价（USD）× 折算率打底，
// 管理端配置（本位币）逐 key 覆盖——覆盖值不二次折算。
// usdToBase = 1 表示本位币为 USD（种子原样）；非法折算率（≤0/NaN/Inf）按 1 兜底。
// 读侧容错（对齐 ParseTenantPricingPatch 的坏数据不阻断计费原则）：
// 非法条目逐个忽略并保留合法兄弟条目，整份非法时退回纯种子价
func buildToolPriceIndex(raw string, usdToBase float64) *toolPriceIndex {
	if !isValidToolCallPrice(usdToBase) || usdToBase <= 0 {
		usdToBase = 1.0
	}
	merged := make(map[string]float64, len(seedToolPricesUSD)+8)
	for k, v := range seedToolPricesUSD {
		merged[k] = v * usdToBase
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" && trimmed != "null" {
		var prices map[string]float64
		if err := json.Unmarshal([]byte(trimmed), &prices); err != nil {
			prices = nil // 整份非法：退回种子价
		}
		for k, v := range prices {
			if !toolPriceKeyPattern.MatchString(k) || !isValidToolCallPrice(v) {
				continue // 单条非法：忽略，兄弟条目继续生效
			}
			merged[k] = v
		}
	}

	idx := &toolPriceIndex{
		defaults: make(map[string]float64, len(merged)),
		prefixes: make(map[string][]toolPrefixEntry),
	}
	for key, price := range merged {
		before, after, hasColon := strings.Cut(key, ":")
		if !hasColon {
			idx.defaults[key] = price
			continue
		}
		prefix := strings.TrimSuffix(after, "*")
		idx.prefixes[before] = append(idx.prefixes[before], toolPrefixEntry{prefix: prefix, price: price})
	}
	// 最长前缀优先（同长度按字典序保证跨次稳定，对齐 new-api 排序语义）
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

// loadToolPriceIndex 取当前生效的工具价格索引（缓存键 = 配置串 + 折算率，未变直接复用）。
// 计费热路径每请求至多一次；配置更新经 ConfigService 缓存失效后串变化触发重建，
// 支付汇率调整后折算率变化同样触发重建
func loadToolPriceIndex(ctx context.Context) *toolPriceIndex {
	raw := strings.TrimSpace(lcommon.Config().GetOption(ctx, ToolCallPriceOptionKey))
	usdToBase := toolSeedUSDToBase(ctx)
	key := fmt.Sprintf("%s\n%.10g", raw, usdToBase)
	if snap := toolPriceCache.Load(); snap != nil && snap.key == key {
		return snap.idx
	}
	idx := buildToolPriceIndex(raw, usdToBase)
	toolPriceCache.Store(&toolPriceSnapshot{key: key, idx: idx})
	return idx
}

// toolSeedUSDToBase 种子价 USD → 本位币折算率：本位币=USD 时恒 1（无换算）；
// 本位币=CNY 时取支付配置汇率（payment_exchange_rate_cny_to_usd）的倒数。
// 配置基础设施不可用（单测环境无 DB 驱动触发 panic）时 recover 兜底 1——
// 与 Currency() 的 USD 回退口径一致（此时本位币判定本就回退 USD）
func toolSeedUSDToBase(ctx context.Context) (rate float64) {
	rate = 1.0
	defer func() {
		if r := recover(); r != nil {
			rate = 1.0
		}
	}()
	if IsCNY(ctx) {
		rate = GetExchangeRateUSDToCNY(ctx)
	}
	return rate
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
