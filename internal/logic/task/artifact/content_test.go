package artifact

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	rcommon "github.com/qianfree/team-api/relay/common"
)

func stubSigning(t *testing.T, now time.Time) {
	t.Helper()
	oldKey, oldNow, oldBase, oldTTL := signingKeyFn, nowFn, publicBaseFn, contentTTLFn
	signingKeyFn = func(context.Context) ([]byte, error) { return []byte("test-signing-key"), nil }
	nowFn = func() time.Time { return now }
	publicBaseFn = func(context.Context) string { return "https://api.example.com" }
	contentTTLFn = func(context.Context) time.Duration { return time.Hour }
	t.Cleanup(func() { signingKeyFn, nowFn, publicBaseFn, contentTTLFn = oldKey, oldNow, oldBase, oldTTL })
}

func parseSigned(t *testing.T, raw string) (path, expires, sig string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("签名链接不可解析: %v", err)
	}
	return u.Path, u.Query().Get("expires"), u.Query().Get("signature")
}

func TestContentURLSignVerify(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	stubSigning(t, now)
	ctx := context.Background()

	raw := SignContentURL(ctx, "task_abc", "")
	if !strings.HasPrefix(raw, "https://api.example.com/v1/tasks/task_abc/content?") {
		t.Fatalf("主产物链接形态异常: %s", raw)
	}
	_, exp, sig := parseSigned(t, raw)
	if err := VerifyContentURL(ctx, "task_abc", "", exp, sig); err != nil {
		t.Fatalf("合法签名校验失败: %v", err)
	}
	// 签名绑定任务 ID 与产物键：换任意一个都失效
	if err := VerifyContentURL(ctx, "task_other", "", exp, sig); !errors.Is(err, ErrContentURLInvalid) {
		t.Fatalf("换任务 ID 应失效: %v", err)
	}
	if err := VerifyContentURL(ctx, "task_abc", "cover", exp, sig); !errors.Is(err, ErrContentURLInvalid) {
		t.Fatalf("换产物键应失效: %v", err)
	}
	// 篡改过期时刻
	if err := VerifyContentURL(ctx, "task_abc", "", exp+"0", sig); !errors.Is(err, ErrContentURLInvalid) {
		t.Fatalf("篡改 expires 应失效: %v", err)
	}
	if err := VerifyContentURL(ctx, "task_abc", "", "bad", sig); !errors.Is(err, ErrContentURLInvalid) {
		t.Fatalf("非法 expires 应失效: %v", err)
	}
	if err := VerifyContentURL(ctx, "task_abc", "", exp, ""); !errors.Is(err, ErrContentURLInvalid) {
		t.Fatalf("空签名应失效: %v", err)
	}
	// 过期
	nowFn = func() time.Time { return now.Add(2 * time.Hour) }
	if err := VerifyContentURL(ctx, "task_abc", "", exp, sig); !errors.Is(err, ErrContentURLExpired) {
		t.Fatalf("过期链接应报 expired: %v", err)
	}
}

func TestContentURLWithArtifactKey(t *testing.T) {
	stubSigning(t, time.Unix(1_800_000_000, 0))
	ctx := context.Background()
	raw := SignContentURL(ctx, "task_abc", "cover")
	path, exp, sig := parseSigned(t, raw)
	if path != "/v1/tasks/task_abc/content/cover" {
		t.Fatalf("产物链接路径异常: %s", path)
	}
	if err := VerifyContentURL(ctx, "task_abc", "cover", exp, sig); err != nil {
		t.Fatalf("产物链接校验失败: %v", err)
	}
}

func TestPickArtifactRow(t *testing.T) {
	if pickArtifactRow(nil, "") != nil {
		t.Fatal("空集应返回 nil")
	}
	cover := &artifactRow{ArtifactKey: "cover", MimeType: "image/png"}
	clip := &artifactRow{ArtifactKey: "clip", MimeType: "video/mp4"}
	video := &artifactRow{ArtifactKey: "video", MimeType: "video/mp4"}
	if got := pickArtifactRow([]*artifactRow{cover, clip, video}, ""); got != video {
		t.Fatalf("key=video 应优先: %+v", got)
	}
	if got := pickArtifactRow([]*artifactRow{cover, clip}, ""); got != clip {
		t.Fatalf("video/* 次优先: %+v", got)
	}
	if got := pickArtifactRow([]*artifactRow{cover}, ""); got != cover {
		t.Fatalf("回落首个: %+v", got)
	}
	if got := pickArtifactRow([]*artifactRow{cover, clip}, "cover"); got != cover {
		t.Fatalf("指定 key 精确匹配: %+v", got)
	}
	if got := pickArtifactRow([]*artifactRow{cover}, "missing"); got != nil {
		t.Fatalf("指定 key 不存在应返回 nil: %+v", got)
	}
}

func TestArtifactRowDeclPath(t *testing.T) {
	cases := []struct {
		row  artifactRow
		want string
	}{
		{artifactRow{SourcePath: "_video_url", SourceUrl: "https://x/v.mp4"}, "_video_url"},
		{artifactRow{SourceUrl: "data.videos[0].url"}, "data.videos[0].url"}, // 存量行：source_url 存路径
		{artifactRow{SourceUrl: "https://x/v.mp4"}, ""},                      // 存量行已改写为 URL
		{artifactRow{SourceUrl: "/v1/videos/1/content"}, ""},
	}
	for _, c := range cases {
		if got := c.row.declPath(); got != c.want {
			t.Errorf("declPath(%+v) = %q want %q", c.row, got, c.want)
		}
	}
}

func TestOpenFromSourceInline(t *testing.T) {
	payload := []byte("fake-png-bytes")
	task := &rcommon.AsyncTask{
		ID:   1,
		Data: []byte(`{"data":{"b64":"` + base64.StdEncoding.EncodeToString(payload) + `","empty":""}}`),
	}
	art, err := openFromSource(context.Background(), task, &artifactRow{
		ArtifactKey: "image", SourceType: "inline", SourcePath: "data.b64", MimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("inline 回源失败: %v", err)
	}
	defer art.Body.Close()
	got, _ := io.ReadAll(art.Body)
	if string(got) != string(payload) || art.ContentType != "image/png" || art.Size != int64(len(payload)) {
		t.Fatalf("inline 内容异常: %q %s %d", got, art.ContentType, art.Size)
	}
	// 转存后 base64 已清空：如实报错
	if _, err := openFromSource(context.Background(), task, &artifactRow{
		ArtifactKey: "image", SourceType: "inline", SourcePath: "data.empty",
	}); err == nil {
		t.Fatal("空 inline 值应报错")
	}
}

func TestOpenFromSourceURLRejectsPrivateTarget(t *testing.T) {
	// 插件声明的绝对外链经 SSRF 预检：内网目标拒绝回源
	task := &rcommon.AsyncTask{ID: 1, Data: []byte(`{"_video_url":"http://127.0.0.1:9/v.mp4"}`)}
	if _, err := openFromSource(context.Background(), task, &artifactRow{
		ArtifactKey: "video", SourceType: "url", SourcePath: "_video_url",
	}); err == nil {
		t.Fatal("内网外链应被 SSRF 预检拒绝")
	}
}

func TestDeliveryPending(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	oldNow := nowFn
	nowFn = func() time.Time { return now }
	t.Cleanup(func() { nowFn = oldNow })

	justDone := now.Add(-time.Minute)
	longAgo := now.Add(-deliveryWaitLimit - time.Minute)
	task := &rcommon.AsyncTask{FinishTime: &justDone}
	row := func(status string, retry int) *artifactRow {
		return &artifactRow{ArtifactKey: "video", Status: status, RetryCount: retry}
	}
	cases := []struct {
		name string
		task *rcommon.AsyncTask
		rows []*artifactRow
		want bool
	}{
		{"无产物", task, nil, false},
		{"待转存", task, []*artifactRow{row("pending", 0)}, true},
		{"转存中", task, []*artifactRow{row("running", 0)}, true},
		{"失败可重试", task, []*artifactRow{row("failed", 2)}, true},
		{"重试耗尽走回源", task, []*artifactRow{row("failed", maxRetryCount)}, false},
		{"已转存", task, []*artifactRow{row("stored", 0)}, false},
		{"关闭转存", task, []*artifactRow{row("direct", 0)}, false},
		{"多产物任一在途", task, []*artifactRow{row("stored", 0), row("pending", 0)}, true},
		{"超等待上限放行", &rcommon.AsyncTask{FinishTime: &longAgo}, []*artifactRow{row("pending", 0)}, false},
	}
	for _, c := range cases {
		if got := deliveryPending(c.task, c.rows); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestResolveDeliveryNonJSTask(t *testing.T) {
	stubSigning(t, time.Unix(1_800_000_000, 0))
	ctx := context.Background()
	// 非 JS 任务不查登记：有 ResultURL 即就绪并签发链接，无则空
	view := ResolveDelivery(ctx, &rcommon.AsyncTask{PublicTaskID: "task_x", Status: "SUCCESS", Platform: "kling", ResultURL: "https://cdn/x.mp4"})
	if view.Pending || !strings.HasPrefix(view.ContentURL, "https://api.example.com/v1/tasks/task_x/content?") {
		t.Fatalf("非 JS 任务交付视图异常: %+v", view)
	}
	if view := ResolveDelivery(ctx, &rcommon.AsyncTask{Status: "SUCCESS", Platform: "kling"}); view.Pending || view.ContentURL != "" {
		t.Fatalf("无结果地址不应签发链接: %+v", view)
	}
	if view := ResolveDelivery(ctx, &rcommon.AsyncTask{Status: "IN_PROGRESS", Platform: "js:myapi", ResultURL: "https://cdn/x.mp4"}); view != (rcommon.TaskDelivery{}) {
		t.Fatalf("非成功任务应为零值: %+v", view)
	}
}
