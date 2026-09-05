package middleware

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/test/gtest"
)

// startRequestIdServer 起一个只挂 RequestId 中间件的测试服务，
// 回显服务端 ID 与客户端追踪 ID，供本文件的用例复用。
//
// 注意：RequestId 依赖 ghttp 的响应对象与中间件链，无法脱离 server 直接构造
// ghttp.Request 来调用（本文件早期版本曾如此尝试，用的 API 并不存在，从未编译通过）。
func startRequestIdServer(t *gtest.T, name string) (*ghttp.Server, *gclient.Client) {
	s := g.Server(name)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(RequestId)
		group.ALL("/test", func(r *ghttp.Request) {
			r.Response.WriteJson(g.Map{
				"server_id": r.GetCtxVar("RequestId").String(),
				"client_id": r.GetCtxVar("ClientTraceId").String(),
			})
		})
	})
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.Start()

	client := g.Client()
	client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort()))
	return s, client
}

// TestRequestId_ServerIDGeneration 服务端 ID 必须每次强制生成且互不相同，
// 无论客户端是否传入 X-Request-Id。
func TestRequestId_ServerIDGeneration(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startRequestIdServer(t, "request-id-server-gen")
		defer s.Shutdown()

		ctx := gctx.New()

		// 场景 1：无客户端 ID
		body1 := client.GetContent(ctx, "/test")
		j1 := gjson.New(body1)
		serverID1 := j1.Get("server_id").String()
		t.AssertNE(serverID1, "")
		t.Assert(j1.Get("client_id").String(), "")

		// 场景 2：有效客户端 ID —— 服务端 ID 仍独立生成
		body2 := client.Header(g.MapStrStr{"X-Request-Id": "valid-client-123"}).
			GetContent(ctx, "/test")
		j2 := gjson.New(body2)
		serverID2 := j2.Get("server_id").String()
		t.AssertNE(serverID2, "")
		t.Assert(j2.Get("client_id").String(), "valid-client-123")

		// 两次请求的服务端 ID 必须不同（幂等键依赖其唯一性）
		t.AssertNE(serverID1, serverID2)
	})
}

// TestRequestId_ClientIDValidation 客户端追踪 ID 的格式校验：
// 仅接受字母/数字/连字符且长度 1-64，其余一律丢弃，防止日志注入与不可控的下游传播。
func TestRequestId_ClientIDValidation(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startRequestIdServer(t, "request-id-client-validation")
		defer s.Shutdown()

		ctx := gctx.New()

		tests := []struct {
			name         string
			clientID     string
			shouldAccept bool
		}{
			{"有效ID-字母数字连字符", "client-trace-123", true},
			{"有效ID-纯数字", "12345678", true},
			{"有效ID-纯字母", "abcdefgh", true},
			{"有效ID-64位边界", strings.Repeat("a", 64), true},
			{"非法ID-含空格", "invalid id", false},
			{"非法ID-超长65位", strings.Repeat("a", 65), false},
			{"非法ID-特殊字符", "id@#$%", false},
			{"非法ID-中文", "客户端ID", false},
		}

		for _, tt := range tests {
			body := client.Header(g.MapStrStr{"X-Request-Id": tt.clientID}).
				GetContent(ctx, "/test")
			clientID := gjson.New(body).Get("client_id").String()

			if tt.shouldAccept {
				t.AssertEQ(clientID, tt.clientID)
			} else {
				t.AssertEQ(clientID, "")
			}
		}
	})
}

// TestRequestId_ResponseHeaders 响应头回显规则：
//   - X-Teamapi-Request-Id 恒为服务端 ID
//   - X-Request-Id 在客户端 ID 有效时回显客户端 ID，否则回填服务端 ID
func TestRequestId_ResponseHeaders(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s, client := startRequestIdServer(t, "request-id-response-headers")
		defer s.Shutdown()

		ctx := gctx.New()

		tests := []struct {
			name      string
			clientID  string
			echoesSrv bool // X-Request-Id 是否应回显服务端 ID
		}{
			{"无客户端 ID", "", true},
			{"有效客户端 ID", "valid-123", false},
			{"非法客户端 ID", "invalid id with spaces", true},
		}

		for _, tt := range tests {
			resp, err := client.Header(g.MapStrStr{"X-Request-Id": tt.clientID}).
				Get(ctx, "/test")
			t.AssertNil(err)

			serverID := gjson.New(resp.ReadAllString()).Get("server_id").String()
			resp.Close()

			t.AssertEQ(resp.Header.Get("X-Teamapi-Request-Id"), serverID)
			if tt.echoesSrv {
				t.AssertEQ(resp.Header.Get("X-Request-Id"), serverID)
			} else {
				t.AssertEQ(resp.Header.Get("X-Request-Id"), tt.clientID)
			}
		}
	})
}
