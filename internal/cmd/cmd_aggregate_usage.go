package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/qianfree/team-api/internal/logic/task"
)

// aggregateUsageCmd 历史回填命令：将 bil_usage_logs 的历史用量按月分块聚合进 bil_usage_daily。
// 用于首次启用流量桑基图时一次性补齐全量历史，使页面首日即有完整数据。
// 用法：team-api aggregate-usage --from 2026-01-01 --to 2026-07-29
var aggregateUsageCmd = gcmd.Command{
	Name:  "aggregate-usage",
	Usage: "aggregate-usage --from <YYYY-MM-DD> --to <YYYY-MM-DD>",
	Brief: "回填 bil_usage_daily 用量日汇总（历史数据）， 一般用不到",
	Description: `
将 bil_usage_logs 的历史用量按月分块聚合写入 bil_usage_daily，供流量流向桑基图与趋势分析使用。
按月分块以避免单次扫描全部分区表。--from/--to 均为包含的日期边界；--to 缺省为昨天。
聚合幂等（ON CONFLICT），可重复执行。`,
	Func: func(ctx context.Context, parser *gcmd.Parser) error {
		fromStr := parser.GetOpt("from").String()
		toStr := parser.GetOpt("to").String()

		if fromStr == "" {
			fmt.Println("用法: team-api aggregate-usage --from <YYYY-MM-DD> [--to <YYYY-MM-DD>]")
			fmt.Println("  --from  起始日期（含，YYYY-MM-DD，必填）")
			fmt.Println("  --to    结束日期（含，YYYY-MM-DD，缺省=昨天）")
			return nil
		}

		start, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return fmt.Errorf("--from 日期格式无效（需 YYYY-MM-DD）: %w", err)
		}
		var lastDay time.Time
		if toStr == "" {
			lastDay = time.Now().AddDate(0, 0, -1)
		} else {
			lastDay, err = time.Parse("2006-01-02", toStr)
			if err != nil {
				return fmt.Errorf("--to 日期格式无效（需 YYYY-MM-DD）: %w", err)
			}
		}
		if !lastDay.Before(start.AddDate(0, 0, 1)) {
			// lastDay >= start 才合法；此处确保区间非空
		}
		endExclusive := lastDay.AddDate(0, 0, 1) // [start, endExclusive)

		fmt.Printf("开始回填 bil_usage_daily：%s ~ %s（按月分块）\n",
			start.Format("2006-01-02"), lastDay.Format("2006-01-02"))

		cur := start
		chunk := 0
		for cur.Before(endExclusive) {
			// 当前块的结束 = 下个月 1 号，但不超过 endExclusive
			chunkEnd := time.Date(cur.Year(), cur.Month()+1, 1, 0, 0, 0, 0, cur.Location())
			if chunkEnd.After(endExclusive) {
				chunkEnd = endExclusive
			}
			chunk++
			fmt.Printf("  [%d] 聚合 %s ~ %s ...\n", chunk,
				cur.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))
			if err := task.AggregateUsageRange(ctx, cur.Format("2006-01-02"), chunkEnd.Format("2006-01-02")); err != nil {
				return fmt.Errorf("聚合 %s~%s 失败: %w", cur.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), err)
			}
			cur = chunkEnd
		}

		fmt.Printf("回填完成，共处理 %d 个月度分块\n", chunk)
		return nil
	},
}

func init() {
	if err := Main.AddCommand(&aggregateUsageCmd); err != nil {
		panic(fmt.Sprintf("注册 aggregate-usage 命令失败: %v", err))
	}
}
