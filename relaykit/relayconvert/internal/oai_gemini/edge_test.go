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

func TestMapSchemaType(t *testing.T) {
	cases := map[string]string{
		"string": "STRING", "number": "NUMBER", "integer": "INTEGER",
		"boolean": "BOOLEAN", "object": "OBJECT", "array": "ARRAY", "null": "NULL",
		"custom": "custom", // default 原样返回
	}
	for in, want := range cases {
		assert.Equal(t, want, mapSchemaType(in), "input=%q", in)
	}
}

func TestConvertSchemaMap(t *testing.T) {
	// 非 map 原样返回
	assert.Equal(t, "x", convertSchemaMap("x"))

	in := map[string]any{
		"type":       "string",
		"properties": map[string]any{"a": map[string]any{"type": "integer"}},
		"items":      map[string]any{"type": "boolean"},
		"anyOf":      []any{map[string]any{"type": "null"}},
		"extra":      "keep",
	}
	got := convertSchemaMap(in).(map[string]any)
	assert.Equal(t, "STRING", got["type"])
	assert.Equal(t, "keep", got["extra"])
	props := got["properties"].(map[string]any)
	assert.Equal(t, "INTEGER", props["a"].(map[string]any)["type"])
	assert.Equal(t, "BOOLEAN", got["items"].(map[string]any)["type"])
	anyOf := got["anyOf"].([]any)
	assert.Equal(t, "NULL", anyOf[0].(map[string]any)["type"])
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

	// 非 map 原样返回
	assert.Equal(t, "raw", convertResponseSchema("raw"))
}
