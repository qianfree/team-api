package minimax

import (
	"context"
	"encoding/json"
	"testing"
)

func mustReq(t *testing.T, body string) map[string]any {
	t.Helper()
	var req map[string]any
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("bad body: %v", err)
	}
	return req
}

// TestReferences_V2ContentOrdered references[] → v2 content 项：role 原样、保持原序、
// 兼容字段 images[]/metadata.image 不混入（防首帧重复）
func TestReferences_V2ContentOrdered(t *testing.T) {
	req := mustReq(t, `{"model":"MiniMax-H3","prompt":"reference image 1 in reference video 1",
		"images":["https://legacy/first.png"],"metadata":{"image":"https://legacy/first.png"},
		"references":[
			{"role":"reference_image","url":"https://img/1.png"},
			{"role":"reference_video","url":"https://vid/1.mp4"},
			{"role":"reference_audio","url":"https://aud/1.mp3"}]}`)
	out := buildV2RequestBody(req, "MiniMax-H3")
	content := out["content"].([]map[string]any)
	wantTypes := []string{"text", "image_url", "video_url", "audio_url"}
	if len(content) != len(wantTypes) {
		t.Fatalf("content 项数异常: %v", content)
	}
	for i, typ := range wantTypes {
		if content[i]["type"] != typ {
			t.Fatalf("content[%d].type=%v want %s", i, content[i]["type"], typ)
		}
	}
	if content[2]["role"] != "reference_video" || content[2]["video_url"].(map[string]any)["url"] != "https://vid/1.mp4" {
		t.Fatalf("video 项异常: %v", content[2])
	}
}

// TestReferences_V1Frames v1：references 首尾帧 → first/last_frame_image；S2V-01 参考图 → subject_reference
func TestReferences_V1Frames(t *testing.T) {
	req := mustReq(t, `{"model":"MiniMax-Hailuo-02","prompt":"p","references":[
		{"role":"first_frame","url":"https://a"},{"role":"last_frame","url":"https://b"}]}`)
	out := buildV1RequestBody(req, "MiniMax-Hailuo-02")
	if out["first_frame_image"] != "https://a" || out["last_frame_image"] != "https://b" {
		t.Fatalf("v1 首尾帧映射异常: %v", out)
	}
	s2v := buildV1RequestBody(mustReq(t, `{"model":"S2V-01","prompt":"p","references":[{"role":"reference_image","url":"https://face"}]}`), "S2V-01")
	sr, ok := s2v["subject_reference"].([]map[string]any)
	if !ok || len(sr) != 1 || sr[0]["type"] != "character" {
		t.Fatalf("S2V-01 subject_reference 映射异常: %v", s2v)
	}
}

func TestReferences_Validate(t *testing.T) {
	a := &Adaptor{}
	refs := func(items string) string { return `,"references":[` + items + `]` }
	img := `{"role":"reference_image","url":"https://i"}`
	vid := `{"role":"reference_video","url":"https://v"}`
	aud := `{"role":"reference_audio","url":"https://a"}`
	repeat := func(item string, n int) string {
		s := ""
		for i := 0; i < n; i++ {
			if i > 0 {
				s += ","
			}
			s += item
		}
		return s
	}
	cases := []struct {
		name   string
		model  string
		extra  string
		wantOK bool
	}{
		{"H3 合计 12 通过", "MiniMax-H3", refs(repeat(img, 9) + "," + repeat(vid, 3)), true},
		{"H3 合计超 12", "MiniMax-H3", refs(repeat(img, 9) + "," + repeat(vid, 3) + "," + aud), false},
		{"H3 参考视频超 3", "MiniMax-H3", refs(repeat(vid, 4)), false},
		{"Lite 参考模式缺比例", "MiniMax-H3-Lite", refs(img), false},
		{"Lite 显式比例通过", "MiniMax-H3-Lite", `,"ratio":"16:9"` + refs(img), true},
		{"Hailuo-02 首帧通过", "MiniMax-Hailuo-02", refs(`{"role":"first_frame","url":"https://f"}`), true},
		{"Hailuo-02 参考视频拒绝", "MiniMax-Hailuo-02", refs(vid), false},
		{"S2V-01 单参考图通过", "S2V-01", refs(img), true},
		{"S2V-01 两张参考图拒绝", "S2V-01", refs(img + "," + img), false},
	}
	for _, c := range cases {
		body := `{"model":"` + c.model + `","prompt":"p"` + c.extra + `}`
		terr := a.ValidateRequest(context.Background(), nil, []byte(body))
		if (terr == nil) != c.wantOK {
			t.Errorf("%s: ok=%v want %v (%+v)", c.name, terr == nil, c.wantOK, terr)
		}
	}
}

// TestReferences_BillingSignals 预扣素材信号：references 图片计数（首尾帧 + 参考图）与参考视频存在性；
// 兼容双写 images[0] + metadata.image 只计一张（此前计两张导致多冻结）
func TestReferences_BillingSignals(t *testing.T) {
	a := &Adaptor{}
	cases := []struct {
		name      string
		body      string
		wantCount float64
		wantVideo bool
	}{
		{"references 图+视频", `{"model":"MiniMax-H3","prompt":"p","images":["https://x"],"references":[{"role":"reference_image","url":"https://1"},{"role":"reference_image","url":"https://2"},{"role":"reference_video","url":"https://v"}]}`, 2, true},
		{"兼容双写首帧只计一次", `{"model":"MiniMax-H3","prompt":"p","images":["https://a"],"metadata":{"image":"https://a"}}`, 1, false},
	}
	for _, c := range cases {
		ratios := a.EstimateBilling(context.Background(), nil, []byte(c.body))
		if got := ratios["spec.input_image_count"].(float64); got != c.wantCount {
			t.Errorf("%s: input_image_count=%v want %v", c.name, got, c.wantCount)
		}
		if got := ratios["spec.has_input_video"].(bool); got != c.wantVideo {
			t.Errorf("%s: has_input_video=%v want %v", c.name, got, c.wantVideo)
		}
	}
}
