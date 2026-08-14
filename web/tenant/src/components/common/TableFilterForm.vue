<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { NForm, NFormItem, NInput, NSelect, NDatePicker, NInputNumber, NButton, NDrawer, NDrawerContent, NDropdown } from 'naive-ui'
import Icon from './Icon.vue'
import MobileRangeFilter from './MobileRangeFilter.vue'

// 表单字段配置类型
export interface FilterField {
  type: 'input' | 'select' | 'date' | 'daterange' | 'datetimerange' | 'number' | 'custom'
  key: string
  label: string
  placeholder?: string
  options?: Array<{ label: string; value: string | number }>
  min?: number
  max?: number
  step?: number
  width?: string
  clearable?: boolean
  // 日期时间范围快捷选项
  shortcuts?: Record<string, () => [number, number]>
}

interface Props {
  fields: FilterField[]
  loading?: boolean
  showExport?: boolean
  exporting?: boolean
  exportFormats?: Array<'csv' | 'xlsx'>
  // 移动端适配
  mobileBreakpoint?: number  // 移动端断点（px），默认 768
  alwaysDrawer?: boolean     // 始终使用抽屉模式
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  showExport: false,
  exporting: false,
  exportFormats: () => ['csv', 'xlsx'],
  mobileBreakpoint: 768,
  alwaysDrawer: false,
})

const emit = defineEmits<{
  search: []
  reset: []
  export: [format: 'csv' | 'xlsx']
}>()

// 表单数据（由父组件通过 v-model 绑定）
const modelValue = defineModel<Record<string, any>>({ required: true })

const showFilterDrawer = ref(false)

// 导出格式下拉选项（NDropdown 默认 teleport 到 body，避免被 .card 的 backdrop-filter
// stacking context 困住而被下方表格面板遮挡）
const exportDropdownOptions = computed(() =>
  props.exportFormats.map(format => ({
    label: format === 'csv' ? '导出 CSV' : '导出 Excel',
    key: format,
  }))
)

// 检测是否为移动端
const isMobile = ref(false)
const checkMobile = () => {
  isMobile.value = window.innerWidth < props.mobileBreakpoint
}

// 初始化和监听窗口大小变化
if (typeof window !== 'undefined') {
  checkMobile()
  window.addEventListener('resize', checkMobile)
}

// 是否使用抽屉模式
const useDrawer = computed(() => props.alwaysDrawer || isMobile.value)

// 统计非空筛选条件数量
const activeFilterCount = computed(() => {
  let count = 0
  for (const key in modelValue.value) {
    const value = modelValue.value[key]
    if (value !== '' && value !== null && value !== undefined) {
      // 数组类型（如日期范围）
      if (Array.isArray(value) && value.length > 0) {
        count++
      }
      // 其他非空值
      else if (value !== '') {
        count++
      }
    }
  }
  return count
})

function handleSearch() {
  if (useDrawer.value) {
    showFilterDrawer.value = false
  }
  emit('search')
}

function handleReset() {
  // 重置所有字段
  Object.keys(modelValue.value).forEach(key => {
    modelValue.value[key] = ''
  })
  if (useDrawer.value) {
    showFilterDrawer.value = false
  }
  emit('reset')
}

function handleExport(format: string | number) {
  emit('export', format as 'csv' | 'xlsx')
}

// 日期时间范围默认快捷选项
function dayStart(offsetDays: number): number {
  const d = new Date()
  d.setDate(d.getDate() + offsetDays)
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}
function dayEnd(offsetDays: number): number {
  const d = new Date()
  d.setDate(d.getDate() + offsetDays)
  d.setHours(23, 59, 59, 999)
  return d.getTime()
}

const defaultShortcuts = {
  今日: () => [dayStart(0), dayEnd(0)] as [number, number],
  昨天: () => [dayStart(-1), dayEnd(-1)] as [number, number],
  近3天: () => [dayStart(-2), dayEnd(0)] as [number, number],
  近一周: () => [dayStart(-6), dayEnd(0)] as [number, number],
}

// 渲染单个字段
const renderField = (field: FilterField) => {
  const width = useDrawer.value ? '100%' : (field.width || '160px')

  switch (field.type) {
    case 'input':
      return h(NInput, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: string) => { modelValue.value[field.key] = val },
        placeholder: field.placeholder || '请输入',
        style: { width },
        clearable: field.clearable !== false,
        onKeyup: (e: KeyboardEvent) => e.key === 'Enter' && handleSearch(),
      })

    case 'select':
      return h(NSelect, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: any) => { modelValue.value[field.key] = val },
        options: field.options || [],
        placeholder: field.placeholder || '请选择',
        style: { width: useDrawer.value ? '100%' : (field.width || '120px') },
        clearable: field.clearable !== false,
      })

    case 'date':
      return h(NDatePicker, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: number | null) => { modelValue.value[field.key] = val },
        type: 'date',
        placeholder: field.placeholder || '选择日期',
        style: { width },
        clearable: field.clearable !== false,
      })

    case 'daterange':
      // 移动端抽屉：用快捷范围 + 原生输入替代日历面板（面板固定宽度，移动端会溢出屏幕）
      if (useDrawer.value) {
        return h(MobileRangeFilter, {
          modelValue: modelValue.value[field.key],
          'onUpdate:modelValue': (val: [number, number] | null) => { modelValue.value[field.key] = val },
          type: 'daterange',
          shortcuts: field.shortcuts || defaultShortcuts,
        })
      }
      return h(NDatePicker, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: [number, number] | null) => { modelValue.value[field.key] = val },
        type: 'daterange',
        placeholder: field.placeholder || '选择日期范围',
        style: { width: field.width || '280px' },
        clearable: field.clearable !== false,
      })

    case 'datetimerange':
      // 移动端抽屉：用快捷范围 + 原生输入替代日历面板（面板固定宽度，移动端会溢出屏幕）
      if (useDrawer.value) {
        return h(MobileRangeFilter, {
          modelValue: modelValue.value[field.key],
          'onUpdate:modelValue': (val: [number, number] | null) => { modelValue.value[field.key] = val },
          type: 'datetimerange',
          shortcuts: field.shortcuts || defaultShortcuts,
        })
      }
      return h(NDatePicker, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: [number, number] | null) => { modelValue.value[field.key] = val },
        type: 'datetimerange',
        shortcuts: field.shortcuts || defaultShortcuts,
        placeholder: field.placeholder || '选择时间范围',
        style: { width: field.width || '400px' },
        clearable: field.clearable !== false,
        actions: ['clear', 'confirm'],
      })

    case 'number':
      return h(NInputNumber, {
        value: modelValue.value[field.key],
        'onUpdate:value': (val: number | null) => { modelValue.value[field.key] = val },
        placeholder: field.placeholder || '请输入',
        style: { width: useDrawer.value ? '100%' : (field.width || '120px') },
        min: field.min,
        max: field.max,
        step: field.step || 1,
        clearable: field.clearable !== false,
        onKeyup: (e: KeyboardEvent) => e.key === 'Enter' && handleSearch(),
      })

    default:
      return null
  }
}

</script>

<template>
  <!-- 移动端：紧凑的触发按钮 + 抽屉 -->
  <div v-if="useDrawer" class="card">
    <div class="card-body !p-3">
      <div class="flex items-center justify-between gap-2">
        <!-- 筛选按钮 -->
        <n-button type="primary" @click="showFilterDrawer = true">
          <template #icon>
            <Icon name="search" size="sm" />
          </template>
          筛选
          <span v-if="activeFilterCount > 0" class="ml-1 inline-flex h-5 w-5 items-center justify-center rounded-full bg-white/20 text-xs font-semibold">
            {{ activeFilterCount }}
          </span>
        </n-button>

        <div class="flex items-center gap-2">
          <!-- 导出按钮 -->
          <n-dropdown
            v-if="showExport"
            trigger="click"
            :options="exportDropdownOptions"
            @select="handleExport"
          >
            <n-button :loading="exporting" :disabled="exporting">
              <template #icon>
                <Icon name="download" size="sm" />
              </template>
              导出
            </n-button>
          </n-dropdown>

          <!-- 自定义额外按钮插槽 -->
          <slot name="extra-actions" />
        </div>
      </div>
    </div>

    <!-- 抽屉筛选面板 -->
    <n-drawer v-model:show="showFilterDrawer" :width="'85%'" :max-width="400" placement="right">
      <n-drawer-content title="筛选条件" closable>
        <n-form label-placement="top" :show-feedback="false">
          <div class="space-y-4">
            <!-- 动态渲染查询字段 -->
            <n-form-item
              v-for="field in fields"
              :key="field.key"
              :label="field.label"
              class="!mb-0"
            >
              <!-- 使用 JSX 渲染函数 -->
              <component :is="() => renderField(field)" />

              <!-- 自定义插槽 -->
              <slot v-if="field.type === 'custom'" :name="`field-${field.key}`" :field="field" />
            </n-form-item>
          </div>
        </n-form>

        <template #footer>
          <div class="flex gap-2">
            <n-button type="primary" block :loading="loading" @click="handleSearch">
              <template #icon>
                <Icon name="search" size="sm" />
              </template>
              搜索
            </n-button>
            <n-button block :disabled="loading" @click="handleReset">
              重置
            </n-button>
          </div>
        </template>
      </n-drawer-content>
    </n-drawer>
  </div>

  <!-- 桌面端：横向展开的表单 -->
  <div v-else class="card">
    <div class="card-body !p-4">
      <n-form label-placement="left" :show-feedback="false">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-3">
          <!-- 动态渲染查询字段 -->
          <n-form-item
            v-for="field in fields"
            :key="field.key"
            :label="field.label"
            label-style="color: rgb(107, 114, 128); font-size: 0.875rem; white-space: nowrap;"
            class="!mb-0"
          >
            <!-- 使用 JSX 渲染函数 -->
            <component :is="() => renderField(field)" />

            <!-- 自定义插槽 -->
            <slot v-if="field.type === 'custom'" :name="`field-${field.key}`" :field="field" />
          </n-form-item>

          <!-- 操作按钮 -->
          <div class="ml-auto flex items-center gap-2">
            <n-button type="primary" :loading="loading" @click="handleSearch">
              <template #icon>
                <Icon name="search" size="sm" />
              </template>
              搜索
            </n-button>
            <n-button :disabled="loading" @click="handleReset">
              重置
            </n-button>

            <!-- 导出按钮 -->
            <template v-if="showExport">
              <span class="mx-1 h-6 w-px bg-gray-200" aria-hidden="true"></span>
              <n-dropdown
                trigger="click"
                :options="exportDropdownOptions"
                @select="handleExport"
              >
                <n-button :loading="exporting" :disabled="exporting">
                  <template #icon>
                    <Icon name="download" size="sm" />
                  </template>
                  导出
                  <template #icon-right>
                    <Icon name="chevronDown" size="xs" />
                  </template>
                </n-button>
              </n-dropdown>
            </template>

            <!-- 自定义额外按钮插槽 -->
            <slot name="extra-actions" />
          </div>
        </div>
      </n-form>
    </div>
  </div>
</template>

<style scoped>
/* Naive UI 表单项标签样式覆盖 */
:deep(.n-form-item-label) {
  padding-right: 8px;
}

:deep(.n-form-item) {
  --n-label-font-size: 0.875rem;
}
</style>
