package common

import (
	"testing"
)

func TestResultURLFromArtifacts(t *testing.T) {
	data := []byte(`{"_video_url":"https://cdn.example.com/v.mp4","cover":"https://cdn.example.com/c.png","rel":"/files/v.mp4"}`)
	cases := []struct {
		name      string
		artifacts []TaskArtifact
		want      string
	}{
		{"无产物", nil, ""},
		{"url 直链", []TaskArtifact{{SourceType: "url", SourcePath: "_video_url", MimeType: "video/mp4"}}, "https://cdn.example.com/v.mp4"},
		{"video 类型优先", []TaskArtifact{
			{SourceType: "url", SourcePath: "cover", MimeType: "image/png"},
			{SourceType: "url", SourcePath: "_video_url", MimeType: "video/mp4"},
		}, "https://cdn.example.com/v.mp4"},
		{"无 video 类型回落首个", []TaskArtifact{{SourceType: "url", SourcePath: "cover"}}, "https://cdn.example.com/c.png"},
		{"相对 URL 跳过", []TaskArtifact{{SourceType: "url", SourcePath: "rel", MimeType: "video/mp4"}}, ""},
		{"鉴权产物跳过", []TaskArtifact{{SourceType: "url", SourcePath: "_video_url", SourceAuth: "channel-bearer"}}, ""},
		{"inline 跳过", []TaskArtifact{{SourceType: "inline", SourcePath: "_video_url"}}, ""},
		{"路径缺失", []TaskArtifact{{SourceType: "url", SourcePath: "missing"}}, ""},
	}
	for _, c := range cases {
		if got := ResultURLFromArtifacts(data, c.artifacts); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
	if got := ResultURLFromArtifacts([]byte("not json"), cases[1].artifacts); got != "" {
		t.Errorf("非法快照应返回空: %q", got)
	}
}
