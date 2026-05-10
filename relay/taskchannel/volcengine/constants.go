package volcengine

// Seedance 视频生成模型列表
var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
}

const channelName = "VolcengineVideo"

// videoInputRatioMap 视频输入折扣比率（含视频单价 / 不含视频单价）
var videoInputRatioMap = map[string]float64{
	"doubao-seedance-2-0-260128":      28.0 / 46.0,
	"doubao-seedance-2-0-fast-260128": 22.0 / 37.0,
}

func getVideoInputRatio(modelName string) (float64, bool) {
	r, ok := videoInputRatioMap[modelName]
	return r, ok
}
