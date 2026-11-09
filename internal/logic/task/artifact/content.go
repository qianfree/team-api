package artifact

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/utility/ssrf"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// 回源代理专用 client：不设整体 Timeout（大文件流式转发时长不可预期，生命周期
// 跟随客户端请求 ctx），只限制等待响应头的时间。外链走 SSRF transport，渠道目标
// 与主转发链路同信任级不拦截（与转存下载口径一致）。
var (
	proxyClient = &http.Client{Transport: withHeaderTimeout(ssrf.NewTransport())}
	// 渠道目标与主转发链路同信任级（管理员配置的 baseURL 允许内网中转），不做 SSRF 拦截
	channelProxyClient = &http.Client{Transport: withHeaderTimeout(http.DefaultTransport.(*http.Transport).Clone())}
)

func withHeaderTimeout(t *http.Transport) *http.Transport {
	t.ResponseHeaderTimeout = 60 * time.Second
	return t
}

// artifactRow 产物登记行（内容供给与渲染替换用）。
type artifactRow struct {
	ArtifactKey string `json:"artifact_key"`
	SourceType  string `json:"source_type"`
	SourcePath  string `json:"source_path"`
	SourceUrl   string `json:"source_url"`
	SourceAuth  string `json:"source_auth"`
	StoredKey   string `json:"stored_key"`
	MimeType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	RetryCount  int    `json:"retry_count"`
}

// declPath 声明路径：source_path 优先；存量行（000033 之前）source_url 未被改写为
// URL 时即为路径。
func (r *artifactRow) declPath() string {
	if r.SourcePath != "" {
		return r.SourcePath
	}
	if r.SourceUrl != "" && !strings.HasPrefix(r.SourceUrl, "http://") &&
		!strings.HasPrefix(r.SourceUrl, "https://") && !strings.HasPrefix(r.SourceUrl, "/") {
		return r.SourceUrl
	}
	return ""
}

// loadArtifactRows 任务的产物登记行（task_id + tenant_id 双键限定归属）。
func loadArtifactRows(ctx context.Context, taskID, tenantID int64) ([]*artifactRow, error) {
	var rows []*artifactRow
	err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("task_id", taskID).Where("tenant_id", tenantID).
		OrderAsc("id").Scan(&rows)
	return rows, err
}

// pickArtifactRow 按 key 选产物；key 为空时选主产物：key=video → video/* → 首个。
func pickArtifactRow(rows []*artifactRow, key string) *artifactRow {
	if key != "" {
		for _, row := range rows {
			if row.ArtifactKey == key {
				return row
			}
		}
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	var firstVideo *artifactRow
	for _, row := range rows {
		if row.ArtifactKey == "video" {
			return row
		}
		if firstVideo == nil && strings.HasPrefix(row.MimeType, "video/") {
			firstVideo = row
		}
	}
	if firstVideo != nil {
		return firstVideo
	}
	return rows[0]
}

// Open 打开任务产物内容：已转存（stored）从对象存储读取；未转存（插件关闭转存 /
// 转存未完成或失败）按声明实时回源——url 直链、渠道鉴权端点、inline base64 解码。
// found=false 表示任务无登记产物（非 JS 插件任务，或 key 不存在），调用方自行回落。
func Open(ctx context.Context, task *rcommon.AsyncTask, key string) (*rcommon.StoredArtifact, bool, error) {
	if task == nil || task.ID <= 0 {
		return nil, false, nil
	}
	rows, err := loadArtifactRows(ctx, task.ID, task.TenantID)
	if err != nil {
		return nil, false, fmt.Errorf("查询产物登记失败: %w", err)
	}
	row := pickArtifactRow(rows, key)
	if row == nil {
		return nil, false, nil
	}

	if row.Status == statusStored && row.StoredKey != "" {
		fs, fsErr := acquireFileSvc(ctx)
		if fsErr == nil {
			body, dErr := fs.Provider().Download(ctx, row.StoredKey)
			if dErr == nil {
				return &rcommon.StoredArtifact{Body: body, ContentType: row.MimeType, Size: row.Size}, true, nil
			}
			fsErr = dErr
		}
		// 对象存储不可用/对象丢失：回源兜底（inline 已清空 base64 时回源同样失败，如实报错）
		g.Log().Warningf(ctx, "js artifact: open stored %s failed, fallback to source: %v", row.StoredKey, fsErr)
	}

	artifact, err := openFromSource(ctx, task, row)
	if err != nil {
		return nil, true, err
	}
	return artifact, true, nil
}

// openFromSource 按产物声明回源。
func openFromSource(ctx context.Context, task *rcommon.AsyncTask, row *artifactRow) (*rcommon.StoredArtifact, error) {
	var data map[string]any
	if len(task.Data) > 0 {
		if err := json.Unmarshal(task.Data, &data); err != nil {
			return nil, fmt.Errorf("任务快照解析失败: %w", err)
		}
	}
	path := row.declPath()
	value := ""
	if path != "" && data != nil {
		value, _ = rcommon.GetJSONPath(data, path).(string)
		value = strings.TrimSpace(value)
	}

	switch row.SourceType {
	case "inline":
		if value == "" {
			return nil, fmt.Errorf("inline 产物 %s 内容不可用", row.ArtifactKey)
		}
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("inline base64 解码失败: %w", err)
		}
		return &rcommon.StoredArtifact{
			Body:        io.NopCloser(bytes.NewReader(decoded)),
			ContentType: row.MimeType,
			Size:        int64(len(decoded)),
		}, nil
	case "url":
		rawURL := value
		if rawURL == "" {
			rawURL = row.SourceUrl // 存量行快照已被改写时的兜底
		}
		if rawURL == "" {
			return nil, fmt.Errorf("url 产物 %s 地址不可用", row.ArtifactKey)
		}
		target, err := resolveSource(ctx, task.ChannelID, rawURL, row.SourceAuth)
		if err != nil {
			return nil, err
		}
		client := proxyClient
		if target.FromChannel {
			client = channelProxyClient
		}
		resp, err := doSourceRequest(ctx, client, target, task.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("回源失败: %w", err)
		}
		size := resp.ContentLength
		if size < 0 {
			size = 0
		}
		return &rcommon.StoredArtifact{
			Body:        resp.Body,
			ContentType: firstNonEmpty(row.MimeType, resp.Header.Get("Content-Type")),
			Size:        size,
		}, nil
	default:
		return nil, fmt.Errorf("未知 source type %q", row.SourceType)
	}
}
