package main

import (
	"fmt"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/gctx"
)

// 临时脚本：手动清除渠道探测配置的所有缓存层
func main() {
	ctx := gctx.New()

	fmt.Println("🔧 清除渠道探测配置缓存...")

	// 1. 清除本地内存缓存
	gcache.Remove(ctx, "opt:channel_auto_test_enabled")
	fmt.Println("✅ 清除本地内存缓存")

	// 2. 清除 Redis L2 缓存
	_, err := g.Redis().Do(ctx, "DEL", "opt:channel_auto_test_enabled")
	if err != nil {
		fmt.Printf("⚠️  Redis DEL 失败: %v\n", err)
	} else {
		fmt.Println("✅ 清除 Redis L2 缓存")
	}

	// 3. 发布缓存失效消息（通知其他实例）
	_, err = g.Redis().Do(ctx, "PUBLISH", "settings:changed", "channel_auto_test_enabled")
	if err != nil {
		fmt.Printf("⚠️  发布失效消息失败: %v\n", err)
	} else {
		fmt.Println("✅ 发布跨实例失效消息")
	}

	// 4. 清除分类缓存
	gcache.Remove(ctx, "opt:category:channel")
	_, _ = g.Redis().Do(ctx, "DEL", "opt:category:channel")
	fmt.Println("✅ 清除分类缓存")

	// 5. 验证数据库中的值
	val, err := g.DB().Ctx(ctx).Model("sys_options").
		Where("key", "channel_auto_test_enabled").
		Value("value")
	if err != nil {
		fmt.Printf("⚠️  查询数据库失败: %v\n", err)
	} else {
		dbValue := val.String()
		fmt.Printf("📊 数据库当前值: %s\n", dbValue)

		if dbValue == "false" || dbValue == "0" {
			fmt.Println("\n✅ 数据库配置正确（已关闭）")
		} else {
			fmt.Println("\n⚠️  注意：数据库中配置仍为开启状态")
			fmt.Println("   请在管理后台确认是否已保存配置更改")
		}
	}

	fmt.Println("\n✅ 缓存清除完成！")
	fmt.Println("   - 如果应用已更新代码，下次定时任务执行时会读取最新配置")
	fmt.Println("   - 如果应用未更新代码，需要重启应用或等待缓存过期（10分钟）")
}
