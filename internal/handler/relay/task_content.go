package relay

import (
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	taskartifact "github.com/qianfree/team-api/internal/logic/task/artifact"
	relay_handler "github.com/qianfree/team-api/relay/handler"
)

func init() {
	// 标准任务端点（/v1/videos、/v1/video/generations 查询）：转存 OSS 完成后才给
	// 完成态，就绪时输出统一内容链接
	relay_handler.SetTaskDeliveryResolver(taskartifact.ResolveDelivery)
}

// HandleTaskContent 统一内容链接（GET /v1/tasks/{task_id}/content[/{artifact_key}]）。
// 免 API Key：凭 expires + signature（平台签发，见 artifact.SignContentURL）访问，
// 可直接用于浏览器 / <video> 标签 / 下游拉取。签名绑定任务 ID + 产物键 + 过期时刻，
// 链接本身即授权凭据，不再做租户校验。
func HandleTaskContent(r *ghttp.Request) {
	ctx := r.Context()
	taskID := r.Get("task_id").String()
	key := r.Get("artifact_key").String()
	if taskID == "" {
		writeVideosError(r, http.StatusBadRequest, "task id is required", "")
		return
	}

	err := taskartifact.VerifyContentURL(ctx, taskID, key, r.GetQuery("expires").String(), r.GetQuery("signature").String())
	switch {
	case errors.Is(err, taskartifact.ErrContentURLExpired):
		writeVideosError(r, http.StatusForbidden, "content url has expired, query the task again for a fresh url", "content_url_expired")
		return
	case errors.Is(err, taskartifact.ErrContentURLInvalid):
		writeVideosError(r, http.StatusForbidden, "invalid content url signature", "content_url_invalid")
		return
	case err != nil:
		g.Log().Errorf(ctx, "HandleTaskContent: verify signature failed, taskID=%s, err=%v", taskID, err)
		writeVideosError(r, http.StatusInternalServerError, "verify content url failed", "")
		return
	}

	task, err := taskDataProvider.GetTaskByPublicID(ctx, taskID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleTaskContent: query task failed, taskID=%s, err=%v", taskID, err)
		writeVideosError(r, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeVideosError(r, http.StatusNotFound, "No task found with id '"+taskID+"'.", "")
		return
	}
	if task.Status != "SUCCESS" {
		writeVideosError(r, http.StatusBadRequest, "content is not available until the task is completed", "content_not_ready")
		return
	}
	relay_handler.ServeTaskContent(ctx, r.Response.Writer, task, key, taskDataProvider)
}
