package admin

import (
	"context"
	"encoding/json"
	"errors"
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
//	data: {"type":"delta","content":"..."}        正文增量（markdown 渲染）
//	data: {"type":"code_start","skill":"js_plugin","lang":"js"}
//	data: {"type":"code_delta","content":"..."}   代码增量（专用代码卡片渲染，不经 markdown）
//	data: {"type":"code_end"}
//	data: {"type":"validate", ...}                技能校验报告（流末，提取到代码且有激活技能时）
//	data: {"type":"usage","model":"...","prompt_tokens":N,...}
//	data: {"type":"error","message":"..."}        流中错误（此后紧跟 [DONE]）
//	data: [DONE]
//
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

	splitter := aiassistant.NewFenceSplitter()
	var (
		textBuf    strings.Builder // 全文（兜底提取用）
		codeBuf    strings.Builder // 围栏内代码
		inCode     bool
		codeOpened bool
		fenceLang  string
	)
	activeSkills := aiassistant.FilterSkillKeys(req.Skills)

	usage, err := aiassistant.NewClient(cfg).ChatStream(ctx, msgs, cfg.MaxTokens, cfg.Temperature, func(delta string) {
		textBuf.WriteString(delta)
		for _, ev := range splitter.Feed(delta) {
			switch ev.Kind {
			case aiassistant.FenceText:
				writeSSE(r, map[string]any{"type": "delta", "content": ev.Text})
			case aiassistant.FenceCodeStart:
				inCode, codeOpened = true, true
				fenceLang = ev.Lang
				writeSSE(r, map[string]any{"type": "code_start", "lang": ev.Lang})
			case aiassistant.FenceCodeDelta:
				if inCode {
					codeBuf.WriteString(ev.Text)
				}
				writeSSE(r, map[string]any{"type": "code_delta", "content": ev.Text})
			case aiassistant.FenceCodeEnd:
				inCode = false
				writeSSE(r, map[string]any{"type": "code_end"})
			}
		}
	})

	// 流结束：冲刷未完成行 + 隐式闭合
	for _, ev := range splitter.Finish() {
		switch ev.Kind {
		case aiassistant.FenceText:
			writeSSE(r, map[string]any{"type": "delta", "content": ev.Text})
		case aiassistant.FenceCodeDelta:
			if inCode {
				codeBuf.WriteString(ev.Text)
			}
			writeSSE(r, map[string]any{"type": "code_delta", "content": ev.Text})
		case aiassistant.FenceCodeEnd:
			inCode = false
			writeSSE(r, map[string]any{"type": "code_end"})
		}
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
	if code != "" && len(activeSkills) > 0 {
		if s, ok := skills.Get(aiassistant.SkillKeyForFence(fenceLang, activeSkills)); ok {
			report := s.Validate(strings.TrimSuffix(code, "\n"))
			writeSSE(r, report)
		}
	}

	writeSSE(r, map[string]any{
		"type":              "usage",
		"model":             cfg.Model,
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
	})
	writeDone(r)

	// 元数据日志（不落对话内容）：调用留痕 + 成本可见
	g.Log().Infof(ctx, "[AiAssistant] chat done admin=%d model=%s skills=%v tokens=%d",
		adminID, cfg.Model, activeSkills, usage.TotalTokens)
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
