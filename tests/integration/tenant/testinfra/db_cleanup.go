//go:build integration

package testinfra

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

// HardDeleteTenant cascade-deletes a tenant and ALL associated data from the database.
func HardDeleteTenant(t *testing.T, tenantID int64) {
	t.Helper()
	t.Logf("cleanup: hard deleting tenant %d", tenantID)
	ctx := context.Background()
	db := g.DB()

	for _, table := range tenantLeafTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	for _, table := range tenantMidTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	for _, table := range tenantAssocTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	for _, table := range tenantNullableTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	db.Exec(ctx, "DELETE FROM tnt_tenants WHERE id = $1", tenantID)
}

var tenantLeafTables = []string{
	"api_key_model_scopes",
	"ntf_read_status",
	"tnt_member_model_scopes",
	"ord_promo_code_usages",
	"ord_redemption_usages",
}

var tenantMidTables = []string{
	"api_keys",
	"tnt_users",
	"tnt_invitations",
	"tnt_member_imports",
	"tnt_oauth_identities",
	"tnt_projects",
	"tnt_tenant_plugins",
	"bil_records",
	"bil_transactions",
	"bil_usage_logs",
	"bil_prededuct_tracks",
	"bil_daily_usage_summary",
	"bil_monthly_usage_summary",
	"ord_orders",
	"ord_refunds",
	"pln_tenant_plans",
	"pln_feature_flags",
	"ntf_messages",
	"ntf_preferences",
	"ntf_send_log",
	"opn_apps",
	"opn_webhook_configs",
	"opn_webhook_events",
	"spt_tickets",
	"spt_feedbacks",
	"tsk_model_tasks",
	"aud_request_logs",
	"aud_operation_logs",
	"aud_content_filter_logs",
	"chn_channel_affinities",
	"chn_error_events",
	"fil_files",
	"plg_example_logs",
	"bil_wallets",
}

var tenantAssocTables = []string{
	"mdl_tenant_models",
	"mdl_tenant_groups",
}

var tenantNullableTables = []string{
	"aud_login_history",
	"aud_sensitive_access_logs",
	"bil_daily_revenue_summary",
	"bil_monthly_revenue_summary",
	"sys_sessions",
}

// HardDeleteFeedback deletes a feedback from the database.
func HardDeleteFeedback(t *testing.T, id int64) {
	t.Helper()
	g.DB().Exec(context.Background(), "DELETE FROM spt_feedbacks WHERE id = $1", id)
}
