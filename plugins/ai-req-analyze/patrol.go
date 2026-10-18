package aireqanalyze

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/plugin"
)

// 定时巡查与保留期清理（Cronable 实现，设计文档 5.7 节）。
//
// 巡查 = 对范围内每个租户按回看窗口创建 patrol 任务（复用单元化分析流水线，
// target_user_id=0 全员）。会话跨窗口时 LoadConversationView 以锚点为中心外扩重组，
// 巡查以「会话在窗口内有活动」为准入、分析其完整会话。
//
// patrol_cron 在插件启用时读取注册——修改计划需禁用→启用插件生效（配置项说明已注明）。
// 巡查任务的执行/失败状态落在 plg_ai_req_analyze_jobs（source=patrol），管理后台任务列表可查。

// 默认巡查计划与保留期清理计划（错开分钟，避免整点扎堆）。
const (
	defaultPatrolCron    = "0 3 * * *"
	retentionCleanupCron = "40 4 * * *"
)

// 巡查/清理任务名从 PluginName 派生（框架注册时再加 plugin_ 前缀）。
const (
	patrolJobName    = PluginName + "_patrol"
	retentionJobName = PluginName + "_retention_cleanup"
)

// CronJobs 实现 Cronable 接口。
// 巡查计划声明为默认表达式，实际计划在 Init 时按配置经 Reschedule 校正
// （注册期保持纯函数不触库；修改 patrol_cron 后禁用→启用插件生效）。
func (p *aiReqAnalyzePlugin) CronJobs() []plugin.CronJobDef {
	return []plugin.CronJobDef{
		{
			Name:        patrolJobName,
			DisplayName: "AI请求分析定时巡查",
			CronExpr:    defaultPatrolCron,
			Handler:     p.runPatrol,
			Singleton:   true,
		},
		{
			Name:        retentionJobName,
			DisplayName: "AI请求分析报告保留期清理",
			CronExpr:    retentionCleanupCron,
			Handler:     p.runRetentionCleanup,
			Singleton:   true,
		},
	}
}

// applyPatrolSchedule 按配置校正巡查计划（Init 时调用，有 ctx）。
// Reschedule 对已启动的调度器即时换表达式；未启动时仅更新任务记录，
// StartBackground 会用新值挂载。表达式非法时调度器保持原计划不变。
func applyPatrolSchedule(ctx context.Context) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		return // 配置读取失败沿用默认计划（巡查 handler 内还有开关兜底）
	}
	if cfg.PatrolCron == "" || cfg.PatrolCron == defaultPatrolCron {
		return
	}
	if err := lcommon.GetCronScheduler().Reschedule(ctx, "plugin_"+patrolJobName, cfg.PatrolCron); err != nil {
		glog.Warningf(ctx, "ai-req-analyze: 应用巡查计划失败（沿用默认 %s）: %v", defaultPatrolCron, err)
	}
}

// runPatrol 巡查入口：解析范围 → 逐租户创建 patrol 任务入队。
// 任务自身的成败由 jobrunner 跟踪；这里的失败只影响「本轮没有创建任务」。
func (p *aiReqAnalyzePlugin) runPatrol(ctx context.Context) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze patrol: 读取配置失败: %v", err)
		return
	}
	if !cfg.PatrolEnabled {
		return // 开关关闭，静默跳过（每天空转一次，无成本）
	}
	if err := cfg.Validate(); err != nil {
		glog.Warningf(ctx, "ai-req-analyze patrol: 巡查已开启但分析 AI 未配置完整，跳过本轮: %v", err)
		return
	}

	window := patrolWindow(time.Now(), cfg.PatrolWindowH)
	tenants, err := resolvePatrolTenants(ctx, cfg, window)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze patrol: 解析巡查范围失败: %v", err)
		return
	}
	if len(tenants) == 0 {
		glog.Info(ctx, "ai-req-analyze patrol: 窗口内无活跃租户，跳过本轮")
		return
	}

	created, skipped := 0, 0
	for _, tenantID := range tenants {
		// 上轮巡查任务仍在排队/执行则跳过该租户（长分析 + 短巡查间隔的防堆积）
		busy, err := tenantPatrolBusy(ctx, tenantID)
		if err != nil {
			glog.Warningf(ctx, "ai-req-analyze patrol: 检查租户 %d 在途任务失败: %v", tenantID, err)
			skipped++
			continue
		}
		if busy {
			skipped++
			continue
		}

		jobID, err := g.DB().Model(tableJobs).Ctx(ctx).Data(JobDO{
			TenantId:     tenantID,
			TargetUserId: 0, // 全员
			Source:       SourcePatrol,
			TimeStart:    window.start,
			TimeEnd:      window.end,
			Status:       JobStatusPending,
		}).InsertAndGetId()
		if err != nil {
			glog.Warningf(ctx, "ai-req-analyze patrol: 创建租户 %d 巡查任务失败: %v", tenantID, err)
			skipped++
			continue
		}
		if err := enqueueJob(jobID); err != nil {
			// 队列饱和：本轮停止派发，剩余租户下轮覆盖（窗口仍在，无数据损失）
			failJob(ctx, jobID, err.Error())
			glog.Warningf(ctx, "ai-req-analyze patrol: %v，本轮停止派发（已派 %d 家）", err, created)
			break
		}
		created++
	}

	glog.Infof(ctx, "ai-req-analyze patrol: 本轮派发 %d 家租户（跳过 %d 家），窗口 %s ~ %s",
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
	count, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("source", SourcePatrol).
		WhereIn("status", []string{JobStatusPending, JobStatusRunning}).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// runRetentionCleanup 保留期清理：删除超期报告（连同发现明细）与超期任务记录。
// 子查询批量删除，不逐报告循环；幂等，重复执行无副作用。
func (p *aiReqAnalyzePlugin) runRetentionCleanup(ctx context.Context) {
	cfg, err := LoadConfig(ctx)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze retention: 读取配置失败: %v", err)
		return
	}
	days := cfg.RetentionDays
	if days < 1 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	// 先删发现（子查询定位超期报告），再删报告，最后清超期任务记录
	fRes, err := g.DB().Exec(ctx,
		"DELETE FROM "+tableFindings+" WHERE report_id IN (SELECT id FROM "+tableReports+" WHERE created_at < ?)", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze retention: 清理发现明细失败: %v", err)
		return
	}
	rRes, err := g.DB().Exec(ctx, "DELETE FROM "+tableReports+" WHERE created_at < ?", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze retention: 清理报告失败: %v", err)
		return
	}
	jRes, err := g.DB().Exec(ctx, "DELETE FROM "+tableJobs+" WHERE created_at < ?", cutoff)
	if err != nil {
		glog.Errorf(ctx, "ai-req-analyze retention: 清理任务记录失败: %v", err)
		return
	}

	findingRows, reportRows, jobRows := rowsOf(fRes), rowsOf(rRes), rowsOf(jRes)
	if findingRows+reportRows > 0 {
		glog.Infof(ctx, "ai-req-analyze retention: 清理 %d 份超期报告（%d 条发现、%d 条任务记录），截止 %s",
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
