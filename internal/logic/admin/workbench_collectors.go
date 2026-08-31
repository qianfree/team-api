package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dispatchadapter"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ============================================================
// 服务可用性（channel:view）
// ============================================================

// wbCollectAvailability 渠道与模型可用性。
// 返回模型可用渠道速览、正在熔断列表，以及派生出的待办。
func (s *sAdmin) wbCollectAvailability(ctx context.Context) ([]v1.WorkbenchModelAvail, []v1.WorkbenchBreaker, []wbItem) {
	items := make([]wbItem, 0, 8)

	// --- 目录快照：模型 → 可用/总渠道数 + 熔断明细 ---
	var avail map[string]dispatchadapter.ModelAvail
	if cat := dispatchadapter.CatalogInstance(); cat != nil {
		avail = cat.ModelAvailability()
	}

	models := make([]v1.WorkbenchModelAvail, 0, len(avail))
	breakers := make([]v1.WorkbenchBreaker, 0, 8)
	for model, av := range avail {
		models = append(models, v1.WorkbenchModelAvail{
			Model: model, Available: av.Available, Total: av.Total,
		})
		for _, b := range av.Breaking {
			breakers = append(breakers, v1.WorkbenchBreaker{
				ChannelId:    b.ChannelID,
				ChannelName:  b.ChannelName,
				Model:        b.Model,
				ChannelLevel: b.ChannelLevel,
				HalfOpen:     b.HalfOpen,
			})
		}

		// P0：配了渠道但一条都用不了。
		// 这是最容易被大盘掩盖的静默故障 —— 单个模型全挂时，平台总请求量可能只掉几个
		// 百分点，成功率曲线也被其他健康模型摊薄，但该模型的客户已经 100% 失败了。
		if av.Total > 0 && av.Available == 0 {
			items = append(items, wbNew(
				"model_no_channel:"+model,
				v1.WorkbenchSeverityP0, v1.WorkbenchDomainAvailability, "channel:view",
				fmt.Sprintf("模型 %s 零可用渠道", model),
				fmt.Sprintf("承载该模型的 %d 个渠道全部处于熔断或不可用状态，该模型请求当前必然失败", av.Total),
				"查看渠道", "AdminChannels", map[string]string{"status": "active"}, nil,
			))
		}
	}
	// 排序保证响应稳定：map 遍历顺序随机会让前端列表每次刷新都跳动
	sortModelAvail(models)
	sortBreakers(breakers)

	// P1：渠道级熔断（该渠道所有模型受影响，但还没到零可用）
	chLevel := map[int64]string{}
	for _, b := range breakers {
		if b.ChannelLevel && !b.HalfOpen {
			chLevel[b.ChannelId] = b.ChannelName
		}
	}
	for id, name := range chLevel {
		items = append(items, wbNew(
			fmt.Sprintf("channel_breaker:%d", id),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainAvailability, "channel:view",
			fmt.Sprintf("渠道 %s 已熔断", name),
			"该渠道全部模型已被调度器摘除，容量下降，恢复前流量会压到其余渠道",
			"查看渠道", "AdminChannelDetail", map[string]string{"id": fmt.Sprint(id)}, nil,
		))
	}

	items = append(items, s.wbChannelKeyIssues(ctx)...)
	items = append(items, s.wbOAuthExpiring(ctx)...)
	items = append(items, s.wbHealthDrop(ctx)...)

	return models, breakers, items
}

// wbChannelKeyIssues 渠道 Key 失效 / 额度耗尽。
// Key 挂掉时渠道整体仍是 active，容量却已悄悄下降 —— 列表页看不出来。
func (s *sAdmin) wbChannelKeyIssues(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT k.id, k.channel_id, k.name, k.status, k.last_error, c.name AS channel_name,
		        (SELECT COUNT(*) FROM chn_channel_keys k2 WHERE k2.channel_id = k.channel_id) AS total_keys
		   FROM chn_channel_keys k
		   JOIN chn_channels c ON c.id = k.channel_id
		  WHERE k.status <> 'active' AND c.status = 'active'
		  ORDER BY k.updated_at DESC
		  LIMIT 20`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询异常渠道 Key 失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		status := r["status"].String()
		label := "已禁用"
		if status == "exhausted" {
			label = "额度耗尽"
		}
		total := r["total_keys"].Int()
		desc := fmt.Sprintf("渠道「%s」的 Key「%s」%s", r["channel_name"].String(), r["name"].String(), label)
		if total > 1 {
			desc += fmt.Sprintf("，该渠道可用 Key %d/%d，容量下降", total-1, total)
		} else {
			desc += "，该渠道已无可用 Key"
		}
		if e := strings.TrimSpace(r["last_error"].String()); e != "" {
			desc += "；最后错误：" + truncate(e, 120)
		}

		items = append(items, wbNew(
			fmt.Sprintf("channel_key_invalid:%d", r["id"].Int64()),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainAvailability, "channel:view",
			fmt.Sprintf("%s Key %s", r["channel_name"].String(), label),
			desc,
			"查看渠道", "AdminChannelDetail",
			map[string]string{"id": r["channel_id"].String()}, nil,
		))
	}
	return items
}

// wbOAuthExpiring OAuth 令牌临期。
// 极其隐蔽：过期那一刻整条渠道直接不可用，且没有任何渐进信号。
func (s *sAdmin) wbOAuthExpiring(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT k.id, k.channel_id, k.name, k.token_expires_at, c.name AS channel_name
		   FROM chn_channel_keys k
		   JOIN chn_channels c ON c.id = k.channel_id
		  WHERE k.key_type = 'oauth'
		    AND c.status = 'active'
		    AND k.token_expires_at IS NOT NULL
		    AND k.token_expires_at <= ?
		  ORDER BY k.token_expires_at ASC
		  LIMIT 20`, gtime.Now().Add(wbOAuthExpiryHours*time.Hour))
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询 OAuth 临期令牌失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		expAt := r["token_expires_at"].GTime()
		expired := expAt != nil && expAt.Before(gtime.Now())

		sev := v1.WorkbenchSeverityP1
		title := fmt.Sprintf("%s OAuth 令牌即将过期", r["channel_name"].String())
		desc := fmt.Sprintf("Key「%s」将于 %s 过期，过期后该渠道立即不可用",
			r["name"].String(), fmtTime(expAt))
		if expired {
			// 已经过期就不是预警而是故障了
			sev = v1.WorkbenchSeverityP0
			title = fmt.Sprintf("%s OAuth 令牌已过期", r["channel_name"].String())
			desc = fmt.Sprintf("Key「%s」已于 %s 过期，该渠道当前不可用",
				r["name"].String(), fmtTime(expAt))
		}

		items = append(items, wbNew(
			fmt.Sprintf("channel_oauth_expiring:%d", r["id"].Int64()),
			sev, v1.WorkbenchDomainAvailability, "channel:view",
			title, desc,
			"重新授权", "AdminChannelDetail",
			map[string]string{"id": r["channel_id"].String()}, nil,
		))
	}
	return items
}

// wbHealthDrop 健康分骤降。
// 判据是「相对自己 1 小时前的跌幅」而非绝对分值 —— 一个长期 60 分的渠道是已知状态，
// 而 92 分跌到 61 分是正在发生的事故，绝对阈值分不出这两者。
func (s *sAdmin) wbHealthDrop(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`WITH latest AS (
		    SELECT DISTINCT ON (channel_id) channel_id, health_score, snapshot_at
		      FROM chn_health_snapshots
		     WHERE snapshot_at >= NOW() - INTERVAL '15 minutes'
		     ORDER BY channel_id, snapshot_at DESC
		 ), baseline AS (
		    SELECT DISTINCT ON (channel_id) channel_id, health_score
		      FROM chn_health_snapshots
		     WHERE snapshot_at BETWEEN NOW() - INTERVAL '75 minutes' AND NOW() - INTERVAL '45 minutes'
		     ORDER BY channel_id, snapshot_at DESC
		 )
		 SELECT l.channel_id, c.name AS channel_name,
		        ROUND(l.health_score)::int AS now_score,
		        ROUND(b.health_score)::int AS prev_score,
		        l.snapshot_at
		   FROM latest l
		   JOIN baseline b ON b.channel_id = l.channel_id
		   JOIN chn_channels c ON c.id = l.channel_id
		  WHERE c.status = 'active'
		    AND b.health_score - l.health_score >= ?
		  ORDER BY (b.health_score - l.health_score) DESC
		  LIMIT 10`, wbHealthDropPoints)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询健康分骤降失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, wbNew(
			fmt.Sprintf("channel_health_drop:%d", r["channel_id"].Int64()),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainAvailability, "channel:view",
			fmt.Sprintf("%s 健康分骤降", r["channel_name"].String()),
			fmt.Sprintf("1 小时内从 %d 分跌至 %d 分，调度权重已自动下调，尚未触发熔断",
				r["prev_score"].Int(), r["now_score"].Int()),
			"查看健康趋势", "AdminChannelDetail",
			map[string]string{"id": r["channel_id"].String()},
			r["snapshot_at"].GTime(),
		))
	}
	return items
}

// ============================================================
// 资金安全（billing:view / order:view）
// ============================================================

func (s *sAdmin) wbCollectMoney(ctx context.Context) []wbItem {
	items := make([]wbItem, 0, 8)
	items = append(items, s.wbWalletDrift(ctx)...)
	items = append(items, s.wbFrozenStale(ctx)...)
	items = append(items, s.wbOrderUnfulfilled(ctx)...)
	items = append(items, s.wbLowBalanceTenants(ctx)...)
	return items
}

// wbWalletDrift 钱包 Redis↔DB 物化偏差。
//
// 按架构约定 Redis 是余额唯一实时权威，DB 只是每 5s 物化一次的滞后副本。
// 正常情况下二者差异很小；持续放大的偏差意味着物化器停摆或 Lua 与账本不一致，
// 此时 DB 余额不可用于对账，且问题会随时间累积 —— 属于必须立刻知道的一类。
func (s *sAdmin) wbWalletDrift(ctx context.Context) []wbItem {
	// 只抽查有余额的活跃钱包，避免全表逐个读 Redis
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT w.tenant_id, w.balance, t.name AS tenant_name
		   FROM bil_wallets w
		   JOIN tnt_tenants t ON t.id = w.tenant_id
		  WHERE w.balance > 0 AND t.status IN ('active', 'trial')
		  ORDER BY w.balance DESC
		  LIMIT 30`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询钱包列表失败: %v", err)
		return nil
	}

	var (
		worstTenant string
		worstDrift  float64
		driftCount  int
	)
	for _, r := range rows {
		tenantID := r["tenant_id"].Int64()
		dbBalance := r["balance"].Float64()

		info, err := billing.GetWallet(ctx, tenantID)
		if err != nil || info == nil {
			continue
		}
		drift := info.Balance - dbBalance
		if drift < 0 {
			drift = -drift
		}
		// 相对阈值 + 绝对下限：小额账户的分币级差异不值得报警
		if dbBalance > 0 && drift/dbBalance > wbWalletDriftRatio && drift > 1 {
			driftCount++
			if drift > worstDrift {
				worstDrift = drift
				worstTenant = r["tenant_name"].String()
			}
		}
	}

	if driftCount == 0 {
		return nil
	}
	return []wbItem{wbNew(
		"wallet_drift",
		v1.WorkbenchSeverityP0, v1.WorkbenchDomainMoney, "billing:view",
		"钱包 Redis 与 DB 物化余额偏差",
		fmt.Sprintf("抽查发现 %d 个钱包偏差超阈值，最大偏差 %.2f（租户「%s」）。"+
			"DB 余额由物化器每 5 秒覆盖，持续偏差说明物化器可能停摆，当前 DB 余额不可用于对账",
			driftCount, worstDrift, worstTenant),
		"查看定时任务", "AdminCronJobs", nil, nil,
	)}
}

// wbFrozenStale 长时间未释放的冻结。
// 客户余额被凭空锁住，是客服工单的高发源，且往往是异步任务未回收的信号。
func (s *sAdmin) wbFrozenStale(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT w.tenant_id, t.name AS tenant_name
		   FROM bil_wallets w
		   JOIN tnt_tenants t ON t.id = w.tenant_id
		  WHERE w.frozen_balance > 0
		  ORDER BY w.frozen_balance DESC
		  LIMIT 20`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询冻结钱包失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, 4)
	for _, r := range rows {
		tenantID := r["tenant_id"].Int64()
		frozen, err := billing.GetFrozenItems(ctx, tenantID)
		if err != nil {
			continue
		}
		var (
			staleCount int
			staleTotal float64
		)
		// FrozenItem.CreatedAt 是 Unix 秒（预扣 Lua 写入），与既有冻结明细接口同口径
		staleBefore := time.Now().Add(-wbFrozenStaleHours * time.Hour).Unix()
		for _, f := range frozen {
			if f.CreatedAt > 0 && f.CreatedAt <= staleBefore {
				staleCount++
				staleTotal += f.Amount
			}
		}
		if staleCount == 0 {
			continue
		}
		items = append(items, wbNew(
			fmt.Sprintf("frozen_stale:%d", tenantID),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainMoney, "billing:view",
			fmt.Sprintf("租户「%s」%d 笔冻结超时未释放", r["tenant_name"].String(), staleCount),
			fmt.Sprintf("合计 %.6f 被锁定超过 %d 小时，客户可用余额受影响，通常是异步任务未回收",
				staleTotal, wbFrozenStaleHours),
			"查看冻结明细", "AdminTenantDetail",
			map[string]string{"id": fmt.Sprint(tenantID), "tab": "wallet"}, nil,
		))
	}
	return items
}

// wbOrderUnfulfilled 已支付未履约的卡单。真金白银，客户会追。
func (s *sAdmin) wbOrderUnfulfilled(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT o.id, o.order_no, o.final_amount, o.paid_at, o.order_type, t.name AS tenant_name
		   FROM ord_orders o
		   LEFT JOIN tnt_tenants t ON t.id = o.tenant_id
		  WHERE o.status = 'paid'
		    AND o.fulfilled_at IS NULL
		    AND o.paid_at IS NOT NULL
		    AND o.paid_at <= ?
		  ORDER BY o.paid_at ASC
		  LIMIT 20`, gtime.Now().Add(-wbOrderUnfulfilledMin*time.Minute))
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询卡单失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		paidAt := r["paid_at"].GTime()
		items = append(items, wbNew(
			"order_unfulfilled:"+r["order_no"].String(),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainMoney, "order:view",
			fmt.Sprintf("订单 %s 已支付未履约", r["order_no"].String()),
			fmt.Sprintf("租户「%s」支付 ¥%.2f 于 %s，至今未履约，客户可能已在催",
				r["tenant_name"].String(), r["final_amount"].Float64(), fmtTime(paidAt)),
			"处理订单", "AdminOrders",
			map[string]string{"order_no": r["order_no"].String()}, paidAt,
		))
	}
	return items
}

// wbLowBalanceTenants 余额低于预警阈值的租户。
// 一个信号两个用途：流失预警 + 催充窗口。聚合成一条，不逐租户刷屏。
func (s *sAdmin) wbLowBalanceTenants(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt, COALESCE(SUM(w.balance - w.frozen_balance), 0) AS available
		   FROM bil_wallets w
		   JOIN tnt_tenants t ON t.id = w.tenant_id
		  WHERE t.status IN ('active', 'trial')
		    AND w.warning_threshold > 0
		    AND (w.balance - w.frozen_balance) < w.warning_threshold`)
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询低余额租户失败: %v", err)
		}
		return nil
	}

	cnt := rows[0]["cnt"].Int()
	if cnt == 0 {
		return nil
	}
	return []wbItem{wbNew(
		"tenant_low_balance",
		v1.WorkbenchSeverityP2, v1.WorkbenchDomainCustomer, "billing:view",
		fmt.Sprintf("%d 个租户余额低于预警阈值", cnt),
		fmt.Sprintf("合计可用余额 %.6f，既是服务中断风险，也是催充与流失干预的窗口",
			rows[0]["available"].Float64()),
		"查看租户", "AdminTenants", nil, nil,
	)}
}

// ============================================================
// 客户运营（support:view / plan:view）
// ============================================================

func (s *sAdmin) wbCollectCustomer(ctx context.Context) []wbItem {
	items := make([]wbItem, 0, 8)
	items = append(items, s.wbTicketIssues(ctx)...)
	items = append(items, s.wbFeedbackPending(ctx)...)
	items = append(items, s.wbPlanExpiring(ctx)...)
	return items
}

// wbTicketIssues 工单：只报「超 SLA 未首响」和「未分配」两类。
// 刻意不报工单总数 —— 那是个永远不为零的数字，放进待办只会制造永久红点。
func (s *sAdmin) wbTicketIssues(ctx context.Context) []wbItem {
	items := make([]wbItem, 0, 4)

	// 超 SLA 未首次响应：pending 且创建超过 SLA 时长
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT k.id, k.title, k.urgency, k.created_at, t.name AS tenant_name
		   FROM spt_tickets k
		   LEFT JOIN tnt_tenants t ON t.id = k.tenant_id
		  WHERE k.status = 'pending'
		    AND k.created_at <= ?
		  ORDER BY k.created_at ASC
		  LIMIT 10`, gtime.Now().Add(-wbTicketSLAHours*time.Hour))
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询超时工单失败: %v", err)
	} else {
		for _, r := range rows {
			createdAt := r["created_at"].GTime()
			sev := v1.WorkbenchSeverityP1
			if u := r["urgency"].String(); u == "urgent" {
				sev = v1.WorkbenchSeverityP0
			}
			items = append(items, wbNew(
				fmt.Sprintf("ticket_sla_breach:%d", r["id"].Int64()),
				sev, v1.WorkbenchDomainCustomer, "support:view",
				fmt.Sprintf("工单「%s」超 SLA 未首次响应", truncate(r["title"].String(), 40)),
				fmt.Sprintf("提交方「%s」，已等待 %s（SLA %d 小时）",
					r["tenant_name"].String(), humanizeSince(createdAt), wbTicketSLAHours),
				"去回复", "AdminTickets",
				map[string]string{"id": r["id"].String()}, createdAt,
			))
		}
	}

	// 未分配工单：聚合成一条
	cntRows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt FROM spt_tickets
		  WHERE status IN ('pending', 'reopened')
		    AND (assigned_admin_id IS NULL OR assigned_admin_id = 0)`)
	if err == nil && len(cntRows) > 0 {
		if n := cntRows[0]["cnt"].Int(); n > 0 {
			items = append(items, wbNew(
				"ticket_unassigned",
				v1.WorkbenchSeverityP2, v1.WorkbenchDomainCustomer, "support:view",
				fmt.Sprintf("%d 个工单未分配处理人", n),
				"无人认领的工单容易在交接中被漏掉，建议指派到人",
				"去分配", "AdminTickets", nil, nil,
			))
		}
	}
	return items
}

// wbFeedbackPending 待处理反馈（聚合一条 + 最长等待时间）。
func (s *sAdmin) wbFeedbackPending(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt, MIN(created_at) AS oldest
		   FROM spt_feedbacks WHERE status = 'pending'`)
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询待处理反馈失败: %v", err)
		}
		return nil
	}
	cnt := rows[0]["cnt"].Int()
	if cnt == 0 {
		return nil
	}
	oldest := rows[0]["oldest"].GTime()

	return []wbItem{wbNew(
		"feedback_pending",
		v1.WorkbenchSeverityP2, v1.WorkbenchDomainCustomer, "support:view",
		fmt.Sprintf("%d 条用户反馈待处理", cnt),
		fmt.Sprintf("最早 1 条已等待 %s", humanizeSince(oldest)),
		"去处理", "AdminFeedback", nil, nil,
	)}
}

// wbPlanExpiring 套餐即将到期且未开启自动续费 —— 续费转化窗口。
func (s *sAdmin) wbPlanExpiring(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt
		   FROM pln_tenant_plans
		  WHERE status = 'active'
		    AND auto_renew = false
		    AND end_at IS NOT NULL
		    AND end_at BETWEEN NOW() AND ?`, gtime.Now().AddDate(0, 0, wbPlanExpiringDays))
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询到期套餐失败: %v", err)
		}
		return nil
	}
	cnt := rows[0]["cnt"].Int()
	if cnt == 0 {
		return nil
	}
	return []wbItem{wbNew(
		"plan_expiring",
		v1.WorkbenchSeverityP2, v1.WorkbenchDomainCustomer, "plan:view",
		fmt.Sprintf("%d 个租户套餐 %d 天内到期", cnt, wbPlanExpiringDays),
		"均未开启自动续费，到期即停服，建议提前触达",
		"查看租户", "AdminTenants", nil, nil,
	)}
}

// ============================================================
// 系统与安全（system:view / monitor:view / audit:view）
// ============================================================

func (s *sAdmin) wbCollectSystem(ctx context.Context) []wbItem {
	items := make([]wbItem, 0, 8)
	items = append(items, s.wbCronStalled(ctx)...)
	items = append(items, s.wbEmailFailures(ctx)...)
	items = append(items, s.wbAlertUnacked(ctx)...)
	items = append(items, s.wbErrorLogsUnresolved(ctx)...)
	items = append(items, s.wbNewIPLogin(ctx)...)
	return items
}

// wbCronStalled 定时任务失败或停摆。
// 资金相关任务（预扣清扫、钱包物化）挂掉会直接导致资损，且完全无声。
func (s *sAdmin) wbCronStalled(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT job_name, last_status, last_started_at, last_error_message
		   FROM sys_cron_jobs
		  ORDER BY last_started_at ASC NULLS FIRST`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询定时任务状态失败: %v", err)
		return nil
	}

	// 注册表快照：任务名 → 调度表达式。停摆判定必须按各任务自身的调度周期换算，
	// 固定 1 小时停摆线会把日任务（日对账、日清理等）一天里 23 个小时都误报成停摆。
	schedules := make(map[string]string, 16)
	for _, j := range common.GetCronScheduler().ListJobs() {
		schedules[j.Name] = j.Schedule
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		name := r["job_name"].String()
		startedAt := r["last_started_at"].GTime()
		failed := r["last_status"].String() == "failed"

		// 停摆 = 距上次执行超过「正常间隔 × 2」，下限 1 小时——
		// 下限避免分钟级任务被一次调度抖动（锁等待、实例重启）误报
		interval := wbCronExpectedInterval(schedules, name)
		stallAfter := max(2*interval, time.Hour)
		stalled := startedAt != nil && time.Since(startedAt.Time) > stallAfter

		if !failed && !stalled && startedAt != nil {
			continue
		}

		// 资金相关任务提级为 P0：这些停摆会累积资损，不能等到明天
		sev := v1.WorkbenchSeverityP1
		if isMoneyCriticalJob(name) {
			sev = v1.WorkbenchSeverityP0
		}

		var desc string
		switch {
		case failed:
			desc = "最近一次执行失败"
			if e := strings.TrimSpace(r["last_error_message"].String()); e != "" {
				desc += "：" + truncate(e, 150)
			}
		case startedAt == nil:
			desc = "自服务启动以来从未执行过，调度器可能未正常注册该任务"
		default:
			desc = fmt.Sprintf("正常约 %s 执行一次，已 %s 未执行，任务可能已停摆",
				humanizeDuration(interval), humanizeSince(startedAt))
		}
		if isMoneyCriticalJob(name) {
			desc += "。该任务涉及资金结算，停摆会持续累积差错"
		}

		items = append(items, wbNew(
			"cron_stalled:"+name,
			sev, v1.WorkbenchDomainSystem, "system:view",
			fmt.Sprintf("定时任务 %s 异常", name),
			desc,
			"查看定时任务", "AdminCronJobs", nil, startedAt,
		))
	}
	return items
}

// wbCronExpectedInterval 从任务注册表查正常执行间隔。
// 任务不在注册表中（改名/下线后的遗留行）或表达式解析失败时退回保守兜底值：
// 宁可晚报一条真正死掉的任务，也不要每天误报一串健康任务。
func wbCronExpectedInterval(schedules map[string]string, name string) time.Duration {
	interval := wbCronStallFallbackInterval
	if schedule, ok := schedules[name]; ok {
		if d, err := common.ScheduleInterval(schedule); err == nil {
			interval = d
		}
	}
	return interval
}

// isMoneyCriticalJob 判断任务停摆是否会造成资损。
func isMoneyCriticalJob(name string) bool {
	n := strings.ToLower(name)
	for _, kw := range []string{"prededuct", "wallet", "materiali", "settle", "billing", "refund"} {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// wbEmailFailures 邮件发送失败。
// 隐蔽度极高：验证码发不出去等于新用户注册与找回密码链路整条中断，
// 而受影响的人根本没有渠道告诉你。
func (s *sAdmin) wbEmailFailures(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt,
		        COUNT(*) FILTER (WHERE template_code ILIKE '%verify%' OR template_code ILIKE '%code%') AS verify_cnt,
		        MAX(error_message) AS sample_error
		   FROM ntf_send_log
		  WHERE channel = 'email'
		    AND status = 'failed'
		    AND created_at >= NOW() - INTERVAL '1 hour'`)
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询邮件失败记录失败: %v", err)
		}
		return nil
	}

	cnt := rows[0]["cnt"].Int()
	if cnt < wbEmailFailThreshold {
		return nil
	}

	desc := fmt.Sprintf("近 1 小时内 %d 封邮件发送失败", cnt)
	if vc := rows[0]["verify_cnt"].Int(); vc > 0 {
		desc += fmt.Sprintf("，其中 %d 封为验证码类 —— 新用户注册与找回密码链路已实际中断", vc)
	}
	if e := strings.TrimSpace(rows[0]["sample_error"].String()); e != "" {
		desc += "；样本错误：" + truncate(e, 120)
	}

	return []wbItem{wbNew(
		"email_failures",
		v1.WorkbenchSeverityP1, v1.WorkbenchDomainSystem, "system:view",
		"邮件发送连续失败",
		desc,
		"查看发送记录", "AdminEmailLogs", map[string]string{"status": "failed"}, nil,
	)}
}

// wbAlertUnacked 未确认告警（聚合一条）。
func (s *sAdmin) wbAlertUnacked(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt,
		        COUNT(*) FILTER (WHERE level = 'critical') AS critical_cnt,
		        MIN(created_at) AS oldest
		   FROM ops_alert_events
		  WHERE status = 'firing'`)
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询未确认告警失败: %v", err)
		}
		return nil
	}
	cnt := rows[0]["cnt"].Int()
	if cnt == 0 {
		return nil
	}

	criticalCnt := rows[0]["critical_cnt"].Int()
	sev := v1.WorkbenchSeverityP2
	if criticalCnt > 0 {
		sev = v1.WorkbenchSeverityP1
	}
	desc := fmt.Sprintf("最早 1 条触发于 %s", humanizeSince(rows[0]["oldest"].GTime()))
	if criticalCnt > 0 {
		desc = fmt.Sprintf("其中 %d 条为严重级别；", criticalCnt) + desc
	}

	return []wbItem{wbNew(
		"alert_unack",
		sev, v1.WorkbenchDomainSystem, "monitor:view",
		fmt.Sprintf("%d 条告警事件未确认", cnt),
		desc,
		"去确认", "AdminAlertEvents", map[string]string{"status": "firing"}, nil,
	)}
}

// wbErrorLogsUnresolved 未处理系统错误日志（聚合一条）。
func (s *sAdmin) wbErrorLogsUnresolved(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt, COUNT(DISTINCT error_message) AS distinct_cnt
		   FROM sys_error_logs
		  WHERE resolved = false
		    AND created_at >= NOW() - INTERVAL '7 days'`)
	if err != nil || len(rows) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "workbench: 查询未处理错误日志失败: %v", err)
		}
		return nil
	}
	cnt := rows[0]["cnt"].Int()
	if cnt == 0 {
		return nil
	}

	desc := "近 7 天内产生"
	if d := rows[0]["distinct_cnt"].Int(); d > 0 && cnt > d {
		desc += fmt.Sprintf("，去重后仅 %d 种，建议按类型合并排查", d)
	}

	return []wbItem{wbNew(
		"error_log_unresolved",
		v1.WorkbenchSeverityP2, v1.WorkbenchDomainSystem, "monitor:view",
		fmt.Sprintf("%d 条系统错误日志未处理", cnt),
		desc,
		"查看错误日志", "AdminErrorLogs", nil, nil,
	)}
}

// wbNewIPLogin 管理员从新 IP 登录成功。
// 管理后台账号是整个平台的最高权限，异地登录必须让人看见并确认。
func (s *sAdmin) wbNewIPLogin(ctx context.Context) []wbItem {
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT h.id, h.user_id, h.ip_address, h.location, h.created_at, u.username
		   FROM aud_login_history h
		   LEFT JOIN sys_admin_users u ON u.id = h.user_id
		  WHERE h.user_type = 'admin'
		    AND h.success = true
		    AND h.is_new_device = true
		    AND h.created_at >= NOW() - INTERVAL '24 hours'
		  ORDER BY h.created_at DESC
		  LIMIT 5`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询异常登录失败: %v", err)
		return nil
	}

	items := make([]wbItem, 0, len(rows))
	for _, r := range rows {
		createdAt := r["created_at"].GTime()
		loc := strings.TrimSpace(r["location"].String())
		if loc == "" {
			loc = "未知归属地"
		}
		items = append(items, wbNew(
			fmt.Sprintf("admin_new_ip_login:%d", r["id"].Int64()),
			v1.WorkbenchSeverityP1, v1.WorkbenchDomainSystem, "audit:view",
			fmt.Sprintf("管理员 %s 从新设备登录", r["username"].String()),
			fmt.Sprintf("IP %s（%s），此前无该设备记录，请确认是否本人操作",
				maskIP(r["ip_address"].String()), loc),
			"查看登录历史", "AdminLoginHistory", nil, createdAt,
		))
	}
	return items
}

// ============================================================
// 首屏关键数字
// ============================================================

// wbCollectMetrics 首屏三个数字 + 待办计数由上层补。
//
// 收入对比刻意用「昨日同期」而非「昨日全天」：全天对比在每天早晨永远是 -80%，
// 是个看了就要在脑子里做一次除法的假信号。
func (s *sAdmin) wbCollectMetrics(ctx context.Context) []v1.WorkbenchMetric {
	metrics := make([]v1.WorkbenchMetric, 0, 3)

	// --- 今日收入 vs 昨日同期 ---
	var todayCost, yesterdayCost float64
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT
		    COALESCE(SUM(total_cost) FILTER (WHERE created_at >= date_trunc('day', NOW())), 0) AS today_cost,
		    COALESCE(SUM(total_cost) FILTER (
		        WHERE created_at >= date_trunc('day', NOW()) - INTERVAL '1 day'
		          AND created_at < date_trunc('day', NOW()) - INTERVAL '1 day' + (NOW() - date_trunc('day', NOW()))
		    ), 0) AS yesterday_cost
		 FROM bil_usage_logs
		 WHERE created_at >= date_trunc('day', NOW()) - INTERVAL '1 day'`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询收入指标失败: %v", err)
	} else if len(rows) > 0 {
		todayCost = rows[0]["today_cost"].Float64()
		yesterdayCost = rows[0]["yesterday_cost"].Float64()
	}

	revenueMetric := v1.WorkbenchMetric{
		Key: "today_revenue", Label: "今日收入", Value: todayCost,
		Unit: "money", Sub: "较昨日同期",
	}
	if yesterdayCost > 0 {
		growth := (todayCost - yesterdayCost) / yesterdayCost * 100
		revenueMetric.Growth = &growth
	}
	metrics = append(metrics, revenueMetric)

	// --- 实时错误率（近 5 分钟）---
	// 刻意不用「今日成功率」：那个数被一整天的历史稀释，正在发生的故障根本顶不动它。
	var errRate float64
	rows, err = g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status <> 'success') AS errors
		   FROM bil_usage_logs
		  WHERE created_at >= NOW() - INTERVAL '5 minutes'`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询实时错误率失败: %v", err)
	} else if len(rows) > 0 {
		if total := rows[0]["total"].Float64(); total > 0 {
			errRate = rows[0]["errors"].Float64() / total * 100
		}
	}
	metrics = append(metrics, v1.WorkbenchMetric{
		Key: "error_rate_5m", Label: "实时错误率", Value: round2(errRate),
		Unit: "percent", Sub: "近 5 分钟",
	})

	// --- 余额告警租户数 ---
	var lowBalance float64
	rows, err = g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt
		   FROM bil_wallets w
		   JOIN tnt_tenants t ON t.id = w.tenant_id
		  WHERE t.status IN ('active', 'trial')
		    AND w.warning_threshold > 0
		    AND (w.balance - w.frozen_balance) < w.warning_threshold`)
	if err != nil {
		g.Log().Warningf(ctx, "workbench: 查询余额告警租户失败: %v", err)
	} else if len(rows) > 0 {
		lowBalance = rows[0]["cnt"].Float64()
	}
	metrics = append(metrics, v1.WorkbenchMetric{
		Key: "low_balance_tenants", Label: "余额告警租户", Value: lowBalance,
		Unit: "count", Sub: "低于预警阈值",
	})

	return metrics
}

// ============================================================
// 小工具
// ============================================================

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

func fmtTime(t *gtime.Time) string {
	if t == nil {
		return "未知时间"
	}
	return t.Format("Y-m-d H:i")
}

// humanizeSince 把时间差写成人话。工作台上「3 小时 12 分」比时间戳更能驱动行动。
func humanizeSince(t *gtime.Time) string {
	if t == nil {
		return "未知时长"
	}
	return humanizeDuration(time.Since(t.Time))
}

// humanizeDuration 把时长写成人话（分钟/小时/天）。
func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		return "不到 1 分钟"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%d 小时", h)
		}
		return fmt.Sprintf("%d 小时 %d 分", h, m)
	}
	return fmt.Sprintf("%d 天", int(d.Hours()/24))
}

// maskIP 打码 IP 中间两段，兼顾可辨识与最小暴露。
func maskIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	return parts[0] + ".**.**." + parts[3]
}

func sortModelAvail(list []v1.WorkbenchModelAvail) {
	sortSlice(len(list), func(i, j int) bool {
		// 零可用的排最前，其次按缺口比例，最后按名字保证确定性
		ai := list[i].Available == 0 && list[i].Total > 0
		aj := list[j].Available == 0 && list[j].Total > 0
		if ai != aj {
			return ai
		}
		gi := list[i].Total - list[i].Available
		gj := list[j].Total - list[j].Available
		if gi != gj {
			return gi > gj
		}
		return list[i].Model < list[j].Model
	}, func(i, j int) { list[i], list[j] = list[j], list[i] })
}

func sortBreakers(list []v1.WorkbenchBreaker) {
	sortSlice(len(list), func(i, j int) bool {
		if list[i].ChannelLevel != list[j].ChannelLevel {
			return list[i].ChannelLevel
		}
		if list[i].ChannelName != list[j].ChannelName {
			return list[i].ChannelName < list[j].ChannelName
		}
		return list[i].Model < list[j].Model
	}, func(i, j int) { list[i], list[j] = list[j], list[i] })
}

// sortSlice 简易插入排序：工作台的列表规模都在几十条以内，
// 不值得为此引入反射版 sort.Slice 的开销。
func sortSlice(n int, less func(i, j int) bool, swap func(i, j int)) {
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			swap(j, j-1)
		}
	}
}
