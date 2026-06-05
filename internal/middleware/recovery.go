package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/response"
)

// Recovery catches panics, logs them, writes to sys_error_logs, and returns 500.
func Recovery(r *ghttp.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			stack := string(debug.Stack())
			requestID := ""
			if v := r.GetCtxVar("RequestId"); v != nil {
				requestID = v.String()
			}

			errMsg := fmt.Sprintf("panic: %v", rec)
			g.Log().Criticalf(r.Context(), "[Recovery] %s\nRequest-ID: %s\nPath: %s\n%s",
				errMsg, requestID, r.URL.Path, stack)

			// Persist panic to sys_error_logs
			response.WritePanicLog(requestID, r.Method, r.URL.Path, errMsg, stack)

			r.Response.WriteHeader(500)
			r.Response.WriteJson(g.Map{
				"code":       500,
				"message":    "服务器内部错误",
				"data":       nil,
				"request_id": requestID,
			})
			r.SetError(nil)
			r.ExitAll()
		}
	}()
	r.Middleware.Next()
}
