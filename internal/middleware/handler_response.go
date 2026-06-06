package middleware

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gvalid"

	"github.com/qianfree/team-api/internal/response"
)

// downloadContentTypes 列举表示文件下载的 Content-Type 前缀。
// 匹配时跳过 JSON 包装，避免在文件内容后追加 {"code":0,...}。
var downloadContentTypes = []string{
	"text/csv",
	"application/vnd.openxmlformats",
	"application/octet-stream",
}

// MiddlewareHandlerResponse 是 GoFrame 标准响应处理中间件。
// 通过 r.GetHandlerResponse() 读取控制器方法的返回值，
// 并包装为项目统一响应格式（code/message/data/request_id）。
func MiddlewareHandlerResponse(r *ghttp.Request) {
	r.Middleware.Next()

	// 如果控制器设置了错误
	if err := r.GetError(); err != nil {
		// 仅记录系统级错误（5xx / 未知）；跳过业务错误和客户端错误，
		// 避免正常的参数校验失败、认证错误等污染日志。
		if isSystemError(err) {
			g.Log().Warningf(r.Context(), "[HandlerResponse] path=%s, body=%s, error=%+v, type=%T", r.URL.Path, r.GetBodyString(), err, err)
		}
		response.Error(r, err)
		return
	}

	// 跳过文件下载的 JSON 包装（CSV、Excel 等）。
	// 导出函数直接写入原始 ResponseWriter 并在写入前设置 Content-Type。
	ct := r.Response.Header().Get("Content-Type")
	for _, prefix := range downloadContentTypes {
		if strings.HasPrefix(ct, prefix) {
			return
		}
	}

	// 当 handler 已经直接写入响应时跳过 JSON 包装（如模型导出设置 Content-Disposition: attachment）。
	// 同时规避 Go 的 nil-interface 陷阱：类型化的 nil 指针 (*T)(nil) 存入 interface{} 后 != nil，
	// 导致 GetHandlerResponse() 看起来非 nil，但 handler 实际返回了 nil。
	if r.Response.Header().Get("Content-Disposition") != "" {
		return
	}

	// 如果 handler 已经写入了响应体且返回了 nil 结果，
	// 不在其上追加标准响应包装。使用 isNilInterface 捕获类型化 nil 指针 (*T)(nil)。
	if r.Response.BufferLength() > 0 && isNilInterface(r.GetHandlerResponse()) {
		return
	}

	// 控制器方法返回 (res, error)；GoFrame 将第一个返回值
	// 存入 r.handlerResponse，可通过 GetHandlerResponse() 获取。
	res := r.GetHandlerResponse()
	if !isNilInterface(res) {
		response.Success(r, res)
		return
	}

	// 空响应（如删除/更新操作返回 nil 指针）
	if r.Response.BufferLength() == 0 {
		response.Success(r, nil)
	}
}

// isSystemError 判断错误是否为需要记录日志的系统级错误（5xx / 未知）。
// 业务错误（4xx、>= 10000）和客户端错误属于正常流程，不应污染日志。
func isSystemError(err error) bool {
	// 客户端主动断开连接属于正常现象，不作为系统错误记录
	if errors.Is(err, context.Canceled) {
		return false
	}

	// GoFrame 校验错误（gvalid.Error）属于客户端错误，不是系统错误
	var validErr gvalid.Error
	if errors.As(err, &validErr) {
		return false
	}

	var gerr *gerror.Error
	if !errors.As(err, &gerr) {
		// 原始 Go 错误（无 gerror code）——视为系统错误
		return true
	}
	code := gerr.Code().Code()
	// 客户端错误（400-499）：参数校验、认证、未找到等
	if code >= 400 && code < 500 {
		return false
	}
	// 业务规则错误（>= 10000）：余额不足、配额超限等
	if code >= 10000 {
		return false
	}
	return true
}

// isNilInterface 检查接口值是否为 nil 或包含类型化 nil 指针。
func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}
