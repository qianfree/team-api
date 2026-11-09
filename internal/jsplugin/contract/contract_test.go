package contract

import (
	"encoding/json"
	"reflect"
	"strconv"
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

func TestStandardVideosValidation(t *testing.T) {
	// 缺省（不声明）与合法钩子名均通过
	m := validManifest()
	if err := m.Validate(); err != nil {
		t.Fatalf("未声明 standardVideos 不应报错: %v", err)
	}
	m.StandardVideos = "decodeVideos"
	if err := m.Validate(); err != nil {
		t.Fatalf("合法 standardVideos 不应报错: %v", err)
	}

	// 非导出函数名形态（含非法字符/空）应拒绝
	for _, bad := range []string{"1bad", "has space", "带中文"} {
		m.StandardVideos = bad
		if err := m.Validate(); err == nil {
			t.Fatalf("standardVideos %q 应拒绝", bad)
		}
	}
}

func TestModelsFromConfigValidation(t *testing.T) {
	// 合法声明：键在 config 中、类型 text、非加密
	m := validManifest()
	m.ModelsFromConfig = "extra_models"
	m.Config = []ConfigField{{Key: "extra_models", Label: "扩展模型", Type: "text"}}
	if err := m.Validate(); err != nil {
		t.Fatalf("合法 modelsFromConfig 不应报错: %v", err)
	}

	// 未在 config 中声明的键
	m2 := validManifest()
	m2.ModelsFromConfig = "extra_models"
	if err := m2.Validate(); err == nil {
		t.Fatal("未声明的配置键应拒绝")
	}

	// 类型不可承载名单 + 加密声明
	m3 := validManifest()
	m3.ModelsFromConfig = "extra_models"
	m3.Config = []ConfigField{{Key: "extra_models", Label: "扩展模型", Type: "int"}}
	if err := m3.Validate(); err == nil {
		t.Fatal("int 类型应拒绝")
	}
	m4 := validManifest()
	m4.ModelsFromConfig = "extra_models"
	m4.Config = []ConfigField{{Key: "extra_models", Label: "扩展模型", Type: "text", Encrypted: true}}
	if err := m4.Validate(); err == nil {
		t.Fatal("encrypted 声明应拒绝")
	}
}

func TestParseConfigModels(t *testing.T) {
	// 字符串形态：换行/逗号/分号混用、trim、去空、按出现序去重
	got, err := ParseConfigModels("\n m-a , m-b\n;m-a \nm-c,,\n")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := []string{"m-a", "m-b", "m-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("解析异常: %v", got)
	}

	// JSON 数组形态（json 型配置）
	if _, err := ParseConfigModels([]any{"m-x", " m-y ", 1}); err == nil {
		t.Fatal("非字符串元素应报错")
	}
	got3, err := ParseConfigModels([]any{"m-x", " m-y "})
	if err != nil || !reflect.DeepEqual(got3, []string{"m-x", "m-y"}) {
		t.Fatalf("数组形态解析异常: %v %v", got3, err)
	}

	// 空值与非法类型
	if got4, err := ParseConfigModels(nil); err != nil || got4 != nil {
		t.Fatalf("nil 应返回空: %v %v", got4, err)
	}
	if got5, err := ParseConfigModels(""); err != nil || got5 != nil {
		t.Fatalf("空串应返回空: %v %v", got5, err)
	}
	if _, err := ParseConfigModels(42); err == nil {
		t.Fatal("数值类型应报错")
	}

	// 超上限拒绝（去重后的互异名超限才触发）
	big := make([]any, MaxConfigModels+1)
	for i := range big {
		big[i] = "m-" + strconv.Itoa(i)
	}
	if _, err := ParseConfigModels(big); err == nil {
		t.Fatal("超上限应报错")
	}
}

func TestHostConfigFields(t *testing.T) {
	m := &Manifest{Config: []ConfigField{{Key: "extra_models", Label: "扩展模型", Type: "text"}}}
	all := m.AllConfig()
	if len(all) != 2 || all[0].Key != "extra_models" || all[1].Key != HostConfigArtifactStorage {
		t.Fatalf("AllConfig 应为插件声明项 + 宿主内置项: %+v", all)
	}
	if f := m.ConfigFieldByKey(HostConfigArtifactStorage); f == nil || f.Type != "bool" || f.Default != true {
		t.Fatalf("宿主内置项应可按 key 查到且缺省开启: %+v", f)
	}
	if len(m.Config) != 1 {
		t.Fatal("AllConfig 不得改写插件声明切片")
	}
	reserved := &Manifest{Config: []ConfigField{{Key: HostConfigArtifactStorage, Label: "x", Type: "bool"}}}
	if err := reserved.validateConfig(); err == nil {
		t.Fatal("插件声明宿主保留键应被拒绝")
	}
}

func TestReferenceRolesManifest(t *testing.T) {
	m := &Manifest{}
	if !m.SupportsReferenceRole("first_frame") || m.SupportsReferenceRole("reference_image") {
		t.Fatal("未声明 referenceRoles 时应仅支持 first_frame")
	}
	m.ReferenceRoles = []string{"reference_image"}
	if m.SupportsReferenceRole("first_frame") || !m.SupportsReferenceRole("reference_image") {
		t.Fatal("声明后应按声明判定")
	}
}
