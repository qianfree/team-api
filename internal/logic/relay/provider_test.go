package relay

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/qianfree/team-api/relay/common"
)

func TestSafeUTF8Truncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
	}{
		{"shorter than max", "hello", 10},
		{"equal to max", "hello", 5},
		{"ascii cut", "hello world", 5},
		{"multibyte euro cut mid-rune", "a€b", 2}, // € = 3 bytes
		{"multibyte euro cut mid-rune 2", "a€b", 3},
		{"cjk cut", "你好世界", 5}, // each rune 3 bytes
		{"emoji cut", "🚀🚀", 3}, // each emoji 4 bytes
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeUTF8Truncate(tt.s, tt.maxLen)
			if len(got) > tt.maxLen {
				t.Errorf("result len %d exceeds maxLen %d", len(got), tt.maxLen)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result %q is not valid UTF-8 (truncated mid-rune)", got)
			}
			if !strings.HasPrefix(tt.s, got) {
				t.Errorf("result %q is not a prefix of input %q", got, tt.s)
			}
		})
	}
}

func TestTruncateBody_ShortReturnedVerbatim(t *testing.T) {
	s := "short body"
	if got := truncateBody(s, 100); got != s {
		t.Errorf("short body should be returned verbatim, got %q", got)
	}
}

func TestTruncateBody_LongNonStreamTruncated(t *testing.T) {
	s := strings.Repeat("x", 500)
	got := truncateBody(s, 100)
	if len(got) >= len(s) {
		t.Errorf("expected truncation, got len %d (input %d)", len(got), len(s))
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("expected truncation marker suffix, got %q", got)
	}
}

func TestTruncateStreamBody_KeepsHeadAndTailDropsMiddle(t *testing.T) {
	// 40 行短 SSE，每行 "data: Lxx"（9 字节）。尾部 20 行 = 199 字节。
	// maxLen=250 留出 headBudget>0，结果形如 head + marker + tail。
	var lines []string
	for i := range 40 {
		lines = append(lines, fmt.Sprintf("data: L%02d", i))
	}
	s := strings.Join(lines, "\n")

	got := truncateStreamBody(s, 250)

	if len(got) > len(s) {
		t.Fatalf("output longer than input: %d > %d", len(got), len(s))
	}
	if !strings.Contains(got, "data: L00") {
		t.Errorf("head line 'data: L00' should be kept, got:\n%s", got)
	}
	if !strings.Contains(got, "data: L39") {
		t.Errorf("tail line 'data: L39' should be kept, got:\n%s", got)
	}
	if strings.Contains(got, "data: L10") {
		t.Errorf("middle line 'data: L10' should have been dropped, got:\n%s", got)
	}
	if !strings.Contains(got, "...[truncated]...") {
		t.Errorf("expected middle truncation marker, got:\n%s", got)
	}
}

func TestTruncateBody_RoutesStreamAndPreservesLastLine(t *testing.T) {
	// 流式响应的关键信息（usage / [DONE]）在末尾。普通截断会保留头部、丢掉末尾；
	// 流式感知截断会保留尾部。以此验证 truncateBody 正确路由到流式分支。
	var lines []string
	for i := range 39 {
		lines = append(lines, fmt.Sprintf("data: L%02d", i))
	}
	lines = append(lines, "data: [DONE]")
	s := strings.Join(lines, "\n")

	got := truncateBody(s, 300)
	if !strings.Contains(got, "[DONE]") {
		t.Errorf("stream-aware truncation should preserve trailing [DONE], got:\n%s", got)
	}
	if len(got) >= len(s) {
		t.Errorf("expected truncation, got len %d (input %d)", len(got), len(s))
	}
}

func TestJSONNullIfEmpty(t *testing.T) {
	if got := jsonNullIfEmpty(""); got != nil {
		t.Errorf("empty string should map to nil, got %v", got)
	}
	if got := jsonNullIfEmpty("{}"); got != "{}" {
		t.Errorf("non-empty string should pass through, got %v", got)
	}
}

func TestParseCapabilitiesJSON(t *testing.T) {
	if got := parseCapabilitiesJSON(""); got != nil {
		t.Errorf("empty -> nil, got %v", got)
	}
	if got := parseCapabilitiesJSON("{}"); got != nil {
		t.Errorf("empty object -> nil, got %v", got)
	}
	if got := parseCapabilitiesJSON("not json"); got != nil {
		t.Errorf("invalid json -> nil, got %v", got)
	}
	got := parseCapabilitiesJSON(`{"vision":true,"tools":false}`)
	if got == nil {
		t.Fatal("valid json should parse to a map")
	}
	if !got["vision"] || got["tools"] {
		t.Errorf("parsed map wrong: %v", got)
	}
}

func TestBuildUsageLogDO_RequestTypeDerivation(t *testing.T) {
	tests := []struct {
		name        string
		requestType int
		isStream    bool
		want        int
	}{
		{"explicit type preserved", 3, true, 3},
		{"derived stream", 0, true, 2},
		{"derived sync", 0, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := buildUsageLogDO(&common.UsageRecord{
				RequestType: tt.requestType,
				IsStream:    tt.isStream,
			})
			got, ok := d.RequestType.(int)
			if !ok || got != tt.want {
				t.Errorf("RequestType = %v (%T), want %d", d.RequestType, d.RequestType, tt.want)
			}
		})
	}
}

func TestBuildUsageLogDO_FieldMapping(t *testing.T) {
	rec := &common.UsageRecord{
		TenantID:        7,
		LatencyMs:       123.9, // 应被截断为 int 123
		BillingSnapshot: "",    // 空 -> nil
	}
	d := buildUsageLogDO(rec)

	if d.TenantId != int64(7) {
		t.Errorf("TenantId = %v, want 7", d.TenantId)
	}
	if got, ok := d.LatencyMs.(int); !ok || got != 123 {
		t.Errorf("LatencyMs = %v (%T), want int 123", d.LatencyMs, d.LatencyMs)
	}
	if d.BillingSnapshot != nil {
		t.Errorf("empty BillingSnapshot should map to nil, got %v", d.BillingSnapshot)
	}

	rec2 := &common.UsageRecord{BillingSnapshot: `{"k":1}`}
	if got := buildUsageLogDO(rec2).BillingSnapshot; got != `{"k":1}` {
		t.Errorf("non-empty BillingSnapshot should pass through, got %v", got)
	}
}
