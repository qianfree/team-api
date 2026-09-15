<script setup lang="ts">
import { ref, computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { formatBytes, type Attachment } from './useAttachments'

/**
 * 附件预览带：位于输入框上方，承载上传入口与已选附件的缩略展示。
 * 点击某个附件 = 在输入框光标处插入它的 [@名称] 标记（比打 @ 更快的路径）。
 */

const props = withDefaults(defineProps<{
	attachments: Attachment[]
	/** input 的 accept 属性，如 "image/*,audio/*" */
	accept: string
	max: number
	disabled?: boolean
	/** 禁用原因，作为上传按钮的 title 与旁注展示 */
	disabledReason?: string
	/**
	 * 视觉变体：
	 * - boxed（默认）：带虚线框的独立附件带，含上传按钮与提示文案
	 * - bare：只渲染附件缩略片，无边框无上传按钮，供父组件把输入区整合成一体时使用
	 *   （此变体下由父组件通过 ref 调用 pick() 触发选择文件）
	 */
	variant?: 'boxed' | 'bare'
}>(), {
	disabled: false,
	disabledReason: '',
	variant: 'boxed',
})

const emit = defineEmits<{
	add: [files: File[]]
	remove: [id: number]
	insert: [name: string]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)

const full = computed(() => props.attachments.length >= props.max)
const uploadDisabled = computed(() => props.disabled || full.value)

function pick() {
	if (uploadDisabled.value) return
	fileInput.value?.click()
}

function onPicked(e: Event) {
	const input = e.target as HTMLInputElement
	if (input.files?.length) emit('add', Array.from(input.files))
	// 置空 value，否则连续选同一个文件不会再触发 change
	input.value = ''
}

function onDrop(e: DragEvent) {
	dragging.value = false
	if (uploadDisabled.value) return
	const files = Array.from(e.dataTransfer?.files || [])
	if (files.length) emit('add', files)
}

function onDragOver() {
	if (!uploadDisabled.value) dragging.value = true
}

// bare 变体下不渲染上传按钮，选择文件由父组件调用此方法触发
defineExpose({ pick })
</script>

<template>
	<!-- bare：只渲染附件缩略片，无边框、无上传入口，融入父组件的输入容器 -->
	<div v-if="variant === 'bare'" class="flex flex-wrap items-center gap-1.5">
		<div
			v-for="att in attachments"
			:key="att.id"
			class="group relative flex items-center gap-1.5 rounded-lg border border-gray-200 bg-gray-50 py-0.5 pl-0.5 pr-6 transition-colors duration-150 hover:border-primary-300 hover:bg-primary-50/40"
		>
			<button
				class="flex items-center gap-1.5 cursor-pointer"
				:title="`${att.fileName} · 点击插入 [@${att.name}]`"
				@click="emit('insert', att.name)"
			>
				<img
					v-if="att.kind === 'image'"
					:src="att.previewUrl"
					class="h-6 w-6 rounded object-cover"
					alt=""
				/>
				<span v-else class="flex h-6 w-6 items-center justify-center rounded bg-white text-gray-500">
					<Icon :name="att.kind === 'audio' ? 'musicalNote' : 'film'" size="xs" />
				</span>
				<span class="flex items-baseline gap-1 leading-none">
					<span class="text-xs font-medium text-gray-700">{{ att.name }}</span>
					<span class="text-[10px] text-gray-400">{{ formatBytes(att.bytes) }}</span>
				</span>
			</button>
			<button
				class="absolute right-0.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-gray-300 transition-colors duration-150 hover:bg-white hover:text-red-500"
				title="移除"
				@click="emit('remove', att.id)"
			>
				<Icon name="x" size="xs" />
			</button>
		</div>

		<!-- 文件选择器仍需挂载，供父组件调 pick() 打开 -->
		<input
			ref="fileInput"
			type="file"
			multiple
			class="hidden"
			:accept="accept"
			@change="onPicked"
		/>
	</div>

	<div
		v-else
		class="rounded-xl border border-dashed px-3 py-2 transition-colors duration-150"
		:class="dragging ? 'border-primary-400 bg-primary-50/60' : 'border-gray-200 bg-gray-50/60'"
		@dragover.prevent="onDragOver"
		@dragleave="dragging = false"
		@drop.prevent="onDrop"
	>
		<div class="flex flex-wrap items-center gap-2">
			<!-- 已选附件 -->
			<div
				v-for="att in attachments"
				:key="att.id"
				class="group relative flex items-center gap-2 rounded-lg border border-gray-200 bg-white py-1 pl-1 pr-7 transition-colors duration-150 hover:border-primary-300"
			>
				<button
					class="flex items-center gap-2 cursor-pointer"
					:title="`${att.fileName} · 点击插入 [@${att.name}]`"
					@click="emit('insert', att.name)"
				>
					<img
						v-if="att.kind === 'image'"
						:src="att.previewUrl"
						class="h-8 w-8 rounded object-cover"
						alt=""
					/>
					<span v-else class="flex h-8 w-8 items-center justify-center rounded bg-gray-100 text-gray-500">
						<Icon :name="att.kind === 'audio' ? 'musicalNote' : 'film'" size="sm" />
					</span>
					<span class="flex flex-col items-start leading-tight">
						<span class="text-xs font-medium text-gray-700">{{ att.name }}</span>
						<span class="text-[11px] text-gray-400">{{ formatBytes(att.bytes) }}</span>
					</span>
				</button>
				<button
					class="absolute right-1 top-1 rounded p-0.5 text-gray-300 transition-colors duration-150 hover:bg-gray-100 hover:text-red-500"
					title="移除"
					@click="emit('remove', att.id)"
				>
					<Icon name="x" size="xs" />
				</button>
			</div>

			<!-- 上传入口 -->
			<button
				class="btn btn-secondary btn-sm"
				:disabled="uploadDisabled"
				:title="disabled ? disabledReason : (full ? `最多 ${max} 个附件` : '可拖拽文件或在输入框粘贴图片')"
				@click="pick"
			>
				<Icon name="paperclip" size="sm" />
				上传附件
			</button>

			<span v-if="disabled && disabledReason" class="text-xs text-amber-600">{{ disabledReason }}</span>
			<span v-else-if="!attachments.length" class="text-xs text-gray-400">
				可拖拽文件到此处，或在输入框直接粘贴图片
			</span>
		</div>

		<input
			ref="fileInput"
			type="file"
			multiple
			class="hidden"
			:accept="accept"
			@change="onPicked"
		/>
	</div>
</template>
