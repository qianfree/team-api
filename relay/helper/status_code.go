package helper

import (
	"encoding/json"
	"strconv"

	"github.com/qianfree/team-api/relay/constant"
)

// RemapStatusCode 根据渠道配置重映射错误响应的 HTTP 状态码
// mappingJSON 格式如 {"429": 500, "403": 500}
func RemapStatusCode(err error, mappingJSON string) error {
	if err == nil || mappingJSON == "" || mappingJSON == "{}" {
		return err
	}

	relayErr, ok := err.(*constant.RelayError)
	if !ok || relayErr.StatusCode == 0 || relayErr.StatusCode == 200 {
		return err
	}

	mapping := make(map[string]any)
	if err := json.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
		return relayErr // mapping 解析失败，返回原始错误
	}

	codeStr := strconv.Itoa(relayErr.StatusCode)
	if mapped, exists := mapping[codeStr]; exists {
		newCode := parseMappingValue(mapped)
		if newCode > 0 {
			relayErr.StatusCode = newCode
		}
	}

	return relayErr
}

func parseMappingValue(v any) int {
	switch val := v.(type) {
	case string:
		n, _ := strconv.Atoi(val)
		return n
	case float64:
		return int(val)
	case int:
		return val
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	default:
		return 0
	}
}
