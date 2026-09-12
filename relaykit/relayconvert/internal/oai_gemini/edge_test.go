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

func TestConvertResponseSchema(t *testing.T) {
	assert.Nil(t, convertResponseSchema(nil))

	// json_schema 包装（{name,schema,strict}）解包为裸 schema：带着 name 外壳发出会被
	// protojson 拒绝（Unknown name "name" at 'request.generation_config.response_schema'）
	got := convertResponseSchema(map[string]any{
		"name":   "weather",
		"schema": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
	})
	m := got.(map[string]any)
	assert.Equal(t, "object", m["type"])
	assert.NotContains(t, m, "name")
	props := m["properties"].(map[string]any)
	assert.Equal(t, "string", props["city"].(map[string]any)["type"])

	// 裸 schema（无包装）原样归一化
	got = convertResponseSchema(map[string]any{"type": "string"})
	assert.Equal(t, "string", got.(map[string]any)["type"])

	// 归一化生效：白名单外关键字剔除、缺 type 节点补全
	got = convertResponseSchema(map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"properties": map[string]any{
			"n": map[string]any{"type": "integer"},
			"x": map[string]any{"description": "补 type"},
		},
	})
	m = got.(map[string]any)
	assert.NotContains(t, m, "$schema")
	props = m["properties"].(map[string]any)
	assert.Equal(t, "string", props["x"].(map[string]any)["type"])
}
