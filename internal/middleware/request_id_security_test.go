package middleware

import (
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
)

// TestRequestId_ServerIDGeneration 测试服务端 ID 强制生成
func TestRequestId_ServerIDGeneration(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 模拟请求上下文
		r := &ghttp.Request{}
		r.Request = gtest.NewRequest("GET", "/test")
		r.Response = ghttp.DefaultServer().GetRouterTree()

		// 测试场景1：无客户端 ID
		r.Request.Header.Set("X-Request-Id", "")
		RequestId(r)

		serverID := r.GetCtxVar("RequestId").String()
		clientID := r.GetCtxVar("ClientTraceId").String()

		t.Assert(serverID != "", true, "服务端 ID 必须生成")
		t.Assert(clientID, "", "无客户端 ID 时 ClientTraceId 应为空")

		// 测试场景2：有效客户端 ID
		r2 := &ghttp.Request{}
		r2.Request = gtest.NewRequest("GET", "/test")
		r2.Request.Header.Set("X-Request-Id", "valid-client-123")
		r2.Response = ghttp.DefaultServer().GetRouterTree()

		RequestId(r2)

		serverID2 := r2.GetCtxVar("RequestId").String()
		clientID2 := r2.GetCtxVar("ClientTraceId").String()

		t.Assert(serverID2 != "", true, "服务端 ID 必须生成")
		t.Assert(clientID2, "valid-client-123", "有效客户端 ID 应保存")
		t.Assert(serverID != serverID2, true, "两次请求的服务端 ID 必须不同")
	})
}

// TestRequestId_ClientIDValidation 测试客户端 ID 格式校验
func TestRequestId_ClientIDValidation(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			name         string
			clientID     string
			shouldAccept bool
		}{
			{"有效ID-字母数字连字符", "client-trace-123", true},
			{"有效ID-纯数字", "12345678", true},
			{"有效ID-纯字母", "abcdefgh", true},
			{"非法ID-含空格", "invalid id", false},
			{"非法ID-超长", string(make([]byte, 65)), false},
			{"非法ID-特殊字符", "id@#$%", false},
			{"非法ID-中文", "客户端ID", false},
		}

		for _, tt := range tests {
			r := &ghttp.Request{}
			r.Request = gtest.NewRequest("GET", "/test")
			r.Request.Header.Set("X-Request-Id", tt.clientID)
			r.Response = ghttp.DefaultServer().GetRouterTree()

			RequestId(r)

			clientID := r.GetCtxVar("ClientTraceId").String()

			if tt.shouldAccept {
				t.Assert(clientID, tt.clientID, tt.name+" 应被接受")
			} else {
				t.Assert(clientID, "", tt.name+" 应被拒绝")
			}
		}
	})
}

// TestRequestId_SecurityIsolation 测试安全隔离
func TestRequestId_SecurityIsolation(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		serverIDs := make(map[string]bool)
		maliciousClientID := "malicious-repeated-id"

		// 模拟恶意客户端：10 次请求使用相同的客户端 ID
		for i := 0; i < 10; i++ {
			r := &ghttp.Request{}
			r.Request = gtest.NewRequest("GET", "/test")
			r.Request.Header.Set("X-Request-Id", maliciousClientID)
			r.Response = ghttp.DefaultServer().GetRouterTree()

			RequestId(r)

			serverID := r.GetCtxVar("RequestId").String()
			serverIDs[serverID] = true
		}

		// 验证：每次请求的服务端 ID 必须不同
		t.Assert(len(serverIDs), 10, "10 次请求应生成 10 个不同的服务端 ID")
	})
}

// TestRequestId_ResponseHeaders 测试响应头设置
func TestRequestId_ResponseHeaders(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		tests := []struct {
			name                    string
			clientID                string
			expectXTeamapiRequestId bool
			expectXRequestIdEcho    string
		}{
			{
				name:                    "无客户端 ID",
				clientID:                "",
				expectXTeamapiRequestId: true,
				expectXRequestIdEcho:    "server", // 应回显服务端 ID
			},
			{
				name:                    "有效客户端 ID",
				clientID:                "valid-123",
				expectXTeamapiRequestId: true,
				expectXRequestIdEcho:    "valid-123", // 应回显客户端 ID
			},
			{
				name:                    "非法客户端 ID",
				clientID:                "invalid id with spaces",
				expectXTeamapiRequestId: true,
				expectXRequestIdEcho:    "server", // 应回显服务端 ID
			},
		}

		for _, tt := range tests {
			r := &ghttp.Request{}
			r.Request = gtest.NewRequest("GET", "/test")
			r.Request.Header.Set("X-Request-Id", tt.clientID)
			r.Response = ghttp.DefaultServer().GetRouterTree()

			RequestId(r)

			serverID := r.GetCtxVar("RequestId").String()
			xTeamapiRequestId := r.Response.Header().Get("X-Teamapi-Request-Id")
			xRequestId := r.Response.Header().Get("X-Request-Id")

			// 验证 X-Teamapi-Request-Id
			if tt.expectXTeamapiRequestId {
				t.Assert(xTeamapiRequestId, serverID, tt.name+": X-Teamapi-Request-Id 应等于服务端 ID")
			}

			// 验证 X-Request-Id 回显逻辑
			if tt.expectXRequestIdEcho == "server" {
				t.Assert(xRequestId, serverID, tt.name+": X-Request-Id 应回显服务端 ID")
			} else {
				t.Assert(xRequestId, tt.expectXRequestIdEcho, tt.name+": X-Request-Id 应回显客户端 ID")
			}
		}
	})
}
