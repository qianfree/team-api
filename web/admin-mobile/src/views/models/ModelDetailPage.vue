<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'

const route = useRoute()
const modelId = computed(() => route.params.id as string)
const loading = ref(true)
const model = ref<any>(null)
const pricing = ref<any>(null)

async function fetchModel() {
  loading.value = true
  try {
    // 后端没有 GET /admin/models/{id} 单模型详情接口
    // 从列表页路由 state 获取模型数据，只额外请求定价
    model.value = history.state?.model || null

    // 如果没有 state 数据，从列表接口按 ID 查找
    if (!model.value) {
      const { data: res } = await request.get('/admin/models', {
        params: { page: 1, page_size: 1 },
      })
      const list = res.data?.list || []
      // 列表接口可能不支持按 id 筛选，这里做个兜底
      const found = list.find((m: any) => String(m.id) === modelId.value)
      if (found) model.value = found
    }

    // 获取定价信息（路径参数是 model_id 数字 ID）
    if (modelId.value) {
      try {
        const { data: pricingRes } = await request.get(`/admin/models/${modelId.value}/pricing`)
        pricing.value = pricingRes.data
      } catch {
        // 定价接口可能不存在
      }
    }
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

function formatPrice(price: number | undefined): string {
  if (price === undefined || price === null) return '-'
  return `$${price.toFixed(6)}`
}

function formatNumber(n: number | undefined): string {
  if (!n) return '-'
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return String(n)
}

// 推断模型供应商颜色
const providerInfo = computed(() => {
  const id = (model.value?.model_id || '').toLowerCase()
  if (id.includes('gpt') || id.includes('o1') || id.includes('o3') || id.includes('dall')) return { name: 'OpenAI', color: '#10a37f' }
  if (id.includes('claude')) return { name: 'Anthropic', color: '#d97706' }
  if (id.includes('gemini')) return { name: 'Google', color: '#4285f4' }
  if (id.includes('deepseek')) return { name: 'DeepSeek', color: '#4f46e5' }
  if (id.includes('qwen')) return { name: '阿里', color: '#ff6a00' }
  if (id.includes('glm') || id.includes('chatglm')) return { name: '智谱', color: '#4f46e5' }
  if (id.includes('moonshot') || id.includes('kimi')) return { name: '月之暗面', color: '#1a1a2e' }
  if (id.includes('llama')) return { name: 'Meta', color: '#0668E1' }
  if (id.includes('mistral')) return { name: 'Mistral', color: '#f97316' }
  return { name: '其他', color: '#64748b' }
})

// 能力标签映射
const capabilityLabels: Record<string, string> = {
  vision: '视觉理解',
  function_call: '函数调用',
  streaming: '流式输出',
  json_mode: 'JSON 模式',
  tool_use: '工具调用',
  code_interpreter: '代码解释',
  web_search: '联网搜索',
  embedding: '向量嵌入',
  audio: '音频处理',
  image_generation: '图像生成',
  fine_tuning: '微调',
  batch: '批量处理',
}

const capabilities = computed(() => {
  const caps = model.value?.capabilities
  if (!caps || typeof caps !== 'object') return []
  return Object.entries(caps).map(([key, value]) => ({
    key,
    label: capabilityLabels[key] || key,
    enabled: !!value,
  }))
})

const tags = computed(() => model.value?.tags || [])

onMounted(fetchModel)
</script>

<template>
  <div class="model-detail">
    <!-- Loading Skeleton -->
    <template v-if="loading">
      <div class="skeleton-header">
        <div class="skeleton-circle"></div>
        <div class="skeleton-lines">
          <div class="skeleton-line w60"></div>
          <div class="skeleton-line w40"></div>
        </div>
      </div>
    </template>

    <template v-if="model">
      <!-- Hero Header -->
      <div class="detail-hero" :style="{ '--hero-color': providerInfo.color }">
        <div class="hero-bg">
          <div class="hero-pattern"></div>
        </div>
        <div class="hero-content">
          <div class="hero-icon">
            <van-icon name="chart-trending-o" size="28" />
          </div>
          <h2 class="hero-title">{{ model.model_name || model.model_id }}</h2>
          <p class="hero-subtitle">{{ model.model_id }}</p>
          <div class="hero-tags">
            <StatusTag :status="model.status" />
            <span class="provider-chip" :style="{ color: providerInfo.color, borderColor: providerInfo.color + '30', background: providerInfo.color + '10' }">
              {{ providerInfo.name }}
            </span>
            <span v-if="model.category" class="category-chip">{{ model.category }}</span>
          </div>
        </div>
      </div>

      <!-- Token Stats -->
      <div class="token-stats">
        <div class="token-stat-card">
          <div class="token-stat-icon" style="--c: #6366f1">
            <van-icon name="arrow-down" size="16" />
          </div>
          <div class="token-stat-body">
            <span class="token-stat-value">{{ formatNumber(model.max_context_tokens) }}</span>
            <span class="token-stat-label">最大上下文</span>
          </div>
        </div>
        <div class="token-stat-card">
          <div class="token-stat-icon" style="--c: #0d9488">
            <van-icon name="arrow-up" size="16" />
          </div>
          <div class="token-stat-body">
            <span class="token-stat-value">{{ formatNumber(model.max_output_tokens) }}</span>
            <span class="token-stat-label">最大输出</span>
          </div>
        </div>
      </div>

      <!-- Pricing Section -->
      <div class="detail-section" style="--si: 0">
        <div class="section-header">
          <span class="section-dot" style="--c: #f59e0b"></span>
          <span class="section-title">定价信息</span>
        </div>
        <div class="section-card">
          <template v-if="pricing">
            <div class="price-grid">
              <div class="price-item">
                <span class="price-label">输入价格</span>
                <span class="price-value">{{ formatPrice(pricing.input_price) }}</span>
                <span class="price-unit">/ 1K tokens</span>
              </div>
              <div class="price-item">
                <span class="price-label">输出价格</span>
                <span class="price-value">{{ formatPrice(pricing.output_price) }}</span>
                <span class="price-unit">/ 1K tokens</span>
              </div>
              <div v-if="pricing.cache_read_price" class="price-item">
                <span class="price-label">缓存读取</span>
                <span class="price-value">{{ formatPrice(pricing.cache_read_price) }}</span>
                <span class="price-unit">/ 1K tokens</span>
              </div>
              <div v-if="pricing.cache_creation_price" class="price-item">
                <span class="price-label">缓存创建</span>
                <span class="price-value">{{ formatPrice(pricing.cache_creation_price) }}</span>
                <span class="price-unit">/ 1K tokens</span>
              </div>
            </div>
            <div class="price-footer">
              <span class="price-mode">计费模式：{{ pricing.pricing_mode || pricing.mode || 'token' }}</span>
            </div>
          </template>
          <div v-else class="empty-pricing">
            <van-icon name="info-o" size="16" color="#94a3b8" />
            <span>暂无定价信息</span>
          </div>
        </div>
      </div>

      <!-- Capabilities -->
      <div v-if="capabilities.length" class="detail-section" style="--si: 1">
        <div class="section-header">
          <span class="section-dot" style="--c: #10b981"></span>
          <span class="section-title">模型能力</span>
          <span class="section-count">{{ capabilities.filter(c => c.enabled).length }}/{{ capabilities.length }}</span>
        </div>
        <div class="section-card">
          <div class="capability-grid">
            <div
              v-for="cap in capabilities"
              :key="cap.key"
              class="capability-chip"
              :class="{ active: cap.enabled }"
            >
              <span class="cap-dot"></span>
              <span>{{ cap.label }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Description -->
      <div v-if="model.description" class="detail-section" style="--si: 2">
        <div class="section-header">
          <span class="section-dot" style="--c: #3b82f6"></span>
          <span class="section-title">描述</span>
        </div>
        <div class="section-card">
          <p class="description-text">{{ model.description }}</p>
        </div>
      </div>

      <!-- Tags -->
      <div v-if="tags.length" class="detail-section" style="--si: 3">
        <div class="section-header">
          <span class="section-dot" style="--c: #8b5cf6"></span>
          <span class="section-title">标签</span>
        </div>
        <div class="section-card">
          <div class="tags-wrap">
            <span v-for="tag in tags" :key="tag" class="detail-tag">{{ tag }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.model-detail {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ===== Loading Skeleton ===== */
.skeleton-header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 32px 20px;
}

.skeleton-circle {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  background: #e2e8f0;
  animation: pulse 1.5s ease-in-out infinite;
}

.skeleton-lines {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-line {
  height: 14px;
  border-radius: 6px;
  background: #e2e8f0;
  animation: pulse 1.5s ease-in-out infinite;
}

.skeleton-line.w60 { width: 60%; }
.skeleton-line.w40 { width: 40%; }

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* ===== Hero Header ===== */
.detail-hero {
  position: relative;
  padding: 28px 20px 24px;
  overflow: hidden;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, var(--hero-color, #0d9488), color-mix(in srgb, var(--hero-color, #0d9488) 70%, #000));
  border-radius: 0 0 28px 28px;
}

.hero-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 200px 200px at 15% 40%, rgba(255,255,255,0.1) 0%, transparent 70%),
    radial-gradient(ellipse 140px 140px at 85% 70%, rgba(255,255,255,0.05) 0%, transparent 70%);
}

.hero-pattern {
  position: absolute;
  inset: 0;
  opacity: 0.03;
  background-image:
    radial-gradient(circle at 25% 50%, white 1px, transparent 1px),
    radial-gradient(circle at 75% 25%, white 0.5px, transparent 0.5px);
  background-size: 50px 50px, 40px 40px;
}

.hero-content {
  position: relative;
  z-index: 1;
  animation: fadeSlideUp 0.5s both;
}

.hero-icon {
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: rgba(255,255,255,0.15);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  border: 1px solid rgba(255,255,255,0.1);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 14px;
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 4px;
  letter-spacing: -0.2px;
  line-height: 1.3;
}

.hero-subtitle {
  font-size: 13px;
  color: rgba(255,255,255,0.55);
  margin: 0 0 14px;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
}

.hero-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.provider-chip {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid;
}

.category-chip {
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(255,255,255,0.1);
  color: rgba(255,255,255,0.7);
  border: 1px solid rgba(255,255,255,0.08);
}

/* ===== Token Stats ===== */
.token-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  padding: 0 16px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
  animation: fadeSlideUp 0.45s 0.12s both;
}

.token-stat-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  box-shadow:
    0 2px 8px rgba(0,0,0,0.04),
    0 1px 2px rgba(0,0,0,0.03);
}

.token-stat-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--c) 10%, transparent);
  color: var(--c);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.token-stat-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.token-stat-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  letter-spacing: -0.3px;
}

.token-stat-label {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* ===== Sections ===== */
.detail-section {
  padding: 16px 16px 0;
  animation: fadeSlideUp 0.45s calc(0.2s + var(--si, 0) * 0.06s) both;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px 10px;
}

.section-dot {
  width: 8px;
  height: 8px;
  border-radius: 3px;
  background: var(--c);
  flex-shrink: 0;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
  flex: 1;
}

.section-count {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  background: var(--ta-bg-secondary, #f1f5f9);
  padding: 1px 7px;
  border-radius: 8px;
}

.section-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  padding: 4px 0;
  box-shadow:
    0 1px 3px rgba(0,0,0,0.03),
    0 1px 2px rgba(0,0,0,0.04);
}

/* ===== Pricing Grid ===== */
.price-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
}

.price-item {
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  position: relative;
}

.price-item:not(:last-child):nth-child(odd)::after {
  content: '';
  position: absolute;
  right: 0;
  top: 12px;
  bottom: 12px;
  width: 1px;
  background: var(--ta-border-light, #f1f5f9);
}

.price-item:nth-child(n+3) {
  border-top: 1px solid var(--ta-border-light, #f1f5f9);
}

.price-label {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
}

.price-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
  letter-spacing: -0.3px;
}

.price-unit {
  font-size: 10px;
  color: var(--ta-text-weakest, #cbd5e1);
}

.price-footer {
  padding: 10px 16px;
  border-top: 1px solid var(--ta-border-light, #f1f5f9);
}

.price-mode {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
}

.empty-pricing {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 20px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-size: 13px;
}

/* ===== Capabilities ===== */
.capability-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 14px;
}

.capability-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 10px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid #f1f5f9;
  font-size: 12px;
  font-weight: 500;
  color: var(--ta-text-tertiary, #94a3b8);
  transition: all 0.15s;
}

.capability-chip.active {
  background: #f0fdfa;
  border-color: #ccfbf1;
  color: #0d9488;
}

.cap-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #cbd5e1;
  flex-shrink: 0;
}

.capability-chip.active .cap-dot {
  background: #0d9488;
}

/* ===== Description ===== */
.description-text {
  padding: 14px 16px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--ta-text-secondary, #475569);
  margin: 0;
}

/* ===== Tags ===== */
.tags-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px 14px;
}

.detail-tag {
  font-size: 12px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 8px;
  background: #f0fdfa;
  color: #0d9488;
  border: 1px solid #ccfbf1;
}

/* ===== Animation ===== */
@keyframes fadeSlideUp {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
