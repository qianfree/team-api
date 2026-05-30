//go:build integration

package testinfra

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

// HardDeleteTenant cascade-deletes a tenant and ALL associated data from the database.
// Tables are deleted in dependency order (leaf → mid → main).
func HardDeleteTenant(t *testing.T, tenantID int64) {
	t.Helper()
	t.Logf("cleanup: hard deleting tenant %d", tenantID)
	ctx := context.Background()
	db := g.DB()

	// Leaf tables (no dependents)
	for _, table := range tenantLeafTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	// Mid-tier tables
	for _, table := range tenantMidTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	// Tenant-model associations
	for _, table := range tenantAssocTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	// Nullable tenant_id tables
	for _, table := range tenantNullableTables {
		db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID)
	}
	// Main table
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

// HardDeleteAnnouncement deletes an announcement from the database.
func HardDeleteAnnouncement(t *testing.T, id int64) {
	t.Helper()
	g.DB().Exec(context.Background(), "DELETE FROM ntf_announcements WHERE id = $1", id)
}

// HardDeleteMessage deletes a notification message and its read statuses from the database.
func HardDeleteMessage(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()
	db.Exec(ctx, "DELETE FROM ntf_read_status WHERE message_id = $1", id)
	db.Exec(ctx, "DELETE FROM ntf_messages WHERE id = $1", id)
}

// HardDeleteMessagesByTitle deletes notification messages matching a title prefix and their read statuses.
func HardDeleteMessagesByTitle(t *testing.T, titlePrefix string) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()

	var ids []int64
	err := db.Model("ntf_messages").Ctx(ctx).
		Fields("id").
		Where("title LIKE ?", titlePrefix+"%").
		Scan(&ids)
	if err != nil {
		return
	}
	for _, id := range ids {
		db.Exec(ctx, "DELETE FROM ntf_read_status WHERE message_id = $1", id)
		db.Exec(ctx, "DELETE FROM ntf_messages WHERE id = $1", id)
	}
}

// HardDeletePromoCode deletes a promo code and its usages from the database.
func HardDeletePromoCode(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()
	db.Exec(ctx, "DELETE FROM ord_promo_code_usages WHERE promo_code_id = $1", id)
	db.Exec(ctx, "DELETE FROM ord_promo_codes WHERE id = $1", id)
}

// HardDeleteRedemption deletes a redemption code and its usages from the database.
func HardDeleteRedemption(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()
	db.Exec(ctx, "DELETE FROM ord_redemption_usages WHERE redemption_id = $1", id)
	db.Exec(ctx, "DELETE FROM ord_redemptions WHERE id = $1", id)
}

// HardDeleteFeedback deletes a feedback from the database.
func HardDeleteFeedback(t *testing.T, id int64) {
	t.Helper()
	g.DB().Exec(context.Background(), "DELETE FROM spt_feedbacks WHERE id = $1", id)
}

// HardDeletePlan deletes a plan and its associations from the database.
func HardDeletePlan(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()
	db.Exec(ctx, "DELETE FROM pln_tenant_plans WHERE plan_id = $1", id)
	db.Exec(ctx, "DELETE FROM pln_feature_flags WHERE plan_id = $1", id)
	db.Exec(ctx, "DELETE FROM pln_plans WHERE id = $1", id)
}

// HardDeleteTicket deletes a ticket, its replies and attachments from the database.
func HardDeleteTicket(t *testing.T, id int64) {
	t.Helper()
	ctx := context.Background()
	db := g.DB()
	db.Exec(ctx, "DELETE FROM spt_replies WHERE ticket_id = $1", id)
	db.Exec(ctx, "DELETE FROM spt_attachments WHERE ticket_id = $1", id)
	db.Exec(ctx, "DELETE FROM spt_tickets WHERE id = $1", id)
}
