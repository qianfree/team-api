// 文本转换器配对（Req+Resp）注册表层。包级文档见 doc.go。
//
// TextConverterSpec 是请求侧与响应侧的成对外观；registerBuiltinTextConverter
// 将其拆分并分别注册进 request / response 注册表（同 ID）。
package relayconvert

import (
	"fmt"
	"strings"
	"sync"

	"github.com/qianfree/team-api/relaykit/types"
)

type TextConverterQuality string

const (
	TextConverterQualityGood        TextConverterQuality = "good"
	TextConverterQualityFair        TextConverterQuality = "fair"
	TextConverterQualityDiscouraged TextConverterQuality = "discouraged"
)

type TextRequestSide struct {
	Convert        RequestConverterFunc
	StepConverters []string
}

type TextResponseSide struct {
	Convert        ResponseConverterFunc
	StepConverters []string
	Aliases        []string
}

type TextConverterSpec struct {
	ID      string
	From    types.RelayFormat
	To      types.RelayFormat
	Quality TextConverterQuality
	Req     TextRequestSide
	Resp    TextResponseSide
}

var (
	textConverterMu      sync.RWMutex
	textConverters       = make(map[string]TextConverterSpec)
	textConverterAliases = make(map[string]string)
)

// builtinTextConverters 与 init() 注入：这里会出现
//
//	var builtinTextConverters = []TextConverterSpec{ ... 12 个内置转换器 ... }
//	func init() { for _, spec := range builtinTextConverters { registerBuiltinTextConverter(spec) } }

func LookupTextConverter(converter string) (TextConverterSpec, bool) {
	textConverterMu.RLock()
	defer textConverterMu.RUnlock()

	converterID := resolveTextConverterID(converter)
	spec, ok := textConverters[converterID]
	if !ok {
		return TextConverterSpec{}, false
	}
	return cloneTextConverterSpec(spec), true
}

// RegisterTextConverter 将一个 TextConverterSpec 拆分为请求侧与响应侧，
// 分别注册进两张注册表（同 ID），并登记响应侧别名。
// 导出供内部转换器包的 init() 调用。
func RegisterTextConverter(spec TextConverterSpec) {
	registerBuiltinTextConverter(spec)
}

// registerBuiltinTextConverter 将一个 TextConverterSpec 拆分为请求侧与响应侧，
// 分别注册进两张注册表（同 ID），并登记响应侧别名。init() 调用本函数。
//
// 并发安全：持有 textConverterMu 写锁保护注册过程，防止 data race。
// 虽然通常在包 init() 中调用（单线程），但加锁确保未来动态注册或测试并发场景安全。
func registerBuiltinTextConverter(spec TextConverterSpec) {
	spec.ID = strings.TrimSpace(spec.ID)
	if spec.ID == "" {
		panic("text converter ID is required")
	}
	if spec.From == "" || spec.To == "" {
		panic(fmt.Sprintf("text converter %q must declare from and to formats", spec.ID))
	}
	if spec.Quality == "" {
		panic(fmt.Sprintf("text converter %q must declare quality", spec.ID))
	}
	// 允许单侧 spec：仅请求方向（Req-only）或仅响应方向（Resp-only）的转换器合法，
	// 至少一侧已配置即可（如 Responses→OpenAI chat 请求侧转换暂无对应响应侧实现）。
	if !textRequestSideConfigured(spec.Req) && !textResponseSideConfigured(spec.Resp) {
		panic(fmt.Sprintf("text converter %q must declare request or response conversion", spec.ID))
	}

	textConverterMu.Lock()
	defer textConverterMu.Unlock()

	if _, exists := textConverters[spec.ID]; exists {
		panic(fmt.Sprintf("text converter %q is already registered", spec.ID))
	}

	if textRequestSideConfigured(spec.Req) {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID:             spec.ID,
			From:           spec.From,
			To:             spec.To,
			Quality:        RequestConverterQuality(spec.Quality),
			Convert:        spec.Req.Convert,
			StepConverters: cloneTextConverterStrings(spec.Req.StepConverters),
		})
	}
	if textResponseSideConfigured(spec.Resp) {
		registerBuiltinResponseConverter(ResponseConverterSpec{
			ID:             spec.ID,
			From:           spec.From,
			To:             spec.To,
			Quality:        ResponseConverterQuality(spec.Quality),
			Convert:        spec.Resp.Convert,
			StepConverters: cloneTextConverterStrings(spec.Resp.StepConverters),
		})
	}

	textConverters[spec.ID] = cloneTextConverterSpec(spec)
	for _, alias := range spec.Resp.Aliases {
		registerResponseConverterAlias(alias, spec.ID)
		registerTextConverterAlias(alias, spec.ID)
	}
}

func registerTextConverterAlias(alias string, converter string) {
	alias = strings.TrimSpace(alias)
	converter = strings.TrimSpace(converter)
	if alias == "" {
		panic("text converter alias is required")
	}
	if converter == "" {
		panic(fmt.Sprintf("text converter alias %q target is required", alias))
	}
	if alias == converter {
		return
	}
	if _, exists := textConverters[alias]; exists {
		panic(fmt.Sprintf("text converter alias %q conflicts with registered converter", alias))
	}
	if _, exists := textConverters[converter]; !exists {
		panic(fmt.Sprintf("text converter alias %q references unknown converter %q", alias, converter))
	}
	if existing, exists := textConverterAliases[alias]; exists && existing != converter {
		panic(fmt.Sprintf("text converter alias %q is already registered for %q", alias, existing))
	}
	textConverterAliases[alias] = converter
}

func textRequestSideConfigured(side TextRequestSide) bool {
	return side.Convert != nil || len(side.StepConverters) > 0
}

func textResponseSideConfigured(side TextResponseSide) bool {
	return side.Convert != nil || len(side.StepConverters) > 0
}

func resolveTextConverterID(converter string) string {
	converter = strings.TrimSpace(converter)
	if canonical, ok := textConverterAliases[converter]; ok {
		return canonical
	}
	return converter
}

func cloneTextConverterSpec(spec TextConverterSpec) TextConverterSpec {
	spec.Req.StepConverters = cloneTextConverterStrings(spec.Req.StepConverters)
	spec.Resp.StepConverters = cloneTextConverterStrings(spec.Resp.StepConverters)
	spec.Resp.Aliases = cloneTextConverterStrings(spec.Resp.Aliases)
	return spec
}

func cloneTextConverterStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string{}, values...)
}
