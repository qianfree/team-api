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
		return &AliImageAdaptor{}
	})
}

// AliImageAdaptor 阿里云 DashScope 异步图片生成适配器
type AliImageAdaptor struct {
	info *common.RelayInfo
}

func (a *AliImageAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *AliImageAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if _, ok := req["prompt"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "prompt is required"}
	}
	return nil
}

func (a *AliImageAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}
	if n, ok := req["n"].(float64); ok && n > 1 {
		ratios["count"] = n
	}
	return ratios
}

func (a *AliImageAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *AliImageAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	return baseURL + "/api/v1/services/aigc/text2image/image-synthesis", nil
}

func (a *AliImageAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *AliImageAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	converted, err := convertToDashScopeRequest(body, info)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(string(converted)), nil
}

func (a *AliImageAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
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

func (a *AliImageAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
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
		// DashScope 有时在错误时返回 200 但带有 code/message
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

func (a *AliImageAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
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

func (a *AliImageAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
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

func (a *AliImageAdaptor) GetModelList() []string {
	return ModelList
}

func (a *AliImageAdaptor) GetChannelName() string {
	return channelName
}
