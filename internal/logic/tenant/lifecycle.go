package tenant

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// 合法状态转换映射
var validTransitions = map[string][]string{
	"trial":     {"active", "frozen"},
	"active":    {"past_due", "suspended", "closing"},
	"past_due":  {"active", "frozen", "suspended"},
	"frozen":    {"active", "terminated"},
	"closing":   {"active", "closed"},
	"suspended": {"active"},
}

// TransitionTenantStatus 租户状态转换（内部函数，被定时任务调用）
func TransitionTenantStatus(ctx context.Context, tenantID int64, newStatus string) error {
	var current struct {
		Status string `json:"status"`
	}
	err := dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Fields("status").
		Scan(&current)
	if err != nil {
		return err
	}

	allowed, ok := validTransitions[current.Status]
	if !ok {
		return gerror.Newf("invalid current status: %s", current.Status)
	}
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return gerror.Newf("cannot transition from %s to %s", current.Status, newStatus)
	}

	now := time.Now()

	data := do.TntTenants{Status: newStatus}
	switch newStatus {
	case "frozen":
		data.FrozenAt = gtime.NewFromTime(now.Add(30 * 24 * time.Hour))
		data.DataRemovalAt = gtime.NewFromTime(now.Add(30 * 24 * time.Hour))
	case "past_due":
		data.GracePeriodEndsAt = gtime.NewFromTime(now.Add(7 * 24 * time.Hour))
	case "closing":
		data.ClosingRequestedAt = gtime.Now()
	}

	_, err = dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Data(data).Update()
	return err
}

// RequestClosure 申请关户
func (s *sTenant) RequestClosure(ctx context.Context, req *v1.TenantRequestClosureReq) (*v1.TenantRequestClosureRes, error) {
	tenantID := ctxTenantID(ctx)
	if err := TransitionTenantStatus(ctx, tenantID, "closing"); err != nil {
		return nil, err
	}
	return &v1.TenantRequestClosureRes{}, nil
}

// CancelClosure 取消关户
func (s *sTenant) CancelClosure(ctx context.Context, req *v1.TenantCancelClosureReq) (*v1.TenantCancelClosureRes, error) {
	tenantID := ctxTenantID(ctx)
	if err := TransitionTenantStatus(ctx, tenantID, "active"); err != nil {
		return nil, err
	}
	return &v1.TenantCancelClosureRes{}, nil
}
