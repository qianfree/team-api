package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

const (
	litellmPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	litellmCacheKey   = "litellm_pricing_data"
	litellmCacheTTL   = 24 * time.Hour
)

// LiteLLMModelEntry represents a single model entry in the LiteLLM pricing JSON.
type LiteLLMModelEntry struct {
	LitellmProvider             string  `json:"litellm_provider"`
	Mode                        string  `json:"mode"`
	MaxInputTokens              int     `json:"max_input_tokens"`
	MaxOutputTokens             int     `json:"max_output_tokens"`
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	InputCostPerCharacter       float64 `json:"input_cost_per_character"`
	OutputCostPerCharacter      float64 `json:"output_cost_per_character"`
	OutputCostPerImage          float64 `json:"output_cost_per_image"`
	SupportsVision              bool    `json:"supports_vision"`
	SupportsFunctionCalling     bool    `json:"supports_function_calling"`
	SupportsParallelFuncCalling bool    `json:"supports_parallel_function_calling"`
	SupportsToolChoice          bool    `json:"supports_tool_choice"`
	SupportsResponseSchema      bool    `json:"supports_response_schema"`
	SupportsSystemMessages      bool    `json:"supports_system_messages"`
	SupportsPromptCaching       bool    `json:"supports_prompt_caching"`
	SupportsAudioInput          bool    `json:"supports_audio_input"`
	SupportsAudioOutput         bool    `json:"supports_audio_output"`
	SupportsPdfInput            bool    `json:"supports_pdf_input"`
	SupportsEmbeddingImage      bool    `json:"supports_embedding_image"`
	SupportsReasoning           bool    `json:"supports_reasoning"`
	SupportsWebSearch           bool    `json:"supports_web_search"`
	DeprecationDate             string  `json:"deprecation_date"`
}

var litellmCache = gcache.New()

// FetchLiteLLMPricing fetches and caches the LiteLLM pricing data.
// Returns the full map of model_name → LiteLLMModelEntry.
func FetchLiteLLMPricing(ctx context.Context) (map[string]*LiteLLMModelEntry, error) {
	cached, err := litellmCache.Get(ctx, litellmCacheKey)
	if err == nil && cached != nil {
		if data, ok := cached.Val().(map[string]*LiteLLMModelEntry); ok {
			return data, nil
		}
	}

	g.Log().Info(ctx, "[PricingSource] fetching LiteLLM pricing data from remote...")

	resp, err := g.Client().SetHeaderMap(map[string]string{
		"User-Agent": "github.com/qianfree/team-api/1.0",
	}).Get(ctx, litellmPricingURL)
	if err != nil {
		return nil, fmt.Errorf("获取远程数据失败: %w", err)
	}
	defer resp.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取远程数据失败: HTTP %d", resp.StatusCode)
	}

	body := resp.ReadAll()
	if len(body) == 0 {
		return nil, fmt.Errorf("获取远程数据失败: 响应为空")
	}

	var rawEntries map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawEntries); err != nil {
		return nil, fmt.Errorf("获取远程数据失败: JSON 解析失败: %w", err)
	}

	delete(rawEntries, "sample_spec")

	raw := make(map[string]*LiteLLMModelEntry, len(rawEntries))
	for name, rawMsg := range rawEntries {
		var entry LiteLLMModelEntry
		if err := json.Unmarshal(rawMsg, &entry); err != nil {
			continue
		}
		raw[name] = &entry
	}

	if err := litellmCache.Set(ctx, litellmCacheKey, raw, litellmCacheTTL); err != nil {
		g.Log().Warningf(ctx, "[PricingSource] failed to cache pricing data: %v", err)
	}

	g.Log().Infof(ctx, "[PricingSource] loaded %d models from LiteLLM", len(raw))
	return raw, nil
}

// FindLiteLLMModel searches for a model in the LiteLLM pricing data.
// Tries exact match first, then strips common prefixes (provider/).
func FindLiteLLMModel(data map[string]*LiteLLMModelEntry, modelName string) (string, *LiteLLMModelEntry) {
	// Exact match
	if entry, ok := data[modelName]; ok {
		return modelName, entry
	}

	// Try common provider prefixes that LiteLLM uses
	providers := []string{"openai/", "anthropic/", "vertex_ai/", "bedrock/", "azure/", "deepseek/", "volcengine/"}
	for _, prefix := range providers {
		if entry, ok := data[prefix+modelName]; ok {
			return prefix + modelName, entry
		}
	}

	// Try stripping our custom prefixes (in case model name has a provider prefix)
	for _, prefix := range providers {
		if strings.HasPrefix(modelName, prefix) {
			stripped := strings.TrimPrefix(modelName, prefix)
			if entry, ok := data[stripped]; ok {
				return stripped, entry
			}
		}
	}

	return "", nil
}

// ---------- models.dev 数据源 ----------

const (
	modelsDevURL      = "https://models.dev/api.json"
	modelsDevCacheKey = "models_dev_pricing_data"
	modelsDevCacheTTL = 24 * time.Hour
)

// ModelsDevProvider represents a provider entry in the models.dev JSON.
type ModelsDevProvider struct {
	Models map[string]ModelsDevModel `json:"models"`
}

// ModelsDevModel represents a model entry with cost info.
type ModelsDevModel struct {
	Cost ModelsDevCost `json:"cost"`
}

// ModelsDevCost represents pricing in USD per 1M tokens.
type ModelsDevCost struct {
	Input     *float64 `json:"input"`
	Output    *float64 `json:"output"`
	CacheRead *float64 `json:"cache_read"`
}

// ModelsDevModelEntry is a flattened, validated model entry used for lookup.
type ModelsDevModelEntry struct {
	Provider  string
	Input     float64  // USD/1M tokens, 0 = free
	Output    *float64 // USD/1M tokens, nil = not available
	CacheRead *float64 // USD/1M tokens, nil = not available
}

var modelsDevCache = gcache.New()

// FetchModelsDevPricing fetches and caches the models.dev pricing data.
// Returns a map of model_name → ModelsDevModelEntry (cheapest non-zero input across providers).
func FetchModelsDevPricing(ctx context.Context) (map[string]*ModelsDevModelEntry, error) {
	cached, err := modelsDevCache.Get(ctx, modelsDevCacheKey)
	if err == nil && cached != nil {
		if data, ok := cached.Val().(map[string]*ModelsDevModelEntry); ok {
			return data, nil
		}
	}

	g.Log().Info(ctx, "[PricingSource] fetching models.dev pricing data from remote...")

	resp, err := g.Client().SetHeaderMap(map[string]string{
		"User-Agent": "github.com/qianfree/team-api/1.0",
	}).Get(ctx, modelsDevURL)
	if err != nil {
		return nil, fmt.Errorf("获取远程数据失败: %w", err)
	}
	defer resp.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取远程数据失败: HTTP %d", resp.StatusCode)
	}

	body := resp.ReadAll()
	if len(body) == 0 {
		return nil, fmt.Errorf("获取远程数据失败: 响应为空")
	}

	var rawProviders map[string]ModelsDevProvider
	if err := json.Unmarshal(body, &rawProviders); err != nil {
		return nil, fmt.Errorf("获取远程数据失败: JSON 解析失败: %w", err)
	}

	// Build map: model_name → cheapest ModelsDevModelEntry across providers
	result := make(map[string]*ModelsDevModelEntry, 256)
	for providerName, provider := range rawProviders {
		for modelName, model := range provider.Models {
			if model.Cost.Input == nil {
				continue
			}
			input := *model.Cost.Input
			if input < 0 || math.IsNaN(input) || math.IsInf(input, 0) {
				continue
			}

			var output *float64
			if model.Cost.Output != nil && *model.Cost.Output >= 0 {
				output = model.Cost.Output
			}

			// input=0 but output>0 cannot derive ratios
			if input == 0 && output != nil && *output > 0 {
				continue
			}

			var cacheRead *float64
			if model.Cost.CacheRead != nil && *model.Cost.CacheRead >= 0 {
				cacheRead = model.Cost.CacheRead
			}

			candidate := &ModelsDevModelEntry{
				Provider:  providerName,
				Input:     input,
				Output:    output,
				CacheRead: cacheRead,
			}

			existing, exists := result[modelName]
			if !exists || shouldReplaceModelsDevCandidate(existing, candidate) {
				result[modelName] = candidate
			}
		}
	}

	if err := modelsDevCache.Set(ctx, modelsDevCacheKey, result, modelsDevCacheTTL); err != nil {
		g.Log().Warningf(ctx, "[PricingSource] failed to cache models.dev data: %v", err)
	}

	g.Log().Infof(ctx, "[PricingSource] loaded %d models from models.dev", len(result))
	return result, nil
}

func shouldReplaceModelsDevCandidate(current, next *ModelsDevModelEntry) bool {
	currentNonZero := current.Input > 0
	nextNonZero := next.Input > 0
	if currentNonZero != nextNonZero {
		return nextNonZero
	}
	if nextNonZero && next.Input != current.Input {
		return next.Input < current.Input
	}
	return next.Provider < current.Provider
}

// FindModelsDevModel searches for a model in the models.dev pricing data.
// Tries exact match first, then common provider prefixes.
func FindModelsDevModel(data map[string]*ModelsDevModelEntry, modelName string) (string, *ModelsDevModelEntry) {
	// Exact match
	if entry, ok := data[modelName]; ok {
		return modelName, entry
	}

	// Try common provider prefixes
	prefixes := []string{"openai/", "anthropic/", "google/", "vertex_ai/", "bedrock/", "azure/"}
	for _, prefix := range prefixes {
		if entry, ok := data[prefix+modelName]; ok {
			return prefix + modelName, entry
		}
	}

	// Try stripping provider prefix
	for _, prefix := range prefixes {
		if strings.HasPrefix(modelName, prefix) {
			stripped := strings.TrimPrefix(modelName, prefix)
			if entry, ok := data[stripped]; ok {
				return stripped, entry
			}
		}
	}

	return "", nil
}

// ---------- BaseLLM 数据源（国内模型覆盖好） ----------

const (
	baseLLMURL      = "https://basellm.github.io/llm-metadata/api/newapi/models.json"
	baseLLMCacheKey = "basellm_pricing_data"
	baseLLMCacheTTL = 24 * time.Hour
)

// BaseLLMModelEntry represents a single model entry in the BaseLLM data.
type BaseLLMModelEntry struct {
	ModelName           string   `json:"model_name"`
	VendorName          string   `json:"vendor_name"`
	Tags                string   `json:"tags"`
	Status              int      `json:"status"`
	PricePerMInput      float64  `json:"price_per_m_input"`
	PricePerMOutput     float64  `json:"price_per_m_output"`
	PricePerMCacheRead  *float64 `json:"price_per_m_cache_read"`
	PricePerMCacheWrite *float64 `json:"price_per_m_cache_write"`
}

var baseLLMCache = gcache.New()

// FetchBaseLLMPricing fetches and caches the BaseLLM pricing data.
func FetchBaseLLMPricing(ctx context.Context) (map[string]*BaseLLMModelEntry, error) {
	cached, err := baseLLMCache.Get(ctx, baseLLMCacheKey)
	if err == nil && cached != nil {
		if data, ok := cached.Val().(map[string]*BaseLLMModelEntry); ok {
			return data, nil
		}
	}

	g.Log().Info(ctx, "[PricingSource] fetching BaseLLM pricing data from remote...")

	resp, err := g.Client().SetHeaderMap(map[string]string{
		"User-Agent": "github.com/qianfree/team-api/1.0",
	}).Get(ctx, baseLLMURL)
	if err != nil {
		return nil, fmt.Errorf("获取远程数据失败: %w", err)
	}
	defer resp.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取远程数据失败: HTTP %d", resp.StatusCode)
	}

	body := resp.ReadAll()
	if len(body) == 0 {
		return nil, fmt.Errorf("获取远程数据失败: 响应为空")
	}

	var raw struct {
		Data []*BaseLLMModelEntry `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("获取远程数据失败: JSON 解析失败: %w", err)
	}

	result := make(map[string]*BaseLLMModelEntry, len(raw.Data))
	for _, entry := range raw.Data {
		if entry.ModelName == "" || entry.Status != 1 {
			continue
		}
		name := strings.ToLower(entry.ModelName)
		if _, exists := result[name]; !exists {
			result[name] = entry
		}
	}

	if err := baseLLMCache.Set(ctx, baseLLMCacheKey, result, baseLLMCacheTTL); err != nil {
		g.Log().Warningf(ctx, "[PricingSource] failed to cache BaseLLM data: %v", err)
	}

	g.Log().Infof(ctx, "[PricingSource] loaded %d models from BaseLLM", len(result))
	return result, nil
}

// FindBaseLLMModel searches for a model in the BaseLLM pricing data.
func FindBaseLLMModel(data map[string]*BaseLLMModelEntry, modelName string) (string, *BaseLLMModelEntry) {
	name := strings.ToLower(modelName)
	if entry, ok := data[name]; ok {
		return name, entry
	}
	return "", nil
}

// ParseBaseLLMContext extracts context window size from BaseLLM tags (e.g. "128K" → 128000, "1M" → 1000000).
func ParseBaseLLMContext(tags string) int {
	for _, tag := range strings.Split(tags, ",") {
		tag = strings.TrimSpace(tag)
		if len(tag) < 2 {
			continue
		}
		suffix := tag[len(tag)-1:]
		numStr := tag[:len(tag)-1]
		switch suffix {
		case "K":
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * 1000)
			}
		case "M":
			if num, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int(num * 1_000_000)
			}
		}
	}
	return 0
}

// ParseBaseLLMCapabilities extracts capability map from BaseLLM tags.
func ParseBaseLLMCapabilities(tags string) map[string]bool {
	caps := make(map[string]bool)
	for _, tag := range strings.Split(tags, ",") {
		switch strings.TrimSpace(tag) {
		case "Vision":
			caps["vision"] = true
		case "Tools":
			caps["function_calling"] = true
			caps["tool_choice"] = true
		case "Reasoning":
			caps["reasoning"] = true
		case "Audio":
			caps["audio_input"] = true
			caps["audio_output"] = true
		case "Files":
			caps["pdf_input"] = true
		}
	}
	return caps
}

// ---------- OpenRouter 数据源 ----------

const (
	openRouterURL      = "https://openrouter.ai/api/v1/models"
	openRouterCacheKey = "openrouter_models_data"
	openRouterCacheTTL = 24 * time.Hour
)

// OpenRouterModelEntry represents a single model entry in the OpenRouter API.
type OpenRouterModelEntry struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	ContextLength   int                     `json:"context_length"`
	Architecture    *OpenRouterArchitecture `json:"architecture"`
	Pricing         *OpenRouterPricing      `json:"pricing"`
	TopProvider     *OpenRouterTopProvider  `json:"top_provider"`
	SupportedParams []string                `json:"supported_parameters"`
}

// OpenRouterArchitecture describes model input/output modalities.
type OpenRouterArchitecture struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// OpenRouterPricing holds per-token pricing (USD/token, string format).
type OpenRouterPricing struct {
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"input_cache_read"`
	InputCacheWrite string `json:"input_cache_write"`
}

// OpenRouterTopProvider holds provider-level limits.
type OpenRouterTopProvider struct {
	ContextLength       int `json:"context_length"`
	MaxCompletionTokens int `json:"max_completion_tokens"`
}

var openRouterCache = gcache.New()

// FetchOpenRouterModels fetches and caches the OpenRouter model catalog.
func FetchOpenRouterModels(ctx context.Context) (map[string]*OpenRouterModelEntry, error) {
	cached, err := openRouterCache.Get(ctx, openRouterCacheKey)
	if err == nil && cached != nil {
		if data, ok := cached.Val().(map[string]*OpenRouterModelEntry); ok {
			return data, nil
		}
	}

	g.Log().Info(ctx, "[PricingSource] fetching OpenRouter model data from remote...")

	resp, err := g.Client().SetHeaderMap(map[string]string{
		"User-Agent": "github.com/qianfree/team-api/1.0",
	}).Get(ctx, openRouterURL)
	if err != nil {
		return nil, fmt.Errorf("获取远程数据失败: %w", err)
	}
	defer resp.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取远程数据失败: HTTP %d", resp.StatusCode)
	}

	body := resp.ReadAll()
	if len(body) == 0 {
		return nil, fmt.Errorf("获取远程数据失败: 响应为空")
	}

	var raw struct {
		Data []*OpenRouterModelEntry `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("获取远程数据失败: JSON 解析失败: %w", err)
	}

	// 同时以完整 ID 和裸模型名作为 key，方便匹配
	result := make(map[string]*OpenRouterModelEntry, len(raw.Data)*2)
	for _, entry := range raw.Data {
		if entry.ID == "" {
			continue
		}
		result[entry.ID] = entry
		if idx := strings.LastIndex(entry.ID, "/"); idx >= 0 {
			bareName := entry.ID[idx+1:]
			if _, exists := result[bareName]; !exists {
				result[bareName] = entry
			}
		}
	}

	if err := openRouterCache.Set(ctx, openRouterCacheKey, result, openRouterCacheTTL); err != nil {
		g.Log().Warningf(ctx, "[PricingSource] failed to cache OpenRouter data: %v", err)
	}

	g.Log().Infof(ctx, "[PricingSource] loaded %d models from OpenRouter", len(raw.Data))
	return result, nil
}

// FindOpenRouterModel searches for a model in the OpenRouter data.
func FindOpenRouterModel(data map[string]*OpenRouterModelEntry, modelName string) (string, *OpenRouterModelEntry) {
	if entry, ok := data[modelName]; ok {
		return modelName, entry
	}

	providers := []string{
		"openai/", "anthropic/", "google/", "meta-llama/", "mistralai/",
		"deepseek/", "qwen/", "zhipu/", "moonshot/", "minimax/",
		"baichuan/", "01-ai/", "stepfun/", "volcengine/",
	}
	for _, prefix := range providers {
		if entry, ok := data[prefix+modelName]; ok {
			return prefix + modelName, entry
		}
	}

	for _, prefix := range providers {
		if strings.HasPrefix(modelName, prefix) {
			stripped := strings.TrimPrefix(modelName, prefix)
			if entry, ok := data[stripped]; ok {
				return stripped, entry
			}
		}
	}

	return "", nil
}

// ParseOpenRouterCapabilities extracts capability map from OpenRouter model entry.
func ParseOpenRouterCapabilities(entry *OpenRouterModelEntry) map[string]bool {
	caps := make(map[string]bool)

	paramSet := make(map[string]bool, len(entry.SupportedParams))
	for _, p := range entry.SupportedParams {
		paramSet[p] = true
	}
	if paramSet["tools"] || paramSet["tool_choice"] {
		caps["function_calling"] = true
		caps["tool_choice"] = true
	}
	if paramSet["reasoning"] || paramSet["include_reasoning"] {
		caps["reasoning"] = true
	}
	if paramSet["response_format"] || paramSet["structured_outputs"] {
		caps["response_schema"] = true
	}

	if entry.Architecture != nil {
		for _, m := range entry.Architecture.InputModalities {
			switch m {
			case "image":
				caps["vision"] = true
			case "audio":
				caps["audio_input"] = true
			case "file":
				caps["pdf_input"] = true
			}
		}
		for _, m := range entry.Architecture.OutputModalities {
			if m == "audio" {
				caps["audio_output"] = true
			}
		}
	}

	return caps
}

// OpenRouterPricePerM converts an OpenRouter per-token price string to USD/1M tokens.
func OpenRouterPricePerM(perToken string) float64 {
	if perToken == "" {
		return 0
	}
	v, err := strconv.ParseFloat(perToken, 64)
	if err != nil {
		return 0
	}
	return v * 1_000_000
}
