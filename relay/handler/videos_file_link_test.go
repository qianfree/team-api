package handler

import (
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// file/link 参考角色（万相 wan3.0 文档文件与网页链接）：
// 协议层组合规则（各 ≤1、互斥、可与 reference_* 组合、与首尾帧互斥）与 URL/file_id 形态校验。

func TestVideosFileLink_CombinationRules(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"file 单独", `{"role":"file","url":"https://a.com/doc.pdf"}`, true},
		{"link 单独", `{"role":"link","url":"https://a.com/page"}`, true},
		{"file+reference_image 组合", `[{"role":"file","url":"https://a.com/d.pdf"},{"role":"reference_image","url":"https://a.com/i.png"}]`, true},
		{"file+link 互斥", `[{"role":"file","url":"https://a.com/d.pdf"},{"role":"link","url":"https://a.com/p"}]`, false},
		{"两个 file 超限", `[{"role":"file","url":"https://a.com/1.pdf"},{"role":"file","url":"https://a.com/2.pdf"}]`, false},
		{"file+first_frame 互斥", `[{"role":"first_frame","url":"https://a.com/f.png"},{"role":"file","url":"https://a.com/d.pdf"}]`, false},
		{"file data URL 拒绝", `{"role":"file","url":"data:application/pdf;base64,AAAA"}`, false},
		{"link data URL 拒绝", `{"role":"link","url":"data:text/html;base64,PGh0bWw+"}`, false},
		{"link oss:// 拒绝", `{"role":"link","url":"oss://dashscope-instant/x"}`, false},
		{"file oss:// 临时 URL 放行", `{"role":"file","url":"oss://dashscope-instant/x/doc.pdf"}`, true},
		{"reference_image oss:// 放行", `{"role":"reference_image","url":"oss://dashscope-instant/x/i.png"}`, true},
		{"file_id 传给 link 拒绝", `{"role":"link","file_id":"file-abcdef12"}`, false},
	}
	for _, c := range cases {
		refs := c.body
		if !strings.HasPrefix(refs, "[") {
			refs = "[" + refs + "]"
		}
		_, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":`+refs+`}`)
		if c.ok != (terr == nil) {
			t.Fatalf("%s: ok=%v got %+v", c.name, c.ok, terr)
		}
	}
}

// file 角色的 file_id 解析：内容类型不做主类型前缀限制（application/pdf 等文档类型多样）
func TestVideosFileLink_FileIDResolution(t *testing.T) {
	req, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[
		{"role":"file","file_id":"file-pdf12345"},
		{"role":"reference_image","file_id":"file-img67890"}]}`)
	if terr != nil {
		t.Fatalf("解析应通过: %+v", terr)
	}
	resolved := map[string]ResolvedReferenceFile{
		"file-pdf12345": {URL: "https://oss/doc.pdf", ContentType: "application/pdf"},
		"file-img67890": {URL: "https://oss/img.png", ContentType: "image/png"},
	}
	terr = ResolveVideosReferenceFiles(req, func(ids []string) (map[string]ResolvedReferenceFile, error) {
		return resolved, nil
	})
	if terr != nil {
		t.Fatalf("file/file_id 应放行任意内容类型: %+v", terr)
	}
	if req.References[0].URL != "https://oss/doc.pdf" {
		t.Fatalf("file_id 应解析为签名 URL: %+v", req.References[0])
	}

	// 媒体角色的前缀校验不受影响：reference_image 拿到 pdf 应拒绝
	req2, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[{"role":"reference_image","file_id":"file-pdf12345"}]}`)
	if terr != nil {
		t.Fatal(terr)
	}
	terr = ResolveVideosReferenceFiles(req2, func([]string) (map[string]ResolvedReferenceFile, error) {
		return resolved, nil
	})
	if terr == nil || terr.ErrCode != errCodeReferenceFileMismatch {
		t.Fatalf("reference_image+pdf 应拒绝: %+v", terr)
	}
}

// ReferenceKind 新角色类别 + RejectFileLinkRoles
func TestVideosFileLink_KindAndReject(t *testing.T) {
	if common.ReferenceKind("file") != common.RefKindFile || common.ReferenceKind("link") != common.RefKindLink {
		t.Fatal("file/link 应有独立类别")
	}
	if common.IsFrameRole("file") {
		t.Fatal("file 不是帧角色")
	}
	terr := common.RejectFileLinkRoles([]common.VideoReference{{Role: "file", URL: "https://a.com/d.pdf"}}, "doubao-seedance")
	if terr == nil || terr.ErrCode != common.ErrCodeReferenceNotSupported {
		t.Fatalf("Seedance 应拒绝 file 角色: %+v", terr)
	}
	if terr := common.RejectFileLinkRoles([]common.VideoReference{{Role: "reference_image", URL: "https://a.com/i.png"}}, "m"); terr != nil {
		t.Fatalf("reference_image 不受影响: %+v", terr)
	}
}
