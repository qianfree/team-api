package sora

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

const channelName = "Sora"

func init() {
	taskchannel.Register(constant.ProviderSora, func() common.TaskAdaptor {
		return &SoraAdaptor{}
	})
}

type SoraAdaptor struct {
	info *common.RelayInfo
}

func (a *SoraAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *SoraAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, _ []byte) *common.TaskError {
	return nil
}

func (a *SoraAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	var req map[string]json.RawMessage
	if json.Unmarshal(body, &req) == nil {
		// 按秒数加价
		if secs, ok := req["seconds"]; ok {
			var s string
			if json.Unmarshal(secs, &s) == nil && s != "" {
				if s == "10" || s == "15" || s == "20" {
					ratios["duration"] = 1.5
				}
			}
		}
		if dur, ok := req["duration"]; ok {
			var d float64
			if json.Unmarshal(dur, &d) == nil && d > 10 {
				ratios["duration"] = 1.5
			}
		}
	}
	return ratios
}

func (a *SoraAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *SoraAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	return fmt.Sprintf("%s/v1/videos", baseURL), nil
}

func (a *SoraAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *SoraAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}
	// 使用上游模型名
	if info.ChannelMeta.UpstreamModelName != "" {
		req["model"] = info.ChannelMeta.UpstreamModelName
	}
	data, _ := json.Marshal(req)
	return strings.NewReader(string(data)), nil
}

func (a *SoraAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}
	a.BuildRequestHeader(req.Header, info)

	client := &http.Client{Timeout: 120 * 1e9 /* 120s */}
	return client.Do(req)
}

func (a *SoraAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: 500, Message: "read response failed"}
	}

	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: 500, Message: "parse response failed"}
	}
	if result.ID == "" {
		return "", body, &common.TaskError{StatusCode: 500, Message: "upstream returned empty task id"}
	}
	return result.ID, body, nil
}

func (a *SoraAdaptor) FetchTask(baseURL, apiKey string, _ []byte) (*http.Response, error) {
	// Sora 的 FetchTask 需要上游任务 ID，通过 taskData 传入
	return nil, fmt.Errorf("sora: use FetchTaskByID instead")
}

// FetchTaskByID 根据上游任务 ID 查询状态
func (a *SoraAdaptor) FetchTaskByID(baseURL, apiKey, taskID string) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseURL, "/"), taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 30 * 1e9}
	return client.Do(req)
}

func (a *SoraAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp struct {
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		Error    *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sora: parse task result: %w", err)
	}

	info := &common.TaskInfo{
		Progress: fmt.Sprintf("%d%%", resp.Progress),
		Data:     body,
	}

	switch resp.Status {
	case "queued", "pending":
		info.Status = common.TaskStatusQueued
	case "processing", "in_progress":
		info.Status = common.TaskStatusInProgress
	case "completed":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
	case "failed", "cancelled":
		info.Status = common.TaskStatusFailure
		if resp.Error != nil {
			info.FailReason = resp.Error.Message
		}
	default:
		info.Status = common.TaskStatusSubmitted
	}

	return info, nil
}

func (a *SoraAdaptor) GetModelList() []string {
	return []string{"sora-2", "sora-2-pro"}
}

func (a *SoraAdaptor) GetChannelName() string {
	return channelName
}
