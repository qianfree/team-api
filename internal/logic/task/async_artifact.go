package task

import (
	"context"

	"github.com/qianfree/team-api/internal/logic/task/artifact"
	"github.com/qianfree/team-api/relay/common"
)

var _ common.TaskContentOpener = (*AsyncProvider)(nil)

// OpenTaskContent 打开任务产物内容（委托产物流水线：已转存读对象存储，否则按声明回源）。
func (p *AsyncProvider) OpenTaskContent(ctx context.Context, task *common.AsyncTask, key string) (*common.StoredArtifact, bool, error) {
	return artifact.Open(ctx, task, key)
}
