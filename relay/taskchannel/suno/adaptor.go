package suno

import (
	"bytes"
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

const channelName = "Suno"

func init() {
	taskchannel.Register(constant.ProviderSuno, func() common.TaskAdaptor {
		return &SunoAdaptor{}
	})
}

type SunoAdaptor struct {
	info *common.RelayInfo
}

func (a *SunoAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *SunoAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, _ []byte) *common.TaskError {
	return nil
}

func (a *SunoAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, _ []byte) map[string]float64 {
	return map[string]float64{"base": 1.0}
}

func (a *SunoAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *SunoAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	// action 从 RelayInfo 上下文中提取
	action := "music"
	if info.RequestURLPath != "" {
		parts := strings.Split(strings.TrimPrefix(info.RequestURLPath, "/suno/submit/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			action = parts[0]
		}
	}
	return fmt.Sprintf("%s/suno/submit/%s", baseURL, action), nil
}

func (a *SunoAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *SunoAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}
	// 默认使用 chirp-v3-0
	if _, ok := req["mv"]; !ok {
		req["mv"] = "chirp-v3-0"
	}
	data, _ := json.Marshal(req)
	return strings.NewReader(string(data)), nil
}

func (a *SunoAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, requestBody)
	if err != nil {
		return nil, err
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, err
	}
	client := common.NewPooledClient(120, info.ChannelMeta.Settings.UseProxy)
	return client.Do(req)
}

func (a *SunoAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: 500, Message: "read response failed"}
	}

	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: 500, Message: "parse response failed"}
	}
	if result.Code != "success" {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    result.Message,
			ErrCode:    result.Code,
		}
	}
	if result.Data == "" {
		return "", body, &common.TaskError{StatusCode: 500, Message: "upstream returned empty task id"}
	}
	return result.Data, body, nil
}

func (a *SunoAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var meta struct {
		UseProxy bool `json:"use_proxy"`
	}
	_ = json.Unmarshal(taskData, &meta)

	url := fmt.Sprintf("%s/suno/fetch", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequest("POST", url, bytes.NewReader(taskData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := common.NewPooledClient(30, meta.UseProxy)
	return client.Do(req)
}

func (a *SunoAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			TaskID     string `json:"task_id"`
			Status     string `json:"status"`
			FailReason string `json:"fail_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("suno: parse task result: %w", err)
	}

	if resp.Code != "success" {
		return &common.TaskInfo{
			Status:     common.TaskStatusFailure,
			FailReason: resp.Message,
			Data:       body,
		}, nil
	}

	if len(resp.Data) == 0 {
		return &common.TaskInfo{
			Status: common.TaskStatusSubmitted,
			Data:   body,
		}, nil
	}

	task := resp.Data[0]
	info := &common.TaskInfo{Data: body}

	switch task.Status {
	case "submitted":
		info.Status = common.TaskStatusSubmitted
	case "queueing":
		info.Status = common.TaskStatusQueued
	case "processing":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "success":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
	case "failed":
		info.Status = common.TaskStatusFailure
		info.FailReason = task.FailReason
	default:
		info.Status = common.TaskStatusSubmitted
	}

	return info, nil
}

func (a *SunoAdaptor) GetModelList() []string {
	return []string{"suno_music", "suno_lyrics"}
}

func (a *SunoAdaptor) GetChannelName() string {
	return channelName
}
