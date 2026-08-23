package oai_gemini

import "time"

// NowFunc 时间源（默认 time.Now）。gemini→openai 方向转换器生成响应时间戳与兜底 ID 时使用，
// golden 测试替换为固定时钟以保证输出确定性。
var NowFunc = time.Now
