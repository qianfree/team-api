package update

import (
	"sync"
	"sync/atomic"

	"github.com/gogf/gf/v2/os/gtime"
)

// Phase constants for update progress
const (
	PhaseIdle        = ""
	PhaseDownloading = "downloading"
	PhaseVerifying   = "verifying"
	PhaseBackingUp   = "backing_up"
	PhaseReplacing   = "replacing"
	PhaseRestarting  = "restarting"
	PhaseComplete    = "complete"
	PhaseFailed      = "failed"
)

// Deployment modes
const (
	DeploymentBinary = "binary"
	DeploymentDocker = "docker"
)

// GitHubRelease represents a GitHub release from the API
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt *gtime.Time   `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
	Prerelease  bool          `json:"prerelease"`
	Draft       bool          `json:"draft"`
}

// GitHubAsset represents a single asset in a GitHub release
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckResult holds the result of a version check
type CheckResult struct {
	CurrentVersion string      `json:"current_version"`
	LatestVersion  string      `json:"latest_version"`
	HasUpdate      bool        `json:"has_update"`
	ReleaseNotes   string      `json:"release_notes"`
	ReleaseURL     string      `json:"release_url"`
	PublishedAt    *gtime.Time `json:"published_at,omitempty"`
	CheckedAt      *gtime.Time `json:"checked_at"`
	DeploymentMode string      `json:"deployment_mode"`
	DownloadURL    string      `json:"download_url"` // matched asset URL
	ChecksumURL    string      `json:"checksum_url"` // checksums-sha256.txt URL
	AssetSize      int64       `json:"asset_size"`
}

// Progress holds the current update progress
type Progress struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Percentage int    `json:"percentage"`
	Error      string `json:"error,omitempty"`
}

// RollbackInfo holds information needed for rollback (persisted to disk)
type RollbackInfo struct {
	BackupPath    string `json:"backup_path"`
	OriginalPath  string `json:"original_path"`
	BackupVersion string `json:"backup_version"`
	Timestamp     string `json:"timestamp"`
}

// UpdateManager is the singleton that manages update state
type UpdateManager struct {
	status      atomic.Value // *Status
	updating    atomic.Bool  // update lock
	mu          sync.Mutex   // serialize update operations
	checkResult atomic.Value // *CheckResult
	lastETag    atomic.Value // string - GitHub API ETag for conditional requests
}

// Status holds the full update status
type Status struct {
	CurrentVersion    string       `json:"current_version"`
	DeploymentMode    string       `json:"deployment_mode"`
	Updating          bool         `json:"updating"`
	LastCheck         *CheckResult `json:"last_check,omitempty"`
	Progress          *Progress    `json:"progress,omitempty"`
	RollbackAvailable bool         `json:"rollback_available"`
	BackupVersion     string       `json:"backup_version,omitempty"`
}
