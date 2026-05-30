package common

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

// VerifyPurpose defines the purpose of a verification code.
type VerifyPurpose string

const (
	VerifyPurposeRegister    VerifyPurpose = "register"
	VerifyPurposeResetPwd    VerifyPurpose = "reset_password"
	VerifyPurposeChangeEmail VerifyPurpose = "change_email"
)

// SendVerifyCode generates a 6-digit code, saves it to the database, and sends it via email.
// Rate limits: 60s cooldown between sends, max 5 per hour.
func SendVerifyCode(ctx context.Context, email string, purpose VerifyPurpose) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !IsValidEmail(email) {
		return NewBadRequestError("邮箱格式无效")
	}

	// Rate limit: 60 second cooldown
	cooldownKey := fmt.Sprintf("verify:cooldown:%s:%s", email, purpose)
	cooldownVal, err := g.Redis().Do(ctx, "GET", cooldownKey)
	if err == nil && !cooldownVal.IsNil() {
		remaining, _ := g.Redis().Do(ctx, "TTL", cooldownKey)
		secs := remaining.Int()
		return NewBusinessError(consts.CodeRateLimitExceeded,
			fmt.Sprintf("请 %d 秒后再试", secs))
	}

	// Rate limit: max 5 per hour
	hourlyKey := fmt.Sprintf("verify:hourly:%s:%s", email, purpose)
	hourlyCount, err := g.Redis().Do(ctx, "INCR", hourlyKey)
	if err == nil && hourlyCount.Int() > 5 {
		return NewBusinessError(consts.CodeRateLimitExceeded, "每小时最多发送5次验证码")
	}
	if hourlyCount.Int() == 1 {
		// Set expiry on first increment
		_, _ = g.Redis().Do(ctx, "EXPIRE", hourlyKey, 3600)
	}

	// Generate 6-digit code
	code, err := generateCode(6)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	// Save to database
	_, err = dao.SysEmailVerifyCodes.Ctx(ctx).Data(do.SysEmailVerifyCodes{
		Email:     email,
		Code:      code,
		Purpose:   string(purpose),
		ExpiresAt: gtime.Now().Add(10 * time.Minute),
	}).Insert()
	if err != nil {
		return gerror.Wrapf(err, "save verify code")
	}

	// Send email (fire and forget, log error but don't fail)
	go func() {
		bgCtx := context.Background()
		emailCfg, err := EmailConfigFromOptions(bgCtx)
		if err != nil {
			g.Log().Errorf(bgCtx, "load email config: %v", err)
			return
		}
		sender := NewEmailSender(emailCfg)
		subject := fmt.Sprintf("验证码：%s", code)
		body := fmt.Sprintf(
			`<p>您的验证码是：<strong style="font-size:24px;letter-spacing:4px;">%s</strong></p>
			<p>验证码 10 分钟内有效。如非本人操作，请忽略此邮件。</p>`,
			code,
		)
		err = sender.Send(bgCtx, &EmailMessage{
			To:       email,
			Subject:  subject,
			BodyHTML: body,
			TenantID: 0,
			UserID:   0,
		})
		if err != nil {
			g.Log().Errorf(bgCtx, "send verify code email to %s: %v", email, err)
		}
	}()

	// Set cooldown
	_, _ = g.Redis().Do(ctx, "SETEX", cooldownKey, 60, "1")

	return nil
}

// VerifyCode checks if a verification code is valid.
func VerifyCode(ctx context.Context, email, code, purpose string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)

	if email == "" || code == "" || purpose == "" {
		return NewBadRequestError("参数不完整")
	}

	var record *struct {
		ID        int64      `json:"id"`
		Code      string     `json:"code"`
		ExpiresAt *time.Time `json:"expires_at"`
		UsedAt    *time.Time `json:"used_at"`
	}

	err := dao.SysEmailVerifyCodes.Ctx(ctx).
		Where("email", email).
		Where("purpose", purpose).
		Where("used_at IS NULL").
		OrderDesc("created_at").
		Limit(1).
		Scan(&record)
	if err != nil && err != sql.ErrNoRows {
		return gerror.Wrapf(err, "query verify code")
	}

	if record.ID == 0 {
		return NewBusinessError(consts.CodeVerifyCodeInvalid, "验证码错误")
	}

	// Check expiration
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return NewBusinessError(consts.CodeEmailVerifyExpired, consts.MsgEmailVerifyExpired)
	}

	// Check code match
	if record.Code != code {
		return NewBusinessError(consts.CodeVerifyCodeInvalid, "验证码错误")
	}

	// Mark as used
	_, err = dao.SysEmailVerifyCodes.Ctx(ctx).
		Where("id", record.ID).
		Data(do.SysEmailVerifyCodes{
			UsedAt: gtime.Now(),
		}).
		Update()
	if err != nil {
		return gerror.Wrapf(err, "mark code used")
	}

	return nil
}

// generateCode generates a random numeric code of the given length.
func generateCode(length int) (string, error) {
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n.Int64()), nil
}
