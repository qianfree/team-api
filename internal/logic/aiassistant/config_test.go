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

// TestGenerationParams 代码生成模式必须给足输出预算、压低温度，且不改写更严格的配置。
func TestGenerationParams(t *testing.T) {
	// 默认配置（4096 / 0.7）在代码模式下被抬到 16384 / 0.2
	if mt, temp := GenerationParams(4096, 0.7, true); mt != CodeGenMaxTokens || temp != CodeGenTemperature {
		t.Fatalf("代码模式参数不符: %d / %v", mt, temp)
	}
	// 管理员配得更大/更低时沿用其值
	if mt, temp := GenerationParams(32768, 0.05, true); mt != 32768 || temp != 0.05 {
		t.Fatalf("不应覆盖更严格的配置: %d / %v", mt, temp)
	}
	// 非代码模式原样返回
	if mt, temp := GenerationParams(4096, 0.7, false); mt != 4096 || temp != 0.7 {
		t.Fatalf("非代码模式不应改动参数: %d / %v", mt, temp)
	}
}
