package update

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/gctx"
)

// buildReleaseTarGz 构造符合 getPlatformAssetName 命名的 release 资产 tar.gz，
// 内部结构 {assetDir}/team-api（与 extractBinary 的查找约定一致）。
func buildReleaseTarGz(t *testing.T, dir, version string, content []byte) string {
	t.Helper()
	assetDir := fmt.Sprintf("team-api-%s-%s-%s", version, runtime.GOOS, runtime.GOARCH)
	path := filepath.Join(dir, assetDir+".tar.gz")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	_ = tw.WriteHeader(&tar.Header{Name: assetDir + "/", Typeflag: tar.TypeDir, Mode: 0755})
	if err := tw.WriteHeader(&tar.Header{
		Name: assetDir + "/team-api", Typeflag: tar.TypeReg,
		Size: int64(len(content)), Mode: 0755,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

// startMockReleaseServer 启动一个模拟 GitHub release 的本地 HTTP 服务：
// /releases/latest 返回版本信息，/assets/ 下发 tar.gz 与 checksums。
// 返回服务地址与工作目录（含资产文件）。
func startMockReleaseServer(t *testing.T, version string) (*httptest.Server, string) {
	t.Helper()
	workDir := t.TempDir()
	tarPath := buildReleaseTarGz(t, workDir, version, []byte("FAKE-NEW-BINARY-"+version))
	assetName := filepath.Base(tarPath)
	checksums := fmt.Sprintf("%s  %s\n", sha256File(t, tarPath), assetName)

	var tsURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tag_name":"v%s","name":"v%s",`+
			`"assets":[{"name":%q,"browser_download_url":%q,"size":%d},`+
			`{"name":"checksums-sha256.txt","browser_download_url":%q}]}`,
			version, version,
			assetName, tsURL+"/assets/"+assetName, fileSize(t, tarPath),
			tsURL+"/assets/checksums-sha256.txt")
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/assets/") {
		case assetName:
			http.ServeFile(w, r, tarPath)
		case "checksums-sha256.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	})

	ts := httptest.NewServer(mux)
	tsURL = ts.URL
	t.Cleanup(ts.Close)
	return ts, workDir
}

// TestUpdatePipeline_Local 在本地用模拟 release 服务器驱动生产流水线
// （CheckForUpdate → stageUpdate：下载→校验→备份→替换→写标记），
// 验证整个更新链路的磁盘产物，无需 GitHub / PostgreSQL / Redis。
func TestUpdatePipeline_Local(t *testing.T) {
	ctx := gctx.New()
	version := "0.9.0" // 模拟的"新版本"，必须高于当前版本才判定有更新

	ts, _ := startMockReleaseServer(t, version)

	// 模拟"当前二进制"：stageUpdate 会把它备份/替换（executablePath seam 指向这里，
	// 避免真实操作测试进程自身的可执行文件）
	work := t.TempDir()
	currentBin := filepath.Join(work, "team-api")
	if err := os.WriteFile(currentBin, []byte("FAKE-CURRENT-0.2.0"), 0755); err != nil {
		t.Fatal(err)
	}

	origExec := executablePath
	executablePath = func() (string, error) { return currentBin, nil }
	t.Cleanup(func() { executablePath = origExec })

	// 更新目录指向临时目录，避免污染全局 /tmp/team-api-update
	origUpdateDir := updateDir
	updateDir = t.TempDir()
	t.Cleanup(func() { updateDir = origUpdateDir })

	// 更新源指向本地 mock。Redis 缓存未配置时已被 getCheckCache/setCheckCache 防御性
	// recover 降级，无需在测试里配置 Redis。
	adapter, err := gcfg.NewAdapterContent(fmt.Sprintf("update:\n  api_base: %s\n", ts.URL))
	if err != nil {
		t.Fatal(err)
	}
	gcfg.Instance().SetAdapter(adapter)

	// 1) 版本检查应命中 mock：判定有更新，下载地址指向本地
	res, err := CheckForUpdate(ctx, true)
	if err != nil {
		t.Fatalf("CheckForUpdate failed: %v", err)
	}
	if !res.HasUpdate {
		t.Fatalf("expected HasUpdate=true, got false (current=%q latest=%q)", res.CurrentVersion, res.LatestVersion)
	}
	if !strings.HasPrefix(res.DownloadURL, ts.URL) {
		t.Fatalf("DownloadURL %q does not point at local mock %s", res.DownloadURL, ts.URL)
	}
	if res.LatestVersion != version {
		t.Fatalf("LatestVersion=%q, want %q", res.LatestVersion, version)
	}

	// 2) 驱动真实生产流水线（stageUpdate，不触发进程重启）。
	// stageUpdate 内部会 EvalSymlinks（如 macOS 上 /var → /private/var），
	// 返回值是解析后的绝对路径，断言前同样解析。
	resolvedBin, err := filepath.EvalSymlinks(currentBin)
	if err != nil {
		t.Fatal(err)
	}
	newExe, err := stageUpdate(ctx, version, res.DownloadURL, res.ChecksumURL, res.AssetSize)
	if err != nil {
		t.Fatalf("stageUpdate failed: %v", err)
	}
	if newExe != resolvedBin {
		t.Fatalf("stageUpdate returned %q, want %q", newExe, resolvedBin)
	}

	// 3) 校验磁盘产物
	got, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "FAKE-NEW-BINARY-"+version {
		t.Fatalf("current binary not replaced, content=%q", string(got))
	}
	if _, err := os.Stat(currentBin + ".old"); err != nil {
		t.Fatalf("expected .old file: %v", err)
	}

	entries, _ := os.ReadDir(work)
	hasBackup := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "team-api.backup.") {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Fatal("expected a team-api.backup.* file")
	}

	if _, err := os.Stat(filepath.Join(updateDir, rollbackFile)); err != nil {
		t.Fatalf("rollback.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(updateDir, pendingVerificationFile)); err != nil {
		t.Fatalf("pending_verification not written: %v", err)
	}

	// 4) 待重启的新版本路径已记录（供退出链 exec 换壳）
	if got := manager.ConsumeRestartBinary(); got != resolvedBin {
		t.Fatalf("restart binary = %q, want %q", got, resolvedBin)
	}
}
