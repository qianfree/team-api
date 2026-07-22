package constant

import (
	"io"
	"syscall"
	"testing"
)

// TestIsRetryableForRequest 覆盖 DoRequest 阶段的 mode 感知重试判定：
// 对高成本非幂等生成（图片/视频），「可能已送达上游」的模糊网络错误不重试；
// 「确定未送达」的错误与状态码类错误仍照常重试；其它端点（chat）行为不变。
func TestIsRetryableForRequest(t *testing.T) {
	relay502 := &RelayError{StatusCode: 502}
	relay400 := &RelayError{StatusCode: 400}

	cases := []struct {
		name      string
		err       error
		expensive bool
		want      bool
	}{
		// chat（expensive=false）：与 IsRetryable 一致，模糊错误照常重试。
		{"chat/eof", io.EOF, false, true},
		{"chat/reset", syscall.ECONNRESET, false, true},
		{"chat/refused", syscall.ECONNREFUSED, false, true},

		// 图片/视频（expensive=true）：模糊「可能已送达」错误不重试。
		{"image/eof", io.EOF, true, false},
		{"image/unexpected_eof", io.ErrUnexpectedEOF, true, false},
		{"image/reset", syscall.ECONNRESET, true, false},

		// 图片/视频：确定未送达 → 仍重试。
		{"image/refused", syscall.ECONNREFUSED, true, true},

		// 状态码类错误：已拿到上游响应（未成功生成）→ 图片也可重试。
		{"image/status_502", relay502, true, true},

		// 本就不可重试的错误：任何情况都不重试。
		{"image/status_400", relay400, true, false},
		{"chat/status_400", relay400, false, false},
		{"nil", nil, true, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsRetryableForRequest(c.err, c.expensive); got != c.want {
				t.Fatalf("IsRetryableForRequest(%v, expensive=%v) = %v, want %v", c.err, c.expensive, got, c.want)
			}
		})
	}
}
