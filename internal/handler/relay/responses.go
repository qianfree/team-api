package relay

import (
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	relay_constant "github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/handler"
)

// HandleResponsesRetrieve 处理 GET /v1/responses/{id}（retrieve，background 模式轮询用）
func HandleResponsesRetrieve(r *ghttp.Request) {
	handleResponsesLifecycle(r, http.MethodGet)
}

// HandleResponsesCancel 处理 POST /v1/responses/{id}/cancel（取消进行中的响应）
func HandleResponsesCancel(r *ghttp.Request) {
	handleResponsesLifecycle(r, http.MethodPost)
}

// HandleResponsesDelete 处理 DELETE /v1/responses/{id}（删除已存储的响应）
func HandleResponsesDelete(r *ghttp.Request) {
	handleResponsesLifecycle(r, http.MethodDelete)
}

// handleResponsesLifecycle Responses 生命周期端点入口：
// 无请求体（ApiKeyAuth 纯 header 鉴权、ContentFilter 空 body 放行），
// 不走 validateRelayRequest（无 model），scope/IP 校验对齐 HandleModels 先例。
func handleResponsesLifecycle(r *ghttp.Request, method string) {
	responseID := r.Get("id").String()
	if responseID == "" {
		r.Response.WriteStatus(http.StatusBadRequest, g.Map{
			"error": g.Map{"type": "invalid_request_error", "message": "response id is required"},
		})
		return
	}

	rc := buildRelayContext(r)
	if !billingProvider.CheckScope(rc.Scope, "responses") {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("API key scope denied"))
		return
	}
	if !billingProvider.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
		handler.WriteRelayError(r.Response.Writer, relay_constant.NewAuthError("IP address is not allowed"))
		return
	}

	if err := handler.HandleResponsesLifecycle(r.Context(), method, responseID, rc, dataProvider); err != nil {
		handler.WriteRelayError(r.Response.Writer, err)
	}
}
