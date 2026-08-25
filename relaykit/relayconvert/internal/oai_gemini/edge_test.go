package oai_gemini

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestParseStopSequences(t *testing.T) {
	assert.Nil(t, parseStopSequences(nil))
	assert.Nil(t, parseStopSequences(""))
	assert.Equal(t, []string{"stop"}, parseStopSequences("stop"))
	assert.Equal(t, []string{"a", "b"}, parseStopSequences([]string{"a", "b"}))
	// []any：非字符串项被过滤
	assert.Equal(t, []string{"a", "b"}, parseStopSequences([]any{"a", 123, "b"}))
	// 其它类型 → nil
	assert.Nil(t, parseStopSequences(123))
}

func TestExtractText(t *testing.T) {
	assert.Equal(t, "hi", extractText("hi"))
	// 多个 text part → 取第一个
	assert.Equal(t, "first", extractText([]any{
		map[string]any{"type": "text", "text": "first"},
		map[string]any{"type": "text", "text": "second"},
	}))
	// 无 text part
	assert.Equal(t, "", extractText([]any{map[string]any{"type": "image_url"}}))
	assert.Equal(t, "", extractText(nil))
	assert.Equal(t, "", extractText(123))
}

func TestConvertUserParts(t *testing.T) {
	// 空字符串 → nil
	assert.Nil(t, convertUserParts(""))
	// 纯文本
	assert.Equal(t, []dto.GeminiPart{{Text: "hi"}}, convertUserParts("hi"))

	// 多模态：text + image_url(data URL) + input_audio + 非 map 项
	parts := convertUserParts([]any{
		map[string]any{"type": "text", "text": "hello"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc"}},
		map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": "blob", "format": "mp3"}},
		"not-a-map", // 被跳过
	})
	if !assert.Len(t, parts, 3) {
		return
	}
	assert.Equal(t, "hello", parts[0].Text)
	if !assert.NotNil(t, parts[1].InlineData) {
		return
	}
	assert.Equal(t, "image/png", parts[1].InlineData.MimeType)
	assert.Equal(t, "abc", parts[1].InlineData.Data)
	if !assert.NotNil(t, parts[2].InlineData) {
		return
	}
	assert.Equal(t, "audio/mp3", parts[2].InlineData.MimeType)
	assert.Equal(t, "blob", parts[2].InlineData.Data)

	// 非 string / 非 []any → nil
	assert.Nil(t, convertUserParts(123))
}

func TestParseDataURL(t *testing.T) {
	mt, data, ok := parseDataURL("data:image/png;base64,abc")
	assert.True(t, ok)
	assert.Equal(t, "image/png", mt)
	assert.Equal(t, "abc", data)

	// 非 data: 前缀
	_, _, ok = parseDataURL("http://x/y")
	assert.False(t, ok)
	// 过短
	_, _, ok = parseDataURL("data:")
	assert.False(t, ok)
	// 无分号
	_, _, ok = parseDataURL("data:imagepngbase64abc")
	assert.False(t, ok)
	// 无 base64,
	_, _, ok = parseDataURL("data:image/png;notbase64,abc")
	assert.False(t, ok)
}

func TestNormalizeGeminiSchemaType(t *testing.T) {
	// 小写 → Gemini 大写枚举名；"null" 折叠为 nullable
	cases := map[string]string{
		"string": "STRING", "number": "NUMBER", "integer": "INTEGER",
		"boolean": "BOOLEAN", "object": "OBJECT", "array": "ARRAY",
		"custom": "custom", // 未知类型原样保留
	}
	for in, want := range cases {
		schema := map[string]any{"type": in}
		normalizeGeminiSchemaType(schema)
		assert.Equal(t, want, schema["type"], "input=%q", in)
	}
	nullSchema := map[string]any{"type": "null"}
	normalizeGeminiSchemaType(nullSchema)
	assert.NotContains(t, nullSchema, "type")
	assert.Equal(t, true, nullSchema["nullable"])

	// 数组形态（schemars 对 Option<T> 生成 ["string","null"]）→ 单一 type + nullable
	arrSchema := map[string]any{"type": []any{"string", "null"}}
	normalizeGeminiSchemaType(arrSchema)
	assert.Equal(t, "STRING", arrSchema["type"])
	assert.Equal(t, true, arrSchema["nullable"])

	// 无 type / nil type → 不动
	empty := map[string]any{"description": "d"}
	normalizeGeminiSchemaType(empty)
	assert.NotContains(t, empty, "nullable")
}

func TestCleanGeminiSchema(t *testing.T) {
	// 非 map 原样返回
	assert.Equal(t, "x", cleanGeminiSchema("x", 0))

	in := map[string]any{
		"type":       "string",
		"properties": map[string]any{"a": map[string]any{"type": "integer"}},
		"items":      map[string]any{"type": "boolean"},
		"anyOf":      []any{map[string]any{"type": "null"}},
		"extra":      "dropped", // 不在白名单 → 剔除
	}
	got := cleanGeminiSchema(in, 0).(map[string]any)
	assert.Equal(t, "STRING", got["type"])
	assert.NotContains(t, got, "extra")
	props := got["properties"].(map[string]any)
	assert.Equal(t, "INTEGER", props["a"].(map[string]any)["type"])
	assert.Equal(t, "BOOLEAN", got["items"].(map[string]any)["type"])
	// 嵌套 anyOf 内的 "null" 折叠为 nullable
	anyOf := got["anyOf"].([]any)
	nested := anyOf[0].(map[string]any)
	assert.NotContains(t, nested, "type")
	assert.Equal(t, true, nested["nullable"])
}

// TestCleanParamsStripsNestedAdditionalProperties 回归测试：codex CLI（schemars 生成）的
// 工具 schema 在每个嵌套 object 上携带 additionalProperties:false，旧实现仅清洗顶层，
// 嵌套 items 内的透传导致 Gemini 400 "Unknown name additionalProperties"。
// 数据取自生产抓包（update_plan / request_user_input 两个真实工具）。
func TestCleanParamsStripsNestedAdditionalProperties(t *testing.T) {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"explanation": map[string]any{"type": "string"},
			"plan": map[string]any{
				"description": "The list of steps",
				"type":        "array",
				"items": map[string]any{
					"additionalProperties": false,
					"type":                 "object",
					"properties": map[string]any{
						"step":   map[string]any{"type": "string"},
						"status": map[string]any{"type": "string", "enum": []any{"pending", "in_progress", "completed"}},
					},
					"required": []any{"step", "status"},
				},
				"$schema": "http://json-schema.org/draft-07/schema#", // 不在白名单 → 剔除
			},
		},
		"required":             []any{"plan"},
		"additionalProperties": false, // 顶层（旧实现也能洗掉）
	}

	cleaned := cleanParams(params).(map[string]any)
	assert.NotContains(t, cleaned, "additionalProperties")
	assert.Equal(t, "OBJECT", cleaned["type"])

	plan := cleaned["properties"].(map[string]any)["plan"].(map[string]any)
	assert.Equal(t, "ARRAY", plan["type"])
	assert.NotContains(t, plan, "$schema")

	items := plan["items"].(map[string]any)
	assert.NotContains(t, items, "additionalProperties", "嵌套 items 内的 additionalProperties 必须被剔除")
	assert.Equal(t, "OBJECT", items["type"])
	status := items["properties"].(map[string]any)["status"].(map[string]any)
	assert.Equal(t, "STRING", status["type"])
	assert.Equal(t, []any{"pending", "in_progress", "completed"}, status["enum"])
}

// TestCleanParamsDeepNested 三层以上嵌套（request_user_input.questions.items.properties.options.items）
// 也必须逐层清洗
func TestCleanParamsDeepNested(t *testing.T) {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"additionalProperties": false,
					"type":                 "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "string"},
						"options": map[string]any{
							"type": "array",
							"items": map[string]any{
								"additionalProperties": false,
								"type":                 "object",
								"properties": map[string]any{
									"label":       map[string]any{"type": "string"},
									"description": map[string]any{"type": "string"},
								},
							},
							"$defs": "x", // 不在白名单 → 剔除
						},
					},
				},
			},
		},
	}

	cleaned := cleanParams(params).(map[string]any)
	questions := cleaned["properties"].(map[string]any)["questions"].(map[string]any)
	qItems := questions["items"].(map[string]any)
	assert.NotContains(t, qItems, "additionalProperties")
	options := qItems["properties"].(map[string]any)["options"].(map[string]any)
	assert.NotContains(t, options, "$defs")
	optItems := options["items"].(map[string]any)
	assert.NotContains(t, optItems, "additionalProperties", "三层以上嵌套的 additionalProperties 必须被剔除")
	assert.Equal(t, "OBJECT", optItems["type"])
}

func TestConvertResponseSchema(t *testing.T) {
	// nil → nil
	assert.Nil(t, convertResponseSchema(nil))

	// json_schema 包装
	got := convertResponseSchema(map[string]any{
		"json_schema": map[string]any{
			"schema": map[string]any{"type": "object"},
		},
	})
	assert.Equal(t, "OBJECT", got.(map[string]any)["type"])

	// 裸 schema（无包装）
	got = convertResponseSchema(map[string]any{"type": "string"})
	assert.Equal(t, "STRING", got.(map[string]any)["type"])

	// strict 模式 schema 顶层带 additionalProperties:false → 必须剔除（回归）
	got = convertResponseSchema(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "string",
					"additionalProperties": false,
				},
			},
		},
	})
	respMap := got.(map[string]any)
	assert.NotContains(t, respMap, "additionalProperties")
	tags := respMap["properties"].(map[string]any)["tags"].(map[string]any)
	assert.NotContains(t, tags["items"].(map[string]any), "additionalProperties")

	// 非 map 原样返回
	assert.Equal(t, "raw", convertResponseSchema("raw"))
}
