// Package relay —— /v1/files（OpenAI 兼容素材上传）端点胶水。
//
// 与 videos.go 同型：解析协议请求 → 调业务逻辑 → 按 OpenAI 原生格式输出
// （不走统一 JSON 包装）。非模型调用，不进计费/用量/审计链路。
package relay

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	commonlogic "github.com/qianfree/team-api/internal/logic/common"
	relaylogic "github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/middleware"
)

// HandleFilesUpload 处理 POST /v1/files（multipart 上传素材）。
//
// 单文件上限由全局 clientMaxBodySize（RelayBodyLimit 中间件）约束，
// 与 /v1/audio/transcriptions 等既有上传端点口径一致。
func HandleFilesUpload(r *ghttp.Request) {
	ctx := r.Context()

	fileHeader := r.GetUploadFile("file")
	if fileHeader == nil {
		writeFilesError(r, 400, "the 'file' multipart form part is required", "")
		return
	}
	purpose := r.Get("purpose").String()

	f, err := fileHeader.Open()
	if err != nil {
		writeFilesError(r, 400, "failed to open uploaded file: "+err.Error(), "")
		return
	}
	defer f.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	obj, err := relaylogic.UploadRelayFile(ctx,
		middleware.GetTenantID(ctx), middleware.GetUserID(ctx),
		f, fileHeader.Filename, contentType, fileHeader.Size, purpose)
	if err != nil {
		writeFilesLogicError(r, err)
		return
	}
	r.Response.WriteJsonExit(obj)
}

// HandleFilesList 处理 GET /v1/files（列出当前租户的上传素材）。
func HandleFilesList(r *ghttp.Request) {
	ctx := r.Context()

	list, err := relaylogic.ListRelayFiles(ctx,
		middleware.GetTenantID(ctx),
		r.Get("purpose").String(),
		r.Get("limit", 20).Int())
	if err != nil {
		writeFilesLogicError(r, err)
		return
	}
	r.Response.WriteJsonExit(g.Map{"object": "list", "data": list})
}

// HandleFileRetrieve 处理 GET /v1/files/{file_id}（查询文件元数据）。
func HandleFileRetrieve(r *ghttp.Request) {
	fileID := r.Get("file_id").String()
	if fileID == "" {
		writeFilesError(r, 400, "file id is required", "")
		return
	}

	obj, err := relaylogic.GetRelayFile(r.Context(), middleware.GetTenantID(r.Context()), fileID)
	if err != nil {
		writeFilesLogicError(r, err)
		return
	}
	r.Response.WriteJsonExit(obj)
}

// HandleFileContent 处理 GET /v1/files/{file_id}/content（下载文件内容）。
// 与 admin file_serve 同型：流式转发 + 强制 attachment（上传文件的 mime 不可信，
// inline 渲染有存储型 XSS 风险；SDK 拉取不受 disposition 影响）。
func HandleFileContent(r *ghttp.Request) {
	ctx := r.Context()
	fileID := r.Get("file_id").String()
	if fileID == "" {
		writeFilesError(r, 400, "file id is required", "")
		return
	}

	record, reader, err := relaylogic.OpenRelayFile(ctx, middleware.GetTenantID(ctx), fileID)
	if err != nil {
		writeFilesLogicError(r, err)
		return
	}
	defer reader.Close()

	contentType := record.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := record.OriginalName
	if filename == "" {
		filename = record.Filename
	}

	r.Response.Header().Set("Content-Type", contentType)
	// RFC 5987：filename* 支持 UTF-8 文件名（中文等）
	r.Response.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(filename)))
	if record.Size > 0 {
		r.Response.Header().Set("Content-Length", strconv.FormatInt(record.Size, 10))
	}

	if _, err := io.Copy(r.Response.Writer, reader); err != nil {
		// 流式传输中断只能记日志，响应头已发出无法回滚
		g.Log().Warningf(ctx, "[RelayFiles] stream %s interrupted: %v", fileID, err)
	}
}

// HandleFileDelete 处理 DELETE /v1/files/{file_id}（删除文件对象与记录）。
func HandleFileDelete(r *ghttp.Request) {
	fileID := r.Get("file_id").String()
	if fileID == "" {
		writeFilesError(r, 400, "file id is required", "")
		return
	}

	if err := relaylogic.DeleteRelayFile(r.Context(), middleware.GetTenantID(r.Context()), fileID); err != nil {
		writeFilesLogicError(r, err)
		return
	}
	r.Response.WriteJsonExit(g.Map{"id": fileID, "object": "file", "deleted": true})
}

// writeFilesLogicError 把业务层哨兵错误映射为 OpenAI 风格的 HTTP 响应。
// 面向客户端的话术收紧：不回显内部错误链（可能带出存储端点/桶名等部署细节），
// 详细原因一律进服务端日志。
func writeFilesLogicError(r *ghttp.Request, err error) {
	switch {
	case errors.Is(err, commonlogic.ErrStorageNotConfigured):
		writeFilesError(r, 503, relaylogic.RelayFilesUnavailableMsg, "files_unavailable")
	case errors.Is(err, relaylogic.ErrRelayFileBadPurpose):
		writeFilesError(r, 400, err.Error(), "invalid_purpose")
	case errors.Is(err, relaylogic.ErrRelayFileTypeNotAllowed):
		writeFilesError(r, 400, err.Error(), "file_type_not_allowed")
	case errors.Is(err, relaylogic.ErrRelayFileSizeExceeded):
		writeFilesError(r, 413, err.Error(), "file_too_large")
	case errors.Is(err, relaylogic.ErrRelayFileQuotaExceeded):
		writeFilesError(r, 400, err.Error(), "storage_quota_exceeded")
	case errors.Is(err, relaylogic.ErrRelayFileNotFound):
		writeFilesError(r, 404, fmt.Sprintf("The file '%s' was not found", r.Get("file_id").String()), "file_not_found")
	default:
		g.Log().Warningf(r.Context(), "[RelayFiles] operation failed: %v", err)
		writeFilesError(r, 500, "file operation failed", "")
	}
}

// writeFilesError OpenAI 官方错误格式输出（与 writeVideosError 同构）。
func writeFilesError(r *ghttp.Request, statusCode int, message, code string) {
	errType := "invalid_request_error"
	if statusCode >= 500 {
		errType = "server_error"
	}
	errObj := g.Map{
		"type":    errType,
		"message": message,
	}
	if code != "" {
		errObj["code"] = code
	}
	r.Response.WriteStatus(statusCode, g.Map{"error": errObj})
}
