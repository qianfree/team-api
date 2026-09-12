package common

import (
	"context"
	"io"
	"net/http"
)

// Adaptor 是所有 AI 供应商适配器必须实现的接口。
// 每个方法对应 relay 管线中的一个步骤。
type Adaptor interface {
	// Init 使用渠道元数据初始化适配器
	Init(info *RelayInfo)

	// GetRequestURL 构建上游请求的完整 URL
	GetRequestURL(info *RelayInfo) (string, error)

	// SetupRequestHeader 设置上游请求的 HTTP 头
	SetupRequestHeader(header http.Header, info *RelayInfo) error

	// ConvertRequest 将入站请求体转换为供应商原生格式。
	ConvertRequest(ctx context.Context, info *RelayInfo, requestBody []byte) (io.Reader, error)

	// DoRequest 发送 HTTP 请求到上游供应商
	DoRequest(ctx context.Context, info *RelayInfo, requestBody io.Reader) (*http.Response, error)

	// DoResponse 处理上游响应并写回客户端。
	DoResponse(ctx context.Context, resp *http.Response, info *RelayInfo, writer http.ResponseWriter) (*Usage, error)

	// GetChannelName 返回渠道名称（用于日志）
	GetChannelName() string
}

// RequestPostProcessor 供应商私有的「转换后请求体后处理」可选能力。
//
// 为什么需要：relaykit 的转换矩阵按**协议格式**裁决（入站格式 × 有效上游格式），
// 与供应商无关；命中后 Adaptor.ConvertRequest 整个不再被调用。但部分 OpenAI 兼容
// 供应商在格式转换之外还有私有请求适配——参数兼容裁剪（zhipu 的 GLM top_p 裁剪与
// 图片前缀剥离、ali 的 DashScope thinking_budget 剥离）。这些都不属于协议转换，
// 不能塞进 relaykit（它必须与宿主和供应商解耦），但漏掉就会静默降级甚至上游 400。
//
// （历史上 xai/baidu_v2/deepseek 也实现过本接口，承载模型名能力后缀（-search 等）
// 的剥离与思考方言注入；该后缀语法已于 2026-09 移除，其余职责由 relaykit 通用
// 处理覆盖后，三家的实现一并删除。）
//
// 因此由本接口把这段私有后处理接回 relaykit 路径：实现方只处理「格式转换已完成、
// 请求体已是上游主协议」之后的那一段，与自身 ConvertRequest 的尾段共用同一实现。
//
// 契约：
//   - 仅在 relaykit 接管了转换时由 handler 调用；走 adaptor.ConvertRequest 的路径
//     不调用（那条路径的 ConvertRequest 内部已含同一段后处理，重复执行可能双写参数）；
//   - 入参 body 是 relaykit 转换后的上游主协议请求体，返回替换后的完整请求体；
//   - 必须幂等于 relaykit 已做的通用处理（模型名替换、stream_options）：
//     覆盖写同值无害，但不得依赖「字段一定不存在」；
//   - 返回 error 即拒绝请求（hard-fail，与转换失败同等对待）。
type RequestPostProcessor interface {
	PostProcessConvertedRequest(ctx context.Context, info *RelayInfo, body []byte) ([]byte, error)
}
