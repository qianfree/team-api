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

// === 定价方式三态（互斥，与后端 hasTenantCustomPrice 一口价判定同口径）===
// inherit=继承平台价；discount=平台价×折扣；custom=专属价（PricingBlob 覆盖，一口价不叠加折扣）
const pricingMode = ref<'inherit' | 'discount' | 'custom'>('inherit')
// 折扣快捷档（discount 模式）
const discountPresets = [0.95, 0.9, 0.85, 0.8, 0.5]
// 折扣档中文标签：0.9 → 9折、0.85 → 85折
function discountLabel(d: number): string {
  const pct = Math.round(d * 100)
  return `${pct % 10 === 0 ? pct / 10 : pct}折`
}

const form = reactive({
  enabled: true,
  discount_ratio: null as number | null,
  max_concurrency: 5 as number | null,
  // custom 专属价（PricingBlob 键，null/0=该维度继承平台）
  billing_mode: null as string | null, // null=跟随平台模式
  input_price: null as number | null,
  output_price: null as number | null,
  cache_read_price: null as number | null,
  cache_creation_price: null as number | null,
  cache_creation_1h_price: null as number | null,
  per_request_price: null as number | null,
  tiers: [] as any[],
})

// === 扩展计费覆盖（custom_pricing blob 横切键 + 展示字段）===
// 三态：inherit=继承平台（键缺失=清除覆盖恢复继承）；custom=自定义覆盖；off=显式关闭平台配置（提交 []）
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

// 计费模式单选框的 v-model 适配：Arco ARadio 的 value 不接受 null（回退为默认值 true），
// 「跟随平台」用字符串哨兵承载，读写时与 form.billing_mode 的 null 互转
const FOLLOW_PLATFORM = '__platform__'
const billingModeRadio = computed<string>({
  get: () => form.billing_mode || FOLLOW_PLATFORM,
  set: (v) => { form.billing_mode = v === FOLLOW_PLATFORM ? null : v },
})

// 平台计费模式（scheme 声明的特殊方案归 special）
const platformMode = computed<string | null>(() => {
  if (platformPricing.value?.scheme) return 'special'
  return platformPricing.value?.list?.[0]?.billing_mode || null
})

// 回显装载的租户覆盖 blob（推断「跟随平台」时的展示模式用）
const loadedPricing = ref<any>(null)

// 按 blob 已有价格键推断计费模式：与旧数据落库时的含义一致（存量行常见「模式跟随平台 + 扁平价」）
function inferModeFromBlob(p: any): string | null {
  if (!p) return null
  if (Array.isArray(p.tiers) && p.tiers.length > 0) return 'tiered'
  if ((p.price ?? null) !== null) return 'per_request'
  if (['input_price', 'output_price', 'cache_read_price', 'cache_creation_price', 'cache_creation_1h_price']
    .some((k) => (p[k] ?? null) !== null)) return 'token'
  return null
}

// 价格区块的展示模式。与 form.billing_mode（提交给后端的覆盖模式）解耦：
// 「跟随平台」不写 billing_mode 键，但价格输入仍须按实际生效模式展示——否则存量
// 「跟随平台 + 专属价」行的价格框被隐藏、保存时漏传价格键，后端全量替换即静默清价
const displayMode = computed<string>(() =>
  form.billing_mode || inferModeFromBlob(loadedPricing.value) || platformMode.value || 'token',
)

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

// 官方价直填：token 四价 / 按次价（per_request 用官方输出价作按次价，与平台定价弹窗口径一致）。
// 直填即切专属价模式（官方价是绝对价，不与折扣叠加）
function applyOfficialRef(source: any) {
  const p = source?.pricing
  if (!p) return
  pricingMode.value = 'custom'
  if (displayMode.value === 'per_request') {
    form.per_request_price = toBasePrice(p.output_price)
  } else {
    // 已按 token 展示（含跟随平台）时不改覆盖模式，避免把「跟随平台」写死为 token
    if (displayMode.value !== 'token') form.billing_mode = 'token'
    form.input_price = toBasePrice(p.input_price)
    form.output_price = toBasePrice(p.output_price)
    if ((Number(p.cache_read_price) || 0) > 0) form.cache_read_price = toBasePrice(p.cache_read_price)
    if ((Number(p.cache_creation_price) || 0) > 0) form.cache_creation_price = toBasePrice(p.cache_creation_price)
    if ((Number(p.cache_creation_1h_price) || 0) > 0) form.cache_creation_1h_price = toBasePrice(p.cache_creation_1h_price)
  }
  Message.success(`已填入 ${source.source} 官方价（已折算为本位币）`)
}

// === 一口价判定（与后端 hasTenantCustomPrice 同口径：五价/按次/阶梯/矩阵正价）===
const hasCustomPriceInput = computed(() =>
  (form.input_price ?? 0) > 0 ||
  (form.output_price ?? 0) > 0 ||
  (form.cache_read_price ?? 0) > 0 ||
  (form.cache_creation_price ?? 0) > 0 ||
  (form.cache_creation_1h_price ?? 0) > 0 ||
  (form.per_request_price ?? 0) > 0 ||
  form.tiers.length > 0 ||
  (perSecondMode.value === 'custom' && customPerSecondRows.value.some((r) => (Number(r.price) || 0) > 0)),
)

// 折扣模式生效价预览：平台价 × 折扣逐项展示（token 模式五价；其他模式显示提示）
const discountPreview = computed(() => {
  if (pricingMode.value !== 'discount' || !form.discount_ratio || form.discount_ratio <= 0) return []
  const p = platformItem.value
  if (!p) return []
  const rows: Array<{ label: string; base: number; eff: number }> = []
  const mul = form.discount_ratio
  const push = (label: string, v: number | undefined | null) => {
    const base = Number(v ?? 0)
    if (base > 0) rows.push({ label, base, eff: Math.round(base * mul * 1e6) / 1e6 })
  }
  push('输入', p.input_price)
  push('输出', p.output_price)
  push('缓存读', p.cache_read_price)
  push('缓存写', p.cache_creation_price)
  push('缓存写(长时)', p.cache_creation_1h_price)
  push('按次', p.per_request_price)
  return rows
})

function addTenantTier() {
  const tiers = form.tiers
  const last = tiers[tiers.length - 1]
  const newMin = last?.max_tokens ?? 0
  tiers.push({
    min_tokens: newMin,
    max_tokens: null,
    input_price: 0,
    output_price: 0,
    cache_read_price: 0,
    cache_creation_price: 0,
    cache_creation_1h_price: 0,
  })
  if (last && last.max_tokens === null) {
    last.max_tokens = newMin
  }
}

// 定价方式/计费模式/三态切换均不清理表单字段：误切换不丢已填数据，切回即恢复。
// 互斥与拣选在保存时收口——折扣仅 pricingMode=discount 时提交、价格键仅按
// displayMode 对应组提交、时段/参数仅 custom/off 态提交（见 onSave），
// 残留的其他组字段不会入库，不会产生「专属价叠加折扣」的混合配置

// blob 是否含价格键（定价方式回显判定：有价格键=custom；无但有折扣=discount；否则 inherit）
function blobHasPriceKey(p: any): boolean {
  if (!p) return false
  return ['input_price', 'output_price', 'cache_read_price', 'cache_creation_price', 'cache_creation_1h_price', 'price', 'tiers', 'prices'].some((k) => {
    const v = p[k]
    if (Array.isArray(v)) return v.length > 0
    if (v !== null && typeof v === 'object') return Object.keys(v).length > 0
    return v !== null && v !== undefined && v !== ''
  })
}

// 弹窗打开：从分配行回显 + 拉平台对照
watch(() => props.visible, (v) => {
  if (!v || !props.model) return
  const record = props.model
  form.enabled = record.enabled
  form.discount_ratio = record.discount_ratio || null
  form.max_concurrency = record.max_concurrency ?? 5

  // 定价覆盖回显（record.pricing = custom_pricing blob 的 API 形态）
  const p: any = record.pricing || null
  loadedPricing.value = p
  const hasPrice = blobHasPriceKey(p)
  pricingMode.value = hasPrice ? 'custom' : (record.discount_ratio ? 'discount' : 'inherit')
  form.billing_mode = p?.billing_mode || null
  form.input_price = p?.input_price ?? null
  form.output_price = p?.output_price ?? null
  form.cache_read_price = p?.cache_read_price ?? null
  form.cache_creation_price = p?.cache_creation_price ?? null
  form.cache_creation_1h_price = p?.cache_creation_1h_price ?? null
  form.per_request_price = p?.price ?? null
  form.tiers = p?.tiers?.length > 0 ? [...p.tiers] : []

  // 扩展覆盖回显：null=继承；[]=显式关闭；非空=自定义（纯函数直填行数组）
  const segs: any[] | null = p?.time_segments ?? null
  if (Array.isArray(segs)) {
    timeSegMode.value = segs.length > 0 ? 'custom' : 'off'
    customTimeSegRows.value = timeSegmentRowsFromAPI(segs).rows
  } else {
    timeSegMode.value = 'inherit'
    customTimeSegRows.value = []
  }
  const rules: any[] | null = p?.param_multipliers ?? null
  if (Array.isArray(rules)) {
    paramMode.value = rules.length > 0 ? 'custom' : 'off'
    customParamRules.value = paramRuleRowsFromAPI(rules)
  } else {
    paramMode.value = 'inherit'
    customParamRules.value = []
  }
  if (p?.prices && Object.keys(p.prices).length > 0) {
    perSecondMode.value = 'custom'
    customPerSecondRows.value = perSecondRowsFromAPI(p.prices)
  } else {
    perSecondMode.value = 'inherit'
    customPerSecondRows.value = []
  }

  editPriceNote.value = record.price_note || ''
  editDiscountLabel.value = record.discount_label || ''
  editPriceChangeNote.value = record.price_change_note || ''

  platformPricing.value = null
  officialRefData.value = null
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
  // 折扣模式必填校验：未填比例时提交 null 会被后端解释为「清除折扣」，
  // 与管理员意图（设置折扣）相悖，前端拦截
  if (pricingMode.value === 'discount' && !(form.discount_ratio ?? 0)) {
    Message.warning('请填写折扣比例（0-1 之间）')
    return
  }

  // 构造 pricing 覆盖对象（PricingBlob 形状，全量替换 custom_pricing：
  // 键缺失=继承平台；custom 模式关闭时价格键全清仅保留横切键）
  const pricing: any = {}
  if (timeSegMode.value === 'custom') pricing.time_segments = timeSegPayload
  else if (timeSegMode.value === 'off') pricing.time_segments = []
  if (paramMode.value === 'custom') pricing.param_multipliers = paramPayload
  else if (paramMode.value === 'off') pricing.param_multipliers = []
  if (pricingMode.value === 'custom') {
    // billing_mode 键只在显式选择时写入（null=跟随平台，不得写死为当前平台模式）；
    // 价格键按展示模式取，保证「跟随平台 + 专属价」的存量行保存后价格不丢
    if (['token', 'tiered', 'per_request'].includes(form.billing_mode as string)) {
      pricing.billing_mode = form.billing_mode
    }
    const mode = displayMode.value
    if (mode === 'token') {
      if ((form.input_price ?? 0) > 0) pricing.input_price = form.input_price
      if ((form.output_price ?? 0) > 0) pricing.output_price = form.output_price
      if ((form.cache_read_price ?? 0) > 0) pricing.cache_read_price = form.cache_read_price
      if ((form.cache_creation_price ?? 0) > 0) pricing.cache_creation_price = form.cache_creation_price
      if ((form.cache_creation_1h_price ?? 0) > 0) pricing.cache_creation_1h_price = form.cache_creation_1h_price
    }
    if (mode === 'per_request' && (form.per_request_price ?? 0) > 0) {
      pricing.price = form.per_request_price
    }
    if (mode === 'tiered' && form.tiers.length > 0) {
      pricing.tiers = form.tiers
    }
    if (perSecondPayload) pricing.prices = perSecondPayload
  }

  saving.value = true
  try {
    const payload: any = {
      enabled: form.enabled,
      max_concurrency: form.max_concurrency,
      version: props.model.version,
      // 定价方式三态：discount 提交折扣值；inherit/custom 清折扣（互斥，专属价不打折）
      discount_ratio: pricingMode.value === 'discount' ? (form.discount_ratio ?? null) : null,
      pricing,
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
                      <div class="platform-ref-item"><span class="platform-ref-label">缓存写(长时)</span><span>{{ fmtPrice(platformItem.cache_creation_1h_price) }} {{ currencySymbol }}/1M</span></div>
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

        <!-- 定价方式（三态互斥：继承 / 打折 / 专属价） -->
        <div class="editor-section">
          <div class="editor-section-header">
            <h3>定价方式</h3>
            <span class="section-hint">继承平台 / 平台价打折 / 专属定价，三选一（专属价与折扣互斥）</span>
          </div>
          <div class="billing-mode-row">
            <ARadioGroup v-model="pricingMode" type="button">
              <ARadio value="inherit">继承平台价</ARadio>
              <ARadio value="discount">平台价打折</ARadio>
              <ARadio value="custom">专属定价</ARadio>
            </ARadioGroup>
          </div>

          <!-- 打折模式：快捷档 + 实时生效价预览 -->
          <template v-if="pricingMode === 'discount'">
            <div class="section-note">
              最终价 = 平台定价 × 折扣比例；与平台时段/参数倍率叠加生效
            </div>
            <div class="discount-preset-row">
              <AButton
                v-for="d in discountPresets"
                :key="d"
                size="small"
                :type="form.discount_ratio === d ? 'primary' : 'outline'"
                @click="form.discount_ratio = d"
              >{{ discountLabel(d) }}</AButton>
            </div>
            <AFormItem label="折扣比例">
              <AInputNumber v-model="form.discount_ratio" :min="0.01" :max="1" :step="0.05" :precision="2" placeholder="如 0.8 = 8折" class="w-full" />
            </AFormItem>
            <div v-if="discountPreview.length" class="discount-preview">
              <div v-for="row in discountPreview" :key="row.label" class="discount-preview-row">
                <span class="discount-preview-label">{{ row.label }}</span>
                <span>{{ fmtPrice(row.base) }} → <b>{{ fmtPrice(row.eff) }}</b> {{ currencySymbol }}/1M</span>
              </div>
            </div>
          </template>

          <!-- 专属定价模式：一口价（不叠加折扣） -->
          <template v-if="pricingMode === 'custom'">
            <div class="fixed-price-banner">
              专属价为一口价：不与折扣比例、租户等级折扣叠加——最终价即所填价格
            </div>
            <div class="billing-mode-row">
              <ARadioGroup v-model="billingModeRadio" type="button">
                <ARadio :value="FOLLOW_PLATFORM">跟随平台模式</ARadio>
                <ARadio value="token">Token</ARadio>
                <ARadio value="per_request">按次</ARadio>
                <ARadio value="tiered">阶梯</ARadio>
              </ARadioGroup>
            </div>
            <div v-if="!form.billing_mode" class="section-note">
              跟随平台模式（当前：{{ platformModeLabel }}），平台调整计费模式时租户随之切换；下方按当前生效模式填写专属价
            </div>

          <!-- Token 模式 -->
          <template v-if="displayMode === 'token'">
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
            <!-- 价格按 6 列栅格等分（每项占 2 列）：第一行输入/输出，第二行缓存三项，与平台定价弹窗排布一致 -->
            <div class="price-grid">
              <AFormItem label="输入价格">
                <AInputNumber v-model="form.input_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="输出价格">
                <AInputNumber v-model="form.output_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
            </div>
            <div class="price-grid">
              <AFormItem label="缓存读取">
                <AInputNumber v-model="form.cache_read_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="缓存创建">
                <AInputNumber v-model="form.cache_creation_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
              <AFormItem label="长时缓存创建">
                <AInputNumber v-model="form.cache_creation_1h_price" :min="0" :precision="4" placeholder="默认" class="w-full">
                  <template #suffix>{{ currencySymbol }} / 1M</template>
                </AInputNumber>
              </AFormItem>
            </div>
            <p class="section-footnote">
              「缓存创建」为短 TTL 桶（Claude 5m / GPT 30m），「长时缓存创建」为长 TTL 桶（Claude 1h / GPT 12h），留空继承平台价（平台未配置时按基准价 ×1.6 推导）。
            </p>
          </template>

          <!-- 按次计费 -->
          <template v-if="displayMode === 'per_request'">
            <div class="section-note">自定义按次单价为一口价，不再乘折扣比例与租户等级折扣系数</div>
            <AFormItem label="按次单价">
              <AInputNumber v-model="form.per_request_price" :min="0" :precision="4" placeholder="每次调用价格" class="w-full">
                <template #suffix>{{ currencySymbol }} / 次</template>
              </AInputNumber>
            </AFormItem>
          </template>

          <!-- 阶梯计费 -->
          <template v-if="displayMode === 'tiered'">
            <div class="section-note">按 Token 用量分段设置不同价格，留空则使用模型基础阶梯定价；自定义阶梯为一口价，不再乘折扣比例与租户等级折扣系数</div>
            <div v-for="(tier, index) in form.tiers" :key="index" class="tier-card">
              <!-- Token 区间内联在档位标题行：标题行管区间划分，下方网格只管本档价格 -->
              <div class="tier-header">
                <div class="tier-range">
                  <span class="tier-label">第 {{ index + 1 }} 梯</span>
                  <AInputNumber v-model="tier.min_tokens" :min="0" :step="1000" placeholder="起始" size="small" class="tier-range-input" />
                  <span class="tier-range-sep">~</span>
                  <AInputNumber v-if="index < form.tiers.length - 1" v-model="tier.max_tokens" :min="0" :step="1000" placeholder="上限" size="small" class="tier-range-input" />
                  <span v-else class="tier-range-unlimited" title="末档无上限">无上限</span>
                  <span class="tier-range-unit">tokens</span>
                </div>
                <AButton v-if="form.tiers.length > 1" size="mini" status="danger" @click="form.tiers.splice(index, 1)">删除</AButton>
              </div>
              <!-- 价格字段按 6 列栅格排布（每项占 2 列）：第一行输入/输出，第二行缓存三项，与平台定价弹窗排布一致；
                   留空/回退语义统一走区块脚注，不再逐字段挂 extra 长文案 -->
              <div class="price-grid">
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
              <div class="price-grid">
                <AFormItem label="缓存读取">
                  <AInputNumber v-model="tier.cache_read_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="缓存创建">
                  <AInputNumber v-model="tier.cache_creation_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
                <AFormItem label="长时缓存创建">
                  <AInputNumber v-model="tier.cache_creation_1h_price" :min="0" :precision="4" class="w-full">
                    <template #suffix>{{ currencySymbol }}/1M</template>
                  </AInputNumber>
                </AFormItem>
              </div>
            </div>
            <AButton type="dashed" long @click="addTenantTier" class="mt-2">+ 添加梯度</AButton>
            <p class="section-footnote">
              缓存三项填 0 时回退第 1 梯锚点价；「缓存创建」为短 TTL 桶（Claude 5m / GPT 30m），「长时缓存创建」为长 TTL 桶（Claude 1h / GPT 12h），后者为 0 按第 1 梯基准价 ×1.6 推导，建议显式填写。
            </p>
          </template>
          </template><!-- /专属定价模式 -->
        </div>

        <!-- 按秒矩阵覆盖：仅平台 per_second/special 模型 + 专属定价模式（覆盖后为一口价，折扣不叠加） -->
        <div v-if="platformIsPerSecond && pricingMode === 'custom'" class="editor-section">
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

/*
 * 价格输入网格（与平台定价弹窗 ModelPricingModal 同构）：6 列等分，每个字段占 2 列（一行 3 个）。
 * 第一行输入/输出两项占 4 列，第二行缓存三项占满——跨行、跨档位严格等宽对齐。
 */
.price-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  column-gap: 12px;
}

.price-grid > * {
  grid-column: span 2;
  min-width: 0;
}

/* 网格内表单项收紧下间距：默认 16px 会把区块下半截撑得过空 */
.price-grid > :deep(.arco-form-item) {
  margin-bottom: 10px;
}

/* 窄屏（手机端管理后台）回落 2 列，每字段独占 1 列，避免横向溢出 */
@media (max-width: 767px) {
  .price-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .price-grid > * {
    grid-column: span 1;
  }
}

/* 区块级脚注：留空/回退语义收拢成一行，替代逐字段 extra 长文案 */
.section-footnote {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--ta-text-tertiary);
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
  /* 底部内边距收窄：网格内表单项自带 10px 下间距，避免卡片下缘过空（与平台定价弹窗一致） */
  padding: 12px 16px 4px;
  background: var(--color-fill-1);
  border-radius: 8px;
  margin-bottom: 8px;
}

.tier-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

/* 档位标题行内联 Token 区间：序号 + 起止输入，末档无上限为只读文案 */
.tier-range {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.tier-range-input {
  width: 140px;
}

.tier-range-sep,
.tier-range-unit {
  font-size: 12px;
  color: var(--ta-text-tertiary);
}

.tier-range-unlimited {
  font-size: 13px;
  color: var(--ta-text-tertiary);
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

/* 折扣模式：快捷档按钮行 + 实时生效价预览 */
.discount-preset-row {
  display: flex;
  gap: 8px;
  margin: 8px 0;
}
.discount-preview {
  margin: 8px 0 4px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--color-fill-1);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.discount-preview-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--color-text-2);
}
.discount-preview-row b {
  color: rgb(var(--primary-6));
}
.discount-preview-label {
  color: var(--color-text-3);
}
</style>
