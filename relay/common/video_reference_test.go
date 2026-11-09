package common

import "testing"

func TestReferencesFromTaskBody(t *testing.T) {
	req := map[string]any{"references": []any{
		map[string]any{"role": "reference_image", "url": " https://a "},
		map[string]any{"role": "", "url": "https://skip"},
		"not-object",
		map[string]any{"role": "reference_video", "url": "https://v"},
	}}
	refs := ReferencesFromTaskBody(req)
	if len(refs) != 2 || refs[0].URL != "https://a" || refs[1].Role != RefRoleReferenceVideo {
		t.Fatalf("防御性读取异常: %+v", refs)
	}
	if ReferencesFromTaskBody(map[string]any{}) != nil {
		t.Fatal("无 references 应返回 nil")
	}
	if got := ReferencesFromTaskBodyBytes([]byte(`{"references":[{"role":"first_frame","url":"https://f"}]}`)); len(got) != 1 {
		t.Fatalf("字节形态读取异常: %+v", got)
	}
}

func TestRejectUnsupportedReferences(t *testing.T) {
	refs := []VideoReference{{Role: RefRoleFirstFrame, URL: "https://f"}}
	if terr := RejectUnsupportedReferences(refs, "kling-v2", RefRoleFirstFrame); terr != nil {
		t.Fatalf("首帧应放行: %+v", terr)
	}
	refs = append(refs, VideoReference{Role: RefRoleLastFrame, URL: "https://l"})
	terr := RejectUnsupportedReferences(refs, "kling-v2", RefRoleFirstFrame)
	if terr == nil || terr.StatusCode != 400 || terr.ErrCode != ErrCodeReferenceNotSupported {
		t.Fatalf("未声明角色应拒绝: %+v", terr)
	}
	if RejectUnsupportedReferences(nil, "x") != nil {
		t.Fatal("无参考素材不应拒绝")
	}
}

func TestReferenceKind(t *testing.T) {
	cases := map[string]string{
		RefRoleFirstFrame: RefKindImage, RefRoleLastFrame: RefKindImage, RefRoleReferenceImage: RefKindImage,
		RefRoleReferenceVideo: RefKindVideo, RefRoleReferenceAudio: RefKindAudio, "reference_document": "",
	}
	for role, want := range cases {
		if got := ReferenceKind(role); got != want {
			t.Errorf("ReferenceKind(%q)=%q want %q", role, got, want)
		}
	}
}
