package jsplugin

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestBuiltinPluginsCompile 内置插件（go:embed）全部可编译且 manifest 合法——
// 这是发布质量门：内置插件编译失败不许随二进制出门。
// （sora/kling/sunoapi/xai 等已迁出 builtin 目录改为 DB 管理插件，2bffa46f 起不再断言。）
func TestBuiltinPluginsCompile(t *testing.T) {
	builtins, err := compileBuiltins(NewGojaEngine())
	if err != nil {
		t.Fatalf("内置插件编译失败: %v", err)
	}
	keys := map[string]*CompiledPlugin{}
	for _, b := range builtins {
		keys[b.Key] = b
	}
	if _, ok := keys["golden"]; !ok {
		t.Fatal("缺少内置插件 golden")
	}
	// golden 形态断言：视频模型 + 标准端点声明（模板范本必须示范双入口）
	golden := keys["golden"]
	if !golden.Manifest.HasModel("golden_video") || golden.Manifest.StandardVideos != "decodeVideos" {
		t.Fatalf("golden 应示范视频模型与 standardVideos 声明: %+v", golden.Manifest)
	}
}

func mustRawMap(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	m, err := contract.ParseJSONObject(raw)
	if err != nil {
		t.Fatalf("usage 非对象: %v", err)
	}
	return m
}
