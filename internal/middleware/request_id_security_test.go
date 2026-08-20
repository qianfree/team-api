package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/testutil"
)

// TestRequestId_ResponseHeaders 验证双 ID 机制的响应头契约：
// X-Teamapi-Request-Id 恒为服务端生成的 ID（不可被客户端控制），
// X-Request-Id 仅在客户端 ID 合法时回显，否则回落为服务端 ID。
func TestRequestId_ResponseHeaders(t *testing.T) {
	server := g.Server("request-id-headers-test")
	server.Use(RequestId)
	server.BindHandler("/test", func(r *ghttp.Request) {
		r.Response.WriteJson(g.Map{
			"server_id": r.GetCtxVar("RequestId").String(),
		})
	})
	baseURL := testutil.StartGFServer(t, server)

	tests := []struct {
		name             string
		clientID         string
		expectEchoClient bool
	}{
		{"无客户端 ID", "", false},
		{"有效客户端 ID", "valid-123", true},
		{"非法客户端 ID（含空格）", "invalid id with spaces", false},
		{"非法客户端 ID（超长）", strings.Repeat("a", 65), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, baseURL+"/test", nil)
			if err != nil {
				t.Fatalf("构造请求失败: %v", err)
			}
			req.Header.Set("X-Request-Id", tt.clientID)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET /test: %v", err)
			}
			defer resp.Body.Close()

			var body struct {
				ServerID string `json:"server_id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("解析响应失败: %v", err)
			}
			if body.ServerID == "" {
				t.Fatal("服务端 ID 必须生成")
			}

			if got := resp.Header.Get("X-Teamapi-Request-Id"); got != body.ServerID {
				t.Errorf("X-Teamapi-Request-Id = %q, want 服务端 ID %q", got, body.ServerID)
			}
			wantEcho := map[bool]string{true: tt.clientID, false: body.ServerID}[tt.expectEchoClient]
			if got := resp.Header.Get("X-Request-Id"); got != wantEcho {
				t.Errorf("X-Request-Id = %q, want %q", got, wantEcho)
			}
		})
	}
}

// TestServiceNameHeader 验证 ServiceName 中间件在所有响应上写入 X-Service-Name。
func TestServiceNameHeader(t *testing.T) {
	server := g.Server("service-name-test")
	server.Use(ServiceName)
	server.BindHandler("/ping", func(r *ghttp.Request) {
		r.Response.WriteHeader(http.StatusNoContent)
	})
	baseURL := testutil.StartGFServer(t, server)

	resp, err := http.Get(baseURL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Service-Name"); got != serviceName {
		t.Errorf("X-Service-Name = %q, want %q", got, serviceName)
	}
}
