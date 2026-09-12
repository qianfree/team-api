package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/utility/corpus"
)

// corpus-extract：从 chn_debug_logs 提取协议转换测试语料。
//
// 为什么做成 CLI 而不是管理端接口：调试日志是四段完整报文（含请求体原文），
// 不该经 HTTP 出网；CLI 走应用自身配置连库，查询、去重、落盘一次完成。
//
// 默认对报文体做脱敏：自由文本等长替换、内联数据换占位图，同时**保持引用一致性**
// （tool_call_id / 工具名的映射在一条记录内前后一致）与判别式字段原值。
// --no-scrub 可关闭，但那样落盘的是报文原文，仅限确知无敏感数据的测试流量。

const (
	defaultCorpusOutput = "relaykit/relayconvert/register/golden"
	defaultScanLimit    = 5000
	defaultMaxInline    = 8192
)

var corpusExtractCmd = gcmd.Command{
	Name:  "corpus-extract",
	Usage: "corpus-extract [-o <目录>] [--since <时间>] [--channel <ID,...>] [--limit N]",
	Brief: "从渠道调试日志提取协议转换测试语料（默认脱敏）",
	Arguments: []gcmd.Argument{
		{Name: "output", Short: "o", Brief: "语料输出根目录（默认 " + defaultCorpusOutput + "）"},
		{Name: "since", Brief: "起始时间，如 2026-09-01 或 2026-09-01T10:00:00Z"},
		{Name: "until", Brief: "结束时间，同上"},
		{Name: "channel", Brief: "渠道 ID 过滤，逗号分隔"},
		{Name: "format", Brief: "客户端格式过滤：openai / claude / gemini / responses"},
		{Name: "limit", Brief: fmt.Sprintf("最多扫描日志行数（默认 %d）", defaultScanLimit)},
		{Name: "per-shape", Brief: "每种结构形状保留的样本数（默认 1）"},
		{Name: "max-inline", Brief: fmt.Sprintf("内联 base64 数据超过该字节数则替换为占位，控制语料体积（默认 %d，0=不处理）", defaultMaxInline)},
		{Name: "no-scrub", Brief: "关闭脱敏，落盘报文原文（仅限确知无敏感数据的测试流量）", Orphan: true},
		{Name: "keep-tool-names", Brief: "脱敏时保留自定义工具名原值（内置工具名始终保留）", Orphan: true},
		{Name: "include-errors", Brief: "包含失败尝试（默认只取成功的最终尝试）", Orphan: true},
		{Name: "dump-segments", Brief: "打印指定日志 ID（逗号分隔）的段1/段2报文片段，用于诊断"},
		{Name: "dry-run", Brief: "只统计不落盘", Orphan: true},
	},
	Func: runCorpusExtract,
}

func runCorpusExtract(ctx context.Context, parser *gcmd.Parser) error {
	output := parser.GetOpt("output", defaultCorpusOutput).String()
	limit := parser.GetOpt("limit", defaultScanLimit).Int()
	perShape := parser.GetOpt("per-shape", 1).Int()
	maxInline := parser.GetOpt("max-inline", defaultMaxInline).Int()
	noScrub := parser.GetOpt("no-scrub") != nil
	keepToolNames := parser.GetOpt("keep-tool-names") != nil
	includeErrors := parser.GetOpt("include-errors") != nil
	dryRun := parser.GetOpt("dry-run") != nil
	channelFilter := parser.GetOpt("channel").String()
	formatFilter := parser.GetOpt("format").String()
	since := parser.GetOpt("since").String()
	until := parser.GetOpt("until").String()

	rows, err := queryDebugLogs(ctx, debugLogQuery{
		since:         since,
		until:         until,
		channels:      channelFilter,
		format:        formatFilter,
		limit:         limit,
		includeErrors: includeErrors,
	})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("未查询到符合条件的调试日志。请确认：")
		fmt.Println("  1) 目标渠道已开启 debug_log_enabled；")
		fmt.Println("  2) 采集窗口内确有流量；")
		fmt.Println("  3) --since / --until / --channel 过滤条件是否过严。")
		return nil
	}

	scrubOpts := corpus.ScrubOptions{Enabled: !noScrub, KeepToolNames: keepToolNames}
	collector := corpus.NewCollector(perShape, scrubOpts)
	for _, row := range rows {
		collector.Add(row.toRecord(maxInline))
	}

	if includeErrors {
		printFailedAttempts(rows)
	}

	if ids := splitIDs(parser.GetOpt("dump-segments").String()); len(ids) > 0 {
		printSegments(rows, ids)
	}

	samples := collector.Samples()
	stats, err := corpus.Write(samples, collector.Stats(), corpus.WriteOptions{
		OutputDir: output,
		DryRun:    dryRun,
		Scrubbed:  !noScrub,
		Source: corpus.ManifestSource{
			Since: since, Until: until,
			Channel: channelFilter, Format: formatFilter,
			Limit: limit,
		},
	})
	if err != nil {
		return err
	}

	printCorpusSummary(output, stats, samples, dryRun, !noScrub)
	return nil
}

// printFailedAttempts 打印采集窗口内全部请求的时间线，失败尝试附上游状态、
// 错误信息与响应体片段（--include-errors 时）。段3 错误体按设计不进语料
// （不可作为响应语料），但它是排查「客户端没反应」类问题的关键证据。
func printFailedAttempts(rows []debugLogRow) {
	for _, r := range rows {
		created := ""
		if r.CreatedAt != nil {
			created = r.CreatedAt.Time.Format("15:04:05")
		}
		fmt.Printf("%s log=%-4d %-18s ch=%-2d retry=%d us=%-3d cs=%-3d stream=%-5v %s→%s\n",
			created, r.Id, r.ModelName, r.ChannelType, r.RetryIndex,
			r.UpstreamStatusCode, r.ClientStatusCode, r.IsStream,
			convClient(r.Conversion), convUpstream(r.Conversion))
		if r.Error != "" {
			fmt.Printf("    error: %s\n", truncateStr(r.Error, 300))
		}
		if r.UpstreamStatusCode != 200 && r.UpstreamStatusCode != 0 {
			body := decodeBody(r.UpstreamRespBody, r.UpstreamRespEncoding)
			fmt.Printf("    段3: %s\n", truncateStr(string(body), 400))
		}
	}
}

func truncateStr(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// printSegments 打印指定日志的段2（发往上游的最终请求）结构化摘要，诊断转换结果用。
// 只打字段级信息不打原文：报文体动辄数十 KB（codex 系统提示），原文会淹没真正要看的参数区。
func printSegments(rows []debugLogRow, ids []string) {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	for _, r := range rows {
		if !wanted[fmt.Sprintf("%d", r.Id)] {
			continue
		}
		fmt.Printf("===== log=%d %s %s→%s us=%d =====\n", r.Id, r.ModelName,
			convClient(r.Conversion), convUpstream(r.Conversion), r.UpstreamStatusCode)
		body := decodeBody(r.UpstreamReqBody, r.UpstreamReqEncoding)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			fmt.Printf("  段2 非 JSON（%v）: %s\n", err, truncateStr(string(body), 200))
			continue
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Printf("  顶层字段: %s\n", strings.Join(keys, ", "))
		for _, k := range []string{"model", "max_tokens", "max_completion_tokens", "reasoning_effort", "temperature", "top_p", "stream"} {
			if v, ok := m[k]; ok {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
		if v, ok := m["web_search_options"]; ok {
			fmt.Printf("  web_search_options: %s\n", truncateStr(fmt.Sprint(v), 200))
		} else {
			fmt.Println("  web_search_options: <无>")
		}
		if tools, ok := m["tools"].([]any); ok {
			types := map[string]int{}
			for _, t := range tools {
				if tm, ok := t.(map[string]any); ok {
					if ty, _ := tm["type"].(string); ty != "" {
						types[ty]++
					}
				}
			}
			fmt.Printf("  tools: %d 个 %v\n", len(tools), types)
		} else {
			fmt.Println("  tools: <无>")
		}
		if msgs, ok := m["messages"].([]any); ok {
			roles := map[string]int{}
			for _, msg := range msgs {
				if mm, ok := msg.(map[string]any); ok {
					if r, _ := mm["role"].(string); r != "" {
						roles[r]++
					}
				}
			}
			fmt.Printf("  messages: %d 条 %v\n", len(msgs), roles)
		}
	}
}

func convClient(conversion string) string {
	var c struct {
		ClientFormat string `json:"client_format"`
	}
	_ = json.Unmarshal([]byte(conversion), &c)
	return c.ClientFormat
}

func convUpstream(conversion string) string {
	var c struct {
		UpstreamFormat string `json:"upstream_format"`
	}
	_ = json.Unmarshal([]byte(conversion), &c)
	return c.UpstreamFormat
}

func printCorpusSummary(output string, stats corpus.ManifestStats, samples []corpus.Sample, dryRun, scrubbed bool) {
	fmt.Printf("\n扫描 %d 条日志 → %d 种结构形状 → %d 个语料文件\n",
		stats.Scanned, stats.Shapes, stats.Written)
	if stats.SkippedNoConv > 0 {
		fmt.Printf("  跳过 %d 条（缺 conversion 协议元数据，可能是非 relay 路径）\n", stats.SkippedNoConv)
	}
	if stats.SkippedEmpty > 0 {
		fmt.Printf("  跳过 %d 条（报文体为空或非 JSON）\n", stats.SkippedEmpty)
	}

	byDir := map[string]int{}
	for _, s := range samples {
		if s.Meta.ClientFormat != "" && s.Meta.UpstreamFormat != "" {
			byDir[s.Meta.ClientFormat+"→"+s.Meta.UpstreamFormat]++
		}
	}
	if len(byDir) > 0 {
		fmt.Println("\n转换方向覆盖：")
		for dir, n := range byDir {
			fmt.Printf("  %-28s %d\n", dir, n)
		}
		fmt.Println("（方向覆盖不全时，补充对应渠道的调试采集再跑一次）")
	}

	if dryRun {
		fmt.Println("\n[dry-run] 未写入任何文件")
		return
	}
	fmt.Printf("\n语料已写入 %s/{real_inputs,real_stream_inputs,real_responses}\n", output)
	fmt.Printf("清单：%s/corpus_manifest.json\n", output)
	if scrubbed {
		fmt.Println("\n语料已脱敏（自由文本等长替换、标识符一致映射、内联数据换占位图）；" +
			"脱敏是启发式的，提交前建议抽查确认。")
	} else {
		fmt.Println("\n⚠️ 语料为报文原文、未做脱敏，提交前请确认其中不含真实业务数据。")
	}
}

// ---------- 数据库查询 ----------

type debugLogQuery struct {
	since, until  string
	channels      string
	format        string
	limit         int
	includeErrors bool
}

// debugLogRow 与 chn_debug_logs 查询字段对应。
// （dump-segments 诊断需要段2 发往上游的最终请求体）
type debugLogRow struct {
	Id                   int64       `orm:"id"`
	RequestId            string      `orm:"request_id"`
	ChannelType          int         `orm:"channel_type"`
	ModelName            string      `orm:"model_name"`
	UpstreamModel        string      `orm:"upstream_model"`
	RelayMode            string      `orm:"relay_mode"`
	InboundPath          string      `orm:"inbound_path"`
	IsStream             bool        `orm:"is_stream"`
	RetryIndex           int         `orm:"retry_index"`
	UpstreamStatusCode   int         `orm:"upstream_status_code"`
	ClientStatusCode     int         `orm:"client_status_code"`
	Error                string      `orm:"error"`
	ClientReqBody        string      `orm:"client_req_body"`
	ClientReqEncoding    string      `orm:"client_req_encoding"`
	UpstreamReqBody      string      `orm:"upstream_req_body"`
	UpstreamReqEncoding  string      `orm:"upstream_req_encoding"`
	UpstreamRespBody     string      `orm:"upstream_resp_body"`
	UpstreamRespEncoding string      `orm:"upstream_resp_encoding"`
	Conversion           string      `orm:"conversion"`
	CreatedAt            *gtime.Time `orm:"created_at"`
}

func queryDebugLogs(ctx context.Context, q debugLogQuery) ([]debugLogRow, error) {
	m := dao.ChnDebugLogs.Ctx(ctx).
		Fields("id,request_id,channel_type,model_name,upstream_model,relay_mode,inbound_path,"+
			"is_stream,retry_index,upstream_status_code,client_status_code,error,"+
			"client_req_body,client_req_encoding,upstream_req_body,upstream_req_encoding,"+
			"upstream_resp_body,upstream_resp_encoding,conversion,created_at").
		// 只取最终尝试：中间失败尝试的段1 与最终尝试相同，段3 多为错误体
		Where("is_final", true).
		OrderDesc("id")

	if !q.includeErrors {
		// 只取上游成功的尝试：失败尝试的段3 是错误体，不是可用的响应语料
		m = m.Where("upstream_status_code", 200)
	}
	if q.since != "" {
		t, err := parseTimeFlag(q.since)
		if err != nil {
			return nil, fmt.Errorf("--since 时间格式无法解析: %w", err)
		}
		m = m.WhereGTE("created_at", t)
	}
	if q.until != "" {
		t, err := parseTimeFlag(q.until)
		if err != nil {
			return nil, fmt.Errorf("--until 时间格式无法解析: %w", err)
		}
		m = m.WhereLTE("created_at", t)
	}
	if ids := splitIDs(q.channels); len(ids) > 0 {
		m = m.WhereIn("channel_id", ids)
	}
	if q.format != "" {
		// conversion 是 JSONB，按 client_format 过滤
		m = m.Where("conversion->>'client_format' = ?", q.format)
	}
	if q.limit > 0 {
		m = m.Limit(q.limit)
	}

	var rows []debugLogRow
	if err := m.Scan(&rows); err != nil {
		return nil, fmt.Errorf("查询调试日志失败: %w", err)
	}
	return rows, nil
}

// toRecord 解码报文体并解析协议元数据。
// maxInline > 0 时把超长的 base64 内联数据替换为占位，避免单条语料膨胀到数 MB。
func (r debugLogRow) toRecord(maxInline int) corpus.Record {
	rec := corpus.Record{
		ID:             r.Id,
		RequestID:      r.RequestId,
		ChannelType:    r.ChannelType,
		ModelName:      r.ModelName,
		UpstreamModel:  r.UpstreamModel,
		RelayMode:      r.RelayMode,
		InboundPath:    r.InboundPath,
		IsStream:       r.IsStream,
		UpstreamStatus: r.UpstreamStatusCode,

		ClientReqBody:    decodeBody(r.ClientReqBody, r.ClientReqEncoding),
		UpstreamRespBody: decodeBody(r.UpstreamRespBody, r.UpstreamRespEncoding),
	}
	if r.CreatedAt != nil {
		rec.CapturedAt = r.CreatedAt.Time
	}
	if r.Conversion != "" {
		_ = json.Unmarshal([]byte(r.Conversion), &rec.Conversion)
	}
	if maxInline > 0 {
		rec.ClientReqBody = corpus.ShrinkInlineData(rec.ClientReqBody, maxInline)
		rec.UpstreamRespBody = corpus.ShrinkInlineData(rec.UpstreamRespBody, maxInline)
	}
	return rec
}

// decodeBody 按 encoding 列还原原始字节（plain 直取、base64 解码）。
func decodeBody(body, encoding string) []byte {
	if body == "" {
		return nil
	}
	if encoding == "base64" {
		if raw, err := base64.StdEncoding.DecodeString(body); err == nil {
			return raw
		}
		return nil
	}
	return []byte(body)
}

func parseTimeFlag(s string) (*gtime.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		// 无时区的格式按本地时区解释（time.Parse 会按 UTC，造成「白天的时间被当成晚上」
		// 而查不到日志）；RFC3339 自带时区偏移，Parse 与 ParseInLocation 行为一致
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return gtime.NewFromTime(t), nil
		}
	}
	return nil, fmt.Errorf("支持的格式：2006-01-02 / 2006-01-02 15:04:05 / RFC3339，实际收到 %q", s)
}

func splitIDs(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func init() {
	if err := Main.AddCommand(&corpusExtractCmd); err != nil {
		panic(err)
	}
}
