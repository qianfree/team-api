package jsplugin

import (
	"strings"
	"testing"
)

// TestLetterAvatarSVGDeterministic 同 key/name 恒定输出；不同 key 色相散开。
func TestLetterAvatarSVGDeterministic(t *testing.T) {
	a1 := LetterAvatarSVG("golden", "Golden 示例插件")
	a2 := LetterAvatarSVG("golden", "Golden 示例插件")
	if string(a1) != string(a2) {
		t.Fatal("同 key/name 应生成完全一致的头像（缓存与一致性依赖）")
	}
	if !strings.Contains(string(a1), "<svg") || !strings.Contains(string(a1), "G") {
		t.Fatalf("头像应为含首字符的 SVG: %s", a1)
	}

	b := LetterAvatarSVG("kling", "可灵视频")
	if string(a1) == string(b) {
		t.Fatal("不同 key 应产生不同配色")
	}
	// 中文名首字符直接使用
	if !strings.Contains(string(b), "可") {
		t.Fatalf("中文首字符应保留: %s", b)
	}
}

// TestLetterAvatarSVGEscape 特殊字符转义（name 以 & < > 开头的极端情况；
// 头像只取首字符，转义对象是单个字符）。
func TestLetterAvatarSVGEscape(t *testing.T) {
	out := string(LetterAvatarSVG("x", `<script>`))
	if strings.Contains(out, "<script>") {
		t.Fatalf("字母位必须 XML 转义: %s", out)
	}
	// 首字符 < 应以实体形态出现，且不出现在标签结构位之外
	if !strings.Contains(out, "&lt;") {
		t.Fatalf("应包含转义后的首字符: %s", out)
	}
}

// TestLetterAvatarSVGBounds 空 name 回落 key 首字符；两者皆空回落 P。
func TestLetterAvatarSVGBounds(t *testing.T) {
	if !strings.Contains(string(LetterAvatarSVG("demo", "")), "d") {
		t.Fatal("name 空时应回落 key 首字符")
	}
	if !strings.Contains(string(LetterAvatarSVG("", "")), "P") {
		t.Fatal("key/name 均空时应回落 P")
	}
}
