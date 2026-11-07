<script setup lang="ts">
// JS 插件源码查看弹窗：按需拉取指定版本 plugin.js 源文，带行号只读展示。
// 只读查看（复制/下载），不引入代码编辑器依赖；源码上限 2 MiB，单弹窗渲染无压力。
import { computed, ref, watch } from 'vue'
import { Message, Modal, Spin, Tag } from '@arco-design/web-vue'
import { IconCopy, IconDownload } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'

const props = defineProps<{
  visible: boolean
  pluginKey: string
  version: string
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
}>()

interface SourceData {
  key: string
  version: string
  origin: string
  source: string
  source_hash: string
  remark: string
  created_at: string
}

const data = ref<SourceData | null>(null)
const loading = ref(false)
const loadError = ref('')

const lines = computed(() => {
  if (!data.value) return [] as string[]
  const raw = data.value.source.split('\n')
  // 末尾多余空行不占行号（与编辑器行号习惯一致）
  if (raw.length > 1 && raw[raw.length - 1] === '') raw.pop()
  return raw
})

const prettySize = computed(() => {
  const n = data.value?.source.length || 0
  return n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KiB`
})

const hashShort = computed(() => (data.value?.source_hash || '').slice(0, 12))

watch(
  () => [props.visible, props.pluginKey, props.version] as const,
  ([visible, key, version]) => {
    if (visible && key && version) load(key, version)
    else {
      data.value = null
      loadError.value = ''
    }
  },
  { immediate: true },
)

async function load(key: string, version: string) {
  loading.value = true
  loadError.value = ''
  data.value = null
  try {
    const res: any = await request.get(`/admin/js-plugins/${key}/versions/${version}/source`)
    data.value = res.data?.data || res.data
  } catch (err: any) {
    loadError.value = err?.message || '加载源码失败'
  } finally {
    loading.value = false
  }
}

function close() {
  emit('update:visible', false)
}

async function copyAll() {
  if (!data.value) return
  try {
    await navigator.clipboard.writeText(data.value.source)
    Message.success('已复制全部源码')
  } catch {
    Message.warning('复制失败，请手动选中')
  }
}

function download() {
  if (!data.value) return
  const blob = new Blob([data.value.source], { type: 'text/javascript;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${data.value.key}-${data.value.version}.plugin.js`
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <Modal
    :visible="visible"
    :width="960"
    :footer="false"
    unmount-on-close
    body-class="jsrc-modal-body"
    @cancel="close"
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <template #title>
      <div class="jsrc-title">
        <span>插件源码</span>
        <code class="jsrc-title__ident">{{ pluginKey }}@{{ version }}</code>
        <Tag v-if="data" size="small" :color="data.origin === 'builtin' ? 'purple' : 'gray'">
          {{ data.origin === 'builtin' ? '内置' : '上传' }}
        </Tag>
      </div>
    </template>

    <div class="jsrc-body">
      <div class="jsrc-meta">
        <span>{{ lines.length }} 行 · {{ prettySize }}</span>
        <span v-if="hashShort" class="jsrc-meta__hash" :title="data?.source_hash">
          sha256: {{ hashShort }}…
        </span>
        <span v-if="data?.created_at">入库 {{ data.created_at }}</span>
        <span v-if="data?.remark" class="jsrc-meta__remark" :title="data.remark">
          备注：{{ data.remark }}
        </span>
        <span class="jsrc-meta__spacer" />
        <button type="button" class="jsrc-meta__btn" :disabled="!data" @click="copyAll">
          <IconCopy />复制
        </button>
        <button type="button" class="jsrc-meta__btn" :disabled="!data" @click="download">
          <IconDownload />下载
        </button>
      </div>

      <div class="jsrc-view">
        <Spin v-if="loading" class="jsrc-state" />
        <div v-else-if="loadError" class="jsrc-state is-error">{{ loadError }}</div>
        <div v-else-if="!data" class="jsrc-state">暂无源码</div>
        <template v-else>
          <!-- 行号列与代码列同容器滚动，天然对齐 -->
          <pre class="jsrc-view__gutter" aria-hidden="true">{{ lines.map((_, i) => i + 1).join('\n') }}</pre>
          <pre class="jsrc-view__code">{{ data.source }}</pre>
        </template>
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.jsrc-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.jsrc-title__ident {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12.5px;
  font-weight: 400;
  color: var(--color-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jsrc-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 元信息条 + 操作 */
.jsrc-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 14px;
  padding: 8px 12px;
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
  font-size: 12px;
  color: var(--color-text-3);
}

.jsrc-meta__hash {
  font-family: 'JetBrains Mono', Consolas, monospace;
}

.jsrc-meta__remark {
  min-width: 0;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jsrc-meta__spacer {
  flex: 1;
}

.jsrc-meta__btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  background: var(--color-bg-1);
  font-size: 12px;
  color: var(--color-text-2);
  cursor: pointer;
  transition: border-color var(--ta-duration-fast) var(--ta-ease),
    color var(--ta-duration-fast) var(--ta-ease);
}

.jsrc-meta__btn:hover:not(:disabled) {
  border-color: var(--color-primary-6);
  color: var(--color-primary-6);
}

.jsrc-meta__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 代码区：行号列 + 代码列。
   外层整体滚动保证行号与代码纵向对齐；行号列 sticky 钉在左侧，
   横向滚动长行时行号不随内容滑走 */
.jsrc-view {
  display: flex;
  max-height: 65vh;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
  overflow: auto;
}

.jsrc-view__gutter {
  position: sticky;
  left: 0;
  z-index: 1;
  flex-shrink: 0;
  margin: 0;
  padding: 12px 8px 12px 14px;
  text-align: right;
  user-select: none;
  border-right: 1px solid var(--color-border-2);
  background: var(--color-fill-1);
  color: var(--color-text-4);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.65;
}

.jsrc-view__code {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 12px 16px;
  background: var(--color-bg-1);
  color: var(--color-text-1);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.65;
  tab-size: 2;
  white-space: pre;
}

.jsrc-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  font-size: 12.5px;
  color: var(--color-text-3);
}

.jsrc-state.is-error {
  color: rgb(var(--danger-6));
}
</style>
