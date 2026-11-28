package jsplugin

import (
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// modelsFromConfig 扩展模型的运行时合并与冲突防线。
//
// 有效清单 = meta.models（随版本发布的基线）∪ 配置扩展（部署数据，管理后台改）。
// 合并发生在 materialize：写入 ActivePlugin 的 Manifest 内存视图（副本），DB 版本行
// 与编译缓存（CompiledPlugin.Manifest）永不改写。清单是插件支持的上游模型，
// 跨插件可重名（平台模型 → 插件的归属由渠道 settings.plugin_key 决定），无独占预检。

// applyConfigModels 把配置扩展并入 manifest 的副本视图。无扩展时原样返回原指针；
// 有扩展时返回浅拷贝（仅 Models 字段被替换），调用方以返回值为 ActivePlugin.Manifest。
func applyConfigModels(m *contract.Manifest, configValues map[string]any) (*contract.Manifest, error) {
	if m.ModelsFromConfig == "" {
		return m, nil
	}
	extras, err := contract.ParseConfigModels(configValues[m.ModelsFromConfig])
	if err != nil {
		return nil, err
	}
	if len(extras) == 0 {
		return m, nil
	}
	view := *m
	view.Models = mergeModelNames(m.Models, extras)
	return &view, nil
}

// mergeModelNames 基线 ∪ 扩展（保序去重，基线在前）。
func mergeModelNames(base, extras []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extras))
	out := make([]string, 0, len(base)+len(extras))
	for _, name := range base {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, name := range extras {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// effectiveModelSet 计算插件有效模型集（静态 ∪ 配置扩展）。配置值缺失/解析失败时
// 降级静态集（调用方语境：预检用已存值 + 待写值合成，坏值已在保存入口拦下）。
func effectiveModelSet(m *contract.Manifest, configValues map[string]any) ([]string, error) {
	if m.ModelsFromConfig == "" {
		return m.Models, nil
	}
	extras, err := contract.ParseConfigModels(configValues[m.ModelsFromConfig])
	if err != nil {
		return nil, err
	}
	return mergeModelNames(m.Models, extras), nil
}
