package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
)

const (
	defaultStreamTimeout = 300 * time.Second // 两个 chunk 之间的最大空闲间隔（非总超时）
	defaultPingInterval  = 15 * time.Second
	dataChanSize         = 10
	stopChanSize         = 3
	shutdownWaitTimeout  = 5 * time.Second
	minLineLength        = 6
	scannerInitialBuf    = 64 * 1024
	scannerMaxBuf        = 10 * 1024 * 1024
	resetChanSize        = 8
)

// DataHandlerFunc data 处理回调
type DataHandlerFunc func(data string, sr *StreamResult)

// StreamScannerHandler 三协程 SSE 流扫描器（对齐 new-api 架构）
//
// Scanner 协程：读取上游行，检测 [DONE] 终止标记并立即退出（不转发 [DONE] 到 dataChan）
// DataHandler 协程：从 dataChan 读取，调用 handler 回调，持有 writeMutex
// Ping 协程：定期写 SSE 保活注释，持有 writeMutex
//
// 超时策略：per-chunk 空闲超时，scanner.Scan() 每返回一行就通过 resetChan 通知主协程重置 timer。
// scanner.Scan() 阻塞期间不重置，超时从最后收到的字节开始算。
// 对齐 new-api 的 ticker.Reset(streamingTimeout) 行为，但用 Timer+resetChan 避免 ticker 跨协程操作。
//
// 终止流程（对齐 new-api）：
//  1. Scanner 检测到 [DONE] → return → close(dataChan) → safeSendBool(stopChan)
//  2. DataHandler range 退出 → safeSendBool(stopChan)
//  3. 主 goroutine 收到 stopChan → 进入 shutdown
//  4. shutdown: safeSendBool(stopChan) 通知剩余协程 → wg.Wait（最多 5s） → resp.Body.Close()
//  5. [DONE] 由 handler 层（如 openai/stream.go）负责写入客户端
func StreamScannerHandler(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
	handler DataHandlerFunc,
) {
	stopChan := make(chan bool, stopChanSize)
	dataChan := make(chan string, dataChanSize)
	resetChan := make(chan struct{}, resetChanSize) // scanner 每读一行通知主协程重置超时
	var writeMutex sync.Mutex
	var wg sync.WaitGroup
	streamTimeout := getStreamTimeout(info)

	// 确保 resp.Body 最终被关闭（对齐 new-api），解锁被 scanner.Scan() 阻塞的协程
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	SetEventStreamHeaders(writer)

	// Scanner 协程
	wg.Add(1)
	go func() {
		defer func() {
			close(dataChan)
			wg.Done()
			if r := recover(); r != nil {
				info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("scanner panic: %v", r))
			}
			safeSendBool(stopChan)
		}()

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, scannerInitialBuf)
		scanner.Buffer(buf, scannerMaxBuf)

		for scanner.Scan() {
			// 检查停止信号
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
				return
			default:
			}

			// 每读一行就通知主协程重置空闲超时（对齐 new-api ticker.Reset）
			select {
			case resetChan <- struct{}{}:
			default:
			}

			line := scanner.Text()
			if len(line) < minLineLength {
				continue
			}

			// 兼容两种 [DONE] 格式：
			// 1. 裸 "[DONE]"（少数上游）
			// 2. 标准 SSE "data: [DONE]"（主流上游如 OpenAI/DeepSeek）
			// 检测到后立即退出 scanner，由 handler 层负责向客户端写入 [DONE]。
			if line == "[DONE]" {
				info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
				return
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data, ok := ExtractSSEData(line)
			if !ok || data == "" {
				continue
			}

			if data == "[DONE]" {
				info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
				return
			}

			if info.StreamStatus.GetEndReason() == "" {
				info.SetFirstResponseTime()
			}

			select {
			case dataChan <- data:
			case <-stopChan:
				return
			case <-ctx.Done():
				info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonScannerErr, err)
		} else if info.StreamStatus.GetEndReason() == "" {
			info.StreamStatus.SetEndReason(common.StreamEndReasonEOF, nil)
		}
	}()

	// DataHandler 协程
	sr := NewStreamResult(info.StreamStatus)
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("handler panic: %v", r))
			}
			safeSendBool(stopChan)
		}()
		for data := range dataChan {
			sr.reset()
			writeMutex.Lock()
			handler(data, sr)
			writeMutex.Unlock()
			if sr.IsStopped() {
				return
			}
		}
	}()

	// Ping 协程
	pingTicker := time.NewTicker(defaultPingInterval)
	defer pingTicker.Stop()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-pingTicker.C:
				writeMutex.Lock()
				if ctx.Err() != nil {
					writeMutex.Unlock()
					return
				}
				err := WriteSSEPing(writer)
				writeMutex.Unlock()
				if err != nil {
					info.StreamStatus.SetEndReason(common.StreamEndReasonPingFail, err)
					safeSendBool(stopChan)
					return
				}
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// 主 goroutine 阻塞等待（对齐 new-api：单次 select，用 Timer 实现空闲超时）
	timeoutTimer := time.NewTimer(streamTimeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-timeoutTimer.C:
			info.StreamStatus.SetEndReason(common.StreamEndReasonTimeout,
				fmt.Errorf("stream idle timeout: no data received for %v", streamTimeout))
			goto shutdown
		case <-stopChan:
			goto shutdown
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			goto shutdown
		case <-resetChan:
			// scanner 每读一行都通知重置（对齐 new-api ticker.Reset）
			if !timeoutTimer.Stop() {
				select {
				case <-timeoutTimer.C:
				default:
				}
			}
			timeoutTimer.Reset(streamTimeout)
		}
	}

shutdown:
	// 通知所有协程停止（对齐 new-api：先 signal 再 wait）
	safeSendBool(stopChan)

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(shutdownWaitTimeout):
		g.Log().Warning(ctx, "StreamScanner: goroutine shutdown timeout")
	}
}

// getStreamTimeout 获取流式空闲超时时间（两个 chunk 之间的最大间隔），不低于 defaultStreamTimeout。
func getStreamTimeout(info *common.RelayInfo) time.Duration {
	if info.ChannelMeta != nil && info.ChannelMeta.Settings.TimeoutSeconds > 0 {
		t := time.Duration(info.ChannelMeta.Settings.TimeoutSeconds) * time.Second
		if t > defaultStreamTimeout {
			return t
		}
	}
	return defaultStreamTimeout
}

// safeSendBool 安全发送到 stopChan（recover 避免向已关闭 channel 写入）
func safeSendBool(ch chan bool) {
	defer func() { recover() }()
	select {
	case ch <- true:
	default:
	}
}
