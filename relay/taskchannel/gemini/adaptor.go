package gemini

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

const channelName = "Gemini"

func init() {
	taskchannel.Register(constant.ProviderGemini, func() common.TaskAdaptor {
		return &GeminiAdaptor{}
	})
}

type GeminiAdaptor struct {
	info *common.RelayInfo
}

func (a *GeminiAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *GeminiAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if req.Prompt == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "prompt is required"}
	}
	return nil
}

func (a *GeminiAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req openAIVideoRequest
	if json.Unmarshal(body, &req) == nil {
		resolution := parseResolution(req.Metadata.Resolution)
		duration := req.Metadata.Duration
		if resolution == "1080p" {
			ratios["resolution"] = 1.5
		} else if resolution == "4k" {
			ratios["resolution"] = 2.0
		}
		if duration >= 8 {
			ratios["duration"] = 1.3
		}
	}
	return ratios
}

func (a *GeminiAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *GeminiAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	model := info.ChannelMeta.UpstreamModelName
	if model == "" {
		model = info.OriginModelName
	}
	return fmt.Sprintf("%s/v1beta/models/%s:predictLongRunning", baseURL, model), nil
}

func (a *GeminiAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("x-goog-api-key", info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *GeminiAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req openAIVideoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	// 构建 Gemini predictLongRunning 格式
	geminiReq := map[string]any{
		"instances": []map[string]any{
			{"prompt": req.Prompt},
		},
	}

	params := map[string]any{}
	if resolution := parseResolution(req.Metadata.Resolution); resolution != "" {
		params["resolution"] = resolution
		// 竖屏分辨率（WxH 中 H > W）
		if strings.Contains(req.Metadata.Resolution, "x") {
			parts := strings.Split(req.Metadata.Resolution, "x")
			if len(parts) == 2 && parts[1] > parts[0] {
				params["aspectRatio"] = "9:16"
			} else {
				params["aspectRatio"] = "16:9"
			}
		}
	}
	if req.Metadata.Duration > 0 {
		params["durationSeconds"] = fmt.Sprintf("%d", req.Metadata.Duration)
	}
	if len(params) > 0 {
		geminiReq["parameters"] = params
	}

	data, _ := json.Marshal(geminiReq)
	return strings.NewReader(string(data)), nil
}

func (a *GeminiAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}
	a.BuildRequestHeader(req.Header, info)
	client := common.NewPooledClient(120, info.ChannelMeta.Settings.UseProxy)
	return client.Do(req)
}

func (a *GeminiAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: 500, Message: "read response failed"}
	}

	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: parseGeminiError(body)}
	}

	var result struct {
		Name  string `json:"name"`
		Done  bool   `json:"done"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: 500, Message: "parse response failed"}
	}
	if result.Error != nil {
		return "", body, &common.TaskError{
			StatusCode: result.Error.Code,
			Message:    result.Error.Message,
		}
	}
	if result.Name == "" {
		return "", body, &common.TaskError{StatusCode: 500, Message: "upstream returned empty operation name"}
	}
	return result.Name, body, nil
}

func (a *GeminiAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID   string `json:"task_id"`
		UseProxy bool   `json:"use_proxy"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil || data.TaskID == "" {
		return nil, fmt.Errorf("gemini: invalid task data for polling")
	}

	url := fmt.Sprintf("%s/v1beta/%s", strings.TrimRight(baseURL, "/"), data.TaskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	client := common.NewPooledClient(30, data.UseProxy)
	return client.Do(req)
}

func (a *GeminiAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp struct {
		Name  string `json:"name"`
		Done  bool   `json:"done"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Response *struct {
			GenerateVideoResponse *struct {
				GeneratedSamples []struct {
					Video struct {
						URI        string `json:"uri"`
						VideoBytes string `json:"videoBytes"`
					} `json:"video"`
				} `json:"generatedSamples"`
			} `json:"generateVideoResponse"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("gemini: parse task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	if resp.Error != nil {
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Error.Message
		return info, nil
	}

	if !resp.Done {
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
		return info, nil
	}

	// done=true：提取视频 URL
	info.Status = common.TaskStatusSuccess
	info.Progress = "100%"

	if resp.Response != nil && resp.Response.GenerateVideoResponse != nil {
		samples := resp.Response.GenerateVideoResponse.GeneratedSamples
		if len(samples) > 0 {
			info.ResultURL = samples[0].Video.URI
			if len(samples) > 1 {
				for i, s := range samples {
					info.SubTasks = append(info.SubTasks, common.SubTask{
						Index:     i,
						Status:    common.TaskStatusSuccess,
						ResultURL: s.Video.URI,
					})
				}
			}
		}
	}

	return info, nil
}

func (a *GeminiAdaptor) GetModelList() []string {
	return []string{
		"veo-3.1-generate-001",
		"veo-3.1-fast-generate-001",
		"veo-3.1-generate-preview",
		"veo-3.1-fast-generate-preview",
		"veo-3.0-generate-001",
		"veo-3.0-fast-generate-001",
		"veo-2.0-generate-001",
	}
}

func (a *GeminiAdaptor) GetChannelName() string {
	return channelName
}

// parseGeminiError 从 Gemini 错误响应中提取可读消息
func parseGeminiError(body []byte) string {
	var errResp struct {
		Error *struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
		if errResp.Error.Message != "" {
			return errResp.Error.Message
		}
		return errResp.Error.Status
	}
	return string(body)
}

// openAIVideoRequest OpenAI 兼容格式的视频生成请求
type openAIVideoRequest struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Metadata struct {
		Resolution string `json:"resolution"` // "1280x720", "1920x1080" 等
		Duration   int    `json:"duration"`   // 秒数
	} `json:"metadata"`
}

// parseResolution 从 WxH 格式提取分辨率等级
func parseResolution(res string) string {
	switch res {
	case "1920x1080", "1080x1920":
		return "1080p"
	case "1280x720", "720x1280":
		return "720p"
	case "854x480", "480x854":
		return "480p"
	case "3840x2160", "2160x3840":
		return "4k"
	}
	return ""
}
