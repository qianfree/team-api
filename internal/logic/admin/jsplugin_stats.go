package admin

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/internal/logic/billing"
	relaycommon "github.com/qianfree/team-api/relay/common"
)

// JS 插件运营统计：任务侧（tsk_model_tasks 按 platform=js:{key} 聚合）与
// 营收侧（bil_usage_logs 经 active manifest 的模型映射聚合）各自成查询，
// 纯逻辑（platform×status 行 → 每插件聚合、模型映射、组装）拆成无 DB 依赖
// 的函数供单测。

// jsOpsStatsTimeLayout 统一展示时间格式（与管理后台其余接口一致）。
const jsOpsStatsTimeLayout = "2006-01-02 15:04:05"

// normalizeOpsStatsDays 统计窗口天数兜底：默认 30，范围 1-365。
// v 注解（between:1,365）已在入口拦截非法值，这里再兜一层防御零值/注解被绕过的调用路径。
func normalizeOpsStatsDays(days int) int {
	if days < 1 || days > 365 {
		return 30
	}
	return days
}

// jsOpsStatsTaskRow tsk_model_tasks 聚合查询单行（platform × status 分组）。
type jsOpsStatsTaskRow struct {
	Platform string
	Status   string
	Count    int64
	LastAt   time.Time // 该分组 MAX(created_at)，零值 = 无
}

// jsOpsStatsTaskAgg 单插件任务侧聚合结果。
type jsOpsStatsTaskAgg struct {
	Total      int64
	Success    int64
	Failed     int64
	Processing int64     // 非终态数（SUBMITTED/IN_PROGRESS/QUEUED/NOT_START 等）
	LastTaskAt time.Time // 窗口内最近一次任务时间，零值 = 无任务
}

// aggregateJsOpsStatsTasks 纯聚合：platform×status 计数行 → 每插件任务统计。
// platform 只认 "js:{key}"（SQL 已过滤 LIKE 'js:%'，这里再防御非 js 前缀行）；
// 终态判定用 relay/common 状态常量（TaskStatusSuccess/TaskStatusFailure），
// 不自造字符串比较；未知非终态状态一律计入 Processing（口径：非终态即处理中）。
func aggregateJsOpsStatsTasks(rows []jsOpsStatsTaskRow) map[string]*jsOpsStatsTaskAgg {
	out := make(map[string]*jsOpsStatsTaskAgg)
	for _, row := range rows {
		key, ok := jsPlatformPluginKey(row.Platform)
		if !ok {
			continue
		}
		agg := out[key]
		if agg == nil {
			agg = &jsOpsStatsTaskAgg{}
			out[key] = agg
		}
		agg.Total += row.Count
		switch relaycommon.TaskStatusEnum(row.Status) {
		case relaycommon.TaskStatusSuccess:
			agg.Success += row.Count
		case relaycommon.TaskStatusFailure:
			agg.Failed += row.Count
		default:
			agg.Processing += row.Count
		}
		if row.LastAt.After(agg.LastTaskAt) {
			agg.LastTaskAt = row.LastAt
		}
	}
	return out
}

// jsPlatformPluginKey 解析 platform（"js:{key}"）为插件 key；非 js 前缀或空 key 返回 false。
func jsPlatformPluginKey(platform string) (string, bool) {
	if !strings.HasPrefix(platform, "js:") {
		return "", false
	}
	key := strings.TrimPrefix(platform, "js:")
	if key == "" {
		return "", false
	}
	return key, true
}

// jsOpsStatsRevenueRow 营收聚合查询单行（按任务 platform 分组，金额为 decimal 精确值）。
type jsOpsStatsRevenueRow struct {
	Platform string
	Cost     decimal.Decimal
}

// mergeJsOpsStatsRevenue 纯归并：platform 级营收行 → 按插件 decimal 累加。
// 归属按任务 platform（js:{key}）而非模型名——上游模型清单跨插件可重名，同一平台
// 模型可能由多家插件服务，只有任务落库时记下的插件才能准确归属。
// 累加全程 decimal，出口才经 billing.InexactFloat64 转 float（JSON number 契约）。
func mergeJsOpsStatsRevenue(rows []jsOpsStatsRevenueRow) map[string]decimal.Decimal {
	out := make(map[string]decimal.Decimal)
	for _, row := range rows {
		key, ok := jsPlatformPluginKey(row.Platform)
		if !ok {
			continue
		}
		out[key] = billing.AddMoney(out[key], row.Cost)
	}
	return out
}

// jsOpsStatsSuccessRate 成功率口径：终态任务中的成功占比 success/(success+failed)。
// 分母为 0（无终态任务——全部进行中或零任务）时置 0 而非 NaN，取值 0-1。
func jsOpsStatsSuccessRate(success, failed int64) float64 {
	denom := success + failed
	if denom <= 0 {
		return 0
	}
	return float64(success) / float64(denom)
}

// jsOpsStatsAssembly 组装入参（全部纯数据，无 DB/全局依赖，可单测）。
type jsOpsStatsAssembly struct {
	Manifests []*contract.Manifest // active manifest 全集（含禁用插件）
	InService map[string]bool      // 当前 generation 内的插件 key（enabled = 在服务）
	Tasks     map[string]*jsOpsStatsTaskAgg
	Revenue   map[string]decimal.Decimal
}

// buildJsOpsStatsItems 纯组装：结果 = manifest 声明的插件 ∪ 有任务记录的 platform。
// 无任务的插件也列出（零值行）；platform 有记录但 manifest 已不存在的列为孤儿行
// （name 置 key、orphan=true，数据完备性优先）。排序：营收降序、同营收按 key 升序。
func buildJsOpsStatsItems(a jsOpsStatsAssembly) []v1.JSPluginOpsStatsItem {
	items := make([]v1.JSPluginOpsStatsItem, 0, len(a.Manifests)+len(a.Tasks))
	seen := make(map[string]bool, len(a.Manifests)+len(a.Tasks))
	for _, m := range a.Manifests {
		if m == nil || m.Key == "" {
			continue
		}
		seen[m.Key] = true
		items = append(items, buildJsOpsStatsItem(m.Key, m.Name, a, false))
	}
	for key := range a.Tasks {
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, buildJsOpsStatsItem(key, key, a, true))
	}
	// 排序比较用出口 float 值：等值 decimal 经 InexactFloat64 转换结果恒等，
	// 不影响金额本身（金额计算全程 decimal，排序仅决定展示顺序）
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Revenue != items[j].Revenue {
			return items[i].Revenue > items[j].Revenue
		}
		return items[i].PluginKey < items[j].PluginKey
	})
	return items
}

// buildJsOpsStatsItem 组装单插件统计行（name 为空时回落 key）。
func buildJsOpsStatsItem(key, name string, a jsOpsStatsAssembly, orphan bool) v1.JSPluginOpsStatsItem {
	item := v1.JSPluginOpsStatsItem{
		PluginKey: key,
		Name:      name,
		Enabled:   a.InService[key],
		Orphan:    orphan,
	}
	if name == "" {
		item.Name = key
	}
	if agg := a.Tasks[key]; agg != nil {
		item.TasksTotal = agg.Total
		item.TasksSuccess = agg.Success
		item.TasksFailed = agg.Failed
		item.TasksProcessing = agg.Processing
		item.SuccessRate = jsOpsStatsSuccessRate(agg.Success, agg.Failed)
		if !agg.LastTaskAt.IsZero() {
			item.LastTaskTime = agg.LastTaskAt.Format(jsOpsStatsTimeLayout)
		}
	}
	if rev, ok := a.Revenue[key]; ok {
		item.Revenue = billing.InexactFloat64(rev)
	}
	return item
}

// JSPluginOpsStats JS 插件运营统计（GET /api/admin/js-plugins/ops-stats?days=30）。
// 任务侧与营收侧两条独立聚合查询后在内存组装（组装逻辑见 buildJsOpsStatsItems，
// 保证「无任务插件」与「manifest 已不存在的孤儿 platform」都出现在结果里）。
func (s *sAdmin) JSPluginOpsStats(ctx context.Context, req *v1.JSPluginOpsStatsReq) (*v1.JSPluginOpsStatsRes, error) {
	days := normalizeOpsStatsDays(req.Days)
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02") + " 00:00:00"

	// 模型映射源：全部 active manifest（含禁用插件——禁用期间/历史营收也要能归到插件）
	manifests, err := jsplugin.DefaultRegistry().ActiveManifests(ctx)
	if err != nil {
		return nil, err
	}

	// 任务侧聚合：platform LIKE 'js:%' + 时间窗，按 platform×status 分组计数并取
	// 每组 MAX(created_at)（跨 status 组在内存取最大，等价于按 platform 取最近任务时间）
	taskRows := make([]struct {
		Platform string      `json:"platform"`
		Status   string      `json:"status"`
		Cnt      int64       `json:"cnt"`
		LastAt   *gtime.Time `json:"last_at"`
	}, 0)
	if err := dao.TskModelTasks.Ctx(ctx).
		Where("platform LIKE ?", "js:%").
		Where("created_at >= ?", cutoff).
		Fields("platform, status, COUNT(*) AS cnt, MAX(created_at) AS last_at").
		Group("platform, status").
		Scan(&taskRows); err != nil {
		return nil, gerror.Wrap(err, "统计 JS 插件任务量失败")
	}
	pureTaskRows := make([]jsOpsStatsTaskRow, 0, len(taskRows))
	for _, r := range taskRows {
		row := jsOpsStatsTaskRow{Platform: r.Platform, Status: r.Status, Count: r.Cnt}
		if r.LastAt != nil {
			row.LastAt = r.LastAt.Time
		}
		pureTaskRows = append(pureTaskRows, row)
	}
	tasks := aggregateJsOpsStatsTasks(pureTaskRows)

	// 营收侧聚合：任务用量日志按 request_id 关联任务行，按任务 platform（js:{key}）
	// 分组——同名模型多插件服务时只有任务归属能准确拆分。bil_usage_logs 是全系统
	// 最大写入表，聚合必须带 created_at 时间窗（BRIN 索引）；实扣优先口径与仪表盘/
	// 用量汇总一致（actual_cost 优先，0/NULL 回退 total_cost）。
	revRows := make([]struct {
		Platform string          `json:"platform"`
		Cost     decimal.Decimal `json:"cost"`
	}, 0)
	if err := g.DB().Ctx(ctx).Raw(`
SELECT t.platform AS platform,
       COALESCE(SUM(COALESCE(NULLIF(u.actual_cost, 0), u.total_cost)), 0) AS cost
FROM bil_usage_logs u
JOIN tsk_model_tasks t ON t.request_id = u.request_id
WHERE u.created_at >= ? AND u.request_type = 3 AND t.platform LIKE 'js:%'
GROUP BY t.platform`, cutoff).Scan(&revRows); err != nil {
		return nil, gerror.Wrap(err, "统计 JS 插件营收失败")
	}
	pureRevRows := make([]jsOpsStatsRevenueRow, 0, len(revRows))
	for _, r := range revRows {
		pureRevRows = append(pureRevRows, jsOpsStatsRevenueRow{Platform: r.Platform, Cost: r.Cost})
	}
	revenue := mergeJsOpsStatsRevenue(pureRevRows)

	// enabled 口径：当前是否在服务 = 存在于运行中 generation（active+enabled 才会装载）
	gen := jsplugin.CurrentGeneration()
	inService := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		if _, ok := gen.Plugin(m.Key); ok {
			inService[m.Key] = true
		}
	}

	items := buildJsOpsStatsItems(jsOpsStatsAssembly{
		Manifests: manifests,
		InService: inService,
		Tasks:     tasks,
		Revenue:   revenue,
	})
	return &v1.JSPluginOpsStatsRes{List: items}, nil
}
