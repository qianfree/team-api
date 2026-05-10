package ws

import (
	"context"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gorilla/websocket"
)

// Context keys — 与 middleware 包中的常量值保持一致。
// 不直接导入 middleware 包，避免循环依赖（middleware → logic/common → handler/ws）。
const (
	ctxKeyUserID   = "userId"
	ctxKeyTenantID = "tenantId"
)

func getUserID(ctx context.Context) int64 {
	val := ctx.Value(ctxKeyUserID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

func getTenantID(ctx context.Context) int64 {
	val := ctx.Value(ctxKeyTenantID)
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HandleAdminWS 处理 /api/admin/ws WebSocket 连接。
// AdminAuth 中间件已完成 JWT 验证，context 中已设置用户信息。
func HandleAdminWS(r *ghttp.Request) {
	userID := getUserID(r.Context())
	if userID == 0 {
		r.Response.WriteStatus(401, "unauthorized")
		return
	}

	hub := GetHub()
	if hub == nil {
		r.Response.WriteStatus(503, "service unavailable")
		return
	}

	conn, err := wsUpgrader.Upgrade(r.Response.Writer, r.Request, nil)
	if err != nil {
		g.Log().Warningf(r.Context(), "[WS] admin upgrade failed: user=%d err=%v", userID, err)
		return
	}

	client := newClient(hub, conn, "admin", userID, 0)
	hub.register <- client

	go client.writePump(r.Context())
	client.readPump()
}

// HandleTenantWS 处理 /api/tenant/ws WebSocket 连接。
// TenantAuth 中间件已完成 JWT 验证，context 中已设置租户和用户信息。
func HandleTenantWS(r *ghttp.Request) {
	tenantID := getTenantID(r.Context())
	userID := getUserID(r.Context())
	if userID == 0 || tenantID == 0 {
		r.Response.WriteStatus(401, "unauthorized")
		return
	}

	hub := GetHub()
	if hub == nil {
		r.Response.WriteStatus(503, "service unavailable")
		return
	}

	conn, err := wsUpgrader.Upgrade(r.Response.Writer, r.Request, nil)
	if err != nil {
		g.Log().Warningf(r.Context(), "[WS] tenant upgrade failed: tenant=%d user=%d err=%v",
			tenantID, userID, err)
		return
	}

	client := newClient(hub, conn, "tenant", userID, tenantID)
	hub.register <- client

	go client.writePump(r.Context())
	client.readPump()
}
