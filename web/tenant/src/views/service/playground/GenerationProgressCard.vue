<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'

// 卡片式异步任务加载占位：按成品形状（图片 1:1 / 视频 16:9）占位的动效卡片，
// 中央进度环展示百分比（无进度数值时转为旋转弧的不确定态），
// 底层为浅色渐变底 + 双色光斑漂移 + 斜向扫光，视觉对齐 OpenAI / Gemini 生成中的占位效果。
//
// compact：消息气泡式的小卡片。整块宽度按 16:9 反推，只占容器左半侧，
// 高度随宽度线性缩小——提示词输入区在其下方展开时不会再把它挤出视口。
const props = withDefaults(
	defineProps<{
		kind?: 'image' | 'video'
		progress?: string
		label?: string
		sublabel?: string
		/** 紧凑形态：占位尺寸由「宽度撑满」改为「左半侧小卡片」 */
		compact?: boolean
	}>(),
	{
		kind: 'image',
		progress: '',
		label: '生成中...',
		sublabel: '',
		compact: false,
	},
)

// 从进度文本解析 0-100 数值（上游协议为 "45%" 形态）；解析不到时走不确定态
const pct = computed<number | null>(() => {
	const m = /(\d+(?:\.\d+)?)/.exec(props.progress)
	if (!m) return null
	const n = Number(m[1])
	if (!Number.isFinite(n)) return null
	return Math.min(100, Math.max(0, Math.round(n)))
})

// 进度环几何参数：viewBox 100x100，r=42
const RADIUS = 42
const CIRCUMFERENCE = 2 * Math.PI * RADIUS
const dashOffset = computed(() => CIRCUMFERENCE * (1 - (pct.value ?? 0) / 100))

// 渐变 id 按实例自增，避免同页多个卡片时 url(#id) 引用错乱
let seq = 0
const gradId = `gen-grad-${++seq}`
</script>

<template>
	<div
		class="gen-card animate-scale-in"
		:class="[
			kind === 'video' ? 'gen-card--video' : 'gen-card--image',
			compact ? 'gen-card--compact' : '',
		]"
	>
		<!-- 底层：浅色渐变底 + 光斑漂移 + 斜向扫光 -->
		<div class="gen-card__bg"></div>
		<div class="gen-blob gen-blob--a"></div>
		<div class="gen-blob gen-blob--b"></div>
		<div class="gen-sheen"></div>

		<!-- 中央：进度环 + 状态文案 -->
		<div class="gen-center">
			<div class="gen-ring">
				<svg viewBox="0 0 100 100" class="gen-ring__svg">
					<defs>
						<linearGradient :id="gradId" x1="0%" y1="0%" x2="100%" y2="100%">
							<stop offset="0%" stop-color="#2dd4bf" />
							<stop offset="100%" stop-color="#0d9488" />
						</linearGradient>
					</defs>
					<!-- 轨道 -->
					<circle cx="50" cy="50" :r="RADIUS" fill="none" stroke="rgba(148, 163, 184, 0.25)" stroke-width="6" />
					<!-- 不确定态：旋转短弧 -->
					<circle
						v-if="pct === null"
						class="gen-ring__indeterminate"
						cx="50" cy="50" :r="RADIUS"
						fill="none" :stroke="`url(#${gradId})`"
						stroke-width="6" stroke-linecap="round"
						:stroke-dasharray="`${CIRCUMFERENCE * 0.24} ${CIRCUMFERENCE}`"
					/>
					<!-- 确定态：按进度绘制弧长 -->
					<circle
						v-else
						class="gen-ring__progress"
						cx="50" cy="50" :r="RADIUS"
						fill="none" :stroke="`url(#${gradId})`"
						stroke-width="6" stroke-linecap="round"
						:stroke-dasharray="CIRCUMFERENCE"
						:stroke-dashoffset="dashOffset"
					/>
				</svg>
				<div class="gen-ring__text">
					<span v-if="pct !== null" class="gen-pct">{{ pct }}<small>%</small></span>
					<Icon v-else name="hourglass" :size="compact ? 'sm' : 'lg'" class="animate-pulse text-primary-500" />
				</div>
			</div>
			<p class="gen-label">{{ label }}</p>
			<p v-if="sublabel" class="gen-sublabel">{{ sublabel }}</p>
		</div>
	</div>
</template>

<style scoped>
/* 卡片容器：按成品形态占位 */
.gen-card {
	position: relative;
	overflow: hidden;
	border-radius: 16px;
	border: 1px solid #99f6e4;
	box-shadow: 0 10px 34px rgba(91, 104, 151, 0.08);
}
.gen-card--image {
	aspect-ratio: 1 / 1;
	max-width: 360px;
	margin-inline: auto;
}
.gen-card--video {
	aspect-ratio: 16 / 9;
}

/* 紧凑形态：宽度收成左半侧（16:9 反推高度，约 146px @ 260px 宽），
   让下方展开的提示词输入区不再挤压生成态与预览窗口 */
.gen-card--compact {
	width: min(100%, 260px);
}
.gen-card--compact.gen-card--video,
.gen-card--compact.gen-card--image {
	max-width: 260px;
	margin-inline: 0;
	aspect-ratio: 16 / 9;
}
/* 小卡片下进度环与文案同比缩小，避免视觉上「字比卡片大」 */
.gen-card--compact .gen-center {
	gap: 8px;
}
.gen-card--compact .gen-ring {
	width: 56px;
	height: 56px;
}
.gen-card--compact .gen-pct {
	font-size: 16px;
}
.gen-card--compact .gen-pct small {
	font-size: 10px;
}
.gen-card--compact .gen-label {
	font-size: 12px;
}
.gen-card--compact .gen-sublabel {
	font-size: 10px;
}

/* 底层浅色渐变 */
.gen-card__bg {
	position: absolute;
	inset: 0;
	background: linear-gradient(135deg, #f0fdfa 0%, #f8fafc 45%, #eff6ff 100%);
}

/* 双色光斑缓慢漂移（青 + 靛），叠加出极光质感 */
.gen-blob {
	position: absolute;
	width: 60%;
	aspect-ratio: 1 / 1;
	border-radius: 50%;
	filter: blur(42px);
	pointer-events: none;
}
.gen-blob--a {
	left: -12%;
	top: -16%;
	background: radial-gradient(circle, rgba(45, 212, 191, 0.4), transparent 70%);
	animation: gen-drift-a 7s ease-in-out infinite alternate;
}
.gen-blob--b {
	right: -14%;
	bottom: -20%;
	background: radial-gradient(circle, rgba(129, 140, 248, 0.32), transparent 70%);
	animation: gen-drift-b 9s ease-in-out infinite alternate;
}
@keyframes gen-drift-a {
	from { transform: translate(0, 0) scale(1); }
	to { transform: translate(22%, 16%) scale(1.18); }
}
@keyframes gen-drift-b {
	from { transform: translate(0, 0) scale(1.12); }
	to { transform: translate(-20%, -14%) scale(0.94); }
}

/* 斜向扫光带，周期性掠过卡片 */
.gen-sheen {
	position: absolute;
	top: -12%;
	bottom: -12%;
	width: 42%;
	background: linear-gradient(105deg, transparent, rgba(255, 255, 255, 0.55), transparent);
	transform: skewX(-16deg) translateX(-130%);
	animation: gen-sweep 2.8s ease-in-out infinite;
	pointer-events: none;
}
@keyframes gen-sweep {
	0% { transform: skewX(-16deg) translateX(-130%); }
	55%, 100% { transform: skewX(-16deg) translateX(330%); }
}

/* 中央内容 */
.gen-center {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 14px;
}

/* 进度环：白色圆盘承托，保证文字与光斑叠加时的可读性 */
.gen-ring {
	position: relative;
	width: 104px;
	height: 104px;
	background: rgba(255, 255, 255, 0.8);
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 50%;
	box-shadow: 0 8px 24px rgba(76, 91, 142, 0.12);
}
.gen-ring__svg {
	position: absolute;
	inset: 6px;
}
.gen-ring__indeterminate {
	transform-box: view-box;
	transform-origin: center;
	animation: gen-spin 1.1s linear infinite;
}
.gen-ring__progress {
	transition: stroke-dashoffset 0.6s ease;
}
@keyframes gen-spin {
	to { transform: rotate(360deg); }
}
.gen-ring__text {
	position: absolute;
	inset: 0;
	display: flex;
	align-items: center;
	justify-content: center;
}
.gen-pct {
	font-size: 26px;
	font-weight: 700;
	color: #0f766e;
	font-variant-numeric: tabular-nums;
	line-height: 1;
}
.gen-pct small {
	font-size: 14px;
	font-weight: 600;
	margin-left: 1px;
}
.gen-label {
	font-size: 14px;
	font-weight: 500;
	color: #334155;
}
.gen-sublabel {
	font-size: 12px;
	color: #94a3b8;
	max-width: 80%;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

/* 弱动效偏好：静止装饰动画，仅保留进度过渡 */
@media (prefers-reduced-motion: reduce) {
	.gen-blob--a,
	.gen-blob--b,
	.gen-sheen,
	.gen-ring__indeterminate {
		animation: none;
	}
}
</style>
