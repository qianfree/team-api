package admin

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"golang.org/x/sync/singleflight"

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
	// workbenchCacheTTL 新鲜期：结果超过这个年龄即视为陈旧，触发后台刷新。
	workbenchCacheTTL = 30 * time.Second
	// workbenchCacheHardTTL Redis 上的物理 TTL，纯兜底：即使进程在后台刷新途中挂掉，
	// 陈旧结果也不会被无限期当成有效数据返回。
	workbenchCacheHardTTL = 10 * time.Minute
	// workbenchCollectTimeout 一次完整收集的独立超时。收集发生在请求路径之外
	// （后台刷新），必须有自带的截止时间，不能挂在请求上下文上。
	workbenchCollectTimeout = 30 * time.Second

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

// workbenchGroup 合并并发回源的 singleflight 组。
// 缓存为空（首次启动 / Redis 丢数据）时，角标轮询与页面刷新会同时到达，
// 没有它就会 N 个请求各跑一遍完整收集，把冷启动放大成 N 倍的库压力。
var workbenchGroup singleflight.Group

// workbenchRefreshing 标记「已有后台刷新在跑」。
// 陈旧命中时每个请求都想刷新，用 CAS 保证全局同时只有一次刷新。
var workbenchRefreshing atomic.Bool

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
	// GeneratedAtUnix 缓存新鲜度的判定依据（Unix 秒）。
	// 刻意不复用 GeneratedAt：那个字符串由 gtime 格式化后对外展示，格式细节
	// 不该被缓存逻辑依赖；这里用明确的时间戳，解析歧义为零。
	GeneratedAtUnix int64 `json:"generated_at_unix"`
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

// collectWorkbenchCached 读缓存；命中即返回，陈旧则「先返回旧值 + 后台刷新」。
//
// 三层保护，针对菜单角标每 30s 轮询这一固定负载：
//  1. 新鲜命中：直接返回，零 DB 开销；
//  2. 陈旧命中：立即返回旧值，同时触发一次后台刷新（stale-while-revalidate）——
//     工作台是故障时最该能打开的页面，宁可给一份 30 秒前的数据，也不要让轮询
//     把请求堵在十几条联表查询上；
//  3. 完全未命中（首次启动 / Redis 丢数据）：同步收集，由 singleflight 保证
//     并发请求只触发一次。
//
// 缓存读写失败一律降级为直接收集 —— 工作台不能因为 Redis 抖动就打不开。
func (s *sAdmin) collectWorkbenchCached(ctx context.Context) *wbCollected {
	if col, ok := readWorkbenchCache(ctx); ok {
		if wbCacheFresh(col) {
			return col
		}
		s.refreshWorkbenchAsync(ctx)
		return col
	}
	return s.collectWorkbenchSync(ctx)
}

// readWorkbenchCache 读并解析整份收集结果。任何一步失败都当作未命中。
func readWorkbenchCache(ctx context.Context) (*wbCollected, bool) {
	v, err := g.Redis().Get(ctx, workbenchCacheKey)
	if err != nil || v.IsEmpty() {
		return nil, false
	}
	var col wbCollected
	if json.Unmarshal(v.Bytes(), &col) != nil {
		return nil, false
	}
	return &col, true
}

// wbCacheFresh 判断缓存是否还在新鲜期内。
// GeneratedAtUnix 缺失（例如版本升级后读到旧格式缓存）时判为陈旧：那会触发
// 恰好一次刷新把字段补上，之后自愈，不会形成刷新循环。
func wbCacheFresh(col *wbCollected) bool {
	if col.GeneratedAtUnix <= 0 {
		return false
	}
	return time.Since(time.Unix(col.GeneratedAtUnix, 0)) < workbenchCacheTTL
}

// collectWorkbenchSync 冷启动路径：同步收集，singleflight 保证并发只跑一次。
func (s *sAdmin) collectWorkbenchSync(ctx context.Context) *wbCollected {
	v, _, _ := workbenchGroup.Do(workbenchCacheKey, func() (any, error) {
		// 双检：等锁期间别的请求可能已经把缓存写好了
		if col, ok := readWorkbenchCache(ctx); ok {
			return col, nil
		}
		return s.collectWorkbenchAndCache(ctx), nil
	})
	if col, ok := v.(*wbCollected); ok {
		return col
	}
	// 理论不可达（Do 恒返回 *wbCollected）；兜底直接收集，保证工作台永远有数据
	return s.collectWorkbenchAndCache(ctx)
}

// refreshWorkbenchAsync 后台刷新一次缓存。
// 全局只允许一个刷新在跑：陈旧期间所有轮询请求都会走到这里，不去重等于把
// 「每个请求各刷一次」原样搬到后台。
func (s *sAdmin) refreshWorkbenchAsync(ctx context.Context) {
	if !workbenchRefreshing.CompareAndSwap(false, true) {
		return
	}
	// 脱离请求生命周期：请求返回后刷新必须继续，否则角标轮询一结束刷新就被取消，
	// 缓存永远等不到更新。值（trace id 等）保留，取消信号丢弃。
	bg := context.WithoutCancel(ctx)
	go func() {
		defer workbenchRefreshing.Store(false)
		// 后台刷新跑在裸 goroutine 上，没有 HTTP 服务的 panic 恢复兜底 ——
		// 任一 collector 空指针都不能把整个进程带走，落一条日志就够了。
		// defer 顺序：本函数先注册，后执行，确保 recover 之后标志位一定被清掉。
		defer func() {
			if e := recover(); e != nil {
				g.Log().Errorf(bg, "workbench: 后台刷新 panic: %v", e)
			}
		}()
		s.collectWorkbenchAndCache(bg)
	}()
}

// collectWorkbenchAndCache 收集一次并写回缓存。
//
// 收集统一跑在「脱离请求生命周期 + 自带超时」的上下文上：结果会写进全实例共享的
// 缓存，不能被某个客户端断连中断 —— 否则一份半截结果（最坏是「全站无待办」）
// 会被当成有效数据缓存下来。
func (s *sAdmin) collectWorkbenchAndCache(ctx context.Context) *wbCollected {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), workbenchCollectTimeout)
	defer cancel()

	col := s.collectWorkbench(rctx)

	if b, err := json.Marshal(col); err == nil {
		if _, err := g.Redis().Do(rctx, "SET", workbenchCacheKey, b, "EX", int(workbenchCacheHardTTL.Seconds())); err != nil {
			g.Log().Warningf(rctx, "workbench: 写缓存失败: %v", err)
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

// collectWorkbench 并发跑各 collector。单个失败不影响整体。
//
// 四个域 + 首屏指标之间没有任何数据依赖，串行执行只会把一次工作台刷新累加成
// 「十几条联表查询耗时之和」。并发后总耗时≈最慢的那一条（通常是资金域 ——
// 它要对最多 30 个租户逐个读 Redis）。
//
// wg.Wait() 返回后才合并结果，各 goroutine 之间没有共享写入；合并顺序固定为
// 可用性 → 资金 → 客户 → 系统，保证同样的数据产出稳定的 items 顺序。
//
// 用 WaitGroup 而非 errgroup：collector 全部自带兜底（失败只记 warning 并返回空），
// 这里不存在可返回的错误。
func (s *sAdmin) collectWorkbench(ctx context.Context) *wbCollected {
	now := gtime.Now()
	col := &wbCollected{
		GeneratedAt:     now.Format(time.RFC3339),
		GeneratedAtUnix: now.Unix(),
	}

	var (
		models        []v1.WorkbenchModelAvail
		breakers      []v1.WorkbenchBreaker
		availItems    []wbItem
		moneyItems    []wbItem
		customerItems []wbItem
		systemItems   []wbItem
		metrics       []v1.WorkbenchMetric
	)

	var wg sync.WaitGroup
	wg.Add(5)
	go wbSafe(ctx, &wg, "availability", func() { models, breakers, availItems = s.wbCollectAvailability(ctx) })
	go wbSafe(ctx, &wg, "money", func() { moneyItems = s.wbCollectMoney(ctx) })
	go wbSafe(ctx, &wg, "customer", func() { customerItems = s.wbCollectCustomer(ctx) })
	go wbSafe(ctx, &wg, "system", func() { systemItems = s.wbCollectSystem(ctx) })
	go wbSafe(ctx, &wg, "metrics", func() { metrics = s.wbCollectMetrics(ctx) })
	wg.Wait()

	col.Models = models
	col.Breakers = breakers
	col.Items = append(col.Items, availItems...)
	col.Items = append(col.Items, moneyItems...)
	col.Items = append(col.Items, customerItems...)
	col.Items = append(col.Items, systemItems...)
	col.Metrics = metrics
	return col
}

// wbSafe 跑单个 collector，负责 wg.Done 并吞掉 panic。
// collector 自身已经兜住了「查询报错」（记 warning 返回空），这里补的是
// 「不可恢复错误」：并发化之后 panic 发生在子 goroutine，HTTP 中间件的
// 恢复逻辑接不住它 —— 不 recover 就是一次空指针带走整个服务。
//
// defer 顺序：wg.Done 先注册后执行，保证 recover 之后计数一定被减掉
// （否则一次 panic 就会让 collectWorkbench 永久卡在 wg.Wait）。
func wbSafe(ctx context.Context, wg *sync.WaitGroup, name string, fn func()) {
	defer wg.Done()
	defer func() {
		if e := recover(); e != nil {
			g.Log().Errorf(ctx, "workbench: collector %s panic: %v", name, e)
		}
	}()
	fn()
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
