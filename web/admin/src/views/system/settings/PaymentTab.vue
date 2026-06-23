<script setup lang="ts">
import { useFormValues } from './useSettings'
const values = useFormValues()

function syncRate(direction: 'cny_to_usd' | 'usd_to_cny') {
	if (direction === 'cny_to_usd') {
		const usdToCny = Number(values['payment_exchange_rate_usd_to_cny'])
		if (usdToCny > 0) {
			values['payment_exchange_rate_cny_to_usd'] = Math.round((1 / usdToCny) * 10000) / 10000
		}
	} else {
		const cnyToUsd = Number(values['payment_exchange_rate_cny_to_usd'])
		if (cnyToUsd > 0) {
			values['payment_exchange_rate_usd_to_cny'] = Math.round((1 / cnyToUsd) * 10000) / 10000
		}
	}
}
</script>

<template>
	<div class="tab-content">
		<!-- 基础设置 -->
		<div class="section">
			<div class="section-title">基础设置</div>
			<div class="section-grid">
				<AFormItem label="货币单位">
					<ASelect v-model="values['payment_currency']">
						<AOption value="CNY" label="CNY (人民币)" />
						<AOption value="USD" label="USD (美元)" />
					</ASelect>
				</AFormItem>
				<AFormItem label="最低充值金额">
					<AInputNumber
						:model-value="values['payment_min_topup'] as number"
						@change="(v: number | undefined) => values['payment_min_topup'] = v"
						:min="1" style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="支付回调基础 URL" help="为空则使用请求 Host" class="field-full">
					<AInput v-model="values['payment_callback_base_url']" placeholder="https://api.example.com" />
				</AFormItem>
			</div>
		</div>

		<!-- 货币兑换 -->
		<div class="section">
			<div class="section-title">货币兑换</div>
			<div class="section-grid">
				<AFormItem label="CNY → USD" help="1 人民币兑换多少美元">
					<div style="display: flex; align-items: center; gap: 8px; width: 100%">
						<AInputNumber
							:model-value="values['payment_exchange_rate_cny_to_usd'] as number"
							@change="(v: number | undefined) => values['payment_exchange_rate_cny_to_usd'] = v ?? 0"
							:min="0.001" :max="100" :step="0.01" :precision="4"
							style="flex: 1"
						/>
						<AButton type="outline" size="small" @click="syncRate('cny_to_usd')">同步</AButton>
					</div>
				</AFormItem>
				<AFormItem label="USD → CNY" help="1 美元兑换多少人民币">
					<div style="display: flex; align-items: center; gap: 8px; width: 100%">
						<AInputNumber
							:model-value="values['payment_exchange_rate_usd_to_cny'] as number"
							@change="(v: number | undefined) => values['payment_exchange_rate_usd_to_cny'] = v ?? 0"
							:min="0.001" :max="1000" :step="0.01" :precision="4"
							style="flex: 1"
						/>
						<AButton type="outline" size="small" @click="syncRate('usd_to_cny')">同步</AButton>
					</div>
				</AFormItem>
				<div class="field-full" style="font-size: 12px; color: var(--color-text-3); margin-top: -8px">
					点击「同步」可根据另一方向的汇率自动计算当前值（取倒数）
				</div>
			</div>
		</div>

		<!-- 充值选项 -->
		<div class="section">
			<div class="section-title">充值选项</div>
			<div class="section-grid">
				<AFormItem label="充值金额选项" help="JSON 数组，预设充值面额" class="field-full">
					<ATextarea
						v-model="values['payment_amount_options']"
						:auto-size="{ minRows: 2, maxRows: 4 }"
						placeholder='[10, 20, 50, 100, 200, 500]'
					/>
				</AFormItem>
				<AFormItem label="充值折扣" help='JSON 对象，如 {"100":0.9} 表示充100享9折' class="field-full">
					<ATextarea
						v-model="values['payment_amount_discount']"
						:auto-size="{ minRows: 2, maxRows: 4 }"
						placeholder='{"100": 0.9, "500": 0.85}'
					/>
				</AFormItem>
			</div>
		</div>
	</div>
</template>

<style scoped>
@import './common.css';
</style>
