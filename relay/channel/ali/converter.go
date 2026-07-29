package ali

import (
	"encoding/json"
	"fmt"
	"strings"
)

// convertRequest 转换 OpenAI 格式请求以适配 DashScope。
//   - 处理 top_p 参数的合法范围限制：DashScope 要求 top_p 在 (0, 1) 开区间内
//   - 剥离上游模型不支持 thinking_budget 时的 thinking_budget 参数（避免上游 400）
//
// upstreamModelName 为映射后的实际上游模型名；为空时回退到请求体的 model 字段。
func convertRequest(requestBody []byte, upstreamModelName string) ([]byte, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return nil, fmt.Errorf("unmarshal request failed: %w", err)
	}

	// 处理 top_p 参数：DashScope 要求 0 < top_p < 1
	if topPRaw, ok := rawMap["top_p"]; ok {
		var topP float64
		if err := json.Unmarshal(topPRaw, &topP); err == nil {
			if topP >= 1.0 {
				topP = 0.999
			} else if topP <= 0 {
				topP = 0.001
			}
			capped, _ := json.Marshal(topP)
			rawMap["top_p"] = capped
		}
	}

	// 剥离非 Qwen 思考模型的 thinking_budget：thinking_budget 是 Qwen 专属扩展参数，
	// 仅 qwen/qwq 系列支持；模型映射到其他上游（如 deepseek-r1）时透传会导致上游 400。
	// 注意：显式 thinking_budget:0 是合法值（关闭预算），对支持的模型必须保留。
	modelName := upstreamModelName
	if modelName == "" {
		if modelRaw, ok := rawMap["model"]; ok {
			var m string
			if json.Unmarshal(modelRaw, &m) == nil {
				modelName = m
			}
		}
	}
	if !isQwenThinkingModel(modelName) {
		delete(rawMap, "thinking_budget")
	}

	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return converted, nil
}

// isQwenThinkingModel 判断模型是否属于支持 thinking_budget 的 Qwen/Qwq 系列。
// 匹配规则对齐 new-api 的 IsQwenThinkingBudgetModel：忽略大小写与前后空白，
// 模型名以 qwen/qwq 开头，或含 "/qwen"/"/qwq"（如 HF 路径 Qwen/Qwen3-...）。
func isQwenThinkingModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "qwen") ||
		strings.Contains(normalized, "/qwen") ||
		strings.HasPrefix(normalized, "qwq") ||
		strings.Contains(normalized, "/qwq")
}
