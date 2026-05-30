package middleware

import (
	"errors"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/response"
)

// downloadContentTypes lists Content-Type prefixes that indicate a file download.
// When matched, the middleware skips JSON wrapping to avoid appending {"code":0,...} to the file.
var downloadContentTypes = []string{
	"text/csv",
	"application/vnd.openxmlformats",
	"application/octet-stream",
}

// MiddlewareHandlerResponse is the GoFrame standard response handler.
// It reads the controller method's return value via r.GetHandlerResponse()
// and wraps it in the project's custom response format (code/message/data/request_id).
func MiddlewareHandlerResponse(r *ghttp.Request) {
	r.Middleware.Next()

	// If there's an error set by the controller
	if err := r.GetError(); err != nil {
		// Only log system-level errors (5xx / unknown); skip business & client errors
		// to avoid polluting logs with normal validation failures, auth errors, etc.
		if isSystemError(err) {
			g.Log().Warningf(r.Context(), "[HandlerResponse] path=%s, body=%s, error=%+v, type=%T", r.URL.Path, r.GetBodyString(), err, err)
		}
		response.Error(r, err)
		return
	}

	// Skip JSON wrapping for file downloads (CSV, Excel, etc.)
	// Export functions write directly to the raw writer and set Content-Type before writing.
	ct := r.Response.Header().Get("Content-Type")
	for _, prefix := range downloadContentTypes {
		if strings.HasPrefix(ct, prefix) {
			return
		}
	}

	// Skip JSON wrapping when handler has already written directly to the response
	// (e.g., model export sets Content-Disposition: attachment).
	// This also works around Go's nil-interface trap: a typed nil pointer (*T)(nil)
	// stored in interface{} is != nil, causing GetHandlerResponse() to appear non-nil
	// even though the handler returned nil.
	if r.Response.Header().Get("Content-Disposition") != "" {
		return
	}

	// If the handler already wrote body content and returned a nil-ish result,
	// don't append standard response wrapper on top of it.
	if r.Response.BufferLength() > 0 && r.GetHandlerResponse() == nil {
		return
	}

	// The controller method returns (res, error); GoFrame stores the first
	// return value in r.handlerResponse, accessible via GetHandlerResponse().
	res := r.GetHandlerResponse()
	if res != nil {
		response.Success(r, res)
		return
	}

	// Void response (e.g., delete/update operations returning nil pointer)
	if r.Response.BufferLength() == 0 {
		response.Success(r, nil)
	}
}

// isSystemError returns true if the error is a system-level error (5xx / unknown)
// that should be logged. Business errors (4xx, >= 10000) and client errors are
// considered normal flow and should NOT pollute logs.
func isSystemError(err error) bool {
	var gerr *gerror.Error
	if !errors.As(err, &gerr) {
		// Raw Go error (no gerror code) — treat as system error
		return true
	}
	code := gerr.Code().Code()
	// Client errors (400-499): validation, auth, not found, etc.
	if code >= 400 && code < 500 {
		return false
	}
	// Business rule errors (>= 10000): insufficient balance, quota exceeded, etc.
	if code >= 10000 {
		return false
	}
	return true
}
