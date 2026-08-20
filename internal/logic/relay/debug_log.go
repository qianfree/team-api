package relay

import (
	"context"
	"encoding/json"

	relaycommon "github.com/qianfree/team-api/relay/common"

	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"
)

// InitDebugLogRecorder 注入渠道调试日志提交钩子（进程启动时调用一次）。
// relay 层通过包级钩子解耦对 internal/logic 的依赖（仿 billing webhook 钩子模式）。
func InitDebugLogRecorder() {
	relaycommon.SubmitDebugLog = submitDebugLog
}

// submitDebugLog 将四段报文记录映射为 chn_debug_logs 行并投递批量写入器。
// body 经 EncodeBody 处理二进制（base64），headers 序列化为 JSON 串写入 JSONB 列。
func submitDebugLog(_ context.Context, record *relaycommon.DebugLogRecord) {
	if record == nil || common.DefaultChannelDebugLogWriter == nil {
		return
	}

	clientReqBody, clientReqEnc := relaycommon.EncodeBody(record.ClientReqBody)
	upstreamReqBody, upstreamReqEnc := relaycommon.EncodeBody(record.UpstreamReqBody)
	upstreamRespBody, upstreamRespEnc := relaycommon.EncodeBody(record.UpstreamRespBody)
	clientRespBody, clientRespEnc := relaycommon.EncodeBody(record.ClientRespBody)

	insert := do.ChnDebugLogs{
		ChannelId:            record.ChannelID,
		ChannelName:          record.ChannelName,
		ChannelType:          record.ChannelType,
		RequestId:            record.RequestID,
		TenantId:             record.TenantID,
		UserId:               record.UserID,
		ApiKeyId:             record.ApiKeyID,
		ModelName:            record.ModelName,
		UpstreamModel:        record.UpstreamModel,
		RelayMode:            record.RelayMode,
		InboundPath:          record.InboundPath,
		UpstreamUrl:          record.UpstreamURL,
		IsStream:             record.IsStream,
		RetryIndex:           record.RetryIndex,
		IsFinal:              record.IsFinal,
		ClientStatusCode:     record.ClientStatusCode,
		Error:                record.Error,
		ClientReqHeaders:     headersJSON(record.ClientReqHeaders),
		ClientReqBody:        clientReqBody,
		ClientReqEncoding:    clientReqEnc,
		UpstreamReqHeaders:   headersJSON(record.UpstreamReqHeaders),
		UpstreamReqBody:      upstreamReqBody,
		UpstreamReqEncoding:  upstreamReqEnc,
		UpstreamRespHeaders:  headersJSON(record.UpstreamRespHeaders),
		UpstreamRespBody:     upstreamRespBody,
		UpstreamRespEncoding: upstreamRespEnc,
		ClientRespHeaders:    headersJSON(record.ClientRespHeaders),
		ClientRespBody:       clientRespBody,
		ClientRespEncoding:   clientRespEnc,
		UpstreamLatencyMs:    record.UpstreamLatencyMs,
		TotalLatencyMs:       record.TotalLatencyMs,
		FirstTokenMs:         record.FirstTokenMs,
		ClientReqBytes:       int64(len(record.ClientReqBody)),
		UpstreamReqBytes:     int64(len(record.UpstreamReqBody)),
		UpstreamRespBytes:    int64(len(record.UpstreamRespBody)),
		ClientRespBytes:      int64(len(record.ClientRespBody)),
	}
	// 0 表示未发起请求/连接失败，落库为 NULL 以区分真实状态码 0 之外的语义
	if record.UpstreamStatusCode > 0 {
		insert.UpstreamStatusCode = record.UpstreamStatusCode
	}
	// 协议转换信息 → JSONB
	if record.Conversion != nil {
		if b, err := json.Marshal(record.Conversion); err == nil {
			insert.Conversion = string(b)
		}
	}

	common.DefaultChannelDebugLogWriter.Submit(insert)
}

// headersJSON headers map 序列化为 JSON 串（JSONB 列）；nil 时写 "null"（与审计日志同口径）
func headersJSON(h map[string]string) any {
	if h == nil {
		return "null"
	}
	b, err := json.Marshal(h)
	if err != nil {
		return "null"
	}
	return string(b)
}
