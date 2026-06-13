package ali

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

func init() {
	taskchannel.Register(constant.ProviderAli, func() common.TaskAdaptor {
		return &AliAdaptor{}
	})
}

// ==================== 请求/响应结构体 ====================

// dashScopeVideoRequest DashScope 视频生成请求
type dashScopeVideoRequest struct {
	Model      string                `json:"model"`
	Input      dashScopeVideoInput   `json:"input"`
	Parameters *dashScopeVideoParams `json:"parameters,omitempty"`
}

type dashScopeVideoInput struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
}

type dashScopeVideoParams struct {
	Resolution   string `json:"resolution,omitempty"`
	Ratio        string `json:"ratio,omitempty"`
	Size         string `json:"size,omitempty"`
	Duration     *int   `json:"duration,omitempty"`
	PromptExtend *bool  `json:"prompt_extend,omitempty"`
	Watermark    *bool  `json:"watermark,omitempty"`
	Seed         *int   `json:"seed,omitempty"`
}

// aliMetadata Ali DashScope metadata 参数结构体（用于 UnmarshalMetadata 映射）
type aliMetadata struct {
	NegativePrompt string `json:"negative_prompt,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	Ratio          string `json:"ratio,omitempty"`
	Size           string `json:"size,omitempty"`
	Duration       *int   `json:"duration,omitempty"`
	PromptExtend   *bool  `json:"prompt_extend,omitempty"`
	Watermark      *bool  `json:"watermark,omitempty"`
	Seed           *int   `json:"seed,omitempty"`
}

// dashScopeImageRequest DashScope 图片生成请求
type dashScopeImageRequest struct {
	Model      string              `json:"model"`
	Input      dashScopeImageInput `json:"input"`
	Parameters map[string]any      `json:"parameters,omitempty"`
}

type dashScopeImageInput struct {
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

// dashScopeSubmitResponse DashScope 异步提交响应
type dashScopeSubmitResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Code       string `json:"code"`
		Message    string `json:"message"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// dashScopeTaskResponse DashScope 异步任务查询响应
type dashScopeTaskResponse struct {
	Output struct {
		TaskID     string            `json:"task_id"`
		TaskStatus string            `json:"task_status"`
		VideoURL   string            `json:"video_url"`
		Results    []dashScopeResult `json:"results"`
		Code       string            `json:"code"`
		Message    string            `json:"message"`
	} `json:"output"`
	Usage struct {
		Duration            float64 `json:"duration"`
		VideoCount          int     `json:"video_count"`
		OutputVideoDuration int     `json:"output_video_duration"`
		SR                  int     `json:"SR"`
		Ratio               string  `json:"ratio"`
		ImageCount          int     `json:"image_count"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

type dashScopeResult struct {
	URL string `json:"url"`
}

// ==================== Adaptor 实现 ====================

type AliAdaptor struct {
	info    *common.RelayInfo
	isVideo bool // 是否视频生成（否则图片生成）
}

func (a *AliAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// detectVideo 根据模型名判断是否为视频生成
func (a *AliAdaptor) detectVideo(modelName string) bool {
	return strings.HasPrefix(modelName, "wan2.") || strings.HasPrefix(modelName, "wanx2.1-t2v")
}

func (a *AliAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	return nil
}

func (a *AliAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}

	if a.isVideo {
		// duration 影响计费
		metadata := taskchannel.ExtractMetadata(req)
		var meta struct {
			Duration *int `json:"duration"`
		}
		if taskchannel.UnmarshalMetadata(metadata, &meta) == nil && meta.Duration != nil && *meta.Duration > 0 {
			ratios["duration"] = float64(*meta.Duration)
		}
		if seconds, ok := req["seconds"].(string); ok {
			if d, err := parseInt(seconds); err == nil && d > 0 {
				ratios["duration"] = float64(d)
			}
		}
	} else {
		// 图片数量
		if n, ok := req["n"].(float64); ok && n > 1 {
			ratios["count"] = n
		}
	}

	return ratios
}

func (a *AliAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *AliAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	if a.isVideo {
		return baseURL + "/api/v1/services/aigc/video-generation/video-synthesis", nil
	}
	return baseURL + "/api/v1/services/aigc/text2image/image-synthesis", nil
}

func (a *AliAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *AliAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	// 确定模型名，同时判断视频/图片
	modelName := ""
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		modelName = info.ChannelMeta.UpstreamModelName
	} else if m, ok := req["model"].(string); ok {
		modelName = m
	}
	a.isVideo = a.detectVideo(modelName)

	if a.isVideo {
		return a.buildVideoRequest(info, req, modelName)
	}
	return a.buildImageRequest(info, req, modelName)
}

func (a *AliAdaptor) buildVideoRequest(info *common.RelayInfo, req map[string]any, modelName string) (io.Reader, error) {
	dsReq := dashScopeVideoRequest{
		Input: dashScopeVideoInput{},
	}
	dsReq.Model = modelName

	if v, ok := req["prompt"].(string); ok {
		dsReq.Input.Prompt = v
	}

	params := &dashScopeVideoParams{}

	// 从 metadata 提取参数
	metadata := taskchannel.ExtractMetadata(req)
	var meta aliMetadata
	if err := taskchannel.UnmarshalMetadata(metadata, &meta); err != nil {
		return nil, fmt.Errorf("parse ali metadata: %w", err)
	}
	dsReq.Input.NegativePrompt = meta.NegativePrompt
	dsReq.Input.AudioURL = meta.AudioURL
	params.Resolution = meta.Resolution
	params.Ratio = meta.Ratio
	if meta.Size != "" {
		params.Size = strings.ReplaceAll(meta.Size, "x", "*")
	}
	params.Duration = meta.Duration
	params.PromptExtend = meta.PromptExtend
	params.Watermark = meta.Watermark
	params.Seed = meta.Seed

	if seconds, ok := req["seconds"].(string); ok {
		if d, err := parseInt(seconds); err == nil && d > 0 {
			params.Duration = &d
		}
	}

	if params.Resolution != "" || params.Ratio != "" || params.Size != "" ||
		params.Duration != nil || params.PromptExtend != nil || params.Watermark != nil || params.Seed != nil {
		dsReq.Parameters = params
	}

	data, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope video request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *AliAdaptor) buildImageRequest(info *common.RelayInfo, req map[string]any, modelName string) (io.Reader, error) {
	dsReq := dashScopeImageRequest{
		Input:      dashScopeImageInput{},
		Parameters: make(map[string]any),
	}
	dsReq.Model = modelName

	if v, ok := req["prompt"].(string); ok {
		dsReq.Input.Prompt = v
	}
	if v, ok := req["negative_prompt"].(string); ok {
		dsReq.Input.NegativePrompt = v
	}

	// size: 1024x1024 → 1024*1024
	if v, ok := req["size"].(string); ok && v != "" {
		dsReq.Parameters["size"] = strings.ReplaceAll(v, "x", "*")
	}
	if v, ok := req["n"]; ok {
		dsReq.Parameters["n"] = v
	}
	if v, ok := req["seed"]; ok {
		dsReq.Parameters["seed"] = v
	}
	if v, ok := req["style"]; ok {
		dsReq.Parameters["style"] = v
	}
	if v, ok := req["ref_strength"]; ok {
		dsReq.Parameters["ref_strength"] = v
	}
	if v, ok := req["ref_img"]; ok {
		dsReq.Parameters["ref_img"] = v
	}

	if len(dsReq.Parameters) == 0 {
		dsReq.Parameters = nil
	}

	data, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope image request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *AliAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}
	client := &http.Client{Timeout: 120 * 1e9}
	return client.Do(req)
}

func (a *AliAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}

	// DashScope 错误时可能在顶层返回 code + message
	var errResp dashScopeSubmitResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Message,
			ErrCode:    errResp.Code,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result dashScopeSubmitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
	}

	if result.Output.TaskID == "" {
		if result.Output.Code != "" {
			return "", body, &common.TaskError{
				StatusCode: resp.StatusCode,
				Message:    result.Output.Message,
				ErrCode:    result.Output.Code,
			}
		}
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
	}

	return result.Output.TaskID, body, nil
}

func (a *AliAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("ali: invalid task data: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s", strings.TrimRight(baseURL, "/"), data.TaskID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * 1e9}
	return client.Do(req)
}

func (a *AliAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp dashScopeTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ali: parse task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	switch resp.Output.TaskStatus {
	case "PENDING":
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	case "RUNNING":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "SUCCEEDED":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		// 视频结果
		if resp.Output.VideoURL != "" {
			info.ResultURL = resp.Output.VideoURL
		}
		// 图片结果
		if len(resp.Output.Results) > 0 {
			info.ResultURL = resp.Output.Results[0].URL
			for i, r := range resp.Output.Results {
				info.SubTasks = append(info.SubTasks, common.SubTask{
					Index:     i,
					ResultURL: r.URL,
				})
			}
		}
	case "FAILED":
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Output.Message
	case "CANCELED", "UNKNOWN":
		info.Status = common.TaskStatusFailure
		info.FailReason = fmt.Sprintf("task status: %s", resp.Output.TaskStatus)
	default:
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	}

	return info, nil
}

func (a *AliAdaptor) GetModelList() []string {
	return ModelList
}

func (a *AliAdaptor) GetChannelName() string {
	return channelName
}

// ==================== 辅助函数 ====================

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
