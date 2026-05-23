package override

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/qianfree/team-api/relay/common"
)

// Operation 请求体改写操作
type Operation struct {
	Path       string      `json:"path"`
	Mode       string      `json:"mode"`
	Value      any         `json:"value"`
	KeepOrigin bool        `json:"keep_origin"`
	From       string      `json:"from,omitempty"`
	To         string      `json:"to,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
	Logic      string      `json:"logic,omitempty"` // "AND"/"OR"，默认 OR
}

// Condition 操作执行条件
type Condition struct {
	Path           string `json:"path"`
	Mode           string `json:"mode"` // full/prefix/suffix/contains/gt/gte/lt/lte
	Value          any    `json:"value"`
	Invert         bool   `json:"invert"`
	PassMissingKey bool   `json:"pass_missing_key"`
}

// ReturnError return_error 操作返回的错误
type ReturnError struct {
	Message    string
	StatusCode int
	Code       string
	Type       string
	SkipRetry  bool
}

func (e *ReturnError) Error() string {
	if e == nil || e.Message == "" {
		return "request blocked by param override"
	}
	return e.Message
}

// ApplyParamOverride 对请求体执行改写规则
func ApplyParamOverride(body []byte, info *common.RelayInfo) ([]byte, error) {
	paramOverride := info.ChannelMeta.Settings.ParamOverride
	if len(paramOverride) == 0 {
		return body, nil
	}

	ctx := BuildOverrideContext(info)

	var result []byte
	var err error
	// 尝试解析为 operations 数组格式
	if operations, ok := tryParseOperations(paramOverride); ok {
		result, err = applyOperations(body, operations, ctx)
	} else {
		// 兼容旧版 key-value 格式（直接设置/删除字段）
		result, err = applyLegacy(body, paramOverride, ctx)
	}

	if err != nil {
		return nil, err
	}

	// 将 ctx 中 header_override 同步回 info.RuntimeHeadersOverride
	if h, ok := ctx["header_override"].(map[string]string); ok && len(h) > 0 {
		if info.RuntimeHeadersOverride == nil {
			info.RuntimeHeadersOverride = make(map[string]string)
		}
		for k, v := range h {
			info.RuntimeHeadersOverride[k] = v
		}
	}

	return result, nil
}

// BuildOverrideContext 构建条件求值可用的上下文变量
func BuildOverrideContext(info *common.RelayInfo) map[string]any {
	ctx := map[string]any{
		"model":          info.OriginModelName,
		"original_model": info.OriginModelName,
		"upstream_model": info.ChannelMeta.UpstreamModelName,
		"request_path":   info.RequestURLPath,
		"is_retry":       info.RetryIndex > 0,
		"retry_index":    info.RetryIndex,
	}

	// request_headers 转为小写 map
	if info.RequestHeaders != nil {
		headers := make(map[string]string)
		for k, v := range info.RequestHeaders {
			for _, vv := range v {
				if vv != "" {
					headers[k] = vv
					break
				}
			}
		}
		ctx["request_headers"] = headers
	}

	// 运行时 header 覆盖（来自 set_header/delete_header 操作）
	if info.RuntimeHeadersOverride != nil {
		ctx["header_override"] = info.RuntimeHeadersOverride
	}

	return ctx
}

// tryParseOperations 尝试从 paramOverride 中解析 operations 数组
func tryParseOperations(paramOverride map[string]any) ([]Operation, bool) {
	raw, ok := paramOverride["operations"]
	if !ok {
		return nil, false
	}

	arr, ok := raw.([]any)
	if !ok {
		return nil, false
	}

	var operations []Operation
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		op := Operation{
			Path:       toString(m["path"]),
			Mode:       toString(m["mode"]),
			Value:      m["value"],
			KeepOrigin: toBool(m["keep_origin"]),
			From:       toString(m["from"]),
			To:         toString(m["to"]),
			Logic:      toString(m["logic"]),
		}
		if conds, ok := m["conditions"].([]any); ok {
			for _, c := range conds {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				op.Conditions = append(op.Conditions, Condition{
					Path:           toString(cm["path"]),
					Mode:           toString(cm["mode"]),
					Value:          cm["value"],
					Invert:         toBool(cm["invert"]),
					PassMissingKey: toBool(cm["pass_missing_key"]),
				})
			}
		}
		if op.Mode != "" {
			operations = append(operations, op)
		}
	}

	return operations, len(operations) > 0
}

// applyOperations 执行 operations 数组
func applyOperations(data []byte, operations []Operation, ctx map[string]any) ([]byte, error) {
	for _, op := range operations {
		if !checkConditions(data, ctx, op.Conditions, op.Logic) {
			continue
		}

		result, err := applySingleOperation(data, op, ctx)
		if err != nil {
			return nil, err
		}
		data = result
	}
	return data, nil
}

// applyLegacy 兼容旧版 key-value 格式
func applyLegacy(data []byte, paramOverride map[string]any, ctx map[string]any) ([]byte, error) {
	// 分离 operations 和 legacy key-value
	legacy := make(map[string]any)
	for k, v := range paramOverride {
		if k == "operations" {
			continue
		}
		legacy[k] = v
	}

	if len(legacy) == 0 {
		return data, nil
	}

	for path, value := range legacy {
		result, err := sjson.SetRawBytes(data, path, valueToJSON(value))
		if err != nil {
			// 路径不存在时忽略
			continue
		}
		data = result
	}
	return data, nil
}

// applySingleOperation 执行单个操作
func applySingleOperation(data []byte, op Operation, ctx map[string]any) ([]byte, error) {
	// 解析上下文变量占位符
	path := resolveContextPath(op.Path, ctx)
	from := resolveContextPath(op.From, ctx)
	to := resolveContextPath(op.To, ctx)

	switch op.Mode {
	case "set":
		return opSet(data, path, op.Value, op.KeepOrigin)
	case "delete":
		return opDelete(data, path)
	case "move":
		return opMove(data, from, to)
	case "copy":
		return opCopy(data, from, to)
	case "append":
		return opAppend(data, path, op.Value, op.KeepOrigin)
	case "prepend":
		return opPrepend(data, path, op.Value, op.KeepOrigin)
	case "replace":
		return opReplace(data, path, op.Value)
	case "regex_replace":
		return opRegexReplace(data, path, op.Value)
	case "to_lower":
		return opToLower(data, path)
	case "to_upper":
		return opToUpper(data, path)
	case "trim_prefix":
		return opTrimPrefix(data, path, op.Value)
	case "trim_suffix":
		return opTrimSuffix(data, path, op.Value)
	case "ensure_prefix":
		return opEnsurePrefix(data, path, op.Value)
	case "ensure_suffix":
		return opEnsureSuffix(data, path, op.Value)
	case "trim_space":
		return opTrimSpace(data, path)
	case "return_error":
		return nil, opReturnError(op.Value)
	case "set_header":
		opSetHeader(op.Path, op.Value, ctx)
		return data, nil
	case "delete_header":
		opDeleteHeader(op.Path, ctx)
		return data, nil
	case "copy_header":
		opCopyHeader(op.From, op.To, ctx)
		return data, nil
	case "move_header":
		opMoveHeader(op.From, op.To, ctx)
		return data, nil
	case "pass_headers":
		opPassHeaders(op.Value, ctx)
		return data, nil
	default:
		return data, fmt.Errorf("unknown override mode: %s", op.Mode)
	}
}

// checkConditions 检查所有条件是否满足
func checkConditions(data []byte, ctx map[string]any, conditions []Condition, logic string) bool {
	if len(conditions) == 0 {
		return true
	}

	isAnd := logic == "AND"

	for _, cond := range conditions {
		condPath := resolveContextPath(cond.Path, ctx)
		var target any
		var found bool

		// 先查上下文变量
		if val, ok := ctx[condPath]; ok {
			strVal := toString(val)
			if strVal != "" {
				target = val
				found = true
			}
		}
		if !found {
			// 再查请求体 JSON
			res := gjson.GetBytes(data, condPath)
			if res.Exists() {
				target = res.Value()
				found = true
			}
		}

		if !found {
			if cond.PassMissingKey {
				continue
			}
			if isAnd {
				return false
			}
			continue
		}

		matched := evaluateCondition(target, cond.Mode, cond.Value)
		if cond.Invert {
			matched = !matched
		}

		if isAnd && !matched {
			return false
		}
		if !isAnd && matched {
			return true
		}
	}

	return isAnd // AND: all matched; OR: none matched
}

// evaluateCondition 评估单个条件
func evaluateCondition(target any, mode string, expected any) bool {
	if mode == "" {
		mode = "full"
	}

	targetStr := fmt.Sprintf("%v", target)
	expectedStr := fmt.Sprintf("%v", expected)

	switch mode {
	case "full":
		return targetStr == expectedStr
	case "prefix":
		return len(targetStr) >= len(expectedStr) && targetStr[:len(expectedStr)] == expectedStr
	case "suffix":
		if len(expectedStr) > len(targetStr) {
			return false
		}
		return targetStr[len(targetStr)-len(expectedStr):] == expectedStr
	case "contains":
		return containsStr(targetStr, expectedStr)
	case "gt":
		return compareNumeric(targetStr, expectedStr) > 0
	case "gte":
		return compareNumeric(targetStr, expectedStr) >= 0
	case "lt":
		return compareNumeric(targetStr, expectedStr) < 0
	case "lte":
		return compareNumeric(targetStr, expectedStr) <= 0
	default:
		return targetStr == expectedStr
	}
}

// resolveContextPath 解析上下文变量占位符（如 {model}, {upstream_model}）
func resolveContextPath(path string, ctx map[string]any) string {
	if path == "" {
		return path
	}
	// 快速检查：如果没有 { 就不需要处理
	if len(path) < 3 {
		return path
	}

	result := path
	for k, v := range ctx {
		placeholder := "{" + k + "}"
		if containsStr(result, placeholder) {
			val := fmt.Sprintf("%v", v)
			result = replaceAllStr(result, placeholder, val)
		}
	}
	return result
}

// Header 操作函数（修改 RelayInfo.RuntimeHeadersOverride）

func opSetHeader(key string, value any, ctx map[string]any) {
	headers := getRuntimeHeaders(ctx)
	headers[key] = fmt.Sprintf("%v", value)
}

func opDeleteHeader(key string, ctx map[string]any) {
	headers := getRuntimeHeaders(ctx)
	delete(headers, key)
}

func opCopyHeader(from, to string, ctx map[string]any) {
	headers := getRuntimeHeaders(ctx)
	if v, ok := headers[from]; ok {
		headers[to] = v
	}
}

func opMoveHeader(from, to string, ctx map[string]any) {
	headers := getRuntimeHeaders(ctx)
	if v, ok := headers[from]; ok {
		headers[to] = v
		delete(headers, from)
	}
}

func opPassHeaders(value any, ctx map[string]any) {
	headers := getRuntimeHeaders(ctx)
	// value 可以是 true 或者是具体规则
	if requestHeaders, ok := ctx["request_headers"].(map[string]string); ok {
		for k, v := range requestHeaders {
			if isUnsafeHeader(k) {
				continue
			}
			headers[k] = v
		}
	}
}

func getRuntimeHeaders(ctx map[string]any) map[string]string {
	h, ok := ctx["header_override"].(map[string]string)
	if !ok || h == nil {
		h = make(map[string]string)
		ctx["header_override"] = h
	}
	return h
}

// --- 辅助函数 ---

func toString(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toBool(v any) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	if ok {
		return b
	}
	return false
}

func valueToJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || containsStrImpl(s, substr))
}

func containsStrImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func replaceAllStr(s, old, new string) string {
	result := make([]byte, 0, len(s))
	for {
		idx := indexOfStr(s, old)
		if idx == -1 {
			result = append(result, s...)
			break
		}
		result = append(result, s[:idx]...)
		result = append(result, new...)
		s = s[idx+len(old):]
	}
	return string(result)
}

func indexOfStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func compareNumeric(a, b string) int {
	af, aok := parseFloat(a)
	bf, bok := parseFloat(b)
	if !aok || !bok {
		// 退化为字符串比较
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

func parseFloat(s string) (float64, bool) {
	var f float64
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			f = f*10 + float64(c-'0')
			n++
		} else if c == '.' && n > 0 {
			dec := 1.0
			for i := indexOfStr(s, ".") + 1; i < len(s); i++ {
				cc := s[i]
				if cc >= '0' && cc <= '9' {
					f += float64(cc-'0') / dec
					dec *= 10
				} else {
					return 0, false
				}
			}
			return f, true
		} else {
			return 0, false
		}
	}
	return f, true
}

func opReturnError(value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		return &ReturnError{Message: "request blocked", StatusCode: 400}
	}
	return &ReturnError{
		Message:    toString(m["message"]),
		StatusCode: parseIntDefault(toString(m["status_code"]), 400),
		Code:       toString(m["code"]),
		Type:       toString(m["type"]),
		SkipRetry:  toBool(m["skip_retry"]),
	}
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return def
		}
	}
	return n
}

func isUnsafeHeader(key string) bool {
	lower := toLowerStr(key)
	switch lower {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade",
		"cookie", "host", "content-length", "accept-encoding",
		"authorization", "x-api-key", "x-goog-api-key":
		return true
	}
	return false
}

func toLowerStr(s string) string {
	result := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// AsReturnError 检查错误是否为 ReturnError
func AsReturnError(err error) (*ReturnError, bool) {
	if err == nil {
		return nil, false
	}
	var target *ReturnError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
