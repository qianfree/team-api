package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 阿里 DashScope 官方视频协议（POST /api/v1/services/aigc/video-generation/video-synthesis）入站转换层。
// 原生请求体（model/input.prompt/input.media[]/parameters）解析校验后转成任务框架通用请求体，
// 复用任务提交管线（selectTaskChannel 渠道调度 + HandleTaskSubmit 计费/提交/落库）；
// 查询端点 GET /api/v1/tasks/{task_id} 回放 DashScope 官方响应形态（协议文档
// docs/协议文档/阿里系列/视频-wan3.0.md），阿里 dashscope SDK 换 base_url 即可直连。
//
// 转换产物在保留原生字段（media[]/negative_prompt/audio_url）之外，双写通用词汇
// （prompt/seconds/metadata）——参数倍率规则的匹配源是归一化任务体（与 MiniMax 官方
// 协议入站同一约定），保证按通用路径配置的倍率规则对官方协议入站同样生效。

// aliVideoProtocol TaskRelayContext.Protocol 的阿里 DashScope 官方视频协议标识
const aliVideoProtocol = "ali_native"

// aliVideoCreateRequest DashScope 官方创建请求的归一形态（JSON 解析后的产物）
type aliVideoCreateRequest struct {
	Model          string
	Prompt         string
	NegativePrompt string
	AudioURL       string
	Media          []aliVideoMedia
	Parameters     aliVideoParameters
}

// aliVideoMedia wan3.0 input.media 数组元素
type aliVideoMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// aliVideoParameters DashScope parameters（wan2.x 与 wan3.0 字段并集）
type aliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`
	Ratio        string `json:"ratio,omitempty"`
	Size         string `json:"size,omitempty"`
	Duration     *int   `json:"duration,omitempty"` // wan3.0：-1 = 智能时长
	Audio        *bool  `json:"audio,omitempty"`    // wan3.0：输出音频开关
	Seed         *int   `json:"seed,omitempty"`
	PromptExtend *bool  `json:"prompt_extend,omitempty"`
	Watermark    *bool  `json:"watermark,omitempty"`
}

// jsonAliVideoInput 官方请求的 input 节（wan2.x 扁平字段 + wan3.0 media 数组并集）
type jsonAliVideoInput struct {
	Prompt         string          `json:"prompt"`
	NegativePrompt string          `json:"negative_prompt"`
	AudioURL       string          `json:"audio_url"`
	Media          []aliVideoMedia `json:"media"`
}

// ParseAliVideoCreateRequest 解析官方创建请求。
// 返回的 TaskError 已携带面向客户端的 HTTP 状态码与错误信息。
func ParseAliVideoCreateRequest(body []byte) (*aliVideoCreateRequest, *common.TaskError) {
	var raw struct {
		Model      string             `json:"model"`
		Input      jsonAliVideoInput  `json:"input"`
		Parameters aliVideoParameters `json:"parameters"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}

	return &aliVideoCreateRequest{
		Model:          strings.TrimSpace(raw.Model),
		Prompt:         raw.Input.Prompt,
		NegativePrompt: raw.Input.NegativePrompt,
		AudioURL:       raw.Input.AudioURL,
		Media:          raw.Input.Media,
		Parameters:     raw.Parameters,
	}, nil
}

// ValidateAliVideoCreateRequest 请求必填校验（对齐官方约束：model 必填，prompt 与 media 必填其一）
func ValidateAliVideoCreateRequest(req *aliVideoCreateRequest) *common.TaskError {
	if req.Model == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if strings.TrimSpace(req.Prompt) == "" && len(req.Media) == 0 {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "one of input.prompt / input.media is required"}
	}
	return nil
}

// BuildAliVideoTaskBody 把官方创建请求转成任务框架通用请求体（JSON 字节）。
// media 数组放顶层原样透传（适配器 buildWan3Media 消费）；通用词汇（seconds/metadata）
// 双写保证参数倍率规则与 per_second 预扣信号（spec.duration/spec.resolution）同源。
func BuildAliVideoTaskBody(req *aliVideoCreateRequest) ([]byte, error) {
	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if len(req.Media) > 0 {
		body["media"] = req.Media
	}

	metadata := map[string]any{}
	if req.NegativePrompt != "" {
		body["negative_prompt"] = req.NegativePrompt
		metadata["negative_prompt"] = req.NegativePrompt
	}
	if req.AudioURL != "" {
		body["audio_url"] = req.AudioURL
		metadata["audio_url"] = req.AudioURL
	}

	p := req.Parameters
	if p.Resolution != "" {
		body["resolution"] = p.Resolution
		metadata["resolution"] = p.Resolution
	}
	if p.Ratio != "" {
		body["ratio"] = p.Ratio
		metadata["ratio"] = p.Ratio
	}
	if p.Size != "" {
		metadata["size"] = p.Size
	}
	// duration 保留原值（含 -1 智能时长信号，供适配器预扣按上限冻结）；正数另写通用 seconds
	if p.Duration != nil {
		metadata["duration"] = *p.Duration
		if *p.Duration > 0 {
			body["seconds"] = strconv.Itoa(*p.Duration)
		}
	}
	if p.Audio != nil {
		metadata["sound"] = map[bool]string{true: "on", false: "off"}[*p.Audio]
		metadata["generate_audio"] = *p.Audio
	}
	if p.Seed != nil {
		metadata["seed"] = *p.Seed
	}
	if p.PromptExtend != nil {
		metadata["prompt_extend"] = *p.PromptExtend
	}
	if p.Watermark != nil {
		metadata["watermark"] = *p.Watermark
	}
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal ali video task body: %w", err)
	}
	return data, nil
}

// ==================== 查询端点回放 ====================

// aliTaskResponse DashScope 任务查询响应形态（GET /api/v1/tasks/{task_id} 的回放产物）。
// usage 用 RawMessage 原样保留上游计量（duration/output_video_duration/fps/SR/ratio）
type aliTaskResponse struct {
	Output struct {
		TaskID        string `json:"task_id"`
		TaskStatus    string `json:"task_status"`
		SubmitTime    string `json:"submit_time,omitempty"`
		ScheduledTime string `json:"scheduled_time,omitempty"`
		EndTime       string `json:"end_time,omitempty"`
		OrigPrompt    string `json:"orig_prompt,omitempty"`
		VideoURL      string `json:"video_url,omitempty"`
		Code          string `json:"code,omitempty"`
		Message       string `json:"message,omitempty"`
	} `json:"output"`
	Usage     json.RawMessage `json:"usage,omitempty"`
	RequestID string          `json:"request_id"`
}

// aliStatusFromTask 平台任务状态 → DashScope 官方状态机映射
func aliStatusFromTask(status string) string {
	switch status {
	case "IN_PROGRESS":
		return "RUNNING"
	case "SUCCESS":
		return "SUCCEEDED"
	case "FAILURE":
		return "FAILED"
	default:
		// NOT_START / SUBMITTED / QUEUED 及一切未知中间态都归入 PENDING
		return "PENDING"
	}
}

// buildAliTaskResponse 平台任务 → DashScope 官方查询响应。
// 优先回放 task.Data 缓存的上游查询响应（含 usage/orig_prompt/时间线，保真度最高），
// 仅覆写 output.task_id 为平台公开任务 ID、task_status 以平台状态为准（上游响应原文
// 可能滞后一轮轮询）；task.Data 不可解析（提交后首拍轮询前为提交响应形态或空）时
// 构造最小响应。失败任务补 code/message（上游已带则保留）
func buildAliTaskResponse(task *common.AsyncTask) *aliTaskResponse {
	resp := &aliTaskResponse{}
	if len(task.Data) > 0 {
		_ = json.Unmarshal(task.Data, resp)
	}
	status := aliStatusFromTask(task.Status)

	resp.Output.TaskID = task.PublicTaskID
	resp.Output.TaskStatus = status
	resp.RequestID = task.RequestID

	if status == "FAILED" {
		if resp.Output.Code == "" {
			resp.Output.Code = "TaskFailed"
		}
		if resp.Output.Message == "" {
			resp.Output.Message = task.FailReason
			if resp.Output.Message == "" {
				resp.Output.Message = "video generation failed"
			}
		}
	}
	return resp
}

// HandleAliVideoRetrieve GET /api/v1/tasks/{task_id} 查询管线（回放 DashScope 响应形态）。
// 租户双键隔离（user_id + tenant_id）复用 GetTaskByPublicIDAndUser；平台门禁：该端点只服务
// ali 平台任务，防止异构平台任务 ID 被塞进 DashScope 响应结构回放（对齐 MiniMax 官方端点做法）
func HandleAliVideoRetrieve(
	ctx context.Context,
	taskID string,
	rc *TaskRelayContext,
	dataProvider common.TaskDataProvider,
) {
	task, err := dataProvider.GetTaskByPublicIDAndUser(ctx, taskID, rc.UserID, rc.TenantID)
	if err != nil {
		g.Log().Errorf(ctx, "AliVideoRetrieve: query task failed, taskID=%s, err=%v", taskID, err)
		writeTaskError(rc.Writer, http.StatusInternalServerError, "query task failed", "")
		return
	}
	if task == nil || task.Platform != string(constant.TaskPlatformAli) {
		writeTaskError(rc.Writer, http.StatusNotFound, "task not found", "")
		return
	}
	writeJSON(rc.Writer, http.StatusOK, buildAliTaskResponse(task))
}

// writeAliVideoSubmitResponse 提交成功响应：官方 DashScope 形态
// （{"output":{"task_status":"PENDING","task_id":...},"request_id":...}）
func writeAliVideoSubmitResponse(w http.ResponseWriter, publicTaskID, requestID string, now time.Time) {
	_ = now // 提交响应形态无时间字段，参数保留对齐其他协议 writer 的签名习惯
	writeJSON(w, http.StatusOK, map[string]any{
		"output": map[string]any{
			"task_status": "PENDING",
			"task_id":     publicTaskID,
		},
		"request_id": requestID,
	})
}
