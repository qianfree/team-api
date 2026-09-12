<script setup lang="ts">
import { computed } from 'vue'
import { getVendorMeta, vendorLabel } from '@/constants/vendor'
import openaiLogo from '@/assets/vendors/openai.svg'
import anthropicLogo from '@/assets/vendors/anthropic.svg'
import googleLogo from '@/assets/vendors/google.svg'
import xaiLogo from '@/assets/vendors/xai.svg'
import mistralLogo from '@/assets/vendors/mistral.svg'
import cohereLogo from '@/assets/vendors/cohere.svg'
import metaLogo from '@/assets/vendors/meta.svg'
import alibabaLogo from '@/assets/vendors/alibaba.svg'
import bytedanceLogo from '@/assets/vendors/bytedance.svg'
import deepseekLogo from '@/assets/vendors/deepseek.svg'
import zhipuLogo from '@/assets/vendors/zhipu.svg'
import moonshotLogo from '@/assets/vendors/moonshot.svg'
import minimaxLogo from '@/assets/vendors/minimax.svg'
import baiduLogo from '@/assets/vendors/baidu.svg'
import tencentLogo from '@/assets/vendors/tencent.svg'
import xunfeiLogo from '@/assets/vendors/xunfei.svg'
import kuaishouLogo from '@/assets/vendors/kuaishou.svg'
import midjourneyLogo from '@/assets/vendors/midjourney.svg'
import sunoLogo from '@/assets/vendors/suno.svg'

// 厂商 slug → 本地白色单色 logo（多色 logo 见 constants/vendor.ts 的 colored 标记）
const logoMap: Record<string, string> = {
	openai: openaiLogo,
	anthropic: anthropicLogo,
	google: googleLogo,
	xai: xaiLogo,
	mistral: mistralLogo,
	cohere: cohereLogo,
	meta: metaLogo,
	alibaba: alibabaLogo,
	bytedance: bytedanceLogo,
	deepseek: deepseekLogo,
	zhipu: zhipuLogo,
	moonshot: moonshotLogo,
	minimax: minimaxLogo,
	baidu: baiduLogo,
	tencent: tencentLogo,
	xunfei: xunfeiLogo,
	kuaishou: kuaishouLogo,
	midjourney: midjourneyLogo,
	suno: sunoLogo,
}

const props = withDefaults(
	defineProps<{
		/** 厂商枚举值（空串/null = 未分类，渲染灰色兜底头像） */
		vendor?: string | null
		/** 是否附带厂商名称 */
		showName?: boolean
		size?: 'xs' | 'sm' | 'md'
	}>(),
	{ vendor: '', showName: false, size: 'sm' },
)

const meta = computed(() => getVendorMeta(props.vendor))
const logoSrc = computed(() => (meta.value ? logoMap[meta.value.value] ?? null : null))

// 未收录/未设置时的兜底首字：未收录取原值首字符大写，未设置显示 ?
const fallbackInitial = computed(() => {
	if (props.vendor) return props.vendor.slice(0, 1).toUpperCase()
	return '?'
})

const chipClass = computed(() => {
	const sizeClass = { xs: 'h-4 w-4 rounded-[5px]', sm: 'h-5 w-5 rounded-md', md: 'h-7 w-7 rounded-lg' }[props.size]
	const base = 'inline-flex shrink-0 items-center justify-center'
	// 多色 logo 用浅色底 + 细边框；未知/未设置用中性灰
	if (logoSrc.value && !meta.value?.colored) return `${base} ${sizeClass}`
	return `${base} ${sizeClass} border border-gray-200/80 bg-gray-100 text-gray-500`
})

const chipStyle = computed(() => {
	if (!logoSrc.value) return {}
	if (meta.value?.colored) return { backgroundColor: 'rgba(255, 255, 255, 0.92)' }
	return { backgroundColor: meta.value?.color }
})

const imgClass = computed(() => ({ xs: 'h-2.5 w-2.5', sm: 'h-3 w-3', md: 'h-4 w-4' }[props.size]))
const nameClass = computed(() => ({ xs: 'text-[10px]', sm: 'text-xs', md: 'text-sm' }[props.size]))
</script>

<template>
	<span class="inline-flex min-w-0 items-center gap-1.5" :title="showName ? undefined : vendorLabel(vendor)">
		<span :class="chipClass" :style="chipStyle">
			<img v-if="logoSrc" :src="logoSrc" :alt="vendorLabel(vendor)" class="object-contain" :class="imgClass" />
			<span v-else class="font-bold leading-none" :class="nameClass">{{ fallbackInitial }}</span>
		</span>
		<span v-if="showName" class="truncate font-medium text-gray-600" :class="nameClass">{{ vendorLabel(vendor) }}</span>
	</span>
</template>
