package aiassistant

import (
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

// TestConfigFromMapDefaults 空配置回落默认值（与设置注册表 Default 对齐）。
func TestConfigFromMapDefaults(t *testing.T) {
	cfg := configFromMap(nil)
	if cfg.Enabled {
		t.Fatal("默认未启用")
	}
	if cfg.EndpointMode != EndpointModeBase {
		t.Fatalf("端点模式默认 base，得到 %s", cfg.EndpointMode)
	}
	if cfg.MaxTokens != 4096 || cfg.TimeoutMs != 120000 || cfg.HistoryLimit != 20 || cfg.RateLimitPerMin != 30 {
		t.Fatalf("默认值不符: %+v", cfg)
	}
	if cfg.Temperature != 0.7 {
		t.Fatalf("默认温度 0.7，得到 %v", cfg.Temperature)
	}
}

// TestConfigFromMapEndpointMode 非法端点模式回落 base（错配 full 会打到根路径，保守拼接）。
func TestConfigFromMapEndpointMode(t *testing.T) {
	cfg := configFromMap(g.Map{"ai_assistant_endpoint_mode": "full"})
	if cfg.EndpointMode != EndpointModeFull {
		t.Fatalf("full 应原样生效，得到 %s", cfg.EndpointMode)
	}
	cfg = configFromMap(g.Map{"ai_assistant_endpoint_mode": "weird"})
	if cfg.EndpointMode != EndpointModeBase {
		t.Fatalf("非法值应回落 base，得到 %s", cfg.EndpointMode)
	}
}

// TestConfigFromMapInvalidValues 零值/负数回落默认值。
func TestConfigFromMapInvalidValues(t *testing.T) {
	cfg := configFromMap(g.Map{
		"ai_assistant_max_tokens":    10, // 低于下限 → 默认
		"ai_assistant_temperature":   -1, // 负数 → 默认
		"ai_assistant_history_limit": 0,  // 零值 → 默认
	})
	if cfg.MaxTokens != 4096 {
		t.Fatalf("max_tokens 低值应回落默认，得到 %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Fatalf("负温度应回落默认，得到 %v", cfg.Temperature)
	}
	if cfg.HistoryLimit != 20 {
		t.Fatalf("history_limit 零值应回落默认，得到 %d", cfg.HistoryLimit)
	}
}

// TestConfigValidate 必填校验。
func TestConfigValidate(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("缺端点应报错")
	}
	cfg.BaseURL = "http://x"
	if err := cfg.Validate(); err == nil {
		t.Fatal("缺模型应报错")
	}
	cfg.Model = "m"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("配置齐全不应报错: %v", err)
	}
}
