package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
)

// OpenAI Videos 协议（/v1/videos）核心处理：Video 对象构建、retrieve / delete / content。
// 协议规格见 docs/协议文档/OpenAI/OpenAI-Videos-API文档.md。
// 注意状态机只有 queued/in_progress/completed/failed 四值（无 cancelled），
// 错误分两层：请求层用 HTTP 错误（writeTaskError，OpenAI 格式），
// 生成层失败用 200 + status:"failed" + 内嵌 error 对象表达。

// videosProtocolOpenAI TaskRelayContext.Protocol 的 OpenAI Videos 协议标识
const videosProtocolOpenAI = "openai_videos"

// videosPollAfterMs 非终态任务 retrieve 响应头 openai-poll-after-ms 的建议轮询间隔（毫秒）。
// 官方 SDK poll() 优先读取该响应头，缺省时 SDK 自取 1000ms；视频任务耗时长，放慢到 2s。
const videosPollAfterMs = "2000"

// videosContentHTTPClient 成品内容回源拉流的 HTTP 客户端（大文件流式转发，整体超时兜底）
var videosContentHTTPClient = &http.Client{Timeout: 120 * time.Second}

// videoErrorObject 官方 VideoCreateError 的简化形态（misalignment 为 Sora 内容安全特有，第三方上游无对应，不透出）
type videoErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// videoObject 官方 Video 对象。可空字段按官方 schema 输出 null（非省略）。
type videoObject struct {
	ID                 string            `json:"id"`
	Object             string            `json:"object"` // 恒为 "video"
	CreatedAt          int64             `json:"created_at"`
	CompletedAt        *int64            `json:"completed_at"`
	Status             string            `json:"status"` // queued / in_progress / completed / failed
	Progress           int               `json:"progress"`
	Model              string            `json:"model"`
	Prompt             *string           `json:"prompt"`
	Seconds            string            `json:"seconds,omitempty"`
	Size               string            `json:"size,omitempty"`
	Error              *videoErrorObject `json:"error"`
	RemixedFromVideoID *string           `json:"remixed_from_video_id"`
	ExpiresAt          *int64            `json:"expires_at"`
}

// videosStatusFromTask 平台任务状态 → 官方四值状态机映射
func videosStatusFromTask(status string) string {
	switch status {
	case "IN_PROGRESS":
		return "in_progress"
	case "SUCCESS":
		return "completed"
	case "FAILURE":
		return "failed"
	default:
		// NOT_START / SUBMITTED / QUEUED 及一切未知中间态都归入 queued
		return "queued"
	}
}

// videosProgressFromTask 平台进度（"42%" 字符串）→ 官方整数百分比。
// 解析失败时按状态给粗粒度值（第三方上游无进度信息时的兜底）。
func videosProgressFromTask(task *common.AsyncTask) int {
	if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(task.Progress, "%"))); err == nil {
		if n < 0 {
			return 0
		}
		if n > 100 {
			return 100
		}
		return n
	}
	switch videosStatusFromTask(task.Status) {
	case "completed":
		return 100
	case "in_progress":
		return 50
	default:
		return 0
	}
}

// videosEchoFromTask 从 PrivateData.request_echo 提取提交时的请求回显
func videosEchoFromTask(task *common.AsyncTask) videosRequestEcho {
	var echo videosRequestEcho
	if len(task.PrivateData) == 0 {
		return echo
	}
	var pd struct {
		RequestEcho *videosRequestEcho `json:"request_echo"`
	}
	if err := json.Unmarshal(task.PrivateData, &pd); err == nil && pd.RequestEcho != nil {
		echo = *pd.RequestEcho
	}
	return echo
}

// buildVideoObject 平台任务 → 官方 Video 对象
func buildVideoObject(task *common.AsyncTask) *videoObject {
	obj := &videoObject{
		ID:        task.PublicTaskID,
		Object:    "video",
		CreatedAt: task.CreatedAt.Unix(),
		Status:    videosStatusFromTask(task.Status),
		Progress:  videosProgressFromTask(task),
		Model:     task.ModelName,
	}

	echo := videosEchoFromTask(task)
	prompt := echo.Prompt
	obj.Prompt = &prompt
	obj.Seconds = echo.Seconds
	obj.Size = echo.Size

	if task.FinishTime != nil {
		completedAt := task.FinishTime.Unix()
		obj.CompletedAt = &completedAt
	}
	if obj.Status == "failed" {
		message := task.FailReason
		if message == "" {
			message = "video generation failed"
		}
		obj.Error = &videoErrorObject{Code: "video_generation_failed", Message: message}
	}
	return obj
}

// writeVideosSubmitResponse 提交成功响应：官方 Video 对象（初始 queued / progress 0）
func writeVideosSubmitResponse(w http.ResponseWriter, publicTaskID, modelName string, now time.Time, echo any) {
	obj := &videoObject{
		ID:        publicTaskID,
		Object:    "video",
		CreatedAt: now.Unix(),
		Status:    "queued",
		Progress:  0,
		Model:     modelName,
	}
	if echo != nil {
		if e, ok := echo.(videosRequestEcho); ok {
			prompt := e.Prompt
			obj.Prompt = &prompt
			obj.Seconds = e.Seconds
			obj.Size = e.Size
		}
	}
	writeJSON(w, http.StatusOK, obj)
}

// HandleVideosRetrieve GET /v1/videos/{video_id} 查询管线。
// 租户双键隔离（user_id + tenant_id）复用 GetTaskByPublicIDAndUser；软删除任务已被过滤，天然 404。
func HandleVideosRetrieve(
	ctx context.Context,
	videoID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, videoID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleVideosRetrieve: query task failed, videoID=%s, err=%v", videoID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeTaskError(rc.Writer, http.StatusNotFound, "No video found with id '"+videoID+"'.", "")
		return
	}

	obj := buildVideoObject(task)
	// 非终态附带官方轮询提示头（SDK poll() 优先读取）
	if obj.Status == "queued" || obj.Status == "in_progress" {
		rc.Writer.Header().Set("openai-poll-after-ms", videosPollAfterMs)
	}
	writeJSON(rc.Writer, http.StatusOK, obj)
}

// HandleVideosDelete DELETE /v1/videos/{video_id} 删除管线。
// 对齐官方语义「永久删除已完成/失败的视频及其资产」：仅终态可删；
// 网关侧软删除保留计费与审计记录，删除后查询/下载均按 404 处理（幂等）。
func HandleVideosDelete(
	ctx context.Context,
	videoID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, videoID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleVideosDelete: query task failed, videoID=%s, err=%v", videoID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeTaskError(rc.Writer, http.StatusNotFound, "No video found with id '"+videoID+"'.", "")
		return
	}

	// 仅终态（completed/failed）可删，进行中任务官方亦不允许删除
	if videosStatusFromTask(task.Status) != "completed" && videosStatusFromTask(task.Status) != "failed" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "only completed or failed videos can be deleted", "video_not_deletable")
		return
	}
	// 未结算的终态任务删除会造成退款悬空：结算由轮询 cron 兜底完成后即可删除
	if !task.BillingSettled {
		writeTaskError(rc.Writer, http.StatusBadRequest, "video billing is not settled yet, retry later", "billing_pending")
		return
	}

	if err := dataProvider.SoftDeleteTask(ctx, task); err != nil {
		g.Log().Errorf(ctx, "HandleVideosDelete: soft delete failed, videoID=%s, err=%v", videoID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "delete video failed", "")
		return
	}
	g.Log().Infof(ctx, "HandleVideosDelete: video deleted, videoID=%s, tenant=%d, user=%d", videoID, rc.TenantID, rc.UserID)

	writeJSON(rc.Writer, http.StatusOK, map[string]any{
		"id":      task.PublicTaskID,
		"object":  "video.deleted",
		"deleted": true,
	})
}

// HandleVideosContent GET /v1/videos/{video_id}/content 成品下载管线。
// 从任务 ResultURL 回源拉流并转发二进制（不透出上游 CDN 直链）。
// 仅支持官方默认 variant=video；thumbnail/spritesheet 第三方上游无此资产。
func HandleVideosContent(
	ctx context.Context,
	videoID string,
	variant string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	if variant != "" && variant != "video" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "unsupported content variant: "+variant, "unsupported_variant")
		return
	}

	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, videoID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "HandleVideosContent: query task failed, videoID=%s, err=%v", videoID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil {
		writeTaskError(rc.Writer, http.StatusNotFound, "No video found with id '"+videoID+"'.", "")
		return
	}

	// 官方语义：仅 completed 可下载（未完成/失败均报错）
	if task.Status != "SUCCESS" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "video content is not available until the video is completed", "video_not_ready")
		return
	}
	// 部分上游（如官方 sora）不提供成品直链，需要二跳 content 拉取——当前不支持，明确报错
	if task.ResultURL == "" {
		writeTaskError(rc.Writer, http.StatusBadRequest, "video content is not available for this task", "content_not_available")
		return
	}

	// ResultURL 来自上游响应持久化数据，仍做 scheme 白名单防异构值
	resultURL, err := url.Parse(task.ResultURL)
	if err != nil || (resultURL.Scheme != "http" && resultURL.Scheme != "https") {
		g.Log().Errorf(ctx, "HandleVideosContent: invalid result url, videoID=%s, url=%q, err=%v", videoID, task.ResultURL, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "invalid video result url", "")
		return
	}

	resp, err := videosContentHTTPClient.Get(task.ResultURL)
	if err != nil {
		// 上游拉流失败属预期内运营事件，用 Warningf 避免自动打栈（对齐任务提交链路日志分级）
		g.Log().Warningf(ctx, "HandleVideosContent: fetch upstream failed, videoID=%s, err=%v", videoID, err)
		writeTaskError(rc.Writer, http.StatusBadGateway, "failed to fetch video content", "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		g.Log().Warningf(ctx, "HandleVideosContent: upstream status=%d, videoID=%s", resp.StatusCode, videoID)
		writeTaskError(rc.Writer, http.StatusBadGateway, "failed to fetch video content", "")
		return
	}

	// 透传内容元信息后流式转发字节
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		rc.Writer.Header().Set("Content-Type", ct)
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		rc.Writer.Header().Set("Content-Length", cl)
	}
	rc.Writer.WriteHeader(http.StatusOK)
	if _, err := io.Copy(rc.Writer, resp.Body); err != nil {
		// 响应头已发出，只能记录日志，无法再改写为错误响应
		g.Log().Warningf(ctx, "HandleVideosContent: stream copy failed, videoID=%s, err=%v", videoID, err)
	}
}
