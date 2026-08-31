package admin

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ============================================================
// 工作台（运营收件箱）
//
// 核心约定：待办**不落库**，每次都从各业务源表实时派生。
// 因此「归零」是自动的 —— 工单被回复、告警被确认、渠道 Key 换好之后，
// 下一轮收集自然就查不到它了，不需要任何人来点「完成」，也不存在
// 「源表已修好但工作台还挂着」的残留。待办的唯一出路就是去源头解决。
//
// 收集器约定：
//   1. 每个 collector 独立失败不影响其他项 —— 任何一个查询报错只记 warning 并返回空，
//      绝不让整个工作台因为某张表出问题而白屏（工作台本身就是故障时最该能打开的页面）。
//   2. 不做全表实时 count：高频写入表（bil_usage_logs / aud_* ）一律限定时间窗 + 走
//      既有索引，整份结果进 Redis 缓存 30s，菜单红点与页面共用同一份缓存。
// ============================================================

const (
	// workbenchCacheKey 整份收集结果的缓存键。菜单红点轮询与页面刷新共用，
	// 避免每个管理员每 30s 各打一遍库。
	workbenchCacheKey = "workbench:summary:v1"
	workbenchCacheTTL = 30 * time.Second

	// 各项判定阈值。选值理由见对应 collector 注释。
	wbFrozenStaleHours    = 2    // 冻结超过 2 小时未释放视为异常
	wbOrderUnfulfilledMin = 15   // 已支付超过 15 分钟仍未履约视为卡单
	wbOAuthExpiryHours    = 24   // OAuth 令牌 24 小时内过期即预警
	wbHealthDropPoints    = 30   // 健康分 1 小时内跌幅超过 30 分视为骤降
	wbEmailFailThreshold  = 5    // 1 小时内邮件失败超过 5 封视为链路异常
	wbPlanExpiringDays    = 7    // 套餐 7 天内到期
	wbTicketSLAHours      = 2    // 工单首次响应 SLA
	wbWalletDriftRatio    = 0.02 // Redis↔DB 余额偏差超过 2% 且绝对额可观时告警

	// wbCronStallFallbackInterval 停摆判定的调度周期兜底值：任务不在注册表中
	// （改名/下线后的遗留行）或 cron 表达式解析失败时按「日任务」保守处理。
	wbCronStallFallbackInterval = 24 * time.Hour
)

// wbItem 内部待办结构：比 API 的 v1.WorkbenchItem 多一个 perm 字段。
// 工作台跨域聚合，必须按权限逐条过滤 —— 只有 support:view 的客服不该在工作台
// 看到钱包偏差和订单金额，否则等于绕过 RBAC 泄露财务数据。
type wbItem struct {
	v1.WorkbenchItem
	Perm string `json:"perm"`
}

// wbCollected 一次收集的完整结果（进缓存的就是它）。
type wbCollected struct {
	Items       []wbItem                 `json:"items"`
	Models      []v1.WorkbenchModelAvail `json:"models"`
	Breakers    []v1.WorkbenchBreaker    `json:"breakers"`
	Metrics     []v1.WorkbenchMetric     `json:"metrics"`
	GeneratedAt string                   `json:"generated_at"`
}

// ============================================================
// 对外接口
// ============================================================

// GetWorkbenchSummary 工作台汇总。
func (s *sAdmin) GetWorkbenchSummary(ctx context.Context, _ *v1.AdminWorkbenchSummaryReq) (*v1.AdminWorkbenchSummaryRes, error) {
	col := s.collectWorkbenchCached(ctx)

	items := s.filterWorkbenchItems(ctx, col.Items)

	// 分域计数基于过滤后的结果 —— 计数必须与用户实际看得到的列表一致，
	// 否则会出现「显示 3 条但列表只有 1 条」的诡异体验。
	domainStats := make([]v1.WorkbenchDomainStat, 0, 4)
	for _, d := range []string{
		v1.WorkbenchDomainAvailability, v1.WorkbenchDomainMoney,
		v1.WorkbenchDomainCustomer, v1.WorkbenchDomainSystem,
	} {
		st := v1.WorkbenchDomainStat{Domain: d}
		for _, it := range items {
			if it.Domain != d {
				continue
			}
			st.Total++
			if it.Severity == v1.WorkbenchSeverityP0 {
				st.Urgent++
			}
		}
		domainStats = append(domainStats, st)
	}

	out := make([]v1.WorkbenchItem, 0, len(items))
	for _, it := range items {
		out = append(out, it.WorkbenchItem)
	}

	res := &v1.AdminWorkbenchSummaryRes{
		Metrics:     col.Metrics,
		Items:       out,
		Domains:     domainStats,
		Models:      col.Models,
		Breakers:    col.Breakers,
		GeneratedAt: col.GeneratedAt,
	}
	if res.Metrics == nil {
		res.Metrics = []v1.WorkbenchMetric{}
	}
	if res.Models == nil {
		res.Models = []v1.WorkbenchModelAvail{}
	}
	if res.Breakers == nil {
		res.Breakers = []v1.WorkbenchBreaker{}
	}
	return res, nil
}

// GetWorkbenchBadges 工作台菜单角标计数（与 summary 共用缓存，不额外压库）。
//
// 角标只挂工作台菜单一项：待办的排查线索只存在于工作台的描述文案里，
// 业务菜单里没有对应的定位入口，往各业务菜单挂数字只会带来
// 「进去了却找不到问题」的困惑（红点引路却无路可走）。
func (s *sAdmin) GetWorkbenchBadges(ctx context.Context, _ *v1.AdminWorkbenchBadgeReq) (*v1.AdminWorkbenchBadgeRes, error) {
	col := s.collectWorkbenchCached(ctx)
	items := s.filterWorkbenchItems(ctx, col.Items)

	res := &v1.AdminWorkbenchBadgeRes{}
	for _, it := range items {
		res.Total++
		if it.Severity == v1.WorkbenchSeverityP0 {
			res.Urgent++
		}
	}
	return res, nil
}

// ============================================================
// 收集与过滤
// ============================================================

// collectWorkbenchCached 读缓存，未命中则重新收集。
// 缓存失败一律降级为直接收集 —— 工作台不能因为 Redis 抖动就打不开。
func (s *sAdmin) collectWorkbenchCached(ctx context.Context) *wbCollected {
	if v, err := g.Redis().Get(ctx, workbenchCacheKey); err == nil && !v.IsEmpty() {
		var col wbCollected
		if json.Unmarshal(v.Bytes(), &col) == nil {
			return &col
		}
	}

	col := s.collectWorkbench(ctx)

	if b, err := json.Marshal(col); err == nil {
		if _, err := g.Redis().Do(ctx, "SET", workbenchCacheKey, b, "EX", int(workbenchCacheTTL.Seconds())); err != nil {
			g.Log().Warningf(ctx, "workbench: 写缓存失败: %v", err)
		}
	}
	return col
}

// filterWorkbenchItems 按当前管理员权限逐条过滤，再按严重度排序。
func (s *sAdmin) filterWorkbenchItems(ctx context.Context, items []wbItem) []wbItem {
	userID := common.GetCtxUserID(ctx)
	role, _ := ctx.Value("role").(string)

	// 权限判定结果按 perm 记忆化：一次汇总最多 20 条待办但只有 ~10 种权限点，
	// 逐条查库会把一次页面刷新放大成几十次 SQL。
	permCache := make(map[string]bool, 12)
	allowed := func(perm string) bool {
		if perm == "" {
			return true
		}
		if v, ok := permCache[perm]; ok {
			return v
		}
		v := HasPermission(ctx, userID, role, perm)
		permCache[perm] = v
		return v
	}

	out := make([]wbItem, 0, len(items))
	for _, it := range items {
		if !allowed(it.Perm) {
			continue
		}
		out = append(out, it)
	}

	sevRank := map[string]int{
		v1.WorkbenchSeverityP0: 0,
		v1.WorkbenchSeverityP1: 1,
		v1.WorkbenchSeverityP2: 2,
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := sevRank[out[i].Severity], sevRank[out[j].Severity]
		if ri != rj {
			return ri < rj
		}
		// 同 severity 下新发生的排前面；聚合类（OccurredAt 为空）排最后
		if out[i].OccurredAt != out[j].OccurredAt {
			if out[i].OccurredAt == "" {
				return false
			}
			if out[j].OccurredAt == "" {
				return true
			}
			return out[i].OccurredAt > out[j].OccurredAt
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// collectWorkbench 依次跑各 collector。单个失败不影响整体。
func (s *sAdmin) collectWorkbench(ctx context.Context) *wbCollected {
	col := &wbCollected{GeneratedAt: gtime.Now().Format(time.RFC3339)}

	models, breakers, availItems := s.wbCollectAvailability(ctx)
	col.Models = models
	col.Breakers = breakers

	col.Items = append(col.Items, availItems...)
	col.Items = append(col.Items, s.wbCollectMoney(ctx)...)
	col.Items = append(col.Items, s.wbCollectCustomer(ctx)...)
	col.Items = append(col.Items, s.wbCollectSystem(ctx)...)

	col.Metrics = s.wbCollectMetrics(ctx)
	return col
}

// wbNew 构造一条待办。
func wbNew(key, sev, domain, perm, title, desc, actionText, route string, query map[string]string, at *gtime.Time) wbItem {
	it := wbItem{Perm: perm}
	it.Key = key
	it.Severity = sev
	it.Domain = domain
	it.Title = title
	it.Desc = desc
	it.ActionText = actionText
	it.ActionRoute = route
	it.ActionQuery = query
	if at != nil {
		it.OccurredAt = at.Format(time.RFC3339)
	}
	return it
}
