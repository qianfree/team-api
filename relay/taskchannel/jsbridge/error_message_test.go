package jsbridge

import (
	"strings"
	"testing"
)

// upstreamErrorMessage 错误信封剥离测试：三层嵌套（经销商信封内嵌 stringify 的
// 上游错误）、直接 OpenAI/Ark 信封、纯 message、非 JSON 回落原文。
func TestUpstreamErrorMessage(t *testing.T) {
	// 实报形态：MyAPI 信封 {code,message,data}，message 内嵌 stringify 的 Ark 错误
	nested := []byte(`{"code":"fail_to_fetch_task","message":"{\"error\":{\"code\":\"AuthenticationError\",\"message\":\"The API key doesn't exist. Request id: 0217941577\",\"param\":\"\",\"type\":\"Unauthorized\"}}","data":null}`)
	got := upstreamErrorMessage(nested)
	if got != "AuthenticationError: The API key doesn't exist. Request id: 0217941577" {
		t.Fatalf("三层嵌套应剥到最内层消息: %q", got)
	}

	// 直接 OpenAI/Ark 信封
	openai := []byte(`{"error":{"code":"AuthenticationError","message":"The API key doesn't exist.","param":"","type":"Unauthorized"}}`)
	if got := upstreamErrorMessage(openai); got != "AuthenticationError: The API key doesn't exist." {
		t.Fatalf("OpenAI 信封提取异常: %q", got)
	}

	// 顶层纯 message 信封（{code,message} / {message}）
	if got := upstreamErrorMessage([]byte(`{"code":"fail","message":"plain text message"}`)); got != "fail: plain text message" {
		t.Fatalf("经销商信封提取异常: %q", got)
	}
	if got := upstreamErrorMessage([]byte(`{"message":"just a message"}`)); got != "just a message" {
		t.Fatalf("纯 message 提取异常: %q", got)
	}

	// 非 JSON（HTML 错误页 / 纯文本）：回落原文
	if got := upstreamErrorMessage([]byte("<html>Bad Gateway</html>")); !strings.Contains(got, "Bad Gateway") {
		t.Fatalf("非 JSON 应回落原文: %q", got)
	}
	if got := upstreamErrorMessage([]byte("upstream exploded")); !strings.Contains(got, "upstream exploded") {
		t.Fatalf("纯文本应保留: %q", got)
	}

	// 超 4 层嵌套：最多剥三层，不无限递归
	deep := []byte(`{"message":"{\"message\":\"{\"message\":\"{\"message\":\"fourth level\"}\"}"}`)
	if got := upstreamErrorMessage(deep); got == "" {
		t.Fatalf("深层嵌套应有兜底输出: %q", got)
	}
}

func TestAuthFingerprint(t *testing.T) {
	if got := authFingerprint(""); got != "<none>" {
		t.Fatalf("空凭证异常: %q", got)
	}
	if got := authFingerprint("Bearer short"); !strings.Contains(got, "len=5") {
		t.Fatalf("短 key 应全掩码: %q", got)
	}
	got := authFingerprint("Bearer sk-lOMZY6HCGl876xxBwhO5h9ZNHNawLDBNYsvKdawfcTCsefQ4")
	if !strings.HasPrefix(got, "Bearer sk-lO") || !strings.Contains(got, "efQ4(len=51)") {
		t.Fatalf("指纹应保留首6尾4与长度: %q", got)
	}
}
