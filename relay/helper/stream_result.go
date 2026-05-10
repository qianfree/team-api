package helper

import (
	"fmt"

	"github.com/qianfree/team-api/relay/common"
)

// StreamResult 每次 dataHandler 调用的控制对象
type StreamResult struct {
	status  *common.StreamStatus
	stopped bool
}

// NewStreamResult 创建 StreamResult
func NewStreamResult(status *common.StreamStatus) *StreamResult {
	return &StreamResult{status: status}
}

// Error 记录软错误（流继续处理）
func (sr *StreamResult) Error(err error) {
	if sr.status != nil {
		sr.status.RecordError(fmt.Sprintf("handler error: %v", err))
	}
}

// Stop 记录致命错误并停止流
func (sr *StreamResult) Stop(err error) {
	sr.stopped = true
	if sr.status != nil {
		sr.status.SetEndReason(common.StreamEndReasonHandlerStop, err)
	}
}

// Done 标记正常完成并停止流
func (sr *StreamResult) Done() {
	sr.stopped = true
	if sr.status != nil {
		sr.status.SetEndReason(common.StreamEndReasonDone, nil)
	}
}

// IsStopped 是否已停止
func (sr *StreamResult) IsStopped() bool {
	return sr.stopped
}

// reset 重置 stopped 标志（每次 dataHandler 调用前使用）
func (sr *StreamResult) reset() {
	sr.stopped = false
}
