<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'

const router = useRouter()
const activeSection = ref('quickstart')
const activeEndpoint = ref('')

const sections = [
	{ id: 'quickstart', label: '快速入门' },
	{ id: 'api-reference', label: 'API 参考' },
	{ id: 'code-examples', label: '代码示例' },
	{ id: 'error-codes', label: '错误码' },
]

// ============================================================
// Quick Start Examples
// ============================================================
const curlExample = `curl -X POST https://api.team-api.example.com/v1/chat/completions \\
  -H "Authorization: Bearer sk-your-api-key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello, world!"}
    ],
    "stream": true
  }'`

const pythonExample = `import openai

client = openai.OpenAI(
    api_key="sk-your-api-key",
    base_url="https://api.team-api.example.com/v1"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello, world!"}
    ],
    stream=True
)

for chunk in response:
    print(chunk.choices[0].delta.content or "", end="")`

const nodeExample = `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-your-api-key",
  baseURL: "https://api.team-api.example.com/v1",
});

const stream = await client.chat.completions.create({
  model: "gpt-4o",
  messages: [
    { role: "system", content: "You are a helpful assistant." },
    { role: "user", content: "Hello, world!" },
  ],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content || "");
}`

const goExample = `package main

import (
    "context"
    "fmt"
    "io"

    "github.com/sashabaranov/go-openai"
)

func main() {
    config := openai.DefaultConfig("sk-your-api-key")
    config.BaseURL = "https://api.team-api.example.com/v1"

    client := openai.NewClientWithConfig(config)

    stream, _ := client.CreateChatCompletionStream(context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4o,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
                {Role: openai.ChatMessageRoleUser, Content: "Hello, world!"},
            },
        },
    )
    defer stream.Close()

    for {
        resp, err := stream.Recv()
        if err == io.EOF {
            break
        }
        fmt.Print(resp.Choices[0].Delta.Content)
    }
}`

type CodeTab = 'curl' | 'python' | 'node' | 'go'
const activeCodeTab = ref<CodeTab>('curl')

const quickStartExamples: Record<CodeTab, string> = {
	curl: curlExample,
	python: pythonExample,
	node: nodeExample,
	go: goExample,
}

function getExampleCode(tab: CodeTab): string {
	return quickStartExamples[tab]
}

const tabLabels: Record<CodeTab, string> = {
	curl: 'cURL',
	python: 'Python',
	node: 'Node.js',
	go: 'Go',
}

const copiedTab = ref('')
async function copyText(text: string, key: string) {
	try {
		await navigator.clipboard.writeText(text)
		copiedTab.value = key
		setTimeout(() => { copiedTab.value = '' }, 2000)
	} catch (e) { console.error(e) }
}

function goToPlayground() {
	router.push('/tenant/playground')
}

// ============================================================
// API Reference — Endpoint Categories & Data
// ============================================================
type Param = {
	field: string
	type: string
	desc: string
	required?: boolean
}

type Endpoint = {
	id: string
	method: string
	path: string
	title: string
	desc: string
	params: Param[]
	example?: { req?: string; resp?: string }
}

type Category = {
	id: string
	label: string
	icon: string
	endpoints: Endpoint[]
}

const categories: Category[] = [
	{
		id: 'chat',
		label: '对话补全',
		icon: 'messageSquare',
		endpoints: [
			{
				id: 'chat-completions',
				method: 'POST',
				path: '/v1/chat/completions',
				title: 'Chat Completions',
				desc: '创建聊天补全，兼容 OpenAI API 格式。支持流式和非流式响应，支持工具调用、函数调用、JSON 模式等高级功能。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 gpt-4o, claude-sonnet-4-20250514）', required: true },
					{ field: 'messages', type: 'array', desc: '消息数组，每项包含 role（system/user/assistant/tool）和 content', required: true },
					{ field: 'stream', type: 'boolean', desc: '是否启用流式响应（默认 false）' },
					{ field: 'temperature', type: 'number', desc: '采样温度 0-2（默认 1）' },
					{ field: 'max_tokens', type: 'integer', desc: '最大生成 Token 数' },
					{ field: 'max_completion_tokens', type: 'integer', desc: '最大补全 Token 数（新参数，优先于 max_tokens）' },
					{ field: 'top_p', type: 'number', desc: '核采样概率（默认 1）' },
					{ field: 'top_k', type: 'integer', desc: 'Top-K 采样参数' },
					{ field: 'n', type: 'integer', desc: '生成候选数量（默认 1）' },
					{ field: 'frequency_penalty', type: 'number', desc: '频率惩罚 -2 ~ 2' },
					{ field: 'presence_penalty', type: 'number', desc: '存在惩罚 -2 ~ 2' },
					{ field: 'stop', type: 'string | array', desc: '停止生成的字符串序列' },
					{ field: 'tools', type: 'array', desc: '可用工具列表（函数调用定义）' },
					{ field: 'tool_choice', type: 'string | object', desc: '工具选择策略（auto/none/required 或指定函数）' },
					{ field: 'response_format', type: 'object', desc: '响应格式（如 {"type": "json_object"}）' },
					{ field: 'seed', type: 'integer', desc: '随机种子，用于可复现输出' },
					{ field: 'user', type: 'string', desc: '用户标识，用于监控和限流' },
				],
				example: {
					req: `{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Explain quantum computing in one sentence."}
  ],
  "temperature": 0.7,
  "max_tokens": 256
}`,
					resp: `{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1717100000,
  "model": "gpt-4o",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Quantum computing harnesses quantum mechanical phenomena..."
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 22,
    "completion_tokens": 36,
    "total_tokens": 58
  }
}`,
				},
			},
			{
				id: 'completions',
				method: 'POST',
				path: '/v1/completions',
				title: 'Completions',
				desc: '文本补全接口，给定提示词续写文本。兼容 OpenAI Completions API 格式。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID', required: true },
					{ field: 'prompt', type: 'string | array', desc: '提示词文本', required: true },
					{ field: 'max_tokens', type: 'integer', desc: '最大生成 Token 数' },
					{ field: 'temperature', type: 'number', desc: '采样温度 0-2（默认 1）' },
					{ field: 'top_p', type: 'number', desc: '核采样概率（默认 1）' },
					{ field: 'n', type: 'integer', desc: '生成候选数量（默认 1）' },
					{ field: 'stream', type: 'boolean', desc: '是否启用流式响应' },
					{ field: 'stop', type: 'string | array', desc: '停止生成的字符串序列' },
					{ field: 'suffix', type: 'string', desc: '补全后缀文本' },
					{ field: 'echo', type: 'boolean', desc: '是否回显提示词' },
					{ field: 'frequency_penalty', type: 'number', desc: '频率惩罚 -2 ~ 2' },
					{ field: 'presence_penalty', type: 'number', desc: '存在惩罚 -2 ~ 2' },
					{ field: 'user', type: 'string', desc: '用户标识' },
				],
				example: {
					req: `{
  "model": "gpt-4o",
  "prompt": "Once upon a time in a galaxy far, far away",
  "max_tokens": 64,
  "temperature": 0.8
}`,
				},
			},
		],
	},
	{
		id: 'claude',
		label: 'Claude 兼容',
		icon: 'sparkles',
		endpoints: [
			{
				id: 'claude-messages',
				method: 'POST',
				path: '/v1/messages',
				title: 'Claude Messages',
				desc: 'Anthropic Claude Messages API 兼容接口。支持扩展思维（Extended Thinking）、工具调用、流式响应等功能。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 claude-sonnet-4-20250514）', required: true },
					{ field: 'messages', type: 'array', desc: '消息数组，Claude 格式', required: true },
					{ field: 'system', type: 'string | array', desc: '系统提示词（字符串或内容块数组）' },
					{ field: 'max_tokens', type: 'integer', desc: '最大 Token 数（默认 4096）' },
					{ field: 'stream', type: 'boolean', desc: '是否启用流式响应' },
					{ field: 'temperature', type: 'number', desc: '采样温度 0-1' },
					{ field: 'top_p', type: 'number', desc: '核采样概率' },
					{ field: 'top_k', type: 'integer', desc: 'Top-K 采样参数' },
					{ field: 'stop_sequences', type: 'array', desc: '停止序列列表' },
					{ field: 'thinking', type: 'object', desc: '扩展思维配置，如 {"type":"enabled","budget_tokens":10000}' },
					{ field: 'tools', type: 'array', desc: '工具定义列表' },
					{ field: 'tool_choice', type: 'object', desc: '工具选择策略' },
				],
				example: {
					req: `{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Explain quantum entanglement."}
  ]
}`,
					resp: `{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "Quantum entanglement is a phenomenon..."}
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 20,
    "output_tokens": 120
  }
}`,
				},
			},
		],
	},
	{
		id: 'responses',
		label: 'OpenAI Responses',
		icon: 'zap',
		endpoints: [
			{
				id: 'openai-responses',
				method: 'POST',
				path: '/v1/responses',
				title: 'Create Response',
				desc: 'OpenAI Responses API 兼容接口。支持多轮对话管理、推理配置、工具调用和流式响应。可替代 Chat Completions 用于更复杂的对话场景。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID', required: true },
					{ field: 'input', type: 'string | array', desc: '输入内容（文本或消息数组）' },
					{ field: 'instructions', type: 'string', desc: '系统级指令（替代 system message）' },
					{ field: 'max_output_tokens', type: 'integer', desc: '最大输出 Token 数' },
					{ field: 'temperature', type: 'number', desc: '采样温度' },
					{ field: 'top_p', type: 'number', desc: '核采样概率' },
					{ field: 'stream', type: 'boolean', desc: '是否启用流式响应' },
					{ field: 'tools', type: 'array', desc: '工具定义列表' },
					{ field: 'tool_choice', type: 'string | object', desc: '工具选择策略' },
					{ field: 'reasoning', type: 'object', desc: '推理配置，如 {"effort":"high","summary":"auto"}' },
					{ field: 'previous_response_id', type: 'string', desc: '关联上一次响应 ID，用于多轮对话' },
					{ field: 'store', type: 'boolean', desc: '是否存储响应以供后续引用' },
					{ field: 'user', type: 'string', desc: '用户标识' },
				],
				example: {
					req: `{
  "model": "gpt-4o",
  "input": "What is the capital of France?",
  "instructions": "Answer concisely.",
  "max_output_tokens": 128,
  "temperature": 0.5
}`,
				},
			},
			{
				id: 'openai-responses-compact',
				method: 'POST',
				path: '/v1/responses/compact',
				title: 'Create Response (Compact)',
				desc: 'Compact 版本的 Responses API，返回精简格式的响应。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID', required: true },
					{ field: 'input', type: 'string | array', desc: '输入内容' },
				],
			},
		],
	},
	{
		id: 'embeddings',
		label: '向量嵌入',
		icon: 'link',
		endpoints: [
			{
				id: 'embeddings',
				method: 'POST',
				path: '/v1/embeddings',
				title: 'Embeddings',
				desc: '创建文本向量嵌入。将文本转换为高维向量，用于语义搜索、文本分类、聚类等场景。兼容 OpenAI Embeddings API 格式。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 text-embedding-3-small）', required: true },
					{ field: 'input', type: 'string | array', desc: '要嵌入的文本或文本数组（最多 2048 个）', required: true },
					{ field: 'encoding_format', type: 'string', desc: '编码格式（float / base64）' },
					{ field: 'dimensions', type: 'integer', desc: '输出向量维度（如 512、1536）' },
					{ field: 'user', type: 'string', desc: '用户标识' },
				],
				example: {
					req: `{
  "model": "text-embedding-3-small",
  "input": "The quick brown fox jumps over the lazy dog"
}`,
					resp: `{
  "object": "list",
  "data": [{
    "object": "embedding",
    "index": 0,
    "embedding": [0.0023, -0.0094, 0.0151, ...]
  }],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}`,
				},
			},
		],
	},
	{
		id: 'images',
		label: '图像生成',
		icon: 'image',
		endpoints: [
			{
				id: 'images-generations',
				method: 'POST',
				path: '/v1/images/generations',
				title: 'Image Generations',
				desc: 'AI 图像生成。根据文本描述生成图像，支持多种尺寸、风格和输出格式。兼容 OpenAI Images API 格式。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 dall-e-3）', required: true },
					{ field: 'prompt', type: 'string', desc: '图像描述提示词', required: true },
					{ field: 'n', type: 'integer', desc: '生成数量（默认 1）' },
					{ field: 'size', type: 'string', desc: '图像尺寸（如 1024x1024、1792x1024）' },
					{ field: 'quality', type: 'string', desc: '图像质量（standard / hd）' },
					{ field: 'response_format', type: 'string', desc: '返回格式（url / b64_json）' },
					{ field: 'style', type: 'string', desc: '风格（vivid / natural）' },
					{ field: 'output_format', type: 'string', desc: '输出格式（png / jpg / webp）' },
					{ field: 'user', type: 'string', desc: '用户标识' },
				],
				example: {
					req: `{
  "model": "dall-e-3",
  "prompt": "A white siamese cat wearing sunglasses",
  "n": 1,
  "size": "1024x1024",
  "quality": "hd"
}`,
					resp: `{
  "created": 1717100000,
  "data": [{
    "url": "https://cdn.example.com/img/abc123.png",
    "revised_prompt": "A siamese cat with striking blue eyes..."
  }]
}`,
				},
			},
			{
				id: 'images-edits',
				method: 'POST',
				path: '/v1/images/edits',
				title: 'Image Edits',
				desc: '图像编辑接口。基于源图像和文本描述进行编辑或局部修改（inpainting），支持蒙版遮罩。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID', required: true },
					{ field: 'image', type: 'file | string', desc: '源图像（文件上传或 Base64）', required: true },
					{ field: 'prompt', type: 'string', desc: '编辑描述', required: true },
					{ field: 'mask', type: 'file | string', desc: '蒙版图像（透明区域为编辑区域）' },
					{ field: 'n', type: 'integer', desc: '生成数量（默认 1）' },
					{ field: 'size', type: 'string', desc: '输出尺寸' },
					{ field: 'response_format', type: 'string', desc: '返回格式（url / b64_json）' },
					{ field: 'user', type: 'string', desc: '用户标识' },
				],
				example: {
					req: `// multipart/form-data 上传
// image: <file binary>
// mask: <file binary>
{
  "model": "dall-e-2",
  "prompt": "A sunlit indoor lounge area with a pool",
  "n": 1,
  "size": "1024x1024"
}`,
				},
			},
		],
	},
	{
		id: 'audio',
		label: '语音接口',
		icon: 'mic',
		endpoints: [
			{
				id: 'audio-speech',
				method: 'POST',
				path: '/v1/audio/speech',
				title: 'Text-to-Speech (TTS)',
				desc: '文本转语音。将文本转换为自然语音音频，支持多种声音和输出格式。兼容 OpenAI Audio Speech API。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 tts-1）', required: true },
					{ field: 'input', type: 'string', desc: '要转换的文本内容', required: true },
					{ field: 'voice', type: 'string', desc: '声音类型（alloy / echo / fable / onyx / nova / shimmer）', required: true },
					{ field: 'response_format', type: 'string', desc: '输出格式（mp3 / opus / aac / flac / wav / pcm）' },
					{ field: 'speed', type: 'number', desc: '语速（0.25-4.0，默认 1.0）' },
					{ field: 'instructions', type: 'string', desc: '语音生成指令（如语气、情感描述）' },
				],
				example: {
					req: `{
  "model": "tts-1",
  "input": "The quick brown fox jumps over the lazy dog.",
  "voice": "alloy",
  "response_format": "mp3",
  "speed": 1.0
}`,
				},
			},
			{
				id: 'audio-transcriptions',
				method: 'POST',
				path: '/v1/audio/transcriptions',
				title: 'Speech-to-Text (STT)',
				desc: '语音转文本（转录）。将音频文件转录为文本，支持多种音频格式和语言。兼容 OpenAI Audio Transcriptions API。',
				params: [
					{ field: 'file', type: 'file', desc: '音频文件（mp3/mp4/wav/m4a/webm 等）', required: true },
					{ field: 'model', type: 'string', desc: '模型 ID（如 whisper-1）', required: true },
					{ field: 'language', type: 'string', desc: '音频语言（ISO 639-1 如 zh、en）' },
					{ field: 'prompt', type: 'string', desc: '转录提示词（帮助识别专有名词等）' },
					{ field: 'response_format', type: 'string', desc: '输出格式（json / text / srt / vtt）' },
					{ field: 'timestamp_granularities', type: 'array', desc: '时间戳粒度（word / segment）' },
				],
				example: {
					req: `// multipart/form-data 上传
// file: <audio file binary>
{
  "model": "whisper-1",
  "language": "zh",
  "response_format": "json"
}`,
					resp: `{
  "text": "这是一段语音转文字的测试音频。"
}`,
				},
			},
			{
				id: 'audio-translations',
				method: 'POST',
				path: '/v1/audio/translations',
				title: 'Audio Translations',
				desc: '音频翻译。将任意语言的音频翻译为英文文本。兼容 OpenAI Audio Translations API。',
				params: [
					{ field: 'file', type: 'file', desc: '音频文件', required: true },
					{ field: 'model', type: 'string', desc: '模型 ID（如 whisper-1）', required: true },
					{ field: 'prompt', type: 'string', desc: '翻译提示词' },
					{ field: 'response_format', type: 'string', desc: '输出格式（json / text / srt / vtt）' },
				],
				example: {
					req: `// multipart/form-data 上传
// file: <audio file binary>
{
  "model": "whisper-1",
  "response_format": "json"
}`,
					resp: `{
  "text": "This is a test audio for translation."
}`,
				},
			},
		],
	},
	{
		id: 'rerank',
		label: '重排序',
		icon: 'listOrdered',
		endpoints: [
			{
				id: 'rerank',
				method: 'POST',
				path: '/v1/rerank',
				title: 'Rerank',
				desc: '文档重排序。根据查询文本对一组文档进行相关性排序，广泛用于 RAG（检索增强生成）场景中的检索结果优化。兼容 Cohere/Jina Rerank API 格式。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 jina-reranker-v2-base-multilingual）', required: true },
					{ field: 'query', type: 'string', desc: '查询文本', required: true },
					{ field: 'documents', type: 'array', desc: '待排序的文档列表（字符串数组）', required: true },
					{ field: 'top_n', type: 'integer', desc: '返回前 N 个结果' },
					{ field: 'return_documents', type: 'boolean', desc: '是否在响应中返回文档原文' },
					{ field: 'max_chunks_per_doc', type: 'integer', desc: '每个文档最大分块数' },
				],
				example: {
					req: `{
  "model": "jina-reranker-v2-base-multilingual",
  "query": "什么是量子计算？",
  "documents": [
    "量子计算是一种利用量子力学原理进行计算的技术...",
    "机器学习是人工智能的一个分支...",
    "量子比特是量子计算的基本单位..."
  ],
  "top_n": 3,
  "return_documents": true
}`,
					resp: `{
  "model": "jina-reranker-v2-base-multilingual",
  "results": [
    {"index": 0, "relevance_score": 0.95, "document": {"text": "量子计算是一种利用量子力学原理进行计算的技术..."}},
    {"index": 2, "relevance_score": 0.87, "document": {"text": "量子比特是量子计算的基本单位..."}},
    {"index": 1, "relevance_score": 0.12, "document": {"text": "机器学习是人工智能的一个分支..."}}
  ]
}`,
				},
			},
		],
	},
	{
		id: 'moderation',
		label: '内容审核',
		icon: 'shield',
		endpoints: [
			{
				id: 'moderations',
				method: 'POST',
				path: '/v1/moderations',
				title: 'Moderations',
				desc: '内容审核。检查文本是否包含违规内容（仇恨、暴力、色情等），返回各维度的违规判定和置信度。兼容 OpenAI Moderations API。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 omni-moderation-latest）' },
					{ field: 'input', type: 'string | array', desc: '待审核的文本或文本数组', required: true },
				],
				example: {
					req: `{
  "model": "omni-moderation-latest",
  "input": "This is a perfectly normal sentence."
}`,
					resp: `{
  "id": "modr-abc123",
  "model": "omni-moderation-latest",
  "results": [{
    "flagged": false,
    "categories": {
      "harassment": false,
      "harassment/threatening": false,
      "hate": false,
      "hate/threatening": false,
      "self-harm": false,
      "self-harm/instructions": false,
      "self-harm/intent": false,
      "sexual": false,
      "sexual/minors": false,
      "violence": false,
      "violence/graphic": false
    },
    "category_scores": {
      "harassment": 0.001,
      "hate": 0.0002,
      "sexual": 0.0001,
      "violence": 0.0003
    }
  }]
}`,
				},
			},
		],
	},
	{
		id: 'models',
		label: '模型查询',
		icon: 'search',
		endpoints: [
			{
				id: 'list-models',
				method: 'GET',
				path: '/v1/models',
				title: 'List Models',
				desc: '获取当前 API Key 可用的所有模型列表，包括模型 ID、创建时间、归属等信息。',
				params: [],
				example: {
					resp: `{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1715367049,
      "owned_by": "system"
    },
    {
      "id": "claude-sonnet-4-20250514",
      "object": "model",
      "created": 1715367049,
      "owned_by": "system"
    }
  ]
}`,
				},
			},
			{
				id: 'get-model',
				method: 'GET',
				path: '/v1/models/{model_id}',
				title: 'Get Model',
				desc: '获取指定模型的详细信息。',
				params: [
					{ field: 'model_id', type: 'string', desc: '模型 ID（路径参数）', required: true },
				],
				example: {
					resp: `{
  "id": "gpt-4o",
  "object": "model",
  "created": 1715367049,
  "owned_by": "system"
}`,
				},
			},
		],
	},
	{
		id: 'video',
		label: '视频生成',
		icon: 'play',
		endpoints: [
			{
				id: 'video-generations',
				method: 'POST',
				path: '/v1/video/generations',
				title: 'Video Generations (Submit)',
				desc: '提交视频生成任务。该接口为异步任务模式，提交后返回任务 ID，通过查询接口轮询结果。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 kling-video-v1）', required: true },
					{ field: 'prompt', type: 'string', desc: '视频描述提示词' },
					{ field: 'metadata', type: 'object', desc: '附加参数（resolution 分辨率、duration 时长、ratio 比例等）' },
					{ field: 'size', type: 'string', desc: '视频分辨率（已弃用，建议使用 metadata.resolution）' },
					{ field: 'length', type: 'integer', desc: '视频时长秒数（已弃用，建议使用 metadata.duration）' },
				],
				example: {
					req: `{
  "model": "kling-video-v1",
  "prompt": "A cat walking through a garden in slow motion",
  "metadata": {
    "resolution": "1080p",
    "duration": 5,
    "ratio": "16:9"
  }
}`,
					resp: `{
  "id": "task_abc123",
  "status": "pending",
  "model": "kling-video-v1"
}`,
				},
			},
			{
				id: 'video-fetch',
				method: 'GET',
				path: '/v1/video/generations/{task_id}',
				title: 'Video Generations (Fetch)',
				desc: '查询视频生成任务结果。返回任务状态和生成结果（成功时包含视频 URL）。',
				params: [
					{ field: 'task_id', type: 'string', desc: '任务 ID（路径参数）', required: true },
				],
				example: {
					resp: `{
  "id": "task_abc123",
  "status": "succeeded",
  "model": "kling-video-v1",
  "output": {
    "video_url": "https://cdn.example.com/video/abc123.mp4",
    "duration": 5
  }
}`,
				},
			},
		],
	},
	{
		id: 'realtime',
		label: '实时对话',
		icon: 'mic',
		endpoints: [
			{
				id: 'realtime',
				method: 'GET',
				path: '/v1/realtime',
				title: 'Realtime API',
				desc: '实时多模态对话接口。基于 WebSocket 协议，支持实时语音输入输出，适用于语音助手、实时翻译等场景。使用 session.update 事件配置会话参数。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（在 session.update 事件中设置）', required: true },
					{ field: 'modalities', type: 'array', desc: '支持的模态（["text", "audio"]）' },
					{ field: 'instructions', type: 'string', desc: '系统指令' },
					{ field: 'voice', type: 'string', desc: '语音类型（alloy / echo / fable / onyx / nova / shimmer）' },
					{ field: 'input_audio_format', type: 'string', desc: '输入音频格式（pcm16 / g711_ulaw / g711_alaw）' },
					{ field: 'output_audio_format', type: 'string', desc: '输出音频格式' },
					{ field: 'turn_detection', type: 'object', desc: '语音活动检测配置' },
					{ field: 'tools', type: 'array', desc: '工具定义列表' },
					{ field: 'temperature', type: 'number', desc: '采样温度' },
					{ field: 'max_response_output_tokens', type: 'integer', desc: '单次响应最大 Token 数' },
				],
				example: {
					req: `// WebSocket 连接
// wss://api.team-api.example.com/v1/realtime?model=gpt-4o-realtime-preview
// 连接后发送 session.update 事件：
{
  "type": "session.update",
  "session": {
    "model": "gpt-4o-realtime-preview",
    "modalities": ["text", "audio"],
    "instructions": "You are a helpful assistant.",
    "voice": "alloy",
    "input_audio_format": "pcm16",
    "output_audio_format": "pcm16",
    "turn_detection": {
      "type": "server_vad",
      "threshold": 0.5,
      "silence_duration_ms": 500
    }
  }
}`,
				},
			},
		],
	},
	{
		id: 'gemini',
		label: 'Gemini 兼容',
		icon: 'sparkles',
		endpoints: [
			{
				id: 'gemini-generate',
				method: 'POST',
				path: '/v1beta/models/{model}',
				title: 'Gemini Generate Content',
				desc: 'Google Gemini API 兼容接口。支持 Gemini 系列模型的内容生成，包括多模态输入。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（路径参数，如 gemini-2.0-flash）', required: true },
					{ field: 'contents', type: 'array', desc: '对话内容数组，Gemini 格式', required: true },
					{ field: 'system_instruction', type: 'object', desc: '系统指令' },
					{ field: 'generation_config', type: 'object', desc: '生成配置（temperature, topP, maxOutputTokens 等）' },
					{ field: 'safety_settings', type: 'array', desc: '安全过滤配置' },
					{ field: 'tools', type: 'array', desc: '工具定义列表' },
				],
				example: {
					req: `// POST /v1beta/models/gemini-2.0-flash
{
  "contents": [{
    "role": "user",
    "parts": [{"text": "Explain quantum computing simply."}]
  }],
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 256
  }
}`,
					resp: `{
  "candidates": [{
    "content": {
      "parts": [{"text": "Quantum computing uses quantum bits..."}],
      "role": "model"
    },
    "finishReason": "STOP"
  }],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 50,
    "totalTokenCount": 60
  }
}`,
				},
			},
			{
				id: 'gemini-list-models',
				method: 'GET',
				path: '/v1beta/models',
				title: 'Gemini List Models',
				desc: '获取 Gemini 格式的可用模型列表。',
				params: [],
			},
			{
				id: 'gemini-get-model',
				method: 'GET',
				path: '/v1beta/models/{model}',
				title: 'Gemini Get Model',
				desc: '获取 Gemini 格式的指定模型详情。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（路径参数）', required: true },
				],
			},
		],
	},
]

// Flatten all endpoints for sidebar navigation
const allEndpoints = computed(() =>
	categories.flatMap(cat => cat.endpoints.map(ep => ({ ...ep, categoryId: cat.id })))
)


// ============================================================
// Code Examples Section
// ============================================================
const embeddingExamplePython = `import openai

client = openai.OpenAI(
    api_key="sk-your-api-key",
    base_url="https://api.team-api.example.com/v1"
)

response = client.embeddings.create(
    model="text-embedding-3-small",
    input="The quick brown fox jumps over the lazy dog"
)

print(response.data[0].embedding[:5])  # [0.0023, -0.0094, 0.0151, ...]`

const ttsExampleCurl = `curl -X POST https://api.team-api.example.com/v1/audio/speech \\
  -H "Authorization: Bearer sk-your-api-key" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "tts-1",
    "input": "Hello, welcome to Team API!",
    "voice": "alloy"
  }' \\
  --output speech.mp3`

const rerankExamplePython = `import requests

resp = requests.post(
    "https://api.team-api.example.com/v1/rerank",
    headers={
        "Authorization": "Bearer sk-your-api-key",
        "Content-Type": "application/json"
    },
    json={
        "model": "jina-reranker-v2-base-multilingual",
        "query": "What is quantum computing?",
        "documents": [
            "Quantum computing uses quantum mechanics...",
            "Machine learning is a branch of AI...",
            "Quantum bits are the basic unit of quantum computing..."
        ],
        "top_n": 3
    }
)

for result in resp.json()["results"]:
    print(f"#{result['index']} score={result['relevance_score']:.2f}")`

const streamChatExampleNode = `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-your-api-key",
  baseURL: "https://api.team-api.example.com/v1",
});

// Streaming chat with tool calling
const stream = await client.chat.completions.create({
  model: "gpt-4o",
  messages: [
    { role: "system", content: "You are a weather assistant." },
    { role: "user", content: "What's the weather in Beijing?" },
  ],
  tools: [{
    type: "function",
    function: {
      name: "get_weather",
      description: "Get current weather for a location",
      parameters: {
        type: "object",
        properties: {
          location: { type: "string", description: "City name" }
        },
        required: ["location"]
      }
    }
  }],
  stream: true,
});

for await (const chunk of stream) {
  const delta = chunk.choices[0]?.delta;
  if (delta?.content) process.stdout.write(delta.content);
  if (delta?.tool_calls) {
    for (const tc of delta.tool_calls) {
      console.log("Tool call:", tc.function?.name, tc.function?.arguments);
    }
  }
}`

const claudeExamplePython = `from anthropic import Anthropic

client = Anthropic(
    api_key="sk-your-api-key",
    base_url="https://api.team-api.example.com"
)

# Note: The SDK adds /v1/messages automatically
message = client.messages.create(
    model="claude-sonnet-4-20250514",
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "Explain quantum entanglement."}
    ]
)

print(message.content[0].text)`

const codeExamples = [
	{ title: 'Python 流式聊天', lang: 'python', code: pythonExample },
	{ title: 'Node.js 流式调用', lang: 'javascript', code: nodeExample },
	{ title: 'Go 流式调用', lang: 'go', code: goExample },
	{ title: 'Python 向量嵌入', lang: 'python', code: embeddingExamplePython },
	{ title: 'cURL 语音合成', lang: 'bash', code: ttsExampleCurl },
	{ title: 'Python 文档重排序', lang: 'python', code: rerankExamplePython },
	{ title: 'Node.js 工具调用流式', lang: 'javascript', code: streamChatExampleNode },
	{ title: 'Python Claude API 调用', lang: 'python', code: claudeExamplePython },
]

const copiedExample = ref(-1)
async function copyExampleCode(idx: number) {
	try {
		await navigator.clipboard.writeText(codeExamples[idx].code)
		copiedExample.value = idx
		setTimeout(() => { copiedExample.value = -1 }, 2000)
	} catch (e) { console.error(e) }
}

// ============================================================
// Error Codes Section
// ============================================================
const errorCodes = [
	{ status: 400, code: 400, desc: '请求参数格式错误、必填字段缺失或值不合法', example: 'Invalid request: messages is required' },
	{ status: 401, code: 401, desc: 'API Key 无效、缺失或已过期', example: 'Invalid API key provided' },
	{ status: 403, code: 403, desc: 'API Key 无权访问该模型或资源', example: 'Model gpt-5 is not available for your plan' },
	{ status: 404, code: 404, desc: '请求的模型或资源不存在', example: 'Model not found: gpt-4-nonexistent' },
	{ status: 429, code: 429, desc: '请求频率超过限流阈值', example: 'Rate limit exceeded. Please retry after 60s' },
	{ status: 422, code: 10001, desc: '钱包余额不足，请充值后重试', example: 'Insufficient balance' },
	{ status: 422, code: 10002, desc: '套餐或 Key 额度已用完', example: 'Quota exceeded' },
	{ status: 422, code: 10003, desc: '没有可用的上游渠道处理该请求', example: 'No available channel for this model' },
	{ status: 500, code: 500, desc: '服务器内部错误，请联系技术支持', example: 'Internal server error' },
]

// OpenAI-compatible error response format (for /v1/* relay endpoints)
const relayErrorResponse = `{
  "error": {
    "message": "Rate limit exceeded. Please retry after 60s.",
    "type": "rate_limit_error",
    "code": "429"
  }
}`

// Management API error response format (for /api/* endpoints)
const mgmtErrorResponse = `{
  "code": 10001,
  "message": "余额不足",
  "data": null,
  "request_id": "req_abc123"
}`

const copiedBlock = ref('')
async function copyBlock(text: string, key: string) {
	try {
		await navigator.clipboard.writeText(text)
		copiedBlock.value = key
		setTimeout(() => { copiedBlock.value = '' }, 2000)
	} catch (e) { console.error(e) }
}

// Initialize active endpoint
function initEndpoint() {
	if (!activeEndpoint.value && allEndpoints.value.length > 0) {
		activeEndpoint.value = allEndpoints.value[0].id
	}
}
initEndpoint()
</script>

<template>
	<div class="flex gap-6">
		<!-- Main Content -->
		<div class="flex-1 min-w-0 space-y-6">
			<!-- Page Header -->
			<div class="page-header flex items-center justify-between">
				<div>
					<h1 class="page-title">API 文档</h1>
					<p class="page-description">集成 Team API 所需的一切 — 兼容 OpenAI / Claude / Gemini 格式</p>
				</div>
				<button class="btn btn-primary" @click="goToPlayground">
					<Icon name="cog" size="sm" />
					在线测试
				</button>
			</div>

			<!-- Section Navigation -->
			<div class="tabs">
				<button
					v-for="section in sections"
					:key="section.id"
					@click="activeSection = section.id"
					class="tab"
					:class="activeSection === section.id ? 'tab-active' : ''"
				>
					{{ section.label }}
				</button>
			</div>

			<!-- ==================== Quick Start ==================== -->
			<div v-if="activeSection === 'quickstart'" class="space-y-6">
				<!-- Step 1 -->
				<div class="card">
					<div class="card-header flex items-center gap-3">
						<div class="h-7 w-7 rounded-full flex items-center justify-center text-xs font-bold text-white bg-gradient-to-r from-primary-500 to-primary-600">1</div>
						<h3 class="font-semibold text-gray-900">获取 API Key</h3>
					</div>
					<div class="card-body">
						<p class="text-gray-600 text-sm leading-relaxed">
							在控制台中进入 <router-link to="/tenant/api-keys" class="text-primary-600 hover:underline">API 密钥</router-link> 页面创建新密钥。每个密钥可限定特定模型并配置速率限制。请妥善保管您的密钥——不要分享或提交到版本控制中。
						</p>
					</div>
				</div>

				<!-- Step 2 -->
				<div class="card">
					<div class="card-header flex items-center gap-3">
						<div class="h-7 w-7 rounded-full flex items-center justify-center text-xs font-bold text-white bg-gradient-to-r from-primary-500 to-primary-600">2</div>
						<h3 class="font-semibold text-gray-900">发起第一次请求</h3>
					</div>
					<div class="card-body">
						<p class="text-gray-600 text-sm leading-relaxed mb-4">
							Team API 完全兼容 OpenAI API 格式。只需将 <code class="code">base_url</code> 改为我们的端点并使用您的 API Key 即可：
						</p>

						<!-- Code tabs -->
						<div class="card overflow-hidden !p-0">
							<div class="flex items-center bg-gray-50 border-b border-gray-200 px-2">
								<button
									v-for="tab in (['curl', 'python', 'node', 'go'] as CodeTab[])"
									:key="tab"
									@click="activeCodeTab = tab"
									class="px-4 py-2.5 text-xs font-medium transition-colors rounded-t-lg border-b-2 -mb-px"
									:class="activeCodeTab === tab ? 'text-primary-600 border-primary-600 bg-white' : 'text-gray-500 border-transparent hover:text-gray-700'"
								>
									{{ tabLabels[tab] }}
								</button>
								<div class="flex-1"></div>
								<button
									@click="copyText(getExampleCode(activeCodeTab), activeCodeTab)"
									class="mr-2 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
									:class="copiedTab === activeCodeTab ? 'text-emerald-600 bg-emerald-50' : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'"
								>
									{{ copiedTab === activeCodeTab ? '已复制' : '复制' }}
								</button>
							</div>
							<pre class="code-block"><code>{{ getExampleCode(activeCodeTab) }}</code></pre>
						</div>
					</div>
				</div>

				<!-- Step 3 -->
				<div class="card">
					<div class="card-header flex items-center gap-3">
						<div class="h-7 w-7 rounded-full flex items-center justify-center text-xs font-bold text-white bg-gradient-to-r from-primary-500 to-primary-600">3</div>
						<h3 class="font-semibold text-gray-900">探索与测试</h3>
					</div>
					<div class="card-body">
						<p class="text-gray-600 text-sm leading-relaxed mb-4">
							Team API 通过统一接口提供 40+ 个 AI 模型的访问，同时兼容 OpenAI、Claude、Gemini 三大 API 格式。您可以使用 Playground 在线调试不同参数组合，或通过 <code class="code">/v1/models</code> 接口查询完整模型列表。
						</p>
						<div class="flex gap-3">
							<button class="btn btn-primary btn-sm" @click="goToPlayground">
								<Icon name="cog" size="sm" />
								打开 Playground
							</button>
							<button class="btn btn-secondary btn-sm" @click="activeSection = 'api-reference'">
								查看 API 参考
							</button>
						</div>
					</div>
				</div>

				<!-- Compatibility Banner -->
				<div class="card bg-gradient-to-r from-primary-50 to-cyan-50 border-primary-200">
					<div class="card-body">
						<h3 class="font-semibold text-gray-900 mb-3">兼容性说明</h3>
						<p class="text-gray-600 text-sm leading-relaxed mb-4">
							Team API 设计为与主流大模型 API 完全兼容，您可以直接使用各语言官方 SDK：
						</p>
						<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
							<div class="bg-white/80 rounded-xl p-4 border border-primary-100">
								<div class="text-sm font-semibold text-gray-900 mb-1">OpenAI 兼容</div>
								<div class="text-xs text-gray-500 mb-2">Chat / Embeddings / Images / Audio</div>
								<code class="code text-xs">base_url = ".../v1"</code>
							</div>
							<div class="bg-white/80 rounded-xl p-4 border border-primary-100">
								<div class="text-sm font-semibold text-gray-900 mb-1">Claude 兼容</div>
								<div class="text-xs text-gray-500 mb-2">Anthropic Messages API</div>
								<code class="code text-xs">base_url = ".../v1"</code>
							</div>
							<div class="bg-white/80 rounded-xl p-4 border border-primary-100">
								<div class="text-sm font-semibold text-gray-900 mb-1">Gemini 兼容</div>
								<div class="text-xs text-gray-500 mb-2">Google GenerateContent API</div>
								<code class="code text-xs">端点: /v1beta/models/...</code>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- ==================== API Reference ==================== -->
			<div v-if="activeSection === 'api-reference'" class="space-y-6">
				<!-- Category Cards -->
				<div v-for="cat in categories" :key="cat.id" class="space-y-3">
					<h3 class="text-lg font-semibold text-gray-900 flex items-center gap-2">
						{{ cat.label }}
					</h3>
					<div class="space-y-3">
						<div
							v-for="ep in cat.endpoints"
							:key="ep.id"
							class="card cursor-pointer transition-all duration-200"
							:class="activeEndpoint === ep.id ? 'ring-2 ring-primary-500/30 shadow-glow' : 'hover:-translate-y-0.5 hover:shadow-card-hover'"
							@click="activeEndpoint = ep.id"
						>
							<div class="card-header">
								<div class="flex items-center gap-3">
									<span
										class="badge text-xs font-bold"
										:class="{
											'badge-success': ep.method === 'POST',
											'badge-primary': ep.method === 'GET',
											'badge-warning': ep.method === 'PUT',
											'badge-danger': ep.method === 'DELETE',
										}"
									>{{ ep.method }}</span>
									<code class="code text-sm">{{ ep.path }}</code>
								</div>
								<h4 class="font-semibold text-gray-900 mt-2">{{ ep.title }}</h4>
								<p class="text-sm text-gray-500 mt-0.5">{{ ep.desc }}</p>
							</div>

							<!-- Expanded Detail -->
							<div v-if="activeEndpoint === ep.id" class="card-body space-y-4">
								<!-- Parameters Table -->
								<div v-if="ep.params.length > 0">
									<h5 class="text-sm font-semibold text-gray-800 mb-2">请求参数</h5>
									<div class="table-container">
										<table class="table">
											<thead>
												<tr>
													<th>字段</th>
													<th>类型</th>
													<th>必填</th>
													<th>说明</th>
												</tr>
											</thead>
											<tbody>
												<tr v-for="p in ep.params" :key="p.field">
													<td class="font-mono text-primary-600 text-xs">{{ p.field }}</td>
													<td class="text-gray-500 text-xs">{{ p.type }}</td>
													<td>
														<span v-if="p.required" class="badge badge-danger text-xs">必填</span>
														<span v-else class="text-gray-400 text-xs">可选</span>
													</td>
													<td class="text-gray-600 text-sm">{{ p.desc }}</td>
												</tr>
											</tbody>
										</table>
									</div>
								</div>

								<!-- Request/Response Examples -->
								<div v-if="ep.example" class="space-y-3">
									<!-- Authentication hint -->
									<div class="bg-primary-50 rounded-xl p-3 text-xs text-primary-700">
										<Icon name="infoCircle" size="sm" class="inline -mt-0.5" />
										所有请求需携带 <code class="bg-white/60 rounded px-1">Authorization: Bearer sk-your-api-key</code> 头
									</div>

									<!-- Request Example -->
									<div v-if="ep.example.req">
										<div class="flex items-center justify-between mb-1.5">
											<span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">请求示例</span>
											<button
												class="text-xs text-gray-400 hover:text-primary-600 transition-colors"
												@click="copyBlock(ep.example!.req!, 'req-' + ep.id)"
											>
												{{ copiedBlock === 'req-' + ep.id ? '✓ 已复制' : '复制' }}
											</button>
										</div>
										<pre class="code-block !text-xs"><code>{{ ep.example.req }}</code></pre>
									</div>

									<!-- Response Example -->
									<div v-if="ep.example.resp">
										<div class="flex items-center justify-between mb-1.5">
											<span class="text-xs font-semibold text-gray-500 uppercase tracking-wider">响应示例</span>
											<button
												class="text-xs text-gray-400 hover:text-primary-600 transition-colors"
												@click="copyBlock(ep.example!.resp!, 'resp-' + ep.id)"
											>
												{{ copiedBlock === 'resp-' + ep.id ? '✓ 已复制' : '复制' }}
											</button>
										</div>
										<pre class="code-block !text-xs"><code>{{ ep.example.resp }}</code></pre>
									</div>
								</div>

								<!-- Action buttons -->
								<div v-if="ep.method === 'POST'" class="flex justify-end">
									<button class="btn btn-ghost btn-sm" @click.stop="goToPlayground">
										<Icon name="cog" size="sm" />
										在 Playground 中测试
									</button>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- ==================== Code Examples ==================== -->
			<div v-if="activeSection === 'code-examples'" class="space-y-6">
				<p class="text-sm text-gray-500">Team API 兼容 OpenAI / Anthropic SDK，只需修改 <code class="code">base_url</code> 即可无缝切换。</p>
				<div v-for="(ex, idx) in codeExamples" :key="ex.title" class="card overflow-hidden">
					<div class="card-header flex items-center justify-between">
						<div>
							<h3 class="font-semibold text-gray-900">{{ ex.title }}</h3>
							<span class="text-xs text-gray-400 mt-0.5">{{ ex.lang }}</span>
						</div>
						<button
							@click="copyExampleCode(idx)"
							class="btn btn-ghost btn-sm"
							:class="copiedExample === idx ? 'text-emerald-600' : ''"
						>
							<Icon :name="copiedExample === idx ? 'check' : 'copy'" size="sm" />
							{{ copiedExample === idx ? '已复制' : '复制' }}
						</button>
					</div>
					<pre class="code-block"><code>{{ ex.code }}</code></pre>
				</div>
			</div>

			<!-- ==================== Error Codes ==================== -->
			<div v-if="activeSection === 'error-codes'" class="space-y-6">
				<p class="text-sm text-gray-500">
					API 返回两种错误格式：AI 代理端点（<code class="code">/v1/*</code>）使用 OpenAI 兼容格式；管理类端点（<code class="code">/api/*</code>）使用统一响应格式。
				</p>

				<!-- Error response format examples -->
				<div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
					<div class="card">
						<div class="card-header">
							<h3 class="font-semibold text-gray-900">AI 代理端点错误格式</h3>
							<p class="text-xs text-gray-400">/v1/* 端点使用 OpenAI 兼容错误格式</p>
						</div>
						<div class="relative">
							<pre class="code-block !text-xs rounded-t-none"><code>{{ relayErrorResponse }}</code></pre>
							<button
								class="absolute top-2 right-2 text-xs text-gray-400 hover:text-primary-600 transition-colors"
								@click="copyBlock(relayErrorResponse, 'relay-err')"
							>
								{{ copiedBlock === 'relay-err' ? '✓' : '复制' }}
							</button>
						</div>
					</div>
					<div class="card">
						<div class="card-header">
							<h3 class="font-semibold text-gray-900">管理类端点错误格式</h3>
							<p class="text-xs text-gray-400">/api/* 端点使用统一响应格式</p>
						</div>
						<div class="relative">
							<pre class="code-block !text-xs rounded-t-none"><code>{{ mgmtErrorResponse }}</code></pre>
							<button
								class="absolute top-2 right-2 text-xs text-gray-400 hover:text-primary-600 transition-colors"
								@click="copyBlock(mgmtErrorResponse, 'mgmt-err')"
							>
								{{ copiedBlock === 'mgmt-err' ? '✓' : '复制' }}
							</button>
						</div>
					</div>
				</div>

				<!-- Error code table -->
				<div class="card">
					<div class="card-header">
						<h3 class="font-semibold text-gray-900">错误码一览</h3>
					</div>
					<div class="card-body !pt-0">
						<div class="table-container">
							<table class="table">
								<thead>
									<tr>
										<th>HTTP 状态码</th>
										<th>业务错误码</th>
										<th>说明</th>
										<th>示例消息</th>
									</tr>
								</thead>
								<tbody>
									<tr v-for="err in errorCodes" :key="err.code">
										<td>
											<span class="badge text-xs" :class="{
												'badge-warning': err.status >= 400 && err.status < 500,
												'badge-danger': err.status >= 500,
											}">{{ err.status }}</span>
										</td>
										<td class="font-mono text-xs text-gray-600">{{ err.code }}</td>
										<td class="text-sm text-gray-700">{{ err.desc }}</td>
										<td class="text-xs text-gray-400 font-mono">{{ err.example }}</td>
									</tr>
								</tbody>
							</table>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Right Sidebar — API Reference TOC (only visible in api-reference section on desktop) -->
		<aside v-if="activeSection === 'api-reference'" class="hidden lg:block w-64 shrink-0">
			<div class="sticky top-24 space-y-4">
				<div class="card !p-4">
					<h4 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">接口目录</h4>
					<nav class="space-y-3">
						<div v-for="cat in categories" :key="cat.id">
							<div class="text-xs font-semibold text-gray-900 mb-1.5">{{ cat.label }}</div>
							<button
								v-for="ep in cat.endpoints"
								:key="ep.id"
								@click="activeEndpoint = ep.id"
								class="block w-full text-left px-2 py-1 text-xs rounded-lg transition-colors"
								:class="activeEndpoint === ep.id
									? 'bg-primary-50 text-primary-600 font-medium'
									: 'text-gray-500 hover:text-gray-700 hover:bg-gray-50'"
							>
								<span
									class="inline-block w-8 text-[10px] font-bold mr-1"
									:class="ep.method === 'POST' ? 'text-emerald-500' : 'text-primary-500'"
								>{{ ep.method }}</span>
								{{ ep.title }}
							</button>
						</div>
					</nav>
				</div>
			</div>
		</aside>
	</div>
</template>
