package jsplugin

import (
	"os"
	"strings"
	"testing"
)

// TestDemoEchoFileDryRun 临时验证：docs/plugin-api/demo-echo.plugin.js 经 DryRun 应无 fail。
func TestDemoEchoFileDryRun(t *testing.T) {
	data, err := os.ReadFile("../../docs/plugin-api/demo-echo.plugin.js")
	if err != nil {
		t.Fatalf("读取示例插件失败: %v", err)
	}
	report := DryRun(string(data))
	for _, c := range report.Checks {
		t.Logf("检查项 %-16s %s | %s", c.Name, c.Status, c.Detail)
		if c.Status == "fail" {
			t.Errorf("示例插件不应有 fail 项: %s - %s", c.Name, c.Detail)
		}
	}
	if report.Manifest == nil || report.Manifest.Key != "demo-echo" {
		t.Fatalf("manifest 解析异常: %+v", report.Manifest)
	}
	if strings.Contains(string(data), "import ") {
		t.Fatal("示例插件不应包含 import")
	}
}
