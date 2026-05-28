package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/logic/common"
)

// MaintenanceMode is a non-blocking middleware that sets maintenance-related
// response headers when maintenance mode is enabled. It never blocks requests.
func MaintenanceMode(r *ghttp.Request) {
	ctx := r.Context()
	if common.Config().GetBool(ctx, "maintenance_mode") {
		r.Response.Header().Set("X-Maintenance-Mode", "true")
		r.Response.Header().Set("X-Maintenance-Message", common.Config().GetString(ctx, "maintenance_message"))
		r.Response.Header().Set("X-Maintenance-Duration", common.Config().GetString(ctx, "maintenance_duration"))
	}
	r.Middleware.Next()
}

// ApiMaintenance is a blocking middleware that rejects API requests with 503
// when API maintenance mode is enabled.
func ApiMaintenance(r *ghttp.Request) {
	ctx := r.Context()
	if !common.Config().GetBool(ctx, "api_maintenance_enabled") {
		r.Middleware.Next()
		return
	}

	durationStr := common.Config().GetString(ctx, "maintenance_duration")
	retryAfter := parseMaintenanceDuration(ctx, durationStr)

	message := common.Config().GetString(ctx, "maintenance_message")
	if message == "" {
		message = "系统正在维护中，请稍后再试"
	}

	r.Response.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.WriteHeader(http.StatusServiceUnavailable)

	if strings.HasPrefix(r.URL.Path, "/v1/messages") {
		body, _ := json.Marshal(map[string]any{
			"type": "error",
			"error": map[string]string{
				"type":    "server_error",
				"message": message,
			},
		})
		r.Response.Write(body)
	} else {
		body, _ := json.Marshal(map[string]any{
			"error": map[string]string{
				"type":    "server_error",
				"message": message,
			},
		})
		r.Response.Write(body)
	}
}

// parseMaintenanceDuration parses a duration string like "2h", "30m", "1h30m"
// and returns the total number of seconds. Returns 300 (5 minutes) as default
// if parsing fails or the string is empty.
func parseMaintenanceDuration(ctx context.Context, s string) int {
	if s == "" {
		return 300
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		g.Log().Warningf(ctx, "failed to parse maintenance duration %q: %v, using default 300s", s, err)
		return 300
	}

	seconds := int(d.Seconds())
	if seconds <= 0 {
		return 300
	}

	return seconds
}
