package contract

import (
	"encoding/json"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		APIVersion: 1,
		Key:        "test-plugin",
		Name:       "Test",
		Version:    "1.0.0",
		Models:     []string{"test_model"},
		Routes:     []Route{{Method: "POST", Path: "/submit", Type: RouteTypeSubmit}},
	}
}

func TestUsageSchemaValidation(t *testing.T) {
	m := validManifest()
	m.UsageSchema = []UsageField{
		{Key: "clips", Type: "number", Unit: "count"},
		{Key: "duration", Type: "number", Unit: "second"},
		{Key: "tier", Type: "enum"},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("合法 usageSchema 不应报错: %v", err)
	}

	m.UsageSchema = append(m.UsageSchema, UsageField{Key: "clips", Type: "number"})
	if err := m.Validate(); err == nil {
		t.Fatal("重复 key 应当报错")
	}

	m.UsageSchema = []UsageField{{Key: "clips", Type: "number", Unit: "kg"}}
	if err := m.Validate(); err == nil {
		t.Fatal("非法 unit 应当报错")
	}

	m.UsageSchema = []UsageField{{Key: "tier", Type: "enum", Unit: "second"}}
	if err := m.Validate(); err == nil {
		t.Fatal("enum 带 unit 应当报错")
	}

	m.UsageSchema = []UsageField{{Key: "bad key", Type: "number"}}
	if err := m.Validate(); err == nil {
		t.Fatal("非法 key 应当报错")
	}
}

func TestConfigValidation(t *testing.T) {
	m := validManifest()
	m.Config = []ConfigField{
		{Key: "url", Label: "地址", Type: "string"},
		{Key: "secret", Label: "密钥", Type: "string", Encrypted: true},
		{Key: "mode", Label: "模式", Type: "select", Options: []string{"a", "b"}},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("合法 config 不应报错: %v", err)
	}

	m.Config = append(m.Config, ConfigField{Key: "mode", Label: "x", Type: "string"})
	if err := m.Validate(); err == nil {
		t.Fatal("重复配置 key 应当报错")
	}

	m.Config = []ConfigField{{Key: "s", Label: "x", Type: "select"}}
	if err := m.Validate(); err == nil {
		t.Fatal("select 缺 options 应当报错")
	}

	m.Config = []ConfigField{{Key: "s", Label: "x", Type: "string", Encrypted: true, Default: "secret"}}
	if err := m.Validate(); err == nil {
		t.Fatal("加密字段带默认值应当报错")
	}
}

func TestValidateUsageFacts(t *testing.T) {
	m := validManifest()
	m.UsageSchema = []UsageField{
		{Key: "clips", Type: "number", Unit: "count"},
		{Key: "tier", Type: "enum"},
	}

	raw := map[string]json.RawMessage{
		"clips": json.RawMessage(`4`),
		"tier":  json.RawMessage(`"pro"`), // enum 合法但不计费
	}
	facts, err := ValidateUsageFacts(raw, m)
	if err != nil {
		t.Fatalf("合法事实不应报错: %v", err)
	}
	if len(facts) != 1 || facts["clips"] != 4 {
		t.Fatalf("数值过滤异常: %v", facts)
	}

	bad := map[string]json.RawMessage{"unknown": json.RawMessage(`1`)}
	if _, err := ValidateUsageFacts(bad, m); err == nil {
		t.Fatal("未声明键应当报错")
	}

	neg := map[string]json.RawMessage{"clips": json.RawMessage(`-1`)}
	if _, err := ValidateUsageFacts(neg, m); err == nil {
		t.Fatal("负数应当报错")
	}

	str := map[string]json.RawMessage{"clips": json.RawMessage(`"abc"`)}
	if _, err := ValidateUsageFacts(str, m); err == nil {
		t.Fatal("number 型非数值字符串应当报错")
	}
}

func TestArtifactValidation(t *testing.T) {
	good := map[string]json.RawMessage{
		"items": json.RawMessage(`[{"taskId":"t1","done":true,"artifacts":[{"key":"video","source":{"type":"url","path":"data.videos[0].url"}}]}]`),
	}
	result, err := ValidateQueried(good, []QueryTaskRef{{TaskID: "t1"}})
	if err != nil {
		t.Fatalf("合法 artifacts 不应报错: %v", err)
	}
	if len(result.Items[0].Artifacts) != 1 {
		t.Fatal("artifacts 解析缺失")
	}

	badType := map[string]json.RawMessage{
		"items": json.RawMessage(`[{"taskId":"t1","done":true,"artifacts":[{"key":"v","source":{"type":"ftp","path":"x"}}]}]`),
	}
	if _, err := ValidateQueried(badType, []QueryTaskRef{{TaskID: "t1"}}); err == nil {
		t.Fatal("非法 source.type 应当报错")
	}

	inlineNoMime := map[string]json.RawMessage{
		"items": json.RawMessage(`[{"taskId":"t1","done":true,"artifacts":[{"key":"v","source":{"type":"inline","path":"x"}}]}]`),
	}
	if _, err := ValidateQueried(inlineNoMime, []QueryTaskRef{{TaskID: "t1"}}); err == nil {
		t.Fatal("inline 缺 mimeType 应当报错")
	}
}

func TestIconValidation(t *testing.T) {
	cases := []struct {
		icon  string
		valid bool
	}{
		{"", true},
		{"text", true},
		{"text:标签", true},
		{"text:太长的标签会失败", false},
		{"video", true},
		{"video-camera", true},
		{"Video", false}, // 大写不是合法图标名
		{"data:image/png;base64,x", false},
		{"https://evil.com/a.svg", false},
	}
	for _, c := range cases {
		m := validManifest()
		m.Icon = c.icon
		err := m.Validate()
		if c.valid && err != nil {
			t.Fatalf("icon %q 应合法: %v", c.icon, err)
		}
		if !c.valid && err == nil {
			t.Fatalf("icon %q 应被拒绝", c.icon)
		}
	}
}

func TestDynamicQueryIntent(t *testing.T) {
	m := validManifest()
	m.Routes = []Route{{Method: "POST", Path: "/fetch", Type: RouteTypeDynamic, Decode: "decodeBatch"}}
	if err := m.Validate(); err != nil {
		t.Fatalf("dynamic 路由应合法: %v", err)
	}

	// kind=query 仅 dynamic 允许
	queryRaw := map[string]json.RawMessage{
		"kind":    json.RawMessage(`"query"`),
		"taskIds": json.RawMessage(`["task_1","task_2"]`),
	}
	dec, err := ValidateDecoded(queryRaw, m, true)
	if err != nil || len(dec.TaskIDs) != 2 {
		t.Fatalf("dynamic 查询意图应通过: %+v err=%v", dec, err)
	}
	if _, err := ValidateDecoded(queryRaw, m, false); err == nil {
		t.Fatal("非 dynamic 路由应拒绝 kind=query")
	}

	empty := map[string]json.RawMessage{"kind": json.RawMessage(`"query"`), "taskIds": json.RawMessage(`[]`)}
	if _, err := ValidateDecoded(empty, m, true); err == nil {
		t.Fatal("空 taskIds 应拒绝")
	}
}

func TestImmediateValidation(t *testing.T) {
	ok := map[string]json.RawMessage{
		"taskId":    json.RawMessage(`"t1"`),
		"immediate": json.RawMessage(`{"status":"SUCCESS","taskData":{"url":"https://x/a.png"},"usage":{"images":1},"artifacts":[{"key":"img","mimeType":"image/png","source":{"type":"url","path":"url"}}]}`),
	}
	res, err := ValidateSubmitted(ok)
	if err != nil {
		t.Fatalf("合法 immediate 应通过: %v", err)
	}
	if res.Im == nil || res.Im.Status != "SUCCESS" || len(res.Im.Artifacts) != 1 {
		t.Fatalf("immediate 解析异常: %+v", res.Im)
	}

	badStatus := map[string]json.RawMessage{
		"taskId":    json.RawMessage(`"t1"`),
		"immediate": json.RawMessage(`{"status":"QUEUED"}`),
	}
	if _, err := ValidateSubmitted(badStatus); err == nil {
		t.Fatal("非法 immediate.status 应拒绝")
	}

	failNoReason := map[string]json.RawMessage{
		"taskId":    json.RawMessage(`"t1"`),
		"immediate": json.RawMessage(`{"status":"FAILURE"}`),
	}
	res2, err := ValidateSubmitted(failNoReason)
	if err != nil || res2.Im.FailReason == "" {
		t.Fatalf("FAILURE 缺原因应补默认: %+v err=%v", res2.Im, err)
	}
}

func TestAuthorCompat(t *testing.T) {
	// 字符串形态（存量插件）
	var m1 Manifest
	if err := json.Unmarshal([]byte(`{"author":"team-api"}`), &m1); err != nil {
		t.Fatalf("字符串 author 兼容失败: %v", err)
	}
	if m1.Author.Name != "team-api" || m1.Author.URL != "" {
		t.Fatalf("字符串 author 解析异常: %+v", m1.Author)
	}

	// 对象形态（规范形态）
	var m2 Manifest
	if err := json.Unmarshal([]byte(`{"author":{"name":"qian","url":"https://example.com"}}`), &m2); err != nil {
		t.Fatalf("对象 author 解析失败: %v", err)
	}
	if m2.Author.Name != "qian" || m2.Author.URL != "https://example.com" {
		t.Fatalf("对象 author 解析异常: %+v", m2.Author)
	}

	// website 校验
	m3 := validManifest()
	m3.Website = "http://insecure.example.com"
	if err := m3.Validate(); err == nil {
		t.Fatal("非 https website 应拒绝")
	}
	m3.Website = "https://example.com"
	if err := m3.Validate(); err != nil {
		t.Fatalf("https website 应通过: %v", err)
	}
}
