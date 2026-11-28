package common

import (
	"encoding/json"
	"testing"
)

func TestJSONPathRoundtrip(t *testing.T) {
	var data map[string]any
	err := json.Unmarshal([]byte(`{
		"status": "success",
		"data": {
			"audio_url": "https://up.example.com/a.mp3",
			"videos": [{"url": "https://up.example.com/v1.mp4"}, {"url": "https://up.example.com/v2.mp4"}]
		}
	}`), &data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if v := GetJSONPath(data, "data.audio_url"); v != "https://up.example.com/a.mp3" {
		t.Fatalf("顶层路径取值异常: %v", v)
	}
	if v := GetJSONPath(data, "data.videos[1].url"); v != "https://up.example.com/v2.mp4" {
		t.Fatalf("嵌套数组取值异常: %v", v)
	}
	if v := GetJSONPath(data, "data.videos[5].url"); v != nil {
		t.Fatalf("越界应返回 nil: %v", v)
	}
	if v := GetJSONPath(data, "data.missing"); v != nil {
		t.Fatalf("缺失路径应返回 nil: %v", v)
	}

	// url 改写（url 型产物）
	if !SetJSONPath(data, "data.videos[1].url", "https://oss.local/v2.mp4") {
		t.Fatal("数组内路径改写失败")
	}
	if v := GetJSONPath(data, "data.videos[1].url"); v != "https://oss.local/v2.mp4" {
		t.Fatalf("改写后取值异常: %v", v)
	}

	// 路径必须已存在（产物声明指向 taskData 已有字段）
	if SetJSONPath(data, "data.not.exist", "x") {
		t.Fatal("不存在的路径不应允许创建")
	}
}

// TestJSONPathNumericKeyAsIndex 数组段的纯数字 key 兼容为下标：
// `results.0.url` ≡ `results[0].url`。AI 生成插件的常见笔误形态——不兼容的话
// 取值恒为 nil，转存/ResultURL/渲染替换整链静默失败（任务完成但前端恒 99%）。
func TestJSONPathNumericKeyAsIndex(t *testing.T) {
	var data map[string]any
	err := json.Unmarshal([]byte(`{
		"results": [{"url": "https://up.example.com/1.mp4"}, {"url": "https://up.example.com/2.mp4"}],
		"meta": {"0": "数字键的 map 字段"}
	}`), &data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if v := GetJSONPath(data, "results.0.url"); v != "https://up.example.com/1.mp4" {
		t.Fatalf("数字 key 应兼容为数组下标: %v", v)
	}
	if v := GetJSONPath(data, "results.1.url"); v != "https://up.example.com/2.mp4" {
		t.Fatalf("数字 key 下标取值异常: %v", v)
	}
	if v := GetJSONPath(data, "results.5.url"); v != nil {
		t.Fatalf("越界应返回 nil: %v", v)
	}
	// map 上的数字字符串键不受影响（类型决定分支，兼容只在 []any 节点生效）
	if v := GetJSONPath(data, "meta.0"); v != "数字键的 map 字段" {
		t.Fatalf("map 数字键取值异常: %v", v)
	}
	if !SetJSONPath(data, "results.0.url", "https://oss.local/1.mp4") {
		t.Fatal("数字 key 形态的改写失败")
	}
	if v := GetJSONPath(data, "results[0].url"); v != "https://oss.local/1.mp4" {
		t.Fatalf("两种下标形态应指向同一位置: %v", v)
	}
}

func TestSplitPath(t *testing.T) {
	cases := []struct {
		path  string
		want  int // 段数
		first string
	}{
		{"data.audio_url", 2, "data"},
		{"data.videos[0].url", 4, "data"},
		{"videos[10]", 2, "videos"},
		{"single", 1, "single"},
	}
	for _, c := range cases {
		segs := splitPath(c.path)
		if len(segs) != c.want || segs[0].key != c.first {
			t.Fatalf("splitPath(%q) = %#v", c.path, segs)
		}
	}
	// 索引段解析
	segs := splitPath("data.videos[12].url")
	if segs[1].key != "videos" || segs[2].index != 12 || segs[3].key != "url" {
		t.Fatalf("索引段解析异常: %#v", segs)
	}
}
