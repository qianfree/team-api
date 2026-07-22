package common

import (
	"context"
	"crypto/rand"
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

	// Rate limit: max 5 per hour (atomic Lua: INCR + first-time EXPIRE)
	hourlyKey := fmt.Sprintf("verify:hourly:%s:%s", email, purpose)
	hourlyCount, err := g.Redis().Do(ctx, "EVAL",
		`local count = redis.call("INCR", KEYS[1])
		if count == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return count`,
		1, hourlyKey, 3600)
	if err == nil && hourlyCount.Int() > 5 {
		return NewBusinessError(consts.CodeRateLimitExceeded, "每小时最多发送5次验证码")
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
	// Set cooldown before async send (prevents duplicate sends within 60s)
	if _, err := g.Redis().Do(ctx, "SETEX", cooldownKey, 60, "1"); err != nil {
		g.Log().Warningf(ctx, "failed to set verify code cooldown: %v", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Errorf(context.Background(), "send verify code email panic: %v", r)
			}
		}()
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

	return nil
}

// maxVerifyAttempts 单个邮箱+用途在验证码有效期内允许的最大验证失败次数，
// 超过后需重新获取验证码。用于防止 6 位数字验证码在 10 分钟窗口内被暴力枚举。
const maxVerifyAttempts = 5

// VerifyCode checks if a verification code is valid.
func VerifyCode(ctx context.Context, email, code, purpose string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	code = strings.TrimSpace(code)

	if email == "" || code == "" || purpose == "" {
		return NewBadRequestError("参数不完整")
	}

	// 验证侧防暴力破解：先检查失败次数，超限直接拒绝
	failKey := fmt.Sprintf("verify:fail:%s:%s", email, purpose)
	if cnt, err := g.Redis().Do(ctx, "GET", failKey); err == nil && !cnt.IsNil() && cnt.Int() >= maxVerifyAttempts {
		return NewBusinessError(consts.CodeRateLimitExceeded, "验证码错误次数过多，请重新获取验证码")
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
	if err != nil {
		return gerror.Wrapf(err, "query verify code")
	}

	if record == nil {
		incrVerifyFail(ctx, failKey)
		return NewBusinessError(consts.CodeVerifyCodeInvalid, "验证码错误")
	}

	// Check expiration
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return NewBusinessError(consts.CodeEmailVerifyExpired, consts.MsgEmailVerifyExpired)
	}

	// Check code match
	if record.Code != code {
		incrVerifyFail(ctx, failKey)
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

	// 验证成功，清除失败计数
	if _, derr := g.Redis().Do(ctx, "DEL", failKey); derr != nil {
		g.Log().Warningf(ctx, "failed to clear verify fail counter: %v", derr)
	}

	return nil
}

// incrVerifyFail 原子递增验证码失败计数，并在首次失败时设置与验证码有效期一致的 TTL（10 分钟）。
func incrVerifyFail(ctx context.Context, failKey string) {
	_, err := g.Redis().Do(ctx, "EVAL",
		`local count = redis.call("INCR", KEYS[1])
		if count == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return count`,
		1, failKey, 600)
	if err != nil {
		g.Log().Warningf(ctx, "failed to incr verify fail counter: %v", err)
	}
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
