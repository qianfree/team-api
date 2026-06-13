package kling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
)

const channelName = "Kling"

// ModelList 可灵视频生成模型列表
var ModelList = []string{
	"kling-v1",
	"kling-v1-5",
	"kling-v1-6",
	"kling-v2-master",
	"kling-v2-1-master",
	"kling-v2-5-turbo",
	"kling-v2-6",
	"kling-v3",
	"kling-video-o1",
}

func init() {
	taskchannel.Register(constant.ProviderKling, func() common.TaskAdaptor {
		return &KlingAdaptor{}
	})
}

// ==================== 请求/响应结构体 ====================

// klingRequest 可灵视频生成请求（文生视频 + 图生视频共用）
type klingRequest struct {
	ModelName      string         `json:"model_name"`
	Prompt         string         `json:"prompt,omitempty"`
	NegativePrompt string         `json:"negative_prompt,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Duration       string         `json:"duration,omitempty"`
	AspectRatio    string         `json:"aspect_ratio,omitempty"`
	CfgScale       *float64       `json:"cfg_scale,omitempty"`
	Image          string         `json:"image,omitempty"`
	ImageTail      string         `json:"image_tail,omitempty"`
	Sound          string         `json:"sound,omitempty"`
	CameraControl  map[string]any `json:"camera_control,omitempty"`
}

// klingMetadata 可灵 metadata 参数结构体（用于 UnmarshalMetadata 映射）
type klingMetadata struct {
	NegativePrompt string         `json:"negative_prompt,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Duration       any            `json:"duration,omitempty"` // string 或 number
	AspectRatio    string         `json:"aspect_ratio,omitempty"`
	CfgScale       *float64       `json:"cfg_scale,omitempty"`
	Image          string         `json:"image,omitempty"`
	ImageTail      string         `json:"image_tail,omitempty"`
	Sound          string         `json:"sound,omitempty"`
	CameraControl  map[string]any `json:"camera_control,omitempty"`
}

// klingSubmitResponse 可灵任务提交响应
type klingSubmitResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"data"`
	RequestID string `json:"request_id"`
}

// klingTaskResponse 可灵任务查询响应
type klingTaskResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID        string `json:"task_id"`
		TaskStatus    string `json:"task_status"`
		TaskStatusMsg string `json:"task_status_msg"`
		TaskResult    struct {
			Videos []struct {
				ID       string `json:"id"`
				URL      string `json:"url"`
				Duration string `json:"duration"`
			} `json:"videos"`
		} `json:"task_result"`
	} `json:"data"`
	RequestID string `json:"request_id"`
}

// ==================== Adaptor 实现 ====================

type KlingAdaptor struct {
	info     *common.RelayInfo
	taskType string // text2video / image2video / omni-video
}

func (a *KlingAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *KlingAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	return nil
}

func (a *KlingAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}

	// 高清模式加价
	if mode, ok := req["mode"].(string); ok && mode == "pro" {
		ratios["quality"] = 3.5
	}

	// 时长加价：10s = 2x
	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Duration float64 `json:"duration"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil && meta.Duration >= 10 {
		ratios["duration_multiplier"] = 2.0
	}
	if seconds, ok := req["seconds"].(string); ok {
		if d, err := parseInt(seconds); err == nil && d >= 10 {
			ratios["duration_multiplier"] = 2.0
		}
	}

	return ratios
}

func (a *KlingAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

// determineTaskType 根据请求内容判断任务类型
func determineTaskType(req map[string]any) string {
	// Omni-Video 模型
	if modelName, ok := req["model"].(string); ok {
		if strings.HasPrefix(modelName, "kling-video-o") || strings.HasPrefix(modelName, "kling-v3") {
			return "omni-video"
		}
	}
	// 有图片输入则 image2video
	metadata := taskchannel.ExtractMetadata(req)
	if _, hasImg := metadata["image"]; hasImg {
		return "image2video"
	}
	if _, hasImg := req["images"].([]any); hasImg {
		return "image2video"
	}
	return "text2video"
}

func (a *KlingAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	// 根据 model 名判断路径（URL 在 BuildRequestBody 阶段确定 taskType，此处用 model 前置判断）
	modelName := info.ChannelMeta.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}

	if strings.HasPrefix(modelName, "kling-video-o") || strings.HasPrefix(modelName, "kling-v3") {
		return fmt.Sprintf("%s/v1/videos/omni-video", baseURL), nil
	}

	// 默认 text2video，image2video 由 BuildRequestBody 设置 task_type 后 FetchTask 使用
	return fmt.Sprintf("%s/v1/videos/text2video", baseURL), nil
}

func (a *KlingAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	apiKey := info.ChannelMeta.ApiKey
	if strings.HasPrefix(apiKey, "sk-") {
		header.Set("Authorization", "Bearer "+apiKey)
	} else {
		token := buildKlingJWT(apiKey)
		header.Set("Authorization", "Bearer "+token)
	}
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *KlingAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	taskType := determineTaskType(req)
	kReq := klingRequest{}

	// 模型名
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		kReq.ModelName = info.ChannelMeta.UpstreamModelName
	} else if m, ok := req["model"].(string); ok {
		kReq.ModelName = m
	}

	// prompt
	if v, ok := req["prompt"].(string); ok {
		kReq.Prompt = v
	}

	// 从 metadata 提取参数
	metadata := taskchannel.ExtractMetadata(req)
	var meta klingMetadata
	if err := taskchannel.UnmarshalMetadata(metadata, &meta); err != nil {
		return nil, fmt.Errorf("parse kling metadata: %w", err)
	}
	kReq.NegativePrompt = meta.NegativePrompt
	kReq.Mode = meta.Mode
	kReq.AspectRatio = meta.AspectRatio
	kReq.CfgScale = meta.CfgScale
	kReq.Image = meta.Image
	kReq.ImageTail = meta.ImageTail
	kReq.Sound = meta.Sound
	kReq.CameraControl = meta.CameraControl
	// duration 支持 string 或 number 两种类型
	switch v := meta.Duration.(type) {
	case string:
		kReq.Duration = v
	case float64:
		kReq.Duration = fmt.Sprintf("%d", int(v))
	}

	// seconds 字段映射到 duration
	if seconds, ok := req["seconds"].(string); ok {
		kReq.Duration = seconds
	}

	// images 数组 → 取第一张作为 image
	if images, ok := req["images"].([]any); ok && len(images) > 0 {
		if url, ok := images[0].(string); ok && kReq.Image == "" {
			kReq.Image = url
		}
	}

	// 默认值
	if kReq.Mode == "" {
		kReq.Mode = "std"
	}
	if kReq.Duration == "" {
		kReq.Duration = "5"
	}

	// 记录 taskType 供 DoRequest 使用正确的 URL
	if taskType == "image2video" || taskType == "omni-video" {
		a.taskType = taskType
	}

	data, err := json.Marshal(kReq)
	if err != nil {
		return nil, fmt.Errorf("marshal kling request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *KlingAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")

	// 根据 taskType 决定 URL
	var reqURL string
	switch a.taskType {
	case "image2video":
		reqURL = fmt.Sprintf("%s/v1/videos/image2video", baseURL)
	case "omni-video":
		reqURL = fmt.Sprintf("%s/v1/videos/omni-video", baseURL)
	default:
		reqURL = fmt.Sprintf("%s/v1/videos/text2video", baseURL)
	}

	req, err := http.NewRequest(http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}
	client := &http.Client{Timeout: 120 * 1e9}
	return client.Do(req)
}

func (a *KlingAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}

	var result klingSubmitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
	}

	if result.Code != 0 {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    result.Message,
			ErrCode:    fmt.Sprintf("%d", result.Code),
		}
	}

	if result.Data.TaskID == "" {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
	}

	return result.Data.TaskID, body, nil
}

func (a *KlingAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID   string `json:"task_id"`
		TaskType string `json:"task_type"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("kling: invalid task data: %w", err)
	}

	taskType := data.TaskType
	if taskType == "" {
		taskType = "text2video"
	}

	url := fmt.Sprintf("%s/v1/videos/%s/%s", strings.TrimRight(baseURL, "/"), taskType, data.TaskID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(apiKey, "sk-") {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+buildKlingJWT(apiKey))
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * 1e9}
	return client.Do(req)
}

func (a *KlingAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp klingTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kling: parse task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	if resp.Code != 0 {
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Message
		return info, nil
	}

	switch resp.Data.TaskStatus {
	case "submitted":
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	case "processing":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "succeed":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		if len(resp.Data.TaskResult.Videos) > 0 {
			info.ResultURL = resp.Data.TaskResult.Videos[0].URL
		}
	case "failed":
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Data.TaskStatusMsg
	default:
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	}

	return info, nil
}

func (a *KlingAdaptor) GetModelList() []string {
	return ModelList
}

func (a *KlingAdaptor) GetChannelName() string {
	return channelName
}

// ==================== 辅助函数 ====================

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
