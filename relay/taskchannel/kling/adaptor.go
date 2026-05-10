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

func init() {
	taskchannel.Register(constant.ProviderKling, func() common.TaskAdaptor {
		return &KlingAdaptor{}
	})
}

type KlingAdaptor struct {
	info *common.RelayInfo
}

func (a *KlingAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *KlingAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, _ []byte) *common.TaskError {
	return nil
}

func (a *KlingAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) == nil {
		// 高清模式加价
		if mode, ok := req["mode"]; ok && fmt.Sprintf("%v", mode) == "hq" {
			ratios["quality"] = 2.0
		}
	}
	return ratios
}

func (a *KlingAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *KlingAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	// 根据是否有 image 参数决定 text2video 或 image2video
	// 默认使用 text2video
	return fmt.Sprintf("%s/v1/videos/text2video", baseURL), nil
}

func (a *KlingAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	apiKey := info.ChannelMeta.ApiKey
	// 如果是 sk- 开头的 key，直接用 Bearer token
	if strings.HasPrefix(apiKey, "sk-") {
		header.Set("Authorization", "Bearer "+apiKey)
	} else {
		// 否则生成 JWT token（accessKey|secretKey 格式）
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

	if info.ChannelMeta.UpstreamModelName != "" {
		req["model_name"] = info.ChannelMeta.UpstreamModelName
		req["model"] = info.ChannelMeta.UpstreamModelName
	}

	// 如果没有指定 mode，默认 std
	if _, ok := req["mode"]; !ok {
		req["mode"] = "std"
	}
	// 如果没有指定 duration，默认 5
	if _, ok := req["duration"]; !ok {
		req["duration"] = "5"
	}

	data, _ := json.Marshal(req)
	return strings.NewReader(string(data)), nil
}

func (a *KlingAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}
	a.BuildRequestHeader(req.Header, info)
	client := &http.Client{Timeout: 120 * 1e9}
	return client.Do(req)
}

func (a *KlingAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: 500, Message: "read response failed"}
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: 500, Message: "parse response failed"}
	}
	if result.Code != 0 {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    result.Message,
			ErrCode:    fmt.Sprintf("%d", result.Code),
		}
	}
	if result.Data.TaskID == "" {
		return "", body, &common.TaskError{StatusCode: 500, Message: "upstream returned empty task id"}
	}
	return result.Data.TaskID, body, nil
}

func (a *KlingAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID   string `json:"task_id"`
		TaskType string `json:"task_type"` // text2video 或 image2video
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("kling: invalid task data: %w", err)
	}

	taskType := data.TaskType
	if taskType == "" {
		taskType = "text2video"
	}
	url := fmt.Sprintf("%s/v1/videos/%s/%s", strings.TrimRight(baseURL, "/"), taskType, data.TaskID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(apiKey, "sk-") {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+buildKlingJWT(apiKey))
	}

	client := &http.Client{Timeout: 30 * 1e9}
	return client.Do(req)
}

func (a *KlingAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
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
	}
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
	}

	return info, nil
}

func (a *KlingAdaptor) GetModelList() []string {
	return []string{"kling-v1", "kling-v1-6", "kling-v2-master"}
}

func (a *KlingAdaptor) GetChannelName() string {
	return channelName
}
