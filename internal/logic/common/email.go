package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"html/template"
	"net/mail"
	"strings"
	"time"

	"github.com/qianfree/team-api/internal/dao"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	do "github.com/qianfree/team-api/internal/model/do"
	"gopkg.in/gomail.v2"
)

// EmailConfig holds SMTP configuration.
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string // 发件人显示名，收件人邮件列表里看到的名字；为空时只显示邮箱地址
	UseTLS   bool
}

// BasicEmailSender provides email sending capability with:
// - SMTP connection
// - Template rendering (Go templates)
// - Retry (3 times)
// - Send log to ntf_send_log
type BasicEmailSender struct {
	config *EmailConfig
}

// NewEmailSender creates a new BasicEmailSender.
func NewEmailSender(config *EmailConfig) *BasicEmailSender {
	return &BasicEmailSender{config: config}
}

// EmailMessage represents an email to send.
type EmailMessage struct {
	To           string
	Subject      string
	BodyHTML     string
	BodyText     string
	TemplateCode string
	Variables    map[string]any
	TenantID     int64
	UserID       int64
}

// Send sends an email message with retry logic.
func (s *BasicEmailSender) Send(ctx context.Context, msg *EmailMessage) error {
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		lastErr = s.sendWithRetry(ctx, msg, attempt)
		if lastErr == nil {
			// Log success
			s.logSend(ctx, msg, "sent", nil, attempt)
			return nil
		}

		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}

	// Log failure
	s.logSend(ctx, msg, "failed", lastErr, 2)
	return lastErr
}

// SendTemplate sends an email using a template from ntf_templates.
func (s *BasicEmailSender) SendTemplate(ctx context.Context, to, templateCode string, variables map[string]any) error {
	// Load template
	var tpl *struct {
		Subject      string `json:"subject"`
		BodyTemplate string `json:"body_template"`
		Channel      string `json:"channel"`
	}

	err := dao.NtfTemplates.Ctx(ctx).
		Where("code", templateCode).
		Where("status", "active").
		Scan(&tpl)
	if err != nil {
		return gerror.Wrapf(err, "load template %s", templateCode)
	}

	if tpl.BodyTemplate == "" {
		return gerror.Newf("template %s not found", templateCode)
	}

	// Render subject
	subject, err := renderTemplate(tpl.Subject, variables)
	if err != nil {
		return gerror.Wrapf(err, "render subject")
	}

	// Render body
	body, err := renderTemplate(tpl.BodyTemplate, variables)
	if err != nil {
		return gerror.Wrapf(err, "render body")
	}

	msg := &EmailMessage{
		To:           to,
		Subject:      subject,
		BodyHTML:     body,
		TemplateCode: templateCode,
		Variables:    variables,
	}

	return s.Send(ctx, msg)
}

// sendWithRetry performs the actual SMTP send.
func (s *BasicEmailSender) sendWithRetry(ctx context.Context, msg *EmailMessage, attempt int) error {
	m := gomail.NewMessage()
	// 发件人显示名：SetAddressHeader 会按 RFC 5322 拼成 "aifree" <system@mail.aifree.com>，
	// 名称含中文时自动做 RFC 2047 编码；FromName 为空时退化为纯地址。
	// From 若已配成 "名称 <地址>" 形式，先拆出地址避免二次包装。
	fromAddr, fromName := s.config.From, s.config.FromName
	if parsed, err := mail.ParseAddress(fromAddr); err == nil {
		fromAddr = parsed.Address
		if fromName == "" {
			fromName = parsed.Name
		}
	}
	m.SetAddressHeader("From", fromAddr, fromName)
	m.SetHeader("To", msg.To)
	m.SetHeader("Subject", msg.Subject)

	if msg.BodyHTML != "" {
		m.SetBody("text/html", msg.BodyHTML)
		if msg.BodyText != "" {
			m.AddAlternative("text/plain", msg.BodyText)
		}
	} else {
		m.SetBody("text/plain", msg.BodyText)
	}

	dialer := gomail.NewDialer(
		s.config.Host,
		s.config.Port,
		s.config.Username,
		s.config.Password,
	)

	if s.config.UseTLS {
		dialer.TLSConfig = &tls.Config{ServerName: s.config.Host}
	}

	return dialer.DialAndSend(m)
}

// logSend records the send attempt in ntf_send_log.
func (s *BasicEmailSender) logSend(ctx context.Context, msg *EmailMessage, status string, sendErr error, retryCount int) {
	data := do.NtfSendLog{
		TemplateCode: msg.TemplateCode,
		Channel:      "email",
		Recipient:    msg.To,
		Subject:      msg.Subject,
		Status:       status,
		RetryCount:   retryCount,
	}

	if sendErr != nil {
		data.ErrorMessage = sendErr.Error()
	}

	if status == "sent" {
		sentAt := gtime.NewFromTime(time.Now())
		data.SentAt = sentAt
		data.Body = msg.BodyHTML
	}

	if msg.TenantID > 0 {
		data.TenantId = msg.TenantID
	}
	if msg.UserID > 0 {
		data.UserId = msg.UserID
	}

	_, err := dao.NtfSendLog.Ctx(ctx).Data(data).Insert()
	if err != nil {
		g.Log().Errorf(ctx, "log email send: %v", err)
	}
}

// renderTemplate renders a Go template string with the given variables.
func renderTemplate(tplStr string, vars map[string]any) (string, error) {
	t, err := template.New("").Parse(tplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// EmailConfigFromOptions loads email config from the settings registry.
func EmailConfigFromOptions(ctx context.Context) (*EmailConfig, error) {
	cfg := &EmailConfig{
		Host:     Config().GetString(ctx, "email_smtp_host"),
		Port:     Config().GetInt(ctx, "email_smtp_port"),
		Username: Config().GetString(ctx, "email_smtp_username"),
		Password: Config().GetString(ctx, "email_smtp_password"),
		From:     Config().GetString(ctx, "email_smtp_from"),
		FromName: Config().GetString(ctx, "email_smtp_from_name"),
		UseTLS:   Config().GetBool(ctx, "email_smtp_tls"),
	}

	if cfg.Host == "" || cfg.From == "" {
		return nil, gerror.New("email SMTP config not set (email_smtp_host, email_smtp_from)")
	}

	// 未单独配置发件人名称时回落到站点名称，避免收件人只看到邮箱地址的本地部分
	if cfg.FromName == "" {
		cfg.FromName = Config().GetString(ctx, "site_name")
	}

	if cfg.Port == 0 {
		cfg.Port = 587
	}

	return cfg, nil
}

// IsValidEmail checks if a string is a valid email format.
func IsValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
