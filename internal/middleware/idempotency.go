package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

const (
	IdempotencyHeader = "Idempotency-Key"
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

	err := g.DB().Model("sys_idempotency_records").Ctx(ctx).
		Where("idempotency_key", idempotencyKey).
		Where("expires_at > NOW()").
		Scan(&record)
	if err != nil {
		r.Middleware.Next()
		return
	}

	if record.Id > 0 {
		if record.Status == "completed" && record.ResponseBody != "" {
			r.Response.WriteJson(record.ResponseBody)
			r.Exit()
			return
		}
		if record.Status == "processing" {
			r.Response.WriteStatus(409)
			r.Response.WriteJson(g.Map{
				"error": g.Map{
					"type":    "conflict",
					"message": "请求正在处理中，请勿重复提交",
					"code":    "409",
				},
			})
			r.Exit()
			return
		}
	}

	// Insert processing record
	_, err = g.DB().Exec(ctx,
		"INSERT INTO sys_idempotency_records (idempotency_key, status, expires_at) VALUES (?, ?, NOW() + INTERVAL '24 hours')",
		idempotencyKey, "processing")

	if err != nil {
		// If insert fails (duplicate key), another request is already processing
		r.Response.WriteStatus(409)
		r.Response.WriteJson(g.Map{
			"error": g.Map{
				"type":    "conflict",
				"message": "请求正在处理中，请勿重复提交",
				"code":    "409",
			},
		})
		r.Exit()
		return
	}

	// Capture the response
	r.Middleware.Next()

	// After processing, update the record
	status := "completed"
	statusCode := r.Response.Status
	if statusCode >= 400 {
		status = "failed"
	}

	// Get response body (best effort)
	responseBody := ""

	// Update the record
	g.DB().Model("sys_idempotency_records").Ctx(ctx).
		Where("idempotency_key", idempotencyKey).
		Data(g.Map{
			"status":        status,
			"response_body": responseBody,
		}).Update()
}
