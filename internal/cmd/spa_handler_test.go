package cmd

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// 构造模拟的前端构建产物：入口文件 + 一个带 hash 的静态 chunk
func newSpaTestFS() fs.FS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!doctype html><html><head><script src=\"/assets/index-x9y8z7.js\"></script></head><body></body></html>"),
		},
		"assets/index-x9y8z7.js":  &fstest.MapFile{Data: []byte("console.log('chunk')")},
		"favicon.ico":             &fstest.MapFile{Data: []byte("ico")},
		"screenshots/page-a.webp": &fstest.MapFile{Data: []byte("img")},
	}
}

func get(t *testing.T, h http.HandlerFunc, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func assertContains(t *testing.T, rec *httptest.ResponseRecorder, key, want string) {
	t.Helper()
	if got := rec.Header().Get(key); got != want {
		t.Errorf("%s = %q, 期望 %q", key, got, want)
	}
}

// 路由请求回退 index.html，且带 no-cache 与 ETag
func TestSpaHandlerRouteFallback(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")
	rec := get(t, h, "/dashboard", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "index-x9y8z7.js") {
		t.Error("响应体应为 index.html 内容")
	}
	assertContains(t, rec, "Cache-Control", "no-cache")
	if rec.Header().Get("ETag") == "" {
		t.Error("index.html 应携带 ETag")
	}
}

// 根路径与显式 /index.html 都走同一入口响应
func TestSpaHandlerIndexPaths(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")
	for _, target := range []string{"/", "/index.html"} {
		rec := get(t, h, target, nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Errorf("%s 应返回 200 的 index.html，实际 %d", target, rec.Code)
		}
	}
}

// 未重新发布时重复访问命中 ETag 协商缓存，返回 304 空响应
func TestSpaHandlerEtagNegotiation(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")
	first := get(t, h, "/", nil)
	tag := first.Header().Get("ETag")

	rec := get(t, h, "/dashboard", map[string]string{"If-None-Match": tag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("状态码 = %d, 期望 304 Not Modified", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Error("304 响应不应携带 body")
	}
}

// 带静态资源特征的缺失文件返回 404 而非回退 SPA HTML（发版后旧 hash chunk 场景）
func TestSpaHandlerMissingAssetReturns404(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")
	cases := []string{
		"/assets/stale-old-hash.js",
		"/assets/some.css",
		"/assets/vendor.wasm",
		"/images/missing.png", // 不存在的资源扩展名
		"/manifest.json",
	}
	for _, target := range cases {
		rec := get(t, h, target, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s 应返回 404，实际 %d", target, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
			t.Errorf("%s 的 404 不应是 text/html", target)
		}
	}
}

// 无扩展名的未知路径仍回退 index.html（深层路由刷新场景）
func TestSpaHandlerUnknownRouteStillFallbacks(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")
	rec := get(t, h, "/team/member-detail/42", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "index") {
		t.Fatalf("深层路由应回退 index.html，实际 %d", rec.Code)
	}
}

// 存在的 assets 文件命中后强缓存一年；根级非 hash 文件要求协商校验
func TestSpaHandlerCacheHeaders(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "")

	rec := get(t, h, "/assets/index-x9y8z7.js", nil)
	assertContains(t, rec, "Cache-Control", "public, max-age=31536000, immutable")

	rec = get(t, h, "/favicon.ico", nil)
	assertContains(t, rec, "Cache-Control", "no-cache")
}

// 带 URL 前缀的 admin 子应用：前缀剥离、前缀下的 404 与缓存规则同样生效
func TestSpaHandlerWithPrefix(t *testing.T) {
	h := spaHandler(newSpaTestFS(), "/admin")

	rec := get(t, h, "/admin/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("/admin/ 应返回 index.html，实际 %d", rec.Code)
	}

	rec = get(t, h, "/admin/assets/missing.js", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("/admin/assets/missing.js 应返回 404，实际 %d", rec.Code)
	}

	rec = get(t, h, "/admin/assets/index-x9y8z7.js", nil)
	assertContains(t, rec, "Cache-Control", "public, max-age=31536000, immutable")
}
