package ws

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"
)

const (
	clientSendBufferSize = 256
	clientMaxMessageSize = 4096
	clientWriteWait      = 10 * time.Second
	clientPongWait       = 60 * time.Second
	clientPingPeriod     = (clientPongWait * 9) / 10
)

// Client 表示一个 WebSocket 连接。
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userType string // "admin" / "tenant"
	userID   int64
	tenantID int64 // admin 为 0
	done     chan struct{}
	seq      int64
}

func newClient(hub *Hub, conn *websocket.Conn, userType string, userID, tenantID int64) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, clientSendBufferSize),
		userType: userType,
		userID:   userID,
		tenantID: tenantID,
		done:     make(chan struct{}),
	}
}

// readPump 从 WebSocket 读取客户端消息。
// 处理 ping 心跳，退出时触发 close 清理。
func (c *Client) readPump() {
	defer c.close()

	c.conn.SetReadLimit(clientMaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(clientPongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(clientPongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				g.Log().Debugf(context.Background(),
					"[WS] read error: type=%s user=%d err=%v", c.userType, c.userID, err)
			}
			return
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Action {
		case ActionPing:
			c.conn.SetReadDeadline(time.Now().Add(clientPongWait))
			pong, _ := json.Marshal(&WsMessage{
				Channel: "",
				Action:  ActionPong,
			})
			select {
			case c.send <- pong:
			default:
			}
		}
	}
}

// writePump 将 send channel 中的消息写入 WebSocket。
// 同时处理定时 ping 和 context 取消时的关闭。
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(clientPingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			c.conn.SetWriteDeadline(time.Now().Add(clientWriteWait))
			c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"))
			return

		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(clientWriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(clientWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.done:
			return
		}
	}
}

// sendMessage 将 WsMessage 序列化并发送到 send channel。
// 返回 false 表示客户端缓冲区满（慢客户端）。
func (c *Client) sendMessage(msg *WsMessage) bool {
	msg.Seq = atomic.AddInt64(&c.seq, 1)
	data, err := json.Marshal(msg)
	if err != nil {
		return true
	}
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

// close 通知 writePump 退出并从 Hub 注销。
func (c *Client) close() {
	select {
	case <-c.done:
		// already closed
	default:
		close(c.done)
	}
	// 非阻塞注销
	select {
	case c.hub.unregister <- c:
	default:
	}
}
