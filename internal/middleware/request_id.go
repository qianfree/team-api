package middleware

import (
	"regexp"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// clientRequestIdRegexp 校验客户端传入的 X-Request-Id：仅允许字母、数字、连字符，
// 长度 1-64。拒绝含空白/特殊字符/超长的值，防止日志注入与不可控的下游传播。
var clientRequestIdRegexp = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// RequestId 双 ID 机制中间件：
//  1. 服务端强制生成唯一的内部 RequestId（用于幂等性控制、数据库主键）
//  2. 客户端可选传入追踪 ID（仅用于日志关联，不影响业务逻辑）
//  3. 通过 X-Teamapi-Request-Id 响应头返回服务端 ID
//  4. 通过 X-Request-Id 回显客户端追踪 ID（如果有效）
//
// 安全性说明：
//   - RequestId（服务端）：不可控，保证唯一性，用于幂等性判断（bil_records.request_id）
//   - ClientTraceId（客户端）：可选，仅用于日志追踪，不参与业务逻辑
func RequestId(r *ghttp.Request) {
	// 1. 服务端强制生成唯一 ID（用于业务幂等性控制）
	serverRequestId := guid.S()
	r.SetCtxVar("RequestId", serverRequestId)
	r.Response.Header().Set("X-Teamapi-Request-Id", serverRequestId)

	// 2. 客户端可选传入追踪 ID（仅用于日志关联）
	clientTraceId := r.GetHeader("X-Request-Id")
	if clientTraceId != "" && clientRequestIdRegexp.MatchString(clientTraceId) {
		r.SetCtxVar("ClientTraceId", clientTraceId)
		// 回显有效的客户端追踪 ID
		r.Response.Header().Set("X-Request-Id", clientTraceId)
	} else {
		// 客户端未传或格式非法，使用服务端 ID 填充
		r.Response.Header().Set("X-Request-Id", serverRequestId)
	}

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
