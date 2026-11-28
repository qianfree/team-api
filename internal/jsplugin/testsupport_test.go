package jsplugin

import (
	"os"
	"testing"
)

// readBuiltin 读取内置插件源码（builtin/{key}/plugin.js），迁移/单测共用。
func readBuiltin(t *testing.T, key string) string {
	t.Helper()
	data, err := os.ReadFile("builtin/" + key + "/plugin.js")
	if err != nil {
		t.Fatalf("读取内置插件 %s 失败: %v", key, err)
	}
	return string(data)
}
