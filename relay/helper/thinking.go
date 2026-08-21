package helper

import (
	"strings"

	"github.com/qianfree/team-api/relaykit/relayconvert/reasoning"
)

// ThinkingInfo 从模型名解析出的 thinking 配置
type ThinkingInfo struct {
	BaseModel    string // 去除后缀的基础模型名
	IsThinking   bool   // 是否有 -thinking 后缀
	EffortLevel  string // effort 级别：low/medium/high/xhigh/max/minimal
	IsNoThinking bool   // 是否有 -nothinking 后缀
}

// ParseThinkingSuffix 从模型名解析 thinking 和 effort 后缀。
// effort 后缀匹配委托 relaykit/reasoning（单一权威定义），路由层与转换器层的
// 后缀集合天然同源；-thinking/-nothinking/-none 是路由层虚拟模型语义，不进 relaykit。
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

	if base, effort, ok := reasoning.TrimEffortSuffix(modelName); ok {
		info.EffortLevel = effort
		info.BaseModel = base
		return info
	}

	info.BaseModel = modelName
	return info
}
