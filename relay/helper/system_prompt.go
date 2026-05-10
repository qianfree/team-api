package helper

import (
	"encoding/json"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// InjectSystemPrompt 在 ConvertRequest 之后、ParamOverride 之前注入系统提示词
// 此时 body 已转换为目标供应商的原生格式
func InjectSystemPrompt(body []byte, info *common.RelayInfo) []byte {
	prompt := info.ChannelMeta.Settings.SystemPrompt
	if prompt == "" {
		return body
	}

	nativeFormat := providerNativeFormat(info.ChannelMeta.ChannelType)
	switch nativeFormat {
	case constant.RelayFormatClaude:
		return injectSystemPromptClaude(body, prompt, info.ChannelMeta.Settings.SystemPromptOverride)
	case constant.RelayFormatGemini:
		return injectSystemPromptGemini(body, prompt, info.ChannelMeta.Settings.SystemPromptOverride)
	default:
		return injectSystemPromptOpenAI(body, prompt, info.ChannelMeta.Settings.SystemPromptOverride)
	}
}

// injectSystemPromptOpenAI 向 OpenAI 格式请求注入系统提示词
func injectSystemPromptOpenAI(body []byte, prompt string, override bool) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	messagesRaw, ok := req["messages"]
	if !ok {
		return body
	}

	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body
	}

	systemMsg := map[string]any{"role": "system", "content": prompt}

	if len(messages) == 0 {
		// 无消息，插入系统消息
		systemRaw, _ := json.Marshal(systemMsg)
		messages = append([]json.RawMessage{systemRaw}, messages...)
	} else {
		// 检查第一条是否为 system 消息
		var first map[string]json.RawMessage
		_ = json.Unmarshal(messages[0], &first)

		if roleBytes, ok := first["role"]; ok {
			var role string
			_ = json.Unmarshal(roleBytes, &role)
			if role == "system" {
				if override {
					// 替换模式：用渠道系统提示词替换
					systemRaw, _ := json.Marshal(systemMsg)
					messages[0] = systemRaw
				} else {
					// 追加模式：在原有 system content 前面插入提示词
					var contentVal any
					if contentRaw, ok := first["content"]; ok {
						_ = json.Unmarshal(contentRaw, &contentVal)
					}
					newContent := prompt
					if contentVal != nil {
						switch cv := contentVal.(type) {
						case string:
							newContent = prompt + "\n" + cv
						}
					}
					first["content"], _ = json.Marshal(newContent)
					messages[0], _ = json.Marshal(first)
				}
			} else {
				// 第一条不是 system，在前面插入
				systemRaw, _ := json.Marshal(systemMsg)
				messages = append([]json.RawMessage{systemRaw}, messages...)
			}
		} else {
			systemRaw, _ := json.Marshal(systemMsg)
			messages = append([]json.RawMessage{systemRaw}, messages...)
		}
	}

	req["messages"], _ = json.Marshal(messages)
	result, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return result
}

// injectSystemPromptClaude 向 Claude 格式请求注入系统提示词
func injectSystemPromptClaude(body []byte, prompt string, override bool) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	systemRaw, hasSystem := req["system"]

	if !hasSystem || override {
		// 无 system 或 override 模式：直接设置
		req["system"] = marshalClaudeSystem(prompt)
	} else {
		// 追加模式：在原有 system 前面插入提示词
		req["system"] = prependClaudeSystem(systemRaw, prompt)
	}

	result, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return result
}

func marshalClaudeSystem(prompt string) json.RawMessage {
	// Claude system 可以是 string 或 array of content blocks
	return json.RawMessage(`"` + jsonEscapeString(prompt) + `"`)
}

func prependClaudeSystem(systemRaw json.RawMessage, prompt string) json.RawMessage {
	// 尝试解析为 string
	var strVal string
	if err := json.Unmarshal(systemRaw, &strVal); err == nil {
		newVal := prompt + "\n" + strVal
		b, _ := json.Marshal(newVal)
		return b
	}

	// 尝试解析为 array
	var arr []map[string]any
	if err := json.Unmarshal(systemRaw, &arr); err == nil {
		// 在第一个 text block 前面插入
		textBlock := map[string]any{"type": "text", "text": prompt + "\n"}
		arr = append([]map[string]any{textBlock}, arr...)
		b, _ := json.Marshal(arr)
		return b
	}

	// 无法解析，直接替换
	return marshalClaudeSystem(prompt)
}

// injectSystemPromptGemini 向 Gemini 格式请求注入系统提示词
func injectSystemPromptGemini(body []byte, prompt string, override bool) []byte {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	instrRaw, hasInstr := req["systemInstruction"]

	if !hasInstr || override {
		req["systemInstruction"] = marshalGeminiSystemInstruction(prompt)
	} else {
		// 追加模式
		req["systemInstruction"] = prependGeminiSystemInstruction(instrRaw, prompt)
	}

	result, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return result
}

func marshalGeminiSystemInstruction(prompt string) json.RawMessage {
	instr := map[string]any{
		"parts": []map[string]any{{"text": prompt}},
	}
	b, _ := json.Marshal(instr)
	return b
}

func prependGeminiSystemInstruction(instrRaw json.RawMessage, prompt string) json.RawMessage {
	var instr map[string]json.RawMessage
	if err := json.Unmarshal(instrRaw, &instr); err != nil {
		return marshalGeminiSystemInstruction(prompt)
	}

	partsRaw, ok := instr["parts"]
	if !ok {
		return marshalGeminiSystemInstruction(prompt)
	}

	var parts []map[string]any
	if err := json.Unmarshal(partsRaw, &parts); err != nil {
		return marshalGeminiSystemInstruction(prompt)
	}

	// 在第一个 text part 前面插入
	textPart := map[string]any{"text": prompt + "\n"}
	parts = append([]map[string]any{textPart}, parts...)
	instr["parts"], _ = json.Marshal(parts)

	b, _ := json.Marshal(instr)
	return b
}

// providerNativeFormat 返回供应商的原生请求格式（与 passthrough.go 中的逻辑一致）
func providerNativeFormat(providerType int) constant.RelayFormat {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return constant.RelayFormatClaude
	case constant.ProviderGemini:
		return constant.RelayFormatGemini
	default:
		return constant.RelayFormatOpenAI
	}
}

// jsonEscapeString 对字符串进行 JSON 转义
func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
