// Package corpus 从渠道调试日志提取协议转换测试语料。
//
// 背景：协议转换的金样本/能力守恒/模糊测试此前全部由手写语料驱动，而手写语料的
// 固有盲区正是「写的人想不到的形状」——真实客户端（Claude Code、Cursor、各家 SDK）
// 发出的请求远比人工构造的刁钻。chn_debug_logs 已经完整记录了四段报文，
// 本包负责把它转成金样本套件能直接消费的语料文件。
//
// 关键取舍：真实流量冗余极大（万级记录往往只有几十种结构），直接落盘既臃肿又无益。
// 因此以**结构指纹**去重，每种形状只留代表样本。指纹只用于去重判等，
// 其激进程度不影响落盘内容——语料始终是原文。
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// freeTextKeys 这些 key 的字符串值是自由文本，对转换行为无影响，
// 计算指纹时一律折叠为类型占位——否则每条不同的用户输入都会算作一种新形状，
// 去重就完全失效了。
//
// 注意 type / role / finish_reason 等判别式字段**不在**此列：它们决定转换分支，
// 是结构的一部分。
var freeTextKeys = map[string]bool{
	"model": true, "text": true, "data": true, "url": true, "image_url": true,
	"arguments": true, "partial_json": true, "content": true, "query": true,
	"instructions": true, "system": true, "prompt": true, "uri": true,
	"title": true, "output": true, "description": true, "name": true,
	"id": true, "call_id": true, "tool_call_id": true, "tool_use_id": true,
	"item_id": true, "response_id": true, "responseId": true,
	"signature": true, "thought_signature": true, "thoughtSignature": true,
	"encrypted_content": true, "page_age": true, "thinking": true,
	"reasoning_content": true, "user": true, "metadata": true,
	// 会话级标识符（codex 把同一会话 UUID 塞进 prompt_cache_key / session_id /
	// thread_id / turn_id / root_turn_id / x-codex-* 镜像等大量字段）：值随会话变化、
	// 与结构无关——不折叠的话跨会话的同形状请求会被判成不同形状，去重碎片化。
	// key 名无法穷举，裸 UUID 形态的值另由 uuidPattern 按值形态兜底
	"prompt_cache_key": true, "session_id": true, "thread_id": true,
	"turn_id": true, "root_turn_id": true,
	"x-codex-window-id": true, "x-codex-installation-id": true,
}

// enumLikeMax 判定为枚举值的字符串长度上限（超过视为自由文本）。
const enumLikeMax = 40

// uuidPattern 标准 UUID 形态（允许尾部 :N 轮次后缀，codex 的 x-codex-window-id 用）。
// 会话/安装级标识符经常被客户端塞进任意命名的字段（prompt_cache_key、x-codex-* 镜像等），
// key 名不可枚举，只能按值形态识别——UUID 永远不是决定转换分支的判别式。
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}(:[0-9]+)?$`)

// bareHexIDPattern OpenAI 风格的 32 位十六进制 id（无连字符）。
var bareHexIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// isIDLikeKey 判断对象 key 是否为「按 id 索引」的动态 key。
// Responses 的 usage 明细按消息 id 建 map（msg_<uuid> / 32 位十六进制），
// id 由服务端逐次生成——原样进指纹的话这类报文每次都是新形状，去重永远失效。
// 仅认 UUID / 32 位十六进制 / 已知 id 前缀 + UUID 三种保守形态，
// 不敢按前缀宽松匹配（如 item_ 前缀会误伤 item_type 这类正常字段名）。
func isIDLikeKey(k string) bool {
	if uuidPattern.MatchString(k) || bareHexIDPattern.MatchString(k) {
		return true
	}
	for _, p := range knownIDPrefixes {
		if strings.HasPrefix(k, p) && uuidPattern.MatchString(k[len(p):]) {
			return true
		}
	}
	return false
}

// Fingerprint 计算 JSON 体的结构指纹。非 JSON 返回 ok=false（流式语料走 FingerprintStream）。
func Fingerprint(body []byte) (string, bool) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return "", false
	}
	return hashShape(shapeOf(v, "")), true
}

// FingerprintStream 计算 SSE / NDJSON 流的结构指纹。
//
// 逐帧取 data 负载的形状，再对帧形状序列做游程压缩——一个流有 3 个文本增量帧
// 还是 300 个，结构上是同一件事，不折叠的话每条流都是独一无二的形状、去重失效。
func FingerprintStream(body []byte) string {
	frames := parseStreamFrames(body)
	shapes := make([]any, 0, len(frames))
	for _, f := range frames {
		var v any
		if err := json.Unmarshal([]byte(f.data), &v); err != nil {
			// 非 JSON 负载（如 [DONE] 哨兵）按原文入形状，它是协议的一部分
			shapes = append(shapes, map[string]any{"event": f.event, "raw": f.data})
			continue
		}
		shapes = append(shapes, map[string]any{"event": f.event, "data": shapeOf(v, "")})
	}
	return hashShape(runLengthCollapse(shapes))
}

func hashShape(shape any) string {
	raw, err := json.Marshal(shape)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// shapeOf 递归归一化为「结构」：
//   - 对象：保留 key（排序后），值递归；
//   - 数组：逐元素求形状后游程压缩，计数分桶（1/2/3/many）；
//   - 标量：判别式字符串保留原值，其余折叠为类型占位。
//
// key 参数是该值所在的字段名，用于判断字符串是否为自由文本。
func shapeOf(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		// 按 id 索引的动态 key（usage 明细等）折叠进同一个桶：
		// 保留条目数量（分桶）与值的形状，抹掉 id 本身
		var idKeyed []any
		for _, k := range keys {
			if isIDLikeKey(k) {
				idKeyed = append(idKeyed, shapeOf(t[k], k))
				continue
			}
			out[k] = shapeOf(t[k], k)
		}
		if idKeyed != nil {
			out["#id-keyed"] = runLengthCollapse(idKeyed)
		}
		return out

	case []any:
		if len(t) == 0 {
			return []any{}
		}
		shapes := make([]any, 0, len(t))
		for _, el := range t {
			shapes = append(shapes, shapeOf(el, key))
		}
		return runLengthCollapse(shapes)

	case string:
		if isEnumLike(key, t) {
			return t
		}
		return "#str"
	case float64:
		return "#num"
	case bool:
		return "#bool"
	case nil:
		return "#null"
	default:
		return "#unknown"
	}
}

// isEnumLike 判断字符串值是否为判别式枚举（决定转换分支，须进指纹）。
func isEnumLike(key, val string) bool {
	if freeTextKeys[key] {
		return false
	}
	// 裸 UUID 是客户端标识符而非枚举：不同会话的 UUID 各不相同，
	// 当枚举保留会让跨会话的同形状请求指纹互异、去重碎片化
	if uuidPattern.MatchString(val) {
		return false
	}
	if val == "" || len(val) > enumLikeMax {
		return false
	}
	// 含空白即视为自然语言
	return !strings.ContainsAny(val, " \t\n\r")
}

// runLengthCollapse 对形状序列做游程压缩：连续的相同形状折叠为 {shape, n}，
// n 按 1/2/3/many 分桶——区分「单条消息」与「多轮对话」这类结构差异，
// 同时把「50 轮」与「51 轮」这类长度噪音抹平。
func runLengthCollapse(shapes []any) []any {
	out := make([]any, 0, len(shapes))
	var lastKey string
	count := 0

	flush := func(shape any) {
		if count == 0 {
			return
		}
		out = append(out, map[string]any{"shape": shape, "n": bucketCount(count)})
	}

	var lastShape any
	for _, s := range shapes {
		raw, err := json.Marshal(s)
		if err != nil {
			continue
		}
		k := string(raw)
		if count > 0 && k == lastKey {
			count++
			continue
		}
		flush(lastShape)
		lastShape, lastKey, count = s, k, 1
	}
	flush(lastShape)
	return out
}

func bucketCount(n int) string {
	switch {
	case n <= 1:
		return "1"
	case n == 2:
		return "2"
	case n == 3:
		return "3"
	default:
		return "many"
	}
}

// streamFrame 流中的一帧（SSE 事件名 + data 负载；NDJSON 的 event 为空）。
type streamFrame struct {
	event string
	data  string
}

// parseStreamFrames 把 SSE 或 NDJSON 流拆成帧。
// 含 "data:" 前缀行的按 SSE 解析（空行分帧），否则按 NDJSON 逐行解析。
func parseStreamFrames(body []byte) []streamFrame {
	text := string(body)
	if !strings.Contains(text, "data:") {
		return parseNDJSONFrames(text)
	}

	var frames []streamFrame
	for _, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var f streamFrame
		var dataLines []string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event:"):
				f.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if len(dataLines) == 0 && f.event == "" {
			continue
		}
		f.data = strings.Join(dataLines, "\n")
		frames = append(frames, f)
	}
	return frames
}

func parseNDJSONFrames(text string) []streamFrame {
	var frames []streamFrame
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		frames = append(frames, streamFrame{data: line})
	}
	return frames
}
