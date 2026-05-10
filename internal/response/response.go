package response

import (
	"errors"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// jsonResp is the unified response structure for all API endpoints.
type jsonResp struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
}

// Success writes a successful response: HTTP 200 + {"code":0, "message":"ok", "data":..., "request_id":...}.
func Success(r *ghttp.Request, data interface{}) {
	requestID := getRequestID(r)
	r.Response.WriteJson(jsonResp{
		Code:      0,
		Message:   "ok",
		Data:      data,
		RequestID: requestID,
	})
	r.SetError(nil) // 清除请求上的错误，防止 ErrorHandler 中间件重复处理
}

// Error writes an error response with automatic sanitization.
//
// Security: Only business errors (code >= 10000) and standard HTTP 4xx errors
// expose their messages to the client. All other errors (database, network,
// internal) are replaced with "服务器内部错误" to prevent information leakage.
// The original error is logged with request_id for debugging.
func Error(r *ghttp.Request, err error) {
	code := 500
	msg := "服务器内部错误"

	// Extract code from GoFrame gerror (supports wrapped errors via errors.As)
	var gerr *gerror.Error
	if errors.As(err, &gerr) {
		code = gerr.Code().Code()

		// Business errors (>= 10000) and HTTP 4xx have safe user-facing messages
		if code >= 10000 || (code >= 400 && code < 500) {
			msg = gerr.Error()
		} else {
			// Internal error — sanitize message, log real cause
			g.Log().Warning(r.Context(), "[Response] Internal error sanitized",
				"request_id", getRequestID(r),
				"code", code,
				"error", err.Error(),
			)
			// Persist system error to sys_error_logs
			writeErrorLog(&ErrorLogRecord{
				RequestId:    getRequestID(r),
				ErrorCode:    code,
				ErrorMessage: err.Error(),
				StackTrace:   captureStackTrace(3),
				HttpMethod:   r.Method,
				RequestPath:  r.URL.Path,
				RequestBody:  truncateString(r.GetBodyString(), 2000),
				Source:       "api",
			})
		}
	} else {
		// Non-gerror (raw Go error, e.g. database driver error) — always sanitize
		g.Log().Warning(r.Context(), "[Response] Raw error sanitized",
			"request_id", getRequestID(r),
			"error", err.Error(),
		)
		// Persist raw error to sys_error_logs
		writeErrorLog(&ErrorLogRecord{
			RequestId:    getRequestID(r),
			ErrorCode:    500,
			ErrorMessage: err.Error(),
			StackTrace:   captureStackTrace(3),
			HttpMethod:   r.Method,
			RequestPath:  r.URL.Path,
			RequestBody:  truncateString(r.GetBodyString(), 2000),
			Source:       "api",
		})
	}

	if code == 0 {
		code = 500
	}

	// Determine HTTP status
	httpStatus := code
	if code >= 10000 {
		httpStatus = 422
	} else if httpStatus < 100 || httpStatus > 599 {
		// GoFrame gcode 可能返回非标准错误码（如 68=CodeNotModified），
		// 不能直接用作 HTTP 状态码，否则 WriteHeader 会 panic
		httpStatus = 500
	}

	requestID := getRequestID(r)
	r.Response.WriteHeader(httpStatus)
	r.Response.WriteJson(jsonResp{
		Code:      code,
		Message:   msg,
		Data:      nil,
		RequestID: requestID,
	})
	r.SetError(nil) // 清除请求上的错误，防止 ErrorHandler 中间件重复处理
}

// Use this for 400/401/403/404/409/429/500 errors.
func ErrorMsg(r *ghttp.Request, code int, message string) {
	ErrorWithCode(r, code, code, message)
}

// ErrorWithCode writes an error with separate HTTP status and business code.
// Use this when HTTP status differs from the business code (e.g., HTTP 401 with code 10020).
func ErrorWithCode(r *ghttp.Request, httpStatus int, code int, message string) {
	requestID := getRequestID(r)
	r.Response.WriteHeader(httpStatus)
	r.Response.WriteJson(jsonResp{
		Code:      code,
		Message:   message,
		Data:      nil,
		RequestID: requestID,
	})
	r.SetError(nil) // 清除请求上的错误，防止 ErrorHandler 中间件重复处理
}

// getRequestID extracts the request ID from context.
func getRequestID(r *ghttp.Request) string {
	if v := r.GetCtxVar("RequestId"); v != nil {
		return v.String()
	}
	return ""
}
