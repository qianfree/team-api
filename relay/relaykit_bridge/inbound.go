package relaykit_bridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// 入站请求解析与 Responses 专属预处理的单一事实源。
// handler 桥（relay/handler/relaykit_bridge.go）与共享桥（本包 request.go）共用，
// 消除两桥各自维护一份解析 switch 和 previous_response_id 哨兵判定的重复。

// ParseInboundRequest 按入站格式将请求体解析为对应 DTO 指针
// （openai → *GeneralOpenAIRequest；responses → *OpenAIResponsesRequest；
// gemini → *GeminiChatRequest；claude → *ClaudeRequest）。
// 解析失败记 Warning 并返回错误（已匹配方向的畸形请求体，由调用方显式报错）。
func ParseInboundRequest(ctx context.Context, inbound constant.RelayFormat, body []byte) (any, error) {
	switch inbound {
	case constant.RelayFormatOpenAI:
		var openaiReq dto.GeneralOpenAIRequest
		if err := json.Unmarshal(body, &openaiReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound openai request failed: %v", err)
			return nil, err
		}
		return &openaiReq, nil
	case constant.RelayFormatResponses:
		var responsesReq dto.OpenAIResponsesRequest
		if err := json.Unmarshal(body, &responsesReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound responses request failed: %v", err)
			return nil, err
		}
		return &responsesReq, nil
	case constant.RelayFormatGemini:
		// gemini 入站：gemini→claude 链走 handler 桥，gemini→openai 走共享桥。
		// 无 stash/预检需求——GeminiChatRequest 无 previous_response_id 类有状态字段
		var geminiReq dto.GeminiChatRequest
		if err := json.Unmarshal(body, &geminiReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound gemini request failed: %v", err)
			return nil, err
		}
		return &geminiReq, nil
	case constant.RelayFormatClaude:
		// claude 入站：claude→gemini 链走 handler 桥，claude→openai 走共享桥。
		// 无 stash/预检需求——ClaudeRequest 无 previous_response_id 类有状态字段
		var claudeReq dto.ClaudeRequest
		if err := json.Unmarshal(body, &claudeReq); err != nil {
			g.Log().Warningf(ctx, "[relaykit] parse inbound claude request failed: %v", err)
			return nil, err
		}
		return &claudeReq, nil
	default:
		return nil, fmt.Errorf("unsupported inbound format %s", inbound)
	}
}

// PrepareResponsesInbound Responses 入站的共享预处理：
//   - 有状态请求（previous_response_id）命中非 Responses 原生上游：会话历史存于上游
//     Responses 服务侧，降级转换会静默丢失全部上下文——返回哨兵错误
//     ErrStatefulResponsesUnsupported 驱动调度 FSM 换渠道；
//   - stash 请求快照，供响应侧合成 Responses 格式时 echo 请求参数。
func PrepareResponsesInbound(info *common.RelayInfo, responsesReq *dto.OpenAIResponsesRequest) error {
	if responsesReq.PreviousResponseID != "" {
		return fmt.Errorf("stateful responses (previous_response_id) not supported by chat-only channels: %w", constant.ErrStatefulResponsesUnsupported)
	}
	info.ResponsesRequest = responsesReq
	return nil
}
