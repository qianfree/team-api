<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

const router = useRouter()

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const channels = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')

async function fetchChannels(append = false) {
  if (!append) {
    page.value = 1
    finished.value = false
  }

  loading.value = true
  try {
    const params: any = {
      page: page.value,
      page_size: 20,
    }
    if (search.value) params.search = search.value

    const { data: res } = await request.get('/admin/channels', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      channels.value = [...channels.value, ...list]
    } else {
      channels.value = list
    }
    finished.value = channels.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchChannels(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchChannels(true)
}

function onSearch(val: string) {
  search.value = val
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

onMounted(() => fetchChannels())
</script>

<template>
  <div class="page">

    <!-- Sub-tab: Models / Channels -->
    <div class="sub-tabs">
      <div class="sub-tab" @click="router.push('/m/models')">模型</div>
      <div class="sub-tab sub-tab--active">渠道</div>
    </div>

    <!-- Search -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索渠道名称..."
        shape="round"
        @search="onSearch"
        @clear="onSearch('')"
      />
    </div>

    <!-- Count -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个渠道</span>
    </div>

    <!-- Channel Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <van-swipe-cell v-for="(item, idx) in channels" :key="item.id">
            <div
              class="channel-card"
              :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
              @click="router.push(`/m/channels/${item.id}`)"
            >
              <!-- Top row: name + actions -->
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

              <!-- Mid row: provider + health -->
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

              <!-- Bottom: priority/weight bar -->
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

            <!-- Swipe actions -->
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

        <div v-if="!loading && !channels.length" class="empty-state">
          <van-icon name="search" size="36" color="#cbd5e1" />
          <span>没有找到匹配的渠道</span>
        </div>
      </van-list>
    </van-pull-refresh>
  </div>
</template>

<style scoped>
.page {
  padding-bottom: 24px;
}

/* ── Sub Tabs ── */
.sub-tabs {
  display: flex;
  gap: 0;
  padding: 12px 16px 0;
}
.sub-tab {
  flex: 1;
  text-align: center;
  padding: 8px 0;
  font-size: 14px;
  font-weight: 600;
  color: #94a3b8;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.2s, border-color 0.2s;
}
.sub-tab--active {
  color: #0d9488;
  border-bottom-color: #0d9488;
}

/* ── Search ── */
.search-wrap {
  padding: 4px 0 0;
}
.search-wrap :deep(.van-search) {
  padding: 8px 12px;
}

/* ── Count ── */
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

/* ── Card List ── */
.card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

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

@keyframes cardIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ── Card: Top ── */
.channel-card__top {
  margin-bottom: 8px;
}

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

/* ── Card: Meta ── */
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

/* ── Card: Health Bar ── */
.channel-card__bar {
  margin-top: 2px;
}

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

/* ── Swipe Actions ── */
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

/* ── Empty ── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 0 24px;
  color: #94a3b8;
  font-size: 13px;
}
</style>
