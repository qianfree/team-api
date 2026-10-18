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

// bareBase64MinPayload 认定 JSON 字符串字面量为裸 base64 媒体载荷的最小长度。
// OpenAI chat 的 input_audio.data、Claude 的 source.data、Gemini 的 inline_data.data
// 按协议携带无 data: 前缀的裸 base64，缺少 ";base64," 上下文佐证，阈值须远高于
// data URL 形态（64）以避免误伤：hex 哈希（≤128 字符）、普通 ID 等自然落选，
// 真实音频/图片 base64 至少数十 KB。
const bareBase64MinPayload = 1024

// ScanInlineMediaBytes 扫描内容中的内联媒体 base64 载荷，返回载荷总字节数与媒体个数。
// 覆盖两种形态：data URL（data:...;base64,xxx）与裸 base64 字符串（input_audio 等）。
//
// 纯字节扫描不做 JSON 反序列化——调用点在请求热路径上，且请求体可能有数 MB。
// base64 字母表不含 `"` 与 `\`，故以二者作为 JSON 字符串内载荷的终止符。
func ScanInlineMediaBytes(b []byte) (payloadBytes, count int) {
	marker := []byte(inlineMediaMarker)
	rest := b
	for {
		idx := bytes.Index(rest, marker)
		if idx < 0 {
			break
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

	bareBytes, bareCount := scanBareBase64Strings(b)
	return payloadBytes + bareBytes, count + bareCount
}

// StripInlineMediaBytes 是 StripInlineMedia 的字节切片版本。
// 未命中内联媒体时原样返回入参切片，不产生任何拷贝——请求热路径上的绝大多数
// 请求体是纯文本，这条快路径让它们零开销通过。
func StripInlineMediaBytes(b []byte) []byte {
	if !bytes.Contains(b, []byte(inlineMediaMarker)) {
		stripped, _, _ := stripBareBase64Strings(b)
		return stripped
	}
	return []byte(StripInlineMedia(string(b)))
}

// StripInlineMedia 把 base64 载荷替换成长度占位，保留 mime 前缀便于排查：
//
//	data:image/png;base64,iVBORw0KGgo...  →  data:image/png;base64,<省略 2097152 字节>
//
// 裸 base64 字符串（input_audio.data 等无 data: 前缀的载荷）同样替换为
// <省略 N 字节>，字段名与 JSON 结构保持不变。未命中任何内联媒体时原样
// 返回，不产生额外分配。
func StripInlineMedia(s string) string {
	out := s
	if strings.Contains(s, inlineMediaMarker) {
		out = stripMarkerPayloads(out)
	}
	stripped, _, _ := stripBareBase64Strings([]byte(out))
	return string(stripped)
}

// stripMarkerPayloads 剥离 data URL 形态（;base64, 标记）的载荷。
func stripMarkerPayloads(s string) string {
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

// scanBareBase64Strings 扫描 JSON 字符串字面量中的长裸 base64 载荷，
// 返回载荷总字节数与个数。stripBareBase64Strings 的只读形态（未命中零分配）。
func scanBareBase64Strings(b []byte) (payloadBytes, count int) {
	_, payloadBytes, count = stripBareBase64Strings(b)
	return payloadBytes, count
}

// stripBareBase64Strings 剥离 JSON 字符串字面量中的长裸 base64 载荷，返回
// （剥离后的体, 载荷总字节数, 个数）。
//
// 按字符串字面量边界识别：内容长度达到 bareBase64MinPayload 且全部字符都在
// base64 字母表内（含 = 填充）即视为媒体载荷。转义对（\x）不在字母表内，
// 含转义的字符串自然落选；data URL 字符串因含 ":;," 等前缀字符同样落选
// （由 marker 路径处理），两条路径互不重复计数。
//
// 未命中时原样返回入参切片（零分配）；命中时载荷替换为 <省略 N 字节>，
// 输出在首次命中后才惰性分配并回补前缀，纯文本请求体依旧近零开销。
func stripBareBase64Strings(b []byte) (out []byte, payloadBytes, count int) {
	var emitted int // 已原样写出的源位置
	i := 0
	for i < len(b) {
		if b[i] != '"' {
			i++
			continue
		}
		// 解析整个字符串字面量，j 停在未转义的闭引号上
		j := i + 1
		pure := true
		for j < len(b) {
			c := b[j]
			if c == '\\' {
				pure = false
				j += 2 // 跳过转义对（\x、\" 等）
				continue
			}
			if c == '"' {
				break
			}
			if !isBase64Alphabet(c) {
				pure = false
			}
			j++
		}
		if j >= len(b) {
			break // 未闭合的畸形尾部：原样保留
		}
		content := b[i+1 : j]
		if pure && len(content) >= bareBase64MinPayload {
			payloadBytes += len(content)
			count++
			if out == nil {
				out = make([]byte, 0, len(b))
			}
			out = append(out, b[emitted:i+1]...)
			out = append(out, "<省略 "...)
			out = strconv.AppendInt(out, int64(len(content)), 10)
			out = append(out, " 字节>"...)
			out = append(out, '"')
			emitted = j + 1
		}
		i = j + 1
	}
	if out == nil {
		return b, payloadBytes, count
	}
	out = append(out, b[emitted:]...)
	return out, payloadBytes, count
}

// isBase64Alphabet 判断字节是否在 base64 标准字母表内（含 = 填充符）。
func isBase64Alphabet(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '+' || c == '/' || c == '='
}
