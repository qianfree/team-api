package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Collector 按结构指纹聚合样本：同形状只保留前 perShape 条代表，
// 并统计该形状的出现次数（反映真实流量权重，写进 manifest）。
type Collector struct {
	perShape int
	scrub    ScrubOptions

	// key = kind + "/" + shapeHash
	kept   map[string][]Sample
	counts map[string]int
	order  []string // 首次出现顺序，保证输出稳定

	stats ManifestStats
}

// NewCollector 创建收集器。perShape < 1 时按 1 处理。
// scrub 为脱敏选项，逐记录新建 Scrubber（映射表按记录隔离）。
func NewCollector(perShape int, scrub ScrubOptions) *Collector {
	if perShape < 1 {
		perShape = 1
	}
	return &Collector{
		perShape: perShape,
		scrub:    scrub,
		kept:     make(map[string][]Sample),
		counts:   make(map[string]int),
	}
}

// Add 处理一条调试日志。返回本条产生的新增样本数（已达 perShape 上限时为 0）。
func (c *Collector) Add(r Record) int {
	c.stats.Scanned++

	if r.Conversion.ClientFormat == "" && r.Conversion.UpstreamFormat == "" {
		c.stats.SkippedNoConv++
		return 0
	}
	samples := Build(r, NewScrubber(c.scrub))
	if len(samples) == 0 {
		c.stats.SkippedEmpty++
		return 0
	}

	added := 0
	for _, s := range samples {
		key := string(s.Kind) + "/" + s.ShapeHash
		if _, exists := c.counts[key]; !exists {
			c.order = append(c.order, key)
		}
		c.counts[key]++
		if len(c.kept[key]) < c.perShape {
			c.kept[key] = append(c.kept[key], s)
			added++
		}
	}
	return added
}

// Samples 返回去重后的样本（按首次出现顺序），并回填 Occurrences。
func (c *Collector) Samples() []Sample {
	out := make([]Sample, 0, len(c.order))
	for _, key := range c.order {
		for _, s := range c.kept[key] {
			s.Meta.Occurrences = c.counts[key]
			out = append(out, s)
		}
	}
	return out
}

// Stats 返回提取统计（Shapes/Written 在 Write 时补全）。
func (c *Collector) Stats() ManifestStats {
	s := c.stats
	s.Shapes = len(c.order)
	return s
}

// WriteOptions 落盘选项。
type WriteOptions struct {
	// OutputDir 语料根目录（各类别落在其下的 real_inputs / real_stream_inputs / real_responses）
	OutputDir string
	// Source 采集条件，写进 manifest 供溯源
	Source ManifestSource
	// DryRun 只统计不写文件
	DryRun bool
	// Scrubbed 本次提取是否启用了脱敏（写进 manifest 供使用方判断语料可信度）
	Scrubbed bool
}

// Write 把样本落盘并生成 manifest.json，返回最终统计。
//
// real_* 目录是本工具全域生成的派生物，每次提取先清空重建：文件名含结构指纹，
// 脱敏/指纹语义变化会改变哈希进而改变文件名，不清空会让旧文件成为孤儿——
// 目录内容与 manifest 脱节，且会被按目录枚举的下游测试误读。
// 手写语料目录（inputs / stream_inputs / expected）不在清理范围。
func Write(samples []Sample, stats ManifestStats, opts WriteOptions) (ManifestStats, error) {
	if !opts.DryRun {
		for _, kind := range []SampleKind{KindRequest, KindStreamResponse, KindResponse} {
			if err := os.RemoveAll(filepath.Join(opts.OutputDir, kind.dirOf())); err != nil {
				return stats, fmt.Errorf("清理旧语料目录失败: %w", err)
			}
		}
	}
	manifest := Manifest{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source:      opts.Source,
		Directions:  map[string]int{},
		Formats:     map[string]int{},
		Notes: map[string]string{
			"purpose":    "真实流量提取的协议转换语料；请求语料驱动请求侧全部方向，响应/流式语料驱动对应上游格式的转换器",
			"dedup":      "按结构指纹去重，occurrences 为该形状在采集窗口内的出现次数",
			"scrub":      scrubNote(opts.Scrubbed),
			"regenerate": "重新采集：team-api corpus-extract -o <本目录>",
		},
	}

	for _, s := range samples {
		if !opts.DryRun {
			full := filepath.Join(opts.OutputDir, filepath.FromSlash(s.RelPath()))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return stats, fmt.Errorf("创建语料目录失败: %w", err)
			}
			if err := os.WriteFile(full, s.Body, 0o644); err != nil {
				return stats, fmt.Errorf("写入语料 %s 失败: %w", s.RelPath(), err)
			}
		}
		stats.Written++
		manifest.Entries = append(manifest.Entries, s.Meta)

		if s.Meta.ClientFormat != "" && s.Meta.UpstreamFormat != "" {
			manifest.Directions[s.Meta.ClientFormat+"→"+s.Meta.UpstreamFormat]++
		}
		manifest.Formats[string(s.Kind)+":"+s.Format]++
	}

	manifest.Stats = stats
	sort.Slice(manifest.Entries, func(i, j int) bool {
		return manifest.Entries[i].File < manifest.Entries[j].File
	})

	if opts.DryRun {
		return stats, nil
	}

	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return stats, fmt.Errorf("序列化 manifest 失败: %w", err)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return stats, fmt.Errorf("创建输出目录失败: %w", err)
	}
	manifestPath := filepath.Join(opts.OutputDir, "corpus_manifest.json")
	if err := os.WriteFile(manifestPath, append(raw, '\n'), 0o644); err != nil {
		return stats, fmt.Errorf("写入 manifest 失败: %w", err)
	}
	return stats, nil
}

// scrubNote manifest 中的脱敏说明。
func scrubNote(scrubbed bool) string {
	if scrubbed {
		return "已脱敏：自由文本等长替换、内联数据换占位图；标识符与工具名一致映射（引用关系保留）；" +
			"判别式字段（type/role/finish_reason/model）与协议内置工具名保留原值。" +
			"注意脱敏经过重新序列化，key 顺序与原文不同，不适合做字节级 wire 保真校验。"
	}
	return "未脱敏——落盘为报文原文，仅供确知无敏感数据的测试流量使用"
}
