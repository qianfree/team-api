package admin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// HandleFileServe 流式返回文件内容（应用层鉴权后由 provider.Download 读取转发）。
//
// 用于 local 存储模式（无预签名 URL）以及任何需要应用层代理下载的场景。走 AdminAuth +
// AdminPermissionGuard 中间件（cmd.go 注册时套上），**不**走统一 JSON 响应包装，直接写
// http.ResponseWriter 返回二进制流。鉴权依赖 Authorization header，前端必须用 axios
// responseType:'blob' 请求后转 ObjectURL，不能直接 window.open / <img src>（不带 header 会 401）。
func HandleFileServe(r *ghttp.Request) {
	ctx := r.Context()

	fileID, err := strconv.ParseInt(r.Get("id").String(), 10, 64)
	if err != nil || fileID <= 0 {
		writeServeError(r, http.StatusBadRequest, "无效的文件 ID")
		return
	}

	var record *common.FileRecord
	if err := dao.FilFiles.Ctx(ctx).Where("id", fileID).Scan(&record); err != nil {
		g.Log().Warningf(ctx, "[FileServe] query file %d: %v", fileID, err)
		writeServeError(r, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if record == nil {
		writeServeError(r, http.StatusNotFound, "文件不存在")
		return
	}

	svc, err := common.NewFileServiceFromConfig(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FileServe] init storage for file %d: %v", fileID, err)
		writeServeError(r, http.StatusInternalServerError, "存储初始化失败")
		return
	}

	reader, err := svc.Provider().Download(ctx, record.StoragePath)
	if err != nil {
		g.Log().Warningf(ctx, "[FileServe] read file %d (path=%s): %v", fileID, record.StoragePath, err)
		writeServeError(r, http.StatusInternalServerError, "文件读取失败")
		return
	}
	defer reader.Close()

	contentType := record.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := "attachment"
	if r.GetQuery("disposition").String() == "inline" {
		disposition = "inline"
	}
	filename := record.OriginalName
	if filename == "" {
		filename = record.Filename
	}

	r.Response.Header().Set("Content-Type", contentType)
	// RFC 5987：filename* 支持 UTF-8 文件名（中文等），现代浏览器优先采用。
	r.Response.Header().Set("Content-Disposition",
		fmt.Sprintf(`%s; filename*=UTF-8''%s`, disposition, url.PathEscape(filename)))
	if record.Size > 0 {
		r.Response.Header().Set("Content-Length", strconv.FormatInt(record.Size, 10))
	}

	if _, err := io.Copy(r.Response.Writer, reader); err != nil {
		// 流式传输中断（客户端取消等）只能记日志，响应头已发出无法回滚。
		g.Log().Warningf(ctx, "[FileServe] stream file %d interrupted: %v", fileID, err)
	}
}

// writeServeError 在流式写入开始前返回 JSON 错误（与统一响应格式一致）。
// 一旦开始 io.Copy，响应头已发出，错误只能记日志。
func writeServeError(r *ghttp.Request, status int, msg string) {
	r.Response.WriteStatus(status)
	r.Response.WriteJson(g.Map{"code": status, "message": msg})
}
