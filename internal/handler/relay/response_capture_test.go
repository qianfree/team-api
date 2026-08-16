package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestResponseCaptureWriterCommitted(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		if capture.ResponseCommitted() {
			t.Fatal("fresh writer should not be committed")
		}
	})

	t.Run("header", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		capture.WriteHeader(http.StatusAccepted)
		if !capture.ResponseCommitted() {
			t.Fatal("writer should be committed after WriteHeader")
		}
		if got := capture.StatusCode(); got != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", got, http.StatusAccepted)
		}
	})

	t.Run("body", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		_, _ = capture.Write([]byte("partial"))
		if !capture.ResponseCommitted() {
			t.Fatal("writer should be committed after Write")
		}
		capture.WriteHeader(http.StatusInternalServerError)
		if got := capture.StatusCode(); got != http.StatusOK {
			t.Fatalf("status = %d, want implicit status %d", got, http.StatusOK)
		}
	})
}

func TestResponseCaptureWriterKeepsFirstStatus(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	capture.WriteHeader(http.StatusAccepted)
	capture.WriteHeader(http.StatusInternalServerError)
	if got := capture.StatusCode(); got != http.StatusAccepted {
		t.Fatalf("status = %d, want first status %d", got, http.StatusAccepted)
	}
}

// 捕获体最终要写入 UTF8 编码的 PostgreSQL，任何无效 UTF-8 字节都会让审计 INSERT
// 整条被拒。以下用例覆盖合法短响应、上游脏字节、head/tail 分界、环形回绕等边界，
// 保证 Body() 恒返回合法 UTF-8。
func TestResponseCaptureBodyRoundTrip(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	_, _ = capture.Write([]byte("data: 你好\n\n"))
	if got := capture.Body(); got != "data: 你好\n\n" {
		t.Fatalf("short valid body should round-trip, got %q", got)
	}
}

// 上游直接输出非法字节（供应商 bug、非 UTF-8 错误页），出口清洗兜底
func TestResponseCaptureBodySanitizesInvalidUpstreamBytes(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	_, _ = capture.Write([]byte("ok \xe4\xb8")) // 半个“中”
	got := capture.Body()
	if !utf8.ValidString(got) {
		t.Fatalf("body should be valid UTF-8, got %q", got)
	}
	if !strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("invalid sequence should be replaced with U+FFFD, got %q", got)
	}
}

// head/tail 分界点落在多字节字符中间时，整字符应归 tail，不留半个字符
func TestResponseCaptureHeadTailSplitKeepsCharIntact(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	_, _ = capture.Write([]byte(strings.Repeat("a", headLimit-2)))
	_, _ = capture.Write([]byte("中xyz"))
	got := capture.Body()
	if !utf8.ValidString(got) {
		t.Fatal("body should be valid UTF-8 after boundary split")
	}
	if !strings.HasSuffix(got, "中xyz") {
		t.Fatalf("char straddling the boundary should stay intact in tail, got %q", got[len(got)-20:])
	}
}

// 环形缓冲区回绕后起点落在字符中间（且无换行可对齐），仅靠出口清洗兜底
func TestResponseCaptureRingWrapBodyValid(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	_, _ = capture.Write([]byte(strings.Repeat("a", headLimit)))
	// 21846 个“中”共 65538 字节，回绕后 tailPos=2，环起点落在“中”的最后一个字节上
	_, _ = capture.Write([]byte(strings.Repeat("中", 21846)))
	if got := capture.Body(); !utf8.ValidString(got) {
		t.Fatal("body should be valid UTF-8 after ring wrap")
	}
}

// 环形回绕起点落在残行中间时应跳到下一行行首，tail 从完整 SSE 行开始
func TestResponseCaptureRingWrapAlignsToLineStart(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	_, _ = capture.Write([]byte(strings.Repeat("a", headLimit)))
	// “d:中中\n”每行 9 字节，14564 行共 131076 字节，回绕后 tailPos=4（落在行首“中”中间）
	_, _ = capture.Write([]byte(strings.Repeat("d:中中\n", 14564)))
	body := capture.Body()
	if !utf8.ValidString(body) {
		t.Fatal("body should be valid UTF-8 after ring wrap")
	}
	prefix := strings.Repeat("a", headLimit) + "\n...[truncated]...\n"
	if !strings.HasPrefix(body, prefix) {
		t.Fatal("body should start with head + truncation marker")
	}
	if rest := body[len(prefix):]; !strings.HasPrefix(rest, "d:") {
		t.Fatalf("tail should start at a complete line, got %q", rest)
	}
}
