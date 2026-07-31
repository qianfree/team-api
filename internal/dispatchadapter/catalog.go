package dispatchadapter

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

const (
	catalogRefreshInterval = 5 * time.Second
	catalogInvalidateChan  = keyPrefix + "catalog:invalidate" // pub/sub 频道：渠道/能力变更时发布
	rampNewChannelWindow   = 10 * time.Minute                 // 新建渠道视为爬坡期的窗口
)

// ChannelMeta 转发所需的渠道元数据（调度核心不关心，handler 构造 ChannelSelection 用）。
type ChannelMeta struct {
	ChannelID      int64
	ChannelName    string
	ChannelType    int
	BaseURL        string
	UpstreamModel  string
	IsModelMapped  bool
	Settings       string
	MaxConcurrency int
	StrictCapacity bool
	Tier           string
}

// catalogRow 目录加载的原始行（渠道×模型）。
type catalogRow struct {
	ChannelID      int64   `json:"channel_id"`
	ChannelName    string  `json:"channel_name"`
	ChannelType    int     `json:"channel_type"`
	BaseURL        string  `json:"base_url"`
	ModelName      string  `json:"model_name"`
	UpstreamModel  string  `json:"upstream_model"`
	Weight         int     `json:"weight"`
	MaxConcurrency int     `json:"max_concurrency"`
	Settings       string  `json:"settings"`
	Tier           string  `json:"tier"`
	StrictCapacity bool    `json:"strict_capacity"`
	CostRatio      float64 `json:"cost_ratio"`
	CreatedAtMs    int64   `json:"created_at_ms"`
}

// catalogData 一次完整加载的目录数据。
type catalogData struct {
	rows          []catalogRow
	keysByChannel map[int64][]int64 // channelID → active Key ID 列表（升序）
}

// catalogLoader 目录加载函数（可注入假实现做单测）。
type catalogLoader func(ctx context.Context) (*catalogData, error)

// runtimeReader 运行时读值函数（可注入假实现做单测）。
type runtimeReader func(ctx context.Context, channelID int64, model string) RuntimeReadout

// snapshotIndex 构建完成的只读快照索引。
type snapshotIndex struct {
	byModel       map[string][]dispatch.Channel
	meta          map[int64]map[string]ChannelMeta // channelID → model → 转发元数据
	strict        map[int64]int                    // 严格容量渠道 → max_concurrency（供 R4 降级查询）
	channelModels map[int64][]string               // channelID → 服务的模型列表（维护 cron 用）
}

// Catalog 实现 dispatch.CatalogPort：渠道目录内存快照，定时刷新 + pub/sub 失效。
// Snapshot 为 O(1) 内存读取，热路径零 DB/Redis 访问（消除 P1）。
type Catalog struct {
	load    catalogLoader
	runtime runtimeReader
	policy  func() *dispatch.RoutingPolicy
	current atomic.Pointer[snapshotIndex]
	refresh chan struct{} // 主动失效信号
	stop    chan struct{}
}

// NewCatalog 构造目录。loader/runtime 传 nil 时使用默认 DB/Redis 实现。
func NewCatalog(policy func() *dispatch.RoutingPolicy, load catalogLoader, runtime runtimeReader) *Catalog {
	c := &Catalog{
		load:    load,
		runtime: runtime,
		policy:  policy,
		refresh: make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}
	if c.load == nil {
		c.load = loadCatalogFromDB
	}
	c.current.Store(&snapshotIndex{
		byModel:       map[string][]dispatch.Channel{},
		meta:          map[int64]map[string]ChannelMeta{},
		strict:        map[int64]int{},
		channelModels: map[int64][]string{},
	})
	return c
}

// Snapshot 实现 dispatch.CatalogPort：返回某模型的候选渠道快照副本。
// scope 非空时按租户渠道范围过滤。tenantID 预留（租户级模型权限由调用方校验）。
func (c *Catalog) Snapshot(_ context.Context, _ int64, model string, scope []int64) []dispatch.Channel {
	idx := c.current.Load()
	channels := idx.byModel[model]
	if len(channels) == 0 {
		return nil
	}
	out := make([]dispatch.Channel, 0, len(channels))
	for _, ch := range channels {
		if len(scope) > 0 && !containsID(scope, ch.ID) {
			continue
		}
		out = append(out, ch)
	}
	return out
}

// ForwardMeta 返回渠道×模型的转发元数据（MaterializeSelection 构造 ChannelSelection 用）。
func (c *Catalog) ForwardMeta(channelID int64, model string) (ChannelMeta, bool) {
	idx := c.current.Load()
	if byModel, ok := idx.meta[channelID]; ok {
		m, ok := byModel[model]
		return m, ok
	}
	return ChannelMeta{}, false
}

// StrictLookup 修订 R4：查询渠道是否严格容量及其手动并发上限（供 RedisState 降级）。
func (c *Catalog) StrictLookup(channelID int64) (bool, int) {
	idx := c.current.Load()
	maxConc, ok := idx.strict[channelID]
	return ok, maxConc
}

// ChannelModels 返回渠道 → 服务模型列表的副本（维护 cron 遍历用）。
func (c *Catalog) ChannelModels() map[int64][]string {
	idx := c.current.Load()
	out := make(map[int64][]string, len(idx.channelModels))
	for id, models := range idx.channelModels {
		out[id] = append([]string(nil), models...)
	}
	return out
}

// Invalidate 主动触发一次刷新（渠道/能力/Key 管理操作后调用；跨实例走 pub/sub）。
func (c *Catalog) Invalidate() {
	select {
	case c.refresh <- struct{}{}:
	default:
	}
}

// PublishInvalidate 发布跨实例目录失效通知（管理后台写操作后调用）。
func PublishInvalidate(ctx context.Context) {
	_, _ = g.Redis().Publish(ctx, catalogInvalidateChan, "*")
}

// Start 启动定时刷新循环与 pub/sub 失效订阅。
func (c *Catalog) Start(ctx context.Context) {
	c.Rebuild(ctx) // 启动即加载一次
	go c.refreshLoop(ctx)
	go c.subscribeInvalidate(ctx)
}

// Stop 停止后台刷新。
func (c *Catalog) Stop() { close(c.stop) }

func (c *Catalog) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(catalogRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.Rebuild(ctx)
		case <-c.refresh:
			c.Rebuild(ctx)
		case <-c.stop:
			return
		}
	}
}

func (c *Catalog) subscribeInvalidate(ctx context.Context) {
	for {
		select {
		case <-c.stop:
			return
		default:
		}
		conn, _, err := g.Redis().Subscribe(ctx, catalogInvalidateChan)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		for {
			v, err := conn.Receive(ctx)
			if err != nil {
				break // 重连
			}
			if _, ok := v.Val().(*gredis.Message); ok {
				c.Invalidate()
			}
		}
		conn.Close(ctx)
		time.Sleep(time.Second)
	}
}

// Rebuild 全量重建快照：DB 目录 + Redis 运行时读值合并。
// 加载失败保留上一份快照（last-known，基线方案 §13）。
func (c *Catalog) Rebuild(ctx context.Context) {
	data, err := c.load(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Dispatch] 目录加载失败，沿用上一份快照: %v", err)
		return
	}

	pol := c.policy()
	now := time.Now().UnixMilli()
	rampWindowMs := int64(pol.Ramp.WindowSeconds) * 1000

	idx := &snapshotIndex{
		byModel:       make(map[string][]dispatch.Channel),
		meta:          make(map[int64]map[string]ChannelMeta),
		strict:        make(map[int64]int),
		channelModels: make(map[int64][]string),
	}
	// 同一渠道的运行时读值按 (channel, model) 缓存，避免同轮重复读
	type rtKey struct {
		ch    int64
		model string
	}
	rtCache := make(map[rtKey]RuntimeReadout)

	for _, row := range data.rows {
		rk := rtKey{row.ChannelID, row.ModelName}
		rt, ok := rtCache[rk]
		if !ok {
			if c.runtime != nil {
				rt = c.runtime(ctx, row.ChannelID, row.ModelName)
			} else {
				rt = RuntimeReadout{SuccEwma: 1}
			}
			rtCache[rk] = rt
		}

		ch := dispatch.Channel{
			ID:             row.ChannelID,
			Name:           row.ChannelName,
			Tier:           dispatch.Tier(row.Tier),
			BaseWeight:     float64(row.Weight),
			CostRatio:      row.CostRatio,
			SuccEwma:       rt.SuccEwma,
			LatEwmaMs:      rt.LatEwmaMs,
			Inflight:       rt.Inflight,
			SoftLimit:      effectiveSoftLimit(row.MaxConcurrency, rt.Onset429Ewma),
			Breaker:        rt.Breaker,
			ModelBreaker:   rt.ModelBreaker,
			RampElapsedMs:  rampElapsed(now, rt.RecoveredMs, row.CreatedAtMs, rampWindowMs),
			KeyIDs:         data.keysByChannel[row.ChannelID],
			StrictCapacity: row.StrictCapacity,
		}
		idx.byModel[row.ModelName] = append(idx.byModel[row.ModelName], ch)
		idx.channelModels[row.ChannelID] = append(idx.channelModels[row.ChannelID], row.ModelName)

		if _, ok := idx.meta[row.ChannelID]; !ok {
			idx.meta[row.ChannelID] = make(map[string]ChannelMeta)
		}
		upstream := row.UpstreamModel
		if upstream == "" {
			upstream = row.ModelName
		}
		idx.meta[row.ChannelID][row.ModelName] = ChannelMeta{
			ChannelID:      row.ChannelID,
			ChannelName:    row.ChannelName,
			ChannelType:    row.ChannelType,
			BaseURL:        row.BaseURL,
			UpstreamModel:  upstream,
			IsModelMapped:  row.UpstreamModel != "" && row.UpstreamModel != row.ModelName,
			Settings:       row.Settings,
			MaxConcurrency: row.MaxConcurrency,
			StrictCapacity: row.StrictCapacity,
			Tier:           row.Tier,
		}
		if row.StrictCapacity {
			idx.strict[row.ChannelID] = row.MaxConcurrency
		}
	}
	c.current.Store(idx)
}

// effectiveSoftLimit softLimit 双来源（基线方案 §8.2）：
// 手动 max_concurrency 优先；未配置时用 429 起始水位自动估计
// softLimit = max(4, floor(onset_ewma × 0.9))；无 429 历史视为无限容量。
func effectiveSoftLimit(manualMaxConcurrency int, onset429Ewma float64) int {
	if manualMaxConcurrency > 0 {
		return manualMaxConcurrency
	}
	if onset429Ewma <= 0 {
		return 0 // 无容量信息，headroomFactor 恒为 1
	}
	return max(4, int(onset429Ewma*0.9))
}

// rampElapsed 计算爬坡状态：熔断恢复或新建渠道在窗口内 → 返回已流逝毫秒；否则 -1。
func rampElapsed(nowMs, recoveredMs, createdMs, windowMs int64) int64 {
	if windowMs <= 0 {
		return -1
	}
	// 熔断恢复优先
	if recoveredMs > 0 && nowMs-recoveredMs < windowMs {
		return nowMs - recoveredMs
	}
	// 新建渠道：创建后 rampNewChannelWindow 内按爬坡处理
	if createdMs > 0 && nowMs-createdMs < min(windowMs, rampNewChannelWindow.Milliseconds()) {
		return nowMs - createdMs
	}
	return -1
}

// loadCatalogFromDB 默认目录加载：三表数据 + 渠道 Key 列表。
// 注意：tier / strict_capacity / cost_ratio 依赖迁移 000014，执行迁移前调用会报错
// （本函数仅在阶段 2 影子模式接线后才会被真实调用）。
func loadCatalogFromDB(ctx context.Context) (*catalogData, error) {
	var rows []catalogRow
	err := dao.ChnAbilities.Ctx(ctx).As("a").
		LeftJoin("chn_channels c ON a.channel_id = c.id").
		Where("a.enabled", true).
		Where("c.status", "active").
		Fields("c.id as channel_id, c.name as channel_name, c.type as channel_type, c.base_url, " +
			"a.model_name, a.upstream_model, c.weight, c.max_concurrency, c.settings, " +
			"c.tier, c.strict_capacity, a.cost_ratio, " +
			"(EXTRACT(EPOCH FROM c.created_at) * 1000)::bigint as created_at_ms").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	type keyRow struct {
		ID        int64 `json:"id"`
		ChannelID int64 `json:"channel_id"`
	}
	var keys []keyRow
	err = dao.ChnChannelKeys.Ctx(ctx).
		Where("status", "active").
		Fields("id, channel_id").
		Scan(&keys)
	if err != nil {
		return nil, err
	}
	keysByChannel := make(map[int64][]int64)
	for _, k := range keys {
		keysByChannel[k.ChannelID] = append(keysByChannel[k.ChannelID], k.ID)
	}
	for id := range keysByChannel {
		slices.Sort(keysByChannel[id])
	}
	return &catalogData{rows: rows, keysByChannel: keysByChannel}, nil
}

func containsID(ids []int64, id int64) bool {
	return slices.Contains(ids, id)
}
