package jsplugin

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"html"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// 图标上传通道的安全校验（规则见 docs/plugin-api/v1.md）。
// 存储 data URI（data:image/svg+xml;base64,... / data:image/png;base64,...），
// 读取端点解码后以声明的 Content-Type 写出——前端仅 <img> 渲染，绝不 innerHTML。

const maxIconBytes = 512 << 10 // 512 KiB

var (
	// SVG 危险构造：script/foreignObject 标签、事件属性、javascript: 取值、DOCTYPE、绝对外链
	svgScriptTag     = regexp.MustCompile(`(?i)<\s*(script|foreignObject)[\s>]`)
	svgEventAttr     = regexp.MustCompile(`(?i)\bon[a-z]+\s*=`)
	svgJavascriptURL = regexp.MustCompile(`(?i)javascript\s*:`)
	svgDoctype       = regexp.MustCompile(`(?i)<!\s*DOCTYPE`)
	svgExternalRef   = regexp.MustCompile(`(?i)(href|src)\s*=\s*["']https?://`)
	// 协议相对外链：//evil.com 形态浏览器会按当前协议解析成绝对外链
	svgProtocolRelativeRef = regexp.MustCompile(`(?i)(href|src)\s*=\s*["']\s*//`)
	dataURIHeaderRe        = regexp.MustCompile(`^data:(image/(svg\+xml|png));base64,$`)
)

// ValidateIconDataURI 校验上传图标的 data URI 形态与内容安全。
func ValidateIconDataURI(dataURI string) (mimeType string, err error) {
	loc := dataURIHeaderRe.FindStringIndex(dataURI)
	if loc == nil {
		return "", gerror.New("图标需为 data:image/svg+xml 或 data:image/png 的 base64 data URI")
	}
	mimeType = dataURI[loc[0]+5 : loc[1]-8] // "image/xxx"（截去 data: 前缀与 ;base64, 后缀）
	payload := dataURI[loc[1]:]
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", gerror.New("图标 base64 解码失败")
	}
	if len(raw) == 0 || len(raw) > maxIconBytes {
		return "", gerror.Newf("图标大小需在 1-%d 字节之间", maxIconBytes)
	}
	if strings.Contains(mimeType, "png") {
		if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
			return "", gerror.New("PNG 图标缺少 PNG 文件签名")
		}
		return mimeType, nil
	}
	// SVG：良构性（含 svg 根）+ 危险构造全拒绝。
	// 扫描前做 HTML 实体解码——浏览器解析时会还原 &#106;avascript: 之类的
	// 实体编码构造，只扫原始字节会被绕过；协议相对外链（//evil.com）也一并拒绝
	svg := html.UnescapeString(string(raw))
	if svgScriptTag.MatchString(svg) || svgEventAttr.MatchString(svg) ||
		svgJavascriptURL.MatchString(svg) || svgDoctype.MatchString(svg) ||
		svgExternalRef.MatchString(svg) || svgProtocolRelativeRef.MatchString(svg) ||
		!strings.Contains(svg, "<svg") {
		return "", gerror.New("SVG 图标包含不允许的构造（script/事件属性/javascript:/DOCTYPE/外链）或缺少 svg 根")
	}
	return mimeType, nil
}

// DecodeIconDataURI 取出图标二进制与 Content-Type（读取端点用；已入库数据视为可信，
// 解码失败返回错误由端点转 404）。
func DecodeIconDataURI(dataURI string) (mimeType string, data []byte, err error) {
	mimeType, err = ValidateIconDataURI(dataURI)
	if err != nil {
		return "", nil, err
	}
	idx := strings.Index(dataURI, ";base64,")
	payload := dataURI[idx+len(";base64,"):]
	data, err = base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", nil, err
	}
	return mimeType, data, nil
}

// LetterAvatarSVG 生成默认字母头像（无上传图标插件的兜底）：插件显示名首字符
// 居中 + 按 key 哈希的稳定配色——同一插件跨请求/跨实例恒定，不同插件色相自然散开。
// 输出为自包含 SVG 字节（无外链、无脚本），由图标端点以 image/svg+xml 直写。
// 内置插件的 manifest.icon 是语义标签（"text"/"video"）而非图片数据，上传图标
// 仅存在于上传版本行——两类来源都取不到时以此兜底，端点不再 404。
func LetterAvatarSVG(key, name string) []byte {
	letter := "P" // plugin 兜底字符（key/name 均空的极端情况）
	if r := []rune(strings.TrimSpace(name)); len(r) > 0 {
		letter = string(r[0])
	} else if r := []rune(strings.TrimSpace(key)); len(r) > 0 {
		letter = string(r[0])
	}

	// 配色：key 哈希 → 色相；固定饱和度/亮度的浅底深字组合，视觉上成组且不刺眼
	hue := fnvHash(key) % 360
	bg := fmt.Sprintf("hsl(%d, 62%%, 88%%)", hue)
	fg := fmt.Sprintf("hsl(%d, 45%%, 30%%)", hue)

	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">` +
		`<rect width="64" height="64" rx="14" fill="` + bg + `"/>` +
		`<text x="32" y="33" text-anchor="middle" dominant-baseline="central" ` +
		`font-family="system-ui, -apple-system, 'Segoe UI', sans-serif" ` +
		`font-size="30" font-weight="600" fill="` + fg + `">` +
		html.EscapeString(letter) + `</text></svg>`
	return []byte(svg)
}

// fnvHash key → 稳定散列（fNV-1a 32 位足够散开色相，无需密码学强度）。
func fnvHash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
