package override

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func opSet(data []byte, path string, value any, keepOrigin bool) ([]byte, error) {
	if keepOrigin {
		if gjson.GetBytes(data, path).Exists() {
			return data, nil
		}
	}
	valJSON := valueToJSON(value)
	result, err := sjson.SetRawBytes(data, path, valJSON)
	if err != nil {
		return data, nil // 路径不存在时静默忽略
	}
	return result, nil
}

func opDelete(data []byte, path string) ([]byte, error) {
	result, err := sjson.DeleteBytes(data, path)
	if err != nil {
		return data, nil
	}
	return result, nil
}

func opMove(data []byte, from, to string) ([]byte, error) {
	val := gjson.GetBytes(data, from)
	if !val.Exists() {
		return data, nil
	}
	result, err := sjson.DeleteBytes(data, from)
	if err != nil {
		return data, nil
	}
	result, err = sjson.SetRawBytes(result, to, []byte(val.Raw))
	if err != nil {
		return data, nil
	}
	return result, nil
}

func opCopy(data []byte, from, to string) ([]byte, error) {
	val := gjson.GetBytes(data, from)
	if !val.Exists() {
		return data, nil
	}
	result, err := sjson.SetRawBytes(data, to, []byte(val.Raw))
	if err != nil {
		return data, nil
	}
	return result, nil
}

func opAppend(data []byte, path string, value any, keepOrigin bool) ([]byte, error) {
	if keepOrigin {
		if gjson.GetBytes(data, path).Exists() {
			return data, nil
		}
	}

	existing := gjson.GetBytes(data, path)
	if !existing.Exists() {
		return opSet(data, path, value, false)
	}

	switch existing.Type {
	case gjson.String:
		newVal := existing.Str + fmt.Sprintf("%v", value)
		return sjson.SetBytes(data, path, newVal)
	case gjson.JSON:
		var arr []any
		if err := json.Unmarshal([]byte(existing.Raw), &arr); err == nil {
			arr = append(arr, value)
			arrJSON, _ := json.Marshal(arr)
			return sjson.SetRawBytes(data, path, arrJSON)
		}
	case gjson.Number:
		return data, nil
	default:
		arrJSON, _ := json.Marshal([]any{existing.Value(), value})
		return sjson.SetRawBytes(data, path, arrJSON)
	}

	return data, nil
}

func opPrepend(data []byte, path string, value any, keepOrigin bool) ([]byte, error) {
	if keepOrigin {
		if gjson.GetBytes(data, path).Exists() {
			return data, nil
		}
	}

	existing := gjson.GetBytes(data, path)
	if !existing.Exists() {
		return opSet(data, path, value, false)
	}

	switch existing.Type {
	case gjson.String:
		newVal := fmt.Sprintf("%v", value) + existing.Str
		return sjson.SetBytes(data, path, newVal)
	case gjson.JSON:
		var arr []any
		if err := json.Unmarshal([]byte(existing.Raw), &arr); err == nil {
			arr = append([]any{value}, arr...)
			arrJSON, _ := json.Marshal(arr)
			return sjson.SetRawBytes(data, path, arrJSON)
		}
	default:
		arrJSON, _ := json.Marshal([]any{value, existing.Value()})
		return sjson.SetRawBytes(data, path, arrJSON)
	}

	return data, nil
}

func opReplace(data []byte, path string, value any) ([]byte, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return data, nil
	}
	oldStr := toString(m["old"])
	newStr := toString(m["new"])
	if oldStr == "" {
		return data, nil
	}

	target := gjson.GetBytes(data, path)
	if !target.Exists() {
		return data, nil
	}

	replaced := replaceAllStr(target.Str, oldStr, newStr)
	return sjson.SetBytes(data, path, replaced)
}

func opRegexReplace(data []byte, path string, value any) ([]byte, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return data, nil
	}
	pattern := toString(m["pattern"])
	replacement := toString(m["replacement"])
	if pattern == "" {
		return data, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return data, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	target := gjson.GetBytes(data, path)
	if !target.Exists() {
		return data, nil
	}

	replaced := re.ReplaceAllString(target.Str, replacement)
	return sjson.SetBytes(data, path, replaced)
}

func opToLower(data []byte, path string) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() {
		return data, nil
	}
	if target.Type == gjson.String {
		return sjson.SetBytes(data, path, strings.ToLower(target.Str))
	}
	return data, nil
}

func opToUpper(data []byte, path string) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() {
		return data, nil
	}
	if target.Type == gjson.String {
		return sjson.SetBytes(data, path, strings.ToUpper(target.Str))
	}
	return data, nil
}

func opTrimPrefix(data []byte, path string, value any) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() || target.Type != gjson.String {
		return data, nil
	}
	prefix := fmt.Sprintf("%v", value)
	result := strings.TrimPrefix(target.Str, prefix)
	return sjson.SetBytes(data, path, result)
}

func opTrimSuffix(data []byte, path string, value any) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() || target.Type != gjson.String {
		return data, nil
	}
	suffix := fmt.Sprintf("%v", value)
	result := strings.TrimSuffix(target.Str, suffix)
	return sjson.SetBytes(data, path, result)
}

func opEnsurePrefix(data []byte, path string, value any) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() || target.Type != gjson.String {
		return data, nil
	}
	prefix := fmt.Sprintf("%v", value)
	result := target.Str
	if !strings.HasPrefix(result, prefix) {
		result = prefix + result
	}
	return sjson.SetBytes(data, path, result)
}

func opEnsureSuffix(data []byte, path string, value any) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() || target.Type != gjson.String {
		return data, nil
	}
	suffix := fmt.Sprintf("%v", value)
	result := target.Str
	if !strings.HasSuffix(result, suffix) {
		result = result + suffix
	}
	return sjson.SetBytes(data, path, result)
}

func opTrimSpace(data []byte, path string) ([]byte, error) {
	target := gjson.GetBytes(data, path)
	if !target.Exists() || target.Type != gjson.String {
		return data, nil
	}
	return sjson.SetBytes(data, path, strings.TrimSpace(target.Str))
}
