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
