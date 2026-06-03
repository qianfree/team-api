package docs

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/docs/v1"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/service"
)

func init() {
	service.RegisterDocs(New())
}

type sDocs struct{}

func New() *sDocs {
	return &sDocs{}
}

func (s *sDocs) OpenAPISpec(ctx context.Context, _ *v1.OpenAPISpecReq) (json.RawMessage, error) {
	// Try cache first
	cached, err := g.Redis().Do(ctx, "GET", "docs:openapi:spec")
	if err == nil && !cached.IsNil() && !cached.IsEmpty() {
		return json.RawMessage(cached.String()), nil
	}

	spec := buildOpenAPISpec()
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	// Cache for 1 hour
	_, _ = g.Redis().Do(ctx, "SET", "docs:openapi:spec", string(raw), "EX", 3600)

	return raw, nil
}

func buildOpenAPISpec() map[string]any {
	baseURL := common.Config().GetString(nil, "api_base_url")
	if baseURL == "" {
		baseURL = "https://api.team-api.example.com"
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Team API",
			"description": "多租户 大模型 API 网关，兼容 OpenAI API 格式",
			"version":     "1.0.0",
		},
		"servers": []map[string]any{
			{"url": baseURL, "description": "Production"},
		},
		"paths": map[string]any{
			"/v1/chat/completions": map[string]any{
				"post": map[string]any{
					"summary":     "Chat Completions",
					"description": "创建聊天补全，兼容 OpenAI API 格式，支持流式和非流式响应。",
					"operationId": "createChatCompletion",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/ChatCompletionRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Successful response",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/ChatCompletionResponse"},
								},
								"text/event-stream": map[string]any{
									"schema": map[string]any{"type": "string", "description": "SSE stream"},
								},
							},
						},
						"401": map[string]any{"description": "Authentication failed"},
						"402": map[string]any{"description": "Insufficient quota"},
						"429": map[string]any{"description": "Rate limit exceeded"},
						"500": map[string]any{"description": "Internal error"},
					},
				},
			},
			"/v1/completions": map[string]any{
				"post": map[string]any{
					"summary":     "Text Completions",
					"description": "文本补全，兼容 OpenAI API 格式。",
					"operationId": "createCompletion",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/CompletionRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
			"/v1/embeddings": map[string]any{
				"post": map[string]any{
					"summary":     "Embeddings",
					"description": "创建文本向量嵌入，兼容 OpenAI API 格式。",
					"operationId": "createEmbedding",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/EmbeddingRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
			"/v1/images/generations": map[string]any{
				"post": map[string]any{
					"summary":     "Image Generations",
					"description": "AI 图像生成，兼容 OpenAI API 格式。",
					"operationId": "createImage",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":     "object",
									"required": []string{"model", "prompt"},
									"properties": map[string]any{
										"model":  map[string]any{"type": "string", "example": "dall-e-3"},
										"prompt": map[string]any{"type": "string"},
										"n":      map[string]any{"type": "integer", "default": 1},
										"size":   map[string]any{"type": "string", "enum": []string{"256x256", "512x512", "1024x1024", "1792x1024", "1024x1792"}},
									},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
			"/v1/messages": map[string]any{
				"post": map[string]any{
					"summary":     "Claude Messages",
					"description": "Claude Messages API，兼容 Anthropic API 格式。",
					"operationId": "createClaudeMessage",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":     "object",
									"required": []string{"model", "messages"},
									"properties": map[string]any{
										"model":      map[string]any{"type": "string", "example": "claude-sonnet-4-20250514"},
										"messages":   map[string]any{"type": "array"},
										"max_tokens": map[string]any{"type": "integer", "default": 4096},
										"stream":     map[string]any{"type": "boolean", "default": false},
									},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
			"/v1/models": map[string]any{
				"get": map[string]any{
					"summary":     "List Models",
					"description": "获取可用模型列表。",
					"operationId": "listModels",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
			"/v1/models/{model_id}": map[string]any{
				"get": map[string]any{
					"summary":     "Get Model",
					"description": "获取模型详情。",
					"operationId": "getModel",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"parameters": []map[string]any{
						{"name": "model_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Successful response"},
					},
				},
			},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "API Key",
				},
			},
			"schemas": map[string]any{
				"ChatCompletionRequest": map[string]any{
					"type":     "object",
					"required": []string{"model", "messages"},
					"properties": map[string]any{
						"model": map[string]any{"type": "string", "example": "gpt-4o", "description": "模型 ID"},
						"messages": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/ChatMessage"},
						},
						"temperature":       map[string]any{"type": "number", "minimum": 0, "maximum": 2, "default": 1},
						"top_p":             map[string]any{"type": "number", "minimum": 0, "maximum": 1, "default": 1},
						"max_tokens":        map[string]any{"type": "integer", "minimum": 1},
						"stream":            map[string]any{"type": "boolean", "default": false},
						"frequency_penalty": map[string]any{"type": "number", "minimum": -2, "maximum": 2},
						"presence_penalty":  map[string]any{"type": "number", "minimum": -2, "maximum": 2},
						"stop":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
				},
				"ChatMessage": map[string]any{
					"type":     "object",
					"required": []string{"role", "content"},
					"properties": map[string]any{
						"role":    map[string]any{"type": "string", "enum": []string{"system", "user", "assistant"}},
						"content": map[string]any{"type": "string"},
					},
				},
				"ChatCompletionResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":      map[string]any{"type": "string"},
						"object":  map[string]any{"type": "string"},
						"created": map[string]any{"type": "integer"},
						"model":   map[string]any{"type": "string"},
						"choices": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/ChatChoice"},
						},
						"usage": map[string]any{"$ref": "#/components/schemas/Usage"},
					},
				},
				"ChatChoice": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"index":         map[string]any{"type": "integer"},
						"message":       map[string]any{"$ref": "#/components/schemas/ChatMessage"},
						"finish_reason": map[string]any{"type": "string"},
					},
				},
				"Usage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"prompt_tokens":     map[string]any{"type": "integer"},
						"completion_tokens": map[string]any{"type": "integer"},
						"total_tokens":      map[string]any{"type": "integer"},
					},
				},
				"CompletionRequest": map[string]any{
					"type":     "object",
					"required": []string{"model", "prompt"},
					"properties": map[string]any{
						"model":      map[string]any{"type": "string", "example": "gpt-4o"},
						"prompt":     map[string]any{"type": "string"},
						"max_tokens": map[string]any{"type": "integer", "default": 16},
						"stream":     map[string]any{"type": "boolean", "default": false},
					},
				},
				"EmbeddingRequest": map[string]any{
					"type":     "object",
					"required": []string{"model", "input"},
					"properties": map[string]any{
						"model": map[string]any{"type": "string", "example": "text-embedding-3-small"},
						"input": map[string]any{"oneOf": []any{
							map[string]any{"type": "string"},
							map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						}},
					},
				},
			},
		},
	}
}
