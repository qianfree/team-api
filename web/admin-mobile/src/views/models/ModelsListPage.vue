<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'

const router = useRouter()

const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const models = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')
const activeTab = ref('models')

const categoryFilters = [
  { key: '', label: '全部' },
  { key: 'chat', label: '对话' },
  { key: 'embedding', label: '向量' },
  { key: 'image', label: '图像' },
  { key: 'audio', label: '音频' },
  { key: 'rerank', label: '重排' },
]
const activeCategory = ref('')

async function fetchModels(append = false) {
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
    if (activeCategory.value) params.category = activeCategory.value

    const { data: res } = await request.get('/admin/models', { params })
    const list = res.data?.list || []
    total.value = res.data?.total || 0

    if (append) {
      models.value = [...models.value, ...list]
    } else {
      models.value = list
    }
    finished.value = models.value.length >= total.value
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function onRefresh() {
  refreshing.value = true
  await fetchModels(false)
}

async function onLoad() {
  if (finished.value) return
  page.value++
  await fetchModels(true)
}

function onSearch(val: string) {
  search.value = val
  fetchModels(false)
}

function setCategory(key: string) {
  activeCategory.value = key
  fetchModels(false)
}

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
  chat: '#0d9488',
  embedding: '#6366f1',
  image: '#ec4899',
  audio: '#f59e0b',
  rerank: '#8b5cf6',
  video: '#ef4444',
}

const categoryLabelMap: Record<string, string> = {
  chat: '对话',
  embedding: '向量',
  image: '图像',
  audio: '音频',
  rerank: '重排',
  video: '视频',
}

function catColor(cat: string): string {
  return categoryColorMap[cat] || '#64748b'
}

onMounted(() => fetchModels())
</script>

<template>
  <div class="page">

    <!-- Sub-tab: Models / Channels -->
    <div class="sub-tabs">
      <div class="sub-tab sub-tab--active">模型</div>
      <div class="sub-tab" @click="router.push('/m/channels')">渠道</div>
    </div>

    <!-- Search -->
    <div class="search-wrap">
      <van-search
        v-model="search"
        placeholder="搜索模型名称..."
        shape="round"
        @search="onSearch"
        @clear="onSearch('')"
      />
    </div>

    <!-- Category filter chips -->
    <div class="cat-scroll">
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

    <!-- Count -->
    <div v-if="total > 0" class="count-bar">
      <span class="count-text">共 <b>{{ total }}</b> 个模型</span>
    </div>

    <!-- Model Cards -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <van-list v-model:loading="loading" :finished="finished" finished-text="" @load="onLoad">
        <div class="card-list">
          <div
            v-for="(item, idx) in models"
            :key="item.id"
            class="model-card"
            :style="{ animationDelay: `${Math.min(idx, 8) * 0.04}s` }"
            @click="router.push({ path: `/m/models/${item.id}`, state: { model: JSON.parse(JSON.stringify(item)) } })"
          >
            <!-- Card top row: name + status -->
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

            <!-- Card mid: category + context -->
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

            <!-- Card bottom: pricing -->
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

        <div v-if="!loading && !models.length" class="empty-state">
          <van-icon name="search" size="36" color="#cbd5e1" />
          <span>没有找到匹配的模型</span>
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

/* ── Category chips ── */
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

/* ── Card: Top ── */
.model-card__top {
  margin-bottom: 8px;
}

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

/* ── Card: Meta ── */
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

/* ── Card: Pricing ── */
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
