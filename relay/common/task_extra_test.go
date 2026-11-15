package common

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestParseTaskExtra(t *testing.T) {
	for _, raw := range []string{"", "null", "{}", "  "} {
		extra, terr := ParseTaskExtra(json.RawMessage(raw))
		if terr != nil || extra != nil {
			t.Fatalf("%q 应视为未传: extra=%v terr=%v", raw, extra, terr)
		}
	}
	for _, raw := range []string{`[1]`, `"x"`, `1`, `{bad`} {
		if _, terr := ParseTaskExtra(json.RawMessage(raw)); terr == nil || terr.ErrCode != ErrCodeInvalidExtra {
			t.Fatalf("%q 应拒绝: %+v", raw, terr)
		}
	}
	extra, terr := ParseTaskExtra(json.RawMessage(`{"seed":9007199254740993}`))
	if terr != nil {
		t.Fatal(terr)
	}
	if n, ok := extra["seed"].(json.Number); !ok || n.String() != "9007199254740993" {
		t.Fatalf("大整数须保真: %#v", extra["seed"])
	}
}

func TestValidateTaskExtra(t *testing.T) {
	protected := []string{"duration", "parameters.resolution"}
	cases := []struct {
		name string
		raw  string
		hit  bool
	}{
		{"无冲突", `{"camera_control":{"type":"simple"},"parameters":{"shot_type":"multi"}}`, false},
		{"全局 model", `{"model":"x"}`, true},
		{"全局 callback_url", `{"callback_url":"https://evil"}`, true},
		{"顶层保护键", `{"duration":15}`, true},
		{"嵌套保护键", `{"parameters":{"resolution":"1080P"}}`, true},
		{"前缀非对象整体替换", `{"parameters":"x"}`, true},
		{"前缀为 null 删除子树", `{"parameters":null}`, true},
		{"保护键置 null 删除", `{"duration":null}`, true},
	}
	for _, c := range cases {
		extra, terr := ParseTaskExtra(json.RawMessage(c.raw))
		if terr != nil {
			t.Fatalf("%s: %v", c.name, terr)
		}
		terr = ValidateTaskExtra(extra, protected)
		if c.hit != (terr != nil) {
			t.Fatalf("%s: hit=%v got %+v", c.name, c.hit, terr)
		}
		if terr != nil && terr.ErrCode != ErrCodeExtraFieldProtected {
			t.Fatalf("%s: 错误码 %s", c.name, terr.ErrCode)
		}
	}
}

func TestMergeTaskExtra(t *testing.T) {
	upstream := `{"model":"wan3.0","input":{"prompt":"p"},"parameters":{"duration":5,"ratio":"16:9"},"tags":["a"]}`
	extra, _ := ParseTaskExtra(json.RawMessage(`{"parameters":{"shot_type":"multi","ratio":null},"tags":["b","c"],"new_flag":true,"seed":9007199254740993}`))

	r, err := MergeTaskExtra(strings.NewReader(upstream), extra)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	got := string(raw)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	params := out["parameters"].(map[string]any)
	if params["duration"].(float64) != 5 || params["shot_type"] != "multi" {
		t.Fatalf("对象须深合并保留原键: %s", got)
	}
	if _, ok := params["ratio"]; ok {
		t.Fatalf("null 须删除键: %s", got)
	}
	if tags := out["tags"].([]any); len(tags) != 2 || tags[0] != "b" {
		t.Fatalf("数组须整体替换: %s", got)
	}
	if out["new_flag"] != true || out["input"].(map[string]any)["prompt"] != "p" {
		t.Fatalf("新增键/未触碰键异常: %s", got)
	}
	if !strings.Contains(got, `"seed":9007199254740993`) {
		t.Fatalf("大整数须原样输出: %s", got)
	}

	if _, err := MergeTaskExtra(strings.NewReader(`not-json`), extra); err == nil {
		t.Fatal("非 JSON 对象上游体须报错")
	}
	r, err = MergeTaskExtra(strings.NewReader(upstream), nil)
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := io.ReadAll(r); string(raw) != upstream {
		t.Fatalf("无 extra 须原样返回: %s", raw)
	}
}
