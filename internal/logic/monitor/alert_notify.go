package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

const (
	// alertNotifyMaxConcurrent 限制 dispatchAlertNotifications 的并发派发数。
	// 告警风暴时一个检测周期可能触发多条规则，每条 spawn 一个 goroutine 发邮件/打
	// webhook；慢响应（SMTP 卡顿、webhook 不可达）会让挂起 goroutine 累积。容量 16 的
	// 带缓冲 channel 作为信号量，把并发派发封顶在 16，避免拖垮 DB/SMTP/出站连接。
	alertNotifyMaxConcurrent = 16
	// alertNotifyTimeout 单次通知派发（email + in_app + webhook 全流程）的最长时长。
	// 给原本无超时的 gctx.New() 兜底，确保即使目标不可达，goroutine 也会在 30s 内退出，
	// 不会因上游挂起而永久泄漏。
	alertNotifyTimeout = 30 * time.Second
)

// alertNotifySem 是 dispatchAlertNotifications 的并发信号量。
// 采用「阻塞获取」语义：拿不到槽位的派发任务会排队等待而非丢弃——告警通知不应丢失，
// 只需限制并发。等待中的 goroutine 栈很小（~2-4KB），配合 alertNotifyTimeout，
// 风暴能在有限时间内自然排空。
var alertNotifySem = make(chan struct{}, alertNotifyMaxConcurrent)

// goDispatchAlertNotifications 异步派发告警通知，带并发上限与超时兜底。
// 由告警引擎在事件创建后调用；阻塞式信号量保证并发不超 alertNotifyMaxConcurrent，
// WithTimeout 保证单个派发不会因上游挂起而无限挂起 goroutine。
func goDispatchAlertNotifications(rule map[string]any, eventID int64, triggerValue, threshold float64) {
	go func() {
		alertNotifySem <- struct{}{}
		defer func() { <-alertNotifySem }()
		// 用全新的带超时 context，不沿用检测循环的 ctx（检测周期结束后原 ctx 可能被取消）。
		ctx, cancel := context.WithTimeout(gctx.New(), alertNotifyTimeout)
		defer cancel()
		dispatchAlertNotifications(ctx, rule, eventID, triggerValue, threshold)
	}()
}

// dispatchAlertNotifications sends notifications for a triggered alert.
func dispatchAlertNotifications(ctx context.Context, rule map[string]any, eventID int64, triggerValue, threshold float64) {
	methods := gconv.Strings(rule["notification_methods"])
	ruleName := gconv.String(rule["name"])
	metricType := gconv.String(rule["metric_type"])
	level := gconv.String(rule["level"])
	webhookURL := gconv.String(rule["webhook_url"])
	notifyUserIDs := gconv.Int64s(rule["notify_user_ids"])

	message := fmt.Sprintf("告警规则「%s」触发：指标 %s 当前值 %.2f 超过阈值 %.2f", ruleName, metricType, triggerValue, threshold)
	subject := fmt.Sprintf("[%s] %s", levelLabel(level), ruleName)

	var notifiedMethods []string

	for _, method := range methods {
		switch method {
		case "email":
			if len(notifyUserIDs) > 0 {
				sendAlertEmailToAdmins(ctx, notifyUserIDs, subject, message)
				notifiedMethods = append(notifiedMethods, "email")
			}
		case "in_app":
			if len(notifyUserIDs) > 0 {
				sendAlertInAppToAdmins(ctx, notifyUserIDs, subject, message)
				notifiedMethods = append(notifiedMethods, "in_app")
			}
		case "webhook":
			if webhookURL != "" {
				sendAlertWebhook(ctx, webhookURL, rule, eventID, triggerValue, threshold)
				notifiedMethods = append(notifiedMethods, "webhook")
			}
		}
	}

	// Update event with notified methods
	if len(notifiedMethods) > 0 && eventID > 0 {
		dao.OpsAlertEvents.Ctx(ctx).
			Where("id", eventID).
			Data(do.OpsAlertEvents{
				NotifiedMethods: notifiedMethods,
			}).
			Update()
	}

}

// sendAlertEmailToAdmins sends alert email to specified admin users.
func sendAlertEmailToAdmins(ctx context.Context, adminIDs []int64, subject, body string) {
	// Create email sender from config
	sender := common.NewEmailSender(&common.EmailConfig{
		Host:     g.Cfg().MustGet(ctx, "email.smtp.host").String(),
		Port:     g.Cfg().MustGet(ctx, "email.smtp.port").Int(),
		Username: g.Cfg().MustGet(ctx, "email.smtp.username").String(),
		Password: g.Cfg().MustGet(ctx, "email.smtp.password").String(),
		From:     g.Cfg().MustGet(ctx, "email.smtp.from").String(),
		UseTLS:   g.Cfg().MustGet(ctx, "email.smtp.port").Int() == 587 || g.Cfg().MustGet(ctx, "email.smtp.port").Int() == 465,
	})

	for _, adminID := range adminIDs {
		type adminUser struct {
			Email string `json:"email"`
		}
		var user adminUser
		err := dao.SysAdminUsers.Ctx(ctx).
			Where("id", adminID).
			Where("status", "active").
			Fields("email").
			Scan(&user)
		if err != nil || user.Email == "" {
			continue
		}

		htmlBody := fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
				<h2 style="color: #e74c3c;">%s</h2>
				<div style="padding: 16px; background: #f8f9fa; border-radius: 8px;">
					<p>%s</p>
					<p style="color: #666;">时间：%s</p>
				</div>
			</div>
		`, subject, body, time.Now().Format("2006-01-02 15:04:05"))

		msg := &common.EmailMessage{
			To:       user.Email,
			Subject:  subject,
			BodyHTML: htmlBody,
		}
		if err := sender.Send(ctx, msg); err != nil {
			g.Log().Warningf(ctx, "send alert email to %s: %v", user.Email, err)
		}
	}
}

// sendAlertInAppToAdmins creates in-app messages for specified admin users.
func sendAlertInAppToAdmins(ctx context.Context, adminIDs []int64, title, content string) {
	for _, adminID := range adminIDs {
		_, err := dao.NtfMessages.Ctx(ctx).Insert(do.NtfMessages{
			TenantId: 0, // System-level message,
			UserId:   adminID,
			Type:     "alert",
			Title:    title,
			Content:  content,
			IsRead:   0,
		})
		if err != nil {
			g.Log().Warningf(ctx, "create in-app alert for admin %d: %v", adminID, err)
		}
	}
}

// sendAlertWebhook sends an alert notification to a webhook URL.
func sendAlertWebhook(ctx context.Context, webhookURL string, rule map[string]any, eventID int64, triggerValue, threshold float64) {
	payload := map[string]any{
		"event_id":      eventID,
		"rule_id":       rule["id"],
		"rule_name":     rule["name"],
		"metric_type":   rule["metric_type"],
		"level":         rule["level"],
		"trigger_value": triggerValue,
		"threshold":     threshold,
		"message":       fmt.Sprintf("告警规则「%s」触发", gconv.String(rule["name"])),
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)
	client := g.Client().SetTimeout(10 * time.Second)
	resp, err := client.DoRequest(ctx, "POST", webhookURL, data)
	if err != nil {
		g.Log().Warningf(ctx, "send alert webhook to %s: %v", webhookURL, err)
		return
	}
	defer resp.Close()

	if resp.StatusCode >= 300 {
		g.Log().Warningf(ctx, "alert webhook returned status %d from %s", resp.StatusCode, webhookURL)
	}
}

// sendTestNotification sends a test notification using the rule's settings.
func sendTestNotification(ctx context.Context, rule map[string]any, event map[string]any) error {
	methods := gconv.Strings(rule["notification_methods"])
	webhookURL := gconv.String(rule["webhook_url"])
	notifyUserIDs := gconv.Int64s(rule["notify_user_ids"])
	subject := fmt.Sprintf("[测试] %s", gconv.String(event["trigger_message"]))
	body := gconv.String(event["trigger_message"])

	sent := false
	for _, method := range methods {
		switch method {
		case "email":
			if len(notifyUserIDs) > 0 {
				sendAlertEmailToAdmins(ctx, notifyUserIDs, subject, body)
				sent = true
			}
		case "in_app":
			if len(notifyUserIDs) > 0 {
				sendAlertInAppToAdmins(ctx, notifyUserIDs, subject, body)
				sent = true
			}
		case "webhook":
			if webhookURL != "" {
				sendAlertWebhook(ctx, webhookURL, rule, 0, gconv.Float64(event["trigger_value"]), gconv.Float64(event["threshold_value"]))
				sent = true
			}
		}
	}

	if !sent {
		return common.NewBadRequestError("未配置任何有效的通知方式")
	}

	return nil
}

func levelLabel(level string) string {
	switch level {
	case "critical":
		return "严重"
	case "warning":
		return "警告"
	case "info":
		return "信息"
	default:
		return level
	}
}
