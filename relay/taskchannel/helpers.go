package taskchannel

import (
	"encoding/json"
	"fmt"
)

// ExtractMetadata 从已解析的请求 map 中提取 metadata 子 map。
// 同时删除 "model" 字段防止通过 metadata 覆盖计费模型。
func ExtractMetadata(req map[string]any) map[string]any {
	if req == nil {
		return nil
	}
	meta, ok := req["metadata"].(map[string]any)
	if !ok {
		return nil
	}
	delete(meta, "model")
	return meta
}

// UnmarshalMetadata 将 metadata map 通过 JSON 往返反序列化到目标结构体。
// metadata 为 nil 时直接返回 nil，不修改 target。
func UnmarshalMetadata(metadata map[string]any, target any) error {
	if len(metadata) == 0 {
		return nil
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	if err := json.Unmarshal(metaBytes, target); err != nil {
		return fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	return nil
}

// DefaultString 返回 val（非空时），否则返回 fallback。
func DefaultString(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

// DefaultInt 返回 val（非零时），否则返回 fallback。
func DefaultInt(val, fallback int) int {
	if val == 0 {
		return fallback
	}
	return val
}
