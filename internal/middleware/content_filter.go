package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

const (
	// contentFilterLogWorkers controls the concurrency for async log writes.
	contentFilterLogWorkers = 4
	// contentFilterLogQueueSize is the buffer size for the log write channel.
	contentFilterLogQueueSize = 256
)

// Context variable keys for content filter results.
const (
	CtxKeyContentFilterMatched  = "ContentFilterMatched"
	CtxKeyContentFilterWords    = "ContentFilterWords"
	CtxKeyContentFilterReplaced = "ContentFilterReplaced"
	CtxKeyContentFilterOriginal = "ContentFilterOriginalBody"
	CtxKeyContentFilterFiltered = "ContentFilterReplacedBody"
)

var contentFilterLogCh = make(chan *contentFilterLogEntry, contentFilterLogQueueSize)

type contentFilterLogEntry struct {
	TenantId     int64
	UserId       int64
	ApiKeyId     int64
	ProjectId    int64
	RequestId    string
	Method       string
	Path         string
	ClientIp     string
	FilterMode   string
	MatchedWords string
	Snippet      string
	Blocked      bool
}

func init() {
	for i := 0; i < contentFilterLogWorkers; i++ {
		go contentFilterLogWorker()
	}
}

func contentFilterLogWorker() {
	ctx := context.Background()
	for entry := range contentFilterLogCh {
		_, err := common.AuditModelCtx(ctx, "aud_content_filter_logs").Data(do.AudContentFilterLogs{
			TenantId:        entry.TenantId,
			UserId:          entry.UserId,
			ApiKeyId:        entry.ApiKeyId,
			ProjectId:       entry.ProjectId,
			RequestId:       entry.RequestId,
			Method:          entry.Method,
			Path:            entry.Path,
			ClientIp:        entry.ClientIp,
			FilterMode:      entry.FilterMode,
			MatchedWords:    entry.MatchedWords,
			OriginalSnippet: entry.Snippet,
			Blocked:         entry.Blocked,
		}).Insert()
		if err != nil {
			g.Log().Warningf(ctx, "[ContentFilter] failed to insert filter log: %v", err)
		}
	}
}

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

	// Queue async log write
	queueContentFilterLog(r, mode, result.MatchedWords, string(body))

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

// queueContentFilterLog queues a content filter log entry for async writing.
// If the queue is full, the log is dropped with a warning to avoid blocking the request.
func queueContentFilterLog(r *ghttp.Request, mode string, matchedWords []string, body string) {
	snippet := ""
	if mode == "replace" && len(body) > 0 {
		const maxSnippet = 500
		if len(body) > maxSnippet {
			snippet = body[:maxSnippet]
		} else {
			snippet = body
		}
	}

	wordsJSON, _ := json.Marshal(matchedWords)

	entry := &contentFilterLogEntry{
		TenantId:     r.GetCtxVar(CtxKeyTenantID).Int64(),
		UserId:       r.GetCtxVar(CtxKeyUserID).Int64(),
		ApiKeyId:     r.GetCtxVar(CtxKeyApiKeyID).Int64(),
		ProjectId:    r.GetCtxVar(CtxKeyProjectID).Int64(),
		RequestId:    r.GetCtxVar("RequestId").String(),
		Method:       r.Method,
		Path:         r.URL.Path,
		ClientIp:     r.GetClientIp(),
		FilterMode:   mode,
		MatchedWords: string(wordsJSON),
		Snippet:      snippet,
		Blocked:      mode == "block",
	}

	select {
	case contentFilterLogCh <- entry:
	default:
		g.Log().Warningf(r.Context(), "[ContentFilter] log queue full, dropping entry for %s", r.URL.Path)
	}
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
