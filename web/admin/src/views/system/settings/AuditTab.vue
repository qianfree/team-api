<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 审计级别 -->
		<div class="section">
			<div class="section-title">审计级别</div>
			<div class="section-grid">
				<AFormItem label="全局审计级别" help="full=完整记录（含请求头/响应头）, full_text=全量文本, masked=脱敏记录, question_only=仅提问, none=不记录">
					<ASelect v-model="values['audit_level']">
						<AOption value="full" label="完整记录 (full)" />
						<AOption value="full_text" label="全量文本 (full_text)" />
						<AOption value="masked" label="脱敏记录 (masked)" />
						<AOption value="question_only" label="仅提问 (question_only)" />
						<AOption value="none" label="不记录 (none)" />
					</ASelect>
				</AFormItem>
			</div>
		</div>

		<!-- 日志保留 -->
		<div class="section">
			<div class="section-title">日志保留</div>
			<div class="section-grid">
				<AFormItem label="审计日志保留天数">
					<AInputNumber
						:model-value="values['audit_retention_days'] as number"
						@change="(v: number | undefined) => values['audit_retention_days'] = String(v ?? 90)"
						:min="7" :max="3650" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="操作日志保留天数">
					<AInputNumber
						:model-value="values['operation_log_retention_days'] as number"
						@change="(v: number | undefined) => values['operation_log_retention_days'] = String(v ?? 365)"
						:min="30" :max="3650" style="width: 100%"
					/>
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
