<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from 'vant'
import request from '@/utils/request'
import StatusTag from '@/components/StatusTag.vue'
import ProviderTag from '@/components/ProviderTag.vue'
import HealthIndicator from '@/components/HealthIndicator.vue'

const router = useRouter()
const route = useRoute()

const channelId = computed(() => route.params.id as string)
const loading = ref(true)
const channel = ref<any>(null)
const keys = ref<any[]>([])
const abilities = ref<any[]>([])

async function fetchChannel() {
  loading.value = true
  try {
    const [chRes, keysRes, abilitiesRes] = await Promise.allSettled([
      request.get(`/admin/channels/${channelId.value}`),
      request.get(`/admin/channels/${channelId.value}/keys`),
      request.get(`/admin/channels/${channelId.value}/abilities`),
    ])

    if (chRes.status === 'fulfilled') {
      channel.value = chRes.value.data?.data
    }
    if (keysRes.status === 'fulfilled') {
      keys.value = keysRes.value.data?.data?.list || keysRes.value.data?.data || []
    }
    if (abilitiesRes.status === 'fulfilled') {
      abilities.value = abilitiesRes.value.data?.data?.list || abilitiesRes.value.data?.data || []
    }
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

async function toggleStatus() {
  if (!channel.value) return
  const newStatus = channel.value.status === 'active' ? 'disabled' : 'active'
  try {
    await request.put(`/admin/channels/${channelId.value}`, { status: newStatus })
    channel.value.status = newStatus
    showToast(newStatus === 'active' ? '已启用' : '已禁用')
  } catch {}
}

// 供应商信息映射
const providerMap: Record<number, { name: string; color: string; icon: string }> = {
  1: { name: 'OpenAI', color: '#10a37f', icon: '◎' },
  2: { name: 'Azure', color: '#0078d4', icon: '◇' },
  3: { name: 'Claude', color: '#d97706', icon: '◈' },
  4: { name: 'Gemini', color: '#4285f4', icon: '◆' },
  5: { name: '百度', color: '#2932e1', icon: '●' },
  6: { name: '阿里', color: '#ff6a00', icon: '◐' },
  7: { name: '字节', color: '#3370ff', icon: '◑' },
  8: { name: '智谱', color: '#4f46e5', icon: '◒' },
  9: { name: '月之暗面', color: '#1a1a2e', icon: '◔' },
  10: { name: 'MiniMax', color: '#6366f1', icon: '◕' },
  11: { name: 'Groq', color: '#f55036', icon: '◉' },
  14: { name: 'Cohere', color: '#39d353', icon: '◍' },
  15: { name: 'Stability', color: '#7c3aed', icon: '▣' },
  16: { name: 'DeepSeek', color: '#4f46e5', icon: '▧' },
  24: { name: 'Mistral', color: '#f97316', icon: '▩' },
}

const provider = computed(() => {
  const info = providerMap[channel.value?.type] || { name: `类型 ${channel.value?.type}`, color: '#64748b', icon: '◻' }
  return info
})

const healthScore = computed(() => channel.value?.health_score ?? 0)
const healthColor = computed(() => {
  if (healthScore.value >= 80) return '#10b981'
  if (healthScore.value >= 50) return '#f59e0b'
  return '#ef4444'
})

const enabledAbilities = computed(() => abilities.value.filter(a => a.enabled))
const disabledAbilities = computed(() => abilities.value.filter(a => !a.enabled))

const activeKeyCount = computed(() => keys.value.filter(k => (k.status || 'active') === 'active').length)

onMounted(fetchChannel)
</script>

<template>
  <div class="channel-detail">
    <!-- Loading -->
    <template v-if="loading">
      <div class="skeleton-hero">
        <div class="skeleton-circle-lg"></div>
        <div class="skeleton-lines">
          <div class="skeleton-line w70"></div>
          <div class="skeleton-line w50"></div>
          <div class="skeleton-line w30"></div>
        </div>
      </div>
    </template>

    <template v-if="channel">
      <!-- Hero Header -->
      <div class="detail-hero" :style="{ '--hero-color': provider.color }">
        <div class="hero-bg">
          <div class="hero-pattern"></div>
        </div>
        <div class="hero-content">
          <div class="hero-top">
            <div class="hero-icon">
              <span class="provider-symbol">{{ provider.icon }}</span>
            </div>
            <div class="hero-status">
              <StatusTag :status="channel.status" />
            </div>
          </div>
          <h2 class="hero-title">{{ channel.name }}</h2>
          <div class="hero-meta">
            <ProviderTag :type="channel.type" />
            <span class="meta-divider">·</span>
            <span class="meta-text">优先级 {{ channel.priority }}</span>
            <span class="meta-divider">·</span>
            <span class="meta-text">权重 {{ channel.weight }}</span>
          </div>
        </div>

        <!-- Health Score Ring -->
        <div class="health-ring-wrap">
          <svg class="health-ring" viewBox="0 0 80 80">
            <circle cx="40" cy="40" r="34" fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="5" />
            <circle
              cx="40" cy="40" r="34" fill="none"
              :stroke="healthColor"
              stroke-width="5"
              stroke-linecap="round"
              :stroke-dasharray="`${(healthScore / 100) * 213.6} 213.6`"
              transform="rotate(-90 40 40)"
              class="health-arc"
            />
            <text x="40" y="36" text-anchor="middle" fill="white" font-size="20" font-weight="700">{{ healthScore }}</text>
            <text x="40" y="50" text-anchor="middle" fill="rgba(255,255,255,0.5)" font-size="9">健康分</text>
          </svg>
        </div>
      </div>

      <!-- Quick Actions -->
      <div class="quick-actions">
        <button
          class="action-btn"
          :class="channel.status === 'active' ? 'action-disable' : 'action-enable'"
          @click="toggleStatus"
        >
          <van-icon :name="channel.status === 'active' ? 'close' : 'checked'" size="16" />
          <span>{{ channel.status === 'active' ? '禁用渠道' : '启用渠道' }}</span>
        </button>
        <div class="action-stats">
          <div class="action-stat">
            <span class="action-stat-value">{{ keys.length }}</span>
            <span class="action-stat-label">密钥</span>
          </div>
          <div class="action-stat-divider"></div>
          <div class="action-stat">
            <span class="action-stat-value">{{ enabledAbilities.length }}</span>
            <span class="action-stat-label">模型</span>
          </div>
          <div class="action-stat-divider"></div>
          <div class="action-stat">
            <span class="action-stat-value">{{ channel.priority }}</span>
            <span class="action-stat-label">优先级</span>
          </div>
        </div>
      </div>

      <!-- Base URL -->
      <div class="detail-section" style="--si: 0">
        <div class="section-header">
          <span class="section-dot" style="--c: #06b6d4"></span>
          <span class="section-title">接入配置</span>
        </div>
        <div class="section-card">
          <div class="url-row">
            <div class="url-label">Base URL</div>
            <div class="url-value">{{ channel.base_url || '-' }}</div>
          </div>
        </div>
      </div>

      <!-- Keys Section -->
      <div class="detail-section" style="--si: 1">
        <div class="section-header">
          <span class="section-dot" style="--c: #f59e0b"></span>
          <span class="section-title">密钥管理</span>
          <span class="section-count">{{ activeKeyCount }}/{{ keys.length }}</span>
        </div>
        <div class="section-card">
          <template v-if="keys.length">
            <div
              v-for="(key, ki) in keys"
              :key="key.id"
              class="key-row"
              :class="{ 'key-disabled': (key.status || 'active') !== 'active' }"
            >
              <div class="key-icon" :style="{ '--kc': (key.status || 'active') === 'active' ? '#f59e0b' : '#94a3b8' }">
                <van-icon name="key" size="16" />
              </div>
              <div class="key-body">
                <span class="key-name">{{ key.name || `Key #${key.id}` }}</span>
                <span class="key-preview">{{ key.key_preview || (key.api_key ? key.api_key.substring(0, 20) + '…' : '-') }}</span>
              </div>
              <StatusTag :status="key.status || 'active'" />
            </div>
          </template>
          <div v-else class="empty-row">
            <van-icon name="info-o" size="16" color="#94a3b8" />
            <span>暂无密钥</span>
          </div>
        </div>
      </div>

      <!-- Abilities Section -->
      <div class="detail-section" style="--si: 2">
        <div class="section-header">
          <span class="section-dot" style="--c: #6366f1"></span>
          <span class="section-title">模型能力</span>
          <span class="section-count">{{ enabledAbilities.length }}/{{ abilities.length }}</span>
        </div>
        <div class="section-card">
          <template v-if="abilities.length">
            <div
              v-for="ab in abilities"
              :key="ab.model_name"
              class="ability-row"
              :class="{ 'ability-disabled': !ab.enabled }"
            >
              <span class="ability-model">{{ ab.model_name }}</span>
              <span v-if="ab.upstream_model && ab.upstream_model !== ab.model_name" class="ability-upstream">
                → {{ ab.upstream_model }}
              </span>
              <div class="ability-right">
                <span class="ability-status-dot" :class="{ active: ab.enabled }"></span>
                <span class="ability-status-text">{{ ab.enabled ? '启用' : '禁用' }}</span>
              </div>
            </div>
          </template>
          <div v-else class="empty-row">
            <van-icon name="info-o" size="16" color="#94a3b8" />
            <span>暂无能力配置</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.channel-detail {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(24px + env(safe-area-inset-bottom, 0px));
}

/* ===== Skeleton ===== */
.skeleton-hero {
  padding: 32px 20px;
}

.skeleton-circle-lg {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: #e2e8f0;
  margin-bottom: 14px;
  animation: pulse 1.5s ease-in-out infinite;
}

.skeleton-lines {
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

.skeleton-line.w70 { width: 70%; }
.skeleton-line.w50 { width: 50%; }
.skeleton-line.w30 { width: 30%; }

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

/* ===== Hero Header ===== */
.detail-hero {
  position: relative;
  padding: 24px 20px 28px;
  overflow: hidden;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, var(--hero-color, #0d9488), color-mix(in srgb, var(--hero-color, #0d9488) 65%, #000));
  border-radius: 0 0 28px 28px;
}

.hero-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 180px 180px at 10% 50%, rgba(255,255,255,0.1) 0%, transparent 70%),
    radial-gradient(ellipse 120px 120px at 90% 80%, rgba(255,255,255,0.05) 0%, transparent 70%);
}

.hero-pattern {
  position: absolute;
  inset: 0;
  opacity: 0.03;
  background-image:
    radial-gradient(circle at 30% 60%, white 1px, transparent 1px),
    radial-gradient(circle at 70% 30%, white 0.5px, transparent 0.5px);
  background-size: 50px 50px, 35px 35px;
}

.hero-content {
  position: relative;
  z-index: 1;
  animation: fadeSlideUp 0.5s both;
}

.hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.hero-icon {
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: rgba(255,255,255,0.15);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  border: 1px solid rgba(255,255,255,0.1);
  display: flex;
  align-items: center;
  justify-content: center;
}

.provider-symbol {
  font-size: 24px;
  color: #fff;
  font-weight: 700;
}

.hero-title {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 8px;
  letter-spacing: -0.2px;
  line-height: 1.3;
}

.hero-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.meta-divider {
  color: rgba(255,255,255,0.25);
  font-size: 12px;
}

.meta-text {
  font-size: 12px;
  color: rgba(255,255,255,0.6);
}

/* ===== Health Ring ===== */
.health-ring-wrap {
  position: absolute;
  right: 20px;
  bottom: 24px;
  z-index: 2;
  animation: fadeSlideUp 0.6s 0.15s both;
}

.health-ring {
  width: 80px;
  height: 80px;
}

.health-arc {
  transition: stroke-dasharray 0.8s cubic-bezier(0.4, 0, 0.2, 1);
}

/* ===== Quick Actions ===== */
.quick-actions {
  padding: 0 16px;
  margin-top: -8px;
  position: relative;
  z-index: 2;
  display: flex;
  gap: 10px;
  animation: fadeSlideUp 0.45s 0.12s both;
}

.action-btn {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 12px 20px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.action-btn:active {
  transform: scale(0.96);
}

.action-disable {
  background: #fef2f2;
  color: #ef4444;
  border: 1px solid #fecaca;
}

.action-disable:active {
  background: #fee2e2;
}

.action-enable {
  background: #f0fdf4;
  color: #16a34a;
  border: 1px solid #bbf7d0;
}

.action-enable:active {
  background: #dcfce7;
}

.action-stats {
  flex: 1;
  display: flex;
  align-items: center;
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  padding: 8px 0;
}

.action-stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
}

.action-stat-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--ta-text-primary, #1e293b);
}

.action-stat-label {
  font-size: 10px;
  color: var(--ta-text-tertiary, #94a3b8);
}

.action-stat-divider {
  width: 1px;
  height: 20px;
  background: var(--ta-border-light, #f1f5f9);
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

/* ===== URL Row ===== */
.url-row {
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.url-label {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-weight: 500;
}

.url-value {
  font-size: 13px;
  color: var(--ta-text-primary, #1e293b);
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  background: #f8fafc;
  padding: 8px 10px;
  border-radius: 8px;
  word-break: break-all;
  line-height: 1.5;
}

/* ===== Key Row ===== */
.key-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  position: relative;
}

.key-row:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 46px;
  right: 16px;
  height: 1px;
  background: var(--ta-border-light, #f1f5f9);
}

.key-row.key-disabled {
  opacity: 0.5;
}

.key-icon {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: color-mix(in srgb, var(--kc) 10%, transparent);
  color: var(--kc);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.key-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.key-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
}

.key-preview {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-family: 'SF Mono', 'Fira Code', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ===== Ability Row ===== */
.ability-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 16px;
  position: relative;
}

.ability-row:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 16px;
  right: 16px;
  height: 1px;
  background: var(--ta-border-light, #f1f5f9);
}

.ability-row.ability-disabled {
  opacity: 0.45;
}

.ability-model {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
  font-family: 'SF Mono', 'Fira Code', monospace;
  white-space: nowrap;
}

.ability-upstream {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ability-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.ability-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #cbd5e1;
}

.ability-status-dot.active {
  background: #10b981;
}

.ability-status-text {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  min-width: 24px;
}

/* ===== Empty Row ===== */
.empty-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 20px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-size: 13px;
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
