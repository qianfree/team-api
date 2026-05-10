<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'

const router = useRouter()
const activeSection = ref('quickstart')

const sections = [
	{ id: 'quickstart', label: '快速入门' },
	{ id: 'api-reference', label: 'API 参考' },
	{ id: 'code-examples', label: '代码示例' },
]

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

function getExampleCode(tab: CodeTab): string {
	switch (tab) {
		case 'curl': return curlExample
		case 'python': return pythonExample
		case 'node': return nodeExample
		case 'go': return goExample
	}
}

const tabLabels: Record<CodeTab, string> = {
	curl: 'cURL',
	python: 'Python',
	node: 'Node.js',
	go: 'Go',
}

const copiedTab = ref('')
async function copyCode(tab: CodeTab) {
	try {
		await navigator.clipboard.writeText(getExampleCode(tab))
		copiedTab.value = tab
		setTimeout(() => { copiedTab.value = '' }, 2000)
	} catch (e) { console.error(e) }
}

function goToPlayground() {
	router.push('/tenant/playground')
}

// API Reference endpoints
const endpoints = [
	{
		method: 'POST', path: '/v1/chat/completions', title: 'Chat Completions',
		desc: '创建聊天补全，兼容 OpenAI API 格式，支持流式和非流式响应。',
		params: [
			{ field: 'model', type: 'string', desc: '模型 ID（如 gpt-4o, claude-sonnet-4-20250514）', required: true },
			{ field: 'messages', type: 'array', desc: '消息数组，每项包含 role 和 content', required: true },
			{ field: 'stream', type: 'boolean', desc: '是否启用流式响应（默认 false）' },
			{ field: 'temperature', type: 'number', desc: '采样温度 0-2（默认 1）' },
			{ field: 'max_tokens', type: 'integer', desc: '最大生成 Token 数' },
			{ field: 'top_p', type: 'number', desc: '核采样概率（默认 1）' },
			{ field: 'frequency_penalty', type: 'number', desc: '频率惩罚 -2 ~ 2' },
			{ field: 'presence_penalty', type: 'number', desc: '存在惩罚 -2 ~ 2' },
			{ field: 'stop', type: 'array', desc: '停止生成的字符串序列' },
		],
	},
	{
		method: 'POST', path: '/v1/embeddings', title: 'Embeddings',
		desc: '创建文本向量嵌入，兼容 OpenAI API 格式。',
		params: [
			{ field: 'model', type: 'string', desc: '模型 ID（如 text-embedding-3-small）', required: true },
			{ field: 'input', type: 'string | array', desc: '要嵌入的文本或文本数组', required: true },
		],
	},
	{
		method: 'POST', path: '/v1/images/generations', title: 'Image Generations',
		desc: 'AI 图像生成，兼容 OpenAI API 格式。',
		params: [
			{ field: 'model', type: 'string', desc: '模型 ID（如 dall-e-3）', required: true },
			{ field: 'prompt', type: 'string', desc: '图像描述提示词', required: true },
			{ field: 'n', type: 'integer', desc: '生成数量（默认 1）' },
			{ field: 'size', type: 'string', desc: '图像尺寸' },
		],
	},
	{
		method: 'POST', path: '/v1/messages', title: 'Claude Messages',
		desc: 'Claude Messages API，兼容 Anthropic API 格式。',
		params: [
			{ field: 'model', type: 'string', desc: '模型 ID（如 claude-sonnet-4-20250514）', required: true },
			{ field: 'messages', type: 'array', desc: '消息数组', required: true },
			{ field: 'max_tokens', type: 'integer', desc: '最大 Token 数（默认 4096）' },
			{ field: 'stream', type: 'boolean', desc: '是否启用流式响应' },
		],
	},
	{
		method: 'GET', path: '/v1/models', title: 'List Models',
		desc: '获取可用模型列表。',
		params: [],
	},
	{
		method: 'GET', path: '/v1/models/{model_id}', title: 'Get Model',
		desc: '获取指定模型详情。',
		params: [
			{ field: 'model_id', type: 'string', desc: '模型 ID（路径参数）', required: true },
		],
	},
]

const codeExamples = [
	{ title: 'Python 流式聊天', lang: 'python', code: pythonExample },
	{ title: 'Node.js 流式调用', lang: 'javascript', code: nodeExample },
	{ title: 'Go 流式调用', lang: 'go', code: goExample },
]

const copiedExample = ref(-1)
async function copyExampleCode(idx: number) {
	try {
		await navigator.clipboard.writeText(codeExamples[idx].code)
		copiedExample.value = idx
		setTimeout(() => { copiedExample.value = -1 }, 2000)
	} catch (e) { console.error(e) }
}
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">API 文档</h1>
				<p class="page-description">集成 Team API 所需的一切</p>
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

		<!-- Quick Start Section -->
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
								@click="copyCode(activeCodeTab)"
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
						Team API 通过统一接口提供 40+ 个 AI 模型的访问。您可以使用 Playground 在线调试不同参数组合，或通过 <code class="code">/v1/models</code> 接口查询完整模型列表。
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
		</div>

		<!-- API Reference Section -->
		<div v-if="activeSection === 'api-reference'" class="space-y-6">
			<div v-for="ep in endpoints" :key="ep.path" class="card">
				<div class="card-header">
					<div class="flex items-center gap-3">
						<span
							class="badge text-xs font-bold"
							:class="ep.method === 'GET' ? 'badge-primary' : 'badge-success'"
						>{{ ep.method }}</span>
						<code class="code text-sm">{{ ep.path }}</code>
					</div>
					<h3 class="font-semibold text-gray-900 mt-2">{{ ep.title }}</h3>
					<p class="text-sm text-gray-500 mt-0.5">{{ ep.desc }}</p>
				</div>
				<div v-if="ep.params.length > 0" class="card-body">
					<h4 class="text-sm font-semibold text-gray-800 mb-2">请求参数</h4>
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
					<div v-if="ep.method === 'POST'" class="mt-4 flex justify-end">
						<button class="btn btn-ghost btn-sm" @click="goToPlayground">
							<Icon name="cog" size="sm" />
							在 Playground 中测试
						</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Code Examples Section -->
		<div v-if="activeSection === 'code-examples'" class="space-y-6">
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
	</div>
</template>
