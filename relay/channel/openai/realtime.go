package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// RealtimeProxy Realtime API WebSocket 双向代理
type RealtimeProxy struct {
	info       *common.RelayInfo
	clientConn *websocket.Conn
	targetConn *websocket.Conn
}

// NewRealtimeProxy 创建 Realtime 代理
func NewRealtimeProxy(info *common.RelayInfo) *RealtimeProxy {
	return &RealtimeProxy{info: info}
}

// GetTargetConn 返回上游 WebSocket 连接（用于首条消息转发）
func (p *RealtimeProxy) GetTargetConn() *websocket.Conn {
	return p.targetConn
}

// buildRealtimeURL 将 HTTP(S) BaseURL 转换为 WS(S) URL
func buildRealtimeURL(baseURL string) string {
	u := strings.TrimSuffix(baseURL, "/")
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u + "/v1/realtime"
}

// buildRealtimeHeaders 构建上游 WebSocket 连接头
func buildRealtimeHeaders(info *common.RelayInfo) http.Header {
	header := http.Header{}

	// 如果客户端带 Sec-WebSocket-Protocol，构造 OpenAI realtime 子协议
	if info.RequestHeaders != nil {
		if swsp := info.RequestHeaders.Get("Sec-WebSocket-Protocol"); swsp != "" {
			header.Set("Sec-WebSocket-Protocol",
				fmt.Sprintf("realtime,openai-insecure-api-key.%s,openai-beta.realtime-v1", info.ChannelMeta.ApiKey))
			return header
		}
	}

	// 否则使用标准 Bearer 认证
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("OpenAI-Beta", "realtime=v1")
	return header
}

// DialUpstream 建立到上游的 WebSocket 连接
func (p *RealtimeProxy) DialUpstream() error {
	targetURL := buildRealtimeURL(p.info.ChannelMeta.BaseURL)
	header := buildRealtimeHeaders(p.info)

	timeout := p.info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: time.Duration(timeout) * time.Second,
		Subprotocols:     []string{"realtime"},
	}

	// 渠道启用代理时，WebSocket 连接也走代理
	if p.info.ChannelMeta.Settings.UseProxy {
		if proxyURL := common.GetSystemProxyURL(); proxyURL != "" {
			if parsed, err := url.Parse(proxyURL); err == nil {
				dialer.Proxy = http.ProxyURL(parsed)
			}
		}
	}

	conn, resp, err := dialer.Dial(targetURL, header)
	if err != nil {
		if resp != nil {
			body := make([]byte, 0)
			if resp.Body != nil {
				buf := make([]byte, 4096)
				for {
					n, readErr := resp.Body.Read(buf)
					if n > 0 {
						body = append(body, buf[:n]...)
					}
					if readErr != nil {
						break
					}
				}
				resp.Body.Close()
			}
			return fmt.Errorf("dial upstream websocket failed (status %d): %s, body: %s", resp.StatusCode, err, string(body))
		}
		return fmt.Errorf("dial upstream websocket failed: %w", err)
	}

	p.targetConn = conn
	return nil
}

// Proxy 启动双向代理，阻塞直到连接关闭
func (p *RealtimeProxy) Proxy(ctx context.Context) (*common.Usage, error) {
	var (
		sumUsage     dto.RealtimeUsage
		usageMu      sync.Mutex
		clientClosed = make(chan struct{})
		targetClosed = make(chan struct{})
		errChan      = make(chan error, 2)
	)

	// Client → Upstream
	go func() {
		defer close(clientClosed)
		for {
			_, message, err := p.clientConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					g.Log().Debugf(ctx, "[Realtime] client read error: %v", err)
				}
				return
			}

			if err := p.targetConn.WriteMessage(websocket.TextMessage, message); err != nil {
				g.Log().Debugf(ctx, "[Realtime] upstream write error: %v", err)
				return
			}

			// 累计 input tokens
			var event dto.RealtimeEvent
			if json.Unmarshal(message, &event) == nil {
				// input_audio_buffer.append 或 conversation.item.create 等事件可计入 input
				if event.Type == "conversation.item.create" {
					usageMu.Lock()
					estimateInputFromEvent(&sumUsage, message)
					usageMu.Unlock()
				}
			}
		}
	}()

	// Upstream → Client
	go func() {
		defer close(targetClosed)
		for {
			_, message, err := p.targetConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					g.Log().Debugf(ctx, "[Realtime] upstream read error: %v", err)
				}
				return
			}

			if err := p.clientConn.WriteMessage(websocket.TextMessage, message); err != nil {
				g.Log().Debugf(ctx, "[Realtime] client write error: %v", err)
				return
			}

			// 从 response.done 事件提取 usage
			var event dto.RealtimeEvent
			if json.Unmarshal(message, &event) == nil && event.Type == "response.done" {
				if event.Response != nil && event.Response.Usage != nil {
					usageMu.Lock()
					accumulateUsage(&sumUsage, event.Response.Usage)
					usageMu.Unlock()
				}
			}
		}
	}()

	// 等待任一方向关闭
	select {
	case <-clientClosed:
	case <-targetClosed:
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
	}

	// 关闭连接
	_ = p.targetConn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = p.clientConn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	// 等待清理
	select {
	case <-clientClosed:
	case <-time.After(2 * time.Second):
	}
	select {
	case <-targetClosed:
	case <-time.After(2 * time.Second):
	}

	usageMu.Lock()
	defer usageMu.Unlock()
	return &common.Usage{
		PromptTokens:     sumUsage.InputTokens,
		CompletionTokens: sumUsage.OutputTokens,
		TotalTokens:      sumUsage.TotalTokens,
	}, nil
}

// accumulateUsage 累加 Realtime 使用量
func accumulateUsage(sum *dto.RealtimeUsage, u *dto.RealtimeUsage) {
	sum.TotalTokens += u.TotalTokens
	sum.InputTokens += u.InputTokens
	sum.OutputTokens += u.OutputTokens
}

// estimateInputFromEvent 从客户端事件粗略估算 input tokens
func estimateInputFromEvent(sum *dto.RealtimeUsage, message []byte) {
	// 粗略估算：消息长度 / 4
	estimated := len(message) / 4
	if estimated == 0 {
		estimated = 1
	}
	sum.InputTokens += estimated
	sum.TotalTokens = sum.InputTokens + sum.OutputTokens
}

// Close 关闭 WebSocket 连接
func (p *RealtimeProxy) Close() {
	if p.targetConn != nil {
		_ = p.targetConn.Close()
	}
}
