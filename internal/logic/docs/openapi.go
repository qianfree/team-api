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

	spec := buildOpenAPISpec(ctx)
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	// Cache for 1 hour
	_, _ = g.Redis().Do(ctx, "SET", "docs:openapi:spec", string(raw), "EX", 3600)

	return raw, nil
}

func buildOpenAPISpec(ctx context.Context) map[string]any {
	baseURL := common.Config().GetString(ctx, "api_base_url")
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
			"/v1/images/generations/async": map[string]any{
				"post": map[string]any{
					"summary":     "Image Generations Async (Submit)",
					"description": "异步图像生成任务提交。阿里云百炼（DashScope）等异步图像上游无法经同步接口一次性返回，需提交任务拿到 task_id 后轮询 /v1/images/generations/async/{task_id} 取图。",
					"operationId": "createImageAsync",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":     "object",
									"required": []string{"model", "prompt"},
									"properties": map[string]any{
										"model":  map[string]any{"type": "string", "example": "wanx2.1-t2i-turbo"},
										"prompt": map[string]any{"type": "string"},
										"n":      map[string]any{"type": "integer", "default": 1},
										"size":   map[string]any{"type": "string", "example": "1024x1024"},
									},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "任务已提交，返回 task_id 与初始状态"},
					},
				},
			},
			"/v1/images/generations/async/{task_id}": map[string]any{
				"get": map[string]any{
					"summary":     "Image Generations Async (Fetch)",
					"description": "查询异步图像生成任务结果。轮询直到 status 为 SUCCESS（含图片 url）或 FAILURE（含 error），建议间隔 2~3 秒。",
					"operationId": "getImageAsync",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"parameters": []map[string]any{
						{
							"name":     "task_id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "string"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "任务状态与结果"},
					},
				},
			},
			"/v1/videos": map[string]any{
				"post": map[string]any{
					"summary":     "Create Video",
					"description": "创建视频生成任务（OpenAI Videos 协议，兼容 openai-python / openai-node SDK 的 client.videos.create）。请求体支持 multipart/form-data（官方 SDK 形态，input_reference 可为文件 part）与 application/json 两种编码。返回官方 Video 对象（status: queued），轮询 GET /v1/videos/{video_id} 直到 completed，再经 GET /v1/videos/{video_id}/content 获取视频二进制。",
					"operationId": "createVideo",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"multipart/form-data": map[string]any{
								"schema": map[string]any{
									"type":     "object",
									"required": []string{"model", "prompt"},
									"properties": map[string]any{
										"model":           map[string]any{"type": "string", "example": "kling-v2-master"},
										"prompt":          map[string]any{"type": "string"},
										"seconds":         map[string]any{"type": "string", "enum": []string{"4", "8", "12"}, "default": "4"},
										"size":            map[string]any{"type": "string", "enum": []string{"720x1280", "1280x720", "1024x1792", "1792x1024"}, "default": "720x1280"},
										"aspect_ratio":    map[string]any{"type": "string", "description": "扩展字段：屏幕比例，如 16:9 / 9:16 / 1:1；取值原样透传上游，不传由模型取默认档"},
										"sound":           map[string]any{"type": "string", "enum": []string{"on", "off"}, "description": "扩展字段：是否包含声音；仅部分模型支持（可灵 / 豆包），其余模型忽略"},
										"input_reference": map[string]any{"type": "string", "format": "binary", "description": "可选参考图（图生视频），文件上传或 {\"image_url\": \"...\"} 引用对象；file_id 形态不支持"},
									},
								},
							},
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":     "object",
									"required": []string{"model", "prompt"},
									"properties": map[string]any{
										"model":           map[string]any{"type": "string"},
										"prompt":          map[string]any{"type": "string"},
										"seconds":         map[string]any{"type": "string"},
										"size":            map[string]any{"type": "string"},
										"aspect_ratio":    map[string]any{"type": "string", "description": "扩展字段：屏幕比例，如 16:9 / 9:16 / 1:1"},
										"sound":           map[string]any{"type": "string", "enum": []string{"on", "off"}, "description": "扩展字段：是否包含声音（也可用布尔形态 generate_audio）"},
										"generate_audio":  map[string]any{"type": "boolean", "description": "扩展字段：sound 的等价布尔形态，sound 缺省时生效"},
										"input_reference": map[string]any{"type": "object", "properties": map[string]any{"image_url": map[string]any{"type": "string"}}},
									},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Video 对象（object: video，status: queued）"},
					},
				},
			},
			"/v1/videos/{video_id}": map[string]any{
				"get": map[string]any{
					"summary":     "Retrieve Video",
					"description": "查询视频生成任务，返回官方 Video 对象。status: queued / in_progress / completed / failed；非终态响应携带 openai-poll-after-ms 轮询间隔建议头。生成失败以 status=failed + error 对象表达（HTTP 仍为 200）。",
					"operationId": "retrieveVideo",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"parameters": []map[string]any{
						{
							"name":     "video_id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "string"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "Video 对象（含 progress 百分比、completed_at、失败时的 error）"},
						"404": map[string]any{"description": "视频不存在或已被删除"},
					},
				},
				"delete": map[string]any{
					"summary":     "Delete Video",
					"description": "删除已完成或已失败的视频任务（软删除，计费与审计记录保留），删除后查询与下载均返回 404。进行中的任务不可删除。",
					"operationId": "deleteVideo",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"parameters": []map[string]any{
						{
							"name":     "video_id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "string"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "删除确认（object: video.deleted，deleted: true）"},
						"400": map[string]any{"description": "任务非终态或计费未结算，不可删除"},
						"404": map[string]any{"description": "视频不存在或已被删除"},
					},
				},
			},
			"/v1/videos/{video_id}/content": map[string]any{
				"get": map[string]any{
					"summary":     "Download Video Content",
					"description": "下载已生成完成的视频二进制流（mp4）。仅 status=completed 的任务可下载；variant 仅支持默认 video（thumbnail / spritesheet 暂不支持）。",
					"operationId": "downloadVideoContent",
					"security":    []map[string]any{{"BearerAuth": []any{}}},
					"parameters": []map[string]any{
						{
							"name":     "video_id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "string"},
						},
						{
							"name":     "variant",
							"in":       "query",
							"required": false,
							"schema":   map[string]any{"type": "string", "enum": []string{"video"}, "default": "video"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "视频二进制流（video/mp4）"},
						"400": map[string]any{"description": "任务未完成或无可用内容"},
						"404": map[string]any{"description": "视频不存在或已被删除"},
						"502": map[string]any{"description": "上游内容拉取失败"},
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
