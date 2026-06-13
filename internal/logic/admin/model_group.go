package admin

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// ==================== 分组 CRUD ====================

// ListModelGroups 模型分组列表
func (s *sAdmin) ListModelGroups(ctx context.Context, req *v1.ModelGroupListReq) (*v1.ModelGroupListRes, error) {
	query := g.DB().Model("mdl_model_groups mg").Ctx(ctx).
		LeftJoin("(SELECT group_id, COUNT(*) AS model_count FROM mdl_group_models GROUP BY group_id) mc ON mc.group_id = mg.id").
		LeftJoin("(SELECT group_id, COUNT(*) AS tenant_count FROM mdl_tenant_groups GROUP BY group_id) tc ON tc.group_id = mg.id")

	if req.Status != "" {
		query = query.Where("mg.status", req.Status)
	}
	if req.Search != "" {
		query = query.Where("mg.name LIKE ? OR mg.code LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	countQuery := g.DB().Model("mdl_model_groups mg").Ctx(ctx)
	if req.Status != "" {
		countQuery = countQuery.Where("mg.status", req.Status)
	}
	if req.Search != "" {
		countQuery = countQuery.Where("mg.name LIKE ? OR mg.code LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}
	total, _ := countQuery.Count()

	var results []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Code        string `json:"code"`
		Description string `json:"description"`
		Status      string `json:"status"`
		IsDefault   bool   `json:"is_default"`
		ModelCount  int    `json:"model_count"`
		TenantCount int    `json:"tenant_count"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	err := query.
		Fields("mg.id, mg.name, mg.code, mg.description, mg.status, mg.is_default, COALESCE(mc.model_count, 0) AS model_count, COALESCE(tc.tenant_count, 0) AS tenant_count, mg.created_at, mg.updated_at").
		OrderAsc("mg.id").
		Page(req.Page, req.PageSize).
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ModelGroupItem, 0, len(results))
	for _, r := range results {
		list = append(list, v1.ModelGroupItem{
			ID:          r.ID,
			Name:        r.Name,
			Code:        r.Code,
			Description: r.Description,
			Status:      r.Status,
			IsDefault:   r.IsDefault,
			ModelCount:  r.ModelCount,
			TenantCount: r.TenantCount,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
	}

	return &v1.ModelGroupListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// CreateModelGroup 创建模型分组
func (s *sAdmin) CreateModelGroup(ctx context.Context, req *v1.ModelGroupCreateReq) (*v1.ModelGroupCreateRes, error) {
	count, _ := dao.MdlModelGroups.Ctx(ctx).Where("code", req.Code).Count()
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeModelGroupCodeExists, consts.MsgModelGroupCodeExists)
	}

	result, err := dao.MdlModelGroups.Ctx(ctx).Insert(do.MdlModelGroups{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		IsDefault:   req.IsDefault,
	})
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	if len(req.ModelIds) > 0 {
		for _, modelID := range req.ModelIds {
			if _, err := dao.MdlGroupModels.Ctx(ctx).Insert(do.MdlGroupModels{
				GroupId: id,
				ModelId: modelID,
			}); err != nil {
				return nil, gerror.Wrapf(err, "insert group model association for model %d", modelID)
			}
		}
	}

	return &v1.ModelGroupCreateRes{ID: id}, nil
}

// UpdateModelGroup 更新模型分组
func (s *sAdmin) UpdateModelGroup(ctx context.Context, req *v1.ModelGroupUpdateReq) (*v1.ModelGroupUpdateRes, error) {
	count, _ := dao.MdlModelGroups.Ctx(ctx).Where("id", req.ID).Count()
	if count == 0 {
		return nil, common.NewBusinessError(consts.CodeModelGroupNotFound, consts.MsgModelGroupNotFound)
	}

	data := do.MdlModelGroups{}
	if req.Name != "" {
		data.Name = req.Name
	}
	if req.Description != "" {
		data.Description = req.Description
	}
	if req.Status != "" {
		data.Status = req.Status
	}
	if req.IsDefault != nil {
		data.IsDefault = *req.IsDefault
	}

	_, err := dao.MdlModelGroups.Ctx(ctx).Where("id", req.ID).Data(data).Update()
	if err != nil {
		return nil, err
	}

	if req.Status != "" {
		invalidateTenantsInGroup(ctx, req.ID)
	}

	return nil, nil
}

// DeleteModelGroup 删除模型分组
func (s *sAdmin) DeleteModelGroup(ctx context.Context, req *v1.ModelGroupDeleteReq) (*v1.ModelGroupDeleteRes, error) {
	count, _ := dao.MdlModelGroups.Ctx(ctx).Where("id", req.ID).Count()
	if count == 0 {
		return nil, common.NewBusinessError(consts.CodeModelGroupNotFound, consts.MsgModelGroupNotFound)
	}

	tenantCount, _ := dao.MdlTenantGroups.Ctx(ctx).Where("group_id", req.ID).Count()
	if tenantCount > 0 {
		return nil, common.NewBusinessError(consts.CodeModelGroupHasTenants, consts.MsgModelGroupHasTenants)
	}

	if _, err := dao.MdlGroupModels.Ctx(ctx).Where("group_id", req.ID).Delete(); err != nil {
		return nil, gerror.Wrapf(err, "delete group models for group %d", req.ID)
	}
	_, err := dao.MdlModelGroups.Ctx(ctx).Where("id", req.ID).Delete()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ListModelGroupOptions 分组选项列表（不分页，用于下拉选择）
func (s *sAdmin) ListModelGroupOptions(ctx context.Context, req *v1.ModelGroupOptionsReq) (*v1.ModelGroupOptionsRes, error) {
	query := g.DB().Model("mdl_model_groups mg").Ctx(ctx).
		LeftJoin("(SELECT group_id, COUNT(*) AS model_count FROM mdl_group_models GROUP BY group_id) mc ON mc.group_id = mg.id")

	if req.Status != "" {
		query = query.Where("mg.status", req.Status)
	} else {
		query = query.Where("mg.status", "active")
	}

	var results []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Code       string `json:"code"`
		IsDefault  bool   `json:"is_default"`
		ModelCount int    `json:"model_count"`
	}

	err := query.
		Fields("mg.id, mg.name, mg.code, mg.is_default, COALESCE(mc.model_count, 0) AS model_count").
		OrderAsc("mg.id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ModelGroupOptionItem, 0, len(results))
	for _, r := range results {
		list = append(list, v1.ModelGroupOptionItem{
			ID:         r.ID,
			Name:       r.Name,
			Code:       r.Code,
			IsDefault:  r.IsDefault,
			ModelCount: r.ModelCount,
		})
	}

	return &v1.ModelGroupOptionsRes{List: list}, nil
}

// ==================== 分组模型管理 ====================

// ListGroupModels 查看分组内模型列表
func (s *sAdmin) ListGroupModels(ctx context.Context, req *v1.GroupModelsListReq) (*v1.GroupModelsListRes, error) {
	var results []struct {
		ModelId   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Category  string `json:"category"`
		Status    string `json:"status"`
	}

	err := dao.MdlGroupModels.Ctx(ctx).As("gm").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		Where("gm.group_id", req.ID).
		Fields("m.model_id, m.model_name, m.category, m.status").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.GroupModelItem, 0, len(results))
	for _, r := range results {
		list = append(list, v1.GroupModelItem{
			ModelId:   r.ModelId,
			ModelName: r.ModelName,
			Category:  r.Category,
			Status:    r.Status,
		})
	}

	return &v1.GroupModelsListRes{List: list}, nil
}

// SetGroupModels 设置分组内模型（全量替换）
func (s *sAdmin) SetGroupModels(ctx context.Context, req *v1.GroupModelsSetReq) (*v1.GroupModelsSetRes, error) {
	count, _ := dao.MdlModelGroups.Ctx(ctx).Where("id", req.ID).Count()
	if count == 0 {
		return nil, common.NewBusinessError(consts.CodeModelGroupNotFound, consts.MsgModelGroupNotFound)
	}

	// 删除旧关联
	_, err := dao.MdlGroupModels.Ctx(ctx).Where("group_id", req.ID).Delete()
	if err != nil {
		return nil, err
	}

	// 批量插入新关联
	if len(req.ModelIds) > 0 {
		insertData := make([]do.MdlGroupModels, 0, len(req.ModelIds))
		for _, modelID := range req.ModelIds {
			insertData = append(insertData, do.MdlGroupModels{
				GroupId: req.ID,
				ModelId: modelID,
			})
		}
		_, err = dao.MdlGroupModels.Ctx(ctx).Batch(len(insertData)).Insert(insertData)
		if err != nil {
			return nil, err
		}
	}

	invalidateTenantsInGroup(ctx, req.ID)

	return nil, nil
}

// ==================== 租户分组管理 ====================

// ListTenantGroups 查看租户关联的分组列表
func (s *sAdmin) ListTenantGroups(ctx context.Context, req *v1.TenantGroupsListReq) (*v1.TenantGroupsListRes, error) {
	var results []struct {
		GroupID    int64  `json:"group_id"`
		Name       string `json:"name"`
		Code       string `json:"code"`
		Status     string `json:"status"`
		ModelCount int    `json:"model_count"`
	}

	err := dao.MdlTenantGroups.Ctx(ctx).As("tg").
		InnerJoin("mdl_model_groups g ON tg.group_id = g.id").
		LeftJoin("(SELECT group_id, COUNT(*) AS model_count FROM mdl_group_models GROUP BY group_id) mc ON mc.group_id = g.id").
		Where("tg.tenant_id", req.TenantID).
		Fields("g.id AS group_id, g.name, g.code, g.status, COALESCE(mc.model_count, 0) AS model_count").
		OrderAsc("g.id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantGroupItem, 0, len(results))
	for _, r := range results {
		list = append(list, v1.TenantGroupItem{
			GroupID:    r.GroupID,
			Name:       r.Name,
			Code:       r.Code,
			Status:     r.Status,
			ModelCount: r.ModelCount,
		})
	}

	return &v1.TenantGroupsListRes{List: list}, nil
}

// SetTenantGroups 设置租户关联的分组（全量替换）
func (s *sAdmin) SetTenantGroups(ctx context.Context, req *v1.TenantGroupsSetReq) (*v1.TenantGroupsSetRes, error) {
	// 删除旧关联
	_, err := dao.MdlTenantGroups.Ctx(ctx).Where("tenant_id", req.TenantID).Delete()
	if err != nil {
		return nil, err
	}

	// 批量插入新关联
	if len(req.GroupIds) > 0 {
		insertData := make([]do.MdlTenantGroups, 0, len(req.GroupIds))
		for _, groupID := range req.GroupIds {
			insertData = append(insertData, do.MdlTenantGroups{
				TenantId: req.TenantID,
				GroupId:  groupID,
			})
		}
		_, err = dao.MdlTenantGroups.Ctx(ctx).Batch(len(insertData)).Insert(insertData)
		if err != nil {
			return nil, err
		}
	}

	invalidateTenantGroupCache(ctx, req.TenantID)
	billing.ClearTenantPriceCache(ctx, req.TenantID)

	return nil, nil
}

// ==================== 缓存失效辅助函数 ====================

func invalidateTenantGroupCache(ctx context.Context, tenantID int64) {
	common.TenantGroupModelCache.Delete(ctx, fmt.Sprintf("%d", tenantID))
}

func invalidateTenantsForModel(ctx context.Context, modelID int64) {
	var groupIDs []struct {
		GroupId int64 `json:"group_id"`
	}
	dao.MdlGroupModels.Ctx(ctx).
		Where("model_id", modelID).
		Fields("group_id").
		Scan(&groupIDs)

	for _, g := range groupIDs {
		invalidateTenantsInGroup(ctx, g.GroupId)
	}

	// 同时清除直接分配了该模型的租户缓存（mdl_tenant_models）
	var tenantIDs []struct {
		TenantId int64 `json:"tenant_id"`
	}
	dao.MdlTenantModels.Ctx(ctx).
		Where("model_id", modelID).
		Fields("tenant_id").
		Scan(&tenantIDs)

	for _, t := range tenantIDs {
		invalidateTenantGroupCache(ctx, t.TenantId)
	}
}

func invalidateTenantsInGroup(ctx context.Context, groupID int64) {
	var tenants []struct {
		TenantId int64 `json:"tenant_id"`
	}
	dao.MdlTenantGroups.Ctx(ctx).
		Where("group_id", groupID).
		Fields("tenant_id").
		Scan(&tenants)

	for _, t := range tenants {
		invalidateTenantGroupCache(ctx, t.TenantId)
	}
}
