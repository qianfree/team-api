package handler

// matrix_guard_test.go — relaykit 矩阵接管的供应商维度守卫。
//
// 背景：relaykit 转换矩阵只按协议格式裁决（入站格式 × 有效上游格式），与供应商无关；
// 方向命中后 adaptor.ConvertRequest 整个不再被调用。但供应商差异是（供应商 × RelayMode）
// 维度的，两个维度不对齐时矩阵会在自己看不见的地方替供应商做错决定，且不报错——
// 历史上已因此产生 4 个静默断链回归：
//   1. Anthropic 兼容端点漏登记（HasNativeClaudeEndpoint）→ Claude 体转成 chat 发到
//      Anthropic 端点、响应又按 OpenAI 解析，两侧同时断链；
//   2. 供应商私有后处理（RequestPostProcessor）丢失或双写；
//   3. 目标 adaptor 的 GetRequestURL 不认入站 RelayMode → DoRequest 报
//      "unsupported relay mode"、0ms 断链（bridge 测试只直测 DoResponse，测不到）；
//   4. 多协议原生透传渠道（multinative）配模型映射时被矩阵按主协议 OpenAI 接管，
//      chat 体发到 /v1/messages 原生端点，体与端点错配。
//
// 本文件把「新增/修改转换方向必查三条」的人工评审清单固化为自动化测试：
// 枚举 registry 全部供应商 × 四种文本入站，任何新增供应商 / 新增方向 / 改动判定函数
// 若破坏一致性会立即失败，不再依赖人记住检查项。

import (
	"testing"

	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// matrixInboundCombos 文本代理的四种入站（格式 × RelayMode）组合，与各入站 handler
// 的真实取值一致（mode 随请求全程不变，直达 adaptor.GetRequestURL）。
var matrixInboundCombos = []struct {
	inbound constant.RelayFormat
	mode    constant.RelayMode
}{
	{constant.RelayFormatOpenAI, constant.RelayModeChatCompletions},
	{constant.RelayFormatClaude, constant.RelayModeClaudeMessages},
	{constant.RelayFormatGemini, constant.RelayModeGeminiChat},
	{constant.RelayFormatResponses, constant.RelayModeResponses},
}

// guardProviders 枚举 registry 中全部有文本 adaptor 的供应商类型。
// 上界取宽（现有编号最大 42），新增供应商自动纳入。
func guardProviders() []constant.ProviderType {
	var out []constant.ProviderType
	for p := 1; p <= 64; p++ {
		if channel.GetAdaptor(p) != nil {
			out = append(out, constant.ProviderType(p))
		}
	}
	return out
}

// guardChannelMeta 构造最小可用渠道元数据（个别 adaptor 对 ApiKey 形态有硬要求）。
func guardChannelMeta(p constant.ProviderType) *common.ChannelMeta {
	meta := &common.ChannelMeta{
		ChannelType:       int(p),
		BaseURL:           "https://upstream.example.com",
		ApiKey:            "sk-guard-test",
		UpstreamModelName: "guard-model",
	}
	if p == constant.ProviderCloudflare {
		meta.ApiKey = "cf-token|cf-account" // cloudflare 的 URL 需要 accountID（token|accountid 格式）
	}
	return meta
}

func guardInfo(p constant.ProviderType, inbound constant.RelayFormat, mode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat: inbound,
		RelayMode:     int(mode),
		ChannelMeta:   guardChannelMeta(p),
	}
}

// matrixTakeoverExclusions 有意不参加「接管方向 URL 可达」守卫的供应商，必须注明原因。
// 新增排除项前先确认它不是第 3 类断链——排除即承诺该组合在调度层不可达。
var matrixTakeoverExclusions = map[constant.ProviderType]string{
	// codex 是 responses-only 上游（GetRequestURL 有意只认 Responses mode）。
	// 非 Responses 入站按主协议 OpenAI 参与矩阵裁决，但 codex 端点吃不了 chat 体。
	// 已知缺口：依赖渠道只挂 responses 系模型 + 调度 proto 门控避免命中；
	// 若未来放开 codex 承接 chat/claude/gemini 入站，须先实现相应桥接再移除本排除。
	constant.ProviderCodex: "responses-only 上游，非 Responses 入站不可达（渠道模型配置 + 调度门控约束）",
	// jimeng 是图片生成专用上游（GetRequestURL 有意只认 ImagesGenerations mode）。
	// 文本类入站不可达：jimeng 渠道只会挂图片模型，chat/claude/gemini/responses
	// 请求不会被调度到该渠道。
	constant.ProviderJimeng: "图片生成专用上游，文本类入站不可达（渠道只挂图片模型）",
}

// TestMatrixTakeover_AdaptorAcceptsInboundMode 守卫第 3 类断链：
// 矩阵接管的每个（供应商 × 入站）组合，目标 adaptor 的 GetRequestURL 必须认该入站
// RelayMode——矩阵已把请求体转成上游格式，URL 拿不到就是 DoRequest 处 0ms 断链。
func TestMatrixTakeover_AdaptorAcceptsInboundMode(t *testing.T) {
	for _, p := range guardProviders() {
		if reason, excluded := matrixTakeoverExclusions[p]; excluded {
			t.Logf("跳过 %s(%d)：%s", p.String(), p, reason)
			continue
		}
		for _, combo := range matrixInboundCombos {
			info := guardInfo(p, combo.inbound, combo.mode)
			upstream := relaykit_bridge.EffectiveUpstreamFormat(info)
			converterID := relaykit_bridge.RequestConverterIDForRoute(combo.inbound, upstream, info.RelayMode)
			if converterID == "" {
				continue // 同格式直连或矩阵未覆盖方向，不经 relaykit 接管
			}

			adaptor := channel.GetAdaptor(int(p))
			adaptor.Init(info)
			url, err := adaptor.GetRequestURL(info)
			if err != nil {
				t.Errorf("%s(%d) 的 %s 入站（mode=%d）被矩阵接管（%s），但 GetRequestURL 报错：%v\n"+
					"→ 该组合会在 DoRequest 处 0ms 断链。修复：让该 adaptor 的 GetRequestURL 认此 mode"+
					"（矩阵已把请求体转成 %s 格式，通常应返回对应格式的主端点）",
					p.String(), p, combo.inbound, combo.mode, converterID, err, upstream)
				continue
			}
			if url == "" {
				t.Errorf("%s(%d) 的 %s 入站：GetRequestURL 返回空 URL", p.String(), p, combo.inbound)
			}
		}
	}
}

// TestMatrixGuard_PassthroughNeverConverted 守卫第 1/4 类断链的共同根因：
// 两个判定函数必须口径一致——被 inboundMatchesChannelNative 判为「原生匹配」的
// （供应商 × 入站），矩阵必须不产生转换器（有效上游格式 = 入站格式）。
//
// 口径不一致时，同一请求在直连与转换两条路上得到不同格式结论：无映射/改写时直连正常，
// 一旦配置模型映射（canPassThrough=false 强制走转换路径）矩阵就按错误方向转换体，
// 而 adaptor 仍按原生格式选端点——即 multinative 断链（第 4 例）与 Anthropic
// 端点断链（第 1 例）的完整机理。
func TestMatrixGuard_PassthroughNeverConverted(t *testing.T) {
	for _, p := range guardProviders() {
		for _, combo := range matrixInboundCombos {
			info := guardInfo(p, combo.inbound, combo.mode)
			if !inboundMatchesChannelNative(info) {
				continue
			}
			upstream := relaykit_bridge.EffectiveUpstreamFormat(info)
			if id := relaykit_bridge.RequestConverterIDForRoute(combo.inbound, upstream, info.RelayMode); id != "" {
				t.Errorf("%s(%d) 的 %s 入站被判为原生匹配（可直连），但矩阵仍会转换（%s，有效上游=%s）\n"+
					"→ 配置模型映射后请求体会被转成 %s 而 adaptor 按 %s 原生端点发送，体与端点错配。"+
					"检查 EffectiveUpstreamFormat 与 inboundMatchesChannelNative 是否同步",
					p.String(), p, combo.inbound, id, upstream, upstream, combo.inbound)
			}
		}
	}

	// Responses 上游声明（supports_responses）同样两函数联动：原生匹配 ⇒ 不转换
	for _, p := range []constant.ProviderType{constant.ProviderOpenAI, constant.ProviderDeepSeek} {
		info := guardInfo(p, constant.RelayFormatResponses, constant.RelayModeResponses)
		info.ChannelMeta.SupportsResponses = true
		if !inboundMatchesChannelNative(info) {
			t.Errorf("%s(%d)：supports_responses 渠道的 Responses 入站应判为原生匹配", p.String(), p)
		}
		upstream := relaykit_bridge.EffectiveUpstreamFormat(info)
		if id := relaykit_bridge.RequestConverterIDForRoute(constant.RelayFormatResponses, upstream, info.RelayMode); id != "" {
			t.Errorf("%s(%d)：supports_responses 渠道的 Responses 入站不应被矩阵转换（got %s）", p.String(), p, id)
		}
	}
}

// TestMatrixGuard_NativeClaudeEndpointRegistry 守卫第 1 类断链的登记完整性：
// HasNativeClaudeEndpoint 集合必须与各 adaptor 的真实端点行为一致。
// 判据（与集合注释一致）：GetRequestURL 对 ClaudeMessages 返回与 chat **不同**的地址
// ⇔ 该供应商另挂 Anthropic 兼容端点 ⇔ 必须登记。
//
//   - 挂了独立端点却未登记 → Claude 入站被转成 chat 发到 Anthropic 端点（断链）；
//   - 未挂端点却登记了 → Claude 入站被直连到 chat 端点（chat 端点吃不了 Claude 体）。
func TestMatrixGuard_NativeClaudeEndpointRegistry(t *testing.T) {
	for _, p := range guardProviders() {
		// 只考察主协议 OpenAI 的供应商（集合语义即「主协议 OpenAI + 另挂 Anthropic 端点」）
		if helper.ProviderNativeFormat(int(p)) != constant.RelayFormatOpenAI {
			continue
		}
		// 多协议原生透传渠道由 IsMultiNativeProvider 独立判定（三种入站全部原生直连），
		// 不属于「另挂 Anthropic 端点」语义
		if constant.IsMultiNativeProvider(int(p)) {
			continue
		}

		// Claude 入站的 URL（按真实链路：Claude 入站 + ClaudeMessages mode 初始化 adaptor）
		claudeInfo := guardInfo(p, constant.RelayFormatClaude, constant.RelayModeClaudeMessages)
		claudeAdaptor := channel.GetAdaptor(int(p))
		claudeAdaptor.Init(claudeInfo)
		claudeURL, claudeErr := claudeAdaptor.GetRequestURL(claudeInfo)

		// chat 入站的 URL
		chatInfo := guardInfo(p, constant.RelayFormatOpenAI, constant.RelayModeChatCompletions)
		chatAdaptor := channel.GetAdaptor(int(p))
		chatAdaptor.Init(chatInfo)
		chatURL, chatErr := chatAdaptor.GetRequestURL(chatInfo)

		hasDistinctEndpoint := claudeErr == nil && chatErr == nil && claudeURL != chatURL
		registered := constant.HasNativeClaudeEndpoint(int(p))

		if hasDistinctEndpoint && !registered {
			t.Errorf("%s(%d)：ClaudeMessages 端点（%s）≠ chat 端点（%s），但未登记进 HasNativeClaudeEndpoint\n"+
				"→ Claude 入站会被矩阵转成 chat 发到 Anthropic 端点。请在 constant.HasNativeClaudeEndpoint 登记",
				p.String(), p, claudeURL, chatURL)
		}
		if !hasDistinctEndpoint && registered {
			t.Errorf("%s(%d)：已登记 HasNativeClaudeEndpoint 但 ClaudeMessages 与 chat 端点相同（claude=%q err=%v, chat=%q err=%v）\n"+
				"→ Claude 入站会被直连到 chat 端点（chat 端点吃不了 Claude 体）。请移除登记或修 GetRequestURL",
				p.String(), p, claudeURL, claudeErr, chatURL, chatErr)
		}
	}
}

// expectedPostProcessors 实现了 common.RequestPostProcessor 的供应商期望清单。
// 该接口仅在 relaykit 接管转换时由 convertRequestBody 调用，承载格式转换之外的
// 供应商私有适配（zhipu GLM top_p 裁剪 + 图片前缀剥离、ali DashScope thinking_budget 裁剪）。
var expectedPostProcessors = map[constant.ProviderType]bool{
	constant.ProviderAli:   true,
	constant.ProviderZhipu: true,
	// vertex：Claude 模型需注入 anthropic_version 并删除 model 字段
	//（rawPredict 端点要求）。ConvertRequest 与本接口共用 adaptClaudeBodyForVertex，
	// 该函数幂等（delete 幂等、anthropic_version 仅在缺失时写入）
	constant.ProviderVertex: true,
}

// TestMatrixGuard_RequestPostProcessorRegistry 守卫第 2 类断链：私有后处理的登记漂移。
//
//   - 新 adaptor 实现了接口却没进清单 → 失败提醒登记，并核对两条契约：
//     ① adaptor.ConvertRequest 内部含同一段后处理（legacy 路径不经本接口）；
//     ② 后处理幂等（relaykit 通用处理可能已写同名字段，不得依赖字段不存在）。
//   - 清单里的供应商不再实现接口 → 失败提醒同步移除（多为重构时误删，实际是丢私有适配）。
//
// 注意：dispatch 型 adaptor（volcengine/tencent）通过匿名嵌入接口字段委托，接口断言
// 作用在外层 wrapper 上——若未来给其子 adaptor 添加 PostProcessor，必须让 wrapper
// 也实现并转发，否则 convertRequestBody 的断言拿不到（本测试同样会暴露这一点）。
func TestMatrixGuard_RequestPostProcessorRegistry(t *testing.T) {
	for _, p := range guardProviders() {
		info := guardInfo(p, constant.RelayFormatOpenAI, constant.RelayModeChatCompletions)
		adaptor := channel.GetAdaptor(int(p))
		adaptor.Init(info)

		_, implements := adaptor.(common.RequestPostProcessor)
		expected := expectedPostProcessors[p]

		if implements && !expected {
			t.Errorf("%s(%d) 实现了 RequestPostProcessor 但未登记进 expectedPostProcessors\n"+
				"→ 请登记，并核对：① ConvertRequest 内部含同一段后处理（legacy 路径）；② 后处理幂等",
				p.String(), p)
		}
		if !implements && expected {
			t.Errorf("%s(%d) 在 expectedPostProcessors 清单中但 adaptor 未实现 RequestPostProcessor\n"+
				"→ 私有后处理疑似在重构中丢失（relaykit 接管路径将静默跳过参数裁剪），请核实",
				p.String(), p)
		}
	}
}
