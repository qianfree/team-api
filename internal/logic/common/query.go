package common

import "strings"

// EscapeLikePattern 转义 PostgreSQL LIKE 模式中的特殊字符（\、%、_），
// 使其作为字面量参与匹配。配合 WhereLike(col, "%"+EscapeLikePattern(s)+"%") 使用。
//
// 背景：用户输入直接拼到 LIKE 模式时，输入中的 % 与 _ 会被当作通配符，
// 既影响搜索结果，也可能被构造用于探测数据。统一用反斜杠转义（PostgreSQL 默认 ESCAPE 字符）。
func EscapeLikePattern(s string) string {
	if s == "" {
		return ""
	}
	// 反斜杠必须先转义，否则会破坏后续转义序列
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
