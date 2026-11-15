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
