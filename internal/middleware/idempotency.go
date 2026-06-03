package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/response"
)

const (
	IdempotencyHeader = "Idempotency-Key"
	// maxCachedBodyLen limits the response body size stored for idempotent replay.
	// Responses larger than this will still be deduplicated but won't cache the body.
	maxCachedBodyLen = 64 * 1024 // 64 KB
)

// Idempotency ensures that requests with the same Idempotency-Key
// are processed only once. Subsequent requests with the same key
// return the cached response.
func Idempotency(r *ghttp.Request) {
	idempotencyKey := r.GetHeader(IdempotencyHeader)
	if idempotencyKey == "" {
		r.Middleware.Next()
		return
	}

	ctx := r.Context()

	// Check if this key has been processed before
	var record struct {
		Id           int64  `json:"id"`
		Status       string `json:"status"`
		ResponseBody string `json:"response_body"`
	}

	err := dao.SysIdempotencyRecords.Ctx(ctx).
		Where("idempotency_key", idempotencyKey).
		Where("expires_at > NOW()").
		Scan(&record)
	if err != nil {
		// DB error: fail-closed to prevent duplicate processing
		response.ErrorMsg(r, 500, "系统繁忙，请稍后重试")
		return
	}

	if record.Id > 0 {
		if record.Status == "completed" && record.ResponseBody != "" {
			r.Response.WriteStatus(r.Response.Status)
			r.Response.Write([]byte(record.ResponseBody))
			r.Exit()
			return
		}
		if record.Status == "processing" {
			writeConflictResponse(r)
			return
		}
	}

	// Insert processing record
	_, err = dao.SysIdempotencyRecords.Ctx(ctx).Data(do.SysIdempotencyRecords{
		IdempotencyKey: idempotencyKey,
		Status:         "processing",
	}).Insert()

	if err != nil {
		// If insert fails (duplicate key), another request is already processing
		writeConflictResponse(r)
		return
	}

	// Capture the response
	r.Middleware.Next()

	// After processing, update the record with cached body
	status := "completed"
	statusCode := r.Response.Status
	if statusCode >= 400 {
		status = "failed"
	}

	var responseBody string
	if buf := r.Response.Buffer(); len(buf) > 0 && len(buf) <= maxCachedBodyLen {
		responseBody = string(buf)
	}

	// Update the record
	dao.SysIdempotencyRecords.Ctx(ctx).
		Where("idempotency_key", idempotencyKey).
		Data(do.SysIdempotencyRecords{
			Status:       status,
			ResponseBody: responseBody,
		}).Update()
}

// writeConflictResponse writes a 409 Conflict response for duplicate idempotency keys.
func writeConflictResponse(r *ghttp.Request) {
	r.Response.WriteStatus(409)
	r.Response.WriteJson(g.Map{
		"error": g.Map{
			"type":    "conflict",
			"message": "请求正在处理中，请勿重复提交",
			"code":    "409",
		},
	})
	r.Exit()
}
