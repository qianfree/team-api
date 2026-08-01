package kitutil

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshal(t *testing.T) {
	in := map[string]any{"a": 1}
	data, err := Marshal(in)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, Unmarshal(data, &out))
	assert.Equal(t, float64(1), out["a"])

	// UnmarshalJsonStr
	var s string
	require.NoError(t, UnmarshalJsonStr(`"hi"`, &s))
	assert.Equal(t, "hi", s)

	// DecodeJson
	var n int
	require.NoError(t, DecodeJson(strings.NewReader("42"), &n))
	assert.Equal(t, 42, n)
}

func TestGetJsonType(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"a":1}`, "object"},
		{`[1,2]`, "array"},
		{`"x"`, "string"},
		{`true`, "boolean"},
		{`false`, "boolean"},
		{`null`, "null"},
		{`123`, "number"},
		{`   `, "unknown"},
		{``, "unknown"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, GetJsonType(json.RawMessage(c.raw)), "input=%q", c.raw)
	}
}

func TestJsonRawMessageToString(t *testing.T) {
	assert.Equal(t, "hello", JsonRawMessageToString(json.RawMessage(`"hello"`)))
	assert.Equal(t, "", JsonRawMessageToString(json.RawMessage(`null`)))
	assert.Equal(t, "", JsonRawMessageToString(json.RawMessage(``)))
	// 非引号值原样返回
	assert.Equal(t, "42", JsonRawMessageToString(json.RawMessage(`42`)))
	assert.Equal(t, `{"a":1}`, JsonRawMessageToString(json.RawMessage(`{"a":1}`)))
	// 损坏的引号串：解析失败 → 回退到原始文本
	assert.Equal(t, `"bad`, JsonRawMessageToString(json.RawMessage(`"bad`)))
}

func TestStringToByteSlice(t *testing.T) {
	got := StringToByteSlice("abc")
	assert.Equal(t, "abc", string(got))
	assert.Len(t, got, 3)
}

func TestAny2Type(t *testing.T) {
	s, err := Any2Type[string]("hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", s)

	n, err := Any2Type[int](float64(7))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	// 失败：字符串无法转为 int
	_, err = Any2Type[int]("not-a-number")
	assert.Error(t, err)
}

func TestInterface2String(t *testing.T) {
	assert.Equal(t, "ab", Interface2String("ab"))
	assert.Equal(t, "42", Interface2String(42))
	assert.Equal(t, "3.5", Interface2String(3.5))
	assert.Equal(t, "true", Interface2String(true))
	assert.Equal(t, "false", Interface2String(false))
	assert.Equal(t, "", Interface2String(nil))
	// default 分支：使用 %v
	assert.Equal(t, "[1]", Interface2String([]int{1}))
}

func TestString2Int(t *testing.T) {
	assert.Equal(t, 42, String2Int("42"))
	assert.Equal(t, 0, String2Int("not-a-number"))
}

func TestGetPointer(t *testing.T) {
	p := GetPointer(7)
	require.NotNil(t, p)
	assert.Equal(t, 7, *p)
}

func TestGetUUID(t *testing.T) {
	id := GetUUID()
	assert.NotEmpty(t, id)
	assert.NotContains(t, id, "-")
}

func TestGetTimestamp(t *testing.T) {
	assert.Greater(t, GetTimestamp(), int64(0))
}

func TestMaskHostTail(t *testing.T) {
	assert.Equal(t, []string{"com"}, maskHostTail([]string{"com"}))                  // len<2 原样
	assert.Equal(t, []string{"com"}, maskHostTail([]string{"example", "com"}))       // 普通两段
	assert.Equal(t, []string{"co", "uk"}, maskHostTail([]string{"example", "co", "uk"}))      // 国家码 TLD
	assert.Equal(t, []string{"com", "cn"}, maskHostTail([]string{"sub", "com", "cn"}))        // 国家码 TLD
}

func TestMaskHostForURL(t *testing.T) {
	assert.Equal(t, "***.com", maskHostForURL("api.openai.com"))
	assert.Equal(t, "***.co.uk", maskHostForURL("sub.domain.co.uk"))
	assert.Equal(t, "***", maskHostForURL("localhost"))
}

func TestMaskHostForPlainDomain(t *testing.T) {
	assert.Equal(t, "***.com", maskHostForPlainDomain("openai.com"))
	assert.Equal(t, "***.***.com", maskHostForPlainDomain("api.openai.com"))
	assert.Equal(t, "***.***.co.uk", maskHostForPlainDomain("sub.domain.co.uk"))
	assert.Equal(t, "localhost", maskHostForPlainDomain("localhost"))
}

func TestMaskSensitiveInfo(t *testing.T) {
	assert.Equal(t, "http://***.com", MaskSensitiveInfo("http://example.com"))
	assert.Equal(t, "https://***.org/***/***/***?key=***",
		MaskSensitiveInfo("https://api.test.org/v1/users/123?key=secret"))
	assert.Equal(t, "***.***.***.***", MaskSensitiveInfo("192.168.1.1"))
	assert.Equal(t, "***.com", MaskSensitiveInfo("openai.com"))
	assert.Equal(t, "***.***.com", MaskSensitiveInfo("api.openai.com"))
	assert.Equal(t, "api_key:***", MaskSensitiveInfo("api_key:AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70"))
}

func TestLoggingHooks(t *testing.T) {
	// 设置钩子：调用时应触发对应钩子
	var infoBuf, errBuf, sysBuf bytes.Buffer
	SetLogging(func(m string) { infoBuf.WriteString(m) }, func(m string) { errBuf.WriteString(m) })
	SetSystemErrorLogging(func(m string) { sysBuf.WriteString(m) })

	LogInfo("hello")
	LogError("boom")
	LogSystemError("crash")
	assert.Equal(t, "hello", infoBuf.String())
	assert.Equal(t, "boom", errBuf.String())
	assert.Equal(t, "crash", sysBuf.String())

	// nil 钩子时走 stderr 回退（不 panic 即可）
	logInfo.Store(nil)
	logError.Store(nil)
	logSystemError.Store(nil)
	require.NotPanics(t, func() {
		LogInfo("fallback-info")
		LogError("fallback-err")
		LogSystemError("fallback-sys")
	})

	// Debug 开关
	Debug.Store(true)
	assert.True(t, Debug.Load())
	Debug.Store(false)
}
