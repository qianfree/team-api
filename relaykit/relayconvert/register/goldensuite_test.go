package register

// goldensuite_test.go — 注册表级金样本套件（全方向请求 + 响应）。
//
// 与各转换器包内的 golden_test 不同，本套件由注册表驱动：一套共享场景语料
// （golden/inputs/<入站格式>__<场景>.json，wire JSON）自动覆盖所有已注册方向，
// 新注册方向零成本纳入。期望输出按转换器落盘在
// golden/expected/<转换器ID>/<场景>.request.json（响应侧 .response.json）。
//
// 工作流：
//   - 首次生成 / 确认新行为：go test -run TestGoldenSuite -update-golden ./relayconvert/register/
//   - 日常回归：任何改动导致某方向输出变化，对应 expected 文件比对失败；
//     若变化符合预期，重新 -update-golden 后 git diff 审查 expected 文件的变化面——
//     改 A 方向却看到 B 方向的 expected 变化，即为意外的跨方向影响。
//   - 语料扩充：把真实客户端流量（脱敏后）按场景命名放进 golden/inputs/ 即自动生效。
//
// 响应侧非确定字段（合成的 id / created 时间戳等）在比对前于全树置为固定占位符。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/goldentest"
	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

// goldenScenarios 请求场景清单；某入站格式缺少某场景语料时该组合自动跳过。
var goldenScenarios = []string{"basic", "full", "websearch", "thinking"}

// nondeterministicKeys 响应侧转换器合成的非确定字段（任意层级，按 key 名匹配）。
// 比对前统一置为占位符，保证金样本可重复。
var nondeterministicKeys = map[string]bool{
	"id":           true,
	"created":      true,
	"created_at":   true,
	"completed_at": true,
	"responseId":   true,
	"response_id":  true,
	// call_id 不整体清洗：透传自输入的具确定性；仅合成形态按 synthesizedCallID 模式清洗
}

func TestGoldenSuite_Requests(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListRequestConverterIDs() {
		spec, ok := relayconvert.LookupRequestConverter(id)
		require.True(t, ok)

		for _, scenario := range goldenScenarios {
			inPath := filepath.Join("golden", "inputs", string(spec.From)+"__"+scenario+".json")
			input, err := os.ReadFile(inPath)
			if os.IsNotExist(err) {
				continue // 该入站格式无此场景语料
			}
			require.NoError(t, err)

			t.Run(id+"/"+scenario, func(t *testing.T) {
				parsed := parseWireRequest(t, spec.From, string(input))
				converted, convErr := relayconvert.ConvertRequestByID(ctx, capMeta(), id, parsed)
				require.NoError(t, convErr, "转换失败")
				compareOrUpdateGolden(t,
					filepath.Join("golden", "expected", id, scenario+".request.json"),
					converted, false)
			})
		}
	}
}

func TestGoldenSuite_Responses(t *testing.T) {
	ctx := context.Background()

	for _, id := range relayconvert.ListResponseConverterIDs() {
		spec, ok := relayconvert.LookupResponseConverter(id)
		require.True(t, ok)
		if spec.Convert == nil {
			continue // 纯流式登记不在非流式金样本范围
		}

		input, hasInput := upstreamWireResponses[spec.To]
		if !hasInput {
			t.Errorf("响应转换器 %s 的上游格式 %s 没有响应语料", id, spec.To)
			continue
		}

		t.Run(id, func(t *testing.T) {
			parsed := parseWireResponse(t, spec.To, input)
			converted, _, convErr := spec.Convert(ctx, capMeta(), parsed)
			require.NoError(t, convErr, "响应转换失败")
			compareOrUpdateGolden(t,
				filepath.Join("golden", "expected", id, "full.response.json"),
				converted, true)
		})
	}
}

// compareOrUpdateGolden 将转换结果与金样本文件比对；-update-golden 时重写文件。
// scrub=true 时先将全树非确定字段置为占位符（响应侧）。
func compareOrUpdateGolden(t *testing.T, path string, converted any, scrub bool) {
	t.Helper()

	raw, err := json.Marshal(converted)
	require.NoError(t, err, "marshal 转换结果")
	var actual any
	require.NoError(t, json.Unmarshal(raw, &actual))
	if scrub {
		actual = scrubNondeterministic(actual)
	}

	if *goldentest.Update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		data, err := json.MarshalIndent(actual, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o644))
		t.Logf("金样本已更新: %s", path)
		return
	}

	expectedRaw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("金样本 %s 不存在，首次生成请运行: go test -run TestGoldenSuite -update-golden ./relayconvert/register/", path)
	}
	require.NoError(t, err)
	var expected any
	require.NoError(t, json.Unmarshal(expectedRaw, &expected))

	if !goldentest.Equal(actual, expected) {
		actualPretty, _ := json.MarshalIndent(actual, "", "  ")
		t.Errorf("转换输出与金样本 %s 不一致。\n若变更符合预期，运行 -update-golden 重生并在 git diff 中审查变化面。\n实际输出:\n%s", path, actualPretty)
	}
}

// synthesizedCallID 匹配转换器合成的 call_id（如 Gemini functionCall 无 id 时按
// 纳秒时间戳生成 call_<ts>_<idx>）；透传自输入的 call_id（call_1 / toolu_1 等）不清洗。
var synthesizedCallID = regexp.MustCompile(`^call_\d{15,}_\d+$`)

// scrubNondeterministic 递归将非确定字段置为固定占位符。
// 只处理值为字符串/数字的命中 key，避免误伤同名的结构字段。
func scrubNondeterministic(v any) any {
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
			if k == "call_id" {
				if s, ok := val.(string); ok && synthesizedCallID.MatchString(s) {
					node[k] = "<scrubbed>"
					continue
				}
			}
			node[k] = scrubNondeterministic(val)
		}
		return node
	case []any:
		for i := range node {
			node[i] = scrubNondeterministic(node[i])
		}
		return node
	default:
		return v
	}
}

// TestGoldenSuite_InputCorpusRegistered 语料目录完整性：inputs 下的文件名必须能被
// 套件识别（<已知格式>__<已知场景>.json），防止拼错文件名导致语料被静默跳过。
func TestGoldenSuite_InputCorpusRegistered(t *testing.T) {
	knownFormats := map[string]bool{
		string(types.RelayFormatOpenAI):          true,
		string(types.RelayFormatClaude):          true,
		string(types.RelayFormatGemini):          true,
		string(types.RelayFormatOpenAIResponses): true,
	}
	knownScenarios := map[string]bool{}
	for _, s := range goldenScenarios {
		knownScenarios[s] = true
	}

	files, err := os.ReadDir(filepath.Join("golden", "inputs"))
	require.NoError(t, err)
	for _, f := range files {
		name := strings.TrimSuffix(f.Name(), ".json")
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 || !knownFormats[parts[0]] || !knownScenarios[parts[1]] {
			t.Errorf("语料文件 %s 不符合 <格式>__<场景>.json 命名（格式∈%v，场景∈%v），会被套件静默跳过",
				f.Name(), keysOf(knownFormats), goldenScenarios)
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
