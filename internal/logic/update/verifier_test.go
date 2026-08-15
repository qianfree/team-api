package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile 在临时目录写入内容并返回路径
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	return path
}

// serveChecksums 启动一个返回指定内容的测试 HTTP 服务
func serveChecksums(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifyChecksum_成功(t *testing.T) {
	content := "fake-binary-content"
	filePath := writeTempFile(t, "team-api-1.0.0-linux-amd64.tar.gz", content)

	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])

	srv := serveChecksums(t, http.StatusOK, hash+"  team-api-1.0.0-linux-amd64.tar.gz\n")

	if err := VerifyChecksum(context.Background(), filePath, srv.URL, "team-api-1.0.0-linux-amd64.tar.gz"); err != nil {
		t.Fatalf("校验应通过，实际失败: %v", err)
	}
}

func TestVerifyChecksum_哈希不一致须报错(t *testing.T) {
	filePath := writeTempFile(t, "asset.tar.gz", "actual-content")

	wrongHash := strings.Repeat("0", 64)
	srv := serveChecksums(t, http.StatusOK, wrongHash+"  asset.tar.gz\n")

	err := VerifyChecksum(context.Background(), filePath, srv.URL, "asset.tar.gz")
	if err == nil {
		t.Fatal("哈希不一致必须返回错误，不允许更新")
	}
	if !strings.Contains(err.Error(), "SHA256 校验不一致") {
		t.Errorf("错误信息应说明校验不一致，实际: %v", err)
	}
}

func TestVerifyChecksum_缺少校验文件地址须拒绝(t *testing.T) {
	filePath := writeTempFile(t, "asset.tar.gz", "content")

	err := VerifyChecksum(context.Background(), filePath, "", "asset.tar.gz")
	if err == nil {
		t.Fatal("无 checksum URL 时必须返回错误，不允许跳过校验放行")
	}
	if !strings.Contains(err.Error(), "未提供") {
		t.Errorf("错误信息应说明缺少校验文件，实际: %v", err)
	}
}

func TestVerifyChecksum_校验文件拉取失败须拒绝(t *testing.T) {
	filePath := writeTempFile(t, "asset.tar.gz", "content")

	// 服务返回 500，模拟校验文件不可用
	srv := serveChecksums(t, http.StatusInternalServerError, "boom")

	err := VerifyChecksum(context.Background(), filePath, srv.URL, "asset.tar.gz")
	if err == nil {
		t.Fatal("校验文件拉取失败时必须返回错误，不允许跳过校验放行")
	}
}

func TestFetchExpectedHash_解析格式(t *testing.T) {
	content := "aaa  file-a.tar.gz\nbbb *file-b.tar.gz\n"
	srv := serveChecksums(t, http.StatusOK, content)
	ctx := context.Background()

	// 标准格式（两个空格分隔）
	if got, err := fetchExpectedHash(ctx, srv.URL, "file-a.tar.gz"); err != nil || got != "aaa" {
		t.Errorf("双空格格式解析失败: got=%q err=%v", got, err)
	}
	// 二进制格式（* 前缀）
	if got, err := fetchExpectedHash(ctx, srv.URL, "file-b.tar.gz"); err != nil || got != "bbb" {
		t.Errorf("* 前缀格式解析失败: got=%q err=%v", got, err)
	}
	// 传入带路径的文件名应按 basename 匹配
	if got, err := fetchExpectedHash(ctx, srv.URL, "/some/dir/file-a.tar.gz"); err != nil || got != "aaa" {
		t.Errorf("basename 匹配失败: got=%q err=%v", got, err)
	}
	// 文件名不存在应报错
	if _, err := fetchExpectedHash(ctx, srv.URL, "not-exist.tar.gz"); err == nil {
		t.Error("校验文件中不存在目标文件时应返回错误")
	}
}

func TestComputeSHA256(t *testing.T) {
	content := "hello team-api"
	filePath := writeTempFile(t, "bin", content)

	sum := sha256.Sum256([]byte(content))
	want := hex.EncodeToString(sum[:])

	got, err := computeSHA256(context.Background(), filePath)
	if err != nil {
		t.Fatalf("计算 SHA256 失败: %v", err)
	}
	if got != want {
		t.Errorf("SHA256 不匹配: got=%s want=%s", got, want)
	}
}
