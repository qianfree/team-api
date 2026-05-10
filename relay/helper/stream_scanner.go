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

// StreamScannerHandler 三协程 SSE 流扫描器
// Scanner 协程：读取上游行，过滤 data: 行，推入 dataChan
// DataHandler 协程：从 dataChan 读取，调用 handler 回调，持有 writeMutex
// Ping 协程：定期写 SSE 保活注释，持有 writeMutex
//
// 超时策略：per-chunk 空闲超时（参考 new-api），每收到一个有效 chunk 就重置计时器。
// 只要上游持续发送数据，流可以无限延续，适用于长时间思考的模型。
func StreamScannerHandler(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
	handler DataHandlerFunc,
) {
	stopChan := make(chan bool, stopChanSize)
	dataChan := make(chan string, dataChanSize)
	resetChan := make(chan struct{}, resetChanSize) // 收到有效数据时通知主 goroutine 重置超时
	var writeMutex sync.Mutex
	var wg sync.WaitGroup
	streamTimeout := getStreamTimeout(info)

	SetEventStreamHeaders(writer)

	// Scanner 协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(dataChan)

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, scannerInitialBuf)
		scanner.Buffer(buf, scannerMaxBuf)

		for scanner.Scan() {
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
				return
			default:
			}

			line := scanner.Text()
			if len(line) < minLineLength {
				continue
			}

			if line == "[DONE]" {
				safeSendBool(stopChan)
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

			if info.StreamStatus.GetEndReason() == "" {
				info.SetFirstResponseTime()
			}

			// 通知主 goroutine 重置空闲超时
			select {
			case resetChan <- struct{}{}:
			default:
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
			safeSendBool(stopChan)
		} else if info.StreamStatus.GetEndReason() == "" {
			info.StreamStatus.SetEndReason(common.StreamEndReasonEOF, nil)
			safeSendBool(stopChan)
		}
	}()

	// DataHandler 协程
	sr := NewStreamResult(info.StreamStatus)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for data := range dataChan {
			sr.reset()
			writeMutex.Lock()
			handler(data, sr)
			writeMutex.Unlock()
			if sr.IsStopped() {
				safeSendBool(stopChan)
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

	// 主 goroutine 阻塞等待
	// 使用 Timer 而非 Ticker，每次收到有效数据时 Reset，实现 per-chunk 空闲超时
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
			// 收到有效数据，重置空闲超时
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
	// 等待所有协程退出
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
