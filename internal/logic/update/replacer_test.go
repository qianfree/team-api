package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// createTarball 在临时目录生成包含指定文件的 tar.gz，返回其路径。
// files 的 key 为归档内路径，value 为文件内容。
func createTarball(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "asset.tar.gz")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建 tarball 失败: %v", err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("写 tar header 失败: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("写 tar 内容失败: %v", err)
		}
	}
	return path
}

func TestExtractBinary_正常提取(t *testing.T) {
	tarball := createTarball(t, map[string]string{
		"team-api-1.0.0-linux-amd64/team-api":  "new-binary-content",
		"team-api-1.0.0-linux-amd64/README.md": "docs",
	})

	outPath := filepath.Join(t.TempDir(), "team-api.new")
	got, err := extractBinary(context.Background(), tarball, outPath)
	if err != nil {
		t.Fatalf("提取二进制失败: %v", err)
	}
	if got != outPath {
		t.Errorf("返回路径应为 %s，实际 %s", outPath, got)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("读取提取结果失败: %v", err)
	}
	if string(data) != "new-binary-content" {
		t.Errorf("提取内容不匹配: %q", string(data))
	}

	// 提取出的二进制必须带可执行权限（Windows 不支持 Unix 权限位，跳过）
	if runtime.GOOS != "windows" {
		info, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("stat 提取结果失败: %v", err)
		}
		if info.Mode().Perm()&0100 == 0 {
			t.Errorf("提取的二进制缺少可执行权限: %v", info.Mode().Perm())
		}
	}
}

func TestExtractBinary_归档中无二进制(t *testing.T) {
	tarball := createTarball(t, map[string]string{
		"team-api-1.0.0-linux-amd64/README.md": "docs",
	})

	outPath := filepath.Join(t.TempDir(), "team-api.new")
	if _, err := extractBinary(context.Background(), tarball, outPath); err == nil {
		t.Fatal("归档中不含 team-api 二进制时应返回错误")
	}
}

func TestReplaceBinary_正常替换(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "team-api")
	newBin := filepath.Join(dir, "team-api.new")

	if err := os.WriteFile(current, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBin, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(context.Background(), current, newBin); err != nil {
		t.Fatalf("替换失败: %v", err)
	}

	// 新内容应已就位
	data, _ := os.ReadFile(current)
	if string(data) != "new" {
		t.Errorf("替换后二进制内容应为 new，实际 %q", string(data))
	}
	// 旧版本应被挪到 .old
	oldData, _ := os.ReadFile(current + ".old")
	if string(oldData) != "old" {
		t.Errorf("旧二进制应保留在 .old，实际 %q", string(oldData))
	}
	// 替换后必须可执行（Windows 不支持 Unix 权限位，跳过）
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(current)
		if info.Mode().Perm()&0100 == 0 {
			t.Errorf("替换后的二进制缺少可执行权限: %v", info.Mode().Perm())
		}
	}
}

func TestReplaceBinary_残留old文件被清理(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "team-api")
	newBin := filepath.Join(dir, "team-api.new")

	for path, content := range map[string]string{current: "old", newBin: "new", current + ".old": "stale"} {
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := replaceBinary(context.Background(), current, newBin); err != nil {
		t.Fatalf("替换失败: %v", err)
	}

	oldData, _ := os.ReadFile(current + ".old")
	if string(oldData) != "old" {
		t.Errorf("残留 .old 应被清理并被当前旧版本覆盖，实际 %q", string(oldData))
	}
}

func TestReplaceBinary_失败时自救恢复(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "team-api")

	if err := os.WriteFile(current, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	// newBinary 不存在：第二步 rename 失败，应把 current 从 .old 恢复回来
	missing := filepath.Join(dir, "not-exist.new")
	if err := replaceBinary(context.Background(), current, missing); err == nil {
		t.Fatal("新二进制缺失时替换应返回错误")
	}

	data, _ := os.ReadFile(current)
	if string(data) != "old" {
		t.Errorf("替换失败后当前二进制应恢复原内容，实际 %q", string(data))
	}
	if _, err := os.Stat(current + ".old"); !os.IsNotExist(err) {
		t.Error("自救恢复后 .old 不应残留")
	}
}

func TestCopyFile_内容与权限一致(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("binary-bytes"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("复制失败: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "binary-bytes" {
		t.Errorf("复制内容不匹配: %q", string(data))
	}
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("复制后权限不一致: src=%v dst=%v", srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}

func TestCopyFile_源文件不存在(t *testing.T) {
	if err := copyFile(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "dst")); err == nil {
		t.Fatal("源文件不存在时应返回错误")
	}
}
