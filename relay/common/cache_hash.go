package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// excludeCacheFields 是应从缓存哈希中排除的请求体字段，
// 因为它们随请求变化但不影响模型输出。
var excludeCacheFields = map[string]bool{
	"stream":                true,
	"seed":                  true,
	"user":                  true,
	"request_id":            true,
	"top_logprobs":          true,
	"logprobs":              true,
	"n":                     true,
	"frequency_penalty":     false, // 保留——影响输出
	"presence_penalty":      false, // 保留——影响输出
	"temperature":           false, // 保留——影响输出
	"max_tokens":            false, // 保留——影响输出
	"max_completion_tokens": false,
}

// ComputeCacheHash 基于规范化后的请求体生成确定性 SHA-256 哈希。
// 它移除非确定性字段（stream、seed、user、request_id、top_logprobs、logprobs、n），
// 并对 JSON 键排序以保证哈希一致。
func ComputeCacheHash(body []byte, modelName string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		// 兜底：直接哈希原始请求体 + 模型名
		h := sha256.New()
		h.Write([]byte(modelName))
		h.Write(body)
		return hex.EncodeToString(h.Sum(nil))
	}

	// 移除需排除的字段
	for field := range excludeCacheFields {
		delete(raw, field)
	}

	// 排序键以保证确定性顺序
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 用排序后的键重建 JSON
	var buf strings.Builder
	for _, k := range keys {
		if buf.Len() > 0 {
			buf.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k)
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(raw[k])
	}

	h := sha256.New()
	h.Write([]byte(modelName))
	h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil))
}
