package tenant

import (
	"context"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// SendCode sends a verification code email.
func (s *sTenant) SendCode(ctx context.Context, req *v1.TenantSendCodeReq) (*v1.TenantSendCodeRes, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))

	switch common.VerifyPurpose(req.Purpose) {
	case common.VerifyPurposeRegister:
		// Check if email verification is enabled for registration
		if !common.Config().GetBool(ctx, "register_email_verification") {
			return nil, common.NewBusinessError(consts.CodeEmailVerificationDisabled, consts.MsgEmailVerificationDisabled)
		}
	case common.VerifyPurposeResetPwd:
		// Check if user exists with this email
		// For tenant reset, we need to know which tenant - user provides email
		// We'll verify the email exists during the actual reset
	case common.VerifyPurposeChangeEmail:
		// Check if new email is already taken by another user in same tenant
		// Tenant context should be available
	default:
		return nil, common.NewBadRequestError("无效的验证码用途")
	}

	err := common.SendVerifyCode(ctx, email, common.VerifyPurpose(req.Purpose))
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ResetPassword handles password reset.
func (s *sTenant) ResetPassword(ctx context.Context, req *v1.TenantResetPasswordReq) (*v1.TenantResetPasswordRes, error) {

	// Check captcha if required
	if err := common.CheckCaptchaRequired(ctx, "tenant_reset_password", req.CaptchaKey, req.CaptchaX); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Verify code
	err := common.VerifyCode(ctx, email, req.Code, "reset_password")
	if err != nil {
		return nil, err
	}

	// Find user by email (search across all tenants)
	var user entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).
		Where("email", email).
		Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Id == 0 {
		return nil, common.NewBadRequestError("该邮箱未注册")
	}

	if user.Status != "active" {
		return nil, common.NewBadRequestError("账号状态异常")
	}

	// Validate password
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(10015, "密码不符合策略")
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Update password
	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", user.Id).
		Data(do.TntUsers{
			PasswordHash:   passwordHash,
			FailedAttempts: 0,
			LockedUntil:    nil,
		}).Update()
	if err != nil {
		return nil, err
	}

	// Revoke all sessions
	common.RevokeAllSessions(ctx, "tenant", user.Id)

	return nil, nil
}

// ChangeEmail handles email change for a tenant user.
func (s *sTenant) ChangeEmail(ctx context.Context, req *v1.TenantChangeEmailReq) (*v1.TenantChangeEmailRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	newEmail := strings.TrimSpace(strings.ToLower(req.NewEmail))

	// Verify code for new email
	err := common.VerifyCode(ctx, newEmail, req.Code, "change_email")
	if err != nil {
		return nil, err
	}

	// Check if new email is already taken in the same tenant
	count, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("email", newEmail).
		Where("id != ?", userID).
		Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBadRequestError("该邮箱已被其他成员使用")
	}

	// Get old email for notification
	var user entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).Scan(&user)
	if err != nil {
		return nil, err
	}
	oldEmail := user.Email

	// Update email
	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Data(do.TntUsers{
			Email: newEmail,
		}).Update()
	if err != nil {
		return nil, err
	}

	// Send notification emails (fire and forget)
	go func() {
		bgCtx := context.Background()
		emailCfg, err := common.EmailConfigFromOptions(bgCtx)
		if err != nil {
			return
		}
		sender := common.NewEmailSender(emailCfg)

		// Notify old email
		sender.Send(bgCtx, &common.EmailMessage{
			To:       oldEmail,
			Subject:  "邮箱变更通知",
			BodyHTML: "<p>您的账号邮箱已变更为 " + newEmail + "。如非本人操作，请立即联系管理员。</p>",
			TenantID: tenantID,
			UserID:   userID,
		})

		// Notify new email
		sender.Send(bgCtx, &common.EmailMessage{
			To:       newEmail,
			Subject:  "邮箱变更确认",
			BodyHTML: "<p>您的账号邮箱已成功变更为本邮箱。</p>",
			TenantID: tenantID,
			UserID:   userID,
		})
	}()

	return nil, nil
}
