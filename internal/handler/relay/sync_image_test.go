package relay

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripStreamField(t *testing.T) {
	// stream=true 应被移除，其余字段保留
	in := []byte(`{"model":"dall-e-3","prompt":"a cat","stream":true}`)
	out := stripStreamField(in)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}
	if _, ok := m["stream"]; ok {
		t.Fatal("stream field should be removed")
	}
	if _, ok := m["model"]; !ok {
		t.Fatal("model field should be preserved")
	}
	if _, ok := m["prompt"]; !ok {
		t.Fatal("prompt field should be preserved")
	}
}

func TestStripStreamField_NoStream(t *testing.T) {
	in := []byte(`{"model":"dall-e-3","prompt":"a cat"}`)
	out := stripStreamField(in)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("field count = %d, want 2", len(m))
	}
}

func TestStripStreamField_InvalidJSON(t *testing.T) {
	in := []byte(`not json`)
	if got := stripStreamField(in); string(got) != string(in) {
		t.Fatal("invalid json should be returned unchanged")
	}
}

func TestRequestForcesB64(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		model string
		want  bool
	}{
		{"explicit b64_json", `{"response_format":"b64_json"}`, "dall-e-3", true},
		{"b64_json case-insensitive", `{"response_format":"B64_JSON"}`, "dall-e-3", true},
		{"gpt-image always b64", `{}`, "gpt-image-1", true},
		{"gpt-image case-insensitive", `{}`, "GPT-Image-1", true},
		{"response_format url passthrough", `{"response_format":"url"}`, "dall-e-3", false},
		{"default dall-e url", `{"prompt":"a cat"}`, "dall-e-3", false},
		{"invalid json non-b64 model", `not json`, "dall-e-2", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requestForcesB64([]byte(c.body), c.model); got != c.want {
				t.Errorf("requestForcesB64(%q, %q) = %v, want %v", c.body, c.model, got, c.want)
			}
		})
	}
}

func TestCheckSyncImageIPWhitelist(t *testing.T) {
	cases := []struct {
		name      string
		whitelist string
		clientIP  string
		want      bool
	}{
		{"empty allows all", "", "1.2.3.4:5678", true},
		{"exact match", "1.2.3.4", "1.2.3.4:5678", true},
		{"exact no host:port", "1.2.3.4", "1.2.3.4", true},
		{"non-match", "1.2.3.4", "5.6.7.8:1000", false},
		{"cidr match", "10.0.0.0/8", "10.1.2.3:9", true},
		{"cidr non-match", "10.0.0.0/8", "11.1.2.3:9", false},
		{"multi list match", "9.9.9.9, 10.0.0.0/8", "10.5.5.5", true},
		{"invalid ip", "1.2.3.4", "not-an-ip", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkSyncImageIPWhitelist(c.whitelist, c.clientIP); got != c.want {
				t.Errorf("checkSyncImageIPWhitelist(%q, %q) = %v, want %v", c.whitelist, c.clientIP, got, c.want)
			}
		})
	}
}

func TestGenerateSyncImagePublicID(t *testing.T) {
	id := generateSyncImagePublicID()
	if !strings.HasPrefix(id, "task_") {
		t.Fatalf("id %q should have task_ prefix", id)
	}
	if len(id) != len("task_")+32 {
		t.Fatalf("id %q length = %d, want %d", id, len(id), len("task_")+32)
	}
	// 唯一性（极小概率碰撞）
	if generateSyncImagePublicID() == id {
		t.Fatal("two ids should differ")
	}
}
