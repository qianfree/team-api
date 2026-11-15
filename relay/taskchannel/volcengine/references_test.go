package volcengine

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// TestReferences_BuildContentOrdered references[] → 带 role 的 Ark content 项（保持原序），
// 兼容字段 images[] 不混入（防首帧重复）；参考视频命中视频输入计费判定
func TestReferences_BuildContentOrdered(t *testing.T) {
	a := &VolcengineVideoAdaptor{}
	body := []byte(`{"model":"doubao-seedance-2-0-260128","prompt":"图片1在视频1里",
		"images":["https://legacy/first.png"],
		"references":[
			{"role":"reference_image","url":"https://img/1.png"},
			{"role":"reference_video","url":"https://vid/1.mp4"},
			{"role":"reference_audio","url":"https://aud/1.mp3"}]}`)
	info := &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
	reader, err := a.BuildRequestBody(context.Background(), info, body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := io.ReadAll(reader)
	var out submitRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantTypes := []string{"text", "image_url", "video_url", "audio_url"}
	wantRoles := []string{"", "reference_image", "reference_video", "reference_audio"}
	if len(out.Content) != len(wantTypes) {
		t.Fatalf("content 项数异常: %s", raw)
	}
	for i := range wantTypes {
		if out.Content[i].Type != wantTypes[i] || out.Content[i].Role != wantRoles[i] {
			t.Fatalf("content[%d] 异常: %+v", i, out.Content[i])
		}
	}
	if out.Content[2].VideoURL == nil || out.Content[2].VideoURL.URL != "https://vid/1.mp4" {
		t.Fatalf("video_url 映射异常: %s", raw)
	}

	var req map[string]any
	_ = json.Unmarshal(body, &req)
	if !hasVideoInput(req) {
		t.Fatal("references 参考视频应命中视频输入判定")
	}
}

func TestReferences_Validate(t *testing.T) {
	a := &VolcengineVideoAdaptor{}
	const v2 = "doubao-seedance-2-0-260128"
	cases := []struct {
		name   string
		model  string
		refs   string
		wantOK bool
	}{
		{"2.0 多模态通过", v2, `[{"role":"reference_image","url":"https://i"},{"role":"reference_audio","url":"https://a"}]`, true},
		{"首尾帧通过", v2, `[{"role":"first_frame","url":"https://a"},{"role":"last_frame","url":"https://b"}]`, true},
		{"1.x 多模态拒绝", "doubao-seedance-1-0-pro-250528", `[{"role":"reference_image","url":"https://i"}]`, false},
		{"1.x 首帧通过", "doubao-seedance-1-0-lite-i2v", `[{"role":"first_frame","url":"https://i"}]`, true},
		{"音频须搭配图或视频", v2, `[{"role":"reference_audio","url":"https://a"}]`, false},
		{"参考视频不收 base64", v2, `[{"role":"reference_video","url":"data:video/mp4;base64,AAAA"}]`, false},
		{"尾帧须搭配首帧", v2, `[{"role":"last_frame","url":"https://b"}]`, false},
		{"参考图超 9", v2, `[` + repeatRef(`{"role":"reference_image","url":"https://i"}`, 10) + `]`, false},
	}
	for _, c := range cases {
		body := `{"model":"` + c.model + `","prompt":"p","references":` + c.refs + `}`
		terr := a.ValidateRequest(context.Background(), nil, []byte(body))
		if (terr == nil) != c.wantOK {
			t.Errorf("%s: ok=%v want %v (%+v)", c.name, terr == nil, c.wantOK, terr)
		}
	}
}

func repeatRef(item string, n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += item
	}
	return s
}
