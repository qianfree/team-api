<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

// 通用插件配置弹窗：按 config-schema 驱动渲染（分组小节 + 双列网格 + 说明下置 +
// 加密字段密码输入）。插件管理页与插件功能页共用——启用插件后可在自己的页面内
// 直接配置，不必回插件管理页。
//
// 权限说明：本组件走通用配置接口（system:plugin）；调用方自行决定入口按钮的
// 显隐（功能页一般收敛到插件自己的 config 权限点）。

const props = defineProps<{
	visible: boolean
	pluginName: string
	title?: string
}>()
const emit = defineEmits<{
	(e: 'update:visible', v: boolean): void
	(e: 'saved'): void
}>()

const loading = ref(false)
const submitting = ref(false)
const schema = ref<any[]>([])
const form = ref<Record<string, any>>({})

watch(
	() => [props.visible, props.pluginName] as const,
	([visible]) => {
		if (visible && props.pluginName) load()
	},
)

async function load() {
	loading.value = true
	form.value = {}
	schema.value = []

	try {
		const [schemaRes, configRes] = await Promise.all([
			request.get(`/admin/plugins/${props.pluginName}/config-schema`),
			request.get(`/admin/plugins/${props.pluginName}`),
		])
		schema.value = schemaRes.data?.data?.list || []
		const currentConfig = configRes.data?.data?.config || {}
		// 先填默认值再覆盖当前值（掩码回显的加密字段保持掩码，提交掩码=不改动现值）
		const next: Record<string, any> = {}
		for (const field of schema.value) {
			next[field.key] = currentConfig[field.key] ?? field.default
		}
		form.value = next
	} catch {
		Message.error('加载配置失败')
		emit('update:visible', false)
	} finally {
		loading.value = false
	}
}

async function submit() {
	submitting.value = true
	try {
		await request.put(`/admin/plugins/${props.pluginName}/config`, {
			config: form.value,
		})
		Message.success('配置已保存')
		emit('update:visible', false)
		emit('saved')
	} catch {
		// 拦截器已提示
	} finally {
		submitting.value = false
	}
}

// 配置字段按 Group 归拢（保持声明顺序）；未声明 Group 的字段归入空名组不显示标题
const groupedSchema = computed(() => {
	const groups: { name: string; fields: any[] }[] = []
	const index = new Map<string, { name: string; fields: any[] }>()
	for (const field of schema.value) {
		const name = field.group || ''
		let group = index.get(name)
		if (!group) {
			group = { name, fields: [] }
			index.set(name, group)
			groups.push(group)
		}
		group.fields.push(field)
	}
	return groups
})

// 宽度规则：schema 声明 half_width 优先（半宽与相邻字段同行）；
// 未声明时按类型默认——int/bool/select 半宽，string/json/text 占满整行
function isWideField(field: any): boolean {
	if (field.half_width) return false
	return field.type === 'string' || field.type === 'json' || field.type === 'text'
}
</script>

<template>
	<AModal
		:visible="visible"
		:title="title || `配置 - ${pluginName}`"
		:mask-closable="false"
		:width="660"
		:ok-loading="submitting"
		:on-before-ok="submit"
		@update:visible="emit('update:visible', $event)"
		@cancel="emit('update:visible', false)"
	>
		<ASpin :loading="loading" style="width: 100%">
			<AForm :model="form" layout="vertical" v-if="schema.length > 0">
				<template v-for="group in groupedSchema" :key="group.name || '__default__'">
					<div v-if="group.name" class="config-group-title">{{ group.name }}</div>
					<div class="config-grid">
						<AFormItem
							v-for="field in group.fields"
							:key="field.key"
							:class="isWideField(field) ? 'config-wide' : ''"
							:label="field.label"
							:required="field.required"
							:extra="field.description"
						>
							<!-- string -->
							<AInput
								v-if="field.type === 'string' && !field.encrypted"
								v-model="form[field.key]"
								allow-clear
							/>
							<!-- 加密字段：密码型输入；回显为掩码，留空/保持掩码提交即不改动现值 -->
							<AInputPassword
								v-else-if="field.type === 'string' && field.encrypted"
								v-model="form[field.key]"
								allow-clear
							/>
							<!-- int -->
							<AInputNumber
								v-else-if="field.type === 'int'"
								v-model="form[field.key]"
								style="width: 100%"
							/>
							<!-- bool -->
							<ASwitch
								v-else-if="field.type === 'bool'"
								v-model="form[field.key]"
							/>
							<!-- select -->
							<ASelect
								v-else-if="field.type === 'select'"
								v-model="form[field.key]"
								:options="field.options?.map((o: string) => ({ label: o, value: o }))"
							/>
							<!-- json -->
							<ATextarea
								v-else-if="field.type === 'json'"
								v-model="form[field.key]"
								:auto-size="{ minRows: 3, maxRows: 8 }"
							/>
							<!-- text：长文本（如自定义提示词） -->
							<ATextarea
								v-else-if="field.type === 'text'"
								v-model="form[field.key]"
								:auto-size="{ minRows: 6, maxRows: 16 }"
								allow-clear
							/>
							<!-- fallback -->
							<AInput
								v-else
								v-model="form[field.key]"
								allow-clear
							/>
							<div v-if="field.encrypted && form[field.key]" class="config-encrypted-hint">
								已配置（回显为掩码，留空或保持掩码提交即不改动）
							</div>
						</AFormItem>
					</div>
				</template>
			</AForm>
			<AEmpty v-else description="该插件无可配置项" />
		</ASpin>
	</AModal>
</template>

<style scoped>
/* 配置表单：声明了 Group 的 schema 按小节渲染，短字段组内双列 */
.config-group-title {
	font-size: 13px;
	font-weight: 600;
	color: var(--color-text-2);
	margin: 4px 0 12px;
	padding-bottom: 6px;
	border-bottom: 1px solid var(--color-border-2);
}

.config-group-title:not(:first-child) {
	margin-top: 20px;
}

.config-grid {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	column-gap: 16px;
}

.config-grid .config-wide {
	grid-column: span 2;
}

@media (max-width: 640px) {
	.config-grid {
		grid-template-columns: 1fr;
	}
	.config-grid .config-wide {
		grid-column: span 1;
	}
}

.config-encrypted-hint {
	font-size: 12px;
	color: var(--color-text-3);
	margin-top: 2px;
	line-height: 1.5;
}
</style>
