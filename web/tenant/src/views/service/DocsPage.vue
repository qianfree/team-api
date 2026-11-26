<script setup lang="ts">
import { ref, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NTag } from 'naive-ui'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'

const router = useRouter()
const activeSection = ref('quickstart')
// '' 表示 API 参考的「全部接口」总览，否则为当前查看的端点 id
const activeEndpoint = ref('')
// 左栏目录：搜索词、移动端抽屉、分类折叠状态
const railQuery = ref('')
const railOpen = ref(false)
const collapsedCats = ref<string[]>([])

// 中继接口与控制台同源部署，示例中的 API 地址直接取当前站点真实域名
const apiBaseUrl = window.location.origin
const wsBaseUrl = window.location.protocol === 'https:' ? `wss://${window.location.host}` : `ws://${window.location.host}`

const sections = [
	{ id: 'quickstart', label: '快速入门', icon: 'bolt', desc: '5 分钟完成第一次调用' },
	{ id: 'api-reference', label: 'API 参考', icon: 'bookOpen', desc: '全部端点 / 参数 / 示例' },
	{ id: 'code-examples', label: '代码示例', icon: 'terminal', desc: '多语言可复制片段' },
	{ id: 'error-codes', label: '错误码', icon: 'exclamationCircle', desc: '错误格式与排查' },
]

// 分类图标：数据里的 icon 字段部分取值在 Icon.vue 中并不存在，这里做统一映射
const categoryIcons: Record<string, string> = {
	chat: 'chat',
	claude: 'star',
	responses: 'bolt',
	embeddings: 'link',
	images: 'photo',
	audio: 'musicalNote',
	rerank: 'trendingUp',
	moderation: 'shield',
	models: 'search',
	video: 'film',
	realtime: 'terminal',
	gemini: 'cube',
}

const methodTone: Record<string, string> = {
	POST: 'method-post',
	GET: 'method-get',
	PUT: 'method-put',
	DELETE: 'method-delete',
}

function scrollToTop() {
	// 实际滚动容器是 Layout 的 #main-content（overflow: auto），window 并不滚动；
	// 两处都归零以兼容将来布局变化。
	const scroller = document.getElementById('main-content')
	if (scroller) scroller.scrollTo({ top: 0, behavior: 'auto' })
	window.scrollTo({ top: 0, behavior: 'auto' })
}

function openSection(id: string) {
	activeSection.value = id
	if (id !== 'api-reference') activeEndpoint.value = ''
	railOpen.value = false
	scrollToTop()
}

function openOverview() {
	activeSection.value = 'api-reference'
	activeEndpoint.value = ''
	railOpen.value = false
	scrollToTop()
}

function openEndpoint(id: string) {
	activeSection.value = 'api-reference'
	activeEndpoint.value = id
	// 目录默认全折叠，进入某个端点时把它所在的分类展开，否则高亮项藏在收起的组里
	const categoryId = allEndpoints.value.find(ep => ep.id === id)?.categoryId
	if (categoryId && collapsedCats.value.includes(categoryId)) {
		collapsedCats.value = collapsedCats.value.filter(c => c !== categoryId)
	}
	railOpen.value = false
	scrollToTop()
}

function toggleCategory(id: string) {
	const idx = collapsedCats.value.indexOf(id)
	if (idx === -1) collapsedCats.value.push(id)
	else collapsedCats.value.splice(idx, 1)
}

// ============================================================
// Quick Start Examples
// ============================================================
const curlExample = `curl -X POST ${apiBaseUrl}/v1/chat/completions \\
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
    base_url="${apiBaseUrl}/v1"
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
  baseURL: "${apiBaseUrl}/v1",
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
    config.BaseURL = "${apiBaseUrl}/v1"

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
			{
				id: 'images-generations-async-submit',
				method: 'POST',
				path: '/v1/images/generations/async',
				title: 'Image Generations Async (Submit)',
				desc: '异步图像生成任务提交。阿里云百炼（DashScope，通义万相/qwen-image 等）图像上游为异步任务式，无法经同步 /v1/images/generations 一次性返回，需通过本接口提交任务拿到 task_id，再轮询查询接口取图。请求体与同步接口一致。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 wanx2.1-t2i-turbo、qwen-image）', required: true },
					{ field: 'prompt', type: 'string', desc: '图像描述提示词', required: true },
					{ field: 'size', type: 'string', desc: '图像尺寸（如 1024x1024）' },
					{ field: 'n', type: 'integer', desc: '生成数量（默认 1）' },
				],
				example: {
					req: `{
  "model": "wanx2.1-t2i-turbo",
  "prompt": "A white siamese cat wearing sunglasses",
  "size": "1024x1024"
}`,
					resp: `{
  "id": "task_abc123",
  "status": "SUBMITTED",
  "model": "wanx2.1-t2i-turbo",
  "created_at": 1717100000
}`,
				},
			},
			{
				id: 'images-generations-async-fetch',
				method: 'GET',
				path: '/v1/images/generations/async/{task_id}',
				title: 'Image Generations Async (Fetch)',
				desc: '查询异步图像生成任务结果。轮询本接口直到 status 为 SUCCESS（含图片 url）或 FAILURE（含 error）。建议轮询间隔 2~3 秒。状态流转：SUBMITTED / QUEUED / IN_PROGRESS → SUCCESS / FAILURE。',
				params: [
					{ field: 'task_id', type: 'string', desc: '任务 ID（路径参数）', required: true },
				],
				example: {
					resp: `{
  "id": "task_abc123",
  "status": "SUCCESS",
  "model": "wanx2.1-t2i-turbo",
  "created_at": 1717100000,
  "completed_at": 1717100030,
  "url": "https://cdn.example.com/img/abc123.png"
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
				id: 'videos-create',
				method: 'POST',
				path: '/v1/videos',
				title: 'Create Video（OpenAI 协议）',
				desc: '创建视频生成任务，兼容 OpenAI Videos 协议（openai-python / openai-node SDK 的 client.videos.create）。异步任务模式：提交后返回 Video 对象（status: queued），轮询 Retrieve 直到 completed。参考素材（含首帧图）统一经 references 扩展字段传递，支持多参考图、参考视频、参考音频与首尾帧（Seedance / MiniMax H3 等多模态参考模型）；厂商音频开关等私有参数经 extra 透传。历史字段 input_reference / sound / generate_audio 已移除，命中返回 400（错误码 field_deprecated）。参考素材参数错误返回 400，错误码：invalid_references / invalid_reference_role / invalid_reference_source / reference_mode_conflict / reference_file_not_found / reference_file_type_mismatch / too_many_references / reference_not_supported。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 doubao-seedance-2-0-260128、MiniMax-H3）', required: true },
					{ field: 'prompt', type: 'string', desc: '视频描述提示词。多模态参考模型在提示词中按「图片N / 视频N / 音频N」引用素材（N 为同类素材在 references 中的顺序，从 1 起）', required: true },
					{ field: 'seconds', type: 'string | number', desc: '视频时长（秒）。官方 SDK 为字符串形态；各模型档位不同，缺省由模型决定' },
					{ field: 'size', type: 'string', desc: '分辨率。值原样透传，请传目标模型的官方档位词汇（如 Seedance 的 720p/1080p、MiniMax H3 的 768P/2K）' },
					{ field: 'aspect_ratio', type: 'string', desc: '屏幕比例（扩展字段，如 16:9 / 9:16 / 1:1），原样透传，不传由模型取默认' },
					{ field: 'references', type: 'array', desc: '参考素材唯一通道：有序列表 [{role, url | file_id}]。role ∈ first_frame / last_frame / reference_image / reference_video / reference_audio（首帧参考图用 first_frame）；url 支持 http(s)、base64 data URL（MIME 须与角色匹配）、厂商素材 URI（asset:// / mm_file://，仅对应厂商渠道有效）；file_id 为 /v1/files 上传的文件（同租户，类型须与角色匹配），url 与 file_id 二选一。列表顺序即提示词「图片N/视频N/音频N」编号依据；首尾帧与 reference_* 互斥；协议层上限：总数 20、参考图 9、参考视频 3、参考音频 3，模型特有约束（数量、音频须搭配图/视频等）由模型侧校验。官方 SDK 经 extra_body 传入时须 JSON 字符串：extra_body={"references": JSON.stringify([...])}' },
					{ field: 'extra', type: 'object', desc: '厂商私有参数透传（扩展字段）：已知键（可灵 mode/negative_prompt/cfg_scale/camera_control、火山 seed/watermark、音频开关 sound("on"/"off")/generate_audio(bool) 等）直接写键名由网关映射并联动计费；其余键按上游原生路径平铺深合并（如万相音频 {"parameters":{"audio":true}}）' },
				],
				example: {
					req: `{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "图片1的人物在视频1的场景中奔跑，背景音乐参考音频1",
  "seconds": "10",
  "aspect_ratio": "16:9",
  "references": [
    {"role": "reference_image", "url": "https://cdn.example.com/hero.png"},
    {"role": "reference_video", "file_id": "file-AbC123xyz"},
    {"role": "reference_audio", "file_id": "file-DeF456uvw"}
  ]
}`,
					resp: `{
  "id": "video_abc123",
  "object": "video",
  "created_at": 1717100000,
  "status": "queued",
  "progress": 0,
  "model": "doubao-seedance-2-0-260128"
}`,
				},
			},
			{
				id: 'videos-retrieve',
				method: 'GET',
				path: '/v1/videos/{video_id}',
				title: 'Retrieve Video（OpenAI 协议）',
				desc: '查询视频任务，返回官方 Video 对象（status: queued / in_progress / completed / failed）。非终态响应携带 openai-poll-after-ms 轮询间隔建议头；生成失败以 status=failed + error 对象表达（HTTP 仍为 200）。completed 时附扩展字段 video_url：平台签名直链，免 API Key 可直接播放/下载，每次查询重新签发（有效期默认 24 小时，过期后重新查询即可）。产物开启转存平台对象存储时，上游已完成但转存未完成期间保持 in_progress（progress=99），转存完成后才返回 completed + video_url（完成后超过 30 分钟仍未转存成功则放行 completed，链接改走实时回源）。',
				params: [
					{ field: 'video_id', type: 'string', desc: '视频任务 ID（路径参数）', required: true },
				],
				example: {
					resp: `{
  "id": "video_abc123",
  "object": "video",
  "created_at": 1717100000,
  "completed_at": 1717100300,
  "status": "completed",
  "progress": 100,
  "model": "doubao-seedance-2-0-260128",
  "prompt": "图片1的人物在视频1的场景中奔跑，背景音乐参考音频1",
  "seconds": "10",
  "video_url": "${apiBaseUrl}/v1/tasks/video_abc123/content?expires=1817103900&signature=..."
}`,
				},
			},
			{
				id: 'videos-content',
				method: 'GET',
				path: '/v1/videos/{video_id}/content',
				title: 'Download Video Content（OpenAI 协议）',
				desc: '下载已完成视频的二进制流（mp4），需 API Key 认证（官方 SDK 的 videos.download_content 走此端点）。仅 status=completed 可下载；转存途中（查询显示 in_progress / progress=99）同样不可下载。平台代理输出字节，不暴露上游 CDN 直链；也可直接使用查询响应中的 video_url 签名直链（免认证）。variant 仅支持默认 video。',
				params: [
					{ field: 'video_id', type: 'string', desc: '视频任务 ID（路径参数）', required: true },
					{ field: 'variant', type: 'string', desc: '内容变体（查询参数），仅支持 video（默认）' },
				],
				example: {
					resp: `// 200 OK
// Content-Type: video/mp4
<视频二进制流>`,
				},
			},
			{
				id: 'files-upload',
				method: 'POST',
				path: '/v1/files',
				title: 'Upload File',
				desc: '上传文件（multipart/form-data），返回 file_id 供视频参考素材引用（references[].file_id）。适合超过请求体大小上限的视频/音频素材：先上传，再在创建视频时以 file_id 引用，网关提交前自动替换为签名 URL（有效期默认 24 小时，覆盖上游排队时长）。文件类型白名单与每租户配额由平台管理员配置。',
				params: [
					{ field: 'file', type: 'file', desc: '要上传的文件（multipart 文件字段）', required: true },
					{ field: 'purpose', type: 'string', desc: '用途标记（OpenAI 词汇，如 user_data / vision），仅作元数据，不驱动网关行为', required: true },
				],
				example: {
					resp: `{
  "id": "file-AbC123xyz",
  "object": "file",
  "filename": "scene.mp4",
  "purpose": "user_data",
  "bytes": 4812389,
  "created_at": 1717100000
}`,
				},
			},
			{
				id: 'video-generations',
				method: 'POST',
				path: '/v1/video/generations',
				title: 'Video Generations (Submit)',
				desc: '提交视频生成任务（通用任务端点，参数已收敛为 8 个固定词汇）。历史重复词汇（seconds/size/ratio/images/media/content/metadata 等）已废弃，命中直接返回 400 并提示替代字段。厂商参数统一经 extra 传递：网关认识的已知键（可灵 mode/cfg_scale/camera_control、火山 seed/watermark、阿里 audio_url 等）直接写键名由网关映射并联动计费，其余键按上游原生路径平铺透传。',
				params: [
					{ field: 'model', type: 'string', desc: '模型 ID（如 kling-video-v1）', required: true },
					{ field: 'prompt', type: 'string', desc: '视频描述提示词（必填，可传空串；纯图生视频场景）', required: true },
					{ field: 'duration', type: 'integer', desc: '视频时长（秒）；-1 = 智能时长（部分模型支持）' },
					{ field: 'resolution', type: 'string', desc: '分辨率档位，传目标模型的原生词汇（如 720P / 2K / 1080p）' },
					{ field: 'aspect_ratio', type: 'string', desc: '画面比例，如 16:9 / 9:16 / 1:1' },
					{ field: 'sound', type: 'string', desc: '音频开关："on" / "off"（部分模型支持）' },
					{ field: 'references', type: 'array', desc: '参考素材列表 [{role, url|file_id}]，role ∈ first_frame / last_frame / reference_image / reference_video / reference_audio / file / link（file=文档文件参考、link=网页链接参考，万相 wan3.0 原生：各最多 1 个且互斥、可与 reference_* 组合）；file_id 为 /v1/files 上传文件的引用（link 不接受 file_id）' },
					{ field: 'extra', type: 'object', desc: '厂商参数唯一通道：已知键（可灵 mode(std/pro)/negative_prompt/cfg_scale/camera_control/image_tail、火山 seed/watermark/return_last_frame、阿里 negative_prompt/audio_url/prompt_extend、音频开关 sound("on"/"off")/generate_audio(bool)）直接写键名，由网关映射到上游正确位置并联动计费；其余键按上游原生路径写（嵌套支持点路径深合并），model/callback_url 与计费规格字段受保护' },
				],
				example: {
					req: `{
  "model": "kling-video-v1",
  "prompt": "A cat walking through a garden in slow motion",
  "duration": 5,
  "resolution": "1080p",
  "aspect_ratio": "16:9",
  "references": [
    { "role": "first_frame", "url": "https://example.com/cat.png" }
  ],
  "extra": { "mode": "std" }
}`,
					resp: `{
  "id": "task_abc123",
  "status": "SUBMITTED",
  "model": "kling-video-v1",
  "created_at": 1717100000
}`,
				},
			},
			{
				id: 'video-fetch',
				method: 'GET',
				path: '/v1/video/generations/{task_id}',
				title: 'Video Generations (Fetch)',
				desc: '查询视频生成任务结果。返回任务状态与结果地址：成功（status=SUCCESS）时 url 为结果地址——JS 插件任务为平台统一内容链接（签名直链，可直接下载），其余平台为上游结果地址；上游已完成但产物仍在转存平台对象存储的 JS 插件任务保持 IN_PROGRESS（progress=99%），转存完成后才返回 SUCCESS + url。成功任务附 usage 用量对象（渠道上报了哪些字段才出现哪些）。',
				params: [
					{ field: 'task_id', type: 'string', desc: '任务 ID（路径参数）', required: true },
				],
				example: {
					resp: `{
  "id": "task_abc123",
  "status": "SUCCESS",
  "model": "kling-video-v1",
  "created_at": 1717100000,
  "progress": "100%",
  "url": "https://cdn.example.com/video/abc123.mp4",
  "completed_at": 1717100300,
  "usage": {
    "output_video_seconds": 5,
    "resolution": "1080p",
    "prompt_tokens": 0,
    "completion_tokens": 38000,
    "total_tokens": 38000
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
// ${wsBaseUrl}/v1/realtime?model=gpt-4o-realtime-preview
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

// 目录默认全部折叠：27 个端点平铺会把左栏撑成一条长列表，找接口反而更慢。
// collapsedCats 存的是「已折叠」的 id，空数组等于全展开，所以这里要显式填满。
collapsedCats.value = categories.map(cat => cat.id)

// Flatten all endpoints for rail navigation
const allEndpoints = computed(() =>
	categories.flatMap(cat => cat.endpoints.map(ep => ({ ...ep, categoryId: cat.id, categoryLabel: cat.label })))
)

// 计数：分类数 / 端点数，用于左栏标题与概览页
const endpointCount = computed(() => allEndpoints.value.length)

// 搜索过滤：命中端点标题、路径或分类名时保留整组
const filteredCategories = computed(() => {
	const q = railQuery.value.trim().toLowerCase()
	if (!q) return categories
	return categories
		.map(cat => {
			const hitCat = cat.label.toLowerCase().includes(q)
			const endpoints = hitCat
				? cat.endpoints
				: cat.endpoints.filter(ep =>
					ep.title.toLowerCase().includes(q)
					|| ep.path.toLowerCase().includes(q)
					|| ep.method.toLowerCase().includes(q)
				)
			return { ...cat, endpoints }
		})
		.filter(cat => cat.endpoints.length > 0)
})

function isCatOpen(id: string) {
	// 搜索状态下分组强制展开，避免命中结果被折叠隐藏
	return railQuery.value.trim() !== '' || !collapsedCats.value.includes(id)
}

const activeEndpointDetail = computed(() =>
	allEndpoints.value.find(ep => ep.id === activeEndpoint.value) || null
)

// 顶部面包屑当前章节名（api-reference 详情页的末级由 activeEndpointDetail 提供）
const currentSectionLabel = computed(
	() => sections.find(s => s.id === activeSection.value)?.label || '文档'
)

const filteredEndpointCount = computed(() =>
	filteredCategories.value.reduce((sum, cat) => sum + cat.endpoints.length, 0)
)

const activeIndex = computed(() =>
	allEndpoints.value.findIndex(ep => ep.id === activeEndpoint.value)
)

const prevEndpoint = computed(() =>
	activeIndex.value > 0 ? allEndpoints.value[activeIndex.value - 1] : null
)

const nextEndpoint = computed(() =>
	activeIndex.value >= 0 && activeIndex.value < allEndpoints.value.length - 1
		? allEndpoints.value[activeIndex.value + 1]
		: null
)

const copiedBase = ref(false)
async function copyBaseUrl() {
	try {
		await navigator.clipboard.writeText(apiBaseUrl)
		copiedBase.value = true
		setTimeout(() => { copiedBase.value = false }, 2000)
	} catch (e) { console.error(e) }
}


// ============================================================
// Code Examples Section
// ============================================================
const embeddingExamplePython = `import openai

client = openai.OpenAI(
    api_key="sk-your-api-key",
    base_url="${apiBaseUrl}/v1"
)

response = client.embeddings.create(
    model="text-embedding-3-small",
    input="The quick brown fox jumps over the lazy dog"
)

print(response.data[0].embedding[:5])  # [0.0023, -0.0094, 0.0151, ...]`

const ttsExampleCurl = `curl -X POST ${apiBaseUrl}/v1/audio/speech \\
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
    "${apiBaseUrl}/v1/rerank",
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
  baseURL: "${apiBaseUrl}/v1",
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
    base_url="${apiBaseUrl}"
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
// 代码示例默认只展开第一条，其余按需展开，避免整页被长代码淹没
const openExample = ref(0)
function toggleExample(idx: number) {
	openExample.value = openExample.value === idx ? -1 : idx
}
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

// 参数表列定义
const paramColumns = computed<DataTableColumns<Param>>(() => [
	{
		title: '字段',
		key: 'field',
		width: 140,
		render: (row) => h('code', { class: 'font-mono text-primary-600 text-xs' }, row.field),
	},
	{
		title: '类型',
		key: 'type',
		width: 110,
		render: (row) => h('span', { class: 'text-gray-500 text-xs' }, row.type),
	},
	{
		title: '必填',
		key: 'required',
		width: 70,
		// 未声明 required 的参数一律视为可选，避免出现 undefined 文本
		render: (row) =>
			renderBadge(
				row.required ? 'true' : 'false',
				{ 'true': '必填', 'false': '可选' },
				{ 'true': 'badge-danger', 'false': 'badge-gray' }
			),
	},
	{
		title: '说明',
		key: 'desc',
		width: 300,
		render: (row) => h('span', { class: 'text-gray-600 text-sm' }, row.desc),
	},
])

// 错误码一览表列定义
const errorColumns = computed<DataTableColumns<any>>(() => [
	{
		title: 'HTTP 状态码',
		key: 'status',
		width: 120,
		render: (row) =>
			h(NTag, { size: 'small', type: row.status >= 500 ? 'error' : 'warning' }, { default: () => String(row.status) }),
	},
	{
		title: '业务错误码',
		key: 'code',
		width: 120,
		render: (row) => h('span', { class: 'font-mono text-xs text-gray-600' }, String(row.code)),
	},
	{
		title: '说明',
		key: 'desc',
		width: 240,
		render: (row) => h('span', { class: 'text-sm text-gray-700' }, row.desc),
	},
	{
		title: '示例消息',
		key: 'example',
		width: 240,
		render: (row) => h('span', { class: 'text-xs text-gray-400 font-mono' }, row.example),
	},
])
</script>

<template>
	<div class="docs-shell">
		<!-- ==================== 左侧目录（桌面） ==================== -->
		<aside class="docs-rail">
			<div class="docs-rail-card">
				<!-- 搜索 -->
				<div class="docs-rail-search">
					<Icon name="search" size="sm" class="docs-rail-search-icon" />
					<input
						v-model="railQuery"
						type="search"
						class="docs-rail-input"
						placeholder="搜索接口或路径"
						aria-label="搜索接口"
					/>
					<button
						v-if="railQuery"
						class="docs-rail-clear"
						aria-label="清空搜索"
						@click="railQuery = ''"
					>
						<Icon name="x" size="xs" />
					</button>
				</div>

				<!-- 目录树 -->
				<nav class="docs-rail-scroll" aria-label="文档目录">
					<p class="docs-rail-label">文档</p>
					<button
						v-for="section in sections.filter(s => s.id !== 'api-reference')"
						:key="section.id"
						class="docs-rail-item"
						:class="activeSection === section.id ? 'docs-rail-item-active' : ''"
						:aria-current="activeSection === section.id ? 'page' : undefined"
						@click="openSection(section.id)"
					>
						<Icon :name="section.icon" size="sm" class="docs-rail-item-icon" />
						<span class="truncate">{{ section.label }}</span>
					</button>

					<div class="docs-rail-group">
						<button
							class="docs-rail-item docs-rail-item-strong"
							:class="activeSection === 'api-reference' && !activeEndpoint ? 'docs-rail-item-active' : ''"
							@click="openOverview"
						>
							<Icon name="bookOpen" size="sm" class="docs-rail-item-icon" />
							<span class="truncate">API 参考</span>
							<span class="docs-rail-count">{{ endpointCount }}</span>
						</button>

						<div v-for="cat in filteredCategories" :key="cat.id" class="docs-rail-cat">
							<button
								class="docs-rail-cat-head"
								:aria-expanded="isCatOpen(cat.id)"
								@click="toggleCategory(cat.id)"
							>
								<Icon
									name="chevronDown"
									size="xs"
									class="docs-rail-caret"
									:class="{ 'docs-rail-caret-closed': !isCatOpen(cat.id) }"
								/>
								<span class="truncate">{{ cat.label }}</span>
								<span class="docs-rail-count">{{ cat.endpoints.length }}</span>
							</button>
							<div v-show="isCatOpen(cat.id)" class="docs-rail-eps">
								<button
									v-for="ep in cat.endpoints"
									:key="ep.id"
									class="docs-rail-ep"
									:class="activeEndpoint === ep.id ? 'docs-rail-ep-active' : ''"
									:aria-current="activeEndpoint === ep.id ? 'true' : undefined"
									:title="ep.path"
									@click="openEndpoint(ep.id)"
								>
									<span class="docs-rail-method" :class="methodTone[ep.method]">{{ ep.method }}</span>
									<span class="docs-rail-ep-title">{{ ep.title }}</span>
								</button>
							</div>
						</div>

						<p v-if="filteredCategories.length === 0" class="docs-rail-empty">
							没有匹配的接口
						</p>
					</div>
				</nav>

				<!-- Base URL -->
				<div class="docs-rail-foot">
					<span class="docs-rail-foot-label">Base URL</span>
					<button class="docs-rail-base" :title="apiBaseUrl" @click="copyBaseUrl">
						<code>{{ apiBaseUrl }}</code>
						<Icon :name="copiedBase ? 'check' : 'copy'" size="xs" />
					</button>
				</div>
			</div>
		</aside>

		<!-- ==================== 移动端目录抽屉 ==================== -->
		<Transition name="fade">
			<div v-if="railOpen" class="docs-drawer-mask" @click="railOpen = false"></div>
		</Transition>
		<Transition name="docs-drawer">
			<div v-if="railOpen" class="docs-drawer" role="dialog" aria-label="文档目录">
				<div class="docs-drawer-head">
					<span class="text-sm font-semibold text-gray-900">文档目录</span>
					<button class="btn-icon btn-ghost" aria-label="关闭目录" @click="railOpen = false">
						<Icon name="x" size="md" />
					</button>
				</div>
				<div class="docs-drawer-body">
					<div class="docs-rail-search docs-drawer-search">
						<Icon name="search" size="sm" class="docs-rail-search-icon" />
						<input
							v-model="railQuery"
							type="search"
							class="docs-rail-input"
							placeholder="搜索接口或路径"
							aria-label="搜索接口"
						/>
						<button v-if="railQuery" class="docs-rail-clear" aria-label="清空搜索" @click="railQuery = ''">
							<Icon name="x" size="xs" />
						</button>
					</div>

					<p class="docs-rail-label">文档</p>
					<button
						v-for="section in sections.filter(s => s.id !== 'api-reference')"
						:key="section.id"
						class="docs-rail-item"
						:class="activeSection === section.id ? 'docs-rail-item-active' : ''"
						@click="openSection(section.id)"
					>
						<Icon :name="section.icon" size="sm" class="docs-rail-item-icon" />
						<span class="truncate">{{ section.label }}</span>
					</button>

					<div class="docs-rail-group mt-3">
						<button
							class="docs-rail-item docs-rail-item-strong"
							:class="activeSection === 'api-reference' && !activeEndpoint ? 'docs-rail-item-active' : ''"
							@click="openOverview"
						>
							<Icon name="bookOpen" size="sm" class="docs-rail-item-icon" />
							<span class="truncate">API 参考</span>
							<span class="docs-rail-count">{{ endpointCount }}</span>
						</button>

						<div v-for="cat in filteredCategories" :key="cat.id" class="docs-rail-cat">
							<button
								class="docs-rail-cat-head"
								:aria-expanded="isCatOpen(cat.id)"
								@click="toggleCategory(cat.id)"
							>
								<Icon
									name="chevronDown"
									size="xs"
									class="docs-rail-caret"
									:class="{ 'docs-rail-caret-closed': !isCatOpen(cat.id) }"
								/>
								<span class="truncate">{{ cat.label }}</span>
								<span class="docs-rail-count">{{ cat.endpoints.length }}</span>
							</button>
							<div v-show="isCatOpen(cat.id)" class="docs-rail-eps">
								<button
									v-for="ep in cat.endpoints"
									:key="ep.id"
									class="docs-rail-ep"
									:class="activeEndpoint === ep.id ? 'docs-rail-ep-active' : ''"
									@click="openEndpoint(ep.id)"
								>
									<span class="docs-rail-method" :class="methodTone[ep.method]">{{ ep.method }}</span>
									<span class="docs-rail-ep-title">{{ ep.title }}</span>
								</button>
							</div>
						</div>

						<p v-if="filteredCategories.length === 0" class="docs-rail-empty">没有匹配的接口</p>
					</div>
				</div>
			</div>
		</Transition>

		<!-- ==================== 主内容 ==================== -->
		<div class="docs-main">
			<h1 class="sr-only">API 文档</h1>

			<!-- 顶部工具条：面包屑 + 操作。标题已在全局顶栏出现，这里不再重复大标题 -->
			<div class="docs-topbar">
				<nav class="docs-crumb" aria-label="当前位置">
					<button type="button" class="docs-crumb-link" @click="openSection('quickstart')">API 文档</button>
					<span class="docs-crumb-sep" aria-hidden="true">/</span>
					<span class="docs-crumb-item">{{ currentSectionLabel }}</span>
					<template v-if="activeEndpointDetail">
						<span class="docs-crumb-sep" aria-hidden="true">/</span>
						<span class="docs-crumb-item docs-crumb-current">{{ activeEndpointDetail.title }}</span>
					</template>
				</nav>
				<div class="docs-topbar-actions">
					<button class="docs-mobile-toc btn btn-secondary btn-sm" @click="railOpen = true">
						<Icon name="menu" size="sm" />
						目录
					</button>
					<button class="btn btn-primary btn-sm" @click="goToPlayground">
						<Icon name="play" size="sm" />
						在线测试
					</button>
				</div>
			</div>

			<!-- ==================== Quick Start ==================== -->
			<div v-if="activeSection === 'quickstart'" class="docs-stack">
				<!-- 接入信息 -->
				<div class="docs-facts">
					<div class="docs-fact">
						<span class="docs-fact-label">Base URL</span>
						<button class="docs-fact-value docs-fact-copy" :title="apiBaseUrl" @click="copyBaseUrl">
							<code>{{ apiBaseUrl }}</code>
							<Icon :name="copiedBase ? 'check' : 'copy'" size="xs" />
						</button>
					</div>
					<div class="docs-fact">
						<span class="docs-fact-label">认证方式</span>
						<span class="docs-fact-value">Bearer API Key</span>
					</div>
					<div class="docs-fact">
						<span class="docs-fact-label">兼容协议</span>
						<span class="docs-fact-value">OpenAI · Claude · Gemini</span>
					</div>
				</div>

				<!-- Step 1 -->
				<div class="card">
					<div class="docs-step-head">
						<span class="docs-step-num">1</span>
						<div class="step-text">
							<h3 class="docs-step-title">获取 API Key</h3>
							<p class="docs-step-sub">密钥可限定模型与速率，请勿提交到版本控制</p>
						</div>
						<router-link to="/tenant/api-keys" class="btn btn-secondary btn-sm docs-step-action ml-auto shrink-0">
							<Icon name="key" size="sm" />
							管理密钥
						</router-link>
					</div>
					<div class="card-body !pt-0">
						<p class="text-sm leading-relaxed text-gray-600">
							在控制台的
							<router-link to="/tenant/api-keys" class="text-primary-600 hover:underline">API 密钥</router-link>
							页面创建新密钥。每个密钥可限定特定模型并配置速率限制。请妥善保管——不要分享或提交到版本控制中。
						</p>
					</div>
				</div>

				<!-- Step 2 -->
				<div class="card">
					<div class="docs-step-head">
						<span class="docs-step-num">2</span>
						<div class="step-text">
							<h3 class="docs-step-title">发起第一次请求</h3>
							<p class="docs-step-sub">兼容 OpenAI SDK，只需替换 base_url 与 API Key</p>
						</div>
					</div>
					<div class="card-body !pt-0">
						<div class="docs-code">
							<div class="docs-code-bar">
								<button
									v-for="tab in (['curl', 'python', 'node', 'go'] as CodeTab[])"
									:key="tab"
									@click="activeCodeTab = tab"
									class="docs-code-tab"
									:class="{ 'docs-code-tab-active': activeCodeTab === tab }"
								>
									{{ tabLabels[tab] }}
								</button>
								<div class="flex-1"></div>
								<button
									@click="copyText(getExampleCode(activeCodeTab), activeCodeTab)"
									class="docs-code-copy"
									:class="{ 'is-copied': copiedTab === activeCodeTab }"
								>
									<Icon :name="copiedTab === activeCodeTab ? 'check' : 'copy'" size="xs" />
									{{ copiedTab === activeCodeTab ? '已复制' : '复制' }}
								</button>
							</div>
							<pre class="code-block !rounded-none !rounded-b-xl"><code>{{ getExampleCode(activeCodeTab) }}</code></pre>
						</div>
					</div>
				</div>

				<!-- Step 3 -->
				<div class="card">
					<div class="docs-step-head">
						<span class="docs-step-num">3</span>
						<div class="step-text">
							<h3 class="docs-step-title">探索与测试</h3>
							<p class="docs-step-sub">40+ 模型，三大协议格式并行可用</p>
						</div>
					</div>
					<div class="card-body !pt-0">
						<p class="mb-4 text-sm leading-relaxed text-gray-600">
							使用 Playground 在线调试不同参数组合，或通过
							<code class="code">/v1/models</code>
							接口查询当前密钥可用的完整模型列表。
						</p>
						<div class="flex flex-wrap gap-2">
							<button class="btn btn-primary btn-sm" @click="goToPlayground">
								<Icon name="play" size="sm" />
								打开 Playground
							</button>
							<button class="btn btn-secondary btn-sm" @click="openOverview">
								查看 API 参考
								<Icon name="arrowRight" size="sm" />
							</button>
						</div>
					</div>
				</div>

				<!-- 兼容性 -->
				<div class="docs-compat">
					<div class="docs-compat-item">
						<Icon name="chat" size="md" class="docs-compat-icon" />
						<div>
							<div class="docs-compat-title">OpenAI 兼容</div>
							<div class="docs-compat-sub">Chat / Embeddings / Images / Audio</div>
							<code class="code text-xs">base_url = ".../v1"</code>
						</div>
					</div>
					<div class="docs-compat-item">
						<Icon name="star" size="md" class="docs-compat-icon" />
						<div>
							<div class="docs-compat-title">Claude 兼容</div>
							<div class="docs-compat-sub">Anthropic Messages API</div>
							<code class="code text-xs">base_url = ".../v1"</code>
						</div>
					</div>
					<div class="docs-compat-item">
						<Icon name="cube" size="md" class="docs-compat-icon" />
						<div>
							<div class="docs-compat-title">Gemini 兼容</div>
							<div class="docs-compat-sub">Google GenerateContent API</div>
							<code class="code text-xs">/v1beta/models/...</code>
						</div>
					</div>
				</div>
			</div>

			<!-- ==================== API Reference ==================== -->
			<div v-if="activeSection === 'api-reference'" class="docs-stack">
				<!-- 总览 -->
				<template v-if="!activeEndpointDetail">
					<p v-if="!railQuery.trim() || filteredEndpointCount > 0" class="docs-intro">
						<template v-if="railQuery.trim()">
							匹配 <strong>{{ filteredEndpointCount }}</strong> 个端点，共 {{ endpointCount }} 个。
						</template>
						<template v-else>
							共 <strong>{{ endpointCount }}</strong> 个端点。从左侧目录选择接口查看参数与示例，点击任意一行进入详情。
						</template>
					</p>
					<section v-for="cat in filteredCategories" :key="cat.id" class="docs-cat-block">
						<div class="docs-cat-bar">
							<Icon :name="categoryIcons[cat.id] || 'grid'" size="sm" class="docs-cat-icon" />
							<h3 class="docs-cat-title">{{ cat.label }}</h3>
							<span class="docs-cat-count">{{ cat.endpoints.length }}</span>
						</div>
						<div class="card docs-ep-list">
							<button
								v-for="ep in cat.endpoints"
								:key="ep.id"
								class="docs-ep-row"
								@click="openEndpoint(ep.id)"
							>
								<span class="docs-method" :class="methodTone[ep.method]">{{ ep.method }}</span>
								<span class="docs-ep-main">
									<span class="docs-ep-title">{{ ep.title }}</span>
									<code class="docs-ep-path">{{ ep.path }}</code>
									<span class="docs-ep-desc">{{ ep.desc }}</span>
								</span>
								<Icon name="chevronRight" size="sm" class="docs-ep-arrow" />
							</button>
						</div>
					</section>

					<div v-if="filteredCategories.length === 0" class="docs-empty">
						<Icon name="search" size="lg" />
						<p>没有匹配「{{ railQuery }}」的接口或路径</p>
						<button class="btn btn-secondary btn-sm" @click="railQuery = ''">清空搜索</button>
					</div>
				</template>

				<!-- 端点详情 -->
				<template v-else>
					<button class="docs-back" @click="openOverview">
						<Icon name="arrowLeft" size="xs" />
						全部接口
					</button>

					<article class="card docs-detail">
						<header class="docs-detail-head">
							<h2 class="docs-detail-title">{{ activeEndpointDetail.title }}</h2>
							<div class="docs-detail-path">
								<span class="docs-method" :class="methodTone[activeEndpointDetail.method]">{{ activeEndpointDetail.method }}</span>
								<code>{{ activeEndpointDetail.path }}</code>
								<button
									class="docs-detail-copy"
									:aria-label="'复制路径 ' + activeEndpointDetail.path"
									@click="copyBlock(activeEndpointDetail.path, 'path-' + activeEndpointDetail.id)"
								>
									<Icon :name="copiedBlock === 'path-' + activeEndpointDetail.id ? 'check' : 'copy'" size="xs" />
								</button>
							</div>
							<p class="docs-detail-desc">{{ activeEndpointDetail.desc }}</p>
						</header>

						<section v-if="activeEndpointDetail.params.length" class="docs-detail-section">
							<h4 class="docs-detail-label">请求参数</h4>
							<ResponsiveDataTable
								:show-pagination="false"
								:columns="paramColumns"
								:scroll-x="tableScrollX(paramColumns)"
								:data="activeEndpointDetail.params"
								:row-key="(row:any)=>row.field"
								card-title-key="field"
								card-badge-key="required"
								:card-fields="['type', { key: 'desc', full: true }]"
							/>
						</section>

						<section v-if="activeEndpointDetail.example" class="docs-detail-section docs-detail-example">
							<div class="docs-auth-hint">
								<Icon name="infoCircle" size="sm" />
								<span>
									所有请求需携带
									<code>Authorization: Bearer sk-your-api-key</code>
									头
								</span>
							</div>

							<div v-if="activeEndpointDetail.example.req">
								<div class="docs-code-caption">
									<span>请求示例</span>
									<button
										class="docs-code-copy"
										:class="{ 'is-copied': copiedBlock === 'req-' + activeEndpointDetail.id }"
										@click="copyBlock(activeEndpointDetail.example!.req!, 'req-' + activeEndpointDetail.id)"
									>
										<Icon :name="copiedBlock === 'req-' + activeEndpointDetail.id ? 'check' : 'copy'" size="xs" />
										{{ copiedBlock === 'req-' + activeEndpointDetail.id ? '已复制' : '复制' }}
									</button>
								</div>
								<pre class="code-block !text-xs"><code>{{ activeEndpointDetail.example.req }}</code></pre>
							</div>

							<div v-if="activeEndpointDetail.example.resp">
								<div class="docs-code-caption">
									<span>响应示例</span>
									<button
										class="docs-code-copy"
										:class="{ 'is-copied': copiedBlock === 'resp-' + activeEndpointDetail.id }"
										@click="copyBlock(activeEndpointDetail.example!.resp!, 'resp-' + activeEndpointDetail.id)"
									>
										<Icon :name="copiedBlock === 'resp-' + activeEndpointDetail.id ? 'check' : 'copy'" size="xs" />
										{{ copiedBlock === 'resp-' + activeEndpointDetail.id ? '已复制' : '复制' }}
									</button>
								</div>
								<pre class="code-block !text-xs"><code>{{ activeEndpointDetail.example.resp }}</code></pre>
							</div>
						</section>

						<footer class="docs-detail-foot">
							<span class="docs-detail-foot-note">
								<template v-if="activeEndpointDetail.method === 'POST'">可在 Playground 中直接调试该端点</template>
								<template v-else>该端点为只读查询，无需额外配置</template>
							</span>
							<button
								v-if="activeEndpointDetail.method === 'POST'"
								class="btn btn-ghost btn-sm"
								@click="goToPlayground"
							>
								<Icon name="play" size="sm" />
								在 Playground 中测试
							</button>
						</footer>
					</article>

					<!-- 上一个 / 下一个 -->
					<nav class="docs-pager" aria-label="接口翻页">
						<button v-if="prevEndpoint" class="docs-pager-btn" @click="openEndpoint(prevEndpoint.id)">
							<span class="docs-pager-dir"><Icon name="arrowLeft" size="xs" /> 上一个</span>
							<span class="docs-pager-name">{{ prevEndpoint.title }}</span>
						</button>
						<span v-else class="docs-pager-spacer"></span>
						<button v-if="nextEndpoint" class="docs-pager-btn docs-pager-btn-next" @click="openEndpoint(nextEndpoint.id)">
							<span class="docs-pager-dir">下一个 <Icon name="arrowRight" size="xs" /></span>
							<span class="docs-pager-name">{{ nextEndpoint.title }}</span>
						</button>
					</nav>
				</template>
			</div>

			<!-- ==================== Code Examples ==================== -->
			<div v-if="activeSection === 'code-examples'" class="docs-stack">
				<p class="docs-intro">
					Team API 兼容 OpenAI / Anthropic SDK，只需修改 <code class="code">base_url</code> 即可无缝切换。代码块可直接复制使用。
				</p>
				<div class="docs-examples">
					<div v-for="(ex, idx) in codeExamples" :key="ex.title" class="card docs-example">
						<button
							class="docs-example-head"
							:aria-expanded="openExample === idx"
							@click="toggleExample(idx)"
						>
							<Icon
								name="chevronDown"
								size="xs"
								class="docs-rail-caret"
								:class="{ 'docs-rail-caret-closed': openExample !== idx }"
							/>
							<span class="docs-example-title">{{ ex.title }}</span>
							<span class="docs-lang-badge">{{ ex.lang }}</span>
							<span class="flex-1"></span>
							<span
								class="docs-code-copy"
								:class="{ 'is-copied': copiedExample === idx }"
								role="button"
								tabindex="0"
								@click.stop="copyExampleCode(idx)"
								@keydown.enter.stop="copyExampleCode(idx)"
							>
								<Icon :name="copiedExample === idx ? 'check' : 'copy'" size="xs" />
								{{ copiedExample === idx ? '已复制' : '复制' }}
							</span>
						</button>
						<pre v-show="openExample === idx" class="code-block !rounded-none !rounded-b-xl"><code>{{ ex.code }}</code></pre>
					</div>
				</div>
			</div>

			<!-- ==================== Error Codes ==================== -->
			<div v-if="activeSection === 'error-codes'" class="docs-stack">
				<p class="docs-intro">
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
						<ResponsiveDataTable
							:show-pagination="false"
							:columns="errorColumns"
							:scroll-x="tableScrollX(errorColumns)"
							:data="errorCodes"
							:row-key="(row:any)=>row.code"
							card-title-key="code"
							card-badge-key="status"
							:card-fields="[{ key: 'desc', full: true }, { key: 'example', full: true }]"
						/>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
/* ============================================================
   API 文档页布局：左侧目录 + 右侧内容
   ============================================================ */
.docs-shell {
	display: flex;
	/* 不能设 align-items: flex-start：目录列需要拉伸到与正文等高，内部 sticky 才能在整页滚动时持续吸附 */
	align-items: stretch;
	gap: 1.5rem;
}

/* ---------- 左侧目录 ---------- */
/*
 * 全局侧栏已占 ~296px，目录仅在 ≥1280px 常驻；
 * 更窄的视口改用顶部「目录」按钮唤出抽屉，避免三栏把正文压到 400px 以内。
 */
.docs-rail {
	display: none;
	width: 264px;
	flex-shrink: 0;
}

@media (min-width: 1280px) {
	.docs-rail {
		display: block;
	}
}

.docs-rail-card {
	position: sticky;
	/* 滚动容器是 #main-content（已在全局顶栏之下），top 是相对它算的，
	   不能沿用整页滚动的 5.5rem —— 那会把卡片额外推下 80px，顶部留一大片空白 */
	top: 0.75rem;
	display: flex;
	max-height: calc(100vh - 7rem);
	flex-direction: column;
	overflow: hidden;
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 1rem;
	background: rgba(255, 255, 255, 0.82);
	box-shadow: 0 10px 34px rgba(91, 104, 151, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.9);
	backdrop-filter: blur(12px);
}

.docs-rail-search {
	position: relative;
	display: flex;
	flex-shrink: 0;
	align-items: center;
	gap: 0.4rem;
	margin: 0.75rem 0.75rem 0.25rem;
	padding: 0 0.6rem;
	height: 2.25rem;
	border: 1px solid #e2e8f0;
	border-radius: 0.7rem;
	background: #f8fafc;
	color: #94a3b8;
	transition: border-color 160ms ease, background-color 160ms ease;
}

.docs-rail-search:focus-within {
	border-color: var(--color-primary-400);
	background: #fff;
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.12);
}

.docs-rail-search-icon {
	flex-shrink: 0;
	color: #94a3b8;
}

.docs-rail-input {
	width: 100%;
	min-width: 0;
	background: transparent;
	font-size: 0.8125rem;
	color: #334155;
	outline: none;
}

.docs-rail-input::placeholder {
	color: #a8b3c4;
}

.docs-rail-input::-webkit-search-cancel-button {
	display: none;
}

.docs-rail-clear {
	display: inline-flex;
	height: 1.25rem;
	width: 1.25rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 9999px;
	color: #94a3b8;
}

.docs-rail-clear:hover {
	background: #e2e8f0;
	color: #475569;
}

.docs-rail-scroll {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
	padding: 0.5rem 0.6rem 0.75rem;
	overscroll-behavior: contain;
}

.docs-rail-scroll::-webkit-scrollbar {
	width: 6px;
}

.docs-rail-scroll::-webkit-scrollbar-thumb {
	border-radius: 9999px;
	background: #dbe3ee;
}

.docs-rail-label {
	margin: 0.35rem 0.5rem 0.35rem;
	font-size: 0.6875rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: #94a3b8;
}

.docs-rail-item {
	display: flex;
	width: 100%;
	align-items: center;
	gap: 0.5rem;
	border-radius: 0.6rem;
	padding: 0.4rem 0.5rem;
	font-size: 0.8125rem;
	font-weight: 500;
	color: #64748b;
	text-align: left;
	transition: background-color 140ms ease, color 140ms ease;
}

.docs-rail-item:hover {
	background: #f1f5f9;
	color: #1e293b;
}

.docs-rail-item-icon {
	flex-shrink: 0;
	color: #94a3b8;
}

.docs-rail-item-active {
	background: var(--color-primary-50);
	color: var(--color-primary-700);
	font-weight: 600;
}

.docs-rail-item-active .docs-rail-item-icon {
	color: var(--color-primary-600);
}

.docs-rail-item-strong {
	margin-top: 0.15rem;
	color: #334155;
	font-weight: 600;
}

.docs-rail-count {
	margin-left: auto;
	border-radius: 9999px;
	background: #f1f5f9;
	padding: 0 0.35rem;
	font-size: 0.625rem;
	font-weight: 700;
	color: #94a3b8;
}

.docs-rail-item-active .docs-rail-count {
	background: #fff;
	color: var(--color-primary-600);
}

.docs-rail-group {
	margin-top: 0.5rem;
	border-top: 1px solid #eef2f7;
	padding-top: 0.5rem;
}

.docs-rail-cat {
	margin-top: 0.15rem;
}

.docs-rail-cat-head {
	display: flex;
	width: 100%;
	align-items: center;
	gap: 0.35rem;
	border-radius: 0.5rem;
	padding: 0.35rem 0.5rem 0.35rem 0.75rem;
	font-size: 0.75rem;
	font-weight: 600;
	color: #475569;
	text-align: left;
}

.docs-rail-cat-head:hover {
	background: #f8fafc;
	color: #0f172a;
}

.docs-rail-caret {
	flex-shrink: 0;
	color: #b6c2d2;
	transition: transform 180ms ease;
}

.docs-rail-caret-closed {
	transform: rotate(-90deg);
}

.docs-rail-eps {
	margin-left: 0.75rem;
	border-left: 1px solid #eef2f7;
	padding-left: 0.35rem;
}

.docs-rail-ep {
	display: flex;
	width: 100%;
	align-items: baseline;
	gap: 0.4rem;
	border-radius: 0.5rem;
	padding: 0.3rem 0.45rem;
	font-size: 0.78125rem;
	color: #64748b;
	text-align: left;
	transition: background-color 140ms ease, color 140ms ease;
}

.docs-rail-ep:hover {
	background: #f1f5f9;
	color: #1e293b;
}

.docs-rail-ep-active {
	background: var(--color-primary-50);
	color: var(--color-primary-700);
	font-weight: 600;
}

.docs-rail-method {
	flex-shrink: 0;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.5625rem;
	font-weight: 800;
	letter-spacing: 0.02em;
}

.docs-rail-ep-title {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.docs-rail-empty {
	padding: 0.75rem 0.6rem;
	font-size: 0.75rem;
	color: #94a3b8;
}

.docs-rail-foot {
	flex-shrink: 0;
	border-top: 1px solid #eef2f7;
	padding: 0.6rem 0.75rem 0.7rem;
}

.docs-rail-foot-label {
	display: block;
	margin-bottom: 0.25rem;
	font-size: 0.625rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: #a8b3c4;
}

.docs-rail-base {
	display: flex;
	width: 100%;
	align-items: center;
	gap: 0.35rem;
	border-radius: 0.5rem;
	padding: 0.25rem 0.35rem;
	color: #475569;
}

.docs-rail-base:hover {
	background: #f1f5f9;
	color: var(--color-primary-700);
}

.docs-rail-base code {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.6875rem;
	text-align: left;
}

/* method 语义色 */
.method-post { color: #059669; }
.method-get { color: #0284c7; }
.method-put { color: #d97706; }
.method-delete { color: #dc2626; }

/* ---------- 移动端抽屉 ---------- */
.docs-drawer-mask {
	position: fixed;
	inset: 0;
	z-index: 40;
	background: rgba(15, 23, 42, 0.28);
	backdrop-filter: blur(2px);
}

.docs-drawer {
	position: fixed;
	top: 0;
	bottom: 0;
	left: 0;
	z-index: 41;
	display: flex;
	width: min(84vw, 320px);
	flex-direction: column;
	background: #fff;
	box-shadow: 12px 0 40px rgba(15, 23, 42, 0.16);
}

.docs-drawer-head {
	display: flex;
	flex-shrink: 0;
	align-items: center;
	justify-content: space-between;
	border-bottom: 1px solid #eef2f7;
	padding: 0.75rem 0.75rem 0.75rem 1rem;
}

.docs-drawer-body {
	flex: 1;
	min-height: 0;
	overflow-y: auto;
	padding: 0.5rem 0.75rem 2rem;
}

/* 抽屉自带内边距，搜索框不再叠一层外边距 */
.docs-drawer-search {
	margin: 0 0 0.75rem;
}

.docs-drawer-enter-active,
.docs-drawer-leave-active {
	transition: transform 220ms ease;
}

.docs-drawer-enter-from,
.docs-drawer-leave-to {
	transform: translateX(-100%);
}

.docs-mobile-toc {
	display: inline-flex;
}

@media (min-width: 1280px) {
	.docs-mobile-toc {
		display: none;
	}
}

/* 空状态 */
.docs-empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.5rem;
	border-radius: 1rem;
	border: 1px dashed #dbe3ee;
	background: rgba(255, 255, 255, 0.6);
	padding: 2.5rem 1rem;
	color: #94a3b8;
}

.docs-empty p {
	font-size: 0.8125rem;
}

/* ---------- 主内容 ---------- */
.docs-main {
	flex: 1;
	min-width: 0;
}

/* 顶部工具条：面包屑 + 操作，随内容滚动（左栏已提供吸顶导航，这里不需要再吸一层） */
.docs-topbar {
	margin-bottom: 1rem;
	padding-bottom: 0.75rem;
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	border-bottom: 1px solid rgba(226, 232, 240, 0.9);
}

.docs-crumb {
	display: flex;
	align-items: center;
	gap: 0.4rem;
	min-width: 0;
	font-size: 0.8125rem;
	line-height: 1.4;
}

.docs-crumb-link {
	flex-shrink: 0;
	font-weight: 500;
	color: #64748b;
	transition: color 160ms ease;
}

.docs-crumb-link:hover {
	color: var(--color-primary-600);
}

.docs-crumb-sep {
	flex-shrink: 0;
	color: #cbd5e1;
}

.docs-crumb-item {
	font-weight: 600;
	color: #334155;
	white-space: nowrap;
}

.docs-crumb-current {
	overflow: hidden;
	text-overflow: ellipsis;
	font-weight: 700;
	color: #0f172a;
}

.docs-topbar-actions {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	flex-shrink: 0;
}

.docs-stack {
	display: flex;
	flex-direction: column;
	gap: 1rem;
}

.docs-intro {
	font-size: 0.8125rem;
	line-height: 1.7;
	color: #64748b;
}

.docs-intro strong {
	font-weight: 700;
	color: #0f172a;
}

/* 快速入门：信息条 */
.docs-facts {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
	gap: 0.75rem;
}

.docs-fact {
	display: flex;
	flex-direction: column;
	gap: 0.25rem;
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 0.9rem;
	background: rgba(255, 255, 255, 0.78);
	padding: 0.7rem 0.9rem;
	box-shadow: inset 0 1px 0 #fff;
}

.docs-fact-label {
	font-size: 0.625rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: #a8b3c4;
}

.docs-fact-value {
	font-size: 0.8125rem;
	font-weight: 600;
	color: #334155;
}

.docs-fact-copy {
	display: flex;
	align-items: center;
	gap: 0.4rem;
	text-align: left;
}

.docs-fact-copy code {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.75rem;
	font-weight: 500;
	color: var(--color-primary-700);
}

.docs-fact-copy:hover code {
	color: var(--color-primary-600);
	text-decoration: underline;
}

.docs-fact-copy svg {
	flex-shrink: 0;
	color: #94a3b8;
}

/* 步骤 */
.docs-step-head {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.75rem;
	padding: 1rem 1.25rem 0.5rem;
}

/* 文案区块占据剩余宽度，右侧按钮在窄屏换行而不是挤压标题 */
.docs-step-head > .step-text {
	flex: 1 1 auto;
	min-width: 0;
}

/* 窄屏下正文里已有「API 密钥」链接，头部按钮隐藏，避免换行后留下孤行 */
.docs-step-action {
	display: none;
}

@media (min-width: 768px) {
	.docs-step-action {
		display: inline-flex;
	}
}

.docs-step-num {
	display: inline-flex;
	height: 1.6rem;
	width: 1.6rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 9999px;
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	font-size: 0.75rem;
	font-weight: 700;
	color: #fff;
	box-shadow: 0 6px 14px rgba(20, 184, 166, 0.28);
}

.docs-step-title {
	font-size: 0.9375rem;
	font-weight: 600;
	line-height: 1.3;
	color: #0f172a;
}

.docs-step-sub {
	margin-top: 0.1rem;
	font-size: 0.75rem;
	color: #94a3b8;
}

/* 代码块外壳 */
.docs-code {
	overflow: hidden;
	border-radius: 0.75rem;
	border: 1px solid #e8edf4;
	background: #0f172a;
}

.docs-code-bar {
	display: flex;
	align-items: center;
	gap: 0.15rem;
	padding: 0.25rem 0.5rem;
	border-bottom: 1px solid rgba(255, 255, 255, 0.08);
	background: #111c31;
}

.docs-code-tab {
	border-radius: 0.5rem;
	padding: 0.35rem 0.7rem;
	font-size: 0.6875rem;
	font-weight: 600;
	color: #94a3b8;
	transition: background-color 140ms ease, color 140ms ease;
}

.docs-code-tab:hover {
	color: #e2e8f0;
	background: rgba(255, 255, 255, 0.06);
}

.docs-code-tab-active {
	background: rgba(20, 184, 166, 0.16);
	color: #5eead4;
}

.docs-code-copy {
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
	border-radius: 0.45rem;
	padding: 0.25rem 0.5rem;
	font-size: 0.6875rem;
	font-weight: 600;
	color: #94a3b8;
	transition: background-color 140ms ease, color 140ms ease;
}

.docs-code-copy:hover {
	background: rgba(255, 255, 255, 0.08);
	color: #e2e8f0;
}

.docs-code-copy.is-copied {
	color: #34d399;
}

/* 兼容性 */
.docs-compat {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
	gap: 0.75rem;
}

.docs-compat-item {
	display: flex;
	gap: 0.75rem;
	border-radius: 0.9rem;
	border: 1px solid rgba(20, 184, 166, 0.16);
	background: linear-gradient(135deg, rgba(240, 253, 250, 0.9), rgba(236, 254, 255, 0.75));
	padding: 0.9rem 1rem;
}

.docs-compat-icon {
	flex-shrink: 0;
	color: var(--color-primary-600);
}

.docs-compat-title {
	font-size: 0.8125rem;
	font-weight: 600;
	color: #0f172a;
}

.docs-compat-sub {
	margin: 0.1rem 0 0.4rem;
	font-size: 0.6875rem;
	color: #64748b;
	line-height: 1.5;
}

/* 概览：分类 */
.docs-cat-block {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.docs-cat-bar {
	display: flex;
	align-items: center;
	gap: 0.4rem;
}

.docs-cat-icon {
	color: var(--color-primary-600);
}

.docs-cat-title {
	font-size: 0.875rem;
	font-weight: 600;
	color: #0f172a;
}

.docs-cat-count {
	border-radius: 9999px;
	background: #eef2f7;
	padding: 0 0.4rem;
	font-size: 0.625rem;
	font-weight: 700;
	color: #94a3b8;
}

.docs-ep-list {
	padding: 0.25rem;
	overflow: hidden;
}

.docs-ep-row {
	display: flex;
	width: 100%;
	align-items: flex-start;
	gap: 0.75rem;
	border-radius: 0.7rem;
	padding: 0.6rem 0.75rem;
	text-align: left;
	transition: background-color 140ms ease;
}

.docs-ep-row + .docs-ep-row {
	border-top: 1px solid #f4f7fb;
}

.docs-ep-row:hover {
	background: #f8fafc;
}

.docs-ep-row:hover .docs-ep-arrow {
	color: var(--color-primary-600);
	transform: translateX(2px);
}

.docs-method {
	margin-top: 0.1rem;
	flex-shrink: 0;
	border-radius: 0.35rem;
	padding: 0.1rem 0.35rem;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.625rem;
	font-weight: 800;
	background: #f1f5f9;
}

.method-post.docs-method { background: #ecfdf5; }
.method-get.docs-method { background: #eff6ff; }
.method-put.docs-method { background: #fffbeb; }
.method-delete.docs-method { background: #fef2f2; }

.docs-ep-main {
	display: flex;
	min-width: 0;
	flex: 1;
	flex-direction: column;
	gap: 0.15rem;
}

.docs-ep-title {
	font-size: 0.8125rem;
	font-weight: 600;
	color: #1e293b;
}

.docs-ep-path {
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.6875rem;
	color: #94a3b8;
}

.docs-ep-desc {
	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
	font-size: 0.75rem;
	line-height: 1.55;
	color: #64748b;
}

.docs-ep-arrow {
	margin-top: 0.15rem;
	flex-shrink: 0;
	color: #cbd5e1;
	transition: color 140ms ease, transform 140ms ease;
}

/* 详情 */
.docs-back {
	display: inline-flex;
	width: fit-content;
	align-items: center;
	gap: 0.3rem;
	font-size: 0.75rem;
	font-weight: 600;
	color: #64748b;
}

.docs-back:hover {
	color: var(--color-primary-600);
}

.docs-detail {
	overflow: hidden;
	padding: 0;
}

.docs-detail-head {
	border-bottom: 1px solid #f1f5f9;
	padding: 1.1rem 1.25rem 1rem;
}

.docs-detail-title {
	font-size: 1.125rem;
	font-weight: 700;
	line-height: 1.35;
	color: #0f172a;
}

.docs-detail-path {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	margin-top: 0.5rem;
	flex-wrap: wrap;
}

.docs-detail-path code {
	border-radius: 0.4rem;
	background: #f8fafc;
	padding: 0.15rem 0.45rem;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.75rem;
	color: #475569;
}

.docs-detail-copy {
	display: inline-flex;
	height: 1.5rem;
	width: 1.5rem;
	align-items: center;
	justify-content: center;
	border-radius: 0.4rem;
	color: #94a3b8;
}

.docs-detail-copy:hover {
	background: #f1f5f9;
	color: var(--color-primary-600);
}

.docs-detail-desc {
	margin-top: 0.6rem;
	font-size: 0.8125rem;
	line-height: 1.75;
	color: #64748b;
}

.docs-detail-section {
	border-bottom: 1px solid #f1f5f9;
	padding: 1rem 1.25rem;
}

.docs-detail-label {
	margin-bottom: 0.6rem;
	font-size: 0.6875rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: #94a3b8;
}

.docs-detail-example {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
}

.docs-auth-hint {
	display: flex;
	align-items: flex-start;
	gap: 0.35rem;
	border-radius: 0.7rem;
	background: var(--color-primary-50);
	padding: 0.55rem 0.7rem;
	font-size: 0.75rem;
	line-height: 1.6;
	color: var(--color-primary-700);
}

.docs-auth-hint svg {
	flex-shrink: 0;
	margin-top: 0.1rem;
}

.docs-auth-hint code {
	border-radius: 0.3rem;
	background: rgba(255, 255, 255, 0.7);
	padding: 0 0.25rem;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.6875rem;
}

.docs-code-caption {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 0.35rem;
}

.docs-code-caption > span {
	font-size: 0.6875rem;
	font-weight: 600;
	letter-spacing: 0.04em;
	color: #94a3b8;
}

.docs-code-caption .docs-code-copy {
	color: #94a3b8;
}

.docs-code-caption .docs-code-copy:hover {
	background: #f1f5f9;
	color: var(--color-primary-600);
}

.docs-code-caption .docs-code-copy.is-copied {
	color: #059669;
}

.docs-detail-foot {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 0.75rem;
	flex-wrap: wrap;
	padding: 0.7rem 1.25rem;
	background: #fbfcfe;
}

.docs-detail-foot-note {
	font-size: 0.6875rem;
	color: #94a3b8;
}

/* 翻页 */
.docs-pager {
	display: flex;
	align-items: stretch;
	gap: 0.75rem;
}

.docs-pager-btn {
	display: flex;
	flex: 1;
	min-width: 0;
	flex-direction: column;
	gap: 0.15rem;
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 0.85rem;
	background: rgba(255, 255, 255, 0.78);
	padding: 0.6rem 0.85rem;
	text-align: left;
	box-shadow: inset 0 1px 0 #fff;
	transition: border-color 160ms ease, transform 160ms ease;
}

.docs-pager-btn:hover {
	border-color: var(--color-primary-200);
	transform: translateY(-1px);
}

.docs-pager-btn-next {
	text-align: right;
	align-items: flex-end;
}

.docs-pager-dir {
	display: inline-flex;
	align-items: center;
	gap: 0.2rem;
	font-size: 0.625rem;
	font-weight: 700;
	letter-spacing: 0.04em;
	color: #94a3b8;
}

.docs-pager-name {
	max-width: 100%;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 0.78125rem;
	font-weight: 600;
	color: #334155;
}

.docs-pager-spacer {
	flex: 1;
}

/* 代码示例 */
.docs-examples {
	display: flex;
	flex-direction: column;
	gap: 0.6rem;
}

.docs-example {
	overflow: hidden;
	padding: 0;
}

.docs-example-head {
	display: flex;
	width: 100%;
	align-items: center;
	gap: 0.5rem;
	padding: 0.6rem 0.9rem;
	text-align: left;
}

.docs-example-head:hover {
	background: #fbfcfe;
}

.docs-example-title {
	font-size: 0.8125rem;
	font-weight: 600;
	color: #1e293b;
}

.docs-lang-badge {
	border-radius: 9999px;
	background: #f1f5f9;
	padding: 0 0.4rem;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 0.625rem;
	font-weight: 600;
	color: #94a3b8;
}

.docs-example .docs-code-copy {
	color: #94a3b8;
}

.docs-example .docs-code-copy:hover {
	background: #f1f5f9;
	color: var(--color-primary-600);
}

.docs-example .docs-code-copy.is-copied {
	color: #059669;
}
</style>

