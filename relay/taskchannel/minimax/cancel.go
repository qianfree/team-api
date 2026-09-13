package minimax

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// 上游任务取消（DELETE /v2/video_generation/{task_id}）。
// 官方语义：仅 queued 态可取消（action=cancelled）；running / cancelled 不可操作；
// succeeded / failed 是删除任务记录（action=deleted）——网关只做取消、不做删除，
// 因此调用方必须校验返回的 action 字段，仅 cancelled 视为取消成功。

// CancelResult 上游取消响应（DeleteVideoGenerationV2Resp）
type CancelResult struct {
	TaskID string `json:"task_id"`
	Action string `json:"action"` // cancelled = 取消成功；deleted = 任务已终态被删记录（非取消）
	Status string `json:"status"`
}

// CancelTask 取消上游排队中的任务。
// 返回的 TaskError 携带上游状态码与错误信息（含上游判定「运行中不可取消」等场景），
// 调用方以本结果为准决定是否向客户端确认取消。
func CancelTask(baseURL, apiKey, taskID string, useProxy bool) (*CancelResult, *common.TaskError) {
	url := fmt.Sprintf("%s/v2/video_generation/%s", strings.TrimRight(baseURL, "/"), taskID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "create cancel request failed"}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := common.NewPooledClient(30, useProxy)
	resp, err := client.Do(req)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadGateway, Message: "cancel request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read cancel response failed"}
	}

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		var errResp oaiErrorBody
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			msg = errResp.Error.Message
		}
		if msg == "" {
			msg = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		}
		return nil, &common.TaskError{StatusCode: resp.StatusCode, Message: msg}
	}

	var result CancelResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse cancel response failed"}
	}
	return &result, nil
}
