package jsplugin

import (
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
)

// PublishForTest 测试辅助入口（非生产路径）：从源码直接构建并发布 generation，
// 绕过 DB 生命周期（Upload/Activate/Enable 均需数据库）。返回清理函数恢复原 generation
// 并关闭插件池。供 jsbridge 单测与集成 E2E 使用。
func PublishForTest(sources ...string) (func(), error) {
	engine := NewGojaEngine()
	var plugins []*ActivePlugin
	for _, src := range sources {
		compiled, err := engine.Compile(src)
		if err != nil {
			return nil, err
		}
		if _, exists := lookupTestPlugin(plugins, compiled.Key); exists {
			return nil, gerror.Newf("duplicate plugin key %q", compiled.Key)
		}
		opts := InstanceOptions{AllowedHosts: compiled.Manifest.AllowedHosts}
		plugins = append(plugins, &ActivePlugin{
			Key:       compiled.Key,
			Version:   compiled.Version,
			Manifest:  compiled.Manifest,
			Compiled:  compiled,
			Pool:      NewPluginPool(engine, compiled, opts),
			hookCache: buildHookCache(engine, compiled, opts),
		})
	}
	old := CurrentGeneration()
	publishGeneration(buildGeneration(plugins))
	return func() {
		publishGeneration(old)
		for _, p := range plugins {
			p.Pool.Close()
		}
	}, nil
}

func lookupTestPlugin(plugins []*ActivePlugin, key string) (*ActivePlugin, bool) {
	for _, p := range plugins {
		if p.Key == key {
			return p, true
		}
	}
	return nil, false
}

// readBuiltin 读取内置插件源码（go:embed 的 builtin/{key}/plugin.js），迁移测试用。
// 2bffa46f 清理旧迁移测试时误删了共享 helper，残留的 alibaba/doubao/hailuo 测试引用悬空导致包测试编译失败。
func readBuiltin(t *testing.T, key string) string {
	t.Helper()
	src, err := builtinFS.ReadFile("builtin/" + key + "/plugin.js")
	if err != nil {
		t.Fatalf("读取内置插件 %s 失败: %v", key, err)
	}
	return string(src)
}
