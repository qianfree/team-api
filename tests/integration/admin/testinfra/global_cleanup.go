//go:build integration

package testinfra

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// CleanupResidualTestData removes test data that leaked from incomplete test runs.
// It identifies test records by well-known name/code prefixes and hard-deletes them.
func CleanupResidualTestData() {
	ctx := context.Background()

	// 1. Clean up test tenants (cascade delete)
	cleanupTestTenants(ctx)

	// 2. Clean up test model groups
	cleanupTestModelGroups(ctx)

	// 3. Clean up test models
	cleanupTestModels(ctx)

	// 4. Clean up test plans
	cleanupTestPlans(ctx)

	// 5. Clean up test messages
	cleanupTestMessages(ctx)

	// 6. Clean up test announcements
	cleanupTestAnnouncements(ctx)

	// 7. Clean up test admin users
	cleanupTestAdminUsers(ctx)
}

func cleanupTestTenants(ctx context.Context) {
	db := g.DB()
	patterns := []string{"test-tenant-%", "crud-tenant-%", "t-%"}
	for _, pattern := range patterns {
		var ids []int64
		err := db.Model("tnt_tenants").Ctx(ctx).
			Fields("id").
			Where("tenant_code LIKE ?", pattern).
			Scan(&ids)
		if err != nil {
			log.Printf("global cleanup: query test tenants (%s): %v", pattern, err)
			continue
		}
		for _, id := range ids {
			cascadeDeleteTenantData(ctx, id)
		}
	}
}

func cascadeDeleteTenantData(ctx context.Context, tenantID int64) {
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
	log.Printf("global cleanup: hard deleted tenant %d", tenantID)
}

func cleanupTestModelGroups(ctx context.Context) {
	db := g.DB()

	// Match by name patterns (covers both original and updated names)
	namePatterns := []string{
		"CRUD测试分组%", "更新分组名%",
		"Test Group%", "InitModels%",
		"Duplicate Code Test", "ghost",
	}
	// Also match by code patterns (more reliable since code is never updated)
	codePatterns := []string{
		"test-group-%", "crud-group-%", "init-models-%",
	}

	allLikes := make([]string, 0, len(namePatterns)+len(codePatterns))
	allArgs := make([]interface{}, 0, len(namePatterns)+len(codePatterns))
	for _, p := range namePatterns {
		allLikes = append(allLikes, "name LIKE ?")
		allArgs = append(allArgs, p)
	}
	for _, p := range codePatterns {
		allLikes = append(allLikes, "code LIKE ?")
		allArgs = append(allArgs, p)
	}
	where := strings.Join(allLikes, " OR ")

	var ids []int64
	err := db.Model("mdl_model_groups").Ctx(ctx).
		Fields("id").
		Where(where, allArgs...).
		Scan(&ids)
	if err != nil {
		log.Printf("global cleanup: query test model groups: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	// Clear tenant associations first to avoid FK-like constraint errors
	for _, id := range ids {
		db.Exec(ctx, "DELETE FROM mdl_tenant_groups WHERE group_id = $1", id)
		db.Exec(ctx, "DELETE FROM mdl_group_models WHERE group_id = $1", id)
		db.Exec(ctx, "DELETE FROM mdl_model_groups WHERE id = $1", id)
	}
	log.Printf("global cleanup: deleted %d model groups", len(ids))
}

func cleanupTestModels(ctx context.Context) {
	db := g.DB()
	var ids []int64
	err := db.Model("mdl_models").Ctx(ctx).
		Fields("id").
		Where("model_id LIKE ?", "test-model-%").
		Scan(&ids)
	if err != nil {
		log.Printf("global cleanup: query test models: %v", err)
		return
	}
	for _, id := range ids {
		db.Exec(ctx, "DELETE FROM mdl_group_models WHERE model_id = $1", id)
		db.Exec(ctx, "DELETE FROM mdl_models WHERE id = $1", id)
		log.Printf("global cleanup: deleted model %d", id)
	}
}

func cleanupTestPlans(ctx context.Context) {
	db := g.DB()
	// Match by both name and identifier — API delete only archives, so test plans persist
	var ids []int64
	err := db.Model("pln_plans").Ctx(ctx).
		Fields("id").
		Where("name LIKE ?", "Test Plan%").
		WhereOr("identifier LIKE ?", "test-plan-%").
		Scan(&ids)
	if err != nil {
		log.Printf("global cleanup: query test plans: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		db.Exec(ctx, "DELETE FROM pln_tenant_plans WHERE plan_id = $1", id)
		db.Exec(ctx, "DELETE FROM pln_feature_flags WHERE plan_id = $1", id)
		db.Exec(ctx, "DELETE FROM pln_plans WHERE id = $1", id)
	}
	log.Printf("global cleanup: deleted %d plans", len(ids))
}

func cleanupTestMessages(ctx context.Context) {
	db := g.DB()
	titlePatterns := []string{"验证测试消息%", "[集成测试]%", "广播消息%"}
	deleted := 0
	for _, pattern := range titlePatterns {
		var ids []int64
		err := db.Model("ntf_messages").Ctx(ctx).
			Fields("id").
			Where("title LIKE ?", pattern).
			Scan(&ids)
		if err != nil {
			log.Printf("global cleanup: query test messages (%s): %v", pattern, err)
			continue
		}
		for _, id := range ids {
			db.Exec(ctx, "DELETE FROM ntf_read_status WHERE message_id = $1", id)
			db.Exec(ctx, "DELETE FROM ntf_messages WHERE id = $1", id)
			deleted++
		}
	}
	if deleted > 0 {
		log.Printf("global cleanup: deleted %d test messages", deleted)
	}
}

func cleanupTestAnnouncements(ctx context.Context) {
	db := g.DB()
	titlePatterns := []string{"CRUD测试公告%", "更新公告%"}
	likes := make([]string, len(titlePatterns))
	args := make([]interface{}, len(titlePatterns))
	for i, p := range titlePatterns {
		likes[i] = "title LIKE ?"
		args[i] = p
	}
	where := strings.Join(likes, " OR ")

	var ids []int64
	err := db.Model("ntf_announcements").Ctx(ctx).
		Fields("id").
		Where(where, args...).
		Scan(&ids)
	if err != nil {
		log.Printf("global cleanup: query test announcements: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		db.Exec(ctx, "DELETE FROM ntf_announcements WHERE id = $1", id)
	}
	log.Printf("global cleanup: deleted %d test announcements", len(ids))
}

func cleanupTestAdminUsers(ctx context.Context) {
	db := g.DB()
	_, err := db.Exec(ctx, "DELETE FROM sys_admin_users WHERE username LIKE ?", "testuser%")
	if err != nil {
		log.Printf("global cleanup: delete test admin users: %v", err)
	}
}
