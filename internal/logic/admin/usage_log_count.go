package admin

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// 用量日志分页 total 的统计策略：
//
// bil_usage_logs 是全系统写入量最大的分区表，明细精确 COUNT 的代价与筛选范围内
// 行数成正比，时间范围一宽，每翻一页都要重数一遍。这里按天拆段求和：
//   - 完整天：bil_usage_daily 日汇总 SUM(request_count)，O(天数) 而非 O(行数)；
//   - 头尾不完整天：明细表精确 COUNT，保持与列表完全一致的时间边界。
//
// 口径说明（与列表对齐的两个截断）：
//   1. 自动清理（usage_log_cleanup）只删 bil_usage_logs 明细、不删日汇总，
//      超过保留期的完整天在日汇总里仍保留计数而行已删——完整天窗口按保留期
//      截断，被清掉的老区间不再计数（行已删，列表同样查不出）。
//   2. ID/用户/Key/请求类型无法从日粒度汇总还原，出现即整体回退精确 COUNT。
// 保留期内日汇总与明细仍可能因手动按维度清理产生少量偏差，分页 total 允许该近似。
//
// 历史覆盖：日汇总起点（MIN(stat_date)）早于查询下界时才可信；未回填的缺口段
// 落入头段走明细 COUNT 补齐（正确但不加速），空表整体回退精确口径。

// usageLogCountPlan 日汇总计数的拆段计划（纯计算产物，便于单测）
type usageLogCountPlan struct {
	fallback bool // true = 无法安全使用日汇总，整体回退明细精确 COUNT

	dailyFrom time.Time // 完整天窗口下界（含，当天 00:00）
	dailyTo   time.Time // 完整天窗口上界（含，当天 00:00）

	headFrom *time.Time // 头段下界（闭，nil=无下界）；与 dailyFrom 之间为开区间
	headTo   *time.Time // 头段上界（开）
	hasHead  bool

	tailFrom *time.Time // 尾段下界（闭）
	tailTo   *time.Time // 尾段上界（闭，nil=无上界）
	hasTail  bool
}

// usageLogCountDailyServable 判断筛选条件能否映射到 bil_usage_daily 的聚合维度
// （租户/渠道/模型/状态）。日汇总以天为粒度，ID/用户/Key/请求类型/上游请求ID无法还原。
func usageLogCountDailyServable(f v1.AdminUsageLogFilter) bool {
	return f.ID == 0 && f.UserID == 0 && f.ApiKeyID == 0 && f.RequestType == 0 && f.UpstreamRequestId == ""
}

// parseUsageLogRange 解析筛选时间边界（与 buildUsageLogFilter 同口径：
// 仅日期补齐当天边界，带时间原样使用），任一侧缺省返回 nil。格式异常返回 ok=false。
func parseUsageLogRange(f v1.AdminUsageLogFilter) (startTs, endTs *time.Time, ok bool) {
	parse := func(value, expanded string) (*time.Time, bool) {
		if value == "" {
			return nil, true
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", expanded, time.Local)
		if err != nil {
			return nil, false
		}
		return &t, true
	}
	var valid bool
	if startTs, valid = parse(f.StartDate, common.StartOfRange(f.StartDate)); !valid {
		return nil, nil, false
	}
	if endTs, valid = parse(f.EndDate, common.EndOfRange(f.EndDate)); !valid {
		return nil, nil, false
	}
	return startTs, endTs, true
}

// buildUsageLogCountPlan 计算日汇总计数的拆段计划（纯函数，不访问数据库）。
// 完整天 D 的判定：D 00:00 >= start 且 D 次日 00:00 <= end；日汇总只聚合到
// 昨天（cron 每天 01:00 重算最近 3 个完整天，当天进行中的数据不聚合），
// 因此上界无论如何不超过昨天，当天与结束日的不完整部分落在尾段走明细。
func buildUsageLogCountPlan(startTs, endTs *time.Time, now time.Time, retentionDays int, minStat time.Time) usageLogCountPlan {
	dayStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	todayStart := dayStart(now)

	// 完整天窗口下界：起点当天 00:00 起算，起点带非零时刻则从次日起
	lower := time.Time{}
	if startTs != nil {
		lower = dayStart(*startTs)
		if !startTs.Equal(lower) {
			lower = lower.AddDate(0, 0, 1)
		}
	}
	// 完整天窗口上界：min(昨天, 结束时间能完整覆盖的最后一天)
	upper := todayStart.AddDate(0, 0, -1)
	if endTs != nil {
		if e := dayStart(endTs.AddDate(0, 0, -1)); e.Before(upper) {
			upper = e
		}
	}
	// 保留期截断：超期明细已被清理而日汇总仍保留计数，截掉避免多算
	if retentionDays > 0 {
		if retainFrom := todayStart.AddDate(0, 0, -retentionDays); retainFrom.After(lower) {
			lower = retainFrom
		}
	}
	// 日汇总覆盖下界：历史未回填到 lower 时缺口段走明细（头段）补齐
	if minStat.After(lower) {
		lower = minStat
	}

	if lower.After(upper) {
		return usageLogCountPlan{fallback: true}
	}

	plan := usageLogCountPlan{
		dailyFrom: lower,
		dailyTo:   upper,
	}
	// 头段 [start, dailyFrom 00:00)：开区间上界，避免与日汇总重叠
	plan.hasHead = startTs == nil || startTs.Before(lower)
	plan.headFrom = startTs
	headTo := lower
	plan.headTo = &headTo
	// 尾段 [dailyTo 次日 00:00, end]：闭区间，覆盖当天/结束日的进行中数据
	tailFrom := upper.AddDate(0, 0, 1)
	plan.tailFrom = &tailFrom
	plan.tailTo = endTs
	plan.hasTail = endTs == nil || !endTs.Before(tailFrom)
	return plan
}

// usageLogDimConds 构建可映射到日汇总维度的筛选条件（租户/渠道/模型/状态），
// 返回不带 WHERE 前缀的条件串，供明细计数与日汇总查询拼接复用。
func usageLogDimConds(f v1.AdminUsageLogFilter) (string, []any) {
	var conds []string
	var args []any
	if f.TenantID > 0 {
		conds = append(conds, "tenant_id = ?")
		args = append(args, f.TenantID)
	}
	if f.ChannelID > 0 {
		conds = append(conds, "channel_id = ?")
		args = append(args, f.ChannelID)
	}
	if f.Model != "" {
		conds = append(conds, "model_name = ?")
		args = append(args, f.Model)
	}
	if f.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, f.Status)
	}
	return strings.Join(conds, " AND "), args
}

// countUsageLogsExact 对明细表做与列表口径完全一致的精确 COUNT（含全部筛选维度）
func countUsageLogsExact(ctx context.Context, f v1.AdminUsageLogFilter) (int, error) {
	where, args := buildUsageLogFilter(f)
	val, err := g.DB().GetValue(ctx, "SELECT COUNT(*) FROM bil_usage_logs u"+where, args...)
	if err != nil {
		return 0, err
	}
	return int(val.Int64()), nil
}

// usageLogDailyMinStatDate 读取日汇总最早统计日期，用于判断历史覆盖。
// stat_date 上有索引、表又极小，MIN 是首条索引探测；空表返回 ok=false。
func usageLogDailyMinStatDate(ctx context.Context) (time.Time, bool, error) {
	val, err := g.DB().GetValue(ctx, "SELECT MIN(stat_date) FROM bil_usage_daily")
	if err != nil {
		return time.Time{}, false, err
	}
	if val.IsNil() {
		return time.Time{}, false, nil
	}
	t := val.Time()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()), true, nil
}

// countUsageLogsRange 对明细表按维度条件 + 时间窗 COUNT。
// from 闭（created_at >= from，nil=无下界）；toExclusive 为 true 时 to 为开区间
// （created_at < to），否则闭区间（created_at <= to）。
func countUsageLogsRange(ctx context.Context, f v1.AdminUsageLogFilter, from, to *time.Time, toExclusive bool) (int64, error) {
	dimConds, args := usageLogDimConds(f)
	var conds []string
	if dimConds != "" {
		conds = append(conds, dimConds)
	}
	if from != nil {
		conds = append(conds, "created_at >= ?")
		args = append(args, *from)
	}
	if to != nil {
		if toExclusive {
			conds = append(conds, "created_at < ?")
		} else {
			conds = append(conds, "created_at <= ?")
		}
		args = append(args, *to)
	}
	sqlStr := "SELECT COUNT(*) FROM bil_usage_logs"
	if len(conds) > 0 {
		sqlStr += " WHERE " + strings.Join(conds, " AND ")
	}
	val, err := g.DB().GetValue(ctx, sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return val.Int64(), nil
}

// countUsageLogs 统计用量日志筛选结果总数（管理后台分页 total）。
// 不可映射维度 / 时间参数异常 / 短窗口（≤2 天，拆段无收益）一律走精确 COUNT，
// 与原口径完全一致；大范围才进入日汇总拆段路径。
func countUsageLogs(ctx context.Context, f v1.AdminUsageLogFilter) (int, error) {
	if !usageLogCountDailyServable(f) {
		return countUsageLogsExact(ctx, f)
	}
	startTs, endTs, ok := parseUsageLogRange(f)
	if !ok {
		return countUsageLogsExact(ctx, f)
	}
	if startTs != nil && endTs != nil && endTs.Sub(*startTs) <= 48*time.Hour {
		return countUsageLogsExact(ctx, f)
	}

	// 日汇总覆盖检查：空表或读取失败均回退精确口径
	minStat, hasDaily, err := usageLogDailyMinStatDate(ctx)
	if err != nil || !hasDaily {
		return countUsageLogsExact(ctx, f)
	}

	retentionDays := common.Config().GetInt(ctx, "usage_log_retention_days")
	if retentionDays == 0 {
		retentionDays = 90
	}
	plan := buildUsageLogCountPlan(startTs, endTs, time.Now(), retentionDays, minStat)
	if plan.fallback {
		return countUsageLogsExact(ctx, f)
	}

	var total int64
	// 头段：不完整起始天（或无下界时的保留期前零头）
	if plan.hasHead {
		n, err := countUsageLogsRange(ctx, f, plan.headFrom, plan.headTo, true)
		if err != nil {
			return 0, err
		}
		total += n
	}
	// 完整天：日汇总求和（维度条件与明细列一一对应）
	dailySQL := "SELECT COALESCE(SUM(request_count), 0) FROM bil_usage_daily WHERE stat_date >= ? AND stat_date <= ?"
	dailyArgs := []any{plan.dailyFrom.Format("2006-01-02"), plan.dailyTo.Format("2006-01-02")}
	if dimConds, dimArgs := usageLogDimConds(f); dimConds != "" {
		dailySQL += " AND " + dimConds
		dailyArgs = append(dailyArgs, dimArgs...)
	}
	val, err := g.DB().GetValue(ctx, dailySQL, dailyArgs...)
	if err != nil {
		return 0, err
	}
	total += val.Int64()
	// 尾段：当天及结束日的不完整部分
	if plan.hasTail {
		n, err := countUsageLogsRange(ctx, f, plan.tailFrom, plan.tailTo, false)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return int(total), nil
}
