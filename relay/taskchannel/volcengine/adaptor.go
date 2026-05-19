package volcengine

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
	taskchannel.Register(constant.ProviderVolcengine, func() common.TaskAdaptor {
		return &VolcengineVideoAdaptor{}
	})
}

// ==================== 请求/响应结构体 ====================

// contentItem 火山引擎 content 数组项
type contentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *mediaURL `json:"image_url,omitempty"`
	VideoURL *mediaURL `json:"video_url,omitempty"`
	AudioURL *mediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type mediaURL struct {
	URL string `json:"url,omitempty"`
}

// submitRequest 提交任务请求体
type submitRequest struct {
	Model           string        `json:"model"`
	Content         []contentItem `json:"content,omitempty"`
	Resolution      string        `json:"resolution,omitempty"`
	Ratio           string        `json:"ratio,omitempty"`
	Duration        *int          `json:"duration,omitempty"`
	Seed            *int          `json:"seed,omitempty"`
	Watermark       *bool         `json:"watermark,omitempty"`
	GenerateAudio   *bool         `json:"generate_audio,omitempty"`
	ReturnLastFrame *bool         `json:"return_last_frame,omitempty"`
}

// submitResponse 提交任务响应
type submitResponse struct {
	ID string `json:"id"`
}

// taskUsage 火山引擎任务 usage
type taskUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// taskResponse 查询任务响应
type taskResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Usage    taskUsage `json:"usage"`
	Seed     int       `json:"seed"`
	Duration int       `json:"duration"`
	Ratio    string    `json:"ratio"`
	Error    struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ==================== Adaptor 实现 ====================

type VolcengineVideoAdaptor struct {
	info *common.RelayInfo
}

func (a *VolcengineVideoAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *VolcengineVideoAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	return nil
}

func (a *VolcengineVideoAdaptor) EstimateBilling(_ context.Context, info *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}

	// 检测是否含视频输入，应用折扣比率
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}
	if hasVideoInput(req) {
		if ratio, ok := getVideoInputRatio(info.ChannelMeta.UpstreamModelName); ok {
			ratios["video_input"] = ratio
		}
	}
	return ratios
}

func (a *VolcengineVideoAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *VolcengineVideoAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	// 兼容 base URL 带 /api 后缀的情况
	baseURL = strings.TrimSuffix(baseURL, "/api")
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", baseURL), nil
}

func (a *VolcengineVideoAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

func (a *VolcengineVideoAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	// 构建火山引擎请求
	volcReq := submitRequest{}

	// 模型名
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		volcReq.Model = info.ChannelMeta.UpstreamModelName
	} else if m, ok := req["model"].(string); ok {
		volcReq.Model = m
	}

	// content 数组
	volcReq.Content = []contentItem{}

	// 从 prompt 提取文本
	if prompt, ok := req["prompt"].(string); ok && prompt != "" {
		volcReq.Content = append(volcReq.Content, contentItem{
			Type: "text",
			Text: prompt,
		})
	}

	// 从 images 数组提取图片
	if images, ok := req["images"].([]any); ok {
		for _, img := range images {
			if url, ok := img.(string); ok {
				volcReq.Content = append(volcReq.Content, contentItem{
					Type:     "image_url",
					ImageURL: &mediaURL{URL: url},
				})
			}
		}
	}

	// 从 metadata 中提取额外参数
	if metadata, ok := req["metadata"].(map[string]any); ok {
		// content 数组中的额外项（如 video_url、audio_url）
		if contentRaw, ok := metadata["content"].([]any); ok {
			for _, item := range contentRaw {
				if itemMap, ok := item.(map[string]any); ok {
					ci := contentItem{}
					if t, ok := itemMap["type"].(string); ok {
						ci.Type = t
					}
					if t, ok := itemMap["text"].(string); ok {
						ci.Text = t
					}
					if imgURL, ok := itemMap["image_url"].(map[string]any); ok {
						if u, ok := imgURL["url"].(string); ok {
							ci.ImageURL = &mediaURL{URL: u}
						}
					}
					if vidURL, ok := itemMap["video_url"].(map[string]any); ok {
						if u, ok := vidURL["url"].(string); ok {
							ci.VideoURL = &mediaURL{URL: u}
						}
					}
					if audURL, ok := itemMap["audio_url"].(map[string]any); ok {
						if u, ok := audURL["url"].(string); ok {
							ci.AudioURL = &mediaURL{URL: u}
						}
					}
					if ci.Type != "" || ci.Text != "" {
						volcReq.Content = append(volcReq.Content, ci)
					}
				}
			}
		}

		// 其他参数
		if v, ok := metadata["resolution"].(string); ok {
			volcReq.Resolution = v
		}
		if v, ok := metadata["ratio"].(string); ok {
			volcReq.Ratio = v
		}
		if v, ok := metadata["duration"].(float64); ok && v > 0 {
			d := int(v)
			volcReq.Duration = &d
		}
		if v, ok := metadata["seed"].(float64); ok {
			s := int(v)
			volcReq.Seed = &s
		}
		if v, ok := metadata["watermark"].(bool); ok {
			volcReq.Watermark = &v
		}
		if v, ok := metadata["generate_audio"].(bool); ok {
			volcReq.GenerateAudio = &v
		}
		if v, ok := metadata["return_last_frame"].(bool); ok {
			volcReq.ReturnLastFrame = &v
		}
	}

	// seconds 字段映射到 duration
	if seconds, ok := req["seconds"].(string); ok {
		if s, err := parseInt(seconds); err == nil && s > 0 {
			volcReq.Duration = &s
		}
	}

	// 确保至少有一个 text content
	hasText := false
	for _, c := range volcReq.Content {
		if c.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText {
		if prompt, ok := req["prompt"].(string); ok {
			volcReq.Content = append(volcReq.Content, contentItem{Type: "text", Text: prompt})
		}
	}

	data, err := json.Marshal(volcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal volcengine request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *VolcengineVideoAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, requestBody)
	if err != nil {
		return nil, err
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 120 * 1e9}
	return client.Do(req)
}

func (a *VolcengineVideoAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}

	// 非成功状态码
	if resp.StatusCode != http.StatusOK {
		msg := string(body)
		if msg == "" {
			msg = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		}
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var result submitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
	}
	if result.ID == "" {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
	}
	return result.ID, body, nil
}

func (a *VolcengineVideoAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("volcengine: invalid task data: %w", err)
	}

	url := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/api"), data.TaskID)

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

func (a *VolcengineVideoAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp taskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("volcengine: parse task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	// 提取 token 用量
	if resp.Usage.TotalTokens > 0 {
		info.CompletionTokens = resp.Usage.CompletionTokens
		info.TotalTokens = resp.Usage.TotalTokens
		info.PromptTokens = resp.Usage.TotalTokens - resp.Usage.CompletionTokens
	}

	switch resp.Status {
	case "pending", "queued":
		info.Status = common.TaskStatusQueued
		info.Progress = "10%"
	case "processing", "running":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "succeeded":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		info.ResultURL = resp.Content.VideoURL
	case "failed":
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Error.Message
	default:
		info.Status = common.TaskStatusInProgress
		info.Progress = "30%"
	}

	return info, nil
}

func (a *VolcengineVideoAdaptor) GetModelList() []string {
	return ModelList
}

func (a *VolcengineVideoAdaptor) GetChannelName() string {
	return channelName
}

// ==================== 辅助函数 ====================

// hasVideoInput 检测请求是否包含视频输入
func hasVideoInput(req map[string]any) bool {
	// 检查 metadata.content 中是否有 video_url 类型
	metadata, _ := req["metadata"].(map[string]any)
	if metadata == nil {
		return false
	}
	content, _ := metadata["content"].([]any)
	for _, item := range content {
		if m, ok := item.(map[string]any); ok {
			if m["type"] == "video_url" {
				return true
			}
			if _, has := m["video_url"]; has {
				return true
			}
		}
	}
	return false
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
