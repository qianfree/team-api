package relay

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/qianfree/team-api/relay/common"
)

func TestChannelKeyUsageTracker(t *testing.T) {
	tracker := newChannelKeyUsageTracker()
	now := time.Now()
	if !tracker.claim(1, now) {
		t.Fatal("first claim should update")
	}
	if tracker.claim(1, now.Add(channelKeyLastUsedInterval-time.Second)) {
		t.Fatal("claim inside interval should be throttled")
	}
	if !tracker.claim(1, now.Add(channelKeyLastUsedInterval)) {
		t.Fatal("claim at interval boundary should update")
	}
}

func TestChannelKeyUsageTrackerRelease(t *testing.T) {
	tracker := newChannelKeyUsageTracker()
	now := time.Now()
	if !tracker.claim(1, now) {
		t.Fatal("first claim should update")
	}
	tracker.release(1, now)
	if !tracker.claim(1, now.Add(time.Second)) {
		t.Fatal("released claim should be immediately retryable")
	}
}

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

func TestTruncateBody_ZeroMaxLenDisablesTruncation(t *testing.T) {
	// 配置了独立审计库时 auditBodyMaxLen 返回 0，truncateBody 应原样返回不截断。
	s := strings.Repeat("x", 10000)
	if got := truncateBody(s, 0); got != s {
		t.Errorf("maxLen=0 should disable truncation and return input verbatim")
	}
	// 负数 maxLen 同样视为不截断（防御性）
	if got := truncateBody(s, -1); got != s {
		t.Errorf("negative maxLen should disable truncation and return input verbatim")
	}
	// 流式输入在 maxLen=0 时同样完整保留，不进入流式截断分支
	stream := "data: first\ndata: second\ndata: [DONE]"
	if got := truncateBody(stream, 0); got != stream {
		t.Errorf("stream body with maxLen=0 should be returned verbatim")
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

func TestBuildUsageLogDO_SanitizesInvalidUTF8(t *testing.T) {
	// 构造带原始 0xa0 字节（非法 UTF-8，PG 协议层会整体拒绝插入）的 UsageRecord，
	// 验证 buildUsageLogDO 已对所有字符串字段清洗为合法 UTF-8。
	bad := "pre\xa0post"
	rec := &common.UsageRecord{
		TenantID:        7,
		ModelName:       bad,
		RequestID:       bad,
		Status:          bad,
		ErrorMessage:    bad,
		ClientIP:        bad,
		Currency:        bad,
		RequestedModel:  bad,
		UpstreamModel:   bad,
		UserAgent:       bad,
		ServiceTier:     bad,
		ReasoningEffort: bad,
		InboundEndpoint: bad,
		ChannelName:     bad,
		BillingMode:     bad,
		BillingSource:   bad,
		StreamEndReason: bad,
		ImageSize:       bad,
		BillingSnapshot: `{"err":"` + bad + `"}`,
		BillingSummary:  bad,
		TaskID:          bad,
	}

	d := buildUsageLogDO(rec)

	// 逐一校验所有字符串字段
	v := reflect.ValueOf(d)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Interface || field.IsNil() {
			continue
		}
		s, ok := field.Interface().(string)
		if !ok {
			continue
		}
		if !utf8.ValidString(s) {
			t.Errorf("field %s still contains invalid UTF-8: %q", v.Type().Field(i).Name, s)
		}
		if strings.Contains(s, "\xa0") {
			t.Errorf("field %s still contains raw 0xa0 byte: %q", v.Type().Field(i).Name, s)
		}
	}

	// 校验清洗结果：0xa0 被替换为 U+FFFD
	if got, _ := d.ErrorMessage.(string); got != "pre�post" {
		t.Errorf("ErrorMessage = %q, want %q", got, "pre�post")
	}
}
