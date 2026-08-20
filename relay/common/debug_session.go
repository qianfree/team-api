package common

import (
	"context"
	"net/http"

	"github.com/qianfree/team-api/relay/constant"
)

// DebugSession 渠道调试日志会话：挂载于 RelayContext.Debug，整请求存活。
// 重试循环中首个「调试开关开启」的渠道尝试触发创建：捕获段1（客户端请求）并包装段4
// 的客户端响应 writer（保持到请求结束，中间失败尝试不向客户端写字节，累积字节即最终响应）。
// 中间失败尝试经 DebugAttempt.Submit 立即落库（is_final=false）；最终尝试经 MarkFinal
// 挂起，由外层 handler 在响应写完后调用 FinalizeAndSubmit 补段4并统一提交（幂等）。
type DebugSession struct {
	requestID string
	tenantID  int64
	userID    int64
	apiKeyID  int64
	path      string

	clientReqHeaders map[string]string // 段1（脱敏后）
	clientReqBody    []byte            // 段1 完整请求体

	clientWriter *DebugClientWriter // 段4 捕获 writer

	pending   *DebugAttempt // MarkFinal 挂起的最终尝试
	finalized bool
}

// NewDebugSession 创建调试会话
func NewDebugSession(requestID string, tenantID, userID, apiKeyID int64, path string) *DebugSession {
	return &DebugSession{
		requestID: requestID,
		tenantID:  tenantID,
		userID:    userID,
		apiKeyID:  apiKeyID,
		path:      path,
	}
}

// CaptureClientRequest 捕获段1：客户端请求头（脱敏）+ 完整请求体
func (s *DebugSession) CaptureClientRequest(headers http.Header, body []byte) {
	s.clientReqHeaders = MaskHeaders(headers)
	s.clientReqBody = body
}

// SetClientWriter 回填段4捕获 writer（由 RelayHandler 包装 rc.Writer 后调用）
func (s *DebugSession) SetClientWriter(w *DebugClientWriter) {
	s.clientWriter = w
}

// BeginAttempt 为当前渠道尝试创建捕获上下文（每轮重试一次）
func (s *DebugSession) BeginAttempt(selection *ChannelSelection, modelName, relayModeStr string, isStream bool, retryIndex int) *DebugAttempt {
	return &DebugAttempt{
		session:       s,
		Capture:       &DebugAttemptCapture{},
		channelID:     selection.ChannelID,
		channelName:   selection.ChannelName,
		channelType:   selection.ChannelType,
		modelName:     modelName,
		upstreamModel: selection.UpstreamModelName,
		relayMode:     relayModeStr,
		isStream:      isStream,
		retryIndex:    retryIndex,
	}
}

// DebugAttempt 单次尝试的调试上下文：会话引用 + 渠道元数据 + 传输层捕获器。
// Capture 经 WithDebugAttempt 挂入 ctx，由 DebugRoundTripper 填充段2/段3。
type DebugAttempt struct {
	session *DebugSession
	Capture *DebugAttemptCapture

	channelID     int64
	channelName   string
	channelType   int
	modelName     string
	upstreamModel string
	relayMode     string
	isStream      bool
	retryIndex    int
	errText       string
	conversion    *DebugConversion // 协议转换信息（CaptureProtocol 快照）
}

// CaptureProtocol 快照协议转换信息（RelayHandler 在 buildRelayInfo 之后调用一次）。
// 上游协议按桥接标志与渠道类型推导：直传 > Responses 桥接 > Responses 直连 > 渠道原生协议。
func (a *DebugAttempt) CaptureProtocol(info *RelayInfo) {
	if a == nil || info == nil {
		return
	}
	conv := &DebugConversion{
		ClientFormat: string(info.GetOriginalClientFormat()),
	}
	switch {
	case info.ChannelMeta != nil && info.ChannelMeta.Settings.PassThroughBodyEnabled:
		conv.UpstreamFormat = conv.ClientFormat
		conv.Bridge = "pass_through"
	case info.UseResponsesAPI:
		conv.UpstreamFormat = string(constant.RelayFormatResponses)
		conv.Bridge = "responses_api"
	case info.ChannelMeta != nil && info.ChannelMeta.UpstreamSpeaksResponses():
		conv.UpstreamFormat = string(constant.RelayFormatResponses)
		conv.Bridge = "responses_direct"
	default:
		conv.UpstreamFormat = nativeFormatOfProvider(a.channelType)
	}
	for _, f := range info.ConversionChain() {
		conv.Chain = append(conv.Chain, string(f))
	}
	// relaykit 链未覆盖（直传/同构/adaptor 手写转换）且两端协议不同时，兜底记录两端
	if len(conv.Chain) == 0 && conv.UpstreamFormat != conv.ClientFormat {
		conv.Chain = []string{conv.ClientFormat, conv.UpstreamFormat}
	}
	a.conversion = conv
}

// nativeFormatOfProvider 渠道类型 → 上游原生协议（展示用途）。
// openai 兼容系渠道占绝大多数，默认 openai；anthropic / gemini 系例外。
func nativeFormatOfProvider(providerType int) string {
	switch constant.ProviderType(providerType) {
	case constant.ProviderClaude:
		return string(constant.RelayFormatClaude)
	case constant.ProviderGemini, constant.ProviderVertex:
		return string(constant.RelayFormatGemini)
	default:
		return string(constant.RelayFormatOpenAI)
	}
}

// Submit 中间失败尝试：立即经钩子异步落库（is_final=false，段4为空——该尝试未向客户端写出字节）
func (a *DebugAttempt) Submit(err error) {
	if a == nil {
		return
	}
	a.errText = debugErrText(err)
	if hook := SubmitDebugLog; hook != nil {
		hook(context.Background(), a.buildRecord(false, 0, 0))
	}
}

// MarkFinal 标记为最终尝试（成功/终止/流中断）：挂起等待外层 FinalizeAndSubmit 补段4后提交
func (a *DebugAttempt) MarkFinal(err error) {
	if a == nil {
		return
	}
	a.errText = debugErrText(err)
	a.session.pending = a
}

// FinalizeAndSubmit 补段4并提交最终尝试记录。由外层 handler 在客户端响应写完后调用
// （recordAudit 收尾处），幂等：重复调用或无挂起尝试时为 no-op。
func (s *DebugSession) FinalizeAndSubmit(totalLatencyMs, firstTokenMs int64) {
	if s == nil || s.finalized {
		return
	}
	s.finalized = true
	a := s.pending
	if a == nil {
		return
	}
	if hook := SubmitDebugLog; hook != nil {
		hook(context.Background(), a.buildRecord(true, totalLatencyMs, firstTokenMs))
	}
}

// buildRecord 汇总四段与元数据构建落库记录。isFinal=true 时从 clientWriter 补段4。
func (a *DebugAttempt) buildRecord(isFinal bool, totalLatencyMs, firstTokenMs int64) *DebugLogRecord {
	s := a.session
	snap := a.Capture.snapshot()

	rec := &DebugLogRecord{
		ChannelID:     a.channelID,
		ChannelName:   a.channelName,
		ChannelType:   a.channelType,
		RequestID:     s.requestID,
		TenantID:      s.tenantID,
		UserID:        s.userID,
		ApiKeyID:      s.apiKeyID,
		ModelName:     a.modelName,
		UpstreamModel: a.upstreamModel,
		RelayMode:     a.relayMode,
		InboundPath:   s.path,
		UpstreamURL:   snap.URL,
		IsStream:      a.isStream,
		RetryIndex:    a.retryIndex,
		IsFinal:       isFinal,

		UpstreamStatusCode: snap.Status,
		Error:              a.errText,

		// 段1（会话级，全部尝试共享）
		ClientReqHeaders: s.clientReqHeaders,
		ClientReqBody:    s.clientReqBody,

		// 段2/段3（传输层捕获）
		UpstreamReqHeaders:  snap.ReqHeaders,
		UpstreamReqBody:     snap.ReqBody,
		UpstreamRespHeaders: snap.RespHeaders,
		UpstreamRespBody:    snap.RespBody,

		UpstreamLatencyMs: snap.LatencyMs,

		Conversion: a.conversion,
	}

	if snap.Err != "" && rec.Error == "" {
		// 传输层错误（连接失败等）未被上层 err 覆盖时兜底保留
		rec.Error = snap.Err
	}

	if isFinal && s.clientWriter != nil {
		// 段4：客户端响应
		rec.ClientRespHeaders = s.clientWriter.HeaderSnapshot()
		rec.ClientRespBody = s.clientWriter.Bytes()
		rec.ClientStatusCode = s.clientWriter.StatusCode()
		rec.TotalLatencyMs = totalLatencyMs
		rec.FirstTokenMs = firstTokenMs
	}
	return rec
}

func debugErrText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
