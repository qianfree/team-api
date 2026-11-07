package jsplugin

import (
	"encoding/base64"
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
	dataURIHeaderRe  = regexp.MustCompile(`^data:(image/(svg\+xml|png));base64,$`)
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
	// SVG：良构性（含 svg 根）+ 危险构造全拒绝
	svg := string(raw)
	if svgScriptTag.MatchString(svg) || svgEventAttr.MatchString(svg) ||
		svgJavascriptURL.MatchString(svg) || svgDoctype.MatchString(svg) ||
		svgExternalRef.MatchString(svg) || !strings.Contains(svg, "<svg") {
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
