package aianalysis

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// 定时巡查与保留期清理（由 internal/cmd 的 registerCronJobs 注册）。
//
// 巡查 = 对范围内每个租户按回看窗口创建 patrol 任务（复用单元化分析流水线，
// target_user_id=0 全员）。会话跨窗口时 LoadConversationView 以锚点为中心外扩重组，
// 巡查以「会话在窗口内有活动」为准入、分析其完整会话。
//
// 巡查计划（ai_analysis_patrol_cron）保存后经设置变更监听即时重排生效
//（参照渠道自动探测间隔的热更新模式，见 cmd.go 的 registerCronJobs）。
// 巡查任务的执行/失败状态落在 aud_ai_analysis_jobs（source=patrol），管理后台任务列表可查。

// 默认巡查计划与保留期清理计划（错开分钟，避免整点扎堆）。
const (
	DefaultPatrolCron    = "0 3 * * *"
	RetentionCleanupCron = "40 4 * * *"
)

// 巡查/清理任务名（全局唯一，用作分布式锁 key 与 sys_cron_jobs 执行记录主键）。
const (
	PatrolJobName    = "ai_analysis_patrol"
	RetentionJobName = "ai_analysis_retention_cleanup"
)

// PatrolCronExpr 读取巡查计划配置（缺省/为空回落默认表达式），供注册与热更新重排。
func PatrolCronExpr(ctx context.Context) string {
	if expr := lcommon.Config().GetString(ctx, "ai_analysis_patrol_cron"); expr != "" {
		return expr
	}
	return DefaultPatrolCron
}

// RunPatrol 巡查入口：解析范围 → 逐租户创建 patrol 任务入队。
// 任务自身的成败由 jobrunner 跟踪；这里的失败只影响「本轮没有创建任务」。
func RunPatrol(ctx context.Context) {
	cfg := LoadConfig(ctx)
	if !cfg.PatrolEnabled {
		return // 开关关闭，静默跳过（每天空转一次，无成本）
	}
	if err := cfg.Validate(); err != nil {
		glog.Warningf(ctx, "ai_analysis patrol: 巡查已开启但分析 AI 未配置完整，跳过本轮: %v", err)
		return
	}

	window := patrolWindow(time.Now(), cfg.PatrolWindowH)
	tenants, err := resolvePatrolTenants(ctx, cfg, window)
	if err != nil {
		glog.Errorf(ctx, "ai_analysis patrol: 解析巡查范围失败: %v", err)
		return
	}
	if len(tenants) == 0 {
		glog.Info(ctx, "ai_analysis patrol: 窗口内无活跃租户，跳过本轮")
		return
	}

	created, skipped := 0, 0
	for _, tenantID := range tenants {
		// 上轮巡查任务仍在排队/执行则跳过该租户（长分析 + 短巡查间隔的防堆积）
		busy, err := tenantPatrolBusy(ctx, tenantID)
		if err != nil {
			glog.Warningf(ctx, "ai_analysis patrol: 检查租户 %d 在途任务失败: %v", tenantID, err)
			skipped++
			continue
		}
		if busy {
			skipped++
			continue
		}

		jobID, err := g.DB().Model(TableJobs).Ctx(ctx).Data(JobDO{
			TenantId:     tenantID,
			TargetUserId: 0, // 全员
			Source:       SourcePatrol,
			TimeStart:    window.start,
			TimeEnd:      window.end,
			Status:       JobStatusPending,
		}).InsertAndGetId()
		if err != nil {
			glog.Warningf(ctx, "ai_analysis patrol: 创建租户 %d 巡查任务失败: %v", tenantID, err)
			skipped++
			continue
		}
		if err := EnqueueJob(jobID); err != nil {
			// 队列饱和：本轮停止派发，剩余租户下轮覆盖（窗口仍在，无数据损失）
			FailJob(ctx, jobID, err.Error())
			glog.Warningf(ctx, "ai_analysis patrol: %v，本轮停止派发（已派 %d 家）", err, created)
			break
		}
		created++
	}

	glog.Infof(ctx, "ai_analysis patrol: 本轮派发 %d 家租户（跳过 %d 家），窗口 %s ~ %s",
		created, skipped,
		window.start.Format("2006-01-02 15:04"), window.end.Format("2006-01-02 15:04"))
}

// patrolWindowParam 巡查窗口。
type patrolWindowParam struct {
	start time.Time
	end   time.Time
}

// patrolWindow 计算回看窗口：[now - windowH, now]。
func patrolWindow(now time.Time, windowH int) patrolWindowParam {
	if windowH < 1 {
		windowH = 24
	}
	return patrolWindowParam{start: now.Add(-time.Duration(windowH) * time.Hour), end: now}
}

// resolvePatrolTenants 解析巡查租户列表：
// 配置了 patrol_scope 用指定列表（过滤不存在的租户）；
// 否则取窗口内在审计日志中有 API 调用的租户（活跃 = 有调用，且天然排除审计关闭的租户）。
func resolvePatrolTenants(ctx context.Context, cfg *Config, window patrolWindowParam) ([]int64, error) {
	if len(cfg.PatrolScope) > 0 {
		all, err := g.DB().Model("tnt_tenants").Ctx(ctx).
			WhereIn("id", cfg.PatrolScope).
			Fields("id").All()
		if err != nil {
			return nil, err
		}
		out := make([]int64, 0, len(all))
		for _, r := range all {
			out = append(out, r["id"].Int64())
		}
		return out, nil
	}

	// 窗口内 DISTINCT tenant_id：BRIN(created_at) 支撑范围扫描，日频巡一次可接受
	all, err := lcommon.AuditModelCtx(ctx, "aud_request_logs").
		WhereBetween("created_at", window.start, window.end).
		Fields("DISTINCT tenant_id AS tenant_id").
		Order("tenant_id ASC").
		All()
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(all))
	for _, r := range all {
		if id := r["tenant_id"].Int64(); id > 0 {
			out = append(out, id)
		}
	}
	return out, nil
}

// tenantPatrolBusy 租户是否有排队/执行中的巡查任务。
func tenantPatrolBusy(ctx context.Context, tenantID int64) (bool, error) {
	count, err := g.DB().Model(TableJobs).Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("source", SourcePatrol).
		WhereIn("status", []string{JobStatusPending, JobStatusRunning}).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RunRetentionCleanup 保留期清理：删除超期报告（连同发现明细）与超期任务记录。
// 子查询批量删除，不逐报告循环；幂等，重复执行无副作用。
func RunRetentionCleanup(ctx context.Context) {
	cfg := LoadConfig(ctx)
	days := cfg.RetentionDays
	if days < 1 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	// 先删发现（子查询定位超期报告），再删报告，最后清超期任务记录
	fRes, err := g.DB().Exec(ctx,
		"DELETE FROM "+TableFindings+" WHERE report_id IN (SELECT id FROM "+TableReports+" WHERE created_at < ?)", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai_analysis retention: 清理发现明细失败: %v", err)
		return
	}
	rRes, err := g.DB().Exec(ctx, "DELETE FROM "+TableReports+" WHERE created_at < ?", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai_analysis retention: 清理报告失败: %v", err)
		return
	}
	jRes, err := g.DB().Exec(ctx, "DELETE FROM "+TableJobs+" WHERE created_at < ?", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai_analysis retention: 清理任务记录失败: %v", err)
		return
	}

	findingRows, reportRows, jobRows := rowsOf(fRes), rowsOf(rRes), rowsOf(jRes)
	if findingRows+reportRows > 0 {
		glog.Infof(ctx, "ai_analysis retention: 清理 %d 份超期报告（%d 条发现、%d 条任务记录），截止 %s",
			reportRows, findingRows, jobRows, cutoff.Format("2006-01-02"))
	}
}

// rowsOf 安全取受影响行数。
func rowsOf(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
