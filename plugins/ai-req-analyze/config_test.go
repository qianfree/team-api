package aireqanalyze

import (
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestConfigFromMapDefaults(t *testing.T) {
	cfg := configFromMap(nil)
	if cfg.AITimeoutMs != 60000 || cfg.AIMaxTokens != 4096 || cfg.UnitsPerBatch != 6 ||
		cfg.SnippetMaxLen != 4000 || cfg.AIEndpointMode != EndpointModeBase ||
		cfg.PatrolCron != "0 3 * * *" || cfg.PatrolWindowH != 24 || cfg.RetentionDays != 90 {
		t.Fatalf("默认值不符合预期: %+v", cfg)
	}
	if cfg.PatrolEnabled {
		t.Fatal("巡查默认关闭")
	}
}

func TestConfigEndpointMode(t *testing.T) {
	// full 生效；非法/空值回落 base（错配 full 会把请求打到根路径，宁可保守拼接）
	if cfg := configFromMap(g.Map{"ai_endpoint_mode": "full"}); cfg.AIEndpointMode != EndpointModeFull {
		t.Fatalf("full 模式未生效: %q", cfg.AIEndpointMode)
	}
	if cfg := configFromMap(g.Map{"ai_endpoint_mode": "FULL"}); cfg.AIEndpointMode != EndpointModeBase {
		t.Fatalf("大小写敏感，非法值应回落 base: %q", cfg.AIEndpointMode)
	}
	if cfg := configFromMap(g.Map{"ai_endpoint_mode": "whatever"}); cfg.AIEndpointMode != EndpointModeBase {
		t.Fatalf("非法值应回落 base: %q", cfg.AIEndpointMode)
	}
}

func TestConfigFromMapOverrides(t *testing.T) {
	cfg := configFromMap(g.Map{
		"ai_base_url":     "http://ollama:11434/v1",
		"ai_api_key":      "secret",
		"ai_model":        "qwen2.5:32b",
		"units_per_batch": 10,
		"custom_rules":    "禁止分析医疗数据",
		"patrol_enabled":  true,
		"patrol_scope":    `{"tenants":[1,2,3]}`,
	})
	if cfg.AIBaseURL != "http://ollama:11434/v1" || cfg.AIAPIKey != "secret" || cfg.AIModel != "qwen2.5:32b" {
		t.Fatalf("连接配置覆盖失败: %+v", cfg)
	}
	if cfg.UnitsPerBatch != 10 {
		t.Fatalf("批量覆盖失败: %d", cfg.UnitsPerBatch)
	}
	if cfg.CustomRules != "禁止分析医疗数据" {
		t.Fatalf("自定义规则覆盖失败: %q", cfg.CustomRules)
	}
	if !cfg.PatrolEnabled {
		t.Fatal("巡查开关覆盖失败")
	}
	if len(cfg.PatrolScope) != 3 || cfg.PatrolScope[0] != 1 || cfg.PatrolScope[2] != 3 {
		t.Fatalf("巡查范围解析失败: %v", cfg.PatrolScope)
	}
}

func TestConfigFromMapInvalidValuesFallBack(t *testing.T) {
	// 零值/负数/低于下限的值回落默认，不产生危险配置
	cfg := configFromMap(g.Map{
		"ai_timeout_ms":   0,
		"ai_max_tokens":   -5,
		"units_per_batch": 0,
		"snippet_max_len": 10,
		"patrol_window_h": 0,
		"retention_days":  -1,
		"patrol_cron":     "",
	})
	if cfg.AITimeoutMs != 60000 || cfg.AIMaxTokens != 4096 || cfg.UnitsPerBatch != 6 {
		t.Fatalf("零值回落失败: %+v", cfg)
	}
	if cfg.SnippetMaxLen != 4000 {
		t.Fatalf("低于下限回落失败: %+v", cfg)
	}
	if cfg.PatrolWindowH != 24 || cfg.RetentionDays != 90 || cfg.PatrolCron != "0 3 * * *" {
		t.Fatalf("巡查配置回落失败: %+v", cfg)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (&Config{}).Validate(); err == nil {
		t.Fatal("缺少端点应报错")
	}
	if err := (&Config{AIBaseURL: "http://x/v1"}).Validate(); err == nil {
		t.Fatal("缺少模型应报错")
	}
	if err := (&Config{AIBaseURL: "http://x/v1", AIModel: "m"}).Validate(); err != nil {
		t.Fatalf("配置齐全不应报错: %v", err)
	}
}
