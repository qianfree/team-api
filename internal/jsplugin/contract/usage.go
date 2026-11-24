package contract

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// 宿主固定用量词汇：插件只回答「用了多少」（estimateUsage 预扣估算、终态
// items[].usage / immediate.usage 官方用量），按什么口径扣多少钱完全由宿主的
// 模型定价决定（per_second 矩阵 / 公式 u() / 素材方案）。插件不得自定义用量键，
// 也不上报上游消费（积分/成本）——那是插件与上游之间的事，与平台计费无关。
const (
	UsageOutputSeconds     = "output_seconds"      // 生成时长（秒）
	UsageResolution        = "resolution"          // 输出分辨率档位（宿主归一：480P/720P/768P/1080P/2K/4K）
	UsageOutputCount       = "output_count"        // 产出数量（图片张数 / 歌曲段数等）
	UsageInputImageCount   = "input_image_count"   // 输入图片数（含首尾帧）
	UsageInputVideoSeconds = "input_video_seconds" // 输入参考视频时长（秒）
	UsageInputAudioSeconds = "input_audio_seconds" // 输入参考音频时长（秒）
	UsageInputTokens       = "input_tokens"        // 输入 token（按 token 计价的上游）
	UsageOutputTokens      = "output_tokens"       // 输出 token
	UsageGenerateAudio     = "generate_audio"      // 是否生成音频（bool）
)

// usageKind 用量键的值类型。
type usageKind int

const (
	usageNumber usageKind = iota
	usageString
	usageBool
)

// HostUsageField 固定用量键说明（管理端/公式编辑器速查与文档同源）。
type HostUsageField struct {
	Key         string `json:"key"`
	Type        string `json:"type"` // number | string | boolean
	Description string `json:"description"`
}

var hostUsageKinds = map[string]usageKind{
	UsageOutputSeconds:     usageNumber,
	UsageResolution:        usageString,
	UsageOutputCount:       usageNumber,
	UsageInputImageCount:   usageNumber,
	UsageInputVideoSeconds: usageNumber,
	UsageInputAudioSeconds: usageNumber,
	UsageInputTokens:       usageNumber,
	UsageOutputTokens:      usageNumber,
	UsageGenerateAudio:     usageBool,
}

var hostUsageDocs = map[string]string{
	UsageOutputSeconds:     "生成时长（秒）",
	UsageResolution:        "输出分辨率档位（480P/720P/768P/1080P/2K/4K，per_second 矩阵查价键）",
	UsageOutputCount:       "产出数量（图片张数/歌曲段数等）",
	UsageInputImageCount:   "输入图片数（含首尾帧）",
	UsageInputVideoSeconds: "输入参考视频时长（秒）",
	UsageInputAudioSeconds: "输入参考音频时长（秒）",
	UsageInputTokens:       "输入 token 数",
	UsageOutputTokens:      "输出 token 数",
	UsageGenerateAudio:     "是否生成音频（公式中 u() 取 1/0）",
}

// HostUsageFields 返回固定用量键说明（按键名排序）。
func HostUsageFields() []HostUsageField {
	keys := make([]string, 0, len(hostUsageKinds))
	for k := range hostUsageKinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]HostUsageField, 0, len(keys))
	for _, k := range keys {
		t := "number"
		switch hostUsageKinds[k] {
		case usageString:
			t = "string"
		case usageBool:
			t = "boolean"
		}
		out = append(out, HostUsageField{Key: k, Type: t, Description: hostUsageDocs[k]})
	}
	return out
}

// HostUsageFactDocs 进公式 u() 通道的用量键（number + bool）→ 说明，供 formula 注册。
func HostUsageFactDocs() map[string]string {
	out := make(map[string]string, len(hostUsageKinds))
	for k, kind := range hostUsageKinds {
		if kind == usageString {
			continue
		}
		out[k] = hostUsageDocs[k]
	}
	return out
}

// HostUsage 校验后的用量。数值键缺失 = 未上报（区别于显式 0）。
type HostUsage struct {
	Numbers       map[string]float64 // number 键（不含 generate_audio）
	Resolution    string             // 已归一档位；空 = 未上报
	GenerateAudio *bool              // nil = 未上报
}

// Facts 公式 u() 通道的事实集：全部数值键 + generate_audio（1/0）。
func (u *HostUsage) Facts() map[string]float64 {
	out := make(map[string]float64, len(u.Numbers)+1)
	for k, v := range u.Numbers {
		out[k] = v
	}
	if u.GenerateAudio != nil {
		if *u.GenerateAudio {
			out[UsageGenerateAudio] = 1
		} else {
			out[UsageGenerateAudio] = 0
		}
	}
	return out
}

// Number 取数值键（未上报返回 0,false）。
func (u *HostUsage) Number(key string) (float64, bool) {
	v, ok := u.Numbers[key]
	return v, ok
}

// IsEmpty 无任何上报。
func (u *HostUsage) IsEmpty() bool {
	return len(u.Numbers) == 0 && u.Resolution == "" && u.GenerateAudio == nil
}

// ValidateHostUsage 校验用量对象：键 ∈ 固定词汇、类型匹配、数值非负有限。
// 未知键整体拒绝（防拼写错误静默少计费）。数值兼容数字字符串（"10"）。
func ValidateHostUsage(raw map[string]json.RawMessage) (*HostUsage, error) {
	out := &HostUsage{Numbers: make(map[string]float64, len(raw))}
	for key, valueJSON := range raw {
		kind, ok := hostUsageKinds[key]
		if !ok {
			return nil, gerror.Newf("usage.%s 不是宿主用量键（可用：%s）", key, strings.Join(hostUsageKeyList(), ", "))
		}
		var v any
		if err := json.Unmarshal(valueJSON, &v); err != nil {
			return nil, gerror.Newf("usage.%s 不是合法 JSON 值", key)
		}
		if v == nil {
			continue // null = 未上报
		}
		switch kind {
		case usageNumber:
			num, ok := usageFloat(v)
			if !ok {
				return nil, gerror.Newf("usage.%s 需为数值", key)
			}
			if num < 0 || math.IsInf(num, 0) || math.IsNaN(num) {
				return nil, gerror.Newf("usage.%s 需为非负有限数值", key)
			}
			out.Numbers[key] = num
		case usageString:
			s, ok := v.(string)
			if !ok {
				return nil, gerror.Newf("usage.%s 需为字符串", key)
			}
			out.Resolution = NormalizeResolution(s)
		case usageBool:
			b, ok := v.(bool)
			if !ok {
				return nil, gerror.Newf("usage.%s 需为布尔值", key)
			}
			out.GenerateAudio = &b
		}
	}
	return out, nil
}

func hostUsageKeyList() []string {
	keys := make([]string, 0, len(hostUsageKinds))
	for k := range hostUsageKinds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func usageFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

var (
	resolutionWxH = regexp.MustCompile(`^(\d+)\s*[x*×]\s*(\d+)$`)
	resolutionNum = regexp.MustCompile(`^(\d+)\s*[pP]?$`)
)

// NormalizeResolution 分辨率归一为 per_second 矩阵档位键：
//   - "720p"/"720P"/"720" → "720P"；"2k"/"4K" → "2K"/"4K"
//   - "1280x720" / "720*1280" → 按短边取档（720P）；短边 1440/2160 → 2K/4K
//   - 无法识别的值原样大写返回（矩阵查价仍可大小写不敏感命中自定义档位）
func NormalizeResolution(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	upper := strings.ToUpper(v)
	switch upper {
	case "2K", "4K", "8K":
		return upper
	}
	if m := resolutionWxH.FindStringSubmatch(v); m != nil {
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		short := w
		if h < short {
			short = h
		}
		return shortEdgeTier(short)
	}
	if m := resolutionNum.FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[1])
		return shortEdgeTier(n)
	}
	return upper
}

func shortEdgeTier(n int) string {
	switch {
	case n >= 2160:
		return "4K"
	case n >= 1440:
		return "2K"
	}
	return strconv.Itoa(n) + "P"
}
