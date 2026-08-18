package update

import "testing"

// TestNormalizeAssetName 校验资产名归一化：兼容历史上带 v 前缀的资产命名。
func TestNormalizeAssetName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"team-api-0.2.0-linux-amd64.tar.gz", "team-api-0.2.0-linux-amd64.tar.gz"},  // 规范名：不变
		{"team-api-v0.2.0-linux-amd64.tar.gz", "team-api-0.2.0-linux-amd64.tar.gz"}, // 历史带 v：去掉
		{"team-api-v1.2.3-darwin-arm64.zip", "team-api-1.2.3-darwin-arm64.zip"},     // Windows/Darwin zip 同理
		{"checksums-sha256.txt", "checksums-sha256.txt"},                            // 非资产名：不变
	}
	for _, c := range cases {
		if got := normalizeAssetName(c.in); got != c.want {
			t.Fatalf("normalizeAssetName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
