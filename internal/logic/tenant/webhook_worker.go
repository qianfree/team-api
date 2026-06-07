package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

const (
	webhookMaxResponseSize = 2048
	webhookTimeout         = 30 * time.Second
	webhookMaxRetries      = 5
)

// webhookHTTPClient 全局复用的 HTTP 客户端，共享连接池，避免每次投递创建新实例。
var webhookHTTPClient = &http.Client{Timeout: webhookTimeout}

var retryIntervals = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
}

// deliverEventByID loads an event by ID and delivers it.
func deliverEventByID(ctx context.Context, eventID int64) {
	var evt *entity.OpnWebhookEvents
	err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", eventID).Scan(&evt)
	if err = common.IgnoreScanNoRows(err); err != nil {
		g.Log().Errorf(ctx, "webhook: load event %d failed: %v", eventID, err)
		return
	}
	if evt == nil {
		return
	}

	// Skip events already delivered or exhausted retries
	if evt.Status == "delivered" {
		return
	}
	if evt.Attempts >= webhookMaxRetries {
		return
	}

	deliverEvent(ctx, *evt)
}

// deliverEvent delivers a single webhook event.
func deliverEvent(ctx context.Context, evt entity.OpnWebhookEvents) {
	var config *entity.OpnWebhookConfigs
	err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", evt.WebhookConfigId).Scan(&config)
	if err = common.IgnoreScanNoRows(err); err != nil {
		g.Log().Errorf(ctx, "webhook: load config %d failed: %v", evt.WebhookConfigId, err)
		return
	}
	if config == nil {
		g.Log().Errorf(ctx, "webhook: config %d not found", evt.WebhookConfigId)
		return
	}

	if !config.IsActive {
		if _, err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", evt.Id).Data(do.OpnWebhookEvents{
			Status: "failed",
		}).Update(); err != nil {
			g.Log().Errorf(ctx, "webhook: mark event %d failed: %v", evt.Id, err)
		}
		return
	}

	newAttempts := evt.Attempts + 1

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(evt.Payload)
	signature := ComputeWebhookSignature(config.SecretKey, timestamp, body)

	req, err := http.NewRequestWithContext(ctx, "POST", config.Url, bytes.NewReader(body))
	if err != nil {
		recordDeliveryLog(ctx, evt, *config, newAttempts, 0, 0, "", fmt.Sprintf("build request failed: %v", err))
		markEventFailed(ctx, evt.Id, newAttempts)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Timestamp", timestamp)
	req.Header.Set("X-Webhook-Event", evt.EventType)
	req.Header.Set("X-Webhook-ID", evt.EventId)

	startTime := time.Now()
	resp, err := webhookHTTPClient.Do(req)
	elapsed := time.Since(startTime).Milliseconds()

	if err != nil {
		recordDeliveryLog(ctx, evt, *config, newAttempts, 0, 0, "", err.Error())
		markEventFailed(ctx, evt.Id, newAttempts)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, webhookMaxResponseSize))
	respBodyStr := string(respBody)

	recordDeliveryLog(ctx, evt, *config, newAttempts, resp.StatusCode, elapsed, respBodyStr, "")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if _, err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", evt.Id).Data(do.OpnWebhookEvents{
			Status:   "delivered",
			Attempts: newAttempts,
		}).Update(); err != nil {
			g.Log().Errorf(ctx, "webhook: mark event %d delivered failed: %v", evt.Id, err)
		}

		if config.ConsecutiveFailures > 0 {
			if _, err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", config.Id).Data(do.OpnWebhookConfigs{
				ConsecutiveFailures: 0,
				LastDeliveryAt:      gtime.Now(),
			}).Update(); err != nil {
				g.Log().Errorf(ctx, "webhook: reset config %d failures failed: %v", config.Id, err)
			}
		}
	} else {
		markEventFailed(ctx, evt.Id, newAttempts)

		newFailures := config.ConsecutiveFailures + 1
		updateData := do.OpnWebhookConfigs{
			ConsecutiveFailures: newFailures,
			LastDeliveryAt:      gtime.Now(),
		}
		if newFailures >= config.MaxConsecutiveFailures {
			isActive := false
			updateData.IsActive = isActive
			g.Log().Warningf(ctx, "webhook: config %d auto-disabled after %d consecutive failures", config.Id, newFailures)
		}
		if _, err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", config.Id).Data(updateData).Update(); err != nil {
			g.Log().Errorf(ctx, "webhook: update config %d on failure: %v", config.Id, err)
		}
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

	if _, err := dao.OpnWebhookEvents.Ctx(ctx).Where("id", eventID).Data(do.OpnWebhookEvents{
		Status:      "failed",
		Attempts:    attempts,
		NextRetryAt: gtime.NewFromTime(nextRetry),
	}).Update(); err != nil {
		g.Log().Errorf(ctx, "webhook: mark event %d failed for retry: %v", eventID, err)
	}

	if attempts < webhookMaxRetries {
		scheduleRetry(eventID, delay)
	}
}

// recordDeliveryLog records a delivery attempt in opn_webhook_delivery_logs.
func recordDeliveryLog(ctx context.Context, evt entity.OpnWebhookEvents, config entity.OpnWebhookConfigs, attempt int, responseStatus int, responseTimeMs int64, respBody string, errMsg string) {
	reqHeaders := map[string]string{
		"Content-Type":        "application/json",
		"X-Webhook-Signature": "***",
		"X-Webhook-Event":     evt.EventType,
		"X-Webhook-ID":        evt.EventId,
	}
	headersJSON, _ := json.Marshal(reqHeaders)

	if _, err := dao.OpnWebhookDeliveryLogs.Ctx(ctx).Data(do.OpnWebhookDeliveryLogs{
		TenantId:        evt.TenantId,
		WebhookConfigId: config.Id,
		EventId:         evt.Id,
		Attempt:         attempt,
		RequestUrl:      config.Url,
		RequestHeaders:  string(headersJSON),
		ResponseStatus:  responseStatus,
		ResponseBody:    truncateString(respBody, 2000),
		ResponseTimeMs:  int(responseTimeMs),
		ErrorMessage:    truncateString(errMsg, 500),
	}).Insert(); err != nil {
		g.Log().Errorf(ctx, "webhook: record delivery log for event %d failed: %v", evt.Id, err)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
