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
		if cached != nil {
			return cached, nil
		}
	}

	// Call GitHub API
	release, etag, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	if release == nil {
		// 304 Not Modified — use cached result
		cached := getCheckCache(ctx)
		if cached != nil {
			return cached, nil
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

	// Determine if update is available
	var hasUpdate bool
	if consts.Version == "dev" {
		// Dev builds: always report an update if a remote release exists
		hasUpdate = true
	} else {
		currentVer, err := semver.NewVersion(strings.TrimPrefix(consts.Version, "v"))
		if err != nil {
			return nil, fmt.Errorf("invalid current version %q: %w", consts.Version, err)
		}
		latestVer, err := semver.NewVersion(latestStr)
		if err != nil {
			return nil, fmt.Errorf("invalid latest version %q: %w", release.TagName, err)
		}
		hasUpdate = latestVer.GreaterThan(currentVer)
	}

	// Find matching asset for current platform
	assetName := getPlatformAssetName(latestStr)
	var downloadURL, checksumURL string
	var assetSize int64

	for _, asset := range release.Assets {
		if asset.Name == assetName {
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

// fetchLatestRelease calls the GitHub Releases API with conditional request support
func fetchLatestRelease(ctx context.Context) (*GitHubRelease, string, error) {
	url := githubAPIBase + "/releases/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Add ETag for conditional request
	if etagVal := manager.lastETag.Load(); etagVal != nil {
		if etag, ok := etagVal.(string); ok && etag != "" {
			req.Header.Set("If-None-Match", etag)
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

// getCheckCache reads cached check result from Redis
func getCheckCache(ctx context.Context) *CheckResult {
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

// setCheckCache stores check result in Redis
func setCheckCache(ctx context.Context, result *CheckResult) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	ttl := int64(redisCacheTTL.Seconds())
	// TODO: read from config update_check_interval_hours
	g.Redis().Do(ctx, "SETEX", redisCacheKey, ttl, string(data))
}
