package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
)

const (
	githubRepo    = "qianfree/team-api"
	githubAPIBase = "https://api.github.com/repos/" + githubRepo
	redisCacheKey = "update:check_result"
	redisCacheTTL = 24 * time.Hour // default TTL, overridden by config
)

// CheckForUpdate checks GitHub Releases for a newer version
func CheckForUpdate(ctx context.Context, force bool) (*CheckResult, error) {
	// Try Redis cache first (unless force)
	if !force {
		cached := getCheckCache(ctx)
		// 缓存里的 current_version 与当前二进制版本不一致时（通常发生在刚升级后），
		// 视为失效并 fallthrough 重算，避免旧缓存里的 has_update 在"相同版本"下误报更新
		if cached != nil && cached.CurrentVersion == consts.Version {
			return cached, nil
		}
	}

	// Call GitHub API（force 时不带 ETag 条件请求，见 fetchLatestRelease 注释）
	release, etag, err := fetchLatestRelease(ctx, !force)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	if release == nil {
		// 304 Not Modified — GitHub 最新发行版未变化，复用缓存的发行版信息
		cached := getCheckCache(ctx)
		if cached != nil {
			// 二进制版本与缓存一致：直接返回缓存
			if cached.CurrentVersion == consts.Version {
				return cached, nil
			}
			// 二进制刚升级（缓存里仍是旧版本）：基于当前版本重算 hasUpdate 并覆盖缓存，
			// 否则旧缓存里 has_update=true 会让小圆点在"相同版本"下也误亮
			refreshed := *cached
			refreshed.CurrentVersion = consts.Version
			hasUpdate, cmpErr := compareVersions(consts.Version, cached.LatestVersion)
			if cmpErr != nil {
				// latest_version 无法解析（理论上不会发生，缓存写入时已校验），保守沿用旧值
				hasUpdate = cached.HasUpdate
			}
			refreshed.HasUpdate = hasUpdate
			refreshed.CheckedAt = gtime.Now()
			setCheckCache(ctx, &refreshed)
			manager.checkResult.Store(&refreshed)
			return &refreshed, nil
		}
		return &CheckResult{
			CurrentVersion: consts.Version,
			LatestVersion:  consts.Version,
			HasUpdate:      false,
			CheckedAt:      gtime.Now(),
			DeploymentMode: GetDeploymentMode(),
		}, nil
	}

	latestStr := strings.TrimPrefix(release.TagName, "v")

	// 判断是否有更新：dev 构建或无法解析的当前版本保守按"有更新"处理，避免漏报
	hasUpdate, err := compareVersions(consts.Version, latestStr)
	if err != nil {
		return nil, err
	}

	// Find matching asset for current platform
	assetName := getPlatformAssetName(latestStr)
	var downloadURL, checksumURL string
	var assetSize int64

	for _, asset := range release.Assets {
		// 资产名可能带/不带 v 前缀（历史 release 命名不一致），归一化后比较
		if normalizeAssetName(asset.Name) == assetName {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
		}
		if asset.Name == "checksums-sha256.txt" {
			checksumURL = asset.BrowserDownloadURL
		}
	}

	result := &CheckResult{
		CurrentVersion: consts.Version,
		LatestVersion:  latestStr,
		HasUpdate:      hasUpdate,
		ReleaseNotes:   release.Body,
		ReleaseURL:     release.HTMLURL,
		PublishedAt:    release.PublishedAt,
		CheckedAt:      gtime.Now(),
		DeploymentMode: GetDeploymentMode(),
		DownloadURL:    downloadURL,
		ChecksumURL:    checksumURL,
		AssetSize:      assetSize,
	}

	// Store ETag for conditional requests
	manager.lastETag.Store(etag)

	// Cache result
	setCheckCache(ctx, result)
	manager.checkResult.Store(result)

	return result, nil
}

// updateAPIBase 返回更新检查的 API 基地址。默认 GitHub 官方 API；
// 可通过配置 update.api_base 覆盖（自建发布服务器 / 本地测试 / 国内镜像）。
func updateAPIBase(ctx context.Context) string {
	if base := g.Cfg().MustGet(ctx, "update.api_base").String(); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	return githubAPIBase
}

// fetchLatestRelease calls the GitHub Releases API.
// conditional 为 true 时携带 If-None-Match ETag 做条件请求（省配额）；
// 强制检测（force）必须传 false：GitHub /releases/latest 有 CDN 缓存，刚发布新版本后
// 条件请求可能返回过期的 304 Not Modified，导致强制检测仍拿不到最新发行版。
func fetchLatestRelease(ctx context.Context, conditional bool) (*GitHubRelease, string, error) {
	url := updateAPIBase(ctx) + "/releases/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Add ETag for conditional request
	if conditional {
		if etagVal := manager.lastETag.Load(); etagVal != nil {
			if etag, ok := etagVal.(string); ok && etag != "" {
				req.Header.Set("If-None-Match", etag)
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	etag := resp.Header.Get("ETag")

	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, nil // 304 — no change
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, etag, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, etag, fmt.Errorf("failed to decode release: %w", err)
	}

	// Skip drafts and prereleases
	if release.Draft || release.Prerelease {
		return nil, etag, nil
	}

	return &release, etag, nil
}

// getCheckCache reads cached check result from Redis.
// 缓存是纯优化（TTL 24h），Redis 未配置/不可用时应降级为无缓存、走实时检查，
// 绝不能因 g.Redis() 客户端创建失败而 panic（见 goframe-conventions.md 已修复记录）。
func getCheckCache(ctx context.Context) (out *CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Warningf(ctx, "update check cache read skipped: %v", r)
			out = nil
		}
	}()

	val, err := g.Redis().Do(ctx, "GET", redisCacheKey)
	if err != nil || val.IsNil() || val.IsEmpty() {
		return nil
	}

	var result CheckResult
	if err := json.Unmarshal(val.Bytes(), &result); err != nil {
		return nil
	}
	return &result
}

// setCheckCache stores check result in Redis. 与 getCheckCache 同理，失败仅降级不 panic。
func setCheckCache(ctx context.Context, result *CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			g.Log().Warningf(ctx, "update check cache write skipped: %v", r)
		}
	}()

	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	ttl := int64(redisCacheTTL.Seconds())
	// TODO: read from config update_check_interval_hours
	g.Redis().Do(ctx, "SETEX", redisCacheKey, ttl, string(data))
}

// normalizeSemver 将构建期注入的版本字符串规整为合法 semver。
// Makefile 通过 git describe 生成的版本形如 "0.2.0_39"（v0.2.0 之后 39 个提交），
// 其中的下划线在 semver 中非法，此处转为构建元数据 "+"（"0.2.0+39"），
// 比较时与基础版本等价，不会误报更新。
func normalizeSemver(v string) string {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '_'); i > 0 {
		v = v[:i] + "+" + v[i+1:]
	}
	return v
}

// compareVersions 判断 currentVersion 是否落后于 latestVersion（即是否有新版本可用）。
// 判定规则与历史行为保持一致：
//   - dev 构建，或当前版本无法解析为合法 semver 时，保守返回 (true, nil)：
//     只要有远端发行版即提示更新，宁可误报也不漏报。
//   - latestVersion 无法解析时返回错误（正常情况下缓存写入时已校验过），
//     调用方决定降级策略。
func compareVersions(currentVersion, latestVersion string) (bool, error) {
	if currentVersion == "dev" {
		return true, nil
	}
	currentVer, err := semver.NewVersion(normalizeSemver(currentVersion))
	if err != nil {
		// 当前版本无法解析，按开发构建处理
		return true, nil
	}
	latestVer, err := semver.NewVersion(strings.TrimPrefix(latestVersion, "v"))
	if err != nil {
		return false, fmt.Errorf("invalid latest version %q: %w", latestVersion, err)
	}
	return latestVer.GreaterThan(currentVer), nil
}
