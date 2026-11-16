package jsplugin

import (
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func testPlugin(key string, models []string, routes ...contract.Route) *ActivePlugin {
	return &ActivePlugin{
		Key: key,
		Manifest: &contract.Manifest{
			Key: key, Models: models, Routes: routes,
		},
	}
}

func TestGenerationMatchParams(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("alpha", []string{"alpha_music"},
			contract.Route{Method: "POST", Path: "/submit/:action", Type: contract.RouteTypeSubmit},
			contract.Route{Method: "GET", Path: "/fetch/:task_id", Type: contract.RouteTypeQuery},
		),
	})

	plugin, route, params, ok := gen.Match("alpha", "POST", "/submit/music")
	if !ok || plugin.Key != "alpha" || route.Type != contract.RouteTypeSubmit {
		t.Fatalf("提交路由匹配失败: ok=%v plugin=%v", ok, plugin)
	}
	if params["action"] != "music" {
		t.Fatalf("路径参数异常: %v", params)
	}

	if _, _, _, ok := gen.Match("alpha", "GET", "/submit/music"); ok {
		t.Fatal("方法不匹配却命中")
	}
	if _, _, _, ok := gen.Match("alpha", "POST", "/fetch/x"); ok {
		t.Fatal("方法+路径组合错误却命中")
	}
}

func TestGenerationMatchLiteralPriority(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("a", []string{"m"},
			contract.Route{Method: "GET", Path: "/report/:name", Type: contract.RouteTypeQuery},
			contract.Route{Method: "GET", Path: "/report/special", Type: contract.RouteTypeQuery},
		),
	})
	// 插件内字面量路由应先于参数路由命中（即使参数路由先声明）
	_, route, _, ok := gen.Match("a", "GET", "/report/special")
	if !ok || route.Path != "/report/special" {
		t.Fatalf("字面量优先失败: ok=%v route=%+v", ok, route)
	}
	_, route, _, ok = gen.Match("a", "GET", "/report/other")
	if !ok || route.Path != "/report/:name" {
		t.Fatalf("参数路由兜底失败: ok=%v route=%+v", ok, route)
	}
}

func TestGenerationMatchNormalization(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("a", []string{"m"},
			contract.Route{Method: "POST", Path: "/submit/:action", Type: contract.RouteTypeSubmit},
		),
	})
	if _, _, _, ok := gen.Match("a", "POST", "/submit/music/"); !ok {
		t.Fatal("尾部斜杠应归一命中")
	}
	if _, _, _, ok := gen.Match("a", "POST", "//submit//music"); !ok {
		t.Fatal("连续斜杠应归一命中")
	}
}

// TestGenerationRouteNamespaceIsolation 跨插件同形状路由按 /pjs/{key} 命名空间隔离：
// 各厂商兼容路径（如 GET /videos/:task_id）可在多个插件中并存，互不遮蔽、
// 激活预检不拒绝（历史上先全局扁平匹配再校验 key，导致互相遮蔽+激活拒绝同形状路由）。
func TestGenerationRouteNamespaceIsolation(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("kling", []string{"kling_model"},
			contract.Route{Method: "POST", Path: "/videos", Type: contract.RouteTypeSubmit},
			contract.Route{Method: "GET", Path: "/videos/:task_id", Type: contract.RouteTypeQuery},
		),
		testPlugin("origin-sd", []string{"sd_model"},
			contract.Route{Method: "POST", Path: "/videos", Type: contract.RouteTypeSubmit},
			contract.Route{Method: "GET", Path: "/videos/:task_id", Type: contract.RouteTypeQuery},
		),
	})
	// 同形状路由在各插件自己的 key 下均命中，不串扰
	for _, key := range []string{"kling", "origin-sd"} {
		plugin, _, params, ok := gen.Match(key, "GET", "/videos/task-9")
		if !ok || plugin.Key != key || params["task_id"] != "task-9" {
			t.Fatalf("插件 %s 命名空间内路由匹配失败: ok=%v plugin=%s", key, ok, plugin.Key)
		}
	}
	// key 不存在的插件不命中（也不回落到其他插件的同形状路由）
	if _, _, _, ok := gen.Match("notexist", "GET", "/videos/task-9"); ok {
		t.Fatal("未知插件 key 不应命中其他插件的路由")
	}

	// 激活预检：同形状路由不构成冲突；模型同名仍拒绝
	a := &contract.Manifest{
		Key: "a", Version: "1.0.0", Models: []string{"model-a"},
		Routes: []contract.Route{{Method: "GET", Path: "/videos/:task_id", Type: contract.RouteTypeQuery}},
	}
	bSameShape := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"model-b"},
		Routes: []contract.Route{{Method: "GET", Path: "/videos/:request_id", Type: contract.RouteTypeQuery}},
	}
	if err := checkManifestConflict("a", a, "b", bSameShape); err != nil {
		t.Fatalf("跨插件同形状路由不应冲突: %v", err)
	}

	bModelDup := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"model-a"},
		Routes: []contract.Route{{Method: "GET", Path: "/videos/:request_id", Type: contract.RouteTypeQuery}},
	}
	if err := checkManifestConflict("a", a, "b", bModelDup); err == nil {
		t.Fatal("模型同名应当报冲突")
	}

	bModelCase := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"Model-A"},
		Routes: []contract.Route{{Method: "GET", Path: "/videos/:request_id", Type: contract.RouteTypeQuery}},
	}
	if err := checkManifestConflict("a", a, "b", bModelCase); err == nil {
		t.Fatal("模型名仅大小写差异应当报冲突")
	}
}

func TestGenerationFindPluginByModel(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("alpha", []string{"golden_music"}),
	})
	if key, ok := gen.FindPluginByModel("GOLDEN_MUSIC"); !ok || key != "alpha" {
		t.Fatalf("大小写不敏感模型查找失败: %v %v", key, ok)
	}
	if _, ok := gen.FindPluginByModel("unknown"); ok {
		t.Fatal("未知模型不应命中")
	}
}
