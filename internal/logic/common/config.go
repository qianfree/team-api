package common

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
)

// ConfigService provides configuration management with a registry-driven schema,
// typed accessors, validation, and Redis Pub/Sub propagation.
type ConfigService struct {
	cache *Cache
	mu    sync.RWMutex
}

var (
	configService     *ConfigService
	configServiceOnce sync.Once
)

// Config returns the singleton ConfigService instance.
func Config() *ConfigService {
	configServiceOnce.Do(func() {
		configService = &ConfigService{
			cache: NewCache("opt", 10*time.Minute),
		}
	})
	return configService
}

// Option represents a system configuration entry.
type Option struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsPublic    bool   `json:"is_public"`
}

// ──────────────────────────────────────────
//  Core accessors (backward compatible)
// ──────────────────────────────────────────

// GetOption retrieves a configuration value by key.
// Falls back to registry default if not in DB/cache.
func (s *ConfigService) GetOption(ctx context.Context, key string) string {
	val, ok := s.cache.Get(ctx, key)
	if ok {
		return gconv.String(val)
	}

	var option Option
	err := dao.SysOptions.Ctx(ctx).
		Where("key", key).
		Scan(&option)
	if err != nil || option.Key == "" {
		if def := GetSettingDef(key); def != nil {
			return def.Default
		}
		return ""
	}

	s.cache.Set(ctx, key, option.Value)
	return option.Value
}

// GetOptionJSON retrieves a configuration value and unmarshals it as JSON.
func (s *ConfigService) GetOptionJSON(ctx context.Context, key string, target any) error {
	val := s.GetOption(ctx, key)
	if val == "" {
		return nil
	}
	return json.Unmarshal([]byte(val), target)
}

// ──────────────────────────────────────────
//  Typed accessors
// ──────────────────────────────────────────

// GetString returns the setting value as a string.
func (s *ConfigService) GetString(ctx context.Context, key string) string {
	return s.GetOption(ctx, key)
}

// GetInt returns the setting value as an int. Falls back to registry default.
func (s *ConfigService) GetInt(ctx context.Context, key string) int {
	val := s.GetOption(ctx, key)
	if val == "" {
		return 0
	}
	return gconv.Int(val)
}

// GetFloat returns the setting value as a float64.
func (s *ConfigService) GetFloat(ctx context.Context, key string) float64 {
	val := s.GetOption(ctx, key)
	if val == "" {
		return 0
	}
	return gconv.Float64(val)
}

// GetBool returns the setting value as a bool.
func (s *ConfigService) GetBool(ctx context.Context, key string) bool {
	val := s.GetOption(ctx, key)
	return val == "true" || val == "1"
}

// ──────────────────────────────────────────
//  Category loading
// ──────────────────────────────────────────

// LoadCategoryMap loads all settings in a category and returns them as a key→value map.
func (s *ConfigService) LoadCategoryMap(ctx context.Context, category string) map[string]string {
	defs := GetCategorySettings(category)
	result := make(map[string]string, len(defs))
	for _, def := range defs {
		result[def.Key] = s.GetOption(ctx, def.Key)
	}
	return result
}

// ──────────────────────────────────────────
//  Schema-aware read for API
// ──────────────────────────────────────────

// SettingItem represents a single setting with its schema and current value (for API response).
type SettingItem struct {
	Key         string      `json:"key"`
	Value       string      `json:"value"`
	Type        SettingType `json:"type"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
	Validation  string      `json:"validation,omitempty"`
	Default     string      `json:"default"`
}

// GetCategoryWithValues returns schema + current values for a category.
func (s *ConfigService) GetCategoryWithValues(ctx context.Context, category string) []SettingItem {
	defs := GetCategorySettings(category)
	items := make([]SettingItem, 0, len(defs))
	for _, def := range defs {
		val := s.GetOption(ctx, def.Key)
		if def.Sensitive && val != "" {
			val = "******"
		}
		items = append(items, SettingItem{
			Key:         def.Key,
			Value:       val,
			Type:        def.Type,
			Label:       def.Label,
			Description: def.Description,
			Sensitive:   def.Sensitive,
			Validation:  def.Validation,
			Default:     def.Default,
		})
	}
	return items
}

// ──────────────────────────────────────────
//  Write with validation
// ──────────────────────────────────────────

// SetOption updates a configuration value with validation, then invalidates cache.
func (s *ConfigService) SetOption(ctx context.Context, key, value string) error {
	if def := GetSettingDef(key); def != nil {
		// Sanitize: strip surrounding quotes that may have been double-encoded
		value = sanitizeSettingValue(def.Type, value)
		if err := validateSettingValue(def, value); err != nil {
			return err
		}
	}

	count, err := dao.SysOptions.Ctx(ctx).Where("key", key).Count()
	if err != nil {
		return err
	}

	if count > 0 {
		_, err = dao.SysOptions.Ctx(ctx).
			Where("key", key).
			Data(do.SysOptions{Value: value}).
			Update()
	} else {
		def := GetSettingDef(key)
		category := ""
		description := ""
		isPublic := false
		if def != nil {
			category = def.Category
			description = def.Label
			isPublic = def.IsPublic
		}
		_, err = dao.SysOptions.Ctx(ctx).
			Data(do.SysOptions{
				Key:         key,
				Value:       value,
				Category:    category,
				Description: description,
				IsPublic:    isPublic,
			}).Insert()
	}
	if err != nil {
		return err
	}

	s.cache.Delete(ctx, key)
	s.cache.Delete(ctx, "public_options")
	s.cache.Delete(ctx, "category:"+getCategoryForKey(key))
	s.publishChange(key)
	return nil
}

// UpdateCategory batch-updates all settings in a category.
// Skips masked sensitive values ("******").
func (s *ConfigService) UpdateCategory(ctx context.Context, category string, values map[string]string) error {
	defs := GetCategorySettings(category)
	defMap := make(map[string]*SettingDef, len(defs))
	for i := range defs {
		defMap[defs[i].Key] = &defs[i]
	}

	for key, value := range values {
		def, ok := defMap[key]
		if !ok {
			return fmt.Errorf("配置项 %s 不属于分类 %s", key, category)
		}
		if def.Sensitive && value == "******" {
			continue
		}
		if err := s.SetOption(ctx, key, value); err != nil {
			return fmt.Errorf("配置项 %s: %w", key, err)
		}
	}
	return nil
}

// ──────────────────────────────────────────
//  Query helpers (backward compatible)
// ──────────────────────────────────────────

// GetPublicOptions retrieves all public configuration options.
func (s *ConfigService) GetPublicOptions(ctx context.Context) ([]Option, error) {
	cacheKey := "public_options"
	val, ok := s.cache.Get(ctx, cacheKey)
	if ok {
		if options, ok := val.([]Option); ok {
			return options, nil
		}
	}

	var options []Option
	err := dao.SysOptions.Ctx(ctx).
		Where("is_public", true).
		Scan(&options)
	if err != nil {
		return nil, err
	}

	s.cache.Set(ctx, cacheKey, options)
	return options, nil
}

// GetAllOptionsByCategory retrieves all options in a given category.
func (s *ConfigService) GetAllOptionsByCategory(ctx context.Context, category string) ([]Option, error) {
	cacheKey := "category:" + category
	val, ok := s.cache.Get(ctx, cacheKey)
	if ok {
		if options, ok := val.([]Option); ok {
			return options, nil
		}
	}

	var options []Option
	err := dao.SysOptions.Ctx(ctx).
		Where("category", category).
		Scan(&options)
	if err != nil {
		return nil, err
	}

	s.cache.Set(ctx, cacheKey, options)
	return options, nil
}

// ──────────────────────────────────────────
//  Warmup & Pub/Sub
// ──────────────────────────────────────────

// Warmup loads all public options into cache on startup.
func (s *ConfigService) Warmup(ctx context.Context) {
	_, _ = s.GetPublicOptions(ctx)
	g.Log().Info(ctx, "config service warmup completed")
}

// StartSubscriber subscribes to Redis Pub/Sub for cross-instance cache invalidation.
// Called once at startup in cmd.go.
func (s *ConfigService) StartSubscriber(ctx context.Context) {
	go func() {
		for {
			conn, _, err := g.Redis().Subscribe(ctx, "settings:changed")
			if err != nil {
				g.Log().Errorf(ctx, "settings subscriber connect failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			g.Log().Info(ctx, "settings Pub/Sub subscriber started")

			for {
				v, err := conn.Receive(ctx)
				if err != nil {
					g.Log().Warningf(ctx, "settings subscriber recv error: %v", err)
					time.Sleep(5 * time.Second)
					break // reconnect
				}

				msg, ok := v.Val().(*gredis.Message)
				if !ok {
					continue // skip Subscription/Pong etc.
				}

				key := msg.Payload
				s.cache.Delete(ctx, key)
				s.cache.Delete(ctx, "public_options")
				s.cache.Delete(ctx, "category:"+getCategoryForKey(key))
				g.Log().Debugf(ctx, "settings invalidated via Pub/Sub: %s", key)
			}

			conn.Close(ctx)
		}
	}()
}

// ──────────────────────────────────────────
//  Validation
// ──────────────────────────────────────────

func sanitizeSettingValue(_ SettingType, value string) string {
	if value == "" {
		return value
	}
	// Strip surrounding quotes caused by double-encoding (e.g. "\"10\"" → "10")
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		value = value[1 : len(value)-1]
	}
	return value
}

func validateSettingValue(def *SettingDef, value string) error {
	if def.Validation == "" {
		return nil
	}

	// Empty value with a non-empty default → use default
	if value == "" && def.Default != "" {
		return nil
	}

	// Enum validation: "enum:a,b,c"
	if strings.HasPrefix(def.Validation, "enum:") {
		allowed := strings.Split(strings.TrimPrefix(def.Validation, "enum:"), ",")
		for _, a := range allowed {
			if value == a {
				return nil
			}
		}
		return fmt.Errorf("值必须是 %s 之一", strings.Join(allowed, "/"))
	}

	// Range validation: "min:1,max:100"
	if strings.Contains(def.Validation, "min:") || strings.Contains(def.Validation, "max:") {
		numVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("必须是数字")
		}
		parts := strings.Split(def.Validation, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "min:") {
				minVal, _ := strconv.ParseFloat(strings.TrimPrefix(p, "min:"), 64)
				if numVal < minVal {
					return fmt.Errorf("不能小于 %v", minVal)
				}
			}
			if strings.HasPrefix(p, "max:") {
				maxVal, _ := strconv.ParseFloat(strings.TrimPrefix(p, "max:"), 64)
				if numVal > maxVal {
					return fmt.Errorf("不能大于 %v", maxVal)
				}
			}
		}
	}

	return nil
}

func getCategoryForKey(key string) string {
	if def := GetSettingDef(key); def != nil {
		return def.Category
	}
	return ""
}

// publishChange notifies other instances via Redis Pub/Sub to invalidate their local cache.
func (s *ConfigService) publishChange(key string) {
	_, _ = g.Redis().Publish(context.Background(), "settings:changed", key)
}

// TypedValue converts a string value to its proper Go type based on SettingType.
// Returns the typed value for JSON serialization (numbers as float64, bools as bool).
func TypedValue(typ SettingType, value string, defaultVal string) any {
	str := value
	if str == "" {
		str = defaultVal
	}
	switch typ {
	case SettingTypeInt:
		v, _ := strconv.ParseInt(str, 10, 64)
		return float64(v)
	case SettingTypeFloat:
		v, _ := strconv.ParseFloat(str, 64)
		return v
	case SettingTypeBool:
		return str == "true" || str == "1"
	default:
		return str
	}
}

// NormalizeSettingValue converts an interface{} value (from JSON request) to a string for storage.
func NormalizeSettingValue(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case float64:
		// JSON numbers arrive as float64; convert to clean integer string when possible
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	default:
		return gconv.String(v)
	}
}
