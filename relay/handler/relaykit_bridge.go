package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// relaykit 请求转换器接入桥接层（唯一路径，hard-fail）。
//
// 设计要点：
//   - 转换方向由 relaykit_bridge 的方向矩阵统一裁决（入站格式 × 有效上游格式）；
//   - 方向命中后 relaykit 即唯一路径：解析失败/转换失败直接向上返回错误，不再回退
//     旧 adaptor 转换（畸形请求体 hard-fail）；
//   - 方向未命中（同格式直连、audio/rerank 等非文本模式、Ollama generate/embedding）
//     返回 handled=false，调用方走 adaptor.ConvertRequest 的原生后处理路径；
//   - 模型名替换与 thinking 注入已内化在 relaykit 转换器中；OpenAI chat 上游独有的
//     stream_options / reasoning_effort 注入由本层在转换后补齐（与旧 openai adaptor 一致）；
//   - 转换耗时与成败通过 monitor.TrackConverterCall 记录，供 dashboard 观测。

// tryConvertRequestViaRelaykit 尝试用 relaykit 转换器转换请求体。
//
// 返回值语义：
//   - handled=false, err=nil：该方向无 relaykit 转换器，调用方走 adaptor 路径；
//   - handled=true, err!=nil：方向命中但解析/转换失败，调用方应直接返回错误；
//   - handled=true, err=nil：转换成功。
func tryConvertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, false, nil
	}
	return convertRequestViaRelaykit(ctx, info, body)
}

// convertRequestViaRelaykit 是请求转换的核心（info 与 info.ChannelMeta 必须非空）。
func convertRequestViaRelaykit(ctx context.Context, info *common.RelayInfo, body []byte) (io.Reader, bool, error) {
	inbound := info.InboundFormat
	upstream := relaykit_bridge.EffectiveUpstreamFormat(info)
	converterID := relaykit_bridge.RequestConverterIDForRoute(inbound, upstream, info.RelayMode)
	if converterID == "" {
		return nil, false, nil
	}

	if _, ok := relayconvert.LookupRequestConverter(converterID); !ok {
		// 矩阵返回了 ID 但注册表没有：注册缺失属程序性错误，按未接管处理并告警
		g.Log().Errorf(ctx, "[relaykit] request converter %q not registered (inbound=%s upstream=%s)", converterID, inbound, upstream)
		return nil, false, nil
	}

	// 按入站格式解析请求体（hard-fail：解析失败即拒绝请求）
	parsed, err := parseInboundRequest(inbound, body)
	if err != nil {
		return nil, true, constant.NewRequestError(fmt.Sprintf("请求体解析失败（%s 格式）", inbound), err)
	}

	start := time.Now()
	converted, err := relayconvert.ConvertRequestByID(ctx, info, converterID, parsed)
	duration := time.Since(start)
	monitor.TrackConverterCall(converterID, string(inbound), string(upstream), duration, err)
	if err != nil {
		// responses 有状态协议不匹配：映射为宿主哨兵错误，上层按渠道级致命驱动换渠道
		if errors.Is(err, relayconvert.ErrStatefulResponsesUnsupported) {
			return nil, true, fmt.Errorf("stateful responses not supported by chat-only channel: %w", constant.ErrStatefulResponsesUnsupported)
		}
		g.Log().Warningf(ctx, "[relaykit] convert request failed (converter=%s): %v", converterID, err)
		return nil, true, fmt.Errorf("convert request (%s): %w", converterID, err)
	}

	// OpenAI chat 上游后处理：流式请求补 stream_options（客户端未显式设置时，计费需要 usage）
	if upstream == constant.RelayFormatOpenAI {
		if req, ok := converted.(*dto.GeneralOpenAIRequest); ok {
			if info.IsStream && req.StreamOptions == nil {
				req.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
			}
		}
	}

	out, err := json.Marshal(converted)
	if err != nil {
		g.Log().Errorf(ctx, "[relaykit] marshal converted request failed (converter=%s): %v", converterID, err)
		return nil, true, fmt.Errorf("marshal converted request (%s): %w", converterID, err)
	}

	return bytes.NewReader(out), true, nil
}

// parseInboundRequest 按入站格式把请求体解析为对应 DTO 指针。
func parseInboundRequest(inbound constant.RelayFormat, body []byte) (any, error) {
	switch inbound {
	case constant.RelayFormatOpenAI:
		var req dto.GeneralOpenAIRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil
	case constant.RelayFormatClaude:
		var req dto.ClaudeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil
	case constant.RelayFormatGemini:
		var req dto.GeminiChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil
	case constant.RelayFormatResponses:
		var req dto.OpenAIResponsesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil
	default:
		return nil, fmt.Errorf("unsupported inbound format %q", inbound)
	}
}
