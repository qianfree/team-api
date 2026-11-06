package jsplugin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/gogf/gf/v2/frame/g"

	relaycommon "github.com/qianfree/team-api/relay/common"
)

// utils.fetch 限额（与 new-api 对齐，见设计方案 §十一）。
const (
	maxFetchesPerCall   = 4                // 每次钩子调用最多抓取次数
	fetchTimeout        = 15 * time.Second // 单次抓取超时
	fetchTotalBudget    = 30 * time.Second // 单次钩子调用的抓取总时长
	maxFetchResponseMiB = 1 << 20          // 单响应体上限 1MiB
)

// hostUtils 注入到 JS 的 utils 宿主能力对象。预算按「每次钩子调用」重置：
// 实例在 Call 持锁期间串行执行，预算字段无需额外加锁。
type hostUtils struct {
	inst *gojaInstance

	// 每次 Call 的状态（beginCall 重置）
	ctx        context.Context
	fetchCount int
	fetchSpent time.Duration
}

func newHostUtils(inst *gojaInstance) *hostUtils {
	return &hostUtils{inst: inst}
}

// beginCall 在每次钩子调用开始时重置预算与上下文。
func (h *hostUtils) beginCall(ctx context.Context) {
	h.ctx = ctx
	h.fetchCount = 0
	h.fetchSpent = 0
}

// buildObject 构造注入 JS 的 utils 对象。
func (h *hostUtils) buildObject(rt *goja.Runtime) *goja.Object {
	obj := rt.NewObject()
	_ = obj.Set("fetch", func(call goja.FunctionCall) goja.Value { return h.fetch(rt, call) })
	logObj := rt.NewObject()
	_ = logObj.Set("info", func(call goja.FunctionCall) goja.Value { return h.log(call, "info") })
	_ = logObj.Set("warn", func(call goja.FunctionCall) goja.Value { return h.log(call, "warning") })
	_ = logObj.Set("error", func(call goja.FunctionCall) goja.Value { return h.log(call, "error") })
	_ = obj.Set("log", logObj)
	_ = obj.Set("now", func(call goja.FunctionCall) goja.Value {
		return rt.ToValue(time.Now().UnixMilli())
	})
	// utils.config.get(key)：读插件自己的配置（manifest.config 声明、管理端配置，
	// 加密字段已解密注入）。未声明的键返回 undefined——契约外读取直接暴露为 bug。
	configObj := rt.NewObject()
	_ = configObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		if v, ok := h.inst.configValues[call.Arguments[0].String()]; ok {
			return rt.ToValue(v)
		}
		return goja.Undefined()
	})
	_ = obj.Set("config", configObj)

	// utils.crypto（new-api 对齐：签名类厂商如 Kling AK/SK 需要；纯计算无网络）+ uuid
	cryptoObj := rt.NewObject()
	_ = cryptoObj.Set("hmacSHA256", func(call goja.FunctionCall) goja.Value {
		return h.wrapSync(rt, call, "hmacSHA256", 2, func(args []goja.Value) any {
			message, secret := args[0].String(), args[1].String()
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(message))
			return hex.EncodeToString(mac.Sum(nil))
		})
	})
	_ = cryptoObj.Set("jwtSignHS256", func(call goja.FunctionCall) goja.Value {
		return h.wrapSync(rt, call, "jwtSignHS256", 2, func(args []goja.Value) any {
			claims, ok := args[0].Export().(map[string]any)
			if !ok {
				panic(rt.NewGoError(errString("jwtSignHS256 的 claims 必须是对象")))
			}
			token, err := signJWTHS256(claims, args[1].String())
			if err != nil {
				panic(rt.NewGoError(errString("jwtSignHS256: " + err.Error())))
			}
			return token
		})
	})
	_ = cryptoObj.Set("base64", func(call goja.FunctionCall) goja.Value {
		return h.wrapSync(rt, call, "base64", 1, func(args []goja.Value) any {
			return base64.StdEncoding.EncodeToString([]byte(args[0].String()))
		})
	})
	_ = cryptoObj.Set("base64URL", func(call goja.FunctionCall) goja.Value {
		return h.wrapSync(rt, call, "base64URL", 1, func(args []goja.Value) any {
			return base64.RawURLEncoding.EncodeToString([]byte(args[0].String()))
		})
	})
	_ = cryptoObj.Set("base64URLDecode", func(call goja.FunctionCall) goja.Value {
		return h.wrapSync(rt, call, "base64URLDecode", 1, func(args []goja.Value) any {
			decoded, err := base64.RawURLEncoding.DecodeString(args[0].String())
			if err != nil {
				panic(rt.NewGoError(errString("base64URLDecode: 无效 base64url 输入")))
			}
			return string(decoded)
		})
	})
	_ = obj.Set("crypto", cryptoObj)
	_ = obj.Set("uuid", func(call goja.FunctionCall) goja.Value {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return goja.Undefined()
		}
		b[6] = (b[6] & 0x0f) | 0x40 // v4
		b[8] = (b[8] & 0x3f) | 0x80 // RFC4122
		return rt.ToValue(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
	})
	return obj
}

// wrapSync 同步宿主函数的参数数校验薄封装。
func (h *hostUtils) wrapSync(rt *goja.Runtime, call goja.FunctionCall, name string, wantArgs int, fn func(args []goja.Value) any) goja.Value {
	if len(call.Arguments) < wantArgs {
		panic(rt.NewGoError(errString(name + " 参数不足")))
	}
	return rt.ToValue(fn(call.Arguments))
}

// signJWTHS256 签发 HS256 JWT（header 固定 {alg:HS256,typ:JWT}；claims 原样写入，
// exp/nbf 等时效字段由调用方给定——与 new-api jwtSignHS256 同语义）。
func signJWTHS256(claims map[string]any, secret string) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// fetch utils.fetch({url, method?, headers?, body?, responseType?}) →
// {status, headers, body}。JSON 边界：body 按 responseType 解析为对象/字符串/ArrayBuffer。
func (h *hostUtils) fetch(rt *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) {
		panic(rt.NewGoError(errString("utils.fetch 需要一个请求对象参数")))
	}
	arg, ok := call.Arguments[0].Export().(map[string]any)
	if !ok {
		panic(rt.NewGoError(errString("utils.fetch 参数必须是对象")))
	}

	// 限额检查
	if h.fetchCount >= maxFetchesPerCall {
		panic(rt.NewGoError(errString("超过单次钩子调用的抓取次数上限（4 次）")))
	}
	if h.ctx == nil {
		panic(rt.NewGoError(errString("fetch 不在钩子调用上下文内")))
	}

	rawURL, _ := arg["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		panic(rt.NewGoError(errString("url 不合法：" + rawURL)))
	}
	if !h.hostAllowed(parsed.Host) {
		panic(rt.NewGoError(errString("目标 host 不在白名单（渠道 baseUrl host ∪ meta.allowedHosts）：" + parsed.Host)))
	}

	method := strings.ToUpper(stringOf(arg["method"], "GET"))
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodPost {
		panic(rt.NewGoError(errString("method 仅支持 GET/HEAD/POST")))
	}

	// 构造请求
	var bodyReader io.Reader
	if bodyArg, exists := arg["body"]; exists && bodyArg != nil {
		switch b := bodyArg.(type) {
		case string:
			bodyReader = strings.NewReader(b)
		default:
			// 对象/数组 → JSON 序列化
			if bs, err := marshalJSON(bodyArg); err != nil {
				panic(rt.NewGoError(errString("body 无法序列化为 JSON")))
			} else {
				bodyReader = strings.NewReader(string(bs))
			}
		}
		if method == http.MethodGet || method == http.MethodHead {
			method = http.MethodPost
		}
	}
	req, err := http.NewRequestWithContext(h.ctx, method, rawURL, bodyReader)
	if err != nil {
		panic(rt.NewGoError(errString("构造请求失败：" + err.Error())))
	}
	if headers, ok := arg["headers"].(map[string]any); ok {
		for k, v := range headers {
			if strings.EqualFold(k, "Authorization") {
				continue // 凭据由基座管理
			}
			req.Header.Set(k, stringOf(v, ""))
		}
	}

	responseType := stringOf(arg["responseType"], "json")
	if responseType != "json" && responseType != "text" && responseType != "bytes" {
		panic(rt.NewGoError(errString(`responseType 仅支持 "json" / "text" / "bytes"`)))
	}

	// 执行（不跟随重定向；超时计入预算）
	h.fetchCount++
	client := relaycommon.NewPooledClient(int(fetchTimeout.Seconds()), false)
	// 3xx 原样返回
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }

	started := time.Now()
	resp, err := client.Do(req)
	h.fetchSpent += time.Since(started)
	if h.fetchSpent > fetchTotalBudget {
		panic(rt.NewGoError(errString("超过单次钩子调用的抓取总时长（30s）")))
	}
	if err != nil {
		panic(rt.NewGoError(errString("请求失败：" + err.Error())))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchResponseMiB+1))
	if err != nil {
		panic(rt.NewGoError(errString("读取响应失败：" + err.Error())))
	}
	if len(body) > maxFetchResponseMiB {
		panic(rt.NewGoError(errString("响应体超过 1MiB 上限")))
	}

	// 组装返回对象
	out := rt.NewObject()
	_ = out.Set("status", resp.StatusCode)
	headers := rt.NewObject()
	for k, vs := range resp.Header {
		if len(vs) > 0 {
			_ = headers.Set(k, vs[0])
		}
	}
	_ = out.Set("headers", headers)

	contentType := resp.Header.Get("Content-Type")
	switch responseType {
	case "bytes":
		_ = out.Set("body", rt.NewArrayBuffer(body))
	case "text":
		_ = out.Set("body", string(body))
	default: // json
		if strings.Contains(contentType, "json") && len(body) > 0 {
			var parsed any
			if err := unmarshalJSON(body, &parsed); err != nil {
				panic(rt.NewGoError(errString("响应体不是合法 JSON（可用 responseType:\"text\"）")))
			}
			_ = out.Set("body", parsed)
		} else {
			_ = out.Set("body", string(body))
		}
	}
	return out
}

// allowedHostsKey 每调用 fetch 白名单的 context 键：渠道 baseUrl host 因请求而异，
// 由桥接层在每次调用时通过 WithAllowedHosts 注入（完整集合 = 渠道 host ∪ manifest.allowedHosts）。
type allowedHostsKey struct{}

// WithAllowedHosts 为本次调用注入 fetch 白名单（覆盖实例级配置）。
func WithAllowedHosts(ctx context.Context, hosts []string) context.Context {
	return context.WithValue(ctx, allowedHostsKey{}, normalizeHosts(hosts))
}

// hostAllowed 报告 host[:port] 是否命中白名单。无端口的条目匹配任意端口。
func (h *hostUtils) hostAllowed(hostPort string) bool {
	hostPort = strings.ToLower(hostPort)
	// 调用级白名单优先（桥接层注入的完整集合）
	if set, ok := h.ctx.Value(allowedHostsKey{}).(map[string]struct{}); ok && len(set) > 0 {
		return matchHostSet(set, hostPort)
	}
	return matchHostSet(h.inst.allowedHosts, hostPort)
}

func matchHostSet(set map[string]struct{}, hostPort string) bool {
	if _, ok := set[hostPort]; ok {
		return true
	}
	// 条目不带端口 → 匹配任意端口（IPv6 字面量除外）
	if idx := strings.LastIndex(hostPort, ":"); idx > 0 && !strings.Contains(hostPort, "]") {
		if _, ok := set[hostPort[:idx]]; ok {
			return true
		}
	}
	return false
}

// log utils.log.{info,warn,error}(message, fields?) → g.Log + 任务日志回调（observe 注入）。
func (h *hostUtils) log(call goja.FunctionCall, level string) goja.Value {
	msg := ""
	if len(call.Arguments) > 0 {
		msg = call.Arguments[0].String()
	}
	extra := ""
	if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
		if bs, err := marshalJSON(call.Arguments[1].Export()); err == nil {
			extra = " " + string(bs)
		}
	}
	line := "[jsplugin:" + h.inst.compiled.Key + "] " + msg + extra
	switch level {
	case "error":
		g.Log().Error(h.callCtx(), line)
	case "warning":
		g.Log().Warning(h.callCtx(), line)
	default:
		g.Log().Info(h.callCtx(), line)
	}
	if logger := TaskLoggerFromContext(h.callCtx()); logger != nil {
		logger(level, msg+extra)
	}
	return goja.Undefined()
}

func (h *hostUtils) callCtx() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// errString 便捷构造 fetch/log 的抛错消息。
func errString(msg string) error { return &fetchError{msg} }

type fetchError struct{ msg string }

func (e *fetchError) Error() string { return e.msg }

func stringOf(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}
