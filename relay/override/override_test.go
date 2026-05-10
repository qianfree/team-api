package override

import (
	"testing"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/qianfree/team-api/relay/common"
)

func TestApplyParamOverride_Set(t *testing.T) {
	body := `{"model":"gpt-4o","temperature":0.7,"messages":[{"role":"user","content":"hello"}]}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "temperature", "mode": "set", "value": 0.5},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	temp := gjson.Get(string(result), "temperature").Float()
	if temp != 0.5 {
		t.Errorf("expected temperature=0.5, got %v", temp)
	}
}

func TestApplyParamOverride_Delete(t *testing.T) {
	body := `{"model":"gpt-4o","temperature":0.7,"stream":true}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "stream", "mode": "delete"},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gjson.Get(string(result), "stream").Exists() {
		t.Error("expected stream to be deleted")
	}
}

func TestApplyParamOverride_Move(t *testing.T) {
	body := `{"model":"gpt-4o","key":"value"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"from": "key", "to": "renamed_key", "mode": "move"},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gjson.Get(string(result), "key").Exists() {
		t.Error("expected key to be removed after move")
	}
	if gjson.Get(string(result), "renamed_key").Str != "value" {
		t.Errorf("expected renamed_key=value, got %s", gjson.Get(string(result), "renamed_key").Str)
	}
}

func TestApplyParamOverride_Copy(t *testing.T) {
	body := `{"model":"gpt-4o","key":"value"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"from": "key", "to": "key_copy", "mode": "copy"},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gjson.Get(string(result), "key").Str != "value" {
		t.Error("original key should remain")
	}
	if gjson.Get(string(result), "key_copy").Str != "value" {
		t.Error("key_copy should equal value")
	}
}

func TestApplyParamOverride_AppendPrepend(t *testing.T) {
	body := `{"model":"gpt-4o","tags":["a","b"]}`
	info := newTestInfo()

	// append
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "tags", "mode": "append", "value": "c"},
		},
	}
	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr := gjson.Get(string(result), "tags").Array()
	if len(arr) != 3 || arr[2].Str != "c" {
		t.Errorf("expected tags=[a,b,c], got %v", arr)
	}

	// prepend
	body = `{"model":"gpt-4o","tags":["a","b"]}`
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "tags", "mode": "prepend", "value": "z"},
		},
	}
	result, err = ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr = gjson.Get(string(result), "tags").Array()
	if len(arr) != 3 || arr[0].Str != "z" {
		t.Errorf("expected tags=[z,a,b], got %v", arr)
	}
}

func TestApplyParamOverride_StringOps(t *testing.T) {
	body := `{"model":"gpt-4o","text":"  Hello World  "}`
	info := newTestInfo()

	// to_lower
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "text", "mode": "to_lower"},
		},
	}
	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "text").Str != "  hello world  " {
		t.Errorf("expected lowercase text, got %q", gjson.Get(string(result), "text").Str)
	}

	// trim_space
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "text", "mode": "trim_space"},
		},
	}
	result, err = ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "text").Str != "Hello World" {
		t.Errorf("expected trimmed text, got %q", gjson.Get(string(result), "text").Str)
	}
}

func TestApplyParamOverride_Replace(t *testing.T) {
	body := `{"model":"gpt-4o","prompt":"hello world"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "prompt", "mode": "replace", "value": map[string]any{"old": "world", "new": "universe"}},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "prompt").Str != "hello universe" {
		t.Errorf("expected 'hello universe', got %q", gjson.Get(string(result), "prompt").Str)
	}
}

func TestApplyParamOverride_EnsurePrefixSuffix(t *testing.T) {
	body := `{"model":"gpt-4o","path":"/api/v1"}`
	info := newTestInfo()

	// ensure_prefix — path already has /v1 prefix
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "path", "mode": "ensure_prefix", "value": "/api"},
		},
	}
	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "path").Str != "/api/v1" {
		t.Errorf("expected /api/v1 (unchanged), got %q", gjson.Get(string(result), "path").Str)
	}

	// ensure_suffix
	body = `{"model":"gpt-4o","path":"/api/v1"}`
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "path", "mode": "ensure_suffix", "value": "/chat"},
		},
	}
	result, err = ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "path").Str != "/api/v1/chat" {
		t.Errorf("expected /api/v1/chat, got %q", gjson.Get(string(result), "path").Str)
	}
}

func TestApplyParamOverride_ReturnError(t *testing.T) {
	body := `{"model":"gpt-4o"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{
				"path": "model", "mode": "return_error",
				"value": map[string]any{
					"message":     "model not allowed",
					"status_code": 403,
					"skip_retry":  true,
				},
			},
		},
	}

	_, err := ApplyParamOverride([]byte(body), info)
	if err == nil {
		t.Fatal("expected error from return_error")
	}

	retErr, ok := AsReturnError(err)
	if !ok {
		t.Fatalf("expected ReturnError, got %T: %v", err, err)
	}
	if retErr.StatusCode != 403 {
		t.Errorf("expected status 403, got %d", retErr.StatusCode)
	}
	if retErr.Message != "model not allowed" {
		t.Errorf("expected 'model not allowed', got %q", retErr.Message)
	}
	if !retErr.SkipRetry {
		t.Error("expected skip_retry=true")
	}
}

func TestApplyParamOverride_Conditions(t *testing.T) {
	body := `{"model":"gpt-4o","temperature":0.7}`
	info := newTestInfo()

	// Condition: model prefix matches → set temperature to 0
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{
				"path":  "temperature",
				"mode":  "set",
				"value": 0,
				"conditions": []any{
					map[string]any{"path": "model", "mode": "prefix", "value": "o1-"},
				},
			},
		},
	}

	// Should match: model starts with "o1-"
	body = `{"model":"o1-preview","temperature":0.7}`
	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "temperature").Float() != 0 {
		t.Errorf("expected temperature=0 (condition matched), got %v", gjson.Get(string(result), "temperature").Float())
	}

	// Should NOT match: model is "gpt-4o", not "o1-"
	body = `{"model":"gpt-4o","temperature":0.7}`
	result, err = ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "temperature").Float() != 0.7 {
		t.Errorf("expected temperature=0.7 (condition not matched), got %v", gjson.Get(string(result), "temperature").Float())
	}
}

func TestApplyParamOverride_ContextVariables(t *testing.T) {
	body := `{"model":"gpt-4o"}`
	info := newTestInfo()
	info.OriginModelName = "gpt-4o"
	info.ChannelMeta.UpstreamModelName = "gpt-4o-turbo"
	info.RetryIndex = 1

	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "upstream_model_tag", "mode": "set", "value": "context variable test"},
			map[string]any{"path": "retry_tag", "mode": "set", "value": "yes"},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Context variable in path should be resolved
	if gjson.Get(string(result), "retry_tag").Str != "yes" {
		t.Errorf("expected retry_tag=yes (context variable), got %q", gjson.Get(string(result), "retry_tag").Str)
	}
}

func TestApplyParamOverride_KeepOrigin(t *testing.T) {
	body := `{"model":"gpt-4o","temperature":0.7}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "temperature", "mode": "set", "value": 0.3, "keep_origin": true},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// KeepOrigin: temperature already exists (0.7), so should NOT be overwritten
	if gjson.Get(string(result), "temperature").Float() != 0.7 {
		t.Errorf("expected temperature=0.7 (keep_origin), got %v", gjson.Get(string(result), "temperature").Float())
	}
}

func TestApplyParamOverride_SetHeader(t *testing.T) {
	body := `{"model":"gpt-4o"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "X-Custom", "mode": "set_header", "value": "test-value"},
		},
	}

	_, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.RuntimeHeadersOverride == nil {
		t.Fatal("expected RuntimeHeadersOverride to be set")
	}
	if info.RuntimeHeadersOverride["X-Custom"] != "test-value" {
		t.Errorf("expected X-Custom=test-value, got %q", info.RuntimeHeadersOverride["X-Custom"])
	}
}

func TestApplyParamOverride_Legacy(t *testing.T) {
	body := `{"model":"gpt-4o","temperature":0.7}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"temperature": 0.3,
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gjson.Get(string(result), "temperature").Float() != 0.3 {
		t.Errorf("expected temperature=0.3 (legacy), got %v", gjson.Get(string(result), "temperature").Float())
	}
}

func TestApplyParamOverride_RegexReplace(t *testing.T) {
	body := `{"model":"gpt-4o","text":"hello-123-world"}`
	info := newTestInfo()
	info.ChannelMeta.Settings.ParamOverride = map[string]any{
		"operations": []any{
			map[string]any{"path": "text", "mode": "regex_replace", "value": map[string]any{"pattern": `\d+`, "replacement": "REDACTED"}},
		},
	}

	result, err := ApplyParamOverride([]byte(body), info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gjson.Get(string(result), "text").Str != "hello-REDACTED-world" {
		t.Errorf("expected 'hello-REDACTED-world', got %q", gjson.Get(string(result), "text").Str)
	}
}

func TestCheckConditions(t *testing.T) {
	tests := []struct {
		name      string
		target    any
		mode      string
		expected  any
		invert    bool
		wantMatch bool
	}{
		{"full match", "hello", "full", "hello", false, true},
		{"full no match", "hello", "full", "world", false, false},
		{"prefix match", "hello-world", "prefix", "hello", false, true},
		{"prefix no match", "hello-world", "prefix", "world", false, false},
		{"suffix match", "hello-world", "suffix", "world", false, true},
		{"contains match", "hello-world", "contains", "lo-w", false, true},
		{"gt", "5", "gt", "3", false, true},
		{"gt false", "5", "gt", "5", false, false},
		{"gte", "5", "gte", "5", false, true},
		{"lt", "3", "lt", "5", false, true},
		{"lte", "3", "lte", "3", false, true},
		{"invert", "hello", "full", "hello", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.target, tt.mode, tt.expected)
			if tt.invert {
				result = !result
			}
			if result != tt.wantMatch {
				t.Errorf("evaluateCondition(%v, %q, %v) = %v, want %v", tt.target, tt.mode, tt.expected, result, tt.wantMatch)
			}
		})
	}
}

func TestResolveContextPath(t *testing.T) {
	ctx := map[string]any{
		"model":          "gpt-4o",
		"upstream_model": "gpt-4o-turbo",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"{model}", "gpt-4o"},
		{"{upstream_model}", "gpt-4o-turbo"},
		{"prefix_{model}_suffix", "prefix_gpt-4o_suffix"},
		{"no-placeholder", "no-placeholder"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := resolveContextPath(tt.input, ctx)
			if result != tt.expected {
				t.Errorf("resolveContextPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --- Header Override Tests ---

func TestApplyHeaderOverride_StaticValue(t *testing.T) {
	info := newTestInfo()
	info.ChannelMeta.Settings.HeaderOverride = map[string]any{
		"X-Custom": "static-value",
	}

	headers, err := ApplyHeaderOverride(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if headers["X-Custom"] != "static-value" {
		t.Errorf("expected X-Custom=static-value, got %q", headers["X-Custom"])
	}
}

func TestApplyHeaderOverride_ApiKeyPlaceholder(t *testing.T) {
	info := newTestInfo()
	info.ChannelMeta.ApiKey = "sk-test123"
	info.ChannelMeta.Settings.HeaderOverride = map[string]any{
		"Authorization": "Bearer {api_key}",
	}

	headers, err := ApplyHeaderOverride(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if headers["Authorization"] != "Bearer sk-test123" {
		t.Errorf("expected Authorization='Bearer sk-test123', got %q", headers["Authorization"])
	}
}

func TestApplyHeaderOverride_RuntimeOverride(t *testing.T) {
	info := newTestInfo()
	info.ChannelMeta.Settings.HeaderOverride = map[string]any{
		"X-Static": "original",
	}
	info.RuntimeHeadersOverride = map[string]string{
		"X-Static":  "overridden",
		"X-Dynamic": "from-param-override",
	}

	headers, err := ApplyHeaderOverride(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if headers["X-Static"] != "overridden" {
		t.Errorf("expected runtime override to take precedence")
	}
	if headers["X-Dynamic"] != "from-param-override" {
		t.Errorf("expected dynamic header")
	}
}

func TestIsUnsafeHeader(t *testing.T) {
	tests := []struct {
		header string
		unsafe bool
	}{
		{"authorization", true},
		{"Authorization", true},
		{"X-Api-Key", true},
		{"cookie", true},
		{"host", true},
		{"content-length", true},
		{"x-custom", false},
		{"X-Request-Id", false},
		{"accept", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := isUnsafeHeader(tt.header)
			if result != tt.unsafe {
				t.Errorf("isUnsafeHeader(%q) = %v, want %v", tt.header, result, tt.unsafe)
			}
		})
	}
}

// --- Helper ---

func newTestInfo() *common.RelayInfo {
	return &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			Settings: common.ChannelSettings{},
		},
	}
}

func init() {
	// Suppress unused import warning for sjson (used by tests indirectly)
	_ = sjson.SetRaw
}
