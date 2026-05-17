package ali

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// convertToDashScopeRequest 将 OpenAI Images 格式转换为 DashScope 原生格式
// OpenAI: {model, prompt, size, n, style, ...}
// DashScope: {model, input: {prompt, negative_prompt}, parameters: {size, n, style, ...}}
func convertToDashScopeRequest(body []byte, info *common.RelayInfo) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}

	req := dashScopeRequest{
		Input:      dashScopeInput{},
		Parameters: make(map[string]any),
	}

	// model
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		req.Model = info.ChannelMeta.UpstreamModelName
	} else if v, ok := raw["model"]; ok {
		_ = json.Unmarshal(v, &req.Model)
	}

	// prompt → input.prompt
	if v, ok := raw["prompt"]; ok {
		_ = json.Unmarshal(v, &req.Input.Prompt)
	}

	// negative_prompt → input.negative_prompt
	if v, ok := raw["negative_prompt"]; ok {
		_ = json.Unmarshal(v, &req.Input.NegativePrompt)
	}

	// size: 1024x1024 → 1024*1024
	if v, ok := raw["size"]; ok {
		var size string
		if err := json.Unmarshal(v, &size); err == nil && size != "" {
			req.Parameters["size"] = strings.ReplaceAll(size, "x", "*")
		}
	}

	// n
	if v, ok := raw["n"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err == nil && n > 0 {
			req.Parameters["n"] = n
		}
	}

	// seed
	if v, ok := raw["seed"]; ok {
		var seed int
		if err := json.Unmarshal(v, &seed); err == nil {
			req.Parameters["seed"] = seed
		}
	}

	// style
	if v, ok := raw["style"]; ok {
		var style string
		if err := json.Unmarshal(v, &style); err == nil && style != "" {
			req.Parameters["style"] = style
		}
	}

	// ref_strength
	if v, ok := raw["ref_strength"]; ok {
		var strength float64
		if err := json.Unmarshal(v, &strength); err == nil {
			req.Parameters["ref_strength"] = strength
		}
	}

	// ref_img
	if v, ok := raw["ref_img"]; ok {
		var refImg string
		if err := json.Unmarshal(v, &refImg); err == nil && refImg != "" {
			req.Parameters["ref_img"] = refImg
		}
	}

	if len(req.Parameters) == 0 {
		req.Parameters = nil
	}

	result, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope request: %w", err)
	}
	return result, nil
}
