package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/aiassistant"
	"github.com/qianfree/team-api/internal/logic/aiassistant/skills"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/response"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// aiChatRequest 对话请求体（handler 直读：SSE 直写不走 controller 链路）。
type aiChatRequest struct {
	Messages []aiassistant.ChatMessage `json:"messages" v:"required"`
	Skills   []string                  `json:"skills" dc:"激活的技能 key 列表（billing_formula / js_plugin）"`
}

// HandleAiAssistantChat AI 助手对话端点（SSE 流式）。
//
// 流式响应必须与统一 JSON 包装互斥，故走 handler 注册（同 file_serve 的归类逻辑）。
// 事件协议（前端按 type 分发）：
//
//	data: {"type":"thinking","content":"..."}     思考过程增量（推理型模型的 reasoning，与正文分开渲染）
//	data: {"type":"delta","content":"..."}        正文增量（markdown 渲染）
//	data: {"type":"code_start","skill":"js_plugin","lang":"js"}
//	data: {"type":"code_delta","content":"..."}   代码增量（专用代码卡片渲染，不经 markdown）
//	data: {"type":"code_end","implicit":true}      闭栏是服务端补的（模型未写闭栏）
//	data: {"type":"tool_start","index":0,"id":"...","name":"write_code"}   工具调用开始（js_plugin 技能）
//	data: {"type":"tool_delta","index":0,"text":"..."}                     工具参数增量（arguments JSON 原文分片）
//	data: {"type":"tool_end","index":0,"id":"...","name":"...","arguments":"{...}"} 工具调用聚合完成（以此为准解析执行）
//	data: {"type":"truncated","reason":"max_tokens","limit":N} 输出被输出上限截断
//	data: {"type":"validate", ...}                技能校验报告（流末，提取到代码且有激活技能时）
//	data: {"type":"notice","message":"..."}       非致命提示（如 max_tokens 回退重试 / 工具降级）
//	data: {"type":"usage","model":"...","prompt_tokens":N,...}
//	data: {"type":"error","message":"..."}        流中错误（此后紧跟 [DONE]）
//	data: [DONE]
//
// js_plugin 技能激活时请求携带工具定义（write_code/append_code/edit_code/validate_code），
// 模型以 tool_calls 结束一轮，前端执行工具并以 role=tool 消息回传续轮（agent loop）。
// 端点不接受工具（未写出任何增量即失败）时自动降级为无工具重试本轮，围栏文本协议兜底。
// 流开始前的失败（未启用/未配置/限流/参数错误）走统一 JSON 错误响应。
func HandleAiAssistantChat(r *ghttp.Request) {
	ctx := r.Context()

	var req aiChatRequest
	if err := r.Parse(&req); err != nil {
		response.ErrorMsg(r, http.StatusBadRequest, "请求参数错误")
		return
	}

	cfg := aiassistant.LoadConfig(ctx)
	if !cfg.Enabled {
		response.ErrorMsg(r, http.StatusForbidden, "AI 助手未启用")
		return
	}
	if err := cfg.Validate(); err != nil {
		response.ErrorMsg(r, http.StatusBadRequest, err.Error())
		return
	}

	msgs, err := aiassistant.SanitizeAndBuild(cfg, req.Messages, aiassistant.FilterSkillKeys(req.Skills))
	if err != nil {
		response.ErrorMsg(r, http.StatusBadRequest, err.Error())
		return
	}

	adminID := common.GetCtxUserID(ctx)
	if !allowChatRate(ctx, adminID, cfg.RateLimitPerMin) {
		response.ErrorMsg(r, http.StatusTooManyRequests, "AI 助手调用过于频繁，请稍后再试")
		return
	}

	// SSE 头：X-Accel-Buffering 禁用 Nginx 等反代缓冲，否则前端收不到实时流
	r.Response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("X-Accel-Buffering", "no")
	r.Response.Flush()

	writeSSE(r, map[string]any{"type": "start", "model": cfg.Model})

	activeSkills := aiassistant.FilterSkillKeys(req.Skills)
	// 技能激活 = 代码生成模式：给足 max_tokens、压低温度（默认 4096 写不完一个
	// 400 行插件，必然截断）
	maxTokens, temperature := aiassistant.GenerationParams(cfg.MaxTokens, cfg.Temperature, len(activeSkills) > 0)
	client := aiassistant.NewClient(cfg)

	var (
		textBuf    strings.Builder // 全文（兜底提取用）
		codeBuf    strings.Builder // 围栏内代码
		inCode     bool
		codeOpened bool
		fenceLang  string
		wroteAny   bool // 是否已向前端写出过任何增量
	)

	// runStream 跑一次完整流（含收尾冲刷）。wroteAny 由 emit 统一置位：
	// 只有「一个字都没写出去」时，回退重试才是安全且不重复的。
	// tools 非空时模型以 tool_calls 结束，工具事件经 tool_start/tool_delta 透出、
	// 流末按聚合结果逐个发 tool_end（前端以此为准解析执行）。
	runStream := func(mt int, temp float64, tools []aiassistant.ToolDef) (*aiassistant.StreamResult, error) {
		splitter := aiassistant.NewFenceSplitter()
		textBuf.Reset()
		codeBuf.Reset()
		inCode, codeOpened, fenceLang = false, false, ""
		toolSeen := map[int]bool{}

		emit := func(evs []aiassistant.FenceEvent) {
			for _, ev := range evs {
				switch ev.Kind {
				case aiassistant.FenceText:
					wroteAny = true
					writeSSE(r, map[string]any{"type": "delta", "content": ev.Text})
				case aiassistant.FenceCodeStart:
					wroteAny = true
					inCode, codeOpened = true, true
					fenceLang = ev.Lang
					writeSSE(r, map[string]any{"type": "code_start", "lang": ev.Lang})
				case aiassistant.FenceCodeDelta:
					wroteAny = true
					if inCode {
						codeBuf.WriteString(ev.Text)
					}
					writeSSE(r, map[string]any{"type": "code_delta", "content": ev.Text})
				case aiassistant.FenceCodeEnd:
					wroteAny = true
					inCode = false
					// implicit=true：闭栏行是服务端补的，模型没写闭栏 = 输出不完整
					writeSSE(r, map[string]any{"type": "code_end", "implicit": ev.Implicit})
				}
			}
		}

		res, err := client.ChatStream(ctx, msgs, mt, temp, tools, func(delta string) {
			wroteAny = true
			textBuf.WriteString(delta)
			emit(splitter.Feed(delta))
		}, func(reasoning string) {
			// 思考过程不进围栏切分器（它不是正文，也不参与代码提取）
			wroteAny = true
			writeSSE(r, map[string]any{"type": "thinking", "content": reasoning})
		}, func(index int, id, name, argsDelta string) {
			wroteAny = true
			if !toolSeen[index] {
				toolSeen[index] = true
				writeSSE(r, map[string]any{"type": "tool_start", "index": index, "id": id, "name": name})
			}
			if argsDelta != "" {
				writeSSE(r, map[string]any{"type": "tool_delta", "index": index, "text": argsDelta})
			}
		})
		// 流结束：冲刷未完成行 + 隐式闭合（出错也冲刷，保住已生成的部分）
		emit(splitter.Finish())
		// 工具调用聚合完成事件：参数完整原文在此（前端增量拼接仅作进度展示）
		for _, tc := range res.ToolCalls {
			name, args := "", ""
			if tc.Function != nil {
				name = tc.Function.Name
				args = tc.Function.Arguments
			}
			writeSSE(r, map[string]any{"type": "tool_end", "index": tc.Index, "id": tc.ID, "name": name, "arguments": args})
		}
		return res, err
	}

	// js_plugin 技能激活 → 请求携带代码工具集（agent loop 模式）
	var tools []aiassistant.ToolDef
	for _, k := range activeSkills {
		if k == "js_plugin" {
			tools = aiassistant.PluginCodeTools
			break
		}
	}

	res, err := runStream(maxTokens, temperature, tools)
	// 回退一：上游不接受抬高后的 max_tokens（小上限模型/上下文窗口不足）——回退配置值
	// 重试（保留工具模式）。前置条件是还没写出任何增量，否则重试会把内容重复推给前端。
	if err != nil && maxTokens != cfg.MaxTokens && !wroteAny && aiassistant.IsMaxTokensRejected(err) {
		g.Log().Warningf(ctx, "[AiAssistant] max_tokens %d rejected, fallback to %d: %v", maxTokens, cfg.MaxTokens, err)
		writeSSE(r, map[string]any{
			"type":    "notice",
			"message": fmt.Sprintf("上游不接受 %d 的输出上限，已回退到 %d 重试", maxTokens, cfg.MaxTokens),
		})
		maxTokens = cfg.MaxTokens
		res, err = runStream(cfg.MaxTokens, temperature, tools)
	}
	// 回退二：端点不接受工具调用（未写出任何增量即失败）——降级为无工具重试本轮，
	// 围栏文本协议兜底。仅 js_plugin 技能受影响；billing_formula 本就无工具
	if err != nil && tools != nil && !wroteAny && !aiassistant.IsMaxTokensRejected(err) {
		g.Log().Warningf(ctx, "[AiAssistant] tools rejected by endpoint, fallback to fence protocol: %v", err)
		writeSSE(r, map[string]any{
			"type":    "notice",
			"message": "端点不接受工具调用，本轮已回退为文本协议模式",
		})
		res, err = runStream(maxTokens, temperature, nil)
	}

	if err != nil {
		// 客户端已断开时写入静默失败，属正常路径；上游错误按中间件分级惯例记 Warning
		if !errors.Is(err, context.Canceled) {
			g.Log().Warningf(ctx, "[AiAssistant] chat stream failed: %v", err)
		}
		writeSSE(r, map[string]any{"type": "error", "message": err.Error()})
		writeDone(r)
		return
	}

	// 代码校验闭环：模型不守约（未用四反引号围栏）时对全文做兜底提取
	code := codeBuf.String()
	if !codeOpened {
		if fallback, ok := aiassistant.ExtractCodeFallback(textBuf.String()); ok {
			code = fallback
			fenceLang = ""
			writeSSE(r, map[string]any{"type": "code_start", "lang": ""})
			writeSSE(r, map[string]any{"type": "code_delta", "content": code})
			writeSSE(r, map[string]any{"type": "code_end"})
		}
	}
	// 截断标记必须在此刻（兜底提取之后、validate 之前）下发：前端据此给「当前代码段」
	// 打 incomplete 标记，把「让 AI 修复」换成「续写剩余部分」——重写只会撞同一上限。
	if res.Truncated() {
		writeSSE(r, map[string]any{"type": "truncated", "reason": "max_tokens", "limit": maxTokens})
	}

	if code != "" && len(activeSkills) > 0 {
		if s, ok := skills.Get(aiassistant.SkillKeyForFence(fenceLang, activeSkills)); ok {
			report := s.Validate(strings.TrimSuffix(code, "\n"))
			if res.Truncated() {
				// 截断的报告要能自解释根因，否则只有一句「编译失败」误导用户
				report.OK = false
				report.Checks = append([]skills.Check{{
					Name:   "completion",
					Status: "fail",
					Detail: fmt.Sprintf("输出达到 max_tokens 上限（%d），代码被截断，需续写后重新校验", maxTokens),
				}}, report.Checks...)
			}
			writeSSE(r, aiassistant.NewValidateEvent(report))
		}
	}

	writeSSE(r, map[string]any{
		"type":              "usage",
		"model":             cfg.Model,
		"prompt_tokens":     res.Usage.PromptTokens,
		"completion_tokens": res.Usage.CompletionTokens,
		"total_tokens":      res.Usage.TotalTokens,
	})
	writeDone(r)

	// 元数据日志（不落对话内容）：调用留痕 + 成本可见
	g.Log().Infof(ctx, "[AiAssistant] chat done admin=%d model=%s skills=%v tokens=%d",
		adminID, cfg.Model, activeSkills, res.Usage.TotalTokens)
}

// allowChatRate 每管理员固定窗口限流（Redis INCR + EXPIRE）。
// 上游调用花的是平台自己的钱，属必要防护；Redis 不可用时放行（软防护，
// 端点本身已有 JWT 认证兜底），限流为 0 表示不限制。
func allowChatRate(ctx context.Context, adminID int64, limit int) bool {
	if limit <= 0 || adminID <= 0 {
		return true
	}
	redis := rcommon.RedisClientOrNil()
	if redis == nil {
		return true
	}
	key := aiassistant.RateLimitKey(adminID)
	v, err := redis.Do(ctx, "INCR", key)
	if err != nil {
		return true // Redis 故障不阻断对话（认证已兜底）
	}
	if v.Int64() == 1 {
		_, _ = redis.Do(ctx, "EXPIRE", key, 60)
	}
	return v.Int64() <= int64(limit)
}

// writeSSE 写一帧 SSE data 事件并立即冲刷。
func writeSSE(r *ghttp.Request, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	r.Response.Write([]byte("data: " + string(data) + "\n\n"))
	r.Response.Flush()
}

// writeDone 写流结束标记。
func writeDone(r *ghttp.Request) {
	r.Response.Write([]byte("data: [DONE]\n\n"))
	r.Response.Flush()
}
