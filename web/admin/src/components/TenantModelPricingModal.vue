<!-- 租户模型定价弹窗（参考平台 ModelPricingModal 的交互形态：弹窗 + editor-section 分节）。
     覆盖语义：折扣=叠加（租户倍率×时段倍率×附加乘数）；价格/矩阵/时段/参数倍率=整体替换平台配置；
     时段/倍率支持显式关闭；自定义价格（含矩阵）为一口价，折扣比例与等级折扣均不叠加 -->
<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
// 本位币符号：定价输入控件后缀跟随本位币，输入值仍为 bil 层存储原值不折算
import { currencySymbol, displayCurrency, cnyToUsd, formatBilling } from '@/composables/useCurrency'
import PricingTimeSegmentsEditor, {
  type TimeSegmentRow,
  timeSegmentRowsFromAPI,
  timeSegmentPayloadFrom,
} from './PricingTimeSegmentsEditor.vue'
import PricingParamRulesEditor, {
  type ParamRuleRow,
  paramRuleRowsFromAPI,
  paramRulePayloadFrom,
} from './PricingParamRulesEditor.vue'
import PerSecondMatrixEditor, {
  type PerSecondRow,
  perSecondRowsFromAPI,
  perSecondPayloadFrom,
} from './PerSecondMatrixEditor.vue'

const props = defineProps<{
  visible: boolean
  tenantId: string
  // 编辑目标：租户模型分配行（List API 返回项，含覆盖回显字段与 model_id）
  model: any | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'saved'): void
}>()

const saving = ref(false)
const form = reactive({
  enabled: true,
  billing_mode: null as string | null,
  per_request_price: null as number | null,
  discount_ratio: null as number | null,
  max_concurrency: 5 as number | null,
  custom_input_price: null as number | null,
  custom_output_price: null as number | null,
  custom_cache_read_price: null as number | null,
  custom_cache_creation_price: null as number | null,
  custom_pricing_tiers: [] as any[],
})

let suppressBillingWatch = false

// === 扩展计费覆盖（custom_pricing 补丁三键 + 展示字段）===
// 三态：inherit=继承平台（提交 null=清除覆盖恢复继承）；custom=自定义覆盖；off=显式关闭平台配置（提交 []）
type TriState = 'inherit' | 'custom' | 'off'
const perSecondMode = ref<'inherit' | 'custom'>('inherit') // 矩阵无「关闭」态：不覆盖即继承
const timeSegMode = ref<TriState>('inherit')
const paramMode = ref<TriState>('inherit')

// 行数组（父级持有；回显与提交走纯函数，编辑器 custom 态挂载渲染）
const customTimeSegRows = ref<TimeSegmentRow[]>([])
const customParamRules = ref<ParamRuleRow[]>([])
const customPerSecondRows = ref<PerSecondRow[]>([])
// 时段编辑器开关恒 true（三态由外层 RadioGroup 表达，编辑器只在 custom 态挂载）
const timeSegEditorEnabled = ref(true)

// 展示字段覆盖（空串=继承平台，提交 null）
const editPriceNote = ref('')
const editDiscountLabel = ref('')
const editPriceChangeNote = ref('')

// === 平台定价对照（只读参考 + 驱动矩阵区块可见性）===
const platformPricing = ref<any>(null)
const platformLoading = ref(false)

// 平台是否按秒/特殊计费（决定按秒矩阵覆盖区块可见性；special 由 scheme 声明）
const platformIsPerSecond = computed(() => {
  const mode = platformPricing.value?.list?.[0]?.billing_mode || ''
  const scheme = platformPricing.value?.scheme || ''
  return mode === 'per_second' || mode === 'special' || scheme !== ''
})

const platformModeLabel = computed(() => {
  const scheme = platformPricing.value?.scheme
  if (scheme) return `特殊计费（${scheme}）`
  const mode = platformPricing.value?.list?.[0]?.billing_mode
  const labels: Record<string, string> = { token: '按量', per_request: '按次', tiered: '阶梯', per_second: '按秒' }
  return labels[mode] || '按量'
})

const platformTimeSegCount = computed(() => (platformPricing.value?.time_segments || []).length)
const platformParamRuleCount = computed(() => (platformPricing.value?.param_multipliers || []).length)

// 平台对照行便捷格式化（hover 弹出用）
const platformItem = computed(() => platformPricing.value?.list?.[0])

function fmtPrice(v: number | null | undefined): string {
  return formatBilling(Number(v ?? 0), 6)
}

// 星期数组 → 中文短语（与时段编辑器的预设口径一致）
function dayLabel(days: number[]): string {
  if (!days || days.length === 0) return '每天'
  const names = ['', '周一', '周二', '周三', '周四', '周五', '周六', '周日']
  if (days.length === 5 && [1, 2, 3, 4, 5].every((d) => days.includes(d))) return '工作日'
  if (days.length === 2 && [6, 7].every((d) => days.includes(d))) return '周末'
  return days.map((d) => names[d] || d).join('/')
}

function timeWindow(seg: any): string {
  if (!seg.start_time && !seg.end_time) return '全天'
  return `${seg.start_time || '00:00'}~${seg.end_time || '24:00'}`
}

async function fetchPlatformPricing(modelDBID: number) {
  platformLoading.value = true
  try {
    const res: any = await request.get(`/admin/models/${modelDBID}/pricing`)
    platformPricing.value = res.data?.data || null
  } catch {
    platformPricing.value = null
  } finally {
    platformLoading.value = false
  }
}

// === 官方价轻量参考（只读浮窗 + 同模式直填，折扣换算走平台定价弹窗）===
const officialPopoverVisible = ref(false)
const officialRefLoading = ref(false)
const officialRefData = ref<any>(null)

async function fetchOfficialRef() {
  if (!props.model || officialRefLoading.value) return
  officialRefLoading.value = true
  try {
    const res: any = await request.get(`/admin/models/${props.model.model_id}/official-pricing`)
    officialRefData.value = res.data?.data || null
  } catch {
    officialRefData.value = null
  } finally {
    officialRefLoading.value = false
  }
}

function onOfficialVisibleChange(visible: boolean) {
  if (visible && !officialRefData.value) fetchOfficialRef()
}

// 官方数据源价格恒为美元；本位币=CNY 时按汇率倒数折算（汇率恒为 CNY→USD 单向配置，反向取倒数）
const usdToBaseRatio = computed(() => {
  if (displayCurrency.value !== 'CNY') return 1
  return 1 / cnyToUsd.value
})

function toBasePrice(v: number | null | undefined): number {
  const usd = Number(v ?? 0)
  if (usd <= 0) return 0
  return Math.round(usd * usdToBaseRatio.value * 1e6) / 1e6
}

// 官方价直填：token 四价 / 按次价（per_request 用官方输出价作按次价，与平台定价弹窗口径一致）
function applyOfficialRef(source: any) {
  const p = source?.pricing
  if (!p) return
  if (form.billing_mode === 'per_request') {
    form.per_request_price = toBasePrice(p.output_price)
  } else {
    form.billing_mode = 'token'
    form.custom_input_price = toBasePrice(p.input_price)
    form.custom_output_price = toBasePrice(p.output_price)
    if ((Number(p.cache_read_price) || 0) > 0) form.custom_cache_read_price = toBasePrice(p.cache_read_price)
    if ((Number(p.cache_creation_price) || 0) > 0) form.custom_cache_creation_price = toBasePrice(p.cache_creation_price)
  }
  Message.success(`已填入 ${source.source} 官方价（已折算为本位币）`)
}

// === 一口价判定（与后端 billing.hasTenantCustomPrice 同口径：四价/按次/阶梯/矩阵正价）===
const hasCustomPriceInput = computed(() =>
  (form.custom_input_price ?? 0) > 0 ||
  (form.custom_output_price ?? 0) > 0 ||
  (form.custom_cache_read_price ?? 0) > 0 ||
  (form.custom_cache_creation_price ?? 0) > 0 ||
  (form.per_request_price ?? 0) > 0 ||
  form.custom_pricing_tiers.length > 0 ||
  (perSecondMode.value === 'custom' && customPerSecondRows.value.some((r) => (Number(r.price) || 0) > 0)),
)

function addTenantTier() {
  const tiers = form.custom_pricing_tiers
  const last = tiers[tiers.length - 1]
  const newMin = last?.max_tokens ?? 0
  tiers.push({
    min_tokens: newMin,
    max_tokens: null,
    input_price: 0,
    output_price: 0,
    cache_read_price: 0,
    cache_creation_price: 0,
  })
  if (last && last.max_tokens === null) {
    last.max_tokens = newMin
  }
}

// 计费模式切换时清理互斥字段
watch(() => form.billing_mode, (mode) => {
  if (suppressBillingWatch) return
  if (!mode) {
    // 默认模式：清空自定义价格，保留折扣
    form.custom_input_price = null
    form.custom_output_price = null
    form.custom_cache_read_price = null
    form.custom_cache_creation_price = null
    form.per_request_price = null
    form.custom_pricing_tiers = []
  } else {
    // 自定义价格模式：清空折扣比例
    form.discount_ratio = null
  }
})

// 三态切换时清行数组（off/inherit 不携带残留数据；切回 custom 重新填写）
watch(timeSegMode, () => { if (timeSegMode.value !== 'custom') customTimeSegRows.value = [] })
watch(paramMode, () => { if (paramMode.value !== 'custom') customParamRules.value = [] })
watch(perSecondMode, () => { if (perSecondMode.value === 'inherit') customPerSecondRows.value = [] })

// 弹窗打开：从分配行回显 + 拉平台对照
watch(() => props.visible, (v) => {
  if (!v || !props.model) return
  const record = props.model
  suppressBillingWatch = true
  form.enabled = record.enabled
  form.billing_mode = record.billing_mode || null
  form.per_request_price = record.per_request_price || null
  form.discount_ratio = record.discount_ratio || null
  form.max_concurrency = record.max_concurrency ?? 5
  form.custom_input_price = record.custom_input_price || null
  form.custom_output_price = record.custom_output_price || null
  form.custom_cache_read_price = record.custom_cache_read_price || null
  form.custom_cache_creation_price = record.custom_cache_creation_price || null
  form.custom_pricing_tiers = record.custom_pricing_tiers?.length > 0 ? [...record.custom_pricing_tiers] : []

  // 扩展覆盖回显：null=继承；[]=显式关闭；非空=自定义（纯函数直填行数组）
  const segs: any[] | null = record.custom_time_segments ?? null
  if (Array.isArray(segs)) {
    timeSegMode.value = segs.length > 0 ? 'custom' : 'off'
    customTimeSegRows.value = timeSegmentRowsFromAPI(segs).rows
  } else {
    timeSegMode.value = 'inherit'
    customTimeSegRows.value = []
  }
  const rules: any[] | null = record.custom_param_multipliers ?? null
  if (Array.isArray(rules)) {
    paramMode.value = rules.length > 0 ? 'custom' : 'off'
    customParamRules.value = paramRuleRowsFromAPI(rules)
  } else {
    paramMode.value = 'inherit'
    customParamRules.value = []
  }
  if (record.custom_per_second_prices) {
    perSecondMode.value = 'custom'
    customPerSecondRows.value = perSecondRowsFromAPI(record.custom_per_second_prices)
  } else {
    perSecondMode.value = 'inherit'
    customPerSecondRows.value = []
  }

  editPriceNote.value = record.price_note || ''
  editDiscountLabel.value = record.discount_label || ''
  editPriceChangeNote.value = record.price_change_note || ''

  platformPricing.value = null
  officialRefData.value = null
  suppressBillingWatch = false
  fetchPlatformPricing(record.model_id)
})

async function onSave() {
  if (!props.model) return

  // 时段/倍率/矩阵自定义态先行校验（纯函数，失败中止保存）
  let timeSegPayload: any[] | null = null
  if (timeSegMode.value === 'custom') {
    const result = timeSegmentPayloadFrom(customTimeSegRows.value, true)
    if (result.error || result.payload === undefined) {
      Message.warning(result.error || '时段定价配置有误')
      return
    }
    timeSegPayload = result.payload
  }
  let paramPayload: any[] | null = null
  if (paramMode.value === 'custom') {
    const result = paramRulePayloadFrom(customParamRules.value)
    if (result.error || result.payload === undefined) {
      Message.warning(result.error || '参数倍率配置有误')
      return
    }
    paramPayload = result.payload
  }
  let perSecondPayload: Record<string, number> | null = null
  if (perSecondMode.value === 'custom') {
    const result = perSecondPayloadFrom(customPerSecondRows.value, true)
    if (result.map === null) {
      Message.warning(result.error || '按秒矩阵配置有误')
      return
    }
    perSecondPayload = result.map
  }

  saving.value = true
  try {
    // 恒全量提交覆盖键：inherit=null（清除恢复继承）、off=[]（显式关闭）、custom=编辑载荷
    const payload: any = {
      ...form,
      version: props.model.version,
      custom_per_second_prices: perSecondPayload,
      custom_time_segments: timeSegMode.value === 'custom' ? timeSegPayload
        : timeSegMode.value === 'off' ? [] : null,
      custom_param_multipliers: paramMode.value === 'custom' ? paramPayload
        : paramMode.value === 'off' ? [] : null,
      // 展示字段：空串=清除恢复继承（null），非空=覆盖
      price_note: editPriceNote.value.trim() || null,
      discount_label: editDiscountLabel.value.trim() || null,
      price_change_note: editPriceChangeNote.value.trim() || null,
    }
    await request.put(`/admin/tenants/${props.tenantId}/models/${props.model.model_id}`, payload)
    Message.success('保存成功')
    emit('update:visible', false)
    emit('saved')
  } catch {
    // 错误已由拦截器统一提示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AModal
    :visible="visible"
    :width="'min(880px, 96vw)'"
    :mask-closable="false"
    :esc-to-close="false"
    :footer="true"
    :body-style="{ maxHeight: '72vh', overflowY: 'auto' }"
    @cancel="emit('update:visible', false)"
  >
    <template #title>
      <div class="modal-title-row">
        <span>{{ model ? `模型定价 - ${model.model_code}` : '模型定价' }}</span>
        <!-- 平台配置对照标识：悬浮展示平台全局定价（未覆盖各维度继承的就是这些值） -->
        <APopover trigger="hover" position="br">
          <ATag size="small" class="platform-ref-tag">平台配置</ATag>
          <template #content>
            <div class="platform-ref-pop">
              <ASpin :loading="platformLoading" style="width: 100%">
                <template v-if="platformPricing">
                  <div class="platform-ref-pop-title">平台定价（未覆盖维度继承以下配置）</div>
                  <div class="platform-ref-grid">
                    <div class="platform-ref-item">
                      <span class="platform-ref-label">计费模式</span>
                      <span>{{ platformModeLabel }}</span>
                    </div>
                    <div class="platform-ref-item">
                      <span class="platform-ref-label">折扣标签</span>
                      <span>{{ platformPricing.discount_label || '-' }}</span>
                    </div>
                  </div>

                  <!-- 模式价格 -->
                  <template v-if="platformItem">
                    <!-- token：四价（cache_creation_price 兼容 Claude 创建与 OpenAI 写入口径） -->
                    <div v-if="!platformItem.billing_mode || platformItem.billing_mode === 'token'" class="platform-ref-grid platform-ref-section">
                      <div class="platform-ref-item"><span class="platform-ref-label">输入价</span><span>{{ fmtPrice(platformItem.input_price) }} {{ currencySymbol }}/1M</span></div>
                      <div class="platform-ref-item"><span class="platform-ref-label">输出价</span><span>{{ fmtPrice(platformItem.output_price) }} {{ currencySymbol }}/1M</span></div>
                      <div class="platform-ref-item"><span class="platform-ref-label">缓存读</span><span>{{ fmtPrice(platformItem.cache_read_price) }} {{ currencySymbol }}/1M</span></div>
                      <div class="platform-ref-item"><span class="platform-ref-label">缓存写</span><span>{{ fmtPrice(platformItem.cache_creation_price) }} {{ currencySymbol }}/1M</span></div>
                    </div>
                    <!-- 按次 -->
                    <div v-else-if="platformItem.billing_mode === 'per_request'" class="platform-ref-grid platform-ref-section">
                      <div class="platform-ref-item"><span class="platform-ref-label">按次单价</span><span>{{ fmtPrice(platformItem.per_request_price) }} {{ currencySymbol }}/次</span></div>
                    </div>
                    <!-- 阶梯：逐档 -->
                    <div v-else-if="platformItem.billing_mode === 'tiered'" class="platform-ref-section">
                      <div v-for="(t, i) in platformPricing.list" :key="i" class="platform-ref-line">
                        第{{ i + 1 }}梯 {{ t.min_tokens }}~{{ t.max_tokens ?? '∞' }}：入 {{ fmtPrice(t.input_price) }} / 出 {{ fmtPrice(t.output_price) }} {{ currencySymbol }}/1M
                      </div>
                    </div>
                    <!-- 按秒/特殊：矩阵 -->
                    <div v-if="platformItem.per_second_prices && Object.keys(platformItem.per_second_prices).length" class="platform-ref-section">
                      <span class="platform-ref-label">按秒矩阵</span>
                      <span class="platform-ref-matrix">
                        <ATag v-for="(price, spec) in platformItem.per_second_prices" :key="spec" size="small">
                          {{ spec }}: {{ fmtPrice(price as number) }}{{ currencySymbol }}/秒
                        </ATag>
                      </span>
                    </div>
                  </template>

                  <!-- 时段 -->
                  <div class="platform-ref-section">
                    <span class="platform-ref-label">时段定价（{{ platformTimeSegCount }} 条）</span>
                    <template v-if="platformTimeSegCount">
                      <div v-for="(seg, i) in platformPricing.time_segments" :key="i" class="platform-ref-line">
                        {{ seg.name || `时段${i + 1}` }} ×{{ seg.multiplier }}（{{ dayLabel(seg.days) }} {{ timeWindow(seg) }}）
                      </div>
                    </template>
                    <div v-else class="platform-ref-line platform-ref-empty">未配置</div>
                  </div>

                  <!-- 参数倍率 -->
                  <div class="platform-ref-section">
                    <span class="platform-ref-label">参数倍率（{{ platformParamRuleCount }} 条）</span>
                    <template v-if="platformParamRuleCount">
                      <div v-for="(rule, i) in platformPricing.param_multipliers" :key="i" class="platform-ref-line">
                        ×{{ rule.multiplier }}：{{ (rule.conditions || []).map((c: any) => `${c.path} ${c.match} ${c.value ?? ''}`).join(' && ') }}{{ rule.note ? `（${rule.note}）` : '' }}
                      </div>
                    </template>
                    <div v-else class="platform-ref-line platform-ref-empty">未配置</div>
                  </div>
                </template>
                <div v-else class="platform-ref-line platform-ref-empty">平台定价加载失败，重新打开弹窗可重试</div>
              </ASpin>
            </div>
          </template>
        </APopover>
      </div>
    </template>
    <template #footer>
      <div class="pricing-footer">
        <span class="pricing-footer-hint">折扣与平台时段/参数倍率叠加；自定义价格为一口价</span>
        <div class="pricing-footer-actions">
          <AButton @click="emit('update:visible', false)">取消</AButton>
          <AButton type="primary" :loading="saving" @click="onSave">保存</AButton>
        </div>
      </div>
    </template>

    <AForm v-if="model" layout="vertical">
      <div class="pricing-editor">
        <!-- 基础配置：单行内联，不占独立分节 -->
        <div class="base-config-row">
          <AFormItem label="启用">
            <ASwitch v-model="form.enabled" />
          </AFormItem>
          <AFormItem label="单模型并发上限">
            <AInputNumber v-model="form.max_concurrency" :min="0" placeholder="默认5" style="width: 200px" />
          </AFormItem>
        </div>
        <ADivider margin="4px" />

        <!-- 计费模式 -->
        <div class="editor-section">
          <div class="editor-section-header">
            <h3>计费模式</h3>
            <span class="section-hint">默认 = 跟随平台模式（可用折扣比例快捷定价）</span>
          </div>
          <div class="billing-mode-row">
            <ARadioGroup v-model="form.billing_mode" type="button">
              <ARadio value="">默认</ARadio>
              <ARadio value="token">Token</ARadio>
              <ARadio value="per_request">按次</ARadio>
              <ARadio value="tiered">阶梯</ARadio>
            </ARadioGroup>
          </div>

          <!-- 默认模式：折扣快捷方式 -->
          <template v-if="!form.billing_mode">
            <div class="section-note">
              按平台定价 × 折扣比例计费；折扣与平台时段/参数倍率叠加生效
            </div>
            <AFormItem label="折扣比例">
              <AInputNumber v-model="form.discount_ratio" :min="0" :max="1" :step="0.05" :precision="2" placeholder="如 0.8 = 8折" class="w-full" />
            </AFormItem>
          </template>

          <!-- 一口价提示：自定义价格与折扣互斥（防折上折）——等级折扣是隐式配置，
               不填折扣比例也会被屏蔽，填了任一自定义价就提示 -->
          <div
            v-if="hasCustomPriceInput"
            class="fixed-price-banner"
          >
            自定义价格为一口价：不与折扣比例、租户等级折扣叠加——配置自定义价格后不再乘等级折扣系数，最终价即自定义价格
          </div>

          <!-- Token 模式 -->
          <template v-if="form.billing_mode === 'token'">
            <div class="billing-mode-tools-row">
              <div class="section-note" style="margin-bottom: 0">留空则使用模型基础定价</div>
              <!-- 官方价轻量参考（只读 + 直填，完整折扣换算在平台「模型管理 → 定价」） -->
              <APopover v-model:popup-visible="officialPopoverVisible" trigger="click" position="tr">
                <AButton size="mini" type="text">官方价参考</AButton>
                <template #content>
                  <div style="max-width: 380px">
                    <div style="font-weight: 600; margin-bottom: 6px">官方定价（USD / 1M tokens）</div>
                    <ASpin :loading="officialRefLoading" style="width: 100%">
                      <div v-if="(officialRefData?.sources || []).some((s: any) => s.found)">
                        <div v-for="src in officialRefData.sources" :key="src.source" style="margin-bottom: 8px">
                          <div v-if="src.found && src.pricing" class="flex items-center justify-between" style="gap: 8px">
                            <span style="min-width: 70px">{{ src.source }}</span>
                            <span style="flex: 1; font-size: 12px; color: var(--ta-text-tertiary)">
                              in {{ src.pricing.input_price }} / out {{ src.pricing.output_price }}
                            </span>
                            <AButton size="mini" type="outline" @click="applyOfficialRef(src)">填入</AButton>
                          </div>
                        </div>
                      </div>
                      <div v-else style="color: var(--ta-text-tertiary); font-size: 12px">各数据源均未收录该模型</div>
                    </ASpin>
                  </div>
                </template>
              </APopover>
            </div>
            <div class="grid grid-cols-2 md:grid-cols-4 gap-x-3">
              <AFormItem label="输入价格">
                <AInputNumber v-model="form.custom_input_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="输出价格">
                <AInputNumber v-model="form.custom_output_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="缓存读取">
                <AInputNumber v-model="form.custom_cache_read_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="缓存创建">
                <AInputNumber v-model="form.custom_cache_creation_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
            </div>
          </template>

          <!-- 按次计费 -->
          <template v-if="form.billing_mode === 'per_request'">
            <div class="section-note">自定义按次单价为一口价，不再乘折扣比例与租户等级折扣系数</div>
            <AFormItem label="按次单价">
              <AInputNumber v-model="form.per_request_price" :min="0" :precision="4" placeholder="每次调用价格" class="w-full">
                <template #suffix>{{ currencySymbol }} / 次</template>
              </AInputNumber>
            </AFormItem>
          </template>

          <!-- 阶梯计费 -->
          <template v-if="form.billing_mode === 'tiered'">
            <div class="section-note">按 Token 用量分段设置不同价格，留空则使用模型基础阶梯定价；自定义阶梯为一口价，不再乘折扣比例与租户等级折扣系数</div>
            <div v-for="(tier, index) in form.custom_pricing_tiers" :key="index" class="tier-card">
              <div class="tier-header">
                <span class="tier-label">第 {{ index + 1 }} 梯</span>
                <AButton v-if="form.custom_pricing_tiers.length > 1" size="mini" status="danger" @click="form.custom_pricing_tiers.splice(index, 1)">删除</AButton>
              </div>
              <div class="grid grid-cols-2 gap-x-3">
                <AFormItem label="起始 Token">
                  <AInputNumber v-model="tier.min_tokens" :min="0" :step="1000" placeholder="0" class="w-full" />
                </AFormItem>
                <AFormItem label="结束 Token">
                  <AInputNumber v-if="index < form.custom_pricing_tiers.length - 1" v-model="tier.max_tokens" :min="0" :step="1000" placeholder="上限" class="w-full" />
                  <AInput v-else model-value="无上限" disabled class="w-full" />
                </AFormItem>
              </div>
              <div class="grid grid-cols-2 gap-x-3 mt-2">
                <AFormItem label="输入价格">
                  <AInputNumber v-model="tier.input_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="输出价格">
                  <AInputNumber v-model="tier.output_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
              </div>
              <div class="grid grid-cols-2 gap-x-3 mt-2">
                <AFormItem label="缓存读取价格">
                  <AInputNumber v-model="tier.cache_read_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="缓存创建价格">
                  <AInputNumber v-model="tier.cache_creation_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
              </div>
            </div>
            <AButton type="dashed" long @click="addTenantTier" class="mt-2">+ 添加梯度</AButton>
          </template>
        </div>

        <!-- 按秒矩阵覆盖：仅平台 per_second/special 模型显示（覆盖后为一口价，折扣不叠加） -->
        <div v-if="platformIsPerSecond" class="editor-section">
          <div class="editor-section-header">
            <h3>按秒矩阵</h3>
            <ARadioGroup v-model="perSecondMode" type="button" size="small" class="header-radio">
              <ARadio value="inherit">继承平台</ARadio>
              <ARadio value="custom">自定义（一口价）</ARadio>
            </ARadioGroup>
          </div>
          <div v-if="perSecondMode === 'inherit'" class="section-note">
            租户按平台矩阵计费；折扣比例（若配置）叠加在矩阵价上
          </div>
          <template v-else>
            <div class="section-note">
              整体替换平台矩阵，最终价即自定义价——折扣比例与等级折扣不再叠加
            </div>
            <PerSecondMatrixEditor :rows="customPerSecondRows" />
          </template>
        </div>

        <!-- 时段定价覆盖（三态：继承/自定义/显式关闭） -->
        <div class="editor-section">
          <div class="editor-section-header">
            <h3>时段定价</h3>
            <span class="section-hint">平台已配 {{ platformTimeSegCount }} 条</span>
            <ARadioGroup v-model="timeSegMode" type="button" size="small" class="header-radio">
              <ARadio value="inherit">继承</ARadio>
              <ARadio value="custom">自定义</ARadio>
              <ARadio value="off">关闭</ARadio>
            </ARadioGroup>
          </div>
          <div v-if="timeSegMode === 'inherit'" class="section-note">
            平台时段定价对该租户照常生效；平台未配置时段时等效于无时段乘数
          </div>
          <div v-else-if="timeSegMode === 'off'" class="section-note">
            显式关闭：即使平台配置了时段定价，该租户也不应用时段乘数
          </div>
          <PricingTimeSegmentsEditor v-else v-model:enabled="timeSegEditorEnabled" :segments="customTimeSegRows" />
        </div>

        <!-- 参数倍率覆盖（三态） -->
        <div class="editor-section">
          <div class="editor-section-header">
            <h3>参数倍率</h3>
            <span class="section-hint">平台已配 {{ platformParamRuleCount }} 条</span>
            <ARadioGroup v-model="paramMode" type="button" size="small" class="header-radio">
              <ARadio value="inherit">继承</ARadio>
              <ARadio value="custom">自定义</ARadio>
              <ARadio value="off">关闭</ARadio>
            </ARadioGroup>
          </div>
          <div v-if="paramMode === 'inherit'" class="section-note">
            平台参数倍率规则（按任务参数命中连乘）对该租户照常生效
          </div>
          <div v-else-if="paramMode === 'off'" class="section-note">
            显式关闭：该租户的任务请求不应用任何参数倍率
          </div>
          <PricingParamRulesEditor v-else :rules="customParamRules" />
        </div>

        <!-- 展示信息（对外两列；内部说明 textarea，与平台定价弹窗同排版） -->
        <div class="editor-section">
          <div class="editor-section-header">
            <h3>展示信息</h3>
            <span class="section-hint">租户端模型列表展示；留空使用平台配置</span>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-4">
            <AFormItem label="折扣标签（对外展示）">
              <AInput v-model="editDiscountLabel" :max-length="50" allow-clear :placeholder="platformPricing?.discount_label ? `平台：${platformPricing.discount_label}` : '如 7折起'" />
            </AFormItem>
            <AFormItem label="价格调整说明（对外展示）">
              <AInput v-model="editPriceChangeNote" :max-length="200" allow-clear :placeholder="platformPricing?.price_change_note ? `平台：${platformPricing.price_change_note}` : '提示价格有变动'" />
            </AFormItem>
          </div>
          <AFormItem label="价格说明（仅内部可见）">
            <ATextarea
              v-model="editPriceNote"
              :max-length="500"
              :auto-size="{ minRows: 2, maxRows: 4 }"
              :placeholder="platformPricing?.price_note ? `平台：${platformPricing.price_note}` : '调价背景等内部备注，不对外展示'"
            />
          </AFormItem>
        </div>
      </div>
    </AForm>
  </AModal>
</template>

<style scoped>
.pricing-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pricing-footer-hint {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.pricing-footer-actions {
  display: flex;
  gap: 8px;
}

.pricing-editor {
  padding: 4px 0;
}

.editor-section {
  margin-bottom: 24px;
}

.editor-section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.editor-section-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin: 0;
}

/* 三态切换按钮组：分节标题行右对齐（标题左、说明中、按钮右） */
.header-radio {
  margin-left: auto;
  flex-shrink: 0;
}

/* 基础配置内联行：启用 + 并发上限一行排布 */
.base-config-row {
  display: flex;
  align-items: flex-end;
  gap: 24px;
}

.base-config-row :deep(.arco-form-item) {
  margin-bottom: 8px;
}

.section-hint {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.section-note {
  font-size: 12px;
  color: var(--ta-text-tertiary);
  margin-bottom: 12px;
}

.billing-mode-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.billing-mode-tools-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 12px;
}

/* 一口价提示条：自定义价格与折扣互斥（防折上折） */
.fixed-price-banner {
  margin-bottom: 12px;
  padding: 6px 10px;
  border-radius: 4px;
  background: var(--ta-bg-secondary, #f7f8fa);
  color: var(--ta-text-secondary);
  font-size: 12px;
}

.tier-card {
  padding: 12px 16px;
  background: var(--color-fill-1);
  border-radius: 8px;
  margin-bottom: 8px;
}

.tier-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.tier-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary);
}

/* 标题行：标题 + 右侧平台配置对照标识 */
.modal-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-right: 24px;
}

.platform-ref-tag {
  cursor: help;
  color: var(--color-text-2);
  background: var(--color-fill-2);
}

/* 平台配置 hover 弹出层（teleport 到 body，样式需全局作用域） */
.platform-ref-pop {
  max-width: 420px;
  font-size: 12px;
}

.platform-ref-pop-title {
  font-weight: 600;
  color: var(--ta-text-primary);
  margin-bottom: 8px;
}

.platform-ref-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 6px 16px;
}

.platform-ref-item {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.platform-ref-label {
  color: var(--ta-text-tertiary);
}

.platform-ref-section {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.platform-ref-line {
  color: var(--ta-text-secondary);
  line-height: 1.5;
}

.platform-ref-empty {
  color: var(--ta-text-tertiary);
}

.platform-ref-matrix {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>

<style>
/* 弹出层内容 teleport 到 body，scoped 属性选择器不生效，此处全局兜底 */
.platform-ref-pop .arco-spin {
  display: block;
}
</style>
