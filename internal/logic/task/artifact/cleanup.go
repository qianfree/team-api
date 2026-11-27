// 产物保留期清理：js_artifact_retention_days 控制的存储对象与登记行回收。
//
// 与 admin 包的文件保留期清理（exports/、image/*）互补——JS 插件产物以 video/mp4 等
// 非 image MIME 为主，两条既有清理链都不覆盖，本地降级存储（data/files）与 OSS 同样
// 只增不减。行为约定：
//   - stored 行：先经 fil_files 行删对象（FileService.Delete：对象+行），成功后标记
//     expired；对象删除失败保留 stored 等下轮重试（S3/OSS/COS 删除不存在对象均成功，
//     失败即真实瞬时/权限问题，值得重试）。
//   - 其余状态（direct/pending/running/failed）：无对象可删，到期直接批量标记 expired
//     （顺带终止 sweep 对超龄行的无谓重试）。
//   - 标记 expired 而非删行：内容端点据此返回 410 Gone（区别于回源兜底的含混失败），
//     行保留也保住转存统计（jsplugin 指标）与排查现场。
//
// 保留期以 created_at 计（行建于任务终态，≈产物诞生时刻）——不用 updated_at，避免
// failed 行每次重试刷新时钟导致永不过期。
package artifact

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const cleanupBatchSize = 200

// retentionDaysFn 保留期天数（测试可替换；<=0 表示关闭）。
var retentionDaysFn = func(ctx context.Context) int {
	return lcommon.Config().GetInt(ctx, "js_artifact_retention_days")
}

// CleanupExpiredArtifacts 清理过保留期的 JS 插件产物，返回标记 expired 的行数。
// 由 js_artifact_cleanup cron 与管理后台手动清理入口调用。存储服务不可用时跳过
// stored 行的对象删除（避免标记 expired 后对象成为永久孤儿），仅完成无对象行的标记。
func CleanupExpiredArtifacts(ctx context.Context) (int, error) {
	days := retentionDaysFn(ctx)
	if days <= 0 {
		return 0, nil
	}
	cutoff := nowFn().AddDate(0, 0, -days)

	total, err := expireNonStored(ctx, cutoff)
	if err != nil {
		g.Log().Warningf(ctx, "js artifact cleanup: expire non-stored rows: %v", err)
	}

	fs, fsErr := acquireFileSvc(ctx)
	if fsErr != nil {
		// 不返回硬错误：无对象行已处理，stored 行留给下一轮（cron 每日重跑）
		g.Log().Warningf(ctx, "js artifact cleanup: storage unavailable, stored rows deferred: %v", fsErr)
		return total, nil
	}
	stored, err := expireStored(ctx, fs, cutoff)
	total += stored
	if err != nil {
		return total, err
	}
	if total > 0 {
		g.Log().Infof(ctx, "js artifact cleanup: expired %d artifacts (before %s)", total, cutoff.Format("2006-01-02"))
	}
	return total, nil
}

// expireNonStored 批量标记无对象状态（direct/pending/running/failed）的到期行。
func expireNonStored(ctx context.Context, cutoff time.Time) (int, error) {
	result, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("status IN (?)", g.Slice{statusDirect, statusPending, "running", "failed"}).
		Where("created_at < ?", cutoff).
		Data(g.Map{"status": statusExpired, "updated_at": nowFn()}).
		Update()
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// expireStored 分批处理 stored 到期行：删对象成功即标记 expired。
// 整批零进展（删除持续失败）时停止，避免同一批被反复重查形成热循环。
func expireStored(ctx context.Context, fs *lcommon.FileService, cutoff time.Time) (int, error) {
	total := 0
	for {
		var batch []struct {
			ID           int64  `json:"id"`
			PublicTaskID string `json:"public_task_id"`
			ArtifactKey  string `json:"artifact_key"`
			StoredKey    string `json:"stored_key"`
		}
		if err := g.DB().Model(artifactTable).Ctx(ctx).
			Where("status = ?", statusStored).
			Where("created_at < ?", cutoff).
			Fields("id, public_task_id, artifact_key, stored_key").
			OrderAsc("id").Limit(cleanupBatchSize).Scan(&batch); err != nil {
			return total, err
		}
		if len(batch) == 0 {
			return total, nil
		}
		progress := 0
		for _, row := range batch {
			if err := deleteStoredObject(ctx, fs, row.StoredKey); err != nil {
				g.Log().Warningf(ctx, "js artifact cleanup: delete object of task %s/%s failed: %v",
					row.PublicTaskID, row.ArtifactKey, err)
				continue
			}
			markArtifactExpired(ctx, row.ID)
			total++
			progress++
		}
		if progress == 0 {
			g.Log().Warningf(ctx, "js artifact cleanup: no progress in batch, stopping (check storage)")
			return total, nil
		}
		if len(batch) < cleanupBatchSize {
			return total, nil
		}
	}
}

// deleteStoredObject 删除转存对象：优先经 fil_files 行走 FileService.Delete（对象+行
// 一致清理）；行缺失（历史上传未落行/已被单独删除）时按 key 直接删对象兜底，防止
// 对象因失去 DB 索引而永久泄漏。persist 落库的是 raw key，与 js_task_artifacts.stored_key
// 同源，可按 storage_path 精确匹配。
func deleteStoredObject(ctx context.Context, fs *lcommon.FileService, storedKey string) error {
	if storedKey == "" {
		return nil // 防御：stored 行必有 key，空值视为无对象
	}
	ids, err := g.DB().Model("fil_files").Ctx(ctx).
		Where("storage_path", storedKey).Fields("id").Array()
	if err != nil {
		return fmt.Errorf("定位 fil_files 行失败: %w", err)
	}
	if len(ids) == 0 {
		return fs.Provider().Delete(ctx, storedKey)
	}
	for _, id := range ids {
		// tenantID=0：清理是跨租户的管理动作（与 admin 保留期清理口径一致）
		if err := fs.Delete(ctx, id.Int64(), 0); err != nil {
			return fmt.Errorf("删除文件记录 %d 失败: %w", id.Int64(), err)
		}
	}
	return nil
}

// markArtifactExpired 对象清理完成后标记行（行更新失败仅告警：对象删除幂等，
// 下轮重删不产生副作用）。
func markArtifactExpired(ctx context.Context, id int64) {
	if _, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("id", id).
		Data(g.Map{"status": statusExpired, "updated_at": nowFn()}).
		Update(); err != nil {
		g.Log().Warningf(ctx, "js artifact cleanup: mark row %d expired: %v", id, err)
	}
}
