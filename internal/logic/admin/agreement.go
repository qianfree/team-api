package admin

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"
)

// CreateAgreement 创建协议版本
func (s *sAdmin) CreateAgreement(ctx context.Context, req *v1.AgreementCreateReq) (*v1.AgreementCreateRes, error) {
	if _, ok := consts.ValidAgreementCodes[req.Code]; !ok {
		return nil, common.NewBusinessError(consts.CodeAgreementCodeInvalid, consts.MsgAgreementCodeInvalid)
	}

	// 检查 code+version 唯一性
	count, err := dao.SysAgreements.Ctx(ctx).
		Where("code", req.Code).
		Where("version", req.Version).
		Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeAgreementVersionExists, consts.MsgAgreementVersionExists)
	}

	forceAccept := true
	if req.ForceAccept != nil {
		forceAccept = *req.ForceAccept
	}

	result, err := dao.SysAgreements.Ctx(ctx).Data(do.SysAgreements{
		Code:        req.Code,
		Version:     req.Version,
		Title:       req.Title,
		Content:     req.Content,
		Summary:     req.Summary,
		Status:      "draft",
		IsCurrent:   false,
		ForceAccept: forceAccept,
		CreatedBy:   common.GetCtxUserID(ctx),
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.AgreementCreateRes{Id: id}, nil
}

// ListAgreements 协议版本列表
func (s *sAdmin) ListAgreements(ctx context.Context, req *v1.AgreementListReq) (*v1.AgreementListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.SysAgreements.Ctx(ctx)
	if req.Code != "" {
		query = query.Where("code", req.Code)
	}
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}

	var total int
	rows := make([]*v1.AgreementItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.AgreementListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetAgreement 协议版本详情
func (s *sAdmin) GetAgreement(ctx context.Context, req *v1.AgreementGetReq) (*v1.AgreementGetRes, error) {
	var res *v1.AgreementGetRes
	err := dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Scan(&res)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, common.NewBusinessError(consts.CodeAgreementNotFound, consts.MsgAgreementNotFound)
	}
	return res, nil
}

// UpdateAgreement 更新协议版本（仅 draft）
func (s *sAdmin) UpdateAgreement(ctx context.Context, req *v1.AgreementUpdateReq) (*v1.AgreementUpdateRes, error) {
	var agr *struct {
		Id     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Scan(&agr)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if agr == nil {
		return nil, common.NewBusinessError(consts.CodeAgreementNotFound, consts.MsgAgreementNotFound)
	}
	if agr.Status != "draft" {
		return nil, common.NewBusinessError(consts.CodeAgreementNotDraft, consts.MsgAgreementNotDraft)
	}

	data := do.SysAgreements{
		Title:   req.Title,
		Content: req.Content,
		Summary: req.Summary,
	}
	if req.ForceAccept != nil {
		data.ForceAccept = *req.ForceAccept
	}

	_, err = dao.SysAgreements.Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.AgreementUpdateRes{}, nil
}

// DeleteAgreement 删除协议版本（仅 draft）
func (s *sAdmin) DeleteAgreement(ctx context.Context, req *v1.AgreementDeleteReq) (*v1.AgreementDeleteRes, error) {
	var agr *struct {
		Id     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Scan(&agr)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if agr == nil {
		return nil, common.NewBusinessError(consts.CodeAgreementNotFound, consts.MsgAgreementNotFound)
	}
	if agr.Status != "draft" {
		return nil, common.NewBusinessError(consts.CodeAgreementNotDraft, consts.MsgAgreementNotDraft)
	}

	_, err = dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.AgreementDeleteRes{}, nil
}

// PublishAgreement 发布协议版本
func (s *sAdmin) PublishAgreement(ctx context.Context, req *v1.AgreementPublishReq) (*v1.AgreementPublishRes, error) {
	var agr *struct {
		Id     int64  `json:"id"`
		Code   string `json:"code"`
		Status string `json:"status"`
	}
	err := dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Scan(&agr)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if agr == nil {
		return nil, common.NewBusinessError(consts.CodeAgreementNotFound, consts.MsgAgreementNotFound)
	}
	if agr.Status == "published" {
		return &v1.AgreementPublishRes{}, nil
	}
	if agr.Status != "draft" {
		return nil, common.NewBusinessError(consts.CodeAgreementNotDraft, consts.MsgAgreementNotDraft)
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return common.PublishAgreementTx(ctx, tx, agr.Id, agr.Code)
	})
	if err != nil {
		return nil, err
	}

	return &v1.AgreementPublishRes{}, nil
}

// ListAgreementAcceptances 协议接受记录列表
func (s *sAdmin) ListAgreementAcceptances(ctx context.Context, req *v1.AgreementAcceptanceListReq) (*v1.AgreementAcceptanceListRes, error) {
	// 先检查协议是否存在
	var agr *struct {
		Id int64 `json:"id"`
	}
	err := dao.SysAgreements.Ctx(ctx).Where("id", req.Id).Scan(&agr)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if agr == nil {
		return nil, common.NewBusinessError(consts.CodeAgreementNotFound, consts.MsgAgreementNotFound)
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var total int
	rows := make([]*v1.AgreementAcceptanceItem, 0)
	err = dao.SysAgreementAcceptances.Ctx(ctx).
		Where("agreement_id", req.Id).
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.AgreementAcceptanceListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListAdminPendingAgreements 当前管理员待接受的协议
func (s *sAdmin) ListAdminPendingAgreements(ctx context.Context, req *v1.AdminAgreementPendingReq) (*v1.AdminAgreementPendingRes, error) {
	userID := common.GetCtxUserID(ctx)
	list, err := common.GetPendingAgreements(ctx, "admin", userID)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.PendingAgreementItem, 0, len(list))
	for _, a := range list {
		items = append(items, &v1.PendingAgreementItem{
			Id:      a.Id,
			Code:    a.Code,
			Title:   a.Title,
			Version: a.Version,
			Content: a.Content,
		})
	}

	return &v1.AdminAgreementPendingRes{List: items}, nil
}

// AcceptAdminAgreements 管理员接受协议
func (s *sAdmin) AcceptAdminAgreements(ctx context.Context, req *v1.AdminAgreementAcceptReq) (*v1.AdminAgreementAcceptRes, error) {
	userID := common.GetCtxUserID(ctx)
	r := g.RequestFromCtx(ctx)
	ipAddress := r.GetClientIp()
	userAgent := r.GetHeader("User-Agent")

	err := common.AcceptAgreements(ctx, "admin", userID, req.AgreementIds, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	return &v1.AdminAgreementAcceptRes{}, nil
}
