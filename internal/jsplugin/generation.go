package jsplugin

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// ActivePlugin 当前激活且可服务的插件运行时视图。
type ActivePlugin struct {
	Key      string
	Version  string
	Manifest *contract.Manifest
	Compiled *CompiledPlugin
	Pool     *PluginPool

	// hookCache 钩子存在性缓存：materialize 时一次性探测构建，随 generation
	// 原子发布后只读（绝不懒写回——跨 goroutine 共享 struct 的懒写是 data race）。
	// 存在的意义：Pool.HasHook 在池全忙时会绕过并发闸新建完整 runtime，
	// 分发器每个请求都探测一次，高并发下是内存峰值放大器；缓存未命中回落 Pool.HasHook。
	hookCache map[string]bool
}

// HasHook 报告插件是否导出钩子（缓存优先，miss 回落池探测）。
func (p *ActivePlugin) HasHook(hook string) bool {
	if p.hookCache != nil {
		if has, ok := p.hookCache[hook]; ok {
			return has
		}
	}
	return p.Pool.HasHook(hook)
}

// buildHookCache 探测插件导出的钩子集合（候选名见 hookCandidates）。
// 探测失败返回 nil（调用方回落 Pool.HasHook）——不能让探测失败 fail 整个
// materialize：rebuildGeneration 是全有或全无，不能为观测性优化加重热更新失败面。
func buildHookCache(engine Engine, compiled *CompiledPlugin, opts InstanceOptions) map[string]bool {
	candidates := hookCandidates(compiled.Manifest)
	if len(candidates) == 0 {
		return nil
	}
	inst, err := engine.NewInstance(compiled, opts)
	if err != nil {
		return nil
	}
	defer inst.Close()
	cache := make(map[string]bool, len(candidates))
	for _, name := range candidates {
		cache[name] = inst.HasHook(name)
	}
	return cache
}

// hookCandidates 汇总需要缓存存在性的钩子名：路由声明的 decode/render 别名 +
// 按路由类型的渲染缺省名 + 标准钩子名（别名必须入集——否则别名插件全部 miss，
// 缓存优化对其失效，只剩正确性兜底）。
func hookCandidates(m *contract.Manifest) []string {
	seen := make(map[string]struct{}, 16)
	add := func(name string) {
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, name := range []string{
		contract.HookNameDecode, contract.HookNameRenderSubmit, contract.HookNameRenderTask,
		"renderTasks", "estimateUsage",
		"buildSubmitRequest", "parseSubmitResponse", "buildQueryRequest", "parseQueryResponse",
	} {
		add(name)
	}
	// 标准端点解码钩子（standardVideos 声明）：提交期每请求探测一次，进缓存
	add(m.StandardVideos)
	for _, route := range m.Routes {
		add(route.Decode)
		add(route.Render)
		switch route.Type {
		case contract.RouteTypeSubmit:
			add(route.ResolveRender(contract.HookNameRenderSubmit))
		case contract.RouteTypeQuery:
			add(route.ResolveRender(contract.HookNameRenderTask))
		case contract.RouteTypeDynamic:
			// dynamic 一型两用：查询侧缺省 renderTasks，提交侧缺省 renderSubmit，
			// 查询单任务渲染（task 路由复用）缺省 renderTask——三种都探测
			add(route.ResolveRender("renderTasks"))
			add(route.ResolveRender(contract.HookNameRenderSubmit))
			add(route.ResolveRender(contract.HookNameRenderTask))
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}

// genRoute generation 内的一条已索引路由。
type genRoute struct {
	method   string
	segments []string // ":name" 段为参数
	params   int
	plugin   *ActivePlugin
	route    contract.Route
}

// Generation 不可变路由快照：启用/禁用/换版本时整体重建、原子指针发布，读侧无锁。
type Generation struct {
	plugins   map[string]*ActivePlugin
	routes    []genRoute        // 参数少（字面量多）者优先
	models    map[string]string // 模型名(小写) → pluginKey（冲突预检与渠道选择辅助）
	createdAt time.Time
}

// Match 按 method+path 匹配插件路由，返回插件、路由声明与路径参数。
func (g *Generation) Match(method, path string) (*ActivePlugin, contract.Route, map[string]string, bool) {
	shape := contract.NormalizePathShape(path)
	var segs []string
	if shape != "/" {
		segs = strings.Split(strings.TrimPrefix(shape, "/"), "/")
	}
	for _, gr := range g.routes {
		if gr.method != method || len(gr.segments) != len(segs) {
			continue
		}
		params := make(map[string]string, gr.params)
		matched := true
		for i, seg := range gr.segments {
			if strings.HasPrefix(seg, ":") {
				params[seg[1:]] = segs[i]
				continue
			}
			if seg != segs[i] {
				matched = false
				break
			}
		}
		if matched {
			return gr.plugin, gr.route, params, true
		}
	}
	return nil, contract.Route{}, nil, false
}

// Plugin 按 key 取激活插件。
func (g *Generation) Plugin(key string) (*ActivePlugin, bool) {
	p, ok := g.plugins[key]
	return p, ok
}

// Plugins 返回全部激活插件（观测/列表用，不保证顺序）。
func (g *Generation) Plugins() []*ActivePlugin {
	out := make([]*ActivePlugin, 0, len(g.plugins))
	for _, p := range g.plugins {
		out = append(out, p)
	}
	return out
}

// FindPluginByModel 按模型名（大小写不敏感）查声明它的插件（冲突预检用）。
func (g *Generation) FindPluginByModel(model string) (string, bool) {
	key, ok := g.models[strings.ToLower(model)]
	return key, ok
}

// ---------------------------------------------------------------------------
// 原子发布
// ---------------------------------------------------------------------------

var currentGeneration atomic.Pointer[Generation]

func init() {
	currentGeneration.Store(&Generation{
		plugins: map[string]*ActivePlugin{},
		models:  map[string]string{},
	})
}

// CurrentGeneration 返回当前生效的 generation（永不为 nil）。
func CurrentGeneration() *Generation {
	return currentGeneration.Load()
}

// publishGeneration 发布新 generation（仅 registry 重建路径调用）。
func publishGeneration(g *Generation) {
	currentGeneration.Store(g)
}

// buildGeneration 由激活插件集合构建 generation。
func buildGeneration(plugins []*ActivePlugin) *Generation {
	g := &Generation{
		plugins:   make(map[string]*ActivePlugin, len(plugins)),
		models:    make(map[string]string, len(plugins)),
		createdAt: time.Now(),
	}
	for _, p := range plugins {
		g.plugins[p.Key] = p
		for _, m := range p.Manifest.Models {
			lower := strings.ToLower(m)
			// 兜底告警：模型同时出现在多个插件的有效清单（配置扩展的保存预检竞态或
			// 绕过），回落解析按后装载者。正常路径冲突在激活/配置保存时已被拒绝。
			if prev, dup := g.models[lower]; dup && prev != p.Key {
				glog.Warningf(context.Background(),
					"jsplugin: 模型 %s 同时被插件 %s 与 %s 声明（检查 modelsFromConfig 配置），模型回落解析按后者", m, prev, p.Key)
			}
			g.models[lower] = p.Key
		}
		for _, route := range p.Manifest.Routes {
			shape := contract.NormalizePathShape(route.Path)
			var segs []string
			if shape != "/" {
				segs = strings.Split(strings.TrimPrefix(shape, "/"), "/")
			}
			params := 0
			for _, s := range segs {
				if strings.HasPrefix(s, ":") {
					params++
				}
			}
			g.routes = append(g.routes, genRoute{
				method:   route.Method,
				segments: segs,
				params:   params,
				plugin:   p,
				route:    route,
			})
		}
	}
	// 稳定排序：参数少的（更具体的）优先；同级保持声明顺序（sort.Stable）
	stableSortRoutes(g.routes)
	return g
}

func stableSortRoutes(routes []genRoute) {
	for i := 1; i < len(routes); i++ {
		for j := i; j > 0 && routes[j].params < routes[j-1].params; j-- {
			routes[j], routes[j-1] = routes[j-1], routes[j]
		}
	}
}
