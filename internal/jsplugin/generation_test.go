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

	plugin, route, params, ok := gen.Match("POST", "/submit/music")
	if !ok || plugin.Key != "alpha" || route.Type != contract.RouteTypeSubmit {
		t.Fatalf("提交路由匹配失败: ok=%v plugin=%v", ok, plugin)
	}
	if params["action"] != "music" {
		t.Fatalf("路径参数异常: %v", params)
	}

	if _, _, _, ok := gen.Match("GET", "/submit/music"); ok {
		t.Fatal("方法不匹配却命中")
	}
	if _, _, _, ok := gen.Match("POST", "/fetch/x"); ok {
		t.Fatal("方法+路径组合错误却命中")
	}
}

func TestGenerationMatchLiteralPriority(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("a", []string{"m"},
			contract.Route{Method: "GET", Path: "/report/:name", Type: contract.RouteTypeQuery},
		),
		testPlugin("b", []string{"m2"},
			contract.Route{Method: "GET", Path: "/report/special", Type: contract.RouteTypeQuery},
		),
	})
	// 字面量路由应先于参数路由命中（即使参数路由先声明）
	plugin, _, _, ok := gen.Match("GET", "/report/special")
	if !ok || plugin.Key != "b" {
		t.Fatalf("字面量优先失败: ok=%v plugin=%s", ok, plugin.Key)
	}
	plugin, _, _, ok = gen.Match("GET", "/report/other")
	if !ok || plugin.Key != "a" {
		t.Fatalf("参数路由兜底失败: ok=%v plugin=%s", ok, plugin.Key)
	}
}

func TestGenerationMatchNormalization(t *testing.T) {
	gen := buildGeneration([]*ActivePlugin{
		testPlugin("a", []string{"m"},
			contract.Route{Method: "POST", Path: "/submit/:action", Type: contract.RouteTypeSubmit},
		),
	})
	if _, _, _, ok := gen.Match("POST", "/submit/music/"); !ok {
		t.Fatal("尾部斜杠应归一命中")
	}
	if _, _, _, ok := gen.Match("POST", "//submit//music"); !ok {
		t.Fatal("连续斜杠应归一命中")
	}
}

func TestGenerationCrossPluginRouteConflict(t *testing.T) {
	a := &contract.Manifest{
		Key: "a", Version: "1.0.0", Models: []string{"model-a"},
		Routes: []contract.Route{{Method: "POST", Path: "/submit/:action", Type: contract.RouteTypeSubmit}},
	}
	b := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"model-b"},
		Routes: []contract.Route{{Method: "POST", Path: "/submit/:kind", Type: contract.RouteTypeSubmit}},
	}
	if err := checkManifestConflict("a", a, "b", b); err == nil {
		t.Fatal("同形状路由应当报冲突")
	}

	bNoConflict := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"model-b"},
		Routes: []contract.Route{{Method: "POST", Path: "/other/:kind", Type: contract.RouteTypeSubmit}},
	}
	if err := checkManifestConflict("a", a, "b", bNoConflict); err != nil {
		t.Fatalf("不同路由不应冲突: %v", err)
	}

	bModelCase := &contract.Manifest{
		Key: "b", Version: "1.0.0", Models: []string{"Model-A"},
		Routes: []contract.Route{{Method: "POST", Path: "/other/:kind", Type: contract.RouteTypeSubmit}},
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
