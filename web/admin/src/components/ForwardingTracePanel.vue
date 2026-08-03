<script setup lang="ts">
import { computed } from 'vue'

// ForwardingTracePanel 转发路径追踪展示（仅管理员可见）。
// 接收已解析的 trace 对象，渲染：概要 / 转发跳转 hops / 调度决策 scheduler。
// 用量日志页与审计日志页共用，数据来源为 aud_request_logs.forwarding_trace。
const props = defineProps<{ trace: any }>()

// 6 个权重贡献因子（effective 为合成结果，单独高亮在主行，不进条形）
const FACTORS = [
	{ key: 'base', label: '基础' },
	{ key: 'tier', label: '层级' },
	{ key: 'health', label: '健康' },
	{ key: 'headroom', label: '余量' },
	{ key: 'cost', label: '成本' },
	{ key: 'ramp', label: '渐进' },
]

// 选择原因标签 + 颜色（hops 的 selection_reason 与 scheduler 的 reason 共用）
const REASON_META: Record<string, { label: string; color: string }> = {
	bind: { label: '绑定亲和', color: 'arcoblue' },
	hrw: { label: '一致性哈希', color: 'cyan' },
	overflow: { label: '溢出降级', color: 'orange' },
	probe: { label: '探针', color: 'purple' },
	cred_rotate: { label: '凭证轮换', color: 'magenta' },
}

function reasonTag(reason: string) {
	return REASON_META[reason] || { label: reason || '-', color: 'gray' }
}

function formatMs(ms: number): string {
	if (ms === null || ms === undefined) return '-'
	if (ms < 1000) return `${ms}ms`
	return `${(ms / 1000).toFixed(2)}s`
}

// scheduler 决策只带 channel_id，从 hops 建立 id→name 映射补全渠道名
const channelNameMap = computed<Record<number, string>>(() => {
	const m: Record<number, string> = {}
	for (const hop of props.trace?.hops || []) {
		if (hop?.channel_id && hop?.channel_name) m[hop.channel_id] = hop.channel_name
	}
	return m
})

// 隐藏全空的列（任务 trace 等稀疏场景）
const hasKey = computed(() => (props.trace?.scheduler || []).some((d: any) => d?.key_id))
const hasTier = computed(() => (props.trace?.scheduler || []).some((d: any) => d?.tier))
const hasSession = computed(() => (props.trace?.scheduler || []).some((d: any) => d?.session_source))

// 各权重维度按全表最大值归一化（因子为乘法关系，不可堆叠，故逐维独立缩放）
const weightMaxes = computed<Record<string, number>>(() => {
	const maxes: Record<string, number> = {}
	const sched = props.trace?.scheduler || []
	for (const f of FACTORS) {
		let m = 0
		for (const d of sched) {
			const v = Number(d?.weights?.[f.key] || 0)
			if (v > m) m = v
		}
		maxes[f.key] = m || 1
	}
	return maxes
})

function factorPct(d: any, key: string): number {
	const v = Number(d?.weights?.[key] || 0)
	const m = weightMaxes.value[key] || 1
	return m > 0 ? Math.min(100, (v / m) * 100) : 0
}

function excludedTotal(d: any): number {
	return (d?.excluded_breaker || 0) + (d?.excluded_lease || 0) + (d?.excluded_request || 0)
}
</script>

<template>
	<div v-if="trace" class="fwd-trace">
		<!-- 概要 -->
		<a-descriptions :column="1" bordered size="medium">
			<a-descriptions-item label="入口路径">{{ trace.entry_path || '-' }}</a-descriptions-item>
			<a-descriptions-item label="入口格式">{{ trace.entry_format || '-' }}</a-descriptions-item>
			<a-descriptions-item label="请求模型">{{ trace.requested_model || '-' }}</a-descriptions-item>
			<a-descriptions-item label="上游模型">{{ trace.upstream_model || '-' }}</a-descriptions-item>
			<a-descriptions-item v-if="trace.model_mapped" label="模型映射">
				<a-tag color="orange" size="small">是</a-tag>
			</a-descriptions-item>
			<a-descriptions-item label="总尝试次数">{{ trace.total_attempts ?? (trace.hops?.length || 0) }}</a-descriptions-item>
		</a-descriptions>

		<!-- 转发跳转 -->
		<div v-if="trace.hops?.length" class="fwd-section">
			<h4 class="fwd-title">转发跳转</h4>
			<a-table
				:data="trace.hops"
				:bordered="false"
				:stripe="true"
				size="mini"
				:pagination="false"
				:scroll="{ x: 760 }"
			>
				<template #columns>
					<a-table-column title="次数" data-index="attempt" :width="50" />
					<a-table-column title="渠道" :width="150">
						<template #cell="{ record }">
							{{ record.channel_name || '-' }}
							<span class="fwd-muted">#{{ record.channel_id }}</span>
						</template>
					</a-table-column>
					<a-table-column title="供应商" data-index="provider" :width="90" />
					<a-table-column title="选择原因" :width="110">
						<template #cell="{ record }">
							<a-tag v-if="record.selection_reason" :color="reasonTag(record.selection_reason).color" size="small">
								{{ reasonTag(record.selection_reason).label }}
							</a-tag>
							<span v-else class="fwd-muted">-</span>
						</template>
					</a-table-column>
					<a-table-column title="上游模型" data-index="upstream_model" :width="130" ellipsis tooltip />
					<a-table-column title="状态" :width="65">
						<template #cell="{ record }">
							<a-tag :color="record.success ? 'green' : 'red'" size="small">
								{{ record.success ? '成功' : '失败' }}
							</a-tag>
						</template>
					</a-table-column>
					<a-table-column title="延迟" :width="80">
						<template #cell="{ record }">{{ formatMs(record.latency_ms) }}</template>
					</a-table-column>
					<a-table-column title="错误" data-index="error" :width="200" ellipsis tooltip />
				</template>
			</a-table>
		</div>

		<!-- 调度决策（任务 trace 无此项时自动隐藏） -->
		<div v-if="trace.scheduler?.length" class="fwd-section">
			<h4 class="fwd-title">调度决策<span class="fwd-title-hint">（展开查看权重分解）</span></h4>
			<a-table
				:data="trace.scheduler"
				:bordered="false"
				:stripe="true"
				size="mini"
				:pagination="false"
				:scroll="{ x: 720 }"
			>
				<template #columns>
					<a-table-column title="次数" data-index="attempt" :width="55" />
					<a-table-column title="渠道" :width="150">
						<template #cell="{ record }">
							{{ channelNameMap[record.channel_id] || '-' }}
							<span class="fwd-muted">#{{ record.channel_id }}</span>
						</template>
					</a-table-column>
					<a-table-column v-if="hasKey" title="Key" :width="80">
						<template #cell="{ record }">{{ record.key_id || '-' }}</template>
					</a-table-column>
					<a-table-column title="原因" :width="110">
						<template #cell="{ record }">
							<a-tag :color="reasonTag(record.reason).color" size="small">
								{{ reasonTag(record.reason).label }}
							</a-tag>
						</template>
					</a-table-column>
					<a-table-column v-if="hasTier" title="层级" data-index="tier" :width="80" />
					<a-table-column v-if="hasSession" title="会话来源" data-index="session_source" :width="100" ellipsis tooltip />
					<a-table-column title="候选" :width="120">
						<template #cell="{ record }">
							<span>{{ record.candidates ?? '-' }}</span>
							<a-tooltip
								v-if="excludedTotal(record) > 0"
								:content="`熔断 ${record.excluded_breaker || 0} / 租约 ${record.excluded_lease || 0} / 请求 ${record.excluded_request || 0}`"
							>
								<span class="fwd-excluded">（排除 {{ excludedTotal(record) }}）</span>
							</a-tooltip>
						</template>
					</a-table-column>
					<a-table-column title="有效权重" :width="110">
						<template #cell="{ record }">
							<span class="fwd-effective">{{ Number(record.weights?.effective || 0).toFixed(2) }}</span>
						</template>
					</a-table-column>
				</template>
				<template #expand-row="{ record }">
					<div class="fwd-expand">
						<div class="fwd-weights">
							<div v-for="f in FACTORS" :key="f.key" class="fwd-weight">
								<div class="fwd-weight-head">
									<span class="fwd-weight-label">{{ f.label }}</span>
									<span class="fwd-weight-val">{{ Number(record.weights?.[f.key] || 0).toFixed(2) }}</span>
								</div>
								<div class="fwd-bar">
									<div class="fwd-bar-fill" :style="{ width: factorPct(record, f.key) + '%' }"></div>
								</div>
							</div>
						</div>
						<div v-if="excludedTotal(record) > 0" class="fwd-excluded-detail">
							<span class="fwd-muted">排除明细：</span>
							<a-tag v-if="record.excluded_breaker" size="small" color="red">熔断 {{ record.excluded_breaker }}</a-tag>
							<a-tag v-if="record.excluded_lease" size="small" color="orange">租约 {{ record.excluded_lease }}</a-tag>
							<a-tag v-if="record.excluded_request" size="small" color="gray">请求 {{ record.excluded_request }}</a-tag>
						</div>
					</div>
				</template>
			</a-table>
		</div>
	</div>
</template>

<style scoped>
.fwd-trace {
	width: 100%;
}
.fwd-section {
	margin-top: 12px;
}
.fwd-title {
	margin: 0 0 8px;
	font-size: 13px;
	color: var(--color-text-1);
	font-weight: 600;
}
.fwd-title-hint {
	margin-left: 6px;
	font-size: 12px;
	font-weight: 400;
	color: var(--color-text-3);
}
.fwd-muted {
	color: var(--color-text-3);
	font-size: 12px;
}
.fwd-excluded {
	color: var(--color-text-3);
	font-size: 12px;
	cursor: help;
}
.fwd-effective {
	font-weight: 700;
	color: rgb(var(--arcoblue-6));
	font-variant-numeric: tabular-nums;
}
.fwd-expand {
	padding: 8px 12px;
}
.fwd-weights {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 10px 24px;
}
.fwd-weight-head {
	display: flex;
	justify-content: space-between;
	font-size: 12px;
	margin-bottom: 4px;
}
.fwd-weight-label {
	color: var(--color-text-2);
}
.fwd-weight-val {
	color: var(--color-text-1);
	font-variant-numeric: tabular-nums;
}
.fwd-bar {
	height: 6px;
	background: var(--color-fill-2);
	border-radius: 3px;
	overflow: hidden;
}
.fwd-bar-fill {
	height: 100%;
	background: rgb(var(--arcoblue-6));
	border-radius: 3px;
	transition: width 0.3s;
}
.fwd-excluded-detail {
	margin-top: 10px;
	display: flex;
	align-items: center;
	gap: 6px;
	flex-wrap: wrap;
}
</style>
