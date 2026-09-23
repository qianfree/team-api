package corpus

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Conversion 调试日志的 conversion JSONB 字段（协议转换方向元数据）。
type Conversion struct {
	ClientFormat   string   `json:"client_format"`
	UpstreamFormat string   `json:"upstream_format"`
	Chain          []string `json:"chain,omitempty"`
	Bridge         string   `json:"bridge,omitempty"`
}

// Record 一条已解码的调试日志（chn_debug_logs 的一行）。
// 只取提取语料所需字段：段1（客户端请求）与段3（上游响应）是转换器的**输入**；
// 段2/段4 是当时的转换**输出**，属于「生产一致性校验」的范畴，本包不处理。
type Record struct {
	ID             int64
	RequestID      string
	ChannelType    int
	ModelName      string
	UpstreamModel  string
	RelayMode      string
	InboundPath    string
	IsStream       bool
	UpstreamStatus int
	CapturedAt     time.Time

	ClientReqBody    []byte // 段1
	UpstreamRespBody []byte // 段3
	Conversion       Conversion
}

// SampleKind 语料类别，决定落盘目录与文件扩展名。
type SampleKind string

const (
	// KindRequest 客户端请求语料（段1）→ 驱动请求侧转换的全部方向
	KindRequest SampleKind = "request"
	// KindStreamResponse 上游流式响应语料（段3，is_stream）→ 驱动流式转换
	KindStreamResponse SampleKind = "stream_response"
	// KindResponse 上游非流式响应语料（段3）→ 驱动响应侧转换
	KindResponse SampleKind = "response"
)

// dirOf 各类语料的落盘子目录。与既有手写语料（inputs / stream_inputs）分开存放：
// 真实语料量大且会持续增长，混进去会让手写的精选集难以阅读，
// 且金样本套件对手写目录有「文件名必须匹配已知场景」的校验。
func (k SampleKind) dirOf() string {
	switch k {
	case KindRequest:
		return "real_inputs"
	case KindStreamResponse:
		return "real_stream_inputs"
	default:
		return "real_responses"
	}
}

func (k SampleKind) ext() string {
	if k == KindStreamResponse {
		return ".txt"
	}
	return ".json"
}

// Sample 一条待落盘的语料。
type Sample struct {
	Kind      SampleKind
	Format    string // 目录内的格式前缀：请求取 client_format，响应取 upstream_format
	Features  []string
	ShapeHash string
	Body      []byte

	Meta ManifestEntry
}

// FileName 语料文件名：<格式>__<特征>_<指纹前8位>.<ext>
// 与手写语料的 <格式>__<场景> 约定保持形似，便于人工浏览时归类。
func (s Sample) FileName() string {
	feat := "basic"
	if len(s.Features) > 0 {
		feat = strings.Join(s.Features, "-")
	}
	short := s.ShapeHash
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("%s__%s_%s%s", s.Format, feat, short, s.Kind.ext())
}

// RelPath 语料相对输出根目录的路径。
func (s Sample) RelPath() string {
	return s.Kind.dirOf() + "/" + s.FileName()
}

// ManifestEntry manifest 中的一条溯源记录。
type ManifestEntry struct {
	File        string   `json:"file"`
	Kind        string   `json:"kind"`
	Format      string   `json:"format"`
	Features    []string `json:"features,omitempty"`
	ShapeHash   string   `json:"shape_hash"`
	Occurrences int      `json:"occurrences"` // 该形状在采集窗口内出现次数，反映真实流量权重

	ClientFormat   string   `json:"client_format,omitempty"`
	UpstreamFormat string   `json:"upstream_format,omitempty"`
	Chain          []string `json:"chain,omitempty"`
	Bridge         string   `json:"bridge,omitempty"`
	RelayMode      string   `json:"relay_mode,omitempty"`
	IsStream       bool     `json:"is_stream"`
	ChannelType    int      `json:"channel_type,omitempty"`
	ModelName      string   `json:"model_name,omitempty"`
	UpstreamModel  string   `json:"upstream_model,omitempty"`

	SourceRequestID string `json:"source_request_id,omitempty"`
	SourceLogID     int64  `json:"source_log_id,omitempty"`
	CapturedAt      string `json:"captured_at,omitempty"`
}

// Manifest 语料清单，随语料一同落盘，用于溯源与统计。
type Manifest struct {
	GeneratedAt string            `json:"generated_at"`
	Source      ManifestSource    `json:"source"`
	Stats       ManifestStats     `json:"stats"`
	Entries     []ManifestEntry   `json:"entries"`
	Directions  map[string]int    `json:"directions,omitempty"` // "client→upstream" → 样本数
	Formats     map[string]int    `json:"formats,omitempty"`    // 格式 → 样本数
	Notes       map[string]string `json:"notes,omitempty"`
}

// ManifestSource 采集条件。
type ManifestSource struct {
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
	Channel string `json:"channel,omitempty"`
	Format  string `json:"format,omitempty"`
	Limit   int    `json:"limit"`
}

// ManifestStats 提取统计。
type ManifestStats struct {
	Scanned       int `json:"scanned"`         // 扫描的日志行数
	SkippedNoConv int `json:"skipped_no_conv"` // 缺 conversion 元数据被跳过
	SkippedEmpty  int `json:"skipped_empty"`   // 报文体为空
	Shapes        int `json:"shapes"`          // 去重后的不同形状数
	Written       int `json:"written"`         // 实际落盘文件数
}

// ---------- 特征识别 ----------

// featureMarkers 结构特征 → 触发它的 key 或判别式值。
//
// 采用跨格式的通用扫描（遍历 JSON 的全部 key 与判别式字符串），而非按格式写四套解析：
// 各协议表达同一能力的字段名不同但都在这张表里，新增格式无需改代码。
// 特征只用于文件命名与浏览归类，识别偏差不影响语料本身。
var featureMarkers = []struct {
	feature string
	markers []string
}{
	{"tools", []string{"tools", "functionDeclarations", "tool_choice", "toolConfig"}},
	{"toolloop", []string{"tool_result", "function_call_output", "functionResponse", "tool_calls", "tool_use", "functionCall", "function_call"}},
	{"image", []string{"image_url", "inlineData", "input_image", "inline_data"}},
	{"thinking", []string{"thinking", "thinkingConfig", "reasoning", "reasoning_effort", "reasoning_content", "thought", "thoughtSignature"}},
	{"websearch", []string{"web_search_options", "googleSearch", "googleSearchRetrieval", "web_search", "web_search_20250305", "groundingMetadata"}},
	{"audio", []string{"input_audio", "audio"}},
	{"file", []string{"input_file", "fileData", "file_data"}},
}

// DetectFeatures 扫描 JSON 体识别结构特征。多轮对话（消息数 > 2）另加 multiturn。
func DetectFeatures(body []byte) []string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil
	}
	return detectFeaturesFromValue(v)
}

// DetectStreamFeatures 扫描流式语料：逐帧解析 data 负载后合并特征。
func DetectStreamFeatures(body []byte) []string {
	found := map[string]bool{}
	for _, f := range parseStreamFrames(body) {
		var v any
		if err := json.Unmarshal([]byte(f.data), &v); err != nil {
			continue
		}
		for _, feat := range detectFeaturesFromValue(v) {
			found[feat] = true
		}
	}
	return sortedFeatures(found)
}

func detectFeaturesFromValue(v any) []string {
	seen := map[string]bool{}
	collectMarkers(v, seen)

	found := map[string]bool{}
	for _, fm := range featureMarkers {
		for _, m := range fm.markers {
			if seen[m] {
				found[fm.feature] = true
				break
			}
		}
	}
	if isMultiTurn(v) {
		found["multiturn"] = true
	}
	return sortedFeatures(found)
}

// collectMarkers 递归收集全部对象 key 与判别式字符串值（type/role 等的取值本身就是标记，
// 如 content 块的 "tool_result"、Responses input 项的 "function_call_output"）。
func collectMarkers(v any, seen map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			seen[k] = true
			collectMarkers(val, seen)
		}
	case []any:
		for _, el := range t {
			collectMarkers(el, seen)
		}
	case string:
		if len(t) <= enumLikeMax && !strings.ContainsAny(t, " \t\n\r") {
			seen[t] = true
		}
	}
}

// isMultiTurn 顶层消息数组长度 > 2 即视为多轮（各协议的消息数组字段名不同，逐一尝试）。
func isMultiTurn(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"messages", "contents", "input"} {
		if arr, ok := obj[key].([]any); ok && len(arr) > 2 {
			return true
		}
	}
	return false
}

func sortedFeatures(found map[string]bool) []string {
	out := make([]string, 0, len(found))
	for f := range found {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// ---------- Record → Sample ----------

// Build 从一条调试日志提取语料样本（请求侧 + 响应侧，各自可能为空）。
// 不做去重（由调用方按 ShapeHash 聚合）。
//
// scrub 非 nil 时对报文体做脱敏，且**请求与响应共用同一个 Scrubber**——
// 二者的标识符映射必须一致，否则记录内的引用关系会在两个文件之间对不上。
//
// 结构指纹在**脱敏之后**计算：脱敏保持结构不变，但会重排 key 顺序、
// 归一化数值格式，在原文上算指纹会让同形状的记录因序列化差异被判成不同形状。
func Build(r Record, scrub *Scrubber) []Sample {
	var samples []Sample
	if scrub == nil {
		scrub = NewScrubber(ScrubOptions{})
	}
	clientBody := scrub.ScrubJSON(r.ClientReqBody)
	respBody := r.UpstreamRespBody
	if r.IsStream {
		respBody = scrub.ScrubStream(respBody)
	} else {
		respBody = scrub.ScrubJSON(respBody)
	}

	if len(clientBody) > 0 && r.Conversion.ClientFormat != "" {
		if hash, ok := Fingerprint(clientBody); ok {
			samples = append(samples, Sample{
				Kind:      KindRequest,
				Format:    r.Conversion.ClientFormat,
				Features:  DetectFeatures(clientBody),
				ShapeHash: hash,
				Body:      clientBody,
				Meta:      baseMeta(r),
			})
		}
	}

	if len(respBody) > 0 && r.Conversion.UpstreamFormat != "" && r.UpstreamStatus == 200 {
		if r.IsStream {
			samples = append(samples, Sample{
				Kind:      KindStreamResponse,
				Format:    r.Conversion.UpstreamFormat,
				Features:  DetectStreamFeatures(respBody),
				ShapeHash: FingerprintStream(respBody),
				Body:      respBody,
				Meta:      baseMeta(r),
			})
		} else if hash, ok := Fingerprint(respBody); ok {
			samples = append(samples, Sample{
				Kind:      KindResponse,
				Format:    r.Conversion.UpstreamFormat,
				Features:  DetectFeatures(respBody),
				ShapeHash: hash,
				Body:      respBody,
				Meta:      baseMeta(r),
			})
		}
	}

	// 回填各样本自身的 meta 字段
	for i := range samples {
		s := &samples[i]
		s.Meta.File = s.RelPath()
		s.Meta.Kind = string(s.Kind)
		s.Meta.Format = s.Format
		s.Meta.Features = s.Features
		s.Meta.ShapeHash = s.ShapeHash
	}
	return samples
}

func baseMeta(r Record) ManifestEntry {
	captured := ""
	if !r.CapturedAt.IsZero() {
		captured = r.CapturedAt.Format(time.RFC3339)
	}
	return ManifestEntry{
		ClientFormat:    r.Conversion.ClientFormat,
		UpstreamFormat:  r.Conversion.UpstreamFormat,
		Chain:           r.Conversion.Chain,
		Bridge:          r.Conversion.Bridge,
		RelayMode:       r.RelayMode,
		IsStream:        r.IsStream,
		ChannelType:     r.ChannelType,
		ModelName:       r.ModelName,
		UpstreamModel:   r.UpstreamModel,
		SourceRequestID: r.RequestID,
		SourceLogID:     r.ID,
		CapturedAt:      captured,
	}
}
