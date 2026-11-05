package admin

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/robfig/cron/v3"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/aianalysis"
	"github.com/qianfree/team-api/internal/logic/common"
)

// AI请求分析管理端门面：参数校验/查询组装在此，分析流水线（采样→物化→调 AI→
// 落报告）在 internal/logic/aianalysis。配置存 sys_options（ai_analysis_* 键），
// 本文件只做读写编排；权限点 ai_analysis:view/analyze/config 见 middleware/rbac.go。

// AiAnalysisCreate 发起按需分析：目标租户/用户校验 → 配置预检 → 建任务入队。
func (s *sAdmin) AiAnalysisCreate(ctx context.Context, req *v1.AiAnalysisCreateReq) (*v1.AiAnalysisCreateRes, error) {
	params, err := aianalysis.ParseAnalysisParams(req.TenantID, req.UserID, req.TimeStart, req.TimeEnd)
	if err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	// 目标租户必须存在；指定用户时校验其属于该租户
	tenantExists, err := g.DB().Model("tnt_tenants").Ctx(ctx).Where("id", params.TenantID).Fields("id").Value()
	if err != nil {
		return nil, err
	}
	if tenantExists == nil || tenantExists.Int64() == 0 {
		return nil, common.NewNotFoundError("目标租户")
	}
	if params.UserID > 0 {
		v, err := g.DB().Model("tnt_users").Ctx(ctx).
			Where("id = ? AND tenant_id = ?", params.UserID, params.TenantID).
			Fields("id").Value()
		if err != nil {
			return nil, err
		}
		if v == nil || v.Int64() == 0 {
			return nil, common.NewBadRequestError("目标用户不存在或不属于该租户")
		}
	}

	// 配置预检：AI 未配置好时立即失败，不产生空跑任务
	cfg := aianalysis.LoadConfig(ctx)
	if err := cfg.Validate(); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	jobID, err := g.DB().Model(aianalysis.TableJobs).Ctx(ctx).Data(aianalysis.JobDO{
		TenantId:     params.TenantID,
		TargetUserId: params.UserID,
		Source:       aianalysis.SourceManual,
		TimeStart:    params.TimeStart,
		TimeEnd:      params.TimeEnd,
		Status:       aianalysis.JobStatusPending,
		CreatedBy:    common.GetCtxUserID(ctx),
	}).InsertAndGetId()
	if err != nil {
		return nil, err
	}

	if err := aianalysis.EnqueueJob(jobID); err != nil {
		// 入队失败直接把任务标失败，不留永远 pending 的行
		aianalysis.FailJob(ctx, jobID, err.Error())
		return nil, common.NewBadRequestError(err.Error())
	}

	return &v1.AiAnalysisCreateRes{JobId: jobID}, nil
}

// AiAnalysisReportList 报告列表（联查租户/用户名）。
func (s *sAdmin) AiAnalysisReportList(ctx context.Context, req *v1.AiAnalysisReportListReq) (*v1.AiAnalysisReportListRes, error) {
	m := g.DB().Model(aianalysis.TableReports+" AS r").Ctx(ctx).
		LeftJoin("tnt_tenants t", "t.id = r.tenant_id").
		LeftJoin("tnt_users u", "u.id = r.target_user_id")

	if req.TenantID > 0 {
		m = m.Where("r.tenant_id", req.TenantID)
	}
	if req.Source != "" {
		m = m.Where("r.source", req.Source)
	}
	if req.RiskLevel != "" {
		m = m.Where("r.risk_level", req.RiskLevel)
	}
	if req.StartDate != "" {
		m = m.Where("r.created_at >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		m = m.Where("r.created_at <= ?", req.EndDate+" 23:59:59")
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}
	all, err := m.Page(req.Page, req.PageSize).
		Order("r.id DESC").
		Fields("r.*, t.name AS tenant_name, u.username AS target_username").
		All()
	if err != nil {
		return nil, err
	}
	list := all.List()
	if list == nil {
		list = []map[string]any{}
	}
	return &v1.AiAnalysisReportListRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

// AiAnalysisReportDetail 报告详情 + 发现明细。
// 报告证据含用户对话内容，访问即写敏感访问日志。
func (s *sAdmin) AiAnalysisReportDetail(ctx context.Context, req *v1.AiAnalysisReportDetailReq) (*v1.AiAnalysisReportDetailRes, error) {
	one, err := g.DB().Model(aianalysis.TableReports+" AS r").Ctx(ctx).
		LeftJoin("tnt_tenants t", "t.id = r.tenant_id").
		LeftJoin("tnt_users u", "u.id = r.target_user_id").
		Where("r.id", req.Id).
		Fields("r.*, t.name AS tenant_name, u.username AS target_username").
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, common.NewNotFoundError("报告")
	}

	// 严重度按 high > medium > low 排（字典序不等于枚举序，用 CASE 显式排序）
	findings, err := g.DB().Model(aianalysis.TableFindings).Ctx(ctx).
		Where("report_id", req.Id).
		Order("CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, id ASC").
		All()
	if err != nil {
		return nil, err
	}

	common.LogSensitiveAccess(ctx, common.GetCtxUserID(ctx), "admin",
		"ai_analysis_report", req.Id, "view", "查看AI请求分析报告详情")

	data := one.Map()
	if data == nil {
		data = map[string]any{}
	}
	data["findings"] = findings.List()
	return &v1.AiAnalysisReportDetailRes{Data: data}, nil
}

// AiAnalysisReportConversation 按报告内发现还原完整会话：
// finding_id 定位发现的锚点日志，调核心会话重组（平台侧副本）。
func (s *sAdmin) AiAnalysisReportConversation(ctx context.Context, req *v1.AiAnalysisReportConversationReq) (*v1.AiAnalysisReportConversationRes, error) {
	one, err := g.DB().Model(aianalysis.TableFindings).Ctx(ctx).
		Where("id = ? AND report_id = ?", req.FindingId, req.Id).
		Fields("id, anchor_log_id").
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, common.NewNotFoundError("发现记录")
	}
	anchorLogID := one["anchor_log_id"].Int64()

	// admin 全局视角 tenantID=0，平台侧正文副本
	view, err := common.LoadConversationView(ctx, anchorLogID, 0, common.SysBodyColumns())
	if err != nil {
		return nil, err
	}

	common.LogSensitiveAccess(ctx, common.GetCtxUserID(ctx), "admin",
		"ai_analysis_report", req.Id, "view_conversation", "查看分析报告关联的完整会话")

	b, err := json.Marshal(view)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	_ = json.Unmarshal(b, &data)
	return &v1.AiAnalysisReportConversationRes{Data: data}, nil
}

// AiAnalysisReportDelete 删除报告，连同发现明细。
func (s *sAdmin) AiAnalysisReportDelete(ctx context.Context, req *v1.AiAnalysisReportDeleteReq) (*v1.AiAnalysisReportDeleteRes, error) {
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := g.DB().Model(aianalysis.TableFindings).Ctx(ctx).Where("report_id", req.Id).Delete(); err != nil {
			return err
		}
		if _, err := g.DB().Model(aianalysis.TableReports).Ctx(ctx).Where("id", req.Id).Delete(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &v1.AiAnalysisReportDeleteRes{}, nil
}

// AiAnalysisJobList 任务列表与进度。
func (s *sAdmin) AiAnalysisJobList(ctx context.Context, req *v1.AiAnalysisJobListReq) (*v1.AiAnalysisJobListRes, error) {
	m := g.DB().Model(aianalysis.TableJobs).Ctx(ctx)
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}
	if req.TenantID > 0 {
		m = m.Where("tenant_id", req.TenantID)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}
	all, err := m.Page(req.Page, req.PageSize).Order("id DESC").All()
	if err != nil {
		return nil, err
	}
	list := all.List()
	if list == nil {
		list = []map[string]any{}
	}
	return &v1.AiAnalysisJobListRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

// AiAnalysisConfigGet 读取分析配置（与设置页读取同构，敏感字段掩码显示）。
func (s *sAdmin) AiAnalysisConfigGet(ctx context.Context, _ *v1.AiAnalysisConfigGetReq) (*v1.AiAnalysisConfigGetRes, error) {
	items := common.Config().GetCategoryWithValues(ctx, "ai_analysis")
	list := make([]v1.AdminSettingItem, len(items))
	for i, item := range items {
		list[i] = v1.AdminSettingItem{
			Key:         item.Key,
			Value:       common.TypedValue(item.Type, item.Value, item.Default),
			Type:        string(item.Type),
			Label:       item.Label,
			Description: item.Description,
			Sensitive:   item.Sensitive,
			Validation:  item.Validation,
			Default:     common.TypedValue(item.Type, "", item.Default),
		}
	}
	return &v1.AiAnalysisConfigGetRes{List: list}, nil
}

// AiAnalysisConfigUpdate 更新分析配置。敏感字段为掩码/空值时保持现值
// （UpdateCategory 内统一处理）；巡查计划变更经设置监听即时重排 cron。
func (s *sAdmin) AiAnalysisConfigUpdate(ctx context.Context, req *v1.AiAnalysisConfigUpdateReq) (*v1.AiAnalysisConfigUpdateRes, error) {
	// Normalize interface{} values to strings for storage
	strValues := make(map[string]string, len(req.Settings))
	for key, val := range req.Settings {
		strValues[key] = common.NormalizeSettingValue(val)
	}

	// 巡查计划保存前校验表达式（非法表达式入库会让巡查静默停摆，必须在此拦下；
	// 掩码值表示保持现值，跳过校验）
	if expr, ok := strValues["ai_analysis_patrol_cron"]; ok && expr != "" && expr != "******" {
		if _, err := cron.ParseStandard(expr); err != nil {
			return nil, common.NewBadRequestError("巡查计划不是合法的 cron 表达式（5 段式，如 0 3 * * *）")
		}
	}

	if err := common.Config().UpdateCategory(ctx, "ai_analysis", strValues); err != nil {
		return nil, err
	}
	return &v1.AiAnalysisConfigUpdateRes{}, nil
}

// AiAnalysisTest 用已保存配置发起一次最小补全做连通性测试，
// 失败原因原样返回给管理员定位问题（测试失败本身是有效结果，不返回 error）。
func (s *sAdmin) AiAnalysisTest(ctx context.Context, _ *v1.AiAnalysisTestReq) (*v1.AiAnalysisTestRes, error) {
	cfg := aianalysis.LoadConfig(ctx)
	if err := cfg.Validate(); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	result := aianalysis.NewAIClient(cfg).TestConnection(ctx)
	return &v1.AiAnalysisTestRes{
		OK:         result.OK,
		Model:      result.Model,
		LatencyMs:  result.LatencyMs,
		TokensUsed: result.TokensUsed,
		Reply:      result.Reply,
		Error:      result.Error,
	}, nil
}
