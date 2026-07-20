<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { Tag, Button, Modal, Message } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import {
  IconFile, IconImage, IconArchive, IconStorage, IconSettings,
} from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'

// ============================================================
// Helpers
// ============================================================
function formatNumber(val: any) {
  const n = Number(val)
  if (!n && n !== 0) return '0'
  return n.toLocaleString()
}

function formatBytes(val: any) {
  const n = Number(val)
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let size = n
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}

const CATEGORY_META: Record<string, { label: string; color: string }> = {
  image: { label: '图片', color: 'arcoblue' },
  export: { label: '导出', color: 'orange' },
  other: { label: '其它', color: 'gray' },
}

// ============================================================
// Stats (KPI)
// ============================================================
const statsLoading = ref(false)
const stats = ref<any>({ total_count: 0, total_bytes: 0, by_category: [], by_provider: [], top_tenants: [] })

function categoryStat(cat: string) {
  return (stats.value.by_category || []).find((c: any) => c.category === cat) || { count: 0, bytes: 0 }
}

const metricCards = computed(() => [
  {
    label: '文件总数', value: formatNumber(stats.value.total_count),
    icon: IconFile, gradient: 'linear-gradient(135deg,#6366f1,#8b5cf6)',
  },
  {
    label: '总占用', value: formatBytes(stats.value.total_bytes),
    icon: IconStorage, gradient: 'linear-gradient(135deg,#0ea5e9,#22d3ee)',
  },
  {
    label: 'AI 图片', value: `${formatNumber(categoryStat('image').count)} · ${formatBytes(categoryStat('image').bytes)}`,
    icon: IconImage, gradient: 'linear-gradient(135deg,#10b981,#34d399)',
  },
  {
    label: '导出文件', value: `${formatNumber(categoryStat('export').count)} · ${formatBytes(categoryStat('export').bytes)}`,
    icon: IconArchive, gradient: 'linear-gradient(135deg,#f59e0b,#fbbf24)',
  },
])

async function fetchStats() {
  statsLoading.value = true
  try {
    const res: any = await request.get('/admin/files/stats', { params: { top_n: 10 } })
    stats.value = res.data?.data || stats.value
  } catch {
    // interceptor already toasts
  } finally {
    statsLoading.value = false
  }
}

// ============================================================
// File list
// ============================================================
const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
  current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const filter = reactive({
  tenant_id: '',
  user_id: '',
  provider: '',
  category: '',
  keyword: '',
  date_range: [] as string[],
})

const providerOptions = [
  { label: '全部供应商', value: '' },
  { label: 'AWS S3', value: 's3' },
  { label: 'MinIO', value: 'minio' },
  { label: 'Cloudflare R2', value: 'r2' },
  { label: '阿里云 OSS', value: 'oss' },
  { label: '腾讯云 COS', value: 'cos' },
]
const categoryOptions = [
  { label: '全部分类', value: '' },
  { label: '图片', value: 'image' },
  { label: '导出', value: 'export' },
  { label: '其它', value: 'other' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '租户', dataIndex: 'tenant_id', width: 80 },
  { title: '上传者', dataIndex: 'user_id', width: 80 },
  { title: '文件名', dataIndex: 'original_name', ellipsis: true, tooltip: true, minWidth: 180 },
  {
    title: '分类', dataIndex: 'category', width: 80,
    render({ record }) {
      const m = CATEGORY_META[record.category] || CATEGORY_META.other
      return h(Tag, { color: m.color, size: 'small' }, () => m.label)
    },
  },
  { title: 'MIME', dataIndex: 'mime_type', width: 140, ellipsis: true },
  {
    title: '大小', dataIndex: 'size', width: 100,
    render({ record }) { return formatBytes(record.size) },
  },
  { title: '供应商', dataIndex: 'storage_provider', width: 90 },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  {
    title: '操作', dataIndex: 'actions', width: 130, fixed: 'right',
    render({ record }) {
      const first = isImage(record)
        ? h(Button, { size: 'mini', type: 'text', onClick: () => openPreview(record) }, () => '预览')
        : h(Button, { size: 'mini', type: 'text', onClick: () => downloadOriginal(record) }, () => '下载')
      const nodes = [
        first,
        // 删除为破坏性操作，按 file:delete 权限渲染（super_admin 直接放行）。
        hasPermission('file:delete')
          ? h(Button, { size: 'mini', type: 'text', status: 'danger', onClick: () => deleteFile(record) }, () => '删除')
          : null,
      ].filter(Boolean)
      return h('div', { style: 'display:flex;gap:4px' }, nodes)
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (filter.tenant_id) params.tenant_id = parseInt(filter.tenant_id)
    if (filter.user_id) params.user_id = parseInt(filter.user_id)
    if (filter.provider) params.provider = filter.provider
    if (filter.category) params.category = filter.category
    if (filter.keyword) params.keyword = filter.keyword.trim()
    if (filter.date_range.length === 2) {
      params.start_date = filter.date_range[0]
      params.end_date = filter.date_range[1]
    }
    const res: any = await request.get('/admin/files', { params })
    const raw = res.data?.data
    data.value = (raw?.list || []).filter(Boolean)
    pagination.total = Number(raw?.total) || 0
  } catch {
    data.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.current = 1
  fetchList()
}

function resetFilter() {
  filter.tenant_id = ''
  filter.user_id = ''
  filter.provider = ''
  filter.category = ''
  filter.keyword = ''
  filter.date_range = []
  search()
}

// ============================================================
// 预览 / 下载
// ============================================================
const previewVisible = ref(false)
const previewLoading = ref(false)
const previewRecord = ref<any>(null)
const previewThumbUrl = ref('')
const previewError = ref(false)

function isImage(record: any) {
  return record?.category === 'image' || String(record?.mime_type || '').startsWith('image/')
}

// 预览图片：弹窗内展示服务端缩略图（小图，加载快），原图仅在点「下载原图」时请求。
async function openPreview(record: any) {
  previewRecord.value = record
  previewThumbUrl.value = ''
  previewError.value = false
  previewVisible.value = true
  previewLoading.value = true
  try {
    const res: any = await request.get(`/admin/files/${record.id}/download`, {
      params: { variant: 'thumb', width: 600 },
    })
    previewThumbUrl.value = res.data?.data?.url || ''
    if (!previewThumbUrl.value) Message.warning('未获取到预览链接')
  } catch {
    // interceptor toasts
  } finally {
    previewLoading.value = false
  }
}

// 下载/打开原图：请求原图签名链接并在新标签打开。
async function downloadOriginal(record: any) {
  if (!record) return
  try {
    const res: any = await request.get(`/admin/files/${record.id}/download`)
    const url = res.data?.data?.url
    if (url) window.open(url, '_blank')
    else Message.warning('未获取到下载链接')
  } catch {
    // interceptor toasts
  }
}

function deleteFile(record: any) {
  Modal.confirm({
    title: '确认删除',
    content: `确定删除文件「${record.original_name || record.id}」？该操作会同时删除存储对象，不可恢复。`,
    okText: '删除', cancelText: '取消',
    onOk: async () => {
      try {
        await request.delete(`/admin/files/${record.id}`)
        Message.success('删除成功')
        fetchList()
        fetchStats()
      } catch {}
    },
  })
}

// ============================================================
// Retention settings + manual cleanup
// ============================================================
const retentionLoading = ref(false)
const retentionSaving = ref(false)
const cleanupLoading = ref(false)
const retentionEnabled = ref(true)
const imageRetentionDays = ref<number>(0)

async function fetchRetention() {
  retentionLoading.value = true
  try {
    const res: any = await request.get('/admin/settings/data_governance')
    const list = res.data?.data?.list || []
    for (const item of list) {
      if (item.key === 'file_image_retention_days') imageRetentionDays.value = Number(item.value ?? item.default ?? 0)
      if (item.key === 'file_retention_enabled') retentionEnabled.value = String(item.value ?? item.default) === 'true'
    }
  } catch {
  } finally {
    retentionLoading.value = false
  }
}

async function saveRetention() {
  retentionSaving.value = true
  try {
    await request.put('/admin/settings/data_governance', {
      settings: {
        file_image_retention_days: String(imageRetentionDays.value ?? 0),
        file_retention_enabled: String(retentionEnabled.value),
      },
    })
    Message.success('保留策略已保存')
  } catch {
  } finally {
    retentionSaving.value = false
  }
}

function manualCleanup() {
  Modal.confirm({
    title: '手动清理过期文件',
    content: '将按当前保留策略立即清理过期的导出文件与 AI 图片（含存储对象）。图片保留天数为 0 时不会清理图片。是否继续？',
    okText: '开始清理', cancelText: '取消',
    onOk: async () => {
      cleanupLoading.value = true
      try {
        const res: any = await request.post('/admin/files/cleanup')
        const r = res.data?.data || {}
        Message.success(`清理完成：导出 ${r.exports_deleted || 0} 个，图片 ${r.images_deleted || 0} 个`)
        fetchList()
        fetchStats()
      } catch {
      } finally {
        cleanupLoading.value = false
      }
    },
  })
}

onMounted(() => {
  fetchStats()
  fetchList()
  fetchRetention()
})
</script>

<template>
  <div>
    <PageHeader title="文件管理" description="对象存储文件观测与清理">
      <template #actions>
        <APopover v-if="hasPermission('system:edit')" trigger="click" position="br">
          <AButton>
            <template #icon><IconSettings /></template>
            保留策略
          </AButton>
          <template #content>
            <div class="retention-pop">
              <div class="retention-pop__row">
                <span>启用自动保留期检查</span>
                <ASwitch v-model="retentionEnabled" :loading="retentionLoading" />
              </div>
              <div class="retention-pop__label">AI 图片保留天数</div>
              <AInputNumber v-model="imageRetentionDays" :min="0" :max="3650" style="width:100%" placeholder="0=不清理" />
              <div class="retention-pop__hint">0 表示不自动删除图片</div>
              <AButton type="primary" long :loading="retentionSaving" @click="saveRetention">保存策略</AButton>
            </div>
          </template>
        </APopover>
        <AButton v-if="hasPermission('file:cleanup')" :loading="cleanupLoading" @click="manualCleanup">手动清理过期文件</AButton>
      </template>
    </PageHeader>

    <!-- 顶部统计条（紧凑单行） -->
    <ASpin :loading="statsLoading" style="display:block">
      <div class="stat-strip mb-4">
        <div v-for="(stat, idx) in metricCards" :key="idx" class="stat-strip__item">
          <div class="stat-strip__icon" :style="{ background: stat.gradient }">
            <component :is="stat.icon" :size="16" style="color:#fff" />
          </div>
          <div class="stat-strip__text">
            <div class="stat-strip__label">{{ stat.label }}</div>
            <div class="stat-strip__value">{{ stat.value }}</div>
          </div>
        </div>
      </div>
    </ASpin>

    <!-- 列表 -->
    <ACard :bordered="false">
      <ASpace wrap style="margin-bottom:16px">
        <AInput v-model="filter.tenant_id" placeholder="租户ID" allow-clear style="width:110px" />
        <AInput v-model="filter.user_id" placeholder="上传者ID" allow-clear style="width:110px" />
        <ASelect v-model="filter.provider" :options="providerOptions" style="width:150px" />
        <ASelect v-model="filter.category" :options="categoryOptions" style="width:130px" />
        <AInput v-model="filter.keyword" placeholder="文件名 / 路径" allow-clear style="width:200px" @press-enter="search" />
        <ARangePicker v-model="filter.date_range" style="width:260px" format="YYYY-MM-DD" />
        <AButton type="primary" @click="search">搜索</AButton>
        <AButton @click="resetFilter">重置</AButton>
      </ASpace>

      <ATable
        :columns="columns" :data="data" :loading="loading"
        :scroll="{ x: 1200 }" :bordered="false" :stripe="true" size="small"
        :pagination="false" row-key="id"
      />
      <div class="table-footer">
        <APagination
          v-model:current="pagination.current"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-size-options="pagination.pageSizeOptions"
          show-total show-page-size
          @change="fetchList"
          @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }"
        />
      </div>
    </ACard>

    <!-- 图片预览弹窗：展示缩略图，下载才取原图 -->
    <AModal
      v-model:visible="previewVisible"
      title="图片预览"
      :footer="false"
      :width="720"
      :body-style="{ padding: '16px' }"
    >
      <ASpin :loading="previewLoading" style="display:block">
        <div class="preview-box">
          <img v-if="previewThumbUrl && !previewError" :src="previewThumbUrl" class="preview-img" alt="预览" @error="previewError = true" />
          <div v-else-if="previewThumbUrl && previewError" class="preview-error">
            <div class="preview-error__icon">😕</div>
            <div class="preview-error__text">图片加载失败，可能是过期或者被清理了</div>
          </div>
          <div v-else class="preview-empty">暂无预览</div>
        </div>
      </ASpin>
      <div v-if="previewRecord" class="preview-meta">
        <div class="preview-name" :title="previewRecord.original_name">{{ previewRecord.original_name }}</div>
        <div class="preview-sub">{{ formatBytes(previewRecord.size) }} · {{ previewRecord.mime_type }}</div>
      </div>
      <div class="preview-actions">
        <AButton @click="previewVisible = false">关闭</AButton>
        <AButton type="primary" @click="downloadOriginal(previewRecord)">下载原图</AButton>
      </div>
    </AModal>
  </div>
</template>

<style scoped>
.stat-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  background: var(--color-bg-2);
  border: 1px solid var(--color-border-2);
  border-radius: 8px;
  padding: 12px 16px;
}
.stat-strip__item {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1 1 0;
  min-width: 160px;
}
.stat-strip__item + .stat-strip__item {
  border-left: 1px solid var(--color-border-2);
  padding-left: 16px;
}
.stat-strip__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  flex-shrink: 0;
}
.stat-strip__text {
  min-width: 0;
}
.stat-strip__label {
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1.2;
}
.stat-strip__value {
  font-size: 17px;
  font-weight: 700;
  color: var(--color-text-1);
  letter-spacing: -0.01em;
  line-height: 1.3;
  white-space: nowrap;
}
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.retention-pop {
  width: 260px;
}
.retention-pop__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.retention-pop__label {
  font-size: 13px;
  color: var(--color-text-2);
  margin-bottom: 8px;
}
.retention-pop__hint {
  color: var(--color-text-3);
  font-size: 12px;
  margin: 6px 0 14px;
}
.preview-box {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 240px;
  max-height: 60vh;
  background: var(--color-fill-2);
  border-radius: 8px;
  overflow: auto;
}
.preview-img {
  max-width: 100%;
  max-height: 60vh;
  object-fit: contain;
  display: block;
}
.preview-error {
  text-align: center;
  padding: 40px 16px;
}
.preview-error__icon {
  font-size: 32px;
  margin-bottom: 8px;
}
.preview-error__text {
  color: var(--color-text-3);
  font-size: 13px;
}
.preview-empty {
  color: var(--color-text-3);
  font-size: 13px;
}
.preview-meta {
  margin-top: 12px;
}
.preview-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-1);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.preview-sub {
  font-size: 12px;
  color: var(--color-text-3);
  margin-top: 2px;
}
.preview-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>
