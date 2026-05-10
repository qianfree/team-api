<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from 'vue'
import { useCaptcha } from '@/composables/useCaptcha'
import Icon from '@/components/common/Icon.vue'

const THUMB_SIZE = 44

const props = withDefaults(defineProps<{
	modelValue: { captchaKey: string; captchaX: number }
	centered?: boolean
}>(), {
	centered: false,
})

const emit = defineEmits<{
	'update:modelValue': [value: { captchaKey: string; captchaX: number }]
}>()

const { captchaData, loading, error, status, fetchCaptcha, verify, resetCaptcha } = useCaptcha()

function handleReset() {
	activated.value = false
	isDragging.value = false
	sliderLeft.value = 0
	resetCaptcha()
}
defineExpose({ resetCaptcha: handleReset, activate })

const triggerRef = ref<HTMLDivElement | null>(null)
const popupRef = ref<HTMLDivElement | null>(null)
const imageRef = ref<HTMLImageElement | null>(null)
const tileImgRef = ref<HTMLImageElement | null>(null)
const thumbRef = ref<HTMLDivElement | null>(null)

const activated = ref(false)
const isDragging = ref(false)
const startX = ref(0)
const startLeft = ref(0)
const sliderLeft = ref(0)
const tileNaturalSize = ref(0)
const imageNaturalWidth = ref(0)
const imageNaturalHeight = ref(0)
const isReturning = ref(false)
const triggerRect = ref<DOMRect | null>(null)

const containerWidth = computed(() => triggerRect.value?.width || 300)

const scale = computed(() => {
	if (imageNaturalWidth.value === 0) return 1
	return containerWidth.value / imageNaturalWidth.value
})

const maxSliderLeft = computed(() => Math.max(0, containerWidth.value - THUMB_SIZE))

const maxTileLeft = computed(() => {
	if (tileNaturalSize.value === 0 || scale.value === 0) return containerWidth.value
	return Math.max(0, containerWidth.value - tileNaturalSize.value * scale.value)
})

const tileStyle = computed(() => {
	if (!captchaData.value || tileNaturalSize.value === 0) return { display: 'none' }
	const s = scale.value
	const size = tileNaturalSize.value * s
	const ratio = maxSliderLeft.value > 0 ? sliderLeft.value / maxSliderLeft.value : 0
	const tileLeft = ratio * maxTileLeft.value
	return {
		display: 'block',
		width: `${size}px`,
		height: `${size}px`,
		top: `${captchaData.value.tile_y * s}px`,
		left: `${tileLeft}px`,
	}
})

const fillWidth = computed(() => `${sliderLeft.value + THUMB_SIZE / 2}px`)

const popupStyle = computed(() => {
	if (props.centered) {
		return {
			left: '50%',
			top: '50%',
			transform: 'translate(-50%, -50%)',
			width: '320px',
		}
	}
	if (!triggerRect.value) return { display: 'none' }
	const rect = triggerRect.value
	const imgRatio = imageNaturalHeight.value && imageNaturalWidth.value
			? imageNaturalHeight.value / imageNaturalWidth.value
			: 9 / 16
		const est = (rect.width * imgRatio) + 40 + 48 + 20
	const above = rect.top > est + 16
	if (above) {
		return {
			left: `${rect.left}px`,
			bottom: `${window.innerHeight - rect.top + 10}px`,
			width: `${rect.width}px`,
		}
	}
	return {
		left: `${rect.left}px`,
		top: `${rect.bottom + 10}px`,
		width: `${rect.width}px`,
	}
})

const canInteract = computed(() =>
	captchaData.value && !loading.value && status.value !== 'verifying' && status.value !== 'success'
)

onMounted(() => {
	fetchCaptcha()
	window.addEventListener('scroll', onScroll, true)
	window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
	window.removeEventListener('scroll', onScroll, true)
	window.removeEventListener('resize', onResize)
	document.removeEventListener('mousedown', onClickOutside)
})

watch(activated, (val) => {
	if (val) {
		document.addEventListener('mousedown', onClickOutside)
	} else {
		document.removeEventListener('mousedown', onClickOutside)
	}
})

watch(captchaData, () => {
	if (captchaData.value) {
		sliderLeft.value = 0
		tileNaturalSize.value = 0
		isReturning.value = false
		emitValue()
	}
})

function activate() {
	if (status.value === 'success') return
	if (!captchaData.value && !loading.value) fetchCaptcha()
	updateTriggerRect()
	activated.value = true
}

function deactivate() {
	if (status.value === 'verifying') return
	activated.value = false
	isDragging.value = false
}

function updateTriggerRect() {
	triggerRect.value = triggerRef.value?.getBoundingClientRect() || null
}

function onScroll() {
	if (activated.value) updateTriggerRect()
}

function onResize() {
	if (activated.value) updateTriggerRect()
}

function onClickOutside(e: MouseEvent) {
	if (!activated.value) return
	const target = e.target as HTMLElement
	if (popupRef.value?.contains(target)) return
	if (triggerRef.value?.contains(target)) return
	deactivate()
}

function onImageLoad() {
	if (imageRef.value) {
		imageNaturalWidth.value = imageRef.value.naturalWidth
		imageNaturalHeight.value = imageRef.value.naturalHeight
	}
}

function onTileLoad() {
	if (tileImgRef.value) tileNaturalSize.value = tileImgRef.value.naturalWidth
}

function emitValue() {
	const s = scale.value
	const ratio = maxSliderLeft.value > 0 ? sliderLeft.value / maxSliderLeft.value : 0
	const tileLeft = ratio * maxTileLeft.value
	emit('update:modelValue', {
		captchaKey: captchaData.value?.captcha_key || '',
		captchaX: s > 0 ? Math.round(tileLeft / s) : 0,
	})
}

function onPointerDown(e: PointerEvent) {
	if (!canInteract.value) return
	e.preventDefault()
	isDragging.value = true
	isReturning.value = false
	startX.value = e.clientX
	startLeft.value = sliderLeft.value
	thumbRef.value?.setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
	if (!isDragging.value) return
	const delta = e.clientX - startX.value
	sliderLeft.value = Math.max(0, Math.min(maxSliderLeft.value, startLeft.value + delta))
}

async function onPointerUp() {
	if (!isDragging.value) return
	isDragging.value = false
	if (!captchaData.value || sliderLeft.value < 5) {
		springBack()
		return
	}
	emitValue()
	const s = scale.value
	const ratio = maxSliderLeft.value > 0 ? sliderLeft.value / maxSliderLeft.value : 0
	const tileLeft = ratio * maxTileLeft.value
	const captchaX = s > 0 ? Math.round(tileLeft / s) : 0
	await verify(captchaData.value.captcha_key, captchaX)
	if (status.value === 'failed') {
		await delay(700)
		springBack()
		await delay(400)
		resetCaptcha()
	} else if (status.value === 'success') {
		emitValue()
		await delay(800)
		activated.value = false
	}
}

function springBack() {
	isReturning.value = true
	sliderLeft.value = 0
	emitValue()
	setTimeout(() => { isReturning.value = false }, 400)
}

function delay(ms: number) {
	return new Promise(resolve => setTimeout(resolve, ms))
}
</script>

<template>
	<div class="sc select-none touch-none w-full">
		<!-- Floating popup -->
		<Teleport to="body">
			<div v-show="activated" ref="popupRef" class="sc-popup" :style="popupStyle">
				<!-- Header -->
				<div class="flex items-center justify-between px-3 h-9 bg-gray-50/80 border-b border-gray-100">
					<span class="text-[13px] text-gray-700 font-medium">请完成安全验证</span>
					<div class="flex items-center gap-1">
						<button
							v-if="captchaData && status !== 'success'"
							type="button"
							class="w-6 h-6 rounded flex items-center justify-center text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-all duration-200 cursor-pointer hover:rotate-90"
							title="换一张"
							@click="resetCaptcha"
						>
							<Icon name="refresh" size="xs" />
						</button>
						<button
							type="button"
							class="w-6 h-6 rounded flex items-center justify-center text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors cursor-pointer"
							@click="deactivate"
						>
							<Icon name="x" size="xs" />
						</button>
					</div>
				</div>

				<!-- Loading -->
				<div v-if="loading" class="flex items-center justify-center gap-2 py-14 text-gray-400 text-sm">
					<div class="sc-spinner" />
					<span>加载验证码...</span>
				</div>

				<!-- Error -->
				<div v-else-if="error" class="flex items-center justify-center gap-2 py-14 text-red-500 text-sm">
					<span>{{ error }}</span>
					<button type="button" class="text-primary-600 hover:text-primary-700 text-xs font-medium cursor-pointer" @click="resetCaptcha">重试</button>
				</div>

				<!-- Captcha content -->
				<template v-else-if="captchaData">
					<div class="sc-image relative leading-none bg-gray-100">
						<img ref="imageRef" :src="captchaData.master_image" class="w-full block" draggable="false" @load="onImageLoad" />
						<img ref="tileImgRef" :src="captchaData.tile_image" class="sc-tile absolute left-0 pointer-events-none" :style="tileStyle" draggable="false" @load="onTileLoad" />
						<button v-if="status !== 'success'" type="button" class="absolute top-2 right-2 w-7 h-7 rounded-full bg-black/20 hover:bg-black/40 backdrop-blur-sm text-white/90 flex items-center justify-center transition-all duration-200 z-20 hover:rotate-90 cursor-pointer border-none" title="换一张" @click="resetCaptcha">
							<Icon name="refresh" size="xs" />
						</button>
						<div v-if="status === 'verifying'" class="absolute inset-0 flex items-center justify-center bg-white/75 backdrop-blur-[2px] z-10">
							<div class="sc-spinner" />
						</div>
						<div v-if="status === 'success'" class="sc-ov-ok absolute inset-0 flex items-center justify-center z-10">
							<div class="sc-badge-pop w-12 h-12 rounded-full bg-emerald-500 flex items-center justify-center shadow-lg">
								<Icon name="check" size="lg" class="text-white" />
							</div>
						</div>
						<div v-if="status === 'failed'" class="sc-ov-err absolute inset-0 flex items-center justify-center z-10">
							<div class="sc-badge-pop w-12 h-12 rounded-full bg-red-500 flex items-center justify-center shadow-lg">
								<Icon name="x" size="lg" class="text-white" />
							</div>
						</div>
					</div>
					<div class="sc-slider relative h-12 bg-gray-50/80 border-t border-gray-100" :class="{ 'sc-shake': status === 'failed', 'sc-ret': isReturning, 'bg-emerald-50/80 !border-emerald-100': status === 'success' }">
						<div class="absolute top-[5px] bottom-[5px] left-0 right-0 bg-gray-200 rounded-full overflow-hidden">
							<div class="sc-fill absolute top-0 left-0 bottom-0 rounded-full" :class="status === 'success' ? 'bg-gradient-to-r from-emerald-500/15 to-emerald-500/8' : 'bg-gradient-to-r from-primary-500/12 to-primary-500/5'" :style="{ width: fillWidth }" />
							<span class="absolute inset-0 flex items-center justify-center text-xs pointer-events-none select-none transition-opacity duration-200" :class="[sliderLeft > 10 ? 'opacity-0' : '', status === 'success' ? 'text-emerald-500' : 'text-gray-400']">
								{{ status === 'success' ? '验证成功' : '向右滑动完成验证' }}
							</span>
						</div>
						<div
							ref="thumbRef"
							class="sc-thumb absolute top-[2px] w-11 h-11 rounded-xl flex items-center justify-center z-[2] transition-[box-shadow,border-color,color,background] duration-200"
							:class="{
								'bg-white border border-gray-200 text-gray-400 shadow-sm cursor-grab hover:shadow-md hover:text-gray-500': !isDragging && status !== 'success',
								'!cursor-grabbing shadow-lg border-primary-300 text-primary-500 scale-[1.03]': isDragging,
								'bg-emerald-500 border-emerald-500 text-white shadow-md shadow-emerald-500/30 !cursor-default': status === 'success',
							}"
							:style="{ left: sliderLeft + 'px' }"
							@pointerdown="onPointerDown"
							@pointermove="onPointerMove"
							@pointerup="onPointerUp"
						>
							<Icon :name="status === 'success' ? 'check' : 'arrowRight'" size="sm" />
						</div>
					</div>
				</template>
			</div>
		</Teleport>

		<!-- Trigger bar -->
		<div v-show="!centered"
			ref="triggerRef"
			class="flex items-center justify-center gap-1.5 h-10 rounded-xl text-[13px] transition-all duration-200 cursor-pointer"
			:class="{
				'bg-gray-50 border border-gray-200 text-gray-500 hover:bg-gray-100 hover:border-gray-300': status !== 'success' && !(loading && !activated),
				'bg-emerald-50 border border-emerald-200 text-emerald-600 !cursor-default': status === 'success',
				'bg-gray-50 border border-gray-200 text-gray-400 !cursor-default': loading && !activated,
			}"
			@click="activate"
		>
			<template v-if="status === 'success'">
				<Icon name="checkCircle" size="sm" />
				<span class="font-medium">验证成功</span>
			</template>
			<template v-else-if="loading && !activated">
				<div class="sc-spin-sm" />
				<span>加载中...</span>
			</template>
			<template v-else-if="error">
				<span class="text-red-400">加载失败</span>
				<button type="button" class="text-primary-600 hover:text-primary-700 text-xs font-medium cursor-pointer bg-transparent border-none" @click.stop="resetCaptcha">重试</button>
			</template>
			<template v-else>
				<Icon name="shield" size="sm" />
				<span>点击进行验证</span>
			</template>
		</div>
	</div>
</template>

<style scoped>
/* ── Popup ── */
.sc-popup {
	position: fixed;
	z-index: 9999;
	background: #fff;
	border-radius: 16px;
	border: 1px solid #e5e7eb;
	box-shadow: 0 8px 30px rgba(0, 0, 0, 0.1), 0 2px 8px rgba(0, 0, 0, 0.05);
	overflow: hidden;
	animation: sc-pop-in 0.25s ease-out;
}

/* ── Tile ── */
.sc-tile {
	filter: drop-shadow(2px 2px 6px rgba(0, 0, 0, 0.35)) brightness(1.05);
}

/* ── Overlays ── */
.sc-ov-ok { background: rgba(16, 185, 129, 0.12); animation: sc-fade 0.25s ease-out; }
.sc-ov-err { background: rgba(239, 68, 68, 0.12); animation: sc-fade 0.2s ease-out; }
.sc-badge-pop { animation: sc-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1); }

/* ── Slider ── */
.sc-shake { animation: sc-shake 0.45s ease; }
.sc-ret .sc-thumb { transition: left 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94); }
.sc-ret .sc-fill { transition: width 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94); }

/* ── Spinner ── */
.sc-spinner {
	width: 28px;
	height: 28px;
	border-radius: 50%;
	border: 3px solid #e5e7eb;
	border-top-color: #14b8a6;
	animation: sc-spin 0.6s linear infinite;
	flex-shrink: 0;
}

.sc-spin-sm {
	width: 14px;
	height: 14px;
	border-radius: 50%;
	border: 2px solid #e5e7eb;
	border-top-color: #9ca3af;
	animation: sc-spin 0.6s linear infinite;
	flex-shrink: 0;
}

/* ── Keyframes ── */
@keyframes sc-spin { to { transform: rotate(360deg); } }
@keyframes sc-fade { from { opacity: 0; } to { opacity: 1; } }
@keyframes sc-pop { 0% { transform: scale(0); opacity: 0; } 100% { transform: scale(1); opacity: 1; } }
@keyframes sc-shake {
	0%, 100% { transform: translateX(0); }
	15% { transform: translateX(-5px); }
	30% { transform: translateX(5px); }
	45% { transform: translateX(-4px); }
	60% { transform: translateX(4px); }
	75% { transform: translateX(-2px); }
	90% { transform: translateX(2px); }
}
@keyframes sc-pop-in {
	from { opacity: 0; transform: translateY(8px); }
	to { opacity: 1; transform: translateY(0); }
}
</style>
