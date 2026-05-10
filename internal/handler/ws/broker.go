package ws

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const redisWsChannel = "ws:messages"

// WsBroker 封装 Redis Pub/Sub，用于跨实例 WebSocket 消息分发。
type WsBroker struct {
	hub *Hub
}

// InitWsBroker 初始化 WebSocket 消息代理并启动 Redis 订阅者。
// 必须在 InitHub 之后调用。
func InitWsBroker(ctx context.Context) {
	hub := GetHub()
	if hub == nil {
		g.Log().Warning(ctx, "[WsBroker] hub not initialized, skip broker init")
		return
	}
	// 幂等：多次调用只初始化一次
	go startSubscriber(ctx, hub)
	g.Log().Info(ctx, "[WsBroker] initialized")
}

// --- Publish API（业务逻辑调用）---

// PublishToAdmin 向特定管理员推送消息。
func PublishToAdmin(ctx context.Context, userID int64, channel, action string, payload any) error {
	return publish(ctx, &RedisWsMessage{
		UserType: "admin",
		Target:   "user:" + strconv.FormatInt(userID, 10),
		Channel:  channel,
		Action:   action,
	}, payload)
}

// PublishToAllAdmins 向所有在线管理员广播消息。
func PublishToAllAdmins(ctx context.Context, channel, action string, payload any) error {
	return publish(ctx, &RedisWsMessage{
		UserType: "admin",
		Target:   "all",
		Channel:  channel,
		Action:   action,
	}, payload)
}

// PublishToTenantUser 向特定租户用户推送消息。
func PublishToTenantUser(ctx context.Context, tenantID, userID int64, channel, action string, payload any) error {
	return publish(ctx, &RedisWsMessage{
		UserType: "tenant",
		Target:   "user:" + strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(userID, 10),
		Channel:  channel,
		Action:   action,
	}, payload)
}

// PublishToTenantAll 向租户所有在线成员广播消息。
func PublishToTenantAll(ctx context.Context, tenantID int64, channel, action string, payload any) error {
	return publish(ctx, &RedisWsMessage{
		UserType: "tenant",
		Target:   "tenant_all:" + strconv.FormatInt(tenantID, 10),
		Channel:  channel,
		Action:   action,
	}, payload)
}

// --- 内部方法 ---

func publish(ctx context.Context, msg *RedisWsMessage, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		g.Log().Warningf(ctx, "[WsBroker] marshal payload failed: %v", err)
		return err
	}
	msg.Payload = data

	body, err := json.Marshal(msg)
	if err != nil {
		g.Log().Warningf(ctx, "[WsBroker] marshal message failed: %v", err)
		return err
	}

	_, err = g.Redis().Do(ctx, "PUBLISH", redisWsChannel, string(body))
	if err != nil {
		g.Log().Warningf(ctx, "[WsBroker] redis publish failed: %v", err)
	}
	return err
}

func startSubscriber(ctx context.Context, hub *Hub) {
	for {
		conn, _, err := g.Redis().Subscribe(ctx, redisWsChannel)
		if err != nil {
			g.Log().Errorf(ctx, "[WsBroker] subscriber connect failed: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		g.Log().Info(ctx, "[WsBroker] Redis subscriber started")

		for {
			msg, err := conn.ReceiveMessage(ctx)
			if err != nil {
				g.Log().Warningf(ctx, "[WsBroker] subscriber recv error: %v", err)
				conn.Close(ctx)
				time.Sleep(5 * time.Second)
				break
			}

			handleRedisMessage(ctx, hub, msg.Payload)
		}
	}
}

func handleRedisMessage(ctx context.Context, hub *Hub, payload string) {
	var redisMsg RedisWsMessage
	if err := json.Unmarshal([]byte(payload), &redisMsg); err != nil {
		g.Log().Warningf(ctx, "[WsBroker] invalid message: %v", err)
		return
	}

	wsMsg := &WsMessage{
		Channel: redisMsg.Channel,
		Action:  redisMsg.Action,
		Payload: redisMsg.Payload,
	}

	switch {
	case redisMsg.Target == "all":
		if redisMsg.UserType == "admin" {
			hub.DispatchToAdmin(ctx, 0, redisMsg.Channel, wsMsg)
		}

	case strings.HasPrefix(redisMsg.Target, "tenant_all:"):
		tenantID, _ := strconv.ParseInt(strings.TrimPrefix(redisMsg.Target, "tenant_all:"), 10, 64)
		if tenantID > 0 {
			hub.DispatchToTenantAll(ctx, tenantID, redisMsg.Channel, wsMsg)
		}

	case strings.HasPrefix(redisMsg.Target, "user:"):
		if redisMsg.UserType == "admin" {
			userID, _ := strconv.ParseInt(strings.TrimPrefix(redisMsg.Target, "user:"), 10, 64)
			hub.DispatchToAdmin(ctx, userID, redisMsg.Channel, wsMsg)
		} else {
			parts := strings.SplitN(strings.TrimPrefix(redisMsg.Target, "user:"), ":", 2)
			if len(parts) == 2 {
				tenantID, _ := strconv.ParseInt(parts[0], 10, 64)
				userID, _ := strconv.ParseInt(parts[1], 10, 64)
				if tenantID > 0 && userID > 0 {
					hub.DispatchToTenant(ctx, tenantID, userID, redisMsg.Channel, wsMsg)
				}
			}
		}
	}
}
