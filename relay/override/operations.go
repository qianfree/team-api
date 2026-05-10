package override

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func opSet(jsonStr, path string, value any, keepOrigin bool) (string, error) {
	if keepOrigin {
		if gjson.Get(jsonStr, path).Exists() {
			return jsonStr, nil
		}
	}
	valJSON := valueToJSON(value)
	result, err := sjson.SetRaw(jsonStr, path, string(valJSON))
	if err != nil {
		return jsonStr, nil // 路径不存在时静默忽略
	}
	return result, nil
}

func opDelete(jsonStr, path string) (string, error) {
	result, err := sjson.Delete(jsonStr, path)
	if err != nil {
		return jsonStr, nil
	}
	return result, nil
}

func opMove(jsonStr, from, to string) (string, error) {
	val := gjson.Get(jsonStr, from)
	if !val.Exists() {
		return jsonStr, nil
	}
	result, err := sjson.Delete(jsonStr, from)
	if err != nil {
		return jsonStr, nil
	}
	result, err = sjson.SetRaw(result, to, val.Raw)
	if err != nil {
		return jsonStr, nil
	}
	return result, nil
}

func opCopy(jsonStr, from, to string) (string, error) {
	val := gjson.Get(jsonStr, from)
	if !val.Exists() {
		return jsonStr, nil
	}
	result, err := sjson.SetRaw(jsonStr, to, val.Raw)
	if err != nil {
		return jsonStr, nil
	}
	return result, nil
}

func opAppend(jsonStr, path string, value any, keepOrigin bool) (string, error) {
	if keepOrigin {
		if gjson.Get(jsonStr, path).Exists() {
			return jsonStr, nil
		}
	}

	existing := gjson.Get(jsonStr, path)
	if !existing.Exists() {
		return opSet(jsonStr, path, value, false)
	}

	// 如果目标不存在，直接设置
	switch existing.Type {
	case gjson.String:
		newVal := existing.Str + fmt.Sprintf("%v", value)
		return sjson.Set(jsonStr, path, newVal)
	case gjson.JSON:
		var arr []any
		if err := json.Unmarshal([]byte(existing.Raw), &arr); err == nil {
			arr = append(arr, value)
			arrJSON, _ := json.Marshal(arr)
			return sjson.SetRaw(jsonStr, path, string(arrJSON))
		}
	case gjson.Number:
		// 数字追加无意义，跳过
		return jsonStr, nil
	default:
		// 其他类型，设置为数组 [existing, value]
		arrJSON, _ := json.Marshal([]any{existing.Value(), value})
		return sjson.SetRaw(jsonStr, path, string(arrJSON))
	}

	return jsonStr, nil
}

func opPrepend(jsonStr, path string, value any, keepOrigin bool) (string, error) {
	if keepOrigin {
		if gjson.Get(jsonStr, path).Exists() {
			return jsonStr, nil
		}

	}

	existing := gjson.Get(jsonStr, path)
	if !existing.Exists() {
		return opSet(jsonStr, path, value, false)
	}

	switch existing.Type {
	case gjson.String:
		newVal := fmt.Sprintf("%v", value) + existing.Str
		return sjson.Set(jsonStr, path, newVal)
	case gjson.JSON:
		var arr []any
		if err := json.Unmarshal([]byte(existing.Raw), &arr); err == nil {
			arr = append([]any{value}, arr...)
			arrJSON, _ := json.Marshal(arr)
			return sjson.SetRaw(jsonStr, path, string(arrJSON))
		}
	default:
		arrJSON, _ := json.Marshal([]any{value, existing.Value()})
		return sjson.SetRaw(jsonStr, path, string(arrJSON))
	}

	return jsonStr, nil
}

func opReplace(jsonStr, path string, value any) (string, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return jsonStr, nil
	}
	oldStr := toString(m["old"])
	newStr := toString(m["new"])
	if oldStr == "" {
		return jsonStr, nil
	}

	target := gjson.Get(jsonStr, path)
	if !target.Exists() {
		return jsonStr, nil
	}

	replaced := replaceAllStr(target.Str, oldStr, newStr)
	return sjson.Set(jsonStr, path, replaced)
}

func opRegexReplace(jsonStr, path string, value any) (string, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return jsonStr, nil
	}
	pattern := toString(m["pattern"])
	replacement := toString(m["replacement"])
	if pattern == "" {
		return jsonStr, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return jsonStr, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	target := gjson.Get(jsonStr, path)
	if !target.Exists() {
		return jsonStr, nil
	}

	replaced := re.ReplaceAllString(target.Str, replacement)
	return sjson.Set(jsonStr, path, replaced)
}

func opToLower(jsonStr, path string) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() {
		return jsonStr, nil
	}
	if target.Type == gjson.String {
		return sjson.Set(jsonStr, path, strings.ToLower(target.Str))
	}
	return jsonStr, nil
}

func opToUpper(jsonStr, path string) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() {
		return jsonStr, nil
	}
	if target.Type == gjson.String {
		return sjson.Set(jsonStr, path, strings.ToUpper(target.Str))
	}
	return jsonStr, nil
}

func opTrimPrefix(jsonStr, path string, value any) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() || target.Type != gjson.String {
		return jsonStr, nil
	}
	prefix := fmt.Sprintf("%v", value)
	result := strings.TrimPrefix(target.Str, prefix)
	return sjson.Set(jsonStr, path, result)
}

func opTrimSuffix(jsonStr, path string, value any) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() || target.Type != gjson.String {
		return jsonStr, nil
	}
	suffix := fmt.Sprintf("%v", value)
	result := strings.TrimSuffix(target.Str, suffix)
	return sjson.Set(jsonStr, path, result)
}

func opEnsurePrefix(jsonStr, path string, value any) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() || target.Type != gjson.String {
		return jsonStr, nil
	}
	prefix := fmt.Sprintf("%v", value)
	result := target.Str
	if !strings.HasPrefix(result, prefix) {
		result = prefix + result
	}
	return sjson.Set(jsonStr, path, result)
}

func opEnsureSuffix(jsonStr, path string, value any) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() || target.Type != gjson.String {
		return jsonStr, nil
	}
	suffix := fmt.Sprintf("%v", value)
	result := target.Str
	if !strings.HasSuffix(result, suffix) {
		result = result + suffix
	}
	return sjson.Set(jsonStr, path, result)
}

func opTrimSpace(jsonStr, path string) (string, error) {
	target := gjson.Get(jsonStr, path)
	if !target.Exists() || target.Type != gjson.String {
		return jsonStr, nil
	}
	return sjson.Set(jsonStr, path, strings.TrimSpace(target.Str))
}
