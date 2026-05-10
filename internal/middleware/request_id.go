package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// RequestId generates a unique request ID for each request,
// injects it into the context and returns it in the response header.
func RequestId(r *ghttp.Request) {
	requestId := r.GetHeader("X-Request-Id")
	if requestId == "" {
		requestId = guid.S()
	}

	r.SetCtxVar("RequestId", requestId)
	r.Response.Header().Set("X-Request-Id", requestId)

	r.Middleware.Next()
}
