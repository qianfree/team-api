package cmd

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

var resetPwdCmd = gcmd.Command{
	Name:  "reset-pwd",
	Usage: "reset-pwd -u <username> -p <password>",
	Brief: "重置管理员账号密码",
	Func: func(ctx context.Context, parser *gcmd.Parser) error {
		username := parser.GetOpt("u", "admin").String()
		password := parser.GetOpt("p").String()

		if password == "" {
			fmt.Println("用法: team-api reset-pwd -u <用户名> -p <新密码>")
			fmt.Println("  -u  管理员用户名（默认 admin）")
			fmt.Println("  -p  新密码（必填，至少8位，需含大小写字母和数字）")
			return nil
		}

		// Validate password policy
		if err := common.ValidatePassword(password); err != nil {
			fmt.Println(err.Error())
			return nil
		}

		// Hash password using the same crypto.HashPassword used by Login/InitSuperAdmin
		passwordHash, err := crypto.HashPassword(password)
		if err != nil {
			return fmt.Errorf("密码加密失败: %w", err)
		}

		// Check user exists, get id and status
		var user struct {
			Id     int64  `orm:"id"`
			Status string `orm:"status"`
		}
		err = dao.SysAdminUsers.Ctx(ctx).
			Where("username", username).
			Fields("id,status").
			Scan(&user)
		if err != nil {
			return fmt.Errorf("查询用户失败: %w", err)
		}
		if user.Id == 0 {
			fmt.Printf("用户 '%s' 不存在\n", username)
			return nil
		}

		// Update password and reactivate account in case it was disabled
		_, err = dao.SysAdminUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(do.SysAdminUsers{
				PasswordHash: passwordHash,
				Status:       "active",
			}).
			Update()
		if err != nil {
			return fmt.Errorf("更新密码失败: %w", err)
		}

		// 回读数据库验证哈希是否正确落盘（诊断 ORM 写入问题）
		storedHash, err := dao.SysAdminUsers.Ctx(ctx).
			Where("id", user.Id).
			Fields("password_hash").
			Value()
		if err != nil {
			return fmt.Errorf("回读密码哈希失败: %w", err)
		}
		storedHashStr := storedHash.String()
		if !crypto.VerifyPassword(password, storedHashStr) {
			return fmt.Errorf("密码已写入但回读校验失败，哈希长度=%d，预期=60（bcrypt）。请检查数据库连接与 password_hash 字段是否被截断",
				len(storedHashStr))
		}

		// 清理该用户的所有会话（避免老会话残留导致登录状态异常）
		_, _ = g.DB().Model("sys_sessions").Ctx(ctx).
			Where("user_type", "admin").
			Where("user_id", user.Id).
			Delete()

		fmt.Printf("已重置用户 '%s' 的密码，回读校验通过，旧会话已清理\n", username)
		fmt.Printf("  - 用户ID: %d\n", user.Id)
		fmt.Printf("  - 原状态: %s → active\n", user.Status)
		return nil
	},
}

func init() {
	err := Main.AddCommand(&resetPwdCmd)
	if err != nil {
		panic(fmt.Sprintf("注册 reset-pwd 命令失败: %v", err))
	}
}
