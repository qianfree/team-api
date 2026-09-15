<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import type { Attachment } from './useAttachments'

/**
 * 带 @ 提及的提示词输入框。
 *
 * 用原生 textarea 而非 n-input：插入标记要精确读写 selectionStart/selectionEnd，
 * 隔着组件库拿内部 DOM 既脆又绕。autosize 与样式在此自行实现，视觉对齐 .input。
 */

const props = withDefaults(defineProps<{
	modelValue: string
	attachments: Attachment[]
	/** 关闭后不响应 @，输入框退化为普通 textarea */
	mentions?: boolean
	placeholder?: string
	minRows?: number
	maxRows?: number
	disabled?: boolean
	/** 是否接受粘贴图片（对话/视频都开，纯文本场景可关） */
	pasteFiles?: boolean
	/**
	 * 去掉 textarea 自身的边框与内边距，交由父容器的边框统一承载
	 * （用于把附件栏 + 输入框 + 操作按钮整合成一个视觉整体的输入容器）
	 */
	bare?: boolean
}>(), {
	mentions: true,
	placeholder: '',
	minRows: 2,
	maxRows: 8,
	disabled: false,
	pasteFiles: true,
	bare: false,
})

const emit = defineEmits<{
	'update:modelValue': [value: string]
	submit: []
	paste: [files: File[]]
}>()

const textarea = ref<HTMLTextAreaElement | null>(null)
const panelOpen = ref(false)
const query = ref('')
const activeIndex = ref(0)

/** 触发条件：光标紧邻的 @ 后面跟着不含空白与方括号的片段 */
const TRIGGER_RE = /@([^\s@[\]]*)$/

const candidates = computed(() => {
	if (!query.value) return props.attachments
	const q = query.value.toLowerCase()
	return props.attachments.filter(
		a => a.name.toLowerCase().includes(q) || a.fileName.toLowerCase().includes(q),
	)
})

function resize() {
	const el = textarea.value
	if (!el) return
	el.style.height = 'auto'
	const lineHeight = 22
	const max = props.maxRows * lineHeight + 20
	el.style.height = `${Math.min(el.scrollHeight, max)}px`
}

watch(() => props.modelValue, () => nextTick(resize))

// 附件被删光时关掉可能还开着的候选面板
watch(() => props.attachments.length, len => {
	if (!len) closePanel()
})

function closePanel() {
	panelOpen.value = false
	query.value = ''
	activeIndex.value = 0
}

function syncPanel() {
	if (!props.mentions) return
	const el = textarea.value
	if (!el) return
	const before = props.modelValue.slice(0, el.selectionStart)
	const m = TRIGGER_RE.exec(before)
	if (!m) {
		closePanel()
		return
	}
	panelOpen.value = true
	query.value = m[1]
	activeIndex.value = 0
}

function onInput(e: Event) {
	emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
	nextTick(() => {
		resize()
		syncPanel()
	})
}

/** insertMention 把光标前的 @片段替换成 [@名称]；无触发片段时在光标处插入 */
function insertMention(name: string) {
	const el = textarea.value
	if (!el) return
	const token = `[@${name}]`
	// 从未聚焦过的输入框 selectionStart 恒为 0，插到开头很反直觉，改为追加到末尾
	const touched = el.selectionStart > 0 || el.selectionEnd > 0 || document.activeElement === el
	const pos = touched ? el.selectionStart : props.modelValue.length
	const end = touched ? el.selectionEnd : props.modelValue.length
	const before = props.modelValue.slice(0, pos)
	const after = props.modelValue.slice(end)
	const m = TRIGGER_RE.exec(before)
	const head = m ? before.slice(0, m.index) : before

	emit('update:modelValue', head + token + after)
	closePanel()
	nextTick(() => {
		const caret = head.length + token.length
		el.focus()
		el.setSelectionRange(caret, caret)
		resize()
	})
}

function onKeydown(e: KeyboardEvent) {
	if (panelOpen.value && candidates.value.length && !e.isComposing) {
		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault()
				activeIndex.value = (activeIndex.value + 1) % candidates.value.length
				return
			case 'ArrowUp':
				e.preventDefault()
				activeIndex.value = (activeIndex.value - 1 + candidates.value.length) % candidates.value.length
				return
			case 'Enter':
			case 'Tab':
				e.preventDefault()
				insertMention(candidates.value[activeIndex.value].name)
				return
			case 'Escape':
				e.preventDefault()
				closePanel()
				return
		}
	}
	// 面板未接管时，Enter（不带修饰键）提交，Shift+Enter 换行
	if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey && !e.isComposing) {
		e.preventDefault()
		emit('submit')
	}
}

function onPaste(e: ClipboardEvent) {
	if (!props.pasteFiles) return
	const files = Array.from(e.clipboardData?.files || [])
	if (!files.length) return
	e.preventDefault()
	emit('paste', files)
}

defineExpose({ insertMention, focus: () => textarea.value?.focus() })
</script>

<template>
	<div class="relative">
		<!-- @ 候选面板：锚在输入框上方（输入框贴底，向上展开不会被视口裁掉） -->
		<div
			v-if="panelOpen"
			class="absolute bottom-full left-0 z-30 mb-2 w-64 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg animate-scale-in"
		>
			<div class="border-b border-gray-100 px-3 py-1.5 text-xs font-medium text-gray-400">
				引用附件
			</div>
			<template v-if="candidates.length">
				<button
					v-for="(att, i) in candidates"
					:key="att.id"
					class="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors duration-150"
					:class="i === activeIndex ? 'bg-primary-50' : 'hover:bg-gray-50'"
					@mouseenter="activeIndex = i"
					@mousedown.prevent
					@click="insertMention(att.name)"
				>
					<img
						v-if="att.kind === 'image'"
						:src="att.previewUrl"
						class="h-7 w-7 shrink-0 rounded object-cover"
						alt=""
					/>
					<span v-else class="flex h-7 w-7 shrink-0 items-center justify-center rounded bg-gray-100 text-gray-500">
						<Icon :name="att.kind === 'audio' ? 'musicalNote' : 'film'" size="xs" />
					</span>
					<span class="min-w-0 flex-1">
						<span class="block text-sm text-gray-700">{{ att.name }}</span>
						<span class="block truncate text-xs text-gray-400">{{ att.fileName }}</span>
					</span>
				</button>
			</template>
			<div v-else class="px-3 py-3 text-xs text-gray-400">
				{{ attachments.length ? '没有匹配的附件' : '还没有附件，先在上方上传' }}
			</div>
		</div>

		<textarea
			ref="textarea"
			:value="modelValue"
			:placeholder="placeholder"
			:rows="minRows"
			:disabled="disabled"
			:class="bare
				? 'w-full resize-none border-0 bg-transparent px-1 py-0.5 text-sm leading-[22px] text-gray-900 placeholder:text-gray-400 focus:outline-none disabled:cursor-not-allowed disabled:text-gray-400'
				: 'input resize-none leading-[22px]'"
			@input="onInput"
			@keydown="onKeydown"
			@paste="onPaste"
			@click="syncPanel"
			@blur="closePanel"
		/>
	</div>
</template>
