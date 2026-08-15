package update

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    bool
		wantErr bool
	}{
		{"落后于最新版", "1.0.0", "1.0.1", true, false},
		{"小幅落后大版本", "1.9.9", "2.0.0", true, false},
		{"版本相同", "1.0.0", "1.0.0", false, false},
		{"当前更新", "1.2.0", "1.1.0", false, false},
		{"latest 带 v 前缀", "1.0.0", "v1.0.1", true, false},
		{"dev 构建保守提示更新", "dev", "0.0.1", true, false},
		{"当前版本无法解析按 dev 处理", "garbage", "0.0.1", true, false},
		{"git describe 版本与基础版本等价", "0.2.0_39", "0.2.0", false, false},
		{"git describe 版本落后于新版", "0.2.0_39", "0.2.1", true, false},
		{"latest 无法解析报错", "1.0.0", "not-a-version", false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := compareVersions(c.current, c.latest)
			if c.wantErr {
				if err == nil {
					t.Fatalf("latest=%q 无法解析时应返回错误", c.latest)
				}
				return
			}
			if err != nil {
				t.Fatalf("比较失败: %v", err)
			}
			if got != c.want {
				t.Errorf("compareVersions(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
			}
		})
	}
}

func TestNormalizeSemver(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0.2.0_39", "0.2.0+39"}, // Makefile git describe 版本：下划线转构建元数据
		{"v1.2.3", "1.2.3"},      // v 前缀剥离
		{"1.2.3", "1.2.3"},       // 原样保留
		{"dev", "dev"},
	}
	for _, c := range cases {
		if got := normalizeSemver(c.in); got != c.want {
			t.Errorf("normalizeSemver(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGetPlatformAssetName(t *testing.T) {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}

	// v 前缀应被剥离
	got := getPlatformAssetName("v1.2.3")
	want := fmt.Sprintf("team-api-1.2.3-%s-%s.%s", runtime.GOOS, runtime.GOARCH, ext)
	if got != want {
		t.Errorf("getPlatformAssetName(v1.2.3) = %q, want %q", got, want)
	}

	// 无前缀原样使用
	if got := getPlatformAssetName("2.0.0"); !strings.Contains(got, "team-api-2.0.0-") {
		t.Errorf("getPlatformAssetName(2.0.0) = %q, 应包含 team-api-2.0.0-", got)
	}
}
