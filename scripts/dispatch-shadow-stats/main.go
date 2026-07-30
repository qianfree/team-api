// dispatch-shadow-stats 渠道调度影子模式对比日志统计工具（阶段 2 交付物）。
//
// 用法：
//
//	go run ./scripts/dispatch-shadow-stats logs/*.log      # 从日志文件读取
//	cat app.log | go run ./scripts/dispatch-shadow-stats   # 从 stdin 读取
//
// 解析含 "[DispatchShadow] " 标记的行，输出一致率与不一致归因分布。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
)

type entry struct {
	RequestID string `json:"request_id"`
	TenantID  int64  `json:"tenant_id"`
	Model     string `json:"model"`
	Match     bool   `json:"match"`
	Legacy    struct {
		Channel int64  `json:"channel"`
		Reason  string `json:"reason"`
	} `json:"legacy"`
	Shadow struct {
		Channel       int64  `json:"channel"`
		Reason        string `json:"reason"`
		Tier          string `json:"tier"`
		SessionSource string `json:"session_source"`
		Candidates    int    `json:"candidates"`
	} `json:"shadow"`
	CostUs int64 `json:"cost_us"`
}

const marker = "[DispatchShadow] "

func main() {
	var readers []io.Reader
	if len(os.Args) > 1 {
		for _, path := range os.Args[1:] {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "打开 %s 失败: %v\n", path, err)
				os.Exit(1)
			}
			defer f.Close()
			readers = append(readers, f)
		}
	} else {
		readers = append(readers, os.Stdin)
	}

	var (
		total, matched, noCandidate int
		reasonPairs                 = map[string]int{} // legacy_reason→shadow_reason
		channelPairs                = map[string]int{} // legacy_ch→shadow_ch（仅不一致）
		sources                     = map[string]int{}
		shadowReasons               = map[string]int{}
		costs                       []int64
		byModelTotal                = map[string]int{}
		byModelMatch                = map[string]int{}
	)

	for _, r := range readers {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			idx := strings.Index(line, marker)
			if idx < 0 {
				continue
			}
			payload := line[idx+len(marker):]
			var e entry
			if err := json.Unmarshal([]byte(payload), &e); err != nil {
				continue
			}
			total++
			sources[e.Shadow.SessionSource]++
			shadowReasons[e.Shadow.Reason]++
			costs = append(costs, e.CostUs)
			byModelTotal[e.Model]++
			if e.Shadow.Reason == "no_candidate" {
				noCandidate++
				continue
			}
			if e.Match {
				matched++
				byModelMatch[e.Model]++
				continue
			}
			reasonPairs[fmt.Sprintf("%s → %s", e.Legacy.Reason, e.Shadow.Reason)]++
			channelPairs[fmt.Sprintf("ch%d → ch%d (%s)", e.Legacy.Channel, e.Shadow.Channel, e.Model)]++
		}
	}

	if total == 0 {
		fmt.Println("未找到影子对比日志（标记 [DispatchShadow]）")
		return
	}

	fmt.Printf("═══ 影子模式对比统计 ═══\n")
	fmt.Printf("样本总数:     %d\n", total)
	fmt.Printf("一致:         %d (%.2f%%)\n", matched, pct(matched, total))
	fmt.Printf("不一致:       %d (%.2f%%)\n", total-matched-noCandidate, pct(total-matched-noCandidate, total))
	fmt.Printf("影子无候选:   %d (%.2f%%)\n", noCandidate, pct(noCandidate, total))

	if len(costs) > 0 {
		slices.Sort(costs)
		fmt.Printf("影子决策耗时: p50=%dµs p99=%dµs max=%dµs（验收线 p99 < 1000µs）\n",
			costs[len(costs)/2], costs[len(costs)*99/100], costs[len(costs)-1])
	}

	printTop("\n── 会话键来源分布 ──", sources, total, 10)
	printTop("\n── 影子选择原因分布 ──", shadowReasons, total, 10)
	printTop("\n── 不一致归因（legacy_reason → shadow_reason）──", reasonPairs, total, 15)
	printTop("\n── 不一致渠道对 Top ──", channelPairs, total, 15)

	fmt.Printf("\n── 按模型一致率 ──\n")
	models := make([]string, 0, len(byModelTotal))
	for m := range byModelTotal {
		models = append(models, m)
	}
	sort.Strings(models)
	for _, m := range models {
		fmt.Printf("  %-40s %d/%d (%.2f%%)\n", m, byModelMatch[m], byModelTotal[m], pct(byModelMatch[m], byModelTotal[m]))
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

func printTop(title string, m map[string]int, total, limit int) {
	if len(m) == 0 {
		return
	}
	fmt.Println(title)
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(m))
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].v > items[j].v })
	if len(items) > limit {
		items = items[:limit]
	}
	for _, it := range items {
		fmt.Printf("  %-60s %d (%.2f%%)\n", it.k, it.v, pct(it.v, total))
	}
}
