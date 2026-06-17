package tenant

import (
	"context"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
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
		// Require slider captcha before sending a reset code. Non-consuming:
		// the verified flag is left intact for the consuming check at
		// ResetPassword. (register/change_email intentionally not gated here.)
		if err := common.IsCaptchaVerified(ctx, req.CaptchaKey, req.CaptchaX); err != nil {
			return nil, err
		}
		// 仅主用户（owner）可通过邮箱重置密码；校验邮箱是否属于某个 owner 账号。
		// 放在滑块验证之后，避免未通过人机验证就枚举哪些邮箱是主账号。
		var owner *entity.TntUsers
		err := dao.TntUsers.Ctx(ctx).
			Where("email", email).
			Where("role", "owner").
			Scan(&owner)
		if err = common.IgnoreScanNoRows(err); err != nil {
			return nil, err
		}
		if owner == nil {
			return nil, common.NewBadRequestError("该邮箱未注册或非主账号，无法通过邮箱重置密码")
		}
		if owner.Status != "active" {
			return nil, common.NewBadRequestError("账号状态异常，请联系管理员")
		}
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

	// Find owner (主用户) by email — only owners may reset by email.
	// Members are managed by the owner/admin and do not self-reset via email.
	var user *entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).
		Where("email", email).
		Where("role", "owner").
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBadRequestError("该邮箱未注册或非主账号，无法重置密码")
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
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
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
	var user *entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).Where("tenant_id", tenantID).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("用户")
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

// SendChangeEmailCode sends a verification code for setting/changing email.
// Authenticated (unlike the public send-code): pre-checks that newEmail is not
// already taken by another member in the same tenant before sending, so we don't
// waste a code (and an email) on an address the user can't actually use.
func (s *sTenant) SendChangeEmailCode(ctx context.Context, req *v1.TenantSendChangeEmailCodeReq) (*v1.TenantSendChangeEmailCodeRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	newEmail := strings.TrimSpace(strings.ToLower(req.NewEmail))

	// Pre-check: reject early if the email is already used by another member in this tenant.
	count, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("email", newEmail).
		Where("id <> ?", userID).
		Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBadRequestError("该邮箱已被其他成员使用")
	}

	if err := common.SendVerifyCode(ctx, newEmail, common.VerifyPurposeChangeEmail); err != nil {
		return nil, err
	}
	return nil, nil
}
