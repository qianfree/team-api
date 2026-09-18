package common

import (
	"bytes"
	"strconv"
	"strings"
)

// 内联媒体（data URL）处理工具。
//
// 多模态请求体里的图片/音频以 `data:<mime>;base64,<载荷>` 的形式内联在 JSON 中，
// 单个载荷动辄数 MB。这类载荷对三条链路都是纯负担：
//   - 预扣估算按字节折算会把一张图估成几十万 token，直接冻穿租户余额
//   - 审计落库会把整段 base64 灌进 aud_request_logs
//   - 敏感词匹配白扫几 MB，还可能在 base64 串里误命中
//
// 这里提供统一的扫描与剥离能力，三处共用，避免各写一份。

// inlineMediaMarker data URL 中 base64 载荷的起始标记。
const inlineMediaMarker = ";base64,"

// inlineMediaMinPayload 认定为内联媒体的最小载荷长度。
// 真实的图片/音频 base64 远大于此，设下限是为了不误伤正文里恰好出现该标记的短文本。
const inlineMediaMinPayload = 64

// ScanInlineMediaBytes 扫描内容中的 data URL base64 载荷，返回载荷总字节数与媒体个数。
//
// 纯字节扫描不做 JSON 反序列化——调用点在请求热路径上，且请求体可能有数 MB。
// base64 字母表不含 `"` 与 `\`，故以二者作为 JSON 字符串内载荷的终止符。
func ScanInlineMediaBytes(b []byte) (payloadBytes, count int) {
	marker := []byte(inlineMediaMarker)
	rest := b
	for {
		idx := bytes.Index(rest, marker)
		if idx < 0 {
			return
		}
		rest = rest[idx+len(marker):]
		end := bytes.IndexAny(rest, "\"\\")
		if end < 0 {
			end = len(rest)
		}
		if end >= inlineMediaMinPayload {
			payloadBytes += end
			count++
		}
		rest = rest[end:]
	}
}

// StripInlineMediaBytes 是 StripInlineMedia 的字节切片版本。
// 未命中内联媒体时原样返回入参切片，不产生任何拷贝——请求热路径上的绝大多数
// 请求体是纯文本，这条快路径让它们零开销通过。
func StripInlineMediaBytes(b []byte) []byte {
	if !bytes.Contains(b, []byte(inlineMediaMarker)) {
		return b
	}
	return []byte(StripInlineMedia(string(b)))
}

// StripInlineMedia 把 base64 载荷替换成长度占位，保留 mime 前缀便于排查：
//
//	data:image/png;base64,iVBORw0KGgo...  →  data:image/png;base64,<省略 2097152 字节>
//
// 未命中任何内联媒体时原样返回，不产生额外分配。
func StripInlineMedia(s string) string {
	if !strings.Contains(s, inlineMediaMarker) {
		return s
	}

	var sb strings.Builder
	rest := s
	for {
		idx := strings.Index(rest, inlineMediaMarker)
		if idx < 0 {
			sb.WriteString(rest)
			break
		}
		head := idx + len(inlineMediaMarker)
		sb.WriteString(rest[:head])
		rest = rest[head:]

		end := strings.IndexAny(rest, "\"\\")
		if end < 0 {
			end = len(rest)
		}
		if end >= inlineMediaMinPayload {
			sb.WriteString("<省略 ")
			sb.WriteString(strconv.Itoa(end))
			sb.WriteString(" 字节>")
		} else {
			sb.WriteString(rest[:end])
		}
		rest = rest[end:]
	}
	return sb.String()
}
