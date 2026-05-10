<script setup lang="ts">
const values = defineModel<Record<string, any>>({ required: true })

function num(key: string) {
	return Number(values.value[key]) || 0
}
function set(key: string, v: number | undefined): void {
	values.value[key] = String(v ?? 0)
}
</script>

<template>
	<div class="tab-content">
		<!-- 日志保留 -->
		<div class="section">
			<div class="section-title">日志保留</div>
			<div class="section-grid">
				<AFormItem label="API 调用日志保留天数">
					<AInputNumber :model-value="num('data_retention_api_logs_days')" @change="(v: number | undefined) => set('data_retention_api_logs_days', v)" :min="7" :max="3650" style="width: 100%" />
				</AFormItem>
				<AFormItem label="操作日志保留天数">
					<AInputNumber :model-value="num('data_retention_operation_logs_days')" @change="(v: number | undefined) => set('data_retention_operation_logs_days', v)" :min="30" :max="3650" style="width: 100%" />
				</AFormItem>
				<AFormItem label="临时数据保留天数">
					<AInputNumber :model-value="num('data_retention_temp_data_days')" @change="(v: number | undefined) => set('data_retention_temp_data_days', v)" :min="1" :max="30" style="width: 100%" />
				</AFormItem>
			</div>
		</div>

		<!-- 数据导出 -->
		<div class="section">
			<div class="section-title">数据导出</div>
			<div class="section-grid">
				<AFormItem label="导出文件保留天数">
					<AInputNumber :model-value="num('data_export_expiry_days')" @change="(v: number | undefined) => set('data_export_expiry_days', v)" :min="1" :max="30" style="width: 100%" />
				</AFormItem>
				<AFormItem label="GDPR 删除请求完成天数">
					<AInputNumber :model-value="num('data_deletion_completion_days')" @change="(v: number | undefined) => set('data_deletion_completion_days', v)" :min="7" :max="90" style="width: 100%" />
				</AFormItem>
			</div>
		</div>

		<!-- 文件保留 -->
		<div class="section">
			<div class="section-title">文件保留</div>
			<div class="switch-row">
				<span class="switch-label">启用文件保留期检查</span>
				<ASwitch
					:model-value="values['file_retention_enabled'] === 'true' || values['file_retention_enabled'] === '1'"
					@change="(v: string | number | boolean) => values['file_retention_enabled'] = String(v)"
				/>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
