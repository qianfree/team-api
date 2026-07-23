<script setup lang="ts">
import { computed } from 'vue'
import { useFormValues } from './useSettings'
const values = useFormValues()

// 计算 CNY→USD（直接读取存储值）
const computedCnyToUsd = computed(() => {
	const cnyToUsd = Number(values['payment_exchange_rate_cny_to_usd'])
	return cnyToUsd > 0 ? cnyToUsd : 0
})

// 同步 USD→CNY 到底层存储的 CNY→USD
function updateCnyToUsd(usdToCny: number | undefined) {
	if (!usdToCny || usdToCny <= 0) {
		values['payment_exchange_rate_cny_to_usd'] = 0
		return
	}
	// 存储倒数（CNY→USD = 1 / USD→CNY）
	values['payment_exchange_rate_cny_to_usd'] = 1 / usdToCny
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
				<AFormItem label="USD → CNY" help="1 美元兑换多少人民币（常用汇率约 7.0~7.3）" class="field-full">
					<AInputNumber
						:model-value="(1 / (values['payment_exchange_rate_cny_to_usd'] as number || 0.14)) as number"
						@change="updateCnyToUsd"
						:min="0.01" :max="1000" :step="0.01" :precision="4"
						style="width: 100%"
					/>
				</AFormItem>
				<AFormItem label="CNY → USD（自动计算）" help="取上方汇率的倒数，确保往返闭合" class="field-full">
					<AInput :model-value="computedCnyToUsd.toFixed(6)" disabled style="width: 100%" />
				</AFormItem>
				<div class="field-full" style="font-size: 12px; color: var(--color-text-3); margin-top: -8px">
					💡 系统以 USD→CNY 为输入（更符合习惯），内部自动取倒数存储，确保往返换算闭合
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
