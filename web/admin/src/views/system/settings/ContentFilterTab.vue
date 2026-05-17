<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()
</script>

<template>
	<div class="tab-content">
		<!-- 过滤策略 -->
		<div class="section">
			<div class="section-title">过滤策略</div>
			<div class="section-grid">
				<AFormItem label="过滤策略" help="off=关闭, log=仅记录, replace=替换, block=拦截">
					<ASelect v-model="values['content_filter_mode']">
						<AOption value="off" label="关闭 (off)" />
						<AOption value="log" label="仅记录 (log)" />
						<AOption value="replace" label="替换敏感词 (replace)" />
						<AOption value="block" label="拦截请求 (block)" />
					</ASelect>
				</AFormItem>
			</div>
		</div>

		<!-- 过滤配置 -->
		<div class="section">
			<div class="section-title">过滤配置</div>
			<AFormItem label="敏感词列表" help="JSON 数组，支持通配符 *">
				<ATextarea
					v-model="values['content_filter_words']"
					:auto-size="{ minRows: 3, maxRows: 8 }"
					placeholder='["敏感词1", "敏感词2"]'
				/>
			</AFormItem>
			<div class="section-grid" style="margin-top: 12px">
				<AFormItem label="替换文本" help="replace 策略下的替换内容">
					<AInput v-model="values['content_filter_replacement']" placeholder="***" />
				</AFormItem>
				<AFormItem label="拦截提示" help="block 策略下的提示信息">
					<AInput v-model="values['content_filter_response']" placeholder="内容包含敏感词，请修改后重试" />
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
