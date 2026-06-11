package tenant

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// ListTenantPendingAgreements 当前租户用户待接受的协议
func (s *sTenant) ListTenantPendingAgreements(ctx context.Context, req *v1.TenantAgreementPendingReq) (*v1.TenantAgreementPendingRes, error) {
	userID := common.GetCtxUserID(ctx)
	list, err := common.GetPendingAgreements(ctx, "tenant", userID)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.TenantPendingAgreementItem, 0, len(list))
	for _, a := range list {
		items = append(items, &v1.TenantPendingAgreementItem{
			Id:      a.Id,
			Code:    a.Code,
			Title:   a.Title,
			Version: a.Version,
			Content: a.Content,
		})
	}

	return &v1.TenantAgreementPendingRes{List: items}, nil
}

// AcceptTenantAgreements 租户用户接受协议
func (s *sTenant) AcceptTenantAgreements(ctx context.Context, req *v1.TenantAgreementAcceptReq) (*v1.TenantAgreementAcceptRes, error) {
	userID := common.GetCtxUserID(ctx)
	r := g.RequestFromCtx(ctx)
	ipAddress := r.GetClientIp()
	userAgent := r.GetHeader("User-Agent")

	err := common.AcceptAgreements(ctx, "tenant", userID, req.AgreementIds, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	return &v1.TenantAgreementAcceptRes{}, nil
}

// ListCurrentAgreements 获取所有当前生效的协议列表（公开）
func (s *sTenant) ListCurrentAgreements(ctx context.Context, req *v1.AgreementCurrentListReq) (*v1.AgreementCurrentListRes, error) {
	list, err := common.GetCurrentAgreements(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.PublicAgreementItem, 0, len(list))
	for _, a := range list {
		items = append(items, &v1.PublicAgreementItem{
			Id:          a.Id,
			Code:        a.Code,
			Title:       a.Title,
			Version:     a.Version,
			ForceAccept: a.ForceAccept,
			PublishedAt: a.PublishedAt,
		})
	}

	return &v1.AgreementCurrentListRes{List: items}, nil
}

// GetCurrentAgreementByCode 按标识码获取当前协议详情（公开）
func (s *sTenant) GetCurrentAgreementByCode(ctx context.Context, req *v1.AgreementCurrentGetReq) (*v1.AgreementCurrentGetRes, error) {
	detail, err := common.GetCurrentAgreementByCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, common.NewNotFoundError("协议不存在")
	}

	return &v1.AgreementCurrentGetRes{
		Id:          detail.Id,
		Code:        detail.Code,
		Title:       detail.Title,
		Version:     detail.Version,
		Content:     detail.Content,
		ForceAccept: detail.ForceAccept,
		PublishedAt: detail.PublishedAt,
	}, nil
}
