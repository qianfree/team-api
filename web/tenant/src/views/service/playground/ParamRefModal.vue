<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'

export interface ParamDef {
	name: string
	type: string
	required: boolean
	default?: string
	values?: string[]
	description: string
}

export interface ModelParamGroup {
	key: string
	label: string
	params: ParamDef[]
}

const props = defineProps<{
	show: boolean
	mode: 'image' | 'video'
}>()

const emit = defineEmits<{
	close: []
}>()

const activeGroup = ref('')

// ── 图像模型参数参考 ──

const imageGroups: ModelParamGroup[] = [
	{
		key: 'gemini-image',
		label: 'Gemini',
		params: [
			{ name: 'prompt', type: 'string', required: true, description: '图像描述文本，支持中英文，可描述画面内容、风格、构图等' },
			{ name: 'model', type: 'string', required: true, description: '模型 ID，如 gemini-2.5-flash-image、gemini-3-pro-image-preview' },
			{ name: 'n', type: 'integer', required: false, default: '1', values: ['1'], description: '生成图像数量，目前仅支持 1' },
			{ name: 'size', type: 'string', required: false, default: '1024x1024', values: ['1024x1024', '1536x1024', '1024x1536'], description: '图像尺寸（宽x高）。支持正方形和横/纵向' },
			{ name: 'response_format', type: 'string', required: false, default: 'b64_json', values: ['b64_json', 'url'], description: '返回格式。b64_json 返回 Base64 编码；url 返回临时下载链接' },
		],
	},
	{
		key: 'gpt-image',
		label: 'GPT Image',
		params: [
			{ name: 'prompt', type: 'string', required: true, description: '图像描述文本，最长 32000 字符' },
			{ name: 'model', type: 'string', required: true, description: '模型 ID：gpt-image-1、gpt-image-1-mini、gpt-image-1.5、gpt-image-2' },
			{ name: 'n', type: 'integer', required: false, default: '1', values: ['1', '2', '3', '4', '5', '6', '7', '8', '9', '10'], description: '生成图像数量，1-10' },
			{ name: 'size', type: 'string', required: false, default: '1024x1024', values: ['1024x1024', '1536x1024', '1024x1536', 'auto'], description: '图像尺寸。gpt-image-2 支持灵活分辨率（最大边 ≤ 3840px，16 的倍数）' },
			{ name: 'quality', type: 'string', required: false, default: 'auto', values: ['auto', 'high', 'medium', 'low'], description: '图像质量' },
			{ name: 'response_format', type: 'string', required: false, default: 'b64_json', values: ['b64_json', 'url'], description: '返回格式' },
			{ name: 'background', type: 'string', required: false, default: 'auto', values: ['auto', 'opaque', 'transparent'], description: '背景透明度。仅支持 PNG/WebP。gpt-image-2 不支持 transparent' },
			{ name: 'output_format', type: 'string', required: false, default: 'png', values: ['png', 'jpeg', 'webp'], description: '输出图像格式' },
			{ name: 'output_compression', type: 'integer', required: false, description: '输出压缩率 0-100，仅 jpeg/webp 有效。100 = 无压缩' },
			{ name: 'moderation', type: 'string', required: false, default: 'auto', values: ['auto', 'low'], description: '内容审核严格度' },
			{ name: 'partial_images', type: 'integer', required: false, default: '0', values: ['0', '1', '2', '3'], description: '流式中间图数量 0-3，设为非 0 自动启用流式' },
		],
	},
	{
		key: 'dall-e',
		label: 'DALL-E',
		params: [
			{ name: 'prompt', type: 'string', required: true, description: '图像描述文本。dall-e-2 最长 1000 字符，dall-e-3 最长 4000 字符' },
			{ name: 'model', type: 'string', required: true, description: '模型 ID：dall-e-2 或 dall-e-3' },
			{ name: 'n', type: 'integer', required: false, default: '1', values: ['1'], description: '生成数量，目前仅支持 1' },
			{ name: 'size', type: 'string', required: false, default: '1024x1024', values: ['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024'], description: 'dall-e-2: 256/512/1024 正方形；dall-e-3: 1024 正方形 + 1792 横纵' },
			{ name: 'quality', type: 'string', required: false, default: 'standard', values: ['standard', 'hd'], description: '仅 dall-e-3 支持。standard 标准，hd 高清' },
			{ name: 'response_format', type: 'string', required: false, default: 'url', values: ['url', 'b64_json'], description: '返回格式' },
			{ name: 'style', type: 'string', required: false, default: 'vivid', values: ['vivid', 'natural'], description: '仅 dall-e-3 支持。vivid 生动，natural 自然' },
		],
	},
]

// ── 视频模型参数参考（预留） ──

const videoGroups: ModelParamGroup[] = [
	{
		key: 'gemini-video',
		label: 'Gemini Veo',
		params: [
			{ name: 'prompt', type: 'string', required: true, description: '视频描述文本' },
			{ name: 'model', type: 'string', required: true, description: '模型 ID，如 veo-3.1-generate-preview' },
			{ name: 'resolution', type: 'string', required: false, default: '1080p', values: ['720p', '1080p', '4K'], description: '输出分辨率' },
			{ name: 'duration_seconds', type: 'integer', required: false, values: ['4', '6', '8'], description: '视频时长（秒）' },
			{ name: 'aspect_ratio', type: 'string', required: false, description: '视频宽高比' },
			{ name: 'negative_prompt', type: 'string', required: false, description: '排除的内容描述' },
			{ name: 'style', type: 'string', required: false, values: ['cinematic', 'creative'], description: '视觉风格' },
			{ name: 'seed', type: 'integer', required: false, description: '确定性生成的种子值' },
		],
	},
]

const groups = computed(() => props.mode === 'image' ? imageGroups : videoGroups)

// 自动选中第一个 group
watch(() => props.show, (val) => {
	if (val && !activeGroup.value) {
		activeGroup.value = groups.value[0]?.key || ''
	}
})

watch(groups, (val) => {
	if (val.length > 0 && !val.find(g => g.key === activeGroup.value)) {
		activeGroup.value = val[0].key
	}
})

const currentGroup = computed(() => groups.value.find(g => g.key === activeGroup.value))
</script>

<template>
	<BaseModal :show="show" title="参数参考" width="extra-wide" @close="emit('close')">
		<!-- 模型 Tab 栏 -->
		<div class="tabs mb-4">
			<button
				v-for="g in groups"
				:key="g.key"
				class="tab"
				:class="{ 'tab-active': activeGroup === g.key }"
				@click="activeGroup = g.key"
			>
				{{ g.label }}
			</button>
		</div>

		<!-- 参数表格 -->
		<div v-if="currentGroup" class="table-container">
			<table class="table">
				<thead>
					<tr>
						<th class="w-36">参数名</th>
						<th class="w-20">类型</th>
						<th class="w-14">必填</th>
						<th class="w-24">默认值</th>
						<th>可选值</th>
						<th>说明</th>
					</tr>
				</thead>
				<tbody>
					<tr v-for="p in currentGroup.params" :key="p.name">
						<td>
							<code class="text-xs font-mono text-primary-600 bg-primary-50 px-1.5 py-0.5 rounded">{{ p.name }}</code>
						</td>
						<td class="text-xs text-gray-500">{{ p.type }}</td>
						<td>
							<span v-if="p.required" class="text-xs font-medium text-red-500">是</span>
							<span v-else class="text-xs text-gray-400">否</span>
						</td>
						<td class="text-xs text-gray-500">{{ p.default || '—' }}</td>
						<td>
							<template v-if="p.values?.length">
								<div class="flex flex-wrap gap-1">
									<span v-for="v in p.values" :key="v" class="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">{{ v }}</span>
								</div>
							</template>
							<span v-else class="text-xs text-gray-400">—</span>
						</td>
						<td class="text-xs text-gray-600">{{ p.description }}</td>
					</tr>
				</tbody>
			</table>
		</div>

		<!-- 提示信息 -->
		<div class="mt-4 rounded-xl border border-amber-200 bg-amber-50/60 px-4 py-3">
			<div class="flex items-start gap-2">
				<Icon name="infoCircle" size="sm" class="text-amber-500 mt-0.5 flex-shrink-0" />
				<div class="text-xs text-amber-700 space-y-1">
					<p>以上参数通过 <code class="bg-amber-100 px-1 rounded">/v1/images/generations</code> 端点发送，使用自定义参数区域逐行添加。</p>
					<p>参数名为 JSON 键名，值为对应格式的字符串（系统会自动识别布尔值和数字）。</p>
				</div>
			</div>
		</div>
	</BaseModal>
</template>
