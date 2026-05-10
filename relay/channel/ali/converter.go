package ali

import (
	"encoding/json"
	"fmt"
)

// convertRequest 转换 OpenAI 格式请求以适配 DashScope。
// 主要处理 top_p 参数的合法范围限制：DashScope 要求 top_p 在 (0, 1) 开区间内。
func convertRequest(requestBody []byte) ([]byte, error) {
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

	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return converted, nil
}
