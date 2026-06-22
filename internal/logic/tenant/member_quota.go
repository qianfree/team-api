package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"

	do "github.com/qianfree/team-api/internal/model/do"
)

func (s *sTenant) MemberQuota(ctx context.Context, req *v1.TenantMemberQuotaReq) (*v1.TenantMemberQuotaRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var row *struct {
		QuotaType    string     `json:"quota_type"`
		QuotaLimit   float64    `json:"quota_limit"`
		QuotaUsed    float64    `json:"quota_used"`
		QuotaPeriod  string     `json:"quota_period"`
		QuotaResetAt *time.Time `json:"quota_reset_at"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("quota_type, quota_limit, quota_used, quota_period, quota_reset_at").
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, common.NewNotFoundError("成员")
	}
	res := &v1.TenantMemberQuotaRes{
		QuotaType:  row.QuotaType,
		QuotaLimit: row.QuotaLimit,
		QuotaUsed:  row.QuotaUsed,
		Period:     row.QuotaPeriod,
	}

	if row.QuotaType == "periodic" && row.QuotaPeriod != "" {
		nextReset := calcNextReset(row.QuotaResetAt, row.QuotaPeriod)
		if nextReset != nil {
			res.NextResetAt = nextReset.Format("2006-01-02T15:04:05Z07:00")
		}
	}

	return res, nil
}

func (s *sTenant) MemberQuotaSet(ctx context.Context, req *v1.TenantMemberQuotaSetReq) (*v1.TenantMemberQuotaSetRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	if req.QuotaType == "periodic" && req.Period == "" {
		return nil, common.NewBadRequestError("周期类型不能为空")
	}

	data := do.TntUsers{
		QuotaType:   req.QuotaType,
		QuotaLimit:  req.QuotaLimit,
		QuotaPeriod: nil,
	}

	if req.QuotaType == "periodic" {
		data.QuotaPeriod = req.Period
		now := gtime.Now()
		data.QuotaResetAt = now
	}
	if req.QuotaType == "none" {
		data.QuotaLimit = 0
	}

	_, err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	invalidateMemberQuotaCache(ctx, tenantID, req.Id)

	return &v1.TenantMemberQuotaSetRes{}, nil
}

func calcNextReset(resetAt *time.Time, period string) *time.Time {
	now := time.Now().UTC()
	var base time.Time
	if resetAt != nil {
		base = resetAt.UTC()
	} else {
		base = now
	}

	var next time.Time
	switch period {
	case "day":
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		for next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
	case "week":
		weekday := int(base.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		mondayOfWeek := base.AddDate(0, 0, 1-weekday)
		next = time.Date(mondayOfWeek.Year(), mondayOfWeek.Month(), mondayOfWeek.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7)
		for next.Before(now) {
			next = next.AddDate(0, 0, 7)
		}
	case "month":
		next = time.Date(base.Year(), base.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		for next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
	default:
		return nil
	}

	return &next
}

func invalidateMemberQuotaCache(ctx context.Context, tenantID, userID int64) {
	key := fmt.Sprintf("member_quota:%d:%d", tenantID, userID)
	_, _ = g.Redis().Del(ctx, key)
}
