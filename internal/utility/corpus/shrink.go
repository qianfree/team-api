package corpus

import "regexp"

// 语料体积控制。
//
// 这不是脱敏：图片/音频的**内容**对协议转换毫无影响（转换器只搬运 mimeType 与
// data 字段），但一张截图的 base64 动辄数 MB，原样落盘会让语料仓库迅速失控。
// 替换为等价的最小合法数据，既保住「这里有一张图」的结构语义，又把体积压到常数。

// placeholderPNG 1×1 透明 PNG 的 base64，用作超长内联数据的替身。
// 选真实可解码的图片而非任意字符串：图片处理路径（尺寸探测、格式校验）
// 拿到它仍能正常工作，语料不会因替身而走进异常分支。
const placeholderPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// base64RunPattern 匹配连续的 base64 字符串。阈值由调用方给出，
// 在 8KB 量级上几乎不会误伤正常文本（自然语言与 JSON 结构都不会有这么长的无分隔串）。
var base64RunPattern = regexp.MustCompile(`[A-Za-z0-9+/]{64,}={0,2}`)

// ShrinkInlineData 把长度超过 maxBytes 的内联 base64 串替换为占位数据。
// maxBytes <= 0 时原样返回。
//
// 直接在文本层做替换而非解析 JSON：流式语料（SSE）不是单个 JSON，
// 且内联数据可能出现在任意嵌套位置，文本层处理对两种形态都适用。
func ShrinkInlineData(body []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(body) == 0 {
		return body
	}
	return base64RunPattern.ReplaceAllFunc(body, func(match []byte) []byte {
		if len(match) <= maxBytes {
			return match
		}
		return []byte(placeholderPNG)
	})
}
