package tenant

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

const (
	webhookMaxResponseSize = 2048
	webhookTimeout         = 30 * time.Second
	webhookMaxRetries      = 5
)

var retryIntervals = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
}

// deliverEventByID loads an event by ID and delivers it.
func deliverEventByID(ctx context.Context, eventID int64) {
	var evt entity.OpnWebhookEvents
	err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", eventID).Scan(&evt)
	if err != nil {
		g.Log().Errorf(ctx, "webhook: load event %d failed: %v", eventID, err)
		return
	}
	if evt.Id == 0 {
		return
	}

	// Skip events already delivered or exhausted retries
	if evt.Status == "delivered" {
		return
	}
	if evt.Attempts >= webhookMaxRetries {
		return
	}

	deliverEvent(ctx, evt)
}

// deliverEvent delivers a single webhook event.
func deliverEvent(ctx context.Context, evt entity.OpnWebhookEvents) {
	var config entity.OpnWebhookConfigs
	err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", evt.WebhookConfigId).Scan(&config)
	if err != nil || config.Id == 0 {
		g.Log().Errorf(ctx, "webhook: config %d not found", evt.WebhookConfigId)
		return
	}

	if !config.IsActive {
		_, _ = dao.OpnWebhookEvents.Ctx(ctx).Where("id", evt.Id).Data(g.Map{
			"status":     "failed",
			"updated_at": gtime.Now(),
		}).Update()
		return
	}

	newAttempts := evt.Attempts + 1

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(evt.Payload)
	signature := computeDeliverySignature(config.SecretKey, timestamp, body)

	req, err := http.NewRequestWithContext(ctx, "POST", config.Url, bytes.NewReader(body))
	if err != nil {
		recordDeliveryLog(ctx, evt, config, newAttempts, 0, "", fmt.Sprintf("build request failed: %v", err))
		markEventFailed(ctx, evt.Id, newAttempts)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Timestamp", timestamp)
	req.Header.Set("X-Webhook-Event", evt.EventType)
	req.Header.Set("X-Webhook-ID", evt.EventId)

	client := &http.Client{Timeout: webhookTimeout}
	startTime := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(startTime).Milliseconds()

	if err != nil {
		recordDeliveryLog(ctx, evt, config, newAttempts, 0, "", err.Error())
		markEventFailed(ctx, evt.Id, newAttempts)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, webhookMaxResponseSize))
	respBodyStr := string(respBody)

	recordDeliveryLog(ctx, evt, config, newAttempts, elapsed, respBodyStr, "")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = dao.OpnWebhookEvents.Ctx(ctx).Where("id", evt.Id).Data(g.Map{
			"status":     "delivered",
			"attempts":   newAttempts,
			"updated_at": gtime.Now(),
		}).Update()

		if config.ConsecutiveFailures > 0 {
			_, _ = dao.OpnWebhookConfigs.Ctx(ctx).Where("id", config.Id).Data(g.Map{
				"consecutive_failures": 0,
				"last_delivery_at":     gtime.Now(),
			}).Update()
		}
	} else {
		markEventFailed(ctx, evt.Id, newAttempts)

		newFailures := config.ConsecutiveFailures + 1
		updateData := g.Map{
			"consecutive_failures": newFailures,
			"last_delivery_at":     gtime.Now(),
		}
		if newFailures >= config.MaxConsecutiveFailures {
			updateData["is_active"] = false
			g.Log().Warningf(ctx, "webhook: config %d auto-disabled after %d consecutive failures", config.Id, newFailures)
		}
		_, _ = dao.OpnWebhookConfigs.Ctx(ctx).Where("id", config.Id).Data(updateData).Update()
	}
}

// markEventFailed updates the event status and schedules a delayed retry via the dispatcher.
func markEventFailed(ctx context.Context, eventID int64, attempts int) {
	var delay time.Duration
	if attempts-1 < len(retryIntervals) {
		delay = retryIntervals[attempts-1]
	} else {
		delay = 6 * time.Hour
	}
	nextRetry := time.Now().Add(delay)

	_, _ = dao.OpnWebhookEvents.Ctx(ctx).Where("id", eventID).Data(g.Map{
		"status":        "failed",
		"attempts":      attempts,
		"next_retry_at": gtime.NewFromTime(nextRetry),
		"updated_at":    gtime.Now(),
	}).Update()

	if attempts < webhookMaxRetries {
		scheduleRetry(eventID, delay)
	}
}

// recordDeliveryLog records a delivery attempt in opn_webhook_delivery_logs.
func recordDeliveryLog(ctx context.Context, evt entity.OpnWebhookEvents, config entity.OpnWebhookConfigs, attempt int, responseTimeMs int64, respBody string, errMsg string) {
	reqHeaders := map[string]string{
		"Content-Type":        "application/json",
		"X-Webhook-Signature": "***",
		"X-Webhook-Event":     evt.EventType,
		"X-Webhook-ID":        evt.EventId,
	}
	headersJSON, _ := json.Marshal(reqHeaders)

	statusCode := 0
	if respBody != "" {
		statusCode = 200
	}

	_, _ = dao.OpnWebhookDeliveryLogs.Ctx(ctx).Data(do.OpnWebhookDeliveryLogs{
		TenantId:        evt.TenantId,
		WebhookConfigId: config.Id,
		EventId:         evt.Id,
		Attempt:         attempt,
		RequestUrl:      config.Url,
		RequestHeaders:  string(headersJSON),
		ResponseStatus:  statusCode,
		ResponseBody:    truncateString(respBody, 2000),
		ResponseTimeMs:  int(responseTimeMs),
		ErrorMessage:    truncateString(errMsg, 500),
	}).Insert()
}

func computeDeliverySignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
