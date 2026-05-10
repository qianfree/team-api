package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/response"
)

// ErrorHandler is a unified error response middleware.
// It replaces GoFrame's default MiddlewareHandlerResponse.
// All sanitization and logging is delegated to response.Error().
func ErrorHandler(r *ghttp.Request) {
	r.Middleware.Next()

	// If there's no error, skip
	if r.GetError() == nil {
		return
	}

	err := r.GetError()

	// Only log system-level errors; skip business & client errors
	if isSystemError(err) {
		g.Log().Warningf(r.Context(), "[ErrorHandler] path=%s, error=%+v, type=%T", r.URL.Path, err, err)
	}

	// Delegate to response.Error for unified sanitization
	response.Error(r, err)
}
