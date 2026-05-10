package midjourney

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

const channelName = "Midjourney"

func init() {
	taskchannel.Register(constant.ProviderMidjourney, func() common.TaskAdaptor {
		return &MjAdaptor{}
	})
}

type MjAdaptor struct {
	info *common.RelayInfo
}

func (a *MjAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// extractAction 从 URL 路径中提取 MJ action
func (a *MjAdaptor) extractAction() string {
	if a.info == nil {
		return ""
	}
	path := a.info.RequestURLPath
	// /mj/submit/:action
	parts := strings.Split(strings.TrimPrefix(path, "/mj/submit/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "imagine"
}

func (a *MjAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, _ []byte) *common.TaskError {
	action := constant.TaskAction(a.extractAction())
	if !constant.MjActions[action] {
		return &common.TaskError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("invalid midjourney action: %s", action),
			ErrCode:    "invalid_action",
		}
	}
	return nil
}

func (a *MjAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, _ []byte) map[string]float64 {
	ratios := map[string]float64{"base": 1.0}
	action := constant.TaskAction(a.extractAction())
	switch action {
	case constant.TaskActionUpscale:
		ratios["action"] = 0.5
	case constant.TaskActionBlend, constant.TaskActionVariation:
		ratios["action"] = 1.5
	case constant.TaskActionVideo:
		ratios["action"] = 3.0
	}
	return ratios
}

func (a *MjAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]float64 {
	return nil
}

func (a *MjAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	action := a.extractAction()
	return fmt.Sprintf("%s/mj/submit/%s", baseURL, action), nil
}

func (a *MjAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("mj-api-secret", info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	return nil
}

func (a *MjAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	return strings.NewReader(string(body)), nil
}

func (a *MjAdaptor) DoRequest(_ context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
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

func (a *MjAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: 500, Message: "read response failed"}
	}
	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	// 尝试解析标准 MJ proxy 响应格式
	var result struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
		Result      string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: 500, Message: "parse response failed"}
	}

	// code 1 = success, 21 = exists, 22 = queued, 23 = queue full, 24 = sensitive word
	if result.Code == 3 {
		return "", body, &common.TaskError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "no available midjourney account",
			ErrCode:    "no_account",
		}
	}
	if result.Code == 24 {
		return "", body, &common.TaskError{
			StatusCode: http.StatusBadRequest,
			Message:    result.Description,
			ErrCode:    "sensitive_word",
		}
	}
	if result.Result == "" {
		return "", body, &common.TaskError{StatusCode: 500, Message: "upstream returned empty task id"}
	}

	return result.Result, body, nil
}

func (a *MjAdaptor) FetchTask(baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("mj: invalid task data: %w", err)
	}

	url := fmt.Sprintf("%s/mj/task/%s/fetch", strings.TrimRight(baseURL, "/"), data.UpstreamTaskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("mj-api-secret", apiKey)
	client := &http.Client{Timeout: 30 * 1e9}
	return client.Do(req)
}

func (a *MjAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp struct {
		Status     string `json:"status"`
		Progress   string `json:"progress"`
		ImageURL   string `json:"imageUrl"`
		FailReason string `json:"failReason"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mj: parse task result: %w", err)
	}

	info := &common.TaskInfo{
		Progress: resp.Progress,
		Data:     body,
	}

	switch strings.ToUpper(resp.Status) {
	case "NOT_START":
		info.Status = common.TaskStatusNotStart
	case "SUBMITTED":
		info.Status = common.TaskStatusSubmitted
	case "IN_PROGRESS", "QUEUED":
		info.Status = common.TaskStatusInProgress
	case "SUCCESS":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		info.ResultURL = resp.ImageURL
	case "FAILURE":
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.FailReason
	default:
		info.Status = common.TaskStatusSubmitted
	}

	return info, nil
}

func (a *MjAdaptor) GetModelList() []string {
	return []string{
		"mj_imagine", "mj_describe", "mj_blend", "mj_upscale",
		"mj_variation", "mj_reroll", "mj_inpaint", "mj_modal",
		"mj_zoom", "mj_custom_zoom", "mj_shorten",
		"mj_high_variation", "mj_low_variation", "mj_pan",
		"mj_upload", "mj_video", "mj_edits",
		"swap_face",
	}
}

func (a *MjAdaptor) GetChannelName() string {
	return channelName
}

// FetchImage 代理获取 MJ 图片
func FetchImage(baseURL, apiKey, upstreamTaskID string) (*http.Response, error) {
	url := fmt.Sprintf("%s/mj/image/%s", strings.TrimRight(baseURL, "/"), upstreamTaskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("mj-api-secret", apiKey)
	client := &http.Client{Timeout: 60 * 1e9}
	return client.Do(req)
}

// unused guard
var _ = bytes.NewReader
