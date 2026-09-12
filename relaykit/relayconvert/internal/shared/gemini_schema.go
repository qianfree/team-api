package shared

import "strings"

// geminiSchemaAllowedKeys Gemini Schema 支持的 JSON Schema 关键字白名单。
// OpenAI 的 function.parameters 与 Claude 的 input_schema 同源，都可能携带
// $schema、additionalProperties、$defs 等 Gemini 会拒绝的关键字，需按此过滤。
var geminiSchemaAllowedKeys = map[string]struct{}{
	"type": {}, "description": {}, "properties": {}, "required": {}, "items": {},
	"anyOf": {}, "default": {}, "enum": {}, "format": {}, "maxLength": {},
	"minLength": {}, "maximum": {}, "minimum": {}, "pattern": {}, "title": {},
	"nullable": {}, "maxItems": {}, "minItems": {}, "maxProperties": {},
	"minProperties": {}, "example": {},
}

// CleanGeminiToolParams 把 OpenAI function.parameters / Claude input_schema 归一化
// 为 Gemini Schema 接受的形态，claude→gemini 与 openai→gemini 两条请求路径共用。
//
// 早期版本只做顶层白名单过滤、嵌套原样透传，但 Gemini 实际并不宽容——线上真实流量
// （Claude Code 的 Grep 工具）携带 JSON Schema 2020-12 的 prefixItems 元组语法，
// Gemini 丢弃该未知关键字后留下无 items 的裸数组，直接 400：
//
//	GenerateContentRequest...properties[where].items.items: missing field
//
// 因此这里递归做四件事：
//  1. 逐层白名单过滤（$schema / $defs / additionalProperties 等 Gemini 拒绝的关键字）
//  2. prefixItems 元组折叠为 items.anyOf（Gemini 无元组语义，取各位置 schema 的并集近似）
//  3. 缺 type 的节点按内容推断兜底（properties→object / items→array / enum→string，
//     其余→string）——Gemini 要求每个 Schema 节点有确定类型，空节点同样被拒
//  4. type=array 却无 items 的节点补 items（Gemini 对数组元素 schema 有硬性要求）
//
// 非对象输入（含 nil）原样返回。
func CleanGeminiToolParams(params any) any {
	return cleanGeminiSchema(params)
}

func cleanGeminiSchema(node any) any {
	m, ok := node.(map[string]any)
	if !ok {
		return node
	}
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		if _, allowed := geminiSchemaAllowedKeys[k]; !allowed {
			continue
		}
		switch k {
		case "properties":
			if props, ok := v.(map[string]any); ok {
				converted := make(map[string]any, len(props))
				for pk, pv := range props {
					converted[pk] = cleanGeminiSchema(pv)
				}
				out[k] = converted
			} else {
				out[k] = v
			}
		case "items":
			out[k] = cleanGeminiSchema(v)
		case "anyOf":
			if arr, ok := v.([]any); ok {
				converted := make([]any, len(arr))
				for i, el := range arr {
					converted[i] = cleanGeminiSchema(el)
				}
				out[k] = converted
			} else {
				out[k] = v
			}
		default:
			out[k] = v
		}
	}

	// prefixItems 元组 → items.anyOf 并集近似（prefixItems 不在白名单内，
	// 上面的循环不会写出它；这里从原始节点读取后折叠，避免裸数组漏出去）
	if arr, ok := m["prefixItems"].([]any); ok && len(arr) > 0 {
		anyOf := make([]any, 0, len(arr)+1)
		for _, el := range arr {
			anyOf = append(anyOf, cleanGeminiSchema(el))
		}
		// 2020-12 语义：items 约束 prefixItems 之后的尾部元素，并入并集
		if prev, ok := out["items"].(map[string]any); ok {
			anyOf = append(anyOf, prev)
		}
		out["items"] = map[string]any{"anyOf": anyOf}
	}

	// type 兜底。携带 anyOf 的并集节点不加 type——那是 Gemini 自身的 union 形态
	if _, hasType := out["type"]; !hasType {
		if _, hasAnyOf := out["anyOf"]; !hasAnyOf {
			switch {
			case out["properties"] != nil:
				out["type"] = "object"
			case out["items"] != nil:
				out["type"] = "array"
			case out["enum"] != nil:
				out["type"] = "string"
			default:
				out["type"] = "string"
			}
		}
	}

	// 数组必须携带元素 schema：Gemini 对裸数组直接 400（missing field），
	// 无法推断元素类型时退化为 string（工具参数 schema 只是对模型的提示，
	// 合法性优先于精确性）
	if t, _ := out["type"].(string); strings.EqualFold(t, "array") {
		if _, hasItems := out["items"]; !hasItems {
			out["items"] = map[string]any{"type": "string"}
		}
	}
	return out
}
