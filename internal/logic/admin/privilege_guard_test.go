package admin

import (
	"context"
	"testing"
)

// ctxAs 构造带调用者身份的上下文。
// 键名与 middleware 写入 ctx 的保持一致（见 internal/logic/common/session.go）。
func ctxAs(role string, userID int64) context.Context {
	ctx := context.WithValue(context.Background(), "role", role)
	return context.WithValue(ctx, "userId", userID)
}

// TestAssertCanGrantSuperAdmin 守住「谁能把账号设成超级管理员」。
//
// sys_admin_users.role 是超管的唯一判定依据，鉴权时直接短路放行。若允许非超管写这个字段，
// 一个拿到 user:create / user:edit 的账号可以直接把自己提成超管 —— 比分配角色更直接，
// 且完全绕过 superAdminOnlyRules。字段级取值校验（ValidateAdminRole）管不了这件事。
func TestAssertCanGrantSuperAdmin(t *testing.T) {
	cases := []struct {
		name        string
		operator    string
		targetRole  string
		wantBlocked bool
	}{
		{"超管可以设置超管", "super_admin", "super_admin", false},
		{"超管可以设置普通管理员", "super_admin", "admin", false},
		{"非超管不能设置超管", "admin", "super_admin", true},
		{"非超管可以设置普通管理员", "admin", "admin", false},
		{"无身份上下文时不能设置超管", "", "super_admin", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := assertCanGrantSuperAdmin(ctxAs(c.operator, 1), c.targetRole)
			if c.wantBlocked && err == nil {
				t.Fatalf("应被拦截，实际放行")
			}
			if !c.wantBlocked && err != nil {
				t.Fatalf("应放行，实际被拦截: %v", err)
			}
		})
	}
}

// TestAssertCanManageAdminUserBlocksSuperAdminTarget 守住「非超管不得操作超管账号」。
//
// 账号管理（改密/禁用/删除/配特批权限）按 user:* 授权、允许下放。若不限制目标，
// 拿到 user:edit 的账号重置一次超管密码就能接管系统，角色管理限定超管那道闸门形同虚设。
//
// 只覆盖不触库的分支：超管调用者短路、目标为超管、无调用者身份、操作自己。
// 「目标权限须是调用者子集」需要读库，由集成测试覆盖。
func TestAssertCanManageAdminUserBlocksSuperAdminTarget(t *testing.T) {
	if err := assertCanManageAdminUser(ctxAs("super_admin", 1), 2, "super_admin"); err != nil {
		t.Fatalf("超管操作任意账号应放行: %v", err)
	}
	if err := assertCanManageAdminUser(ctxAs("admin", 2), 1, "super_admin"); err == nil {
		t.Fatal("非超管操作超管账号应被拦截，实际放行")
	}
	// 无调用者身份（命令行工具、内部调用）：交由上层接口鉴权把关，此处不拦
	if err := assertCanManageAdminUser(ctxAs("admin", 0), 3, "admin"); err != nil {
		t.Fatalf("无调用者身份时不应拦截: %v", err)
	}
	// 操作自己：自身权限的变更另有 sanitizeGrantedPermissions 把关
	if err := assertCanManageAdminUser(ctxAs("admin", 5), 5, "admin"); err != nil {
		t.Fatalf("操作自己不应被拦截: %v", err)
	}
}
