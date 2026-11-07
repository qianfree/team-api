package jsplugin

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// ActivePlugin 当前激活且可服务的插件运行时视图。
type ActivePlugin struct {
	Key      string
	Version  string
	Manifest *contract.Manifest
	Compiled *CompiledPlugin
	Pool     *PluginPool
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
			g.models[strings.ToLower(m)] = p.Key
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
