package admin

import (
	"testing"
)

// TestMarshalTenantTags 标签校验与序列化：去空白、去重、跳过空项、数量与长度上限
func TestMarshalTenantTags(t *testing.T) {
	cases := []struct {
		name    string
		input   []string
		want    string
		wantErr bool
	}{
		{"普通标签", []string{"vip", "重点客户"}, `["vip","重点客户"]`, false},
		{"去首尾空白", []string{"  vip ", "dev"}, `["vip","dev"]`, false},
		{"跳过空项", []string{"", "  ", "vip"}, `["vip"]`, false},
		{"去重", []string{"vip", "vip", "dev"}, `["vip","dev"]`, false},
		{"空数组清空", []string{}, `[]`, false},
		{"nil 清空", nil, `[]`, false},
		{"超长单标签", []string{"一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一"}, "", true},
		{"数量超限", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, "", true},
		{"英文30字符边界", []string{"abcdefghijklmnopqrstuvwxyz0123"}, `["abcdefghijklmnopqrstuvwxyz0123"]`, false},
		{"英文31字符超限", []string{"abcdefghijklmnopqrstuvwxyz0123" + "4"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := marshalTenantTags(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got=%s want=%s", got, c.want)
			}
		})
	}
}

// TestParseTenantTags 标签解析容错：空/非法输入返回空切片而非报错（展示路径）
func TestParseTenantTags(t *testing.T) {
	if got := parseTenantTags(""); got == nil || len(got) != 0 {
		t.Fatalf("empty input: want empty slice, got %v", got)
	}
	if got := parseTenantTags("not-json"); got == nil || len(got) != 0 {
		t.Fatalf("invalid json: want empty slice, got %v", got)
	}
	if got := parseTenantTags(`["vip","dev"]`); len(got) != 2 || got[0] != "vip" || got[1] != "dev" {
		t.Fatalf("valid json: got %v", got)
	}
}

// TestMarshalTagFilter 标签筛选参数序列化为 JSONB 包含查询条件
func TestMarshalTagFilter(t *testing.T) {
	if got := marshalTagFilter("vip"); got != `["vip"]` {
		t.Fatalf("got=%s want=[\"vip\"]", got)
	}
	if got := marshalTagFilter("  vip "); got != `["vip"]` {
		t.Fatalf("trimmed: got=%s", got)
	}
}
