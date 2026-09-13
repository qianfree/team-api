package minimax

// MiniMax 视频生成模型：H3 系列走 v2 协议（content[] 多模态），Hailuo 2.x 主力型号走 v1 扁平协议。
// 旧型号（T2V/I2V/S2V-01 系列）不进默认清单，但请求传入照样转发（协议分派按模型名，不拦）。
var ModelList = []string{
	"MiniMax-H3",
	"MiniMax-H3-Max",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
}

const channelName = "MiniMaxVideo"

// 分辨率/时长缺省值（与计费估算共用，见 adaptor.go 的 resolveResolution/resolveDuration）
const (
	defaultResolution    = "768P" // H3 与 Hailuo 2.x 的默认档
	defaultResolutionV1  = "720P" // T2V/I2V/S2V 等旧型号的默认档
	defaultDurationSecs  = 6
	defaultTextOnlyRatio = "16:9"
)
