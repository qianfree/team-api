package jsplugin

import (
	"embed"
	"io/fs"

	"github.com/gogf/gf/v2/errors/gerror"
)

// 内置 JS 插件：go:embed 打进二进制（目录 builtin/{key}/plugin.js）。
// 启动时由 Registry.Bootstrap 幂等 seed/refresh 到 js_plugin_versions（source=builtin）：
//   - 同 key+version 的 upload 行不被触碰（上传优先）
//   - builtin 行源码变化（版本升级）时刷新，enabled/active 状态保留在原行
//   - 新版本号以 inactive 入库，由管理员显式激活（升级不改变运行行为）
//
// 内置行不可删除；删除激活中的上传版本后，同 key 回落激活最新内置版本。
//
//go:embed all:builtin
var builtinFS embed.FS

// TemplateSource 返回插件作者起步模板：golden 参考实现的源码全文（管理端
// 模板下载端点用，作者抄它起步、删掉不需要的钩子）。embed 静态资源读取失败
// 属编译期问题（该文件随二进制分发），此时返回空串由调用方按空源码呈现。
func TemplateSource() string {
	data, err := fs.ReadFile(builtinFS, "builtin/golden/plugin.js")
	if err != nil {
		return ""
	}
	return string(data)
}

// compileBuiltins 编译全部内置插件（启动 seed 与单测共用）。
// 单个插件编译失败返回错误——内置插件随版本发布，编译失败属发布缺陷，
// 启动路径只跳过并记日志（不阻断服务），单测路径直接失败。
func compileBuiltins(engine Engine) ([]*CompiledPlugin, error) {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return nil, gerror.Wrap(err, "读取内置插件目录失败")
	}
	var out []*CompiledPlugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		key := entry.Name()
		source, err := fs.ReadFile(builtinFS, "builtin/"+key+"/plugin.js")
		if err != nil {
			return nil, gerror.Wrapf(err, "内置插件 %s 缺少 plugin.js", key)
		}
		compiled, err := engine.Compile(string(source))
		if err != nil {
			return nil, gerror.Wrapf(err, "内置插件 %s 编译失败", key)
		}
		// 目录名必须与 manifest.key 一致：seed 以 compiled.Key 落库，目录名仅用于
		// 定位文件——不一致时（复制目录忘改 key）同一份源码会以两个 key 重复入库
		if compiled.Key != key {
			return nil, gerror.Newf("内置插件目录 %s 的 manifest.key 为 %q，两者必须一致", key, compiled.Key)
		}
		out = append(out, compiled)
	}
	return out, nil
}
