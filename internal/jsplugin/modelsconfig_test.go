// modelsconfig_test.go modelsFromConfig 扩展模型的纯逻辑测试：视图合并（不改写
// 原 manifest）、名单去重、独占冲突判定（精确 + 大小写折叠）。
package jsplugin

import (
	"reflect"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func modelsConfigManifest() *contract.Manifest {
	return &contract.Manifest{
		APIVersion:       1,
		Key:              "std",
		Name:             "Std",
		Version:          "1.0.0",
		Models:           []string{"m-base"},
		ModelsFromConfig: "extra_models",
		Config: []contract.ConfigField{
			{Key: "extra_models", Label: "扩展模型", Type: "text"},
		},
		Routes: []contract.Route{{Method: "POST", Path: "/submit", Type: contract.RouteTypeSubmit}},
	}
}

// TestApplyConfigModels_Merge 视图合并：扩展并入副本、原 manifest 不被改写
// （compiled.Manifest 是编译缓存共享对象，原地改写会毒化缓存）。
func TestApplyConfigModels_Merge(t *testing.T) {
	m := modelsConfigManifest()
	view, err := applyConfigModels(m, map[string]any{"extra_models": "m-extra-1\nm-base, m-extra-2"})
	if err != nil {
		t.Fatalf("合并失败: %v", err)
	}
	if view == m {
		t.Fatal("有扩展时应返回副本视图")
	}
	want := []string{"m-base", "m-extra-1", "m-extra-2"}
	if !reflect.DeepEqual(view.Models, want) {
		t.Fatalf("合并结果异常: %v", view.Models)
	}
	if !reflect.DeepEqual(m.Models, []string{"m-base"}) {
		t.Fatalf("原 manifest 被改写: %v", m.Models)
	}
}

// TestApplyConfigModels_NoExtras 未声明或扩展为空：原样返回原指针（零开销路径）。
func TestApplyConfigModels_NoExtras(t *testing.T) {
	m := modelsConfigManifest()
	if view, err := applyConfigModels(m, map[string]any{"extra_models": ""}); err != nil || view != m {
		t.Fatalf("空扩展应原样返回: %v %v", view, err)
	}

	m2 := modelsConfigManifest()
	m2.ModelsFromConfig = ""
	if view, err := applyConfigModels(m2, nil); err != nil || view != m2 {
		t.Fatalf("未声明应原样返回: %v %v", view, err)
	}
}

// TestApplyConfigModels_BadValue 解析失败返回错误（materialize 据此告警降级，
// 配置保存入口据此拒绝保存）。
func TestApplyConfigModels_BadValue(t *testing.T) {
	m := modelsConfigManifest()
	if _, err := applyConfigModels(m, map[string]any{"extra_models": 42}); err == nil {
		t.Fatal("非字符串/数组值应报错")
	}
}

// TestCheckModelSetConflict 独占冲突：精确同名、仅大小写不同均拒绝；无冲突放行；
// 双方声明 sharedModels 时同名放行（单方声明仍拒）。
func TestCheckModelSetConflict(t *testing.T) {
	others := []modelSetParty{{key: "other", models: []string{"kling-v3", "Sora-2"}}}
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"kling-v3"}}, others); err == nil {
		t.Fatal("精确同名应冲突")
	}
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"sora-2"}}, others); err == nil {
		t.Fatal("仅大小写不同应冲突")
	}
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"kling-v4", "Sora-2-pro"}}, others); err != nil {
		t.Fatalf("无冲突不应报错: %v", err)
	}

	// 双边声明 sharedModels：同名放行
	sharedOthers := []modelSetParty{{key: "other", models: []string{"kling-v3"}, shared: true}}
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"kling-v3"}, shared: true}, sharedOthers); err != nil {
		t.Fatalf("双边 sharedModels 同名应放行: %v", err)
	}
	// 单边声明：仍拒绝（防拼写意外撞名）
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"kling-v3"}, shared: true}, others); err == nil {
		t.Fatal("单边 sharedModels 同名应拒绝")
	}
	if err := checkModelSetConflict(modelSetParty{key: "mine", models: []string{"kling-v3"}}, sharedOthers); err == nil {
		t.Fatal("单边 sharedModels 同名应拒绝（对方声明己未声明）")
	}
}

// TestCheckManifestConflictShared 激活预检的共享放行：双方 sharedModels 同名不报错、
// 单方报错；路由形状冲突不受 sharedModels 影响。
func TestCheckManifestConflictShared(t *testing.T) {
	newM := func(routePath string, models []string, shared bool) *contract.Manifest {
		return &contract.Manifest{Models: models, SharedModels: shared,
			Routes: []contract.Route{{Method: "POST", Path: routePath, Type: contract.RouteTypeSubmit}}}
	}
	// 双边声明：同名模型放行
	if err := checkManifestConflict("a", newM("/a/submit", []string{"m-1"}, true), "b", newM("/b/submit", []string{"m-1"}, true)); err != nil {
		t.Fatalf("双边 sharedModels 同名应放行: %v", err)
	}
	// 单边声明：拒绝
	if err := checkManifestConflict("a", newM("/a/submit", []string{"m-1"}, true), "b", newM("/b/submit", []string{"m-1"}, false)); err == nil {
		t.Fatal("单边 sharedModels 同名应拒绝")
	}
	// 路由形状冲突：即使双方 sharedModels 也拒绝（/pjs 路由表硬约束）
	if err := checkManifestConflict("a", newM("/same/submit", []string{"m-1"}, true), "b", newM("/same/submit", []string{"m-2"}, true)); err == nil {
		t.Fatal("路由形状冲突应拒绝")
	}
}
