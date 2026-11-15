package formula

import (
	"fmt"
	"strings"
	"testing"
)

// TestHeaderSensitiveKeyRejected header() 引用凭证类请求头（authorization/x-api-key 等）
// 在编译期被拒绝,防止密钥明文写入计费快照
func TestHeaderSensitiveKeyRejected(t *testing.T) {
	sensitive := []string{"authorization", "Authorization", "x-api-key", "X-API-Key", "cookie", "x-goog-api-key"}
	for _, key := range sensitive {
		src := fmt.Sprintf(`header(%q) == "pro" ? 2 : 1`, key)
		if _, err := Compile(src); err == nil {
			t.Errorf("header(%q) 应被拒绝，但编译通过", key)
		} else if !strings.Contains(err.Error(), "凭证") && !strings.Contains(err.Error(), "密钥") {
			t.Errorf("header(%q) 错误信息不明确: %v", key, err)
		}
	}
	// 普通请求头不受影响
	if _, err := Compile(`header("user-agent") == "bot" ? 2 : 1`); err != nil {
		t.Errorf("普通请求头应编译通过: %v", err)
	}
}
