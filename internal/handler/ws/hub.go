package ws

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
)

// dispatchRequest 是 Hub 内部消息路由请求。
type dispatchRequest struct {
	userType string // "admin" / "tenant"
	userID   int64  // 0 = 广播
	tenantID int64  // 租户广播时使用
	channel  string
	msg      *WsMessage
}

// Hub 管理所有活跃的 WebSocket 连接，并按用户路由消息。
// 每个服务实例运行一个 Hub。
type Hub struct {
	adminClients  map[int64]map[*Client]bool  // userID → clients（支持多标签页）
	tenantClients map[string]map[*Client]bool // "tenantID:userID" → clients
	allClients    map[*Client]bool

	register   chan *Client
	unregister chan *Client
	dispatch   chan *dispatchRequest

	mu sync.RWMutex
}

var (
	defaultHub     *Hub
	defaultHubOnce sync.Once
)

// InitHub 创建并返回 Hub 单例。
func InitHub(_ context.Context) *Hub {
	defaultHubOnce.Do(func() {
		defaultHub = &Hub{
			adminClients:  make(map[int64]map[*Client]bool),
			tenantClients: make(map[string]map[*Client]bool),
			allClients:    make(map[*Client]bool),
			register:      make(chan *Client, 64),
			unregister:    make(chan *Client, 64),
			dispatch:      make(chan *dispatchRequest, 512),
		}
	})
	return defaultHub
}

// GetHub 返回 Hub 单例，未初始化时返回 nil。
func GetHub() *Hub {
	return defaultHub
}

// Run 启动 Hub 事件循环，阻塞直到 context 取消。
func (h *Hub) Run(ctx context.Context) {
	g.Log().Info(ctx, "[WS Hub] started")
	defer g.Log().Info(ctx, "[WS Hub] stopped")

	for {
		select {
		case <-ctx.Done():
			h.drainAll()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case req := <-h.dispatch:
			h.handleDispatch(req)
		}
	}
}

// Shutdown 立即关闭所有连接。
func Shutdown() {
	h := GetHub()
	if h == nil {
		return
	}
	h.mu.Lock()
	for client := range h.allClients {
		client.close()
	}
	h.mu.Unlock()
}

// DispatchToAdmin 向管理后台用户推送消息。userID=0 时广播给所有管理员。
func (h *Hub) DispatchToAdmin(_ context.Context, userID int64, channel string, msg *WsMessage) {
	select {
	case h.dispatch <- &dispatchRequest{
		userType: "admin",
		userID:   userID,
		channel:  channel,
		msg:      msg,
	}:
	default:
		g.Log().Warningf(context.Background(),
			"[WS Hub] dispatch channel full, dropping admin msg: user=%d channel=%s", userID, channel)
	}
}

// DispatchToTenant 向特定租户用户推送消息。
func (h *Hub) DispatchToTenant(_ context.Context, tenantID, userID int64, channel string, msg *WsMessage) {
	select {
	case h.dispatch <- &dispatchRequest{
		userType: "tenant",
		userID:   userID,
		tenantID: tenantID,
		channel:  channel,
		msg:      msg,
	}:
	default:
		g.Log().Warningf(context.Background(),
			"[WS Hub] dispatch channel full, dropping tenant msg: tenant=%d user=%d channel=%s",
			tenantID, userID, channel)
	}
}

// DispatchToTenantAll 向租户所有在线成员推送消息。
func (h *Hub) DispatchToTenantAll(_ context.Context, tenantID int64, channel string, msg *WsMessage) {
	select {
	case h.dispatch <- &dispatchRequest{
		userType: "tenant",
		userID:   0,
		tenantID: tenantID,
		channel:  channel,
		msg:      msg,
	}:
	default:
		g.Log().Warningf(context.Background(),
			"[WS Hub] dispatch channel full, dropping tenant broadcast: tenant=%d channel=%s",
			tenantID, channel)
	}
}

// ClientCount 返回当前活跃连接数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.allClients)
}

// --- 内部方法 ---

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.allClients[client] = true

	if client.userType == "admin" {
		if h.adminClients[client.userID] == nil {
			h.adminClients[client.userID] = make(map[*Client]bool)
		}
		h.adminClients[client.userID][client] = true
	} else {
		key := tenantClientKey(client.tenantID, client.userID)
		if h.tenantClients[key] == nil {
			h.tenantClients[key] = make(map[*Client]bool)
		}
		h.tenantClients[key][client] = true
	}

	g.Log().Debugf(context.Background(),
		"[WS Hub] client registered: type=%s user=%d tenant=%d total=%d",
		client.userType, client.userID, client.tenantID, len(h.allClients))
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.allClients[client] {
		return
	}

	delete(h.allClients, client)

	if client.userType == "admin" {
		if clients, ok := h.adminClients[client.userID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.adminClients, client.userID)
			}
		}
	} else {
		key := tenantClientKey(client.tenantID, client.userID)
		if clients, ok := h.tenantClients[key]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.tenantClients, key)
			}
		}
	}

	// 排空 send channel，释放消息内存
	drained := 0
drain:
	for {
		select {
		case <-client.send:
			drained++
		default:
			break drain
		}
	}
	close(client.send)

	g.Log().Debugf(context.Background(),
		"[WS Hub] client unregistered: type=%s user=%d drained=%d total=%d",
		client.userType, client.userID, drained, len(h.allClients))
}

func (h *Hub) handleDispatch(req *dispatchRequest) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch req.userType {
	case "admin":
		if req.userID > 0 {
			// 特定管理员
			for client := range h.adminClients[req.userID] {
				if !client.sendMessage(req.msg) {
					go client.close()
				}
			}
		} else {
			// 广播所有管理员
			for _, clients := range h.adminClients {
				for client := range clients {
					if !client.sendMessage(req.msg) {
						go client.close()
					}
				}
			}
		}

	case "tenant":
		if req.userID > 0 {
			// 特定租户用户
			key := tenantClientKey(req.tenantID, req.userID)
			for client := range h.tenantClients[key] {
				if !client.sendMessage(req.msg) {
					go client.close()
				}
			}
		} else if req.tenantID > 0 {
			// 租户广播：匹配 "tenantID:*" 的所有用户
			prefix := tenantClientKeyPrefix(req.tenantID)
			for key, clients := range h.tenantClients {
				if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
					for client := range clients {
						if !client.sendMessage(req.msg) {
							go client.close()
						}
					}
				}
			}
		}
	}
}

func (h *Hub) drainAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.allClients {
		client.close()
	}
	h.adminClients = make(map[int64]map[*Client]bool)
	h.tenantClients = make(map[string]map[*Client]bool)
	h.allClients = make(map[*Client]bool)
}

func tenantClientKey(tenantID, userID int64) string {
	return formatInt64(tenantID) + ":" + formatInt64(userID)
}

func tenantClientKeyPrefix(tenantID int64) string {
	return formatInt64(tenantID) + ":"
}

func formatInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	// 快速 int64 → string，避免 strconv 依赖
	buf := make([]byte, 0, 20)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// 反转
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
