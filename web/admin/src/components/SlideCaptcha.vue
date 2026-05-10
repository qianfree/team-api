<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, computed } from 'vue'
import { useCaptcha } from '@/composables/useCaptcha'

const THUMB_SIZE = 44

const props = defineProps<{
	modelValue: { captchaKey: string; captchaX: number }
}>()

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
defineExpose({ resetCaptcha: handleReset })

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
	if (!triggerRect.value) return { display: 'none' }
	const rect = triggerRect.value
	const imgRatio = imageNaturalHeight.value && imageNaturalWidth.value
			? imageNaturalHeight.value / imageNaturalWidth.value
			: 9 / 16
		const est = (rect.width * imgRatio) + 36 + 48 + 20
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
	<div class="sc">
		<!-- Floating popup -->
		<Teleport to="body">
			<div v-show="activated" ref="popupRef" class="sc-popup" :style="popupStyle">
				<!-- Header -->
				<div class="sc-popup-hd">
					<span class="sc-popup-title">请完成安全验证</span>
					<div class="sc-popup-actions">
						<button v-if="captchaData && status !== 'success'" type="button" class="sc-popup-refresh" title="换一张" @click="resetCaptcha">
							<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/></svg>
						</button>
						<button type="button" class="sc-popup-close" @click="deactivate">
							<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
						</button>
					</div>
				</div>

				<!-- Loading inside popup -->
				<div v-if="loading" class="sc-popup-center">
					<div class="sc-spinner" />
					<span>加载验证码...</span>
				</div>

				<!-- Error inside popup -->
				<div v-else-if="error" class="sc-popup-center sc-popup-center--err">
					<span>{{ error }}</span>
					<button type="button" class="sc-popup-retry" @click="resetCaptcha">重试</button>
				</div>

				<!-- Captcha content -->
				<template v-else-if="captchaData">
					<div class="sc-image">
						<img ref="imageRef" :src="captchaData.master_image" class="sc-image-bg" draggable="false" @load="onImageLoad" />
						<img ref="tileImgRef" :src="captchaData.tile_image" class="sc-image-tile" :style="tileStyle" draggable="false" @load="onTileLoad" />
						<div v-if="status === 'verifying'" class="sc-ov"><div class="sc-spinner" /></div>
						<div v-if="status === 'success'" class="sc-ov sc-ov--ok">
							<div class="sc-badge sc-badge--ok">
								<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg>
							</div>
						</div>
						<div v-if="status === 'failed'" class="sc-ov sc-ov--err">
							<div class="sc-badge sc-badge--err">
								<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
							</div>
						</div>
					</div>
					<div class="sc-slider" :class="{ 'sc-slider--fail': status === 'failed', 'sc-slider--ret': isReturning }">
						<div class="sc-track">
							<div class="sc-track-fill" :class="{ 'sc-track-fill--ok': status === 'success' }" :style="{ width: fillWidth }" />
							<span class="sc-track-text" :class="{ 'sc-track-text--fade': sliderLeft > 10, 'sc-track-text--ok': status === 'success' }">
								{{ status === 'success' ? '验证成功' : '向右滑动完成验证' }}
							</span>
						</div>
						<div
							ref="thumbRef"
							class="sc-thumb"
							:class="{ 'sc-thumb--on': isDragging, 'sc-thumb--ok': status === 'success' }"
							:style="{ left: sliderLeft + 'px' }"
							@pointerdown="onPointerDown"
							@pointermove="onPointerMove"
							@pointerup="onPointerUp"
						>
							<svg v-if="status !== 'success'" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14M12 5l7 7-7 7"/></svg>
							<svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg>
						</div>
					</div>
				</template>
			</div>
		</Teleport>

		<!-- Trigger bar -->
		<div
			ref="triggerRef"
			class="sc-trigger"
			:class="{
				'sc-trigger--ok': status === 'success',
				'sc-trigger--load': loading && !activated,
			}"
			@click="activate"
		>
			<template v-if="status === 'success'">
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg>
				<span>验证成功</span>
			</template>
			<template v-else-if="loading && !activated">
				<div class="sc-spin-sm" />
				<span>加载中...</span>
			</template>
			<template v-else-if="error">
				<span>加载失败</span>
				<button type="button" class="sc-trigger-retry" @click.stop="resetCaptcha">重试</button>
			</template>
			<template v-else>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
				<span>点击进行验证</span>
			</template>
		</div>
	</div>
</template>

<style scoped>
.sc {
	user-select: none;
	touch-action: none;
	width: 100%;
}

/* ── Popup ── */
.sc-popup {
	position: fixed;
	z-index: 9999;
	background: #fff;
	border-radius: 12px;
	border: 1px solid #e5e7eb;
	box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12), 0 2px 8px rgba(0, 0, 0, 0.06);
	overflow: hidden;
	animation: sc-pop-in 0.25s ease-out;
}

.sc-popup-hd {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 0 12px;
	height: 36px;
	background: #fafbfc;
	border-bottom: 1px solid #f3f4f6;
}

.sc-popup-title {
	font-size: 13px;
	color: #374151;
	font-weight: 500;
}

.sc-popup-close {
	width: 24px;
	height: 24px;
	border-radius: 4px;
	background: none;
	border: none;
	cursor: pointer;
	color: #9ca3af;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: background 0.15s, color 0.15s;
}

.sc-popup-close:hover {
	background: #f3f4f6;
	color: #6b7280;
}

.sc-popup-center {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	padding: 52px 20px;
	color: #9ca3af;
	font-size: 13px;
}

.sc-popup-center--err {
	color: #ef4444;
}

.sc-popup-retry {
	color: #3b82f6;
	cursor: pointer;
	font-size: 12px;
	background: none;
	border: none;
}

.sc-popup-retry:hover {
	text-decoration: underline;
}

/* ── Image ── */
.sc-image {
	position: relative;
	line-height: 0;
	background: #f3f4f6;
}

.sc-image-bg {
	width: 100%;
	display: block;
}

.sc-image-tile {
	position: absolute;
	left: 0;
	pointer-events: none;
	filter: drop-shadow(2px 2px 6px rgba(0, 0, 0, 0.35)) brightness(1.05);
}

/* ── Header actions ── */
.sc-popup-actions {
	display: flex;
	align-items: center;
	gap: 4px;
}

.sc-popup-refresh {
	width: 24px;
	height: 24px;
	border-radius: 4px;
	background: none;
	border: none;
	cursor: pointer;
	color: #9ca3af;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: background 0.15s, color 0.15s, transform 0.25s;
}

.sc-popup-refresh:hover {
	background: #f3f4f6;
	color: #6b7280;
	transform: rotate(90deg);
}

/* ── Overlays ── */
.sc-ov {
	position: absolute;
	inset: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	z-index: 4;
}

.sc-ov--ok { background: rgba(34, 197, 94, 0.12); animation: sc-fade 0.25s ease-out; }
.sc-ov--err { background: rgba(239, 68, 68, 0.12); animation: sc-fade 0.2s ease-out; }

.sc-badge {
	width: 48px;
	height: 48px;
	border-radius: 50%;
	display: flex;
	align-items: center;
	justify-content: center;
	box-shadow: 0 4px 14px rgba(0, 0, 0, 0.15);
}

.sc-badge--ok { background: #22c55e; animation: sc-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1); }
.sc-badge--err { background: #ef4444; animation: sc-pop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1); }

/* ── Slider ── */
.sc-slider {
	position: relative;
	height: 48px;
	background: #fafbfc;
	border-top: 1px solid #f3f4f6;
}

.sc-slider--fail { animation: sc-shake 0.45s ease; }

.sc-slider--ret .sc-thumb { transition: left 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94); }
.sc-slider--ret .sc-track-fill { transition: width 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94); }

/* ── Track ── */
.sc-track {
	position: absolute;
	top: 5px;
	bottom: 5px;
	left: 0;
	right: 0;
	background: #e5e7eb;
	border-radius: 19px;
	overflow: hidden;
}

.sc-track-fill {
	position: absolute;
	top: 0;
	left: 0;
	bottom: 0;
	background: linear-gradient(90deg, rgba(59, 130, 246, 0.12), rgba(59, 130, 246, 0.06));
	border-radius: 19px;
}

.sc-track-fill--ok {
	background: linear-gradient(90deg, rgba(34, 197, 94, 0.15), rgba(34, 197, 94, 0.08));
}

.sc-track-text {
	position: absolute;
	inset: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	font-size: 12px;
	color: #9ca3af;
	pointer-events: none;
	user-select: none;
	transition: opacity 0.2s;
}

.sc-track-text--fade { opacity: 0; }
.sc-track-text--ok { color: #22c55e; }

/* ── Thumb ── */
.sc-thumb {
	position: absolute;
	top: 2px;
	width: 44px;
	height: 44px;
	background: #fff;
	border: 1px solid #d1d5db;
	border-radius: 10px;
	display: flex;
	align-items: center;
	justify-content: center;
	cursor: grab;
	z-index: 2;
	color: #9ca3af;
	box-shadow: 0 2px 4px rgba(0, 0, 0, 0.06);
	transition: box-shadow 0.2s, border-color 0.2s, color 0.2s, background 0.25s;
}

.sc-thumb:hover { box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1); color: #6b7280; }

.sc-thumb--on {
	cursor: grabbing;
	box-shadow: 0 6px 16px rgba(0, 0, 0, 0.14);
	border-color: #93c5fd;
	color: #3b82f6;
	transform: scale(1.03);
}

.sc-thumb--ok {
	background: #22c55e;
	border-color: #22c55e;
	color: #fff;
	cursor: default;
	box-shadow: 0 3px 10px rgba(34, 197, 94, 0.3);
}

/* ── Trigger bar ── */
.sc-trigger {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 6px;
	height: 40px;
	border-radius: 8px;
	background: #f7f8fa;
	border: 1px solid #e5e7eb;
	cursor: pointer;
	font-size: 13px;
	color: #666;
	transition: all 0.2s;
}

.sc-trigger:hover {
	background: #f0f1f3;
	border-color: #d1d5db;
}

.sc-trigger--ok {
	background: #f0fdf4;
	border-color: #86efac;
	color: #22c55e;
	cursor: default;
}

.sc-trigger--load {
	cursor: default;
	color: #9ca3af;
}

.sc-trigger-retry {
	color: #3b82f6;
	cursor: pointer;
	font-size: 12px;
	background: none;
	border: none;
}

.sc-trigger-retry:hover {
	text-decoration: underline;
}

/* ── Spinners ── */
.sc-spinner {
	width: 28px;
	height: 28px;
	border-radius: 50%;
	border: 3px solid #e5e7eb;
	border-top-color: #3b82f6;
	animation: sc-spin 0.6s linear infinite;
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
