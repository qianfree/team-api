package common

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/do"
)

// PendingAgreementInfo 待接受协议信息
type PendingAgreementInfo struct {
	Id      int64  `json:"id"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Content string `json:"content"`
}

// CurrentAgreementItem 当前生效协议摘要
type CurrentAgreementItem struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
}

// CurrentAgreementDetail 当前生效协议详情
type CurrentAgreementDetail struct {
	Id          int64       `json:"id"`
	Code        string      `json:"code"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	Content     string      `json:"content"`
	ForceAccept bool        `json:"force_accept"`
	PublishedAt *gtime.Time `json:"published_at,omitempty"`
}

// GetPendingAgreements 获取用户待接受的协议列表
func GetPendingAgreements(ctx context.Context, userType string, userID int64) ([]*PendingAgreementInfo, error) {
	rows := make([]*PendingAgreementInfo, 0)
	err := dao.SysAgreements.Ctx(ctx).
		Where("is_current", true).
		Where("status", "published").
		Where("force_accept", true).
		Where(fmt.Sprintf(
			"id NOT IN (SELECT agreement_id FROM sys_agreement_acceptances WHERE user_type = '%s' AND user_id = %d)",
			userType, userID,
		)).
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// HasPendingAgreements 快速检查是否有待接受协议
func HasPendingAgreements(ctx context.Context, userType string, userID int64) bool {
	count, err := dao.SysAgreements.Ctx(ctx).
		Where("is_current", true).
		Where("status", "published").
		Where("force_accept", true).
		Where(fmt.Sprintf(
			"id NOT IN (SELECT agreement_id FROM sys_agreement_acceptances WHERE user_type = '%s' AND user_id = %d)",
			userType, userID,
		)).
		Count()
	if err != nil {
		return false
	}
	return count > 0
}

// AcceptAgreements 批量接受协议
func AcceptAgreements(ctx context.Context, userType string, userID int64, agreementIDs []int64, ipAddress, userAgent string) error {
	for _, aid := range agreementIDs {
		var agr *struct {
			Id        int64 `json:"id"`
			IsCurrent bool  `json:"is_current"`
		}
		err := dao.SysAgreements.Ctx(ctx).
			Where("id", aid).
			Where("is_current", true).
			Where("status", "published").
			Scan(&agr)
		if err = IgnoreScanNoRows(err); err != nil {
			return err
		}
		if agr == nil {
			continue
		}

		// 检查是否已接受（幂等）
		existCount, err := dao.SysAgreementAcceptances.Ctx(ctx).
			Where("agreement_id", aid).
			Where("user_type", userType).
			Where("user_id", userID).
			Count()
		if err != nil {
			return err
		}
		if existCount > 0 {
			continue
		}

		_, err = dao.SysAgreementAcceptances.Ctx(ctx).Data(do.SysAgreementAcceptances{
			AgreementId: aid,
			UserType:    userType,
			UserId:      userID,
			IpAddress:   ipAddress,
			UserAgent:   userAgent,
		}).Insert()
		if err != nil {
			return err
		}
	}
	return nil
}

// GetCurrentAgreements 获取所有当前生效的协议列表（公开）
func GetCurrentAgreements(ctx context.Context) ([]*CurrentAgreementItem, error) {
	rows := make([]*CurrentAgreementItem, 0)
	err := dao.SysAgreements.Ctx(ctx).
		Where("is_current", true).
		Where("status", "published").
		Fields("id, code, title, version, force_accept, published_at").
		OrderAsc("code").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetCurrentAgreementByCode 按 code 获取当前生效协议详情（公开）
func GetCurrentAgreementByCode(ctx context.Context, code string) (*CurrentAgreementDetail, error) {
	var row *CurrentAgreementDetail
	err := dao.SysAgreements.Ctx(ctx).
		Where("code", code).
		Where("is_current", true).
		Where("status", "published").
		Scan(&row)
	if err = IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	return row, nil
}

// PublishAgreementTx 在事务中发布协议（旧 current 归档，新版本设为 current）
func PublishAgreementTx(ctx context.Context, tx gdb.TX, agreementID int64, code string) error {
	// 旧 current 归档
	_, err := dao.SysAgreements.Ctx(ctx).
		Where("code", code).
		Where("is_current", true).
		Where("id !=", agreementID).
		Data(g.Map{
			"is_current": false,
			"status":     "archived",
			"updated_at": gtime.Now(),
		}).
		Update()
	if err != nil {
		return err
	}

	// 新版本发布
	_, err = dao.SysAgreements.Ctx(ctx).
		Where("id", agreementID).
		Data(g.Map{
			"status":       "published",
			"is_current":   true,
			"published_at": gtime.Now(),
			"updated_at":   gtime.Now(),
		}).
		Update()
	return err
}
