package middleware

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/test/gtest"
)

// TestRequestId_DualIDMechanism 测试双 ID 机制
func TestRequestId_DualIDMechanism(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s := g.Server("mw-request-id-dual")
		s.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(RequestId)
			group.ALL("/test", func(r *ghttp.Request) {
				serverID := r.GetCtxVar("RequestId").String()
				clientID := r.GetCtxVar("ClientTraceId").String()
				r.Response.WriteJson(g.Map{
					"server_id": serverID,
					"client_id": clientID,
				})
			})
		})
		s.SetAddr("127.0.0.1:0")
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		ctx := gctx.New()
		client := g.Client()
		client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))

		tests := []struct {
			name                    string
			clientRequestId         string
			expectServerIDGenerated bool
			expectClientIDInContext bool
		}{
			{
				name:                    "无客户端 ID - 服务端生成双头",
				clientRequestId:         "",
				expectServerIDGenerated: true,
				expectClientIDInContext: false,
			},
			{
				name:                    "有效客户端 ID - 回显 + 独立服务端 ID",
				clientRequestId:         "client-trace-123",
				expectServerIDGenerated: true,
				expectClientIDInContext: true,
			},
			{
				name:                    "非法客户端 ID（含空格）- 拒绝",
				clientRequestId:         "invalid id with spaces",
				expectServerIDGenerated: true,
				expectClientIDInContext: false,
			},
			{
				name:                    "非法客户端 ID（超长）- 拒绝",
				clientRequestId:         strings.Repeat("a", 65),
				expectServerIDGenerated: true,
				expectClientIDInContext: false,
			},
		}

		for _, tt := range tests {
			resp := client.Header(g.MapStrStr{
				"X-Request-Id": tt.clientRequestId,
			}).GetContent(ctx, "/test")

			var body map[string]string
			t.AssertNil(gjson.DecodeTo(resp, &body))

			serverID := body["server_id"]
			clientID := body["client_id"]

			// 验证服务端 ID 必须存在
			t.Assert(serverID != "", true)

			// 验证客户端 ID
			if tt.expectClientIDInContext {
				t.Assert(clientID, tt.clientRequestId)
			} else {
				t.Assert(clientID, "")
			}
		}
	})
}

// TestRequestId_SecurityIsolation 测试安全隔离：服务端 ID 不受客户端控制
func TestRequestId_SecurityIsolation(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		serverIDs := make(map[string]bool)
		var mu sync.Mutex

		s := g.Server("mw-request-id-isolation")
		s.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(RequestId)
			group.ALL("/test", func(r *ghttp.Request) {
				serverID := r.GetCtxVar("RequestId").String()
				mu.Lock()
				serverIDs[serverID] = true
				mu.Unlock()
				r.Response.WriteJson(g.Map{
					"server_id": serverID,
				})
			})
		})
		s.SetAddr("127.0.0.1:0")
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		ctx := gctx.New()
		client := g.Client()
		client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))

		// 模拟恶意客户端：连续 10 次请求，传递相同的客户端 ID
		maliciousClientID := "malicious-repeated-id"
		for i := 0; i < 10; i++ {
			client.Header(g.MapStrStr{
				"X-Request-Id": maliciousClientID,
			}).GetContent(ctx, "/test")
		}

		// 验证：每次请求的服务端 ID 必须不同
		t.Assert(len(serverIDs), 10)
	})
}

// TestRequestId_IdempotencyKeyUniqueness 测试幂等性键的唯一性保证
func TestRequestId_IdempotencyKeyUniqueness(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		const concurrency = 100
		serverIDs := make(chan string, concurrency)

		s := g.Server("mw-request-id-idempotency")
		s.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(RequestId)
			group.ALL("/test", func(r *ghttp.Request) {
				serverID := r.GetCtxVar("RequestId").String()
				r.Response.WriteJson(g.Map{
					"server_id": serverID,
				})
			})
		})
		s.SetAddr("127.0.0.1:0")
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		ctx := gctx.New()
		client := g.Client()
		client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))

		// 并发发起请求
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp := client.GetContent(ctx, "/test")
				var body map[string]string
				if err := gjson.DecodeTo(resp, &body); err == nil {
					serverIDs <- body["server_id"]
				}
			}()
		}

		wg.Wait()
		close(serverIDs)

		// 统计唯一 ID 数量
		uniqueIDs := make(map[string]bool)
		for id := range serverIDs {
			uniqueIDs[id] = true
		}

		// 验证：所有服务端 ID 必须唯一
		t.Assert(len(uniqueIDs), concurrency)
	})
}
