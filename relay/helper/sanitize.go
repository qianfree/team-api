package helper

import (
	"encoding/json"

	"github.com/qianfree/team-api/relay/common"
)

// SanitizeFields 移除请求体中可能产生额外费用或隐私风险的字段
// 默认移除：service_tier, inference_geo, speed, safety_identifier
// 默认保留：store（通过 DisableStore=true 移除）
func SanitizeFields(body []byte, settings common.ChannelSettings) []byte {
	if settings.PassThroughBodyEnabled {
		return body
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	changed := false

	if !settings.AllowServiceTier {
		if _, ok := data["service_tier"]; ok {
			delete(data, "service_tier")
			changed = true
		}
	}

	if !settings.AllowInferenceGeo {
		if _, ok := data["inference_geo"]; ok {
			delete(data, "inference_geo")
			changed = true
		}
	}

	if !settings.AllowSpeed {
		if _, ok := data["speed"]; ok {
			delete(data, "speed")
			changed = true
		}
	}

	if settings.DisableStore {
		if _, ok := data["store"]; ok {
			delete(data, "store")
			changed = true
		}
	}

	if !settings.AllowSafetyIdentifier {
		if _, ok := data["safety_identifier"]; ok {
			delete(data, "safety_identifier")
			changed = true
		}
	}

	if !changed {
		return body
	}

	result, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return result
}

// StripStreamField 移除请求体中的 "stream" 字段
// Gemini 原生 API 通过 URL 路径（:streamGenerateContent）控制流式，不识别 body 中的 "stream" 字段
func StripStreamField(body []byte) []byte {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}
	if _, ok := data["stream"]; !ok {
		return body
	}
	delete(data, "stream")
	result, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return result
}
