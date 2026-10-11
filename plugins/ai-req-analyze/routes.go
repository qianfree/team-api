package aireqanalyze

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/response"
)

// Routes 实现 Routable 接口：功能端点统一挂在 /api/admin/plugins/ai-req-analyze 下，
// 自挂鉴权中间件链（主路由组之外注册不继承），规则见 permission.go。
func (p *aiReqAnalyzePlugin) Routes(ctx context.Context, server *ghttp.Server) error {
	server.Group(routeBase, func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.AdminAuth, middleware.AdminPermissionGuard, middleware.OperationLog)

		group.GET("/ai/test", p.handleAITest)

		group.POST("/analyze", p.handleAnalyze)
		group.GET("/reports", p.handleReportList)
		group.GET("/reports/{id}", p.handleReportDetail)
		group.GET("/reports/{id}/conversation", p.handleReportConversation)
		group.DELETE("/reports/{id}", p.handleReportDelete)
		group.GET("/jobs", p.handleJobList)
	})
	return nil
}

// handleAITest 分析 AI 连通性测试（ai-req-analyze:config 权限）。
// 读当前配置并发起一次最小补全，失败原因原样返回给管理员定位问题。
func (p *aiReqAnalyzePlugin) handleAITest(r *ghttp.Request) {
	ctx := r.Context()

	cfg, err := LoadConfig(ctx)
	if err != nil {
		response.Error(r, err)
		return
	}
	if err := cfg.Validate(); err != nil {
		response.Error(r, err)
		return
	}

	result := NewAIClient(cfg).TestConnection(ctx)
	response.Success(r, result)
}

// handleAnalyze 发起按需分析（ai-req-analyze:analyze 权限）→ 创建异步任务。
func (p *aiReqAnalyzePlugin) handleAnalyze(r *ghttp.Request) {
	ctx := r.Context()

	params, err := parseAnalyzeParams(r)
	if err != nil {
		response.Error(r, err)
		return
	}

	// 目标租户必须存在；指定用户时校验其属于该租户
	tenantExists, err := g.DB().Model("tnt_tenants").Ctx(ctx).Where("id", params.TenantID).Fields("id").Value()
	if err != nil {
		response.Error(r, err)
		return
	}
	if tenantExists == nil || tenantExists.Int64() == 0 {
		response.ErrorMsg(r, 404, "目标租户不存在")
		return
	}
	if params.UserID > 0 {
		v, err := g.DB().Model("tnt_users").Ctx(ctx).
			Where("id = ? AND tenant_id = ?", params.UserID, params.TenantID).
			Fields("id").Value()
		if err != nil {
			response.Error(r, err)
			return
		}
		if v == nil || v.Int64() == 0 {
			response.ErrorMsg(r, 404, "目标用户不存在或不属于该租户")
			return
		}
	}

	// 配置预检：AI 未配置好时立即失败，不产生空跑任务
	cfg, err := LoadConfig(ctx)
	if err != nil {
		response.Error(r, err)
		return
	}
	if err := cfg.Validate(); err != nil {
		response.Error(r, err)
		return
	}

	jobID, err := g.DB().Model(tableJobs).Ctx(ctx).Data(JobDO{
		TenantId:     params.TenantID,
		TargetUserId: params.UserID,
		Source:       SourceManual,
		TimeStart:    params.TimeStart,
		TimeEnd:      params.TimeEnd,
		Status:       JobStatusPending,
		CreatedBy:    middleware.GetUserID(ctx),
	}).InsertAndGetId()
	if err != nil {
		response.Error(r, err)
		return
	}

	if err := enqueueJob(jobID); err != nil {
		// 入队失败直接把任务标失败，不留永远 pending 的行
		failJob(ctx, jobID, err.Error())
		response.Error(r, err)
		return
	}

	response.Success(r, &AnalyzeRes{JobId: jobID})
}

// handleReportList 报告列表（ai-req-analyze:view 权限）。
func (p *aiReqAnalyzePlugin) handleReportList(r *ghttp.Request) {
	ctx := r.Context()
	page := maxInt(r.Get("page").Int(), 1)
	pageSize := clampInt(r.Get("page_size").Int(), 1, 100)

	m := g.DB().Model(tableReports+" AS r").Ctx(ctx).
		LeftJoin("tnt_tenants t", "t.id = r.tenant_id").
		LeftJoin("tnt_users u", "u.id = r.target_user_id")

	if v := r.Get("tenant_id").Int64(); v > 0 {
		m = m.Where("r.tenant_id", v)
	}
	if v := r.Get("source").String(); v != "" {
		m = m.Where("r.source", v)
	}
	if v := r.Get("risk_level").String(); v != "" {
		m = m.Where("r.risk_level", v)
	}
	if v := r.Get("start_date").String(); v != "" {
		m = m.Where("r.created_at >= ?", v)
	}
	if v := r.Get("end_date").String(); v != "" {
		m = m.Where("r.created_at <= ?", v+" 23:59:59")
	}

	total, err := m.Count()
	if err != nil {
		response.Error(r, err)
		return
	}

	var list []ReportListItem
	err = m.Page(page, pageSize).
		Order("r.id DESC").
		Fields("r.*, t.name AS tenant_name, u.username AS target_username").
		Scan(&list)
	if err != nil {
		response.Error(r, err)
		return
	}
	if list == nil {
		list = []ReportListItem{}
	}

	response.Success(r, &ListRes{List: list, Total: total, Page: page, PageSize: pageSize})
}

// handleReportDetail 报告详情 + 发现明细（ai-req-analyze:view 权限）。
// 报告证据含用户对话内容，访问即写敏感访问日志。
func (p *aiReqAnalyzePlugin) handleReportDetail(r *ghttp.Request) {
	ctx := r.Context()
	id, err := pathID(r)
	if err != nil {
		response.Error(r, err)
		return
	}

	var item ReportDetailRes
	err = g.DB().Model(tableReports+" AS r").Ctx(ctx).
		LeftJoin("tnt_tenants t", "t.id = r.tenant_id").
		LeftJoin("tnt_users u", "u.id = r.target_user_id").
		Where("r.id", id).
		Fields("r.*, t.name AS tenant_name, u.username AS target_username").
		Scan(&item)
	if err != nil {
		response.Error(r, err)
		return
	}
	if item.Id == 0 {
		response.ErrorMsg(r, 404, "报告不存在")
		return
	}

	// 严重度按 high > medium > low 排（字典序不等于枚举序，用 CASE 显式排序）
	var findings []FindingEntity
	if err := g.DB().Model(tableFindings).Ctx(ctx).
		Where("report_id", id).
		Order("CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, id ASC").
		Scan(&findings); err != nil {
		response.Error(r, err)
		return
	}
	if findings == nil {
		findings = []FindingEntity{}
	}
	item.Findings = findings

	lcommon.LogSensitiveAccess(ctx, middleware.GetUserID(ctx), "admin",
		"ai_req_analyze_report", id, "view", "查看AI请求分析报告详情")
	response.Success(r, &item)
}

// handleReportConversation 按报告内发现还原完整会话（ai-req-analyze:view 权限）。
// 查询参数 finding_id 定位发现的锚点日志，调核心会话重组（平台侧副本）。
func (p *aiReqAnalyzePlugin) handleReportConversation(r *ghttp.Request) {
	ctx := r.Context()
	reportID, err := pathID(r)
	if err != nil {
		response.Error(r, err)
		return
	}
	findingID := r.Get("finding_id").Int64()
	if findingID <= 0 {
		response.ErrorMsg(r, 400, "请指定 finding_id")
		return
	}

	var finding FindingEntity
	err = g.DB().Model(tableFindings).Ctx(ctx).
		Where("id = ? AND report_id = ?", findingID, reportID).
		Scan(&finding)
	if err != nil {
		response.Error(r, err)
		return
	}
	if finding.Id == 0 {
		response.ErrorMsg(r, 404, "发现记录不存在")
		return
	}

	// admin 全局视角 tenantID=0，平台侧正文副本
	view, err := lcommon.LoadConversationView(ctx, finding.AnchorLogId, 0, lcommon.SysBodyColumns())
	if err != nil {
		response.Error(r, err)
		return
	}

	lcommon.LogSensitiveAccess(ctx, middleware.GetUserID(ctx), "admin",
		"ai_req_analyze_report", reportID, "view_conversation", "查看分析报告关联的完整会话")
	response.Success(r, view)
}

// handleReportDelete 删除报告（ai-req-analyze:analyze 权限），连同发现明细。
func (p *aiReqAnalyzePlugin) handleReportDelete(r *ghttp.Request) {
	ctx := r.Context()
	id, err := pathID(r)
	if err != nil {
		response.Error(r, err)
		return
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := g.DB().Model(tableFindings).Ctx(ctx).Where("report_id", id).Delete(); err != nil {
			return err
		}
		if _, err := g.DB().Model(tableReports).Ctx(ctx).Where("id", id).Delete(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		response.Error(r, err)
		return
	}
	response.Success(r, nil)
}

// handleJobList 任务列表与进度（ai-req-analyze:view 权限）。
func (p *aiReqAnalyzePlugin) handleJobList(r *ghttp.Request) {
	ctx := r.Context()
	page := maxInt(r.Get("page").Int(), 1)
	pageSize := clampInt(r.Get("page_size").Int(), 1, 100)

	m := g.DB().Model(tableJobs).Ctx(ctx)
	if v := r.Get("status").String(); v != "" {
		m = m.Where("status", v)
	}
	if v := r.Get("tenant_id").Int64(); v > 0 {
		m = m.Where("tenant_id", v)
	}

	total, err := m.Count()
	if err != nil {
		response.Error(r, err)
		return
	}

	var list []JobEntity
	if err := m.Page(page, pageSize).Order("id DESC").Scan(&list); err != nil {
		response.Error(r, err)
		return
	}
	if list == nil {
		list = []JobEntity{}
	}

	response.Success(r, &ListRes{List: list, Total: total, Page: page, PageSize: pageSize})
}

// 小工具
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
