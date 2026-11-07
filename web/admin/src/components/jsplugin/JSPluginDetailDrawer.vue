<script setup lang="ts">
// JS 插件详情抽屉：摘要 + 快捷操作 + 基本信息 + 计费单位 + 路由 + 版本历史。
// 数据以 props.plugin 为单一来源（父页面列表对象），激活/启停后 emit('changed')
// 由父页面刷新列表，prop 更新后自动重算，无需抽屉内维护副本。
import { computed, h, ref, watch } from 'vue'
import {
  Button, Drawer, Message, Popconfirm, Space, Table, Tag,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { IconCopy, IconLink } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import JSPluginSourceModal from './JSPluginSourceModal.vue'
import {
  pluginStatus, resolvePluginIcon, fetchModeLabel,
  type JSPlugin, type JSPluginRoute, type JSPluginVersion,
} from './pluginMeta'

const props = defineProps<{
  visible: boolean
  plugin: JSPlugin | null
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'toggle', enabled: boolean): void
  (e: 'config'): void
  (e: 'changed'): void
}>()

const routes = ref<JSPluginRoute[]>([])
const routesLoading = ref(false)
const busyVersion = ref('')

// 源码查看弹窗状态：sourceVersion 记录待查看版本号
const sourceVisible = ref(false)
const sourceVersion = ref('')

function viewSource(version: string) {
  sourceVersion.value = version
  sourceVisible.value = true
}

const status = computed(() => (props.plugin ? pluginStatus(props.plugin) : null))
const icon = computed(() => (props.plugin ? resolvePluginIcon(props.plugin) : null))
const versions = computed<JSPluginVersion[]>(() => props.plugin?.versions || [])

const iconUrl = computed(
  () => `/api/admin/js-plugins/${props.plugin?.key || ''}/icon?v=${props.plugin?.active_version || ''}`,
)

const basicItems = computed(() => {
  const p = props.plugin
  if (!p) return []
  return [
    { label: '插件标识', value: p.key, mono: true },
    { label: '激活版本', value: p.active_version || '—', mono: true },
    { label: '轮询模式', value: fetchModeLabel(p.fetch_mode) },
    { label: '版本数量', value: `${versions.value.length} 个` },
    { label: '作者', value: p.author || '—' },
    { label: '状态', value: status.value?.label || '—' },
  ]
})

async function loadRoutes(key: string) {
  routes.value = []
  routesLoading.value = true
  try {
    const res: any = await request.get(`/admin/js-plugins/${key}/routes`)
    routes.value = res.data?.data?.list || res.data?.list || []
  } catch {
    routes.value = []
  } finally {
    routesLoading.value = false
  }
}

watch(
  () => [props.visible, props.plugin?.key, props.plugin?.active_version] as const,
  ([visible, key]) => {
    if (visible && key) loadRoutes(key)
    else routes.value = []
  },
  { immediate: true },
)

function close() {
  emit('update:visible', false)
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Message.success('已复制')
  } catch {
    Message.warning('复制失败，请手动选中')
  }
}

async function activate(version: string) {
  if (!props.plugin) return
  busyVersion.value = version
  try {
    await request.post(`/admin/js-plugins/${props.plugin.key}/versions/${version}/activate`)
    Message.success(`已激活 ${version}`)
    emit('changed')
  } catch {
    // 拦截器已提示
  } finally {
    busyVersion.value = ''
  }
}

async function removeVersion(version: string) {
  if (!props.plugin) return
  busyVersion.value = version
  try {
    await request.delete(`/admin/js-plugins/${props.plugin.key}/versions/${version}`)
    Message.success('已删除该版本')
    emit('changed')
  } catch {
    // 拦截器已提示
  } finally {
    busyVersion.value = ''
  }
}

const versionColumns: TableColumnData[] = [
  {
    title: '版本',
    dataIndex: 'version',
    width: 110,
    render: ({ record }: any) =>
      h('span', { style: 'font-family: JetBrains Mono, Consolas, monospace; font-weight: 500;' }, record.version),
  },
  {
    title: '来源',
    dataIndex: 'origin',
    width: 80,
    render: ({ record }: any) =>
      h(Tag, { size: 'small', color: record.origin === 'builtin' ? 'purple' : 'gray' },
        () => (record.origin === 'builtin' ? '内置' : '上传')),
  },
  {
    title: '状态',
    dataIndex: 'active',
    width: 130,
    render: ({ record }: any) => {
      const tags: any[] = []
      if (record.active) tags.push(h(Tag, { size: 'small', color: 'green' }, () => '激活中'))
      if (record.enabled) tags.push(h(Tag, { size: 'small', color: 'arcoblue' }, () => '已启用'))
      if (!tags.length) tags.push(h(Tag, { size: 'small' }, () => '未使用'))
      return h('span', { style: 'display: inline-flex; gap: 4px;' }, tags)
    },
  },
  { title: '上传时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 200,
    align: 'right',
    render: ({ record }: any) => {
      const btns: any[] = []
      btns.push(
        h(Button, {
          size: 'mini', type: 'text', onClick: () => viewSource(record.version),
        }, () => '源码'),
      )
      if (!record.active) {
        btns.push(
          h(Popconfirm, {
            content: `激活 ${record.version}？当前激活版本将被替换。`,
            onOk: () => activate(record.version),
          }, () => h(Button, { size: 'mini', type: 'primary', loading: busyVersion.value === record.version }, () => '激活')),
        )
        if (record.origin !== 'builtin') {
          btns.push(
            h(Popconfirm, {
              content: `删除版本 ${record.version}？源码不可恢复。`,
              onOk: () => removeVersion(record.version),
            }, () => h(Button, {
              size: 'mini', status: 'danger', type: 'text', loading: busyVersion.value === record.version,
            }, () => '删除')),
          )
        }
      }
      return h(Space, { size: 4 }, () => btns)
    },
  },
]
</script>

<template>
  <Drawer
    :visible="visible"
    :width="720"
    :footer="false"
    unmount-on-close
    @cancel="close"
  >
    <template #title>
      <div v-if="plugin" class="jd-head">
        <div class="jd-icon" :class="[`jd-icon--${icon?.kind}`, `jd-icon--${plugin.origin}`]">
          <img v-if="icon?.kind === 'img'" :src="iconUrl" :alt="plugin.name" />
          <component :is="icon?.comp" v-else-if="icon?.kind === 'icon'" :size="24" />
          <span v-else>{{ icon?.label }}</span>
        </div>
        <div class="jd-ident">
          <div class="jd-name-row">
            <span class="jd-name">{{ plugin.name || plugin.key }}</span>
            <Tag size="small" :color="plugin.origin === 'builtin' ? 'purple' : 'gray'">
              {{ plugin.origin === 'builtin' ? '内置' : '上传' }}
            </Tag>
          </div>
          <code class="jd-key">{{ plugin.key }}</code>
        </div>
        <span v-if="status" class="jd-status" :class="`is-${status.key}`" :title="status.hint">
          <i />{{ status.label }}
        </span>
      </div>
    </template>

    <div v-if="plugin" class="jd-body">
      <p class="jd-desc">{{ plugin.description || '该插件未填写描述。' }}</p>

      <div class="jd-actions">
        <Popconfirm
          v-if="plugin.enabled"
          content="禁用后路由会被立即摘除，确定？"
          @ok="emit('toggle', false)"
        >
          <Button type="outline" status="warning">禁用插件</Button>
        </Popconfirm>
        <Button v-else type="primary" @click="emit('toggle', true)">启用插件</Button>
        <Button
          v-if="plugin.active_version"
          type="secondary"
          @click="viewSource(plugin.active_version)"
        >查看源码</Button>
        <Button type="secondary" @click="emit('config')">配置</Button>
        <Button v-if="plugin.website" type="text" :href="plugin.website" target="_blank" rel="noopener">
          <template #icon><IconLink /></template>官网
        </Button>
        <span class="jd-actions__hint">{{ status?.hint }}</span>
      </div>

      <section class="jd-section">
        <h4 class="jd-title">基本信息</h4>
        <div class="jd-grid">
          <div v-for="item in basicItems" :key="item.label" class="jd-cell">
            <span class="jd-cell__label">{{ item.label }}</span>
            <span class="jd-cell__value" :class="{ 'is-mono': item.mono }">{{ item.value }}</span>
          </div>
        </div>
      </section>

      <section class="jd-section">
        <h4 class="jd-title">适用模型</h4>
        <div v-if="(plugin.models || []).length" class="jd-chips">
          <Tag v-for="m in plugin.models" :key="m" size="small" color="arcoblue">{{ m }}</Tag>
        </div>
        <p v-else class="jd-empty">未声明适用模型，任何模型都可触发该插件。</p>
      </section>

      <section v-if="(plugin.allowed_hosts || []).length" class="jd-section">
        <h4 class="jd-title">额外出网白名单</h4>
        <div class="jd-chips">
          <code v-for="host in plugin.allowed_hosts" :key="host" class="jd-host">{{ host }}</code>
        </div>
      </section>

      <section v-if="(plugin.usage_schema || []).length" class="jd-section">
        <h4 class="jd-title">
          计费单位
          <span class="jd-title__hint">定价公式中以 u("key") 引用</span>
        </h4>
        <Table
          :columns="[
            { title: 'key', dataIndex: 'key', width: 140 },
            { title: '类型', dataIndex: 'type', width: 90 },
            { title: '单位', dataIndex: 'unit', width: 100 },
            { title: '说明', dataIndex: 'description', ellipsis: true, tooltip: true },
          ]"
          :data="plugin.usage_schema"
          :pagination="false"
          size="small"
          row-key="key"
        />
      </section>

      <section class="jd-section">
        <h4 class="jd-title">调用路由</h4>
        <div v-if="routesLoading" class="jd-empty">加载中…</div>
        <div v-else-if="!routes.length" class="jd-empty">该插件未声明路由。</div>
        <ul v-else class="jd-routes">
          <li v-for="r in routes" :key="`${r.method}${r.path}`" class="jd-route">
            <span class="jd-route__type" :class="`is-${r.type}`">{{ r.type }}</span>
            <span class="jd-route__method">{{ r.method }}</span>
            <code class="jd-route__path">{{ r.path }}</code>
            <Button size="mini" type="text" class="jd-route__copy" @click="copy(r.path)">
              <template #icon><IconCopy /></template>
            </Button>
          </li>
        </ul>
        <p class="jd-note">
          <template v-if="plugin.in_service">客户端按上述地址调用，鉴权方式与 /v1 一致（Bearer API Key）。</template>
          <template v-else>路由声明取自插件源码 manifest；插件当前未启用，<code>/pjs/{{ plugin.key }}/</code> 下暂未挂载，启用后即按上述地址生效。</template>
        </p>
      </section>

      <section class="jd-section">
        <h4 class="jd-title">
          版本历史
          <span class="jd-title__hint">同 key + version 源码一致才允许重复上传</span>
        </h4>
        <Table
          :columns="versionColumns"
          :data="versions"
          :pagination="false"
          size="small"
          row-key="id"
          :scroll="{ x: 680 }"
        />
      </section>
    </div>

    <JSPluginSourceModal
      :visible="sourceVisible"
      :plugin-key="plugin?.key || ''"
      :version="sourceVersion"
      @update:visible="(v: boolean) => (sourceVisible = v)"
    />
  </Drawer>
</template>

<style scoped>
.jd-head {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 2px 8px 2px 0;
}

.jd-icon {
  width: 46px;
  height: 46px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: #fff;
  font-size: 19px;
  font-weight: 700;
  overflow: hidden;
}

.jd-icon--icon,
.jd-icon--text {
  background: linear-gradient(135deg, #0d9488, #2dd4bf);
}

.jd-icon--icon.jd-icon--builtin,
.jd-icon--text.jd-icon--builtin {
  background: linear-gradient(135deg, #7c3aed, #a78bfa);
}

.jd-icon--img {
  background: var(--color-fill-2);
  border: 1px solid var(--color-border-2);
}

.jd-icon--img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  padding: 4px;
}

.jd-ident {
  flex: 1;
  min-width: 0;
}

.jd-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.jd-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-key {
  display: block;
  margin-top: 2px;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  font-weight: 400;
  color: var(--color-text-3);
}

.jd-status {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jd-status i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.jd-status.is-live {
  color: #059669;
  background: rgba(16, 185, 129, 0.12);
}

.jd-status.is-pending {
  color: #b45309;
  background: rgba(245, 158, 11, 0.14);
}

.jd-body {
  display: flex;
  flex-direction: column;
}

.jd-desc {
  margin: 0 0 14px;
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--color-text-2);
}

.jd-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px;
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
}

.jd-actions__hint {
  flex: 1;
  min-width: 160px;
  text-align: right;
  font-size: 12px;
  color: var(--color-text-3);
}

.jd-section {
  margin-top: 22px;
}

.jd-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 0 0 10px;
  padding-bottom: 7px;
  border-bottom: 1px solid var(--color-border-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jd-title__hint {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-text-3);
}

.jd-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 16px;
  padding: 14px 16px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-fill-2);
}

.jd-cell {
  min-width: 0;
}

.jd-cell__label {
  display: block;
  font-size: 12px;
  color: var(--color-text-3);
}

.jd-cell__value {
  display: block;
  margin-top: 2px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-cell__value.is-mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12.5px;
}

.jd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.jd-host {
  padding: 2px 8px;
  border-radius: 5px;
  background: var(--color-fill-2);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--color-text-2);
}

.jd-empty {
  margin: 0;
  padding: 12px 14px;
  border: 1px dashed var(--color-border-2);
  border-radius: var(--ta-radius);
  font-size: 12.5px;
  color: var(--color-text-3);
}

.jd-routes {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.jd-route {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--color-border-2);
  border-radius: 8px;
  background: var(--color-bg-1);
}

.jd-route__type {
  flex-shrink: 0;
  padding: 1px 7px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-primary-6);
  background: var(--color-primary-1);
}

.jd-route__type.is-poll {
  color: #0369a1;
  background: rgba(6, 182, 212, 0.12);
}

.jd-route__method {
  flex-shrink: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-3);
}

.jd-route__path {
  flex: 1;
  min-width: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-route__copy {
  flex-shrink: 0;
}

.jd-note {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1.6;
}

@media (max-width: 640px) {
  .jd-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .jd-actions__hint {
    display: none;
  }
}
</style>
