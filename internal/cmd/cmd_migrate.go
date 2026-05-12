package cmd

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/pressly/goose/v3"

	"github.com/qianfree/team-api/migrations"
)

var migrateCmd = gcmd.Command{
	Name:  "migrate",
	Usage: "migrate [up|down|status]",
	Brief: "执行数据库迁移",
	Description: `数据库迁移管理命令。
  migrate        执行所有待迁移脚本（等同于 migrate up）
  migrate up     执行所有待迁移脚本
  migrate down   回滚上一次迁移
  migrate status 查看迁移状态`,
	Func: func(ctx context.Context, parser *gcmd.Parser) error {
		action := "up"
		if args := parser.GetArgAll(); len(args) > 2 {
			action = args[2]
		}
		return runMigrate(ctx, action)
	},
}

func init() {
	err := Main.AddCommand(&migrateCmd)
	if err != nil {
		panic(fmt.Sprintf("注册 migrate 命令失败: %v", err))
	}
}

func runMigrate(ctx context.Context, action string) error {
	db, err := g.DB().Master()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	switch action {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "status":
		return goose.StatusContext(ctx, db, ".")
	default:
		return fmt.Errorf("未知的迁移操作: %s（可选: up, down, status）", action)
	}
}

// runAutoMigrate is called during application startup to ensure the database
// schema is up to date before serving requests.
func runAutoMigrate(ctx context.Context) error {
	g.Log().Info(ctx, "执行数据库自动迁移...")

	db, err := g.DB().Master()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	g.Log().Info(ctx, "数据库迁移完成")
	return nil
}
