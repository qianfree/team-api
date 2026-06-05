package admin

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListRedemptions 获取兑换码列表
func (s *sAdmin) ListRedemptions(ctx context.Context, req *v1.RedemptionListReq) (*v1.RedemptionListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.OrdRedemptions.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}

	var total int
	items := make([]*v1.RedemptionItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.RedemptionListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// BatchCreateRedemptions 批量生成兑换码
func (s *sAdmin) BatchCreateRedemptions(ctx context.Context, req *v1.RedemptionCreateReq) (*v1.RedemptionCreateRes, error) {
	count := req.Count
	if count <= 0 || count > 1000 {
		return nil, common.NewBadRequestError("count must be between 1 and 1000")
	}
	codeType := req.Type
	if codeType != "quota" && codeType != "plan" && codeType != "duration" {
		return nil, common.NewBadRequestError("type must be quota, plan, or duration")
	}

	now := gtime.Now()
	batchNo := fmt.Sprintf("BATCH%s%04d", now.Format("YmdHis"), now.UnixNano()%10000)
	created := 0

	err := dao.OrdRedemptions.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for i := 0; i < count; i++ {
			code, err := generateCode(12)
			if err != nil {
				return err
			}

			_, err = tx.Model("ord_redemptions").Ctx(ctx).Insert(do.OrdRedemptions{
				Code:         code,
				Type:         codeType,
				Value:        req.Value,
				PlanId:       req.PlanID,
				DurationDays: req.DurationDays,
				MaxUses:      1,
				BatchNo:      batchNo,
				Status:       "active",
				ExpiresAt:    gtime.Now().Add(90 * 24 * time.Hour),
			})
			if err != nil {
				return err
			}
			created++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &v1.RedemptionCreateRes{Created: created}, nil
}

// DisableRedemption 禁用兑换码
func (s *sAdmin) DisableRedemption(ctx context.Context, req *v1.RedemptionDisableReq) (*v1.RedemptionDisableRes, error) {
	count, err := dao.OrdRedemptions.Ctx(ctx).Where("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewNotFoundError("兑换码")
	}
	_, err = dao.OrdRedemptions.Ctx(ctx).
		Where("id", req.Id).
		Where("status", "active").
		Data(do.OrdRedemptions{
			Status: "disabled",
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.RedemptionDisableRes{}, nil
}

func generateCode(length int) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// ListRedemptionUsages 获取兑换码使用记录
func (s *sAdmin) ListRedemptionUsages(ctx context.Context, req *v1.RedemptionUsagesReq) (*v1.RedemptionUsagesRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	type usageRow struct {
		Id            int64       `json:"id" orm:"id"`
		RedemptionId  int64       `json:"redemption_id" orm:"redemption_id"`
		TenantId      int64       `json:"tenant_id" orm:"tenant_id"`
		UserId        int64       `json:"user_id" orm:"user_id"`
		Type          string      `json:"type" orm:"type"`
		Value         float64     `json:"value" orm:"value"`
		TransactionId int64       `json:"transaction_id" orm:"transaction_id"`
		CreatedAt     *gtime.Time `json:"created_at" orm:"created_at"`
		Code          string      `json:"code" orm:"code"`
		TenantName    string      `json:"tenant_name" orm:"tenant_name"`
		Username      string      `json:"username" orm:"username"`
	}

	m := dao.OrdRedemptionUsages.Ctx(ctx).
		LeftJoin("ord_redemptions r", "ord_redemption_usages.redemption_id = r.id").
		LeftJoin("tnt_tenants t", "ord_redemption_usages.tenant_id = t.id").
		LeftJoin("tnt_users u", "ord_redemption_usages.user_id = u.id AND ord_redemption_usages.tenant_id = u.tenant_id").
		Fields("ord_redemption_usages.*, COALESCE(r.code, '') as code, COALESCE(t.name, '') as tenant_name, COALESCE(u.username, '') as username")

	if req.RedemptionId > 0 {
		m = m.Where("ord_redemption_usages.redemption_id", req.RedemptionId)
	}
	if req.TenantId > 0 {
		m = m.Where("ord_redemption_usages.tenant_id", req.TenantId)
	}

	var total int
	rows := make([]usageRow, 0)
	err := m.OrderDesc("ord_redemption_usages.created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.RedemptionUsageItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &v1.RedemptionUsageItem{
			Id:            r.Id,
			RedemptionId:  r.RedemptionId,
			Code:          r.Code,
			TenantId:      r.TenantId,
			TenantName:    r.TenantName,
			UserId:        r.UserId,
			Username:      r.Username,
			Type:          r.Type,
			Value:         r.Value,
			TransactionId: r.TransactionId,
			CreatedAt:     r.CreatedAt,
		})
	}

	return &v1.RedemptionUsagesRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ExportRedemptions exports redemption list to CSV or Excel.
func (s *sAdmin) ExportRedemptions(ctx context.Context, req *v1.RedemptionExportReq) (*v1.RedemptionExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "code", Header: "兑换码"},
		{Field: "type", Header: "类型"},
		{Field: "value", Header: "面值"},
		{Field: "used_count", Header: "已用次数"},
		{Field: "status", Header: "状态"},
		{Field: "batch_no", Header: "批次号"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "兑换码_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	redemptionFields := "id, code, type, value, used_count, status, batch_no, created_at"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.OrdRedemptions.Ctx(ctx)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			var batch []struct {
				Id        int64       `json:"id"`
				Code      string      `json:"code"`
				Type      string      `json:"type"`
				Value     float64     `json:"value"`
				UsedCount int         `json:"used_count"`
				Status    string      `json:"status"`
				BatchNo   string      `json:"batch_no"`
				CreatedAt *gtime.Time `json:"created_at"`
			}
			if err := query.Fields(redemptionFields).OrderDesc("created_at").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, item := range batch {
				if !yield(map[string]any{
					"id":         item.Id,
					"code":       item.Code,
					"type":       item.Type,
					"value":      item.Value,
					"used_count": item.UsedCount,
					"status":     item.Status,
					"batch_no":   item.BatchNo,
					"created_at": item.CreatedAt.String(),
				}) {
					return
				}
			}
			if len(batch) < 1000 {
				break
			}
			offset += 1000
		}
	})
}
