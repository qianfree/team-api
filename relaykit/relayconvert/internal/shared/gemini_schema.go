package shared

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

// CleanGeminiToolParams 过滤 Gemini Schema 不支持的 JSON Schema 关键字。
// 只做顶层白名单过滤，嵌套结构原样透传（与 Gemini 的宽容度一致）。
// 非对象输入（含 nil）原样返回。
func CleanGeminiToolParams(params any) any {
	m, ok := params.(map[string]any)
	if !ok {
		return params
	}
	cleaned := make(map[string]any, len(m))
	for k, v := range m {
		if _, allowed := geminiSchemaAllowedKeys[k]; allowed {
			cleaned[k] = v
		}
	}
	return cleaned
}
