package middleware

import (
	"regexp"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// clientRequestIdRegexp 校验客户端传入的 X-Request-Id：仅允许字母、数字、连字符，
// 长度 1-64。拒绝含空白/特殊字符/超长的值，防止日志注入与不可控的下游传播。
// 不合法时丢弃客户端值并改用服务端生成的 ID。
var clientRequestIdRegexp = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// RequestId generates a unique request ID for each request,
// injects it into the context and returns it in the response header.
func RequestId(r *ghttp.Request) {
	requestId := r.GetHeader("X-Request-Id")
	// 客户端可传入请求追踪 ID，但必须通过格式校验；不合法则忽略，改用服务端生成
	if requestId == "" || !clientRequestIdRegexp.MatchString(requestId) {
		requestId = guid.S()
	}

	r.SetCtxVar("RequestId", requestId)
	r.Response.Header().Set("X-Request-Id", requestId)

	r.Middleware.Next()
}

// serviceName 随响应头 X-Service-Name 返回的稳定服务标识。
const serviceName = "team-api"

// ServiceName 在每个响应上写入服务标识头 X-Service-Name，
// 与 RequestId 同属全局注册的响应头中间件。
func ServiceName(r *ghttp.Request) {
	r.Response.Header().Set("X-Service-Name", serviceName)
	r.Middleware.Next()
}
