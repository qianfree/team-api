package jsplugin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestRouteHookAlias 路由别名解析：声明优先、缺省回落固定钩子名、形态校验。
func TestRouteHookAlias(t *testing.T) {
	r := contract.Route{Method: "POST", Path: "/submit/:action", Type: contract.RouteTypeSubmit,
		Decode: "decodeSubmit", Render: "renderSubmitNamed"}
	if err := r.Validate(); err != nil {
		t.Fatalf("合法别名被拒: %v", err)
	}
	if r.ResolveDecode() != "decodeSubmit" {
		t.Fatalf("decode 别名应优先: %s", r.ResolveDecode())
	}
	if r.ResolveRender(contract.HookNameRenderTask) != "renderSubmitNamed" {
		t.Fatalf("render 别名应优先: %s", r.ResolveRender(contract.HookNameRenderTask))
	}

	plain := contract.Route{Method: "POST", Path: "/s", Type: contract.RouteTypeSubmit}
	if plain.ResolveDecode() != contract.HookNameDecode {
		t.Fatalf("缺省应回落 decodeRequest: %s", plain.ResolveDecode())
	}
	if plain.ResolveRender(contract.HookNameRenderTask) != contract.HookNameRenderTask {
		t.Fatal("缺省应回落默认渲染名")
	}

	// 别名仅 submit 路由可用；非法形态拒绝
	bad := contract.Route{Method: "GET", Path: "/f", Type: contract.RouteTypeQuery, Decode: "x"}
	if err := bad.Validate(); err == nil {
		t.Fatal("query 路由带 decode 别名应被拒绝")
	}
	badName := contract.Route{Method: "POST", Path: "/s", Type: contract.RouteTypeSubmit, Decode: "1bad name"}
	if err := badName.Validate(); err == nil {
		t.Fatal("非法别名形态应被拒绝")
	}
}

// TestCryptoHostFunctions crypto 宿主函数与 Go 标准库计算值对照（new-api 签名对齐）。
func TestCryptoHostFunctions(t *testing.T) {
	src := `
export const meta = { apiVersion: 1, key: "crypto-demo", name: "C", version: "1.0.0",
  models: ["m"], routes: [{method:"POST",path:"/s",type:"submit"}] };
export function hmac(p) { return utils.crypto.hmacSHA256(p[0], p[1]); }
export function jwt(p) { return utils.crypto.jwtSignHS256(p[0], p[1]); }
export function b64(p) { return utils.crypto.base64(p[0]); }
export function b64url(p) { return utils.crypto.base64URL(p[0]); }
export function b64urlDecode(p) { return utils.crypto.base64URLDecode(p[0]); }
export function uid() { return utils.uuid(); }
`
	compiled := mustCompile(t, src)
	inst, err := NewGojaEngine().NewInstance(compiled, InstanceOptions{})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	defer inst.Close()
	call := func(hook string, arg any) string {
		t.Helper()
		args, _ := json.Marshal(arg)
		out, err := inst.Call(context.Background(), hook, args)
		if err != nil {
			t.Fatalf("%s 失败: %v", hook, err)
		}
		return string(out)
	}

	// hmacSHA256(message, secret) → hex
	mac := hmac.New(sha256.New, []byte("sk-secret"))
	mac.Write([]byte("hello"))
	wantHMAC := hex.EncodeToString(mac.Sum(nil))
	if got := call("hmac", []string{"hello", "sk-secret"}); got != `"`+wantHMAC+`"` {
		t.Fatalf("hmacSHA256 不匹配:\n got  %s\n want %s", got, wantHMAC)
	}

	// jwtSignHS256(claims, secret) → 可被标准库验证的 HS256 JWT
	token := call("jwt", []any{map[string]any{"iss": "kling-ak", "exp": time.Now().Add(time.Hour).Unix()}, "sk-secret"})
	var tokenStr string
	if err := json.Unmarshal([]byte(token), &tokenStr); err != nil {
		t.Fatalf("jwt 返回非字符串: %s", token)
	}
	parts := splitDot(tokenStr)
	if len(parts) != 3 {
		t.Fatalf("JWT 应为三段: %s", tokenStr)
	}
	headerJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if string(headerJSON) != `{"alg":"HS256","typ":"JWT"}` {
		t.Fatalf("JWT header 异常: %s", headerJSON)
	}
	verifier := hmac.New(sha256.New, []byte("sk-secret"))
	verifier.Write([]byte(parts[0] + "." + parts[1]))
	wantSig := base64.RawURLEncoding.EncodeToString(verifier.Sum(nil))
	if parts[2] != wantSig {
		t.Fatalf("JWT 签名不匹配:\n got  %s\n want %s", parts[2], wantSig)
	}

	// base64 家族
	if got := call("b64", []string{"hi"}); got != `"`+base64.StdEncoding.EncodeToString([]byte("hi"))+`"` {
		t.Fatalf("base64 异常: %s", got)
	}
	if got := call("b64url", []string{"hi"}); got != `"`+base64.RawURLEncoding.EncodeToString([]byte("hi"))+`"` {
		t.Fatalf("base64URL 异常: %s", got)
	}
	if got := call("b64urlDecode", []string{base64.RawURLEncoding.EncodeToString([]byte("ok"))}); got != `"ok"` {
		t.Fatalf("base64URLDecode 异常: %s", got)
	}

	// uuid v4 形态
	uid := call("uid", nil)
	var uidStr string
	_ = json.Unmarshal([]byte(uid), &uidStr)
	if len(uidStr) != 36 || uidStr[14] != '4' {
		t.Fatalf("uuid v4 形态异常: %s", uidStr)
	}
}

func splitDot(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
