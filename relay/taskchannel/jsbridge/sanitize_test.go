package jsbridge

import (
	"strings"
	"testing"
)

// TestJsHookTaskErrorSanitized 提交链钩子错误直达终端用户（writeTaskError 原样写出），
// 消息必须经脱敏：密钥值抹除、超长截断。
func TestJsHookTaskErrorSanitized(t *testing.T) {
	err := &jspluginTestError{msg: `jsplugin kling hook buildSubmitRequest: sign failed, jwt header {"apiKey":"sk-live-abc","alg":"HS256"} rejected`}
	taskErr := jsHookTaskError(err)
	if taskErr.StatusCode != 502 {
		t.Fatalf("状态码应为 502，当前 %d", taskErr.StatusCode)
	}
	if strings.Contains(taskErr.Message, "sk-live-abc") {
		t.Fatalf("密钥值未脱敏: %s", taskErr.Message)
	}
	long := strings.Repeat("上游业务文案", 200)
	taskErr = jsHookTaskError(&jspluginTestError{msg: long})
	if runes := []rune(taskErr.Message); len(runes) > 520 {
		t.Fatalf("超长消息未截断: %d runes", len(runes))
	}
}

type jspluginTestError struct{ msg string }

func (e *jspluginTestError) Error() string { return e.msg }
