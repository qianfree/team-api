<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const router = useRouter()

// ===== Tab Control =====
const activeSegment = ref<'models' | 'channels' | 'groups'>('models')

// ===== Models State =====
const modelsLoading = ref(false)
const modelsRefreshing = ref(false)
const modelsFinished = ref(false)
const models = ref<any[]>([])
const modelsPage = ref(1)
const modelsTotal = ref(0)
const modelsSearch = ref('')

const categoryFilters = [
  { key: '', label: '全部' },
  { key: 'chat', label: '对话' },
  { key: 'embedding', label: '向量' },
  { key: 'image', label: '图像' },
  { key: 'audio', label: '音频' },
  { key: 'rerank', label: '重排' },
]
const activeCategory = ref('')

// ===== Channels State =====
const channelsLoading = ref(false)
const channelsRefreshing = ref(false)
const channelsFinished = ref(false)
const channels = ref<any[]>([])
const channelsPage = ref(1)
const channelsTotal = ref(0)
const channelsSearch = ref('')

// ===== Groups State =====
const groupsLoading = ref(false)
const groupsRefreshing = ref(false)
const groupsFinished = ref(false)
const groups = ref<any[]>([])
const groupsPage = ref(1)
const groupsTotal = ref(0)
const groupsSearch = ref('')
const activeGroupStatus = ref('')

const groupStatusFilters = [
  { key: '', label: '全部' },
  { key: 'active', label: '启用' },
  { key: 'disabled', label: '禁用' },
]

const showCreateDialog = ref(false)
const createForm = ref({ name: '', code: '', description: '', sort_order: 0 })
const creating = ref(false)

// ===== Models Logic =====
async function fetchModels(append = false) {
  if (!append) {
    modelsPage.value = 1
    modelsFinished.value = false
  }
  modelsLoading.value = true
  try {
    const params: any = { page: modelsPage.value, page_size: 20 }
    if (modelsSearch.value) params.search = modelsSearch.value
    if (activeCategory.value) params.category = activeCategory.value
    const { data: res } = await request.get('/admin/models', { params })
    const list = res.data?.list || []
    modelsTotal.value = res.data?.total || 0
    models.value = append ? [...models.value, ...list] : list
    modelsFinished.value = models.value.length >= modelsTotal.value
  } catch {
    // handled by interceptor
  } finally {
    modelsLoading.value = false
    modelsRefreshing.value = false
  }
}

function onModelsRefresh() {
  modelsRefreshing.value = true
  fetchModels(false)
}

function onModelsLoad() {
  if (modelsFinished.value) return
  modelsPage.value++
  fetchModels(true)
}

function onModelsSearch(val: string) {
  modelsSearch.value = val
  fetchModels(false)
}

function setCategory(key: string) {
  activeCategory.value = key
  fetchModels(false)
}

// ===== Channels Logic =====
async function fetchChannels(append = false) {
  if (!append) {
    channelsPage.value = 1
    channelsFinished.value = false
  }
  channelsLoading.value = true
  try {
    const params: any = { page: channelsPage.value, page_size: 20 }
    if (channelsSearch.value) params.search = channelsSearch.value
    const { data: res } = await request.get('/admin/channels', { params })
    const list = res.data?.list || []
    channelsTotal.value = res.data?.total || 0
    channels.value = append ? [...channels.value, ...list] : list
    channelsFinished.value = channels.value.length >= channelsTotal.value
  } catch {
    // handled by interceptor
  } finally {
    channelsLoading.value = false
    channelsRefreshing.value = false
  }
}

function onChannelsRefresh() {
  channelsRefreshing.value = true
  fetchChannels(false)
}

function onChannelsLoad() {
  if (channelsFinished.value) return
  channelsPage.value++
  fetchChannels(true)
}

function onChannelsSearch(val: string) {
  channelsSearch.value = val
  fetchChannels(false)
}

async function toggleChannel(channel: any) {
  const newStatus = channel.status === 'active' ? 'disabled' : 'active'
  try {
    await request.put(`/admin/channels/${channel.id}`, { status: newStatus })
    channel.status = newStatus
    showToast(newStatus === 'active' ? '已启用' : '已禁用')
  } catch {}
}

async function deleteChannel(channel: any) {
  try {
    await showConfirmDialog({
      title: '确认删除',
      message: `确定要删除渠道「${channel.name}」吗？此操作不可恢复。`,
    })
    await request.delete(`/admin/channels/${channel.id}`)
    channels.value = channels.value.filter(c => c.id !== channel.id)
    showToast('已删除')
  } catch {
    // cancelled or error
  }
}

// ===== Groups Logic =====
async function fetchGroups(append = false) {
  if (!append) {
    groupsPage.value = 1
    groupsFinished.value = false
  }
  groupsLoading.value = true
  try {
    const params: any = { page: groupsPage.value, page_size: 20 }
    if (groupsSearch.value) params.search = groupsSearch.value
    if (activeGroupStatus.value) params.status = activeGroupStatus.value
    const { data: res } = await request.get('/admin/model-groups', { params })
    const list = res.data?.list || []
    groupsTotal.value = res.data?.total || 0
    groups.value = append ? [...groups.value, ...list] : list
    groupsFinished.value = groups.value.length >= groupsTotal.value
  } catch {
    // handled by interceptor
  } finally {
    groupsLoading.value = false
    groupsRefreshing.value = false
  }
}

function onGroupsRefresh() {
  groupsRefreshing.value = true
  fetchGroups(false)
}

function onGroupsLoad() {
  if (groupsFinished.value) return
  groupsPage.value++
  fetchGroups(true)
}

function onGroupsSearch(val: string) {
  groupsSearch.value = val
  fetchGroups(false)
}

function setGroupStatus(key: string) {
  activeGroupStatus.value = key
  fetchGroups(false)
}

function openCreateDialog() {
  createForm.value = { name: '', code: '', description: '', sort_order: 0 }
  showCreateDialog.value = true
}

async function createGroup() {
  if (!createForm.value.name.trim()) {
    showToast('请输入分组名称')
    return
  }
  if (!createForm.value.code.trim()) {
    showToast('请输入分组编码')
    return
  }
  creating.value = true
  try {
    await request.post('/admin/model-groups', createForm.value)
    showToast('创建成功')
    showCreateDialog.value = false
    createForm.value = { name: '', code: '', description: '', sort_order: 0 }
    await fetchGroups(false)
  } catch {
    // handled by interceptor
  } finally {
    creating.value = false
  }
}

async function toggleGroup(group: any) {
  const newStatus = group.status === 'active' ? 'disabled' : 'active'
  try {
    await request.put(`/admin/model-groups/${group.id}`, {
      name: group.name,
      description: group.description,
      sort_order: group.sort_order,
      status: newStatus,
    })
    group.status = newStatus
    showToast(newStatus === 'active' ? '已启用' : '已禁用')
  } catch {
    // handled by interceptor
  }
}

async function deleteGroup(group: any) {
  try {
    await showConfirmDialog({
      title: '确认删除',
      message: `确定要删除分组「${group.name}」吗？此操作不可恢复。`,
    })
    await request.delete(`/admin/model-groups/${group.id}`)
    groups.value = groups.value.filter(g => g.id !== group.id)
    groupsTotal.value = Math.max(0, groupsTotal.value - 1)
    showToast('已删除')
  } catch {
    // cancelled or error
  }
}

// ===== Formatters =====
function formatPrice(p: number | undefined): string {
  if (!p) return '-'
  if (p >= 1) return `$${p.toFixed(2)}`
  if (p >= 0.01) return `$${p.toFixed(3)}`
  return `$${p.toFixed(6)}`
}

function formatTokens(n: number | undefined): string {
  if (!n) return '-'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(0)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return String(n)
}

const categoryColorMap: Record<string, string> = {
  chat: '#0d9488', embedding: '#6366f1', image: '#ec4899',
  audio: '#f59e0b', rerank: '#8b5cf6', video: '#ef4444',
}

const categoryLabelMap: Record<string, string> = {
  chat: '对话', embedding: '向量', image: '图像',
  audio: '音频', rerank: '重排', video: '视频',
}

function catColor(cat: string): string {
  return categoryColorMap[cat] || '#64748b'
}

const providerMap: Record<number, { name: string; color: string }> = {
  1: { name: 'OpenAI', color: '#10a37f' },
  2: { name: 'Azure', color: '#0078d4' },
  3: { name: 'Claude', color: '#d97706' },
  4: { name: 'Gemini', color: '#4285f4' },
  5: { name: '百度', color: '#2932e1' },
  6: { name: '阿里', color: '#ff6a00' },
  7: { name: '字节', color: '#3370ff' },
  8: { name: '智谱', color: '#4f46e5' },
  9: { name: '月之暗面', color: '#1a1a2e' },
  10: { name: 'MiniMax', color: '#6366f1' },
  11: { name: 'Groq', color: '#f55036' },
  14: { name: 'Cohere', color: '#39d353' },
  16: { name: 'DeepSeek', color: '#4f46e5' },
  24: { name: 'Mistral', color: '#f97316' },
}

function providerInfo(type: number) {
  return providerMap[type] || { name: `类型${type}`, color: '#64748b' }
}

function healthColor(score: number): string {
  if (score >= 80) return '#10b981'
  if (score >= 50) return '#f59e0b'
  return '#ef4444'
}

// ===== Init =====
onMounted(() => {
  fetchModels()
})

watch(activeSegment, (val) => {
  if (val === 'channels' && channels.value.length === 0 && !channelsLoading.value) {
    fetchChannels()
  }
  if (val === 'groups' && groups.value.length === 0 && !groupsLoading.value) {
    fetchGroups()
  }
})
</script>

<template>
  <div class="models-tab-page">
    <!-- Fixed Header: search + segment + filters -->
    <div class="page-header">
      <!-- Search Bar -->
      <div class="search-wrap">
        <van-search
          v-if="activeSegment === 'models'"
          v-model="modelsSearch"
          placeholder="搜索模型名称..."
          shape="round"
          @search="onModelsSearch"
          @clear="onModelsSearch('')"
        />
        <van-search
          v-else-if="activeSegment === 'channels'"
          v-model="channelsSearch"
          placeholder="搜索渠道名称..."
          shape="round"
          @search="onChannelsSearch"
          @clear="onChannelsSearch('')"
        />
        <van-search
          v-else
          v-model="groupsSearch"
          placeholder="搜索分组名称或编码..."
          shape="round"
          @search="onGroupsSearch"
          @clear="onGroupsSearch('')"
        />
      </div>

      <!-- Segmented Control -->
      <div class="segment-control">
        <div
          class="segment-pill"
          :class="{ active: activeSegment === 'models' }"
          @click="activeSegment = 'models'"
        >
          <van-icon name="chart-trending-o" size="14" />
          <span>模型</span>
          <span v-if="modelsTotal" class="segment-count">{{ modelsTotal }}</span>
        </div>
        <div
          class="segment-pill"
          :class="{ active: activeSegment === 'channels' }"
          @click="activeSegment = 'channels'"
        >
          <van-icon name="cluster-o" size="14" />
          <span>渠道</span>
          <span v-if="channelsTotal" class="segment-count">{{ channelsTotal }}</span>
        </div>
        <div
          class="segment-pill"
          :class="{ active: activeSegment === 'groups' }"
          @click="activeSegment = 'groups'"
        >
          <van-icon name="apps-o" size="14" />
          <span>分组</span>
          <span v-if="groupsTotal" class="segment-count">{{ groupsTotal }}</span>
        </div>
      </div>

      <!-- Category chips (models only) -->
      <div v-if="activeSegment === 'models'" class="cat-scroll">
        <div
          v-for="c in categoryFilters"
          :key="c.key"
          class="cat-chip"
          :class="{ 'cat-chip--active': activeCategory === c.key }"
          @click="setCategory(c.key)"
        >
          {{ c.label }}
        </div>
      </div>

      <!-- Status chips (groups only) -->
      <div v-if="activeSegment === 'groups'" class="cat-scroll">
        <div
          v-for="f in groupStatusFilters"
          :key="f.key"
          class="cat-chip"
          :class="{ 'cat-chip--active': activeGroupStatus === f.key }"
          @click="setGroupStatus(f.key)"
        >
          {{ f.label }}
        </div>
      </div>

      <!-- Count -->
      <div v-if="activeSegment === 'models' && modelsTotal > 0" class="count-bar">
        <span class="count-text">共 <b>{{ modelsTotal }}</b> 个模型</span>
      </div>
      <div v-if="activeSegment === 'channels' && channelsTotal > 0" class="count-bar">
        <span class="count-text">共 <b>{{ channelsTotal }}</b> 个渠道</span>
      </div>
      <div v-if="activeSegment === 'groups' && groupsTotal > 0" class="count-bar">
        <span class="count-text">共 <b>{{ groupsTotal }}</b> 个分组</span>
      </div>
    </div>

    <!-- Scrollable Body -->
    <div class="page-body">
      <!-- ===== Models List ===== -->
      <div v-show="activeSegment === 'models'">
        <van-pull-refresh v-model="modelsRefreshing" @refresh="onModelsRefresh">
          <van-list v-model:loading="modelsLoading" :finished="modelsFinished" finished-text="" @load="onModelsLoad">
            <div class="card-list">
              <div
                v-for="(item, idx) in models"
                :key="item.id"
                class="model-card"
                :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                @click="router.push({ path: `/m/models/${item.id}`, state: { model: JSON.parse(JSON.stringify(item)) } })"
              >
                <div class="model-card__top">
                  <div class="model-card__name-row">
                    <h4 class="model-card__name">{{ item.model_name || item.model_id }}</h4>
                    <span
                      class="model-card__status"
                      :class="{
                        'model-card__status--active': item.status === 'active',
                        'model-card__status--deprecated': item.status === 'deprecated',
                      }"
                    >
                      {{ item.status === 'active' ? '启用' : item.status === 'deprecated' ? '已弃用' : item.status }}
                    </span>
                  </div>
                  <div class="model-card__id">{{ item.model_id }}</div>
                </div>
                <div class="model-card__meta">
                  <span class="model-card__cat" :style="{ color: catColor(item.category), background: `${catColor(item.category)}12` }">
                    {{ categoryLabelMap[item.category] || item.category || '-' }}
                  </span>
                  <span v-if="item.max_context_tokens" class="model-card__ctx">
                    <van-icon name="expand-o" size="11" />
                    {{ formatTokens(item.max_context_tokens) }}
                  </span>
                  <span v-if="item.pricing_mode === 'per_request' && item.per_request_price" class="model-card__ctx">
                    按次 {{ formatPrice(item.per_request_price) }}
                  </span>
                </div>
                <div v-if="item.input_price || item.output_price" class="model-card__pricing">
                  <div class="price-item">
                    <span class="price-label">输入</span>
                    <span class="price-val">{{ formatPrice(item.input_price) }}</span>
                  </div>
                  <div class="price-divider" />
                  <div class="price-item">
                    <span class="price-label">输出</span>
                    <span class="price-val">{{ formatPrice(item.output_price) }}</span>
                  </div>
                  <span class="price-unit">/1M tokens</span>
                </div>
              </div>
            </div>
            <div v-if="!modelsLoading && !models.length" class="empty-state">
              <van-icon name="search" size="36" color="#cbd5e1" />
              <span>没有找到匹配的模型</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </div>

      <!-- ===== Channels List ===== -->
      <div v-show="activeSegment === 'channels'">
        <van-pull-refresh v-model="channelsRefreshing" @refresh="onChannelsRefresh">
          <van-list v-model:loading="channelsLoading" :finished="channelsFinished" finished-text="" @load="onChannelsLoad">
            <div class="card-list">
              <van-swipe-cell v-for="(item, idx) in channels" :key="item.id">
                <div
                  class="channel-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                  @click="router.push(`/m/channels/${item.id}`)"
                >
                  <div class="channel-card__top">
                    <div class="channel-card__name-row">
                      <h4 class="channel-card__name">{{ item.name }}</h4>
                      <div class="channel-card__badges">
                        <span
                          class="channel-card__status"
                          :class="{
                            'channel-card__status--active': item.status === 'active',
                            'channel-card__status--disabled': item.status === 'disabled',
                          }"
                        >
                          {{ item.status === 'active' ? '启用' : item.status === 'disabled' ? '禁用' : item.status }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div class="channel-card__meta">
                    <span
                      class="channel-card__provider"
                      :style="{ color: providerInfo(item.type).color, background: `${providerInfo(item.type).color}12` }"
                    >
                      {{ providerInfo(item.type).name }}
                    </span>
                    <div class="channel-card__health">
                      <span class="health-dot" :style="{ background: healthColor(item.health_score ?? 0) }" />
                      <span class="health-text" :style="{ color: healthColor(item.health_score ?? 0) }">
                        {{ item.health_score ?? '-' }}
                      </span>
                    </div>
                    <span v-if="item.priority" class="channel-card__info">P{{ item.priority }}</span>
                    <span v-if="item.weight" class="channel-card__info">W{{ item.weight }}</span>
                  </div>
                  <div class="channel-card__bar">
                    <div class="bar-track">
                      <div
                        class="bar-fill"
                        :style="{
                          width: `${Math.min(100, (item.health_score ?? 0))}%`,
                          background: healthColor(item.health_score ?? 0),
                        }"
                      />
                    </div>
                  </div>
                </div>
                <template #right>
                  <div class="swipe-actions">
                    <div
                      class="swipe-btn"
                      :class="item.status === 'active' ? 'swipe-btn--danger' : 'swipe-btn--success'"
                      @click="toggleChannel(item)"
                    >
                      {{ item.status === 'active' ? '禁用' : '启用' }}
                    </div>
                    <div class="swipe-btn swipe-btn--delete" @click="deleteChannel(item)">
                      删除
                    </div>
                  </div>
                </template>
              </van-swipe-cell>
            </div>
            <div v-if="!channelsLoading && !channels.length" class="empty-state">
              <van-icon name="search" size="36" color="#cbd5e1" />
              <span>没有找到匹配的渠道</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </div>

      <!-- ===== Groups List ===== -->
      <div v-show="activeSegment === 'groups'">
        <van-pull-refresh v-model="groupsRefreshing" @refresh="onGroupsRefresh">
          <van-list v-model:loading="groupsLoading" :finished="groupsFinished" finished-text="" @load="onGroupsLoad">
            <div class="card-list">
              <van-swipe-cell v-for="(item, idx) in groups" :key="item.id">
                <div
                  class="group-card"
                  :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
                >
                  <div class="group-card__top">
                    <div class="group-card__name-row">
                      <h4 class="group-card__name">{{ item.name }}</h4>
                      <span
                        class="group-card__status"
                        :class="{
                          'group-card__status--active': item.status === 'active',
                          'group-card__status--disabled': item.status === 'disabled',
                        }"
                      >
                        {{ item.status === 'active' ? '启用' : '禁用' }}
                      </span>
                    </div>
                    <div class="group-card__code">{{ item.code }}</div>
                  </div>
                  <div v-if="item.description" class="group-card__desc">
                    {{ item.description }}
                  </div>
                  <div class="group-card__badges">
                    <span class="group-badge">
                      <van-icon name="cluster-o" size="12" />
                      {{ item.model_count ?? 0 }} 模型
                    </span>
                    <span class="group-badge">
                      <van-icon name="friends-o" size="12" />
                      {{ item.tenant_count ?? 0 }} 租户
                    </span>
                    <span v-if="item.sort_order" class="group-badge group-badge--muted">
                      排序 {{ item.sort_order }}
                    </span>
                  </div>
                </div>
                <template #right>
                  <div class="swipe-actions">
                    <div
                      class="swipe-btn"
                      :class="item.status === 'active' ? 'swipe-btn--danger' : 'swipe-btn--success'"
                      @click="toggleGroup(item)"
                    >
                      {{ item.status === 'active' ? '禁用' : '启用' }}
                    </div>
                    <div class="swipe-btn swipe-btn--delete" @click="deleteGroup(item)">
                      删除
                    </div>
                  </div>
                </template>
              </van-swipe-cell>
            </div>
            <div v-if="!groupsLoading && !groups.length" class="empty-state">
              <van-icon name="apps-o" size="36" color="#cbd5e1" />
              <span>暂无模型分组</span>
              <span class="empty-hint">点击右下角按钮创建第一个分组</span>
            </div>
          </van-list>
        </van-pull-refresh>
      </div>
    </div>

    <!-- FAB (groups only) -->
    <div v-if="activeSegment === 'groups'" class="fab" @click="openCreateDialog">
      <van-icon name="plus" size="22" color="#fff" />
    </div>

    <!-- Create Group Dialog -->
    <van-dialog
      v-model:show="showCreateDialog"
      title="创建分组"
      show-cancel-button
      :loading="creating"
      :before-close="(action: string) => { if (action === 'confirm') return createGroup(); return true }"
    >
      <div class="dialog-form">
        <div class="form-field">
          <label class="form-label">名称 <span class="form-required">*</span></label>
          <van-field v-model="createForm.name" placeholder="输入分组名称" maxlength="50" show-word-limit :border="false" />
        </div>
        <div class="form-field">
          <label class="form-label">编码 <span class="form-required">*</span></label>
          <van-field v-model="createForm.code" placeholder="输入分组编码（英文）" maxlength="30" show-word-limit :border="false" />
        </div>
        <div class="form-field">
          <label class="form-label">描述</label>
          <van-field v-model="createForm.description" type="textarea" placeholder="输入分组描述（可选）" rows="2" maxlength="200" show-word-limit autosize :border="false" />
        </div>
        <div class="form-field">
          <label class="form-label">排序</label>
          <van-field v-model.number="createForm.sort_order" type="digit" placeholder="数值越小越靠前" :border="false" />
        </div>
      </div>
    </van-dialog>
  </div>
</template>

<style scoped>
.models-tab-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

/* ===== Fixed Header ===== */
.page-header {
  flex-shrink: 0;
  z-index: 10;
  background: var(--ta-bg-page, #f8fafc);
}

/* ===== Search ===== */
.search-wrap {
  padding: 4px 0 0;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ===== Segmented Control ===== */
.segment-control {
  display: flex;
  gap: 0;
  padding: 0 16px 12px;
}

.segment-pill {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 9px 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-tertiary, #94a3b8);
  background: var(--ta-bg-secondary, #f1f5f9);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  -webkit-tap-highlight-color: transparent;
  user-select: none;
}

.segment-pill:first-child {
  border-radius: 10px 0 0 10px;
}

.segment-pill:last-child {
  border-radius: 0 10px 10px 0;
}

.segment-pill:not(:last-child) {
  border-right: 1px solid #e2e8f0;
}

.segment-pill.active {
  background: var(--ta-primary, #0d9488);
  color: #fff;
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.25);
}

.segment-pill:active:not(.active) {
  background: #e2e8f0;
}

.segment-count {
  font-size: 10px;
  font-weight: 700;
  padding: 0 5px;
  border-radius: 8px;
  line-height: 16px;
  background: rgba(255, 255, 255, 0.2);
}

.segment-pill:not(.active) .segment-count {
  background: rgba(148, 163, 184, 0.15);
}

/* ===== Category / Status chips ===== */
.cat-scroll {
  display: flex;
  gap: 8px;
  padding: 4px 16px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.cat-scroll::-webkit-scrollbar { display: none; }

.cat-chip {
  flex-shrink: 0;
  padding: 5px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.cat-chip--active {
  color: #fff;
  background: #0d9488;
  box-shadow: 0 2px 8px rgba(13,148,136,0.3);
}

/* ===== Count ===== */
.count-bar {
  padding: 4px 16px 6px;
}
.count-text {
  font-size: 12px;
  color: #94a3b8;
}
.count-text b {
  color: #475569;
  font-weight: 600;
}

/* ===== Scrollable Body ===== */
.page-body {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

/* ===== Card List ===== */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px 12px;
}

/* ===== Model Card ===== */
.model-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.model-card:active {
  transform: scale(0.98);
  box-shadow: 0 1px 2px rgba(0,0,0,0.06);
}

@keyframes cardIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.model-card__top { margin-bottom: 8px; }

.model-card__name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.model-card__name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.model-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  background: rgba(16,185,129,0.1);
  color: #10b981;
}
.model-card__status--deprecated {
  background: rgba(245,158,11,0.1);
  color: #f59e0b;
}

.model-card__id {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 2px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.model-card__cat {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
}

.model-card__ctx {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.model-card__pricing {
  display: flex;
  align-items: center;
  gap: 0;
  background: #f8fafc;
  border-radius: 8px;
  padding: 8px 12px;
}

.price-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.price-label {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
}

.price-val {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.price-divider {
  width: 1px;
  height: 24px;
  background: #e2e8f0;
  margin: 0 12px;
}

.price-unit {
  font-size: 9px;
  color: #94a3b8;
  margin-left: auto;
  white-space: nowrap;
  align-self: flex-end;
}

/* ===== Channel Card ===== */
.channel-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.channel-card:active {
  transform: scale(0.98);
}

.channel-card__top { margin-bottom: 8px; }

.channel-card__name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.channel-card__name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.channel-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.channel-card__status {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  background: rgba(16,185,129,0.1);
  color: #10b981;
}
.channel-card__status--disabled {
  background: rgba(239,68,68,0.08);
  color: #ef4444;
}

.channel-card__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.channel-card__provider {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
}

.channel-card__health {
  display: flex;
  align-items: center;
  gap: 4px;
}

.health-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.health-text {
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.channel-card__info {
  font-size: 11px;
  color: #94a3b8;
  font-weight: 500;
}

.channel-card__bar { margin-top: 2px; }

.bar-track {
  height: 3px;
  background: #f1f5f9;
  border-radius: 2px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}

/* ===== Group Card ===== */
.group-card {
  background: #fff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04);
  transition: transform 0.15s, box-shadow 0.15s;
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.group-card:active {
  transform: scale(0.98);
}

.group-card__top { margin-bottom: 8px; }

.group-card__name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.group-card__name {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.group-card__status {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  line-height: 1.4;
  background: rgba(16,185,129,0.1);
  color: #10b981;
}
.group-card__status--disabled {
  background: rgba(239,68,68,0.08);
  color: #ef4444;
}

.group-card__code {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 2px;
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-card__desc {
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
  margin-bottom: 8px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.group-card__badges {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.group-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: #475569;
  font-weight: 500;
  background: #f1f5f9;
  padding: 3px 8px;
  border-radius: 6px;
}
.group-badge--muted {
  color: #94a3b8;
  background: #f8fafc;
}

/* ===== Swipe Actions ===== */
.swipe-actions {
  display: flex;
  height: 100%;
  border-radius: 0 14px 14px 0;
  overflow: hidden;
}

.swipe-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
}
.swipe-btn--success { background: #10b981; }
.swipe-btn--danger { background: #f59e0b; color: #fff; }
.swipe-btn--delete { background: #ef4444; }

/* ===== Empty ===== */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0 24px;
  color: #94a3b8;
  font-size: 13px;
}

.empty-hint {
  font-size: 12px;
  color: #cbd5e1;
}

/* ===== FAB ===== */
.fab {
  position: fixed;
  bottom: 24px;
  right: 20px;
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  box-shadow: 0 4px 16px rgba(13, 148, 136, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 100;
  transition: transform 0.2s, box-shadow 0.2s;
}
.fab:active {
  transform: scale(0.92);
  box-shadow: 0 2px 8px rgba(13, 148, 136, 0.3);
}

/* ===== Create Dialog ===== */
.dialog-form {
  padding: 8px 16px 0;
}

.form-field {
  margin-bottom: 12px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  margin-bottom: 4px;
}

.form-required {
  color: #ef4444;
}

.form-field :deep(.van-cell) {
  background: #f8fafc;
  border-radius: 10px;
  padding: 8px 12px;
}

.form-field :deep(.van-field__control) {
  font-size: 14px;
}

.form-field :deep(.van-field__word-limit) {
  font-size: 10px;
  color: #94a3b8;
}

/* ===== Animation ===== */
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
</style>
