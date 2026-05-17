package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/common"
)

// Context variable keys for content filter results.
const (
	CtxKeyContentFilterMatched  = "ContentFilterMatched"
	CtxKeyContentFilterWords    = "ContentFilterWords"
	CtxKeyContentFilterReplaced = "ContentFilterReplaced"
	CtxKeyContentFilterOriginal = "ContentFilterOriginalBody"
	CtxKeyContentFilterFiltered = "ContentFilterReplacedBody"
)

// ContentFilter is middleware that checks request body for sensitive content.
// Behavior depends on the content_filter_mode setting:
//   - "off":    skip entirely
//   - "log":    record match in context, pass through
//   - "replace": store filtered body in context, pass through
//   - "block":  reject request with relay-native error format
func ContentFilter(r *ghttp.Request) {
	mode := common.ContentFilter().GetMode()
	if mode == "off" {
		r.Middleware.Next()
		return
	}

	// Only check requests with a body
	body := r.GetBody()
	if len(body) == 0 {
		r.Middleware.Next()
		return
	}

	result := common.ContentFilter().Check(body)
	if !result.Matched {
		r.Middleware.Next()
		return
	}

	// Set context variables for downstream logging/audit
	r.SetCtxVar(CtxKeyContentFilterMatched, true)
	r.SetCtxVar(CtxKeyContentFilterWords, result.MatchedWords)

	// Asynchronously persist the interception log
	insertContentFilterLog(r, mode, result.MatchedWords, string(body))

	switch mode {
	case "log":
		g.Log().Infof(r.Context(), "[ContentFilter] matched words: %v", result.MatchedWords)
		r.Middleware.Next()

	case "replace":
		g.Log().Infof(r.Context(), "[ContentFilter] replaced words: %v", result.MatchedWords)
		r.SetCtxVar(CtxKeyContentFilterReplaced, true)
		r.SetCtxVar(CtxKeyContentFilterOriginal, string(body))
		r.SetCtxVar(CtxKeyContentFilterFiltered, result.FilteredText)
		r.Middleware.Next()

	case "block":
		g.Log().Warningf(r.Context(), "[ContentFilter] blocked request, matched words: %v", result.MatchedWords)
		responseMsg := common.Config().GetString(r.Context(), "content_filter_response")
		if responseMsg == "" {
			responseMsg = "内容包含敏感词，请修改后重试"
		}
		writeContentFilterError(r, http.StatusBadRequest, "content_filter_error", responseMsg)
	}
}

// insertContentFilterLog persists a content filter interception record asynchronously.
func insertContentFilterLog(r *ghttp.Request, mode string, matchedWords []string, body string) {
	tenantId := r.GetCtxVar(CtxKeyTenantID).Int64()
	userId := r.GetCtxVar(CtxKeyUserID).Int64()
	apiKeyId := r.GetCtxVar(CtxKeyApiKeyID).Int64()
	projectId := r.GetCtxVar(CtxKeyProjectID).Int64()
	requestId := r.GetCtxVar("RequestId").String()
	method := r.Method
	path := r.URL.Path
	clientIp := r.GetClientIp()

	// Truncate body snippet for replace mode only
	snippet := ""
	if mode == "replace" && len(body) > 0 {
		const maxSnippet = 500
		if len(body) > maxSnippet {
			snippet = body[:maxSnippet]
		} else {
			snippet = body
		}
	}

	blocked := mode == "block"
	wordsJSON, _ := json.Marshal(matchedWords)

	go func() {
		_, err := g.DB().Exec(context.Background(),
			`INSERT INTO aud_content_filter_logs
			 (tenant_id, user_id, api_key_id, project_id, request_id, method, path, client_ip,
			  filter_mode, matched_words, original_snippet, blocked, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())`,
			tenantId, userId, apiKeyId, projectId, requestId, method, path, clientIp,
			mode, string(wordsJSON), snippet, blocked)
		if err != nil {
			g.Log().Warningf(context.Background(),
				"[ContentFilter] failed to insert filter log: %v", err)
		}
	}()
}

// writeContentFilterError writes an error response in the relay-native format.
// /v1/messages uses Claude format, all other paths use OpenAI format.
func writeContentFilterError(r *ghttp.Request, statusCode int, errType, message string) {
	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.WriteHeader(statusCode)

	path := r.URL.Path
	if path == "/v1/messages" {
		body, _ := json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    errType,
				"message": message,
			},
		})
		r.Response.Write(body)
	} else {
		body, _ := json.Marshal(map[string]any{
			"error": map[string]string{
				"type":    errType,
				"message": message,
			},
		})
		r.Response.Write(body)
	}
}

// GetContentFilterMatched extracts the content filter matched flag from context.
func GetContentFilterMatched(ctx context.Context) bool {
	val := ctx.Value(CtxKeyContentFilterMatched)
	if val == nil {
		return false
	}
	if matched, ok := val.(bool); ok {
		return matched
	}
	return false
}

// GetContentFilterWords extracts the matched sensitive words from context.
func GetContentFilterWords(ctx context.Context) []string {
	val := ctx.Value(CtxKeyContentFilterWords)
	if val == nil {
		return nil
	}
	if words, ok := val.([]string); ok {
		return words
	}
	return nil
}

// GetContentFilterReplacedBody extracts the filtered body from context (replace strategy).
func GetContentFilterReplacedBody(ctx context.Context) string {
	val := ctx.Value(CtxKeyContentFilterFiltered)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}
