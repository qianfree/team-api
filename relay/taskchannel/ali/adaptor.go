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
		return &AliVideoAdaptor{}
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
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		VideoURL   string `json:"video_url"`
		Code       string `json:"code"`
		Message    string `json:"message"`
	} `json:"output"`
	Usage struct {
		Duration            float64 `json:"duration"`
		VideoCount          int     `json:"video_count"`
		OutputVideoDuration int     `json:"output_video_duration"`
		SR                  int     `json:"SR"`
		Ratio               string  `json:"ratio"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

// ==================== Adaptor 实现 ====================

type AliVideoAdaptor struct {
	info *common.RelayInfo
}

func (a *AliVideoAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *AliVideoAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	return nil
}

func (a *AliVideoAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}
	// duration 影响计费，传递给计费引擎
	if metadata, ok := req["metadata"].(map[string]any); ok {
		if v, ok := metadata["duration"].(float64); ok && v > 0 {
			ratios["duration"] = v
		}
	}
	if seconds, ok := req["seconds"].(string); ok {
		if d, err := parseInt(seconds); err == nil && d > 0 {
			ratios["duration"] = float64(d)
		}
	}
	return ratios
}

func (a *AliVideoAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *AliVideoAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	return baseURL + "/api/v1/services/aigc/video-generation/video-synthesis", nil
}

func (a *AliVideoAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *AliVideoAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	dsReq := dashScopeVideoRequest{
		Input: dashScopeVideoInput{},
	}

	// 模型名
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		dsReq.Model = info.ChannelMeta.UpstreamModelName
	} else if m, ok := req["model"].(string); ok {
		dsReq.Model = m
	}

	// prompt → input.prompt
	if v, ok := req["prompt"].(string); ok {
		dsReq.Input.Prompt = v
	}

	// 从 metadata 提取参数
	params := &dashScopeVideoParams{}
	hasParams := false

	if metadata, ok := req["metadata"].(map[string]any); ok {
		if v, ok := metadata["negative_prompt"].(string); ok {
			dsReq.Input.NegativePrompt = v
		}
		if v, ok := metadata["audio_url"].(string); ok {
			dsReq.Input.AudioURL = v
		}

		// wan2.7+ 使用 resolution + ratio
		if v, ok := metadata["resolution"].(string); ok {
			params.Resolution = v
			hasParams = true
		}
		if v, ok := metadata["ratio"].(string); ok {
			params.Ratio = v
			hasParams = true
		}
		// wan2.6 及以下使用 size
		if v, ok := metadata["size"].(string); ok {
			params.Size = strings.ReplaceAll(v, "x", "*")
			hasParams = true
		}
		if v, ok := metadata["duration"].(float64); ok && v > 0 {
			d := int(v)
			params.Duration = &d
			hasParams = true
		}
		if v, ok := metadata["prompt_extend"].(bool); ok {
			params.PromptExtend = &v
			hasParams = true
		}
		if v, ok := metadata["watermark"].(bool); ok {
			params.Watermark = &v
			hasParams = true
		}
		if v, ok := metadata["seed"].(float64); ok {
			s := int(v)
			params.Seed = &s
			hasParams = true
		}
	}

	// seconds 字段映射到 duration
	if seconds, ok := req["seconds"].(string); ok {
		if d, err := parseInt(seconds); err == nil && d > 0 {
			params.Duration = &d
			hasParams = true
		}
	}

	if hasParams {
		dsReq.Parameters = params
	}

	data, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope video request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *AliVideoAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
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

func (a *AliVideoAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
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
		// DashScope 有时在 200 响应中返回错误
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

func (a *AliVideoAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
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

func (a *AliVideoAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
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
		info.ResultURL = resp.Output.VideoURL
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

func (a *AliVideoAdaptor) GetModelList() []string {
	return ModelList
}

func (a *AliVideoAdaptor) GetChannelName() string {
	return channelName
}

// ==================== 辅助函数 ====================

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
