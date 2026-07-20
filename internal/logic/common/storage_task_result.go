package common

import (
	"context"
	"strings"

	"github.com/qianfree/team-api/internal/dao"
)

// TaskResultImageURLs 为「同步图片异步化」任务的结果图生成**新鲜**的预览缩略图 URL 与原图 URL。
//
// tsk_model_tasks 不直接存 file_id：这些 re-host 结果图以 original_name = "{public_task_id}_{index}{ext}"
// 落在 fil_files（见 task/sync_image_worker.go rehostImage），据此定位任务主图（index 0，按 id 升序取第一条），
// 再基于文件记录重新签名，因此也顺带规避了任务表里 result_url 24h 预签名过期的问题。
//
// 只对 re-host 到对象存储的图片任务命中；视频/音频任务或上游直链任务（无 fil_files 记录）返回空串，
// 调用方回退到任务表存储的 result_url，保持向后兼容。thumbWidth<=0 时用 600。
//
// tenantID 必须传任务所属租户，确保只在该租户的文件里查找，不跨租户。
func TaskResultImageURLs(ctx context.Context, tenantID int64, publicTaskID string, thumbWidth int) (thumbURL, originalURL string) {
	if publicTaskID == "" || tenantID <= 0 {
		return "", ""
	}

	// 转义 public_task_id 中的下划线，避免 LIKE 的 "_" 通配符误匹配（task_ 前缀本身含下划线）。
	pattern := strings.ReplaceAll(publicTaskID, "_", `\_`) + `\_%`

	var rec *FileRecord
	err := dao.FilFiles.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("original_name LIKE ?", pattern).
		Where("mime_type LIKE ?", "image/%").
		OrderAsc("id").
		Limit(1).
		Scan(&rec)
	if err != nil || rec == nil {
		return "", ""
	}

	svc, err := NewFileServiceFromConfig(ctx)
	if err != nil {
		return "", "" // 对象存储未配置 → 无法生成，回退原 result_url
	}

	if thumbWidth <= 0 {
		thumbWidth = 600
	}
	if u, e := svc.GetThumbnailURL(ctx, rec.ID, thumbWidth); e == nil {
		thumbURL = u
	}
	if u, e := svc.GetDownloadURL(ctx, rec.ID); e == nil {
		originalURL = u
	}
	return thumbURL, originalURL
}
