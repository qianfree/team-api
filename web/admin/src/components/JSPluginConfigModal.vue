<script setup lang="ts">
// JS 任务插件配置弹窗：manifest.config[] 驱动渲染（分组小节 + 双列网格 + 加密字段
// 密码输入 + 掩码保持语义）。与 Go 插件的 PluginConfigModal 同形态；差异仅在
// API 路径与取值方式——JS 插件的 config-schema 响应已内联当前值（加密掩码），
// 单请求即可，无需再查详情。
import { computed, ref, watch } from 'vue'
import { Button, Input, Message } from '@arco-design/web-vue'
import { IconSearch } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'

const props = defineProps<{
  visible: boolean
  pluginKey: string
  title?: string
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'saved'): void
}>()

type Field = {
  key: string
  label: string
  type: string
  default?: any
  options?: string[]
  required?: boolean
  description?: string
  encrypted?: boolean
  group?: string
  half_width?: boolean
  value?: any
}

const loading = ref(false)
const submitting = ref(false)
const schema = ref<Field[]>([])
const form = ref<Record<string, any>>({})
const keyword = ref('')

// 加密字段的初始值（掩码或空），用于判断「值被用户改过没有」
const encryptedInitial = ref<Record<string, string>>({})

watch(
  () => [props.visible, props.pluginKey] as const,
  ([visible]) => {
    if (visible && props.pluginKey) load()
  },
)

async function load() {
  loading.value = true
  form.value = {}
  schema.value = []
  keyword.value = ''
  try {
    const res: any = await request.get(`/admin/js-plugins/${props.pluginKey}/config-schema`)
    const list: Field[] = res.data?.data?.list || []
    schema.value = list
    const next: Record<string, any> = {}
    const initial: Record<string, string> = {}
    for (const field of list) {
      // 值已内联（加密字段为掩码/空串）；掩码提交=不改动现值
      const v = field.value ?? field.default ?? ''
      next[field.key] = v
      if (field.encrypted) initial[field.key] = String(v ?? '')
    }
    form.value = next
    encryptedInitial.value = initial
  } catch {
    emit('update:visible', false)
  } finally {
    loading.value = false
  }
}

const filteredSchema = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return schema.value
  return schema.value.filter((f) =>
    [f.key, f.label, f.group, f.description].join(' ').toLowerCase().includes(kw),
  )
})

const groupedSchema = computed(() => {
  const groups: { name: string; fields: Field[] }[] = []
  const index = new Map<string, { name: string; fields: Field[] }>()
  for (const field of filteredSchema.value) {
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

const fieldCount = computed(() => schema.value.length)
const encryptedCount = computed(() => schema.value.filter((f) => f.encrypted).length)

function isWideField(field: Field): boolean {
  if (field.half_width) return false
  return field.type === 'string' || field.type === 'json' || field.type === 'text'
}

// 加密字段状态：已配置（掩码）/ 本次已改写 / 尚未配置
function encryptedState(field: Field): 'set' | 'changed' | 'empty' {
  const current = String(form.value[field.key] ?? '')
  if (current && current !== encryptedInitial.value[field.key]) return 'changed'
  if (encryptedInitial.value[field.key]) return 'set'
  return 'empty'
}

function validate(): string | null {
  for (const field of schema.value) {
    const v = form.value[field.key]
    if (field.required && (v === '' || v === null || v === undefined)) {
      return `「${field.label}」为必填项`
    }
    if (field.type === 'json' && typeof v === 'string' && v.trim()) {
      try {
        JSON.parse(v)
      } catch (e: any) {
        return `「${field.label}」不是合法 JSON：${e?.message || ''}`
      }
    }
  }
  return null
}

async function submit(): Promise<boolean> {
  const err = validate()
  if (err) {
    Message.error(err)
    return false
  }
  submitting.value = true
  try {
    await request.put(`/admin/js-plugins/${props.pluginKey}/config`, { config: form.value })
    Message.success('配置已保存（服务中的插件即刻生效）')
    emit('update:visible', false)
    emit('saved')
    return true
  } catch {
    // 拦截器已提示
    return false
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AModal
    :visible="visible"
    :title="title || `配置 - ${pluginKey}`"
    :mask-closable="false"
    :width="720"
    :ok-loading="submitting"
    :on-before-ok="submit"
    @update:visible="emit('update:visible', $event)"
    @cancel="emit('update:visible', false)"
  >
    <div v-if="fieldCount" class="cfg-head">
      <div class="cfg-head__text">
        共 <strong>{{ fieldCount }}</strong> 项
        <template v-if="encryptedCount">，其中 <strong>{{ encryptedCount }}</strong> 项加密存储</template>
        。服务中的插件保存后即刻生效。
      </div>
      <Input
        v-if="fieldCount > 8"
        v-model="keyword"
        class="cfg-search"
        size="small"
        allow-clear
        placeholder="搜索配置项"
      >
        <template #prefix><IconSearch /></template>
      </Input>
    </div>

    <ASpin :loading="loading" style="width: 100%">
      <AForm v-if="groupedSchema.length" :model="form" layout="vertical">
        <template v-for="group in groupedSchema" :key="group.name || '__default__'">
          <div v-if="group.name" class="config-group-title">
            {{ group.name }}
            <span class="config-group-count">{{ group.fields.length }}</span>
          </div>
          <div class="config-grid">
            <AFormItem
              v-for="field in group.fields"
              :key="field.key"
              :class="isWideField(field) ? 'config-wide' : ''"
              :label="field.label"
              :required="field.required"
              :extra="field.description"
            >
              <AInput
                v-if="field.type === 'string' && !field.encrypted"
                v-model="form[field.key]"
                allow-clear
              />
              <AInputPassword
                v-else-if="field.type === 'string' && field.encrypted"
                v-model="form[field.key]"
                allow-clear
                :placeholder="encryptedState(field) === 'set' ? '已配置，留空或保持掩码即不改动' : '尚未配置'"
              />
              <AInputNumber
                v-else-if="field.type === 'int'"
                v-model="form[field.key]"
                style="width: 100%"
              />
              <ASwitch
                v-else-if="field.type === 'bool'"
                v-model="form[field.key]"
              />
              <ASelect
                v-else-if="field.type === 'select'"
                v-model="form[field.key]"
                :options="field.options?.map((o: string) => ({ label: o, value: o }))"
              />
              <ATextarea
                v-else-if="field.type === 'json'"
                v-model="form[field.key]"
                class="cfg-json"
                :auto-size="{ minRows: 3, maxRows: 8 }"
                placeholder="{ }"
              />
              <ATextarea
                v-else-if="field.type === 'text'"
                v-model="form[field.key]"
                :auto-size="{ minRows: 6, maxRows: 16 }"
                allow-clear
              />
              <AInput v-else v-model="form[field.key]" allow-clear />

              <div v-if="field.encrypted" class="config-encrypted-hint">
                <span class="config-dot" :class="`is-${encryptedState(field)}`" />
                <template v-if="encryptedState(field) === 'changed'">本次将覆盖已存值</template>
                <template v-else-if="encryptedState(field) === 'set'">已配置（回显为掩码，提交掩码不改动）</template>
                <template v-else>尚未配置</template>
              </div>
            </AFormItem>
          </div>
        </template>
      </AForm>
      <AEmpty v-else :description="fieldCount ? '没有匹配的配置项' : '该插件没有可配置项'" />
    </ASpin>

    <div v-if="fieldCount" class="cfg-foot">
      <Button v-if="keyword" size="mini" type="text" @click="keyword = ''">清空搜索</Button>
      <span>未填写的可选配置将使用 manifest 中的默认值。</span>
    </div>
  </AModal>
</template>

<style scoped>
.cfg-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  padding: 9px 12px;
  border-radius: 8px;
  background: var(--color-fill-1);
  font-size: 12.5px;
  color: var(--color-text-3);
}

.cfg-head__text strong {
  color: var(--color-text-1);
  font-weight: 600;
}

.cfg-search {
  width: 180px;
  flex-shrink: 0;
}

.config-group-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-2);
  margin: 4px 0 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--color-border-2);
}

.config-group-title:not(:first-child) {
  margin-top: 22px;
}

.config-group-count {
  padding: 0 6px;
  border-radius: 8px;
  background: var(--color-fill-2);
  font-size: 11px;
  font-weight: 400;
  line-height: 16px;
  color: var(--color-text-3);
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 16px;
}

.config-grid .config-wide {
  grid-column: span 2;
}

.cfg-json :deep(textarea) {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
}

.config-encrypted-hint {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--color-text-3);
  margin-top: 2px;
  line-height: 1.5;
}

.config-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--color-text-3);
}

.config-dot.is-set {
  background: var(--ta-success);
}

.config-dot.is-changed {
  background: var(--ta-warning);
}

.config-dot.is-empty {
  background: var(--color-border-2);
}

.cfg-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--color-text-3);
}

@media (max-width: 640px) {
  .config-grid {
    grid-template-columns: 1fr;
  }
  .config-grid .config-wide {
    grid-column: span 1;
  }
  .cfg-search {
    width: 120px;
  }
}
</style>
