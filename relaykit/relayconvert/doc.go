// Package relayconvert —— 协议转换器注册表与执行入口。
//
// # 注册表拓扑
//
//   - request_registry.go：请求侧转换器（请求方向 From→To）
//   - response_registry.go：非流式响应侧转换器（ResponseConverterFunc，整响应 any）
//   - stream_registry.go：流式响应转换器（StreamConverterFunc，io.Reader + chunk 回调，
//     与整响应签名不兼容，故独立建表）
//   - text_converter_registry.go：Req+Resp 成对 spec 的外观层，注册时拆分进上两张表
//   - dispatch.go：ExecuteRequestConverter——链式 spec 的逐跳执行引擎
//
// # 方向约定（读 ID/From/To 前必读）
//
// TextConverterSpec 的 From/To 表达**请求方向**（客户端格式→上游格式）；
// 其 Resp 侧实际转换的是**反方向**（上游响应→客户端格式）。因此 spec ID
// "claude_messages_to_openai_chat" 的 Resp 侧实现是 OpenAI 响应→Claude 响应——
// 这是注册表的方向语义约定，不是实现写反了。
//
// stream_registry.go 中的独立流式转换器不走 spec 成对结构，其 ID/From/To 表达
// **自身真实流方向**（上游 SSE→客户端 SSE），与 spec 方向约定无关。
//
// # 链式组合
//
// 跨原生方向（如 claude→gemini）经 openai chat 中间格式两跳组合：请求侧
// StepConverters 声明逐跳 ID；响应侧 Resp.Convert 直接组合两跳的非流式实现；
// 流式侧经 chainStreamConverters 以 io.Pipe 串联两跳（见 register 包）。
//
// # 接线位置约束（宿主侧约定，在此备案）
//
// claude/gemini/responses 入站 → openai(chat) 上游方向的接管只发生在宿主共享函数
// relay/channel/openai.ConvertToOpenAI 内部（经 relay/relaykit_bridge），严禁在
// relay/handler 的路由层另开入口——那会跳过 20+ 个 openai 兼容 adaptor 的定制后处理。
package relayconvert
