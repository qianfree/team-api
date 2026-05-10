package helper

import "strings"

// ThinkingInfo 从模型名解析出的 thinking 配置
type ThinkingInfo struct {
	BaseModel    string // 去除后缀的基础模型名
	IsThinking   bool   // 是否有 -thinking 后缀
	EffortLevel  string // effort 级别：low/medium/high/xhigh/max/minimal
	IsNoThinking bool   // 是否有 -nothinking 后缀
}

// 后缀列表，按长度降序排列以避免误匹配（如 -minimal 优先于 -min）
var effortSuffixes = []struct {
	suffix string
	level  string
}{
	{"-minimal", "minimal"},
	{"-medium", "medium"},
	{"-xhigh", "xhigh"},
	{"-high", "high"},
	{"-low", "low"},
	{"-max", "max"},
}

// ParseThinkingSuffix 从模型名解析 thinking 和 effort 后缀
func ParseThinkingSuffix(modelName string) ThinkingInfo {
	info := ThinkingInfo{}

	if strings.HasSuffix(modelName, "-thinking") {
		info.IsThinking = true
		info.BaseModel = modelName[:len(modelName)-len("-thinking")]
		return info
	}

	if strings.HasSuffix(modelName, "-nothinking") {
		info.IsNoThinking = true
		info.BaseModel = modelName[:len(modelName)-len("-nothinking")]
		return info
	}

	if strings.HasSuffix(modelName, "-none") {
		info.IsNoThinking = true
		info.BaseModel = modelName[:len(modelName)-len("-none")]
		return info
	}

	for _, s := range effortSuffixes {
		if strings.HasSuffix(modelName, s.suffix) {
			info.EffortLevel = s.level
			info.BaseModel = modelName[:len(modelName)-len(s.suffix)]
			return info
		}
	}

	info.BaseModel = modelName
	return info
}
