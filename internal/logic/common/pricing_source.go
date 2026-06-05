package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	providers := []string{"openai/", "anthropic/", "vertex_ai/", "bedrock/", "azure/"}
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
