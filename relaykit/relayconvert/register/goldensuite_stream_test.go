package register

// goldensuite_stream_test.go — 注册表级流式金样本套件（全部流式方向）。
//
// 与非流式套件（goldensuite_test.go）同构：一套共享上游流语料
// （golden/stream_inputs/<上游格式>__<场景>.txt，SSE / NDJSON wire 原文）由
// ListStreamConverterSpecs 枚举驱动全部已注册流式方向，新注册方向零成本纳入。
// 期望输出（帧序列）按转换器落盘在 golden/expected/<转换器ID>/<场景>.stream.json。
//
// 帧序列表达：每帧 {event?, data, usage?}——
//   - OpenAI chat chunk（*dto.ChatCompletionStreamResponse）：无 event，data 为 chunk 本体；
//   - StreamEvent：event 为事件名（Claude/Responses 风格；Gemini 风格为空省略），
//     usage 为宿主将捕获的计费用量（billing 契约的一部分，必须锁进金样本）。
//
// 工作流与非流式套件一致：
//   - 首次生成 / 确认新行为：go test -run TestGoldenSuite -update-golden ./relayconvert/register/
//   - 日常回归：输出变化 → 比对失败；符合预期则 -update-golden 后 git diff 审查变化面。
//
// 另含 TestStreamInvariants_ToolsAndUsage：不锁具体形状，只断言每个方向的语义底线
// （工具名不丢、用量可捕获、终止语义符合客户端协议），金样本重生成也绕不过它。

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

// streamScenarios 流式场景清单；某上游格式缺少某场景语料时该组合自动跳过。
var streamScenarios = []string{"basic", "tools", "thinking"}

// streamFrame 一帧转换输出的金样本形态。
type streamFrame struct {
	Event string `json:"event,omitempty"`
	Data  any    `json:"data"`
	Usage any    `json:"usage,omitempty"`
}

// streamStats 转换过程的语义统计（供不依赖金样本形状的守恒断言使用）。
type streamStats struct {
	usageSeen      bool     // 出现过宿主可捕获的用量（chat chunk Usage 或 StreamEvent.Usage）
	finishSeen     bool     // chat chunk 出现过非空 finish_reason
	eventNames     []string // StreamEvent 事件名序列（Gemini 风格空名也计入，为空串）
	chatChunkCount int
}

// runStreamConverter 以注册表查到的流式转换器消费语料，收集帧序列与语义统计。
func runStreamConverter(t *testing.T, converterID string, corpus []byte) ([]streamFrame, *streamStats) {
	t.Helper()

	fn, ok := relayconvert.LookupStreamConverterByID(converterID)
	require.True(t, ok, "流式转换器 %s 未注册", converterID)

	var frames []streamFrame
	stats := &streamStats{}
	err := fn(context.Background(), capMeta(), bytes.NewReader(corpus), func(chunk any) error {
		switch c := chunk.(type) {
		case *dto.ChatCompletionStreamResponse:
			stats.chatChunkCount++
			if c.Usage != nil {
				stats.usageSeen = true
			}
			for _, choice := range c.Choices {
				if choice.FinishReason != nil {
					stats.finishSeen = true
				}
			}
			frames = append(frames, streamFrame{Data: c})
		case *relayconvert.StreamEvent:
			if c == nil {
				return nil
			}
			stats.eventNames = append(stats.eventNames, c.Event)
			f := streamFrame{Event: c.Event, Data: c.Data}
			if c.Usage != nil {
				stats.usageSeen = true
				f.Usage = c.Usage
			}
			frames = append(frames, f)
		default:
			t.Errorf("流式转换器 %s 产出未知 chunk 类型 %T（宿主桥接层会静默丢弃）", converterID, chunk)
		}
		return nil
	})
	require.NoError(t, err, "流式转换失败")
	return frames, stats
}

func TestGoldenSuite_Streams(t *testing.T) {
	for _, spec := range relayconvert.ListStreamConverterSpecs() {
		for _, scenario := range streamScenarios {
			inPath := filepath.Join("golden", "stream_inputs", string(spec.From)+"__"+scenario+".txt")
			corpus, err := os.ReadFile(inPath)
			if os.IsNotExist(err) {
				continue // 该上游格式无此场景语料
			}
			require.NoError(t, err)

			t.Run(spec.ID+"/"+scenario, func(t *testing.T) {
				frames, _ := runStreamConverter(t, spec.ID, corpus)
				require.NotEmpty(t, frames, "转换器未产出任何帧")

				// 帧序列 → 通用 JSON 树 → 清洗非确定字段，再走与非流式一致的比对/重生流程
				raw, err := json.Marshal(frames)
				require.NoError(t, err)
				var tree any
				require.NoError(t, json.Unmarshal(raw, &tree))
				tree = scrubStreamNondeterministic(tree)

				compareOrUpdateGolden(t,
					filepath.Join("golden", "expected", spec.ID, scenario+".stream.json"),
					tree, false)
			})
		}
	}
}

// synthesizedStreamID 流式转换器按时间戳合成的 ID 形态（resp_/msg_/fc_/chatcmpl- 前缀
// 接毫秒/纳秒时间戳，call_<ts>[_<idx>]，或 reasoning 项的 rs_resp_<ts>[_<idx>]）。
// 透传自语料的短 ID（msg_up / toolu_1 / call_1 等）不匹配、不清洗，保持可断言。
var synthesizedStreamID = regexp.MustCompile(`^(?:resp_|msg_|fc_|chatcmpl-)\d{10,}$|^(?:call_|rs_resp_)\d{10,}(?:_\d+)?$`)

// scrubStreamNondeterministic 递归清洗流式帧树中的非确定字段：
//   - 命中 nondeterministicKeys 的 key（id/created/created_at/completed_at 等）按类型置占位；
//   - 任意位置的字符串值若匹配合成 ID 模式（如 item_id 里的 msg_<纳秒>）也置占位。
func scrubStreamNondeterministic(v any) any {
	switch node := v.(type) {
	case map[string]any:
		for k, val := range node {
			if nondeterministicKeys[k] {
				switch val.(type) {
				case string:
					node[k] = "<scrubbed>"
				case float64:
					node[k] = 0
				}
				continue
			}
			node[k] = scrubStreamNondeterministic(val)
		}
		return node
	case []any:
		for i := range node {
			node[i] = scrubStreamNondeterministic(node[i])
		}
		return node
	case string:
		if synthesizedStreamID.MatchString(node) {
			return "<scrubbed>"
		}
		return node
	default:
		return v
	}
}

// TestStreamInvariants_ToolsAndUsage 流式语义守恒：对每个已注册流式方向灌入 tools
// 语料（含工具调用 + 用量 + 正常收尾的上游流），断言与金样本形状无关的语义底线：
//
//  1. 工具调用名不得静默丢失（转换后帧序列中必须出现 get_weather）；
//  2. 用量必须以宿主可捕获的形态产出（chat chunk 的 Usage 或 StreamEvent.Usage），
//     否则计费按 0 token 结算；
//  3. 终止语义符合客户端协议：OpenAI 需带 finish_reason 的 chunk、Claude 需
//     message_stop 事件、Responses 需 response.completed 事件、Gemini 需 finishReason 字段。
//
// 每个上游格式必须提供 tools 语料（新增上游流格式时在 stream_inputs 补齐）。
func TestStreamInvariants_ToolsAndUsage(t *testing.T) {
	for _, spec := range relayconvert.ListStreamConverterSpecs() {
		t.Run(spec.ID, func(t *testing.T) {
			corpus, err := os.ReadFile(filepath.Join("golden", "stream_inputs", string(spec.From)+"__tools.txt"))
			require.NoError(t, err, "上游格式 %s 缺少 tools 流式语料，请在 golden/stream_inputs/ 补充", spec.From)

			frames, stats := runStreamConverter(t, spec.ID, corpus)
			require.NotEmpty(t, frames)

			raw, err := json.Marshal(frames)
			require.NoError(t, err)
			require.Contains(t, string(raw), "get_weather", "工具调用名在流式转换后丢失（静默能力丢失）")

			require.True(t, stats.usageSeen, "未产出宿主可捕获的用量（计费将按 0 token 结算）")

			switch spec.To {
			case types.RelayFormatOpenAI:
				require.True(t, stats.finishSeen, "OpenAI 客户端方向未产出带 finish_reason 的终止 chunk")
			case types.RelayFormatClaude:
				require.Contains(t, stats.eventNames, "message_stop", "Claude 客户端方向未产出 message_stop 终止事件")
			case types.RelayFormatOpenAIResponses:
				require.Contains(t, stats.eventNames, "response.completed", "Responses 客户端方向未产出 response.completed 终止事件")
			case types.RelayFormatGemini:
				require.Contains(t, string(raw), "finishReason", "Gemini 客户端方向未产出带 finishReason 的终止帧")
			default:
				t.Errorf("未支持的客户端格式 %s，请补充终止语义断言", spec.To)
			}
		})
	}
}

// TestGoldenSuite_StreamInputCorpusRegistered 流式语料目录完整性：stream_inputs 下的
// 文件名必须能被套件识别（<已知上游格式>__<已知场景>.txt），防止拼错文件名被静默跳过。
func TestGoldenSuite_StreamInputCorpusRegistered(t *testing.T) {
	knownFormats := map[string]bool{
		string(types.RelayFormatOpenAI):          true,
		string(types.RelayFormatClaude):          true,
		string(types.RelayFormatGemini):          true,
		string(types.RelayFormatOpenAIResponses): true,
		string(types.RelayFormatOllama):          true,
	}
	knownScenarios := map[string]bool{}
	for _, s := range streamScenarios {
		knownScenarios[s] = true
	}

	files, err := os.ReadDir(filepath.Join("golden", "stream_inputs"))
	require.NoError(t, err)
	for _, f := range files {
		name := strings.TrimSuffix(f.Name(), ".txt")
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 || !knownFormats[parts[0]] || !knownScenarios[parts[1]] {
			t.Errorf("流式语料文件 %s 不符合 <上游格式>__<场景>.txt 命名（格式∈%v，场景∈%v），会被套件静默跳过",
				f.Name(), keysOf(knownFormats), streamScenarios)
		}
	}
}

// geminiGroundedStream 带 groundingMetadata 的 Gemini 上游流：
// grounding 只在**末帧**到达，正文增量此前早已推给客户端。
const geminiGroundedStream = `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Boston is 68F"}]}}]}

data: {"candidates":[{"index":0,"finishReason":"STOP","groundingMetadata":{"webSearchQueries":["boston weather"],"groundingChunks":[{"web":{"uri":"https://weather.example/boston","title":"weather.example"}}],"groundingSupports":[{"segment":{"startIndex":0,"endIndex":6,"text":"Boston"},"groundingChunkIndices":[0]}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}

`

// TestStreamInvariants_GroundingRestore 流式响应侧搜索证据还原：
// Gemini 上游在末帧给出 groundingMetadata 时，来源 URL 必须出现在客户端收到的流里。
//
// 这是非流式 grounding 还原（TestCapabilityInvariants_GroundingRestore）的流式对照。
// 流式的难点在于时序：证据到得比正文晚，转换器必须缓冲并在收尾补发，
// 漏做就会出现「非流式有引用、流式没有」的不一致——而客户端通常只用流式。
func TestStreamInvariants_GroundingRestore(t *testing.T) {
	for _, spec := range relayconvert.ListStreamConverterSpecs() {
		if spec.From != types.RelayFormatGemini {
			continue // grounding 是 Gemini 侧构件
		}

		t.Run(spec.ID, func(t *testing.T) {
			frames, _ := runStreamConverter(t, spec.ID, []byte(geminiGroundedStream))
			require.NotEmpty(t, frames)

			raw, err := json.Marshal(frames)
			require.NoError(t, err)
			body := string(raw)

			require.Contains(t, body, "https://weather.example/boston",
				"流式转换后搜索来源丢失（非流式已还原、流式漏做会导致两者行为不一致）\n实际帧序列: %s", body)
			require.Contains(t, body, "Boston is 68F", "回答正文在 grounding 还原后丢失")
		})
	}
}
