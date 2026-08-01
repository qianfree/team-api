package goldentest

import (
	"path/filepath"
	"testing"

	"github.com/qianfree/team-api/relaykit/types"
)

func TestEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want bool
	}{
		{"equal maps", map[string]any{"a": 1}, map[string]any{"a": 1}, true},
		{"value diff", map[string]any{"a": 1}, map[string]any{"a": 2}, false},
		{"len diff", map[string]any{"a": 1}, map[string]any{"a": 1, "b": 2}, false},
		{"missing key", map[string]any{"a": 1}, map[string]any{"b": 1}, false},
		{"nested slice equal", map[string]any{"a": []any{1, 2}}, map[string]any{"a": []any{1, 2}}, true},
		{"nested slice diff", map[string]any{"a": []any{1, 2}}, map[string]any{"a": []any{1, 3}}, false},
		{"nested map", map[string]any{"a": map[string]any{"x": 1}}, map[string]any{"a": map[string]any{"x": 1}}, true},
		{"primitive equal", "x", "x", true},
		{"primitive diff", "x", "y", false},
		{"slice equal", []any{1, 2}, []any{1, 2}, true},
		{"nil equal", nil, nil, true},
	}
	for _, c := range cases {
		if got := Equal(c.a, c.b); got != c.want {
			t.Errorf("%s: Equal(%+v,%+v) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

func TestEqualExcluding(t *testing.T) {
	// 忽略 b 后相等
	if !EqualExcluding(
		map[string]any{"a": 1, "b": 2},
		map[string]any{"a": 1, "b": 9},
		"b",
	) {
		t.Error("expected equal after excluding b")
	}
	// a 不同，排除 b 仍不相等
	if EqualExcluding(
		map[string]any{"a": 1},
		map[string]any{"a": 2},
		"b",
	) {
		t.Error("expected unequal when a differs")
	}
}

func TestEqualChunksExcluding(t *testing.T) {
	got := []any{
		map[string]any{"id": "chatcmpl-1", "content": "hi"},
		map[string]any{"id": "chatcmpl-1", "content": ""},
	}
	want := []any{
		map[string]any{"id": "chatcmpl-99", "content": "hi"}, // id 不同但被忽略
		map[string]any{"id": "chatcmpl-99", "content": ""},
	}
	if !EqualChunksExcluding(got, want, "id") {
		t.Error("expected chunks equal after excluding id")
	}
	// 长度不同
	if EqualChunksExcluding(got, want[:1], "id") {
		t.Error("expected unequal for different length")
	}
	// 内容不同（排除 id 后仍不一致）
	diff := []any{map[string]any{"id": "x", "content": "bye"}}
	if EqualChunksExcluding(diff, want[:1], "id") {
		t.Error("expected unequal when content differs")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	tc := TestCase{
		Name:       "roundtrip",
		From:       types.RelayFormatOpenAI,
		To:         types.RelayFormatClaude,
		Request:    map[string]any{"model": "gpt-4"},
		ConverterID: "test_converter",
	}
	path := filepath.Join(t.TempDir(), "case.json")
	Save(t, path, tc)

	loaded := Load(t, path)
	if loaded.Name != "roundtrip" {
		t.Errorf("Name = %q", loaded.Name)
	}
	if loaded.From != types.RelayFormatOpenAI || loaded.To != types.RelayFormatClaude {
		t.Errorf("From/To = %q/%q", loaded.From, loaded.To)
	}
	if loaded.ConverterID != "test_converter" {
		t.Errorf("ConverterID = %q", loaded.ConverterID)
	}
	if m, ok := loaded.Request.(map[string]any); !ok || m["model"] != "gpt-4" {
		t.Errorf("Request not round-tripped: %+v", loaded.Request)
	}
}
