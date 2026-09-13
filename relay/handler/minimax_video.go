package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	taskminimax "github.com/qianfree/team-api/relay/taskchannel/minimax"
)

// MiniMax 官方视频协议（/v2）核心处理：查询回放、任务取消。
// 协议规格见 docs/modeldocs/minimax/video/（02-端点参考.md、04-响应对象.md）。
// 状态机：queued/running/succeeded/failed/cancelled（网关不主动产生 cancelled 任务；
// 上游任务被取消时经轮询映射为 FAILURE，retrieve 回放为 failed）。
// 错误用 OpenAI 风格 OaiError（writeTaskError 输出同构，官方 v2 即此风格）。
//
// DELETE 端点语义（有意与官方分歧）：
//   - 仅支持取消 queued 任务（取消结果以远程返回为准，上游确认才生效）；
//   - 不支持删除任务记录（官方对终态任务的 delete 记录能力网关不提供，软删也不做），
//     网关保留任务数据用于计费与审计；
//   - 任务终态后 content.url 不再回源刷新（回放缓存的上游直链，有时效）。

// minimaxStatusFromTask 平台任务状态 → 官方状态机映射
func minimaxStatusFromTask(status string) string {
	switch status {
	case "IN_PROGRESS":
		return "running"
	case "SUCCESS":
		return "succeeded"
	case "FAILURE":
		return "failed"
	default:
		// NOT_START / SUBMITTED / QUEUED 及一切未知中间态都归入 queued
		return "queued"
	}
}

// isMiniMaxTerminalTask 判断是否终态（succeeded/failed 对应的平台态）
func isMiniMaxTerminalTask(status string) bool {
	return status == string(common.TaskStatusSuccess) || status == string(common.TaskStatusFailure)
}

// parseMiniMaxUpstreamTask 解析 task.Data 中缓存的上游单任务查询响应。
// 提交后首拍轮询前 Data 是提交响应 {"task_id":...} 形态，解析不出 task 时返回 nil（静默降级）。
func parseMiniMaxUpstreamTask(data []byte) *taskminimax.VideoTask {
	if len(data) == 0 {
		return nil
	}
	var resp taskminimax.TaskQueryResponse
	if err := json.Unmarshal(data, &resp); err != nil || resp.Task == nil {
		return nil
	}
	return resp.Task
}

// buildMiniMaxVideoTask 平台任务 → 官方 VideoTask 对象。
// 任务记录是 id/status/model/时间戳的真相源；usage/resolution/duration/ratio 从
// 缓存的上游查询响应（task.Data）富集，不可解析时缺省不输出。
func buildMiniMaxVideoTask(task *common.AsyncTask) *taskminimax.VideoTask {
	obj := &taskminimax.VideoTask{
		ID:        task.PublicTaskID,
		Model:     task.ModelName,
		Status:    minimaxStatusFromTask(task.Status),
		CreatedAt: task.CreatedAt.Unix(),
		UpdatedAt: task.UpdatedAt.Unix(),
		TaskType:  "generation",
		Modality:  "video",
	}

	if obj.Status == "failed" {
		message := task.FailReason
		if message == "" {
			message = "video generation failed"
		}
		obj.Error = &taskminimax.VideoTaskError{Code: "video_generation_failed", Message: message}
	}
	if task.ResultURL != "" {
		obj.Content = &taskminimax.VideoTaskContent{URL: task.ResultURL}
	}

	if upstream := parseMiniMaxUpstreamTask(task.Data); upstream != nil {
		if upstream.Usage != nil {
			obj.Usage = upstream.Usage
		}
		if upstream.Resolution != "" {
			obj.Resolution = upstream.Resolution
		}
		if upstream.Duration > 0 {
			obj.Duration = upstream.Duration
		}
		if upstream.Ratio != "" {
			obj.Ratio = upstream.Ratio
		}
		// 成品直链以任务记录的 ResultURL 为准（轮询结算时写入），上游回放仅兜底
		if obj.Content == nil && upstream.Content != nil {
			obj.Content = upstream.Content
		}
	}

	return obj
}

// writeMiniMaxSubmitResponse 提交成功响应：官方 VideoGenerationV2Resp 形态
func writeMiniMaxSubmitResponse(w http.ResponseWriter, publicTaskID string) {
	writeJSON(w, http.StatusOK, map[string]any{"task_id": publicTaskID})
}

// fetchMiniMaxTaskById 查询任务并做租户双键隔离 + 平台门禁。
// 平台门禁：/v2 端点只服务 minimax 平台任务，防止异构平台任务 ID 被塞进官方响应结构回放。
func fetchMiniMaxTaskById(
	ctx context.Context,
	taskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) (*common.AsyncTask, bool) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, taskID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "MiniMaxVideo: query task failed, taskID=%s, err=%v", taskID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return nil, false
	}
	if task == nil || task.Platform != string(constant.TaskPlatformMiniMax) {
		writeTaskError(rc.Writer, http.StatusNotFound, "No MiniMax video task found with id '"+taskID+"'.", "")
		return nil, false
	}
	return task, true
}

// HandleMiniMaxVideoRetrieve GET /v2/query/video_generation/{task_id} 查询管线。
// 响应为官方 GetVideoGenerationV2Resp 形态（{"task": {...}} 外层包装）。
func HandleMiniMaxVideoRetrieve(
	ctx context.Context,
	taskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, ok := fetchMiniMaxTaskById(ctx, taskID, rc, dataProvider)
	if !ok {
		return
	}
	writeJSON(rc.Writer, http.StatusOK, taskminimax.TaskQueryResponse{Task: buildMiniMaxVideoTask(task)})
}

// HandleMiniMaxVideoCancel DELETE /v2/video_generation/{task_id} 取消管线。
// 仅支持取消排队中的任务，不支持删除任务记录（含软删，网关保留计费与审计数据）。
//
// 取消结果以远程返回为准：调用上游 DELETE，上游确认 action=cancelled 才向客户端
// 返回成功；上游拒绝（如任务已进入 running）时原样透传上游错误。
// 本地任务状态不在此处改写——轮询循环下一拍从上游查询到 cancelled → FAILURE → 退款，
// 预扣退款与活跃任务计数等记账全部走既有轮询链路，避免在本端点重复实现结算。
func HandleMiniMaxVideoCancel(
	ctx context.Context,
	taskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, ok := fetchMiniMaxTaskById(ctx, taskID, rc, dataProvider)
	if !ok {
		return
	}

	// 终态任务既不可取消也不可删除（删除能力网关不提供）
	if isMiniMaxTerminalTask(task.Status) {
		writeTaskError(rc.Writer, http.StatusBadRequest,
			"task is already completed; only queued tasks can be cancelled", "task_not_cancellable")
		return
	}
	// 运行中的任务上游不允许取消，前置拦截省一次上游调用（本地状态若滞后，由上游裁决兜底）
	if task.Status == string(common.TaskStatusInProgress) {
		writeTaskError(rc.Writer, http.StatusBadRequest, "running tasks cannot be cancelled", "task_not_cancellable")
		return
	}
	// v1（Hailuo 系列）上游没有取消端点
	upstreamModel := task.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = task.ModelName
	}
	if !taskminimax.IsV2Model(upstreamModel) {
		writeTaskError(rc.Writer, http.StatusBadRequest,
			"v1 video tasks cannot be cancelled; the MiniMax v1 API has no cancel endpoint", "task_not_cancellable")
		return
	}

	// 提取上游任务 ID
	var pd struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := json.Unmarshal(task.PrivateData, &pd); err != nil || pd.UpstreamTaskID == "" {
		g.Log().Errorf(ctx, "MiniMaxVideoCancel: invalid private data, taskID=%s, err=%v", taskID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "invalid task data", "")
		return
	}

	// 取渠道信息（上游地址与密钥），对齐 HandleMjImageProxy 的任务→渠道回查模式
	channel, err := dataProvider.GetChannelByID(ctx, task.ChannelID)
	if err != nil || channel == nil {
		g.Log().Errorf(ctx, "MiniMaxVideoCancel: channel not found, channelID=%d, err=%v", task.ChannelID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "channel not found", "")
		return
	}

	result, taskErr := taskminimax.CancelTask(channel.BaseURL, channel.ApiKey, pd.UpstreamTaskID, parseMiniMaxUseProxy(channel.Settings))
	if taskErr != nil {
		// 远程取消失败（含上游判定运行中/已结束不可取消）：以远程结论为准
		g.Log().Infof(ctx, "MiniMaxVideoCancel: upstream rejected, taskID=%s, status=%d, message=%q", taskID, taskErr.StatusCode, taskErr.Message)
		writeTaskError(rc.Writer, taskErr.StatusCode, taskErr.Message, taskErr.ErrCode)
		return
	}
	// 防御竞态：本地检查后任务已在上游完成时，上游会执行删记录（action=deleted）而非取消
	if result.Action != "cancelled" {
		g.Log().Infof(ctx, "MiniMaxVideoCancel: upstream action=%s (not cancelled), taskID=%s", result.Action, taskID)
		writeTaskError(rc.Writer, http.StatusBadRequest,
			"task is no longer queued; only queued tasks can be cancelled", "task_not_cancellable")
		return
	}

	g.Log().Infof(ctx, "MiniMaxVideoCancel: task cancelled upstream, taskID=%s, upstreamTaskID=%s, tenant=%d, user=%d",
		taskID, pd.UpstreamTaskID, rc.TenantID, rc.UserID)

	// 官方 DeleteVideoGenerationV2Resp 形态（取消分支）
	writeJSON(rc.Writer, http.StatusOK, map[string]any{
		"task_id": task.PublicTaskID,
		"action":  "cancelled",
		"status":  "cancelled",
	})
}

// parseMiniMaxUseProxy 从渠道 settings JSONB 解析代理开关
func parseMiniMaxUseProxy(settings json.RawMessage) bool {
	var s struct {
		UseProxy bool `json:"use_proxy"`
	}
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &s)
	}
	return s.UseProxy
}

// ==================== v1（Hailuo 系列）官方协议 ====================

// minimaxV1StatusFromTask 平台任务状态 → v1 官方状态机映射
// （Preparing/Queueing/Processing/Success/Fail）
func minimaxV1StatusFromTask(status string) string {
	switch status {
	case "QUEUED":
		return "Queueing"
	case "IN_PROGRESS":
		return "Processing"
	case "SUCCESS":
		return "Success"
	case "FAILURE":
		return "Fail"
	default:
		// NOT_START / SUBMITTED 及未知早期状态
		return "Preparing"
	}
}

// writeMiniMaxV1SubmitResponse v1 提交成功响应：官方 VideoGenerationResp 形态
// （含 base_resp 成功信封）
func writeMiniMaxV1SubmitResponse(w http.ResponseWriter, publicTaskID string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"task_id":   publicTaskID,
		"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
	})
}

// parseMiniMaxV1UpstreamData 解析 task.Data 中缓存的上游 v1 查询响应（含 FetchTask
// 二跳合并的 download_url）。非 v1 形态（v2 的 {"task":...} 包装或提交响应）返回 nil。
func parseMiniMaxV1UpstreamData(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if _, hasTask := m["task"]; hasTask {
		return nil // v2 形态
	}
	if _, hasStatus := m["status"]; !hasStatus {
		return nil // 提交响应等非查询形态
	}
	return m
}

// HandleMiniMaxVideoV1Retrieve GET /v1/query/video_generation?task_id= 查询管线。
// 响应为官方 QueryVideoGenerationTaskResp 形态；附加 download_url（FetchTask 二跳解析的
// 限时直链，官方查询响应无此字段，属网关增补）与 error（失败原因）两个增补字段，
// 官方 SDK 忽略未知字段，不影响兼容。
func HandleMiniMaxVideoV1Retrieve(
	ctx context.Context,
	taskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, ok := fetchMiniMaxTaskById(ctx, taskID, rc, dataProvider)
	if !ok {
		return
	}

	resp := map[string]any{
		"task_id":   task.PublicTaskID,
		"status":    minimaxV1StatusFromTask(task.Status),
		"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
	}
	if task.Status == string(common.TaskStatusFailure) && task.FailReason != "" {
		resp["error"] = task.FailReason
	}

	// 从缓存的上游查询响应富集 file_id/video_width/video_height/download_url
	if upstream := parseMiniMaxV1UpstreamData(task.Data); upstream != nil {
		for _, key := range []string{"file_id", "video_width", "video_height", "download_url"} {
			if v, has := upstream[key]; has {
				resp[key] = v
			}
		}
	}
	// 成品直链以任务记录的 ResultURL 为准（轮询结算时写入），上游回放仅兜底
	if task.ResultURL != "" {
		resp["download_url"] = task.ResultURL
	}

	writeJSON(rc.Writer, http.StatusOK, resp)
}
