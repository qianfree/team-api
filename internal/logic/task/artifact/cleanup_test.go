package artifact

import (
	"context"
	"errors"
	"testing"

	rcommon "github.com/qianfree/team-api/relay/common"
)

// Open 对 expired 行必须返回哨兵错误（内容端点据此 410），不得回落回源——
// inline 的 base64 已清空、上游 URL 大概率失效，回源只会得到含混失败。
func TestOpenExpiredArtifact(t *testing.T) {
	oldLoad := loadArtifactRows
	loadArtifactRows = func(ctx context.Context, taskID, tenantID int64) ([]*artifactRow, error) {
		return []*artifactRow{
			{ArtifactKey: "video", SourceType: "url", SourcePath: "results[0].url", Status: statusExpired},
		}, nil
	}
	t.Cleanup(func() { loadArtifactRows = oldLoad })

	task := &rcommon.AsyncTask{ID: 1, TenantID: 1, PublicTaskID: "task_x"}
	_, found, err := Open(context.Background(), task, "")
	if !found {
		t.Fatalf("expired 行应视为 found（区别于无登记产物）")
	}
	if !errors.Is(err, rcommon.ErrArtifactExpired) {
		t.Fatalf("expired 行应返回 ErrArtifactExpired，got: %v", err)
	}
	// 指定 key 命中 expired 行同样 410
	_, found, err = Open(context.Background(), task, "video")
	if !found || !errors.Is(err, rcommon.ErrArtifactExpired) {
		t.Fatalf("指定 key 的 expired 行应返回 ErrArtifactExpired，found=%v err=%v", found, err)
	}
}

// 保留期关闭（<=0）时清理必须是纯 no-op——早于任何 DB/存储访问返回。
func TestCleanupDisabledNoop(t *testing.T) {
	oldRetention := retentionDaysFn
	retentionDaysFn = func(context.Context) int { return 0 }
	t.Cleanup(func() { retentionDaysFn = oldRetention })

	n, err := CleanupExpiredArtifacts(context.Background())
	if err != nil {
		t.Fatalf("关闭状态下不应报错: %v", err)
	}
	if n != 0 {
		t.Fatalf("关闭状态下不应清理任何行，got %d", n)
	}
}

// 过期行不参与交付等待判定：deliveryPending 不把 expired 视为转存途中，
// 任务查询正常出完成态（链接打开时 410，由 Open 哨兵保证）。
func TestDeliveryPendingIgnoresExpired(t *testing.T) {
	rows := []*artifactRow{
		{ArtifactKey: "video", SourceType: "url", Status: statusExpired, RetryCount: maxRetryCount + 1},
	}
	task := &rcommon.AsyncTask{ID: 1, TenantID: 1, Status: string(rcommon.TaskStatusSuccess)}
	if deliveryPending(task, rows) {
		t.Fatalf("expired 行不应使交付停留在 pending")
	}
}
