package middleware

import (
	"net/http"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	"github.com/qianfree/team-api/internal/testutil"
)

// TestGlobalResponseHeaders 验证两个全局响应头中间件：
// RequestId 生成/透传 X-Request-Id，ServiceName 在所有响应上写入 X-Service-Name。
func TestGlobalResponseHeaders(t *testing.T) {
	server := g.Server(guid.S())
	server.Use(RequestId)
	server.Use(ServiceName)
	server.BindHandler("/ping", func(r *ghttp.Request) {
		r.Response.WriteHeader(http.StatusNoContent)
	})

	baseURL := testutil.StartGFServer(t, server)

	t.Run("服务端生成请求 ID 并写入服务标识头", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/ping")
		if err != nil {
			t.Fatalf("GET /ping: %v", err)
		}
		defer resp.Body.Close()

		if got := resp.Header.Get("X-Service-Name"); got != serviceName {
			t.Errorf("X-Service-Name = %q, want %q", got, serviceName)
		}
		if got := resp.Header.Get("X-Request-Id"); !clientRequestIdRegexp.MatchString(got) {
			t.Errorf("X-Request-Id = %q, want 服务端生成的合法 ID", got)
		}
	})

	t.Run("合法的客户端请求 ID 被透传", func(t *testing.T) {
		const clientID = "client-req-123"
		req, err := http.NewRequest(http.MethodGet, baseURL+"/ping", nil)
		if err != nil {
			t.Fatalf("构造请求失败: %v", err)
		}
		req.Header.Set("X-Request-Id", clientID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /ping: %v", err)
		}
		defer resp.Body.Close()

		if got := resp.Header.Get("X-Request-Id"); got != clientID {
			t.Errorf("X-Request-Id = %q, want %q", got, clientID)
		}
	})
}
