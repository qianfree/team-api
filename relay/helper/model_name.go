package helper

import "bytes"

// ReplaceModelName 替换 JSON 响应中 "model":"xxx" 的值为指定模型名。
// 使用字节级操作避免 JSON 反序列化再序列化的开销。
func ReplaceModelName(body []byte, modelName string) []byte {
	fieldPrefix := []byte(`"model":"`)
	// result 已包含从原文复制的 `"model":"` 前缀，替换值只需模型名 + 闭合引号，
	// 不能再带前缀（否则前缀重复写出，产生非法 JSON）
	replacement := make([]byte, 0, len(modelName)+1)
	replacement = append(replacement, modelName...)
	replacement = append(replacement, '"')

	result := make([]byte, 0, len(body))
	i := 0
	for i < len(body) {
		idx := bytes.Index(body[i:], fieldPrefix)
		if idx == -1 {
			result = append(result, body[i:]...)
			break
		}
		result = append(result, body[i:i+idx+len(fieldPrefix)]...)
		i += idx + len(fieldPrefix)

		endQuote := bytes.IndexByte(body[i:], '"')
		if endQuote == -1 {
			result = append(result, body[i:]...)
			break
		}
		i += endQuote + 1
		result = append(result, replacement...)
	}
	return result
}
