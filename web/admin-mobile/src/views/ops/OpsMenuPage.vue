<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

const menuSections = [
  {
    title: '用户与租户',
    icon: 'friends-o',
    accent: '#6366f1',
    items: [
      { icon: 'friends-o', label: '租户管理', color: '#6366f1', route: '/m/tenants' },
      { icon: 'user-circle-o', label: '用户管理', color: '#64748b', route: '/m/users' },
    ],
  },
  {
    title: '财务与订单',
    icon: 'gold-coin-o',
    accent: '#f59e0b',
    items: [
      { icon: 'gold-coin-o', label: '财务', color: '#f59e0b', route: '/m/wallets' },
      { icon: 'balance-list-o', label: '账单记录', color: '#10b981', route: '/m/billing' },
      { icon: 'orders-o', label: '订单管理', color: '#3b82f6', route: '/m/orders' },
      { icon: 'gift-o', label: '兑换码', color: '#ec4899', route: '/m/redemptions' },
      { icon: 'coupon-o', label: '优惠码', color: '#8b5cf6', route: '/m/promo-codes' },
    ],
  },
  {
    title: '运维与监控',
    icon: 'shield-o',
    accent: '#ef4444',
    items: [
      { icon: 'shield-o', label: '安全审计', color: '#ef4444', route: '/m/audit' },
      { icon: 'records-o', label: '请求日志', color: '#06b6d4', route: '/m/usage-logs' },
      { icon: 'bar-chart-o', label: '错误日志', color: '#ef4444', route: '/m/error-logs' },
    ],
  },
  {
    title: '消息与支持',
    icon: 'chat-o',
    accent: '#ec4899',
    items: [
      { icon: 'chat-o', label: '通知消息', color: '#ec4899', route: '/m/notifications' },
      { icon: 'question-o', label: '工单管理', color: '#f97316', route: '/m/tickets' },
    ],
  },
  {
    title: '系统配置',
    icon: 'setting',
    accent: '#14b8a6',
    items: [
      { icon: 'apps-o', label: '插件管理', color: '#8b5cf6', route: '/m/plugins' },
      { icon: 'setting', label: '系统设置', color: '#64748b', route: '/m/settings' },
    ],
  },
]
</script>

<template>
  <div class="ops-page">
    <!-- Page Header -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">运营管理</h2>
        <p class="page-subtitle">管理与监控中心</p>
      </div>
      <div class="header-badge">
        <van-icon name="apps-o" size="18" />
      </div>
    </div>

    <!-- Quick Stats -->
    <div class="quick-stats">
      <div class="stat-chip">
        <span class="stat-dot" style="background: #10b981"></span>
        <span class="stat-chip-text">系统正常</span>
      </div>
      <div class="stat-chip">
        <span class="stat-dot" style="background: #f59e0b"></span>
        <span class="stat-chip-text">5 项待处理</span>
      </div>
      <div class="stat-chip">
        <span class="stat-dot" style="background: #6366f1"></span>
        <span class="stat-chip-text">{{ menuSections.length }} 个模块</span>
      </div>
    </div>

    <!-- Sections -->
    <div class="sections-list">
      <div
        v-for="(section, si) in menuSections"
        :key="section.title"
        class="ops-section"
        :style="{ '--section-delay': si }"
      >
        <!-- Section Header -->
        <div class="ops-section-header">
          <div class="section-accent-line" :style="{ background: section.accent }"></div>
          <span class="ops-section-title">{{ section.title }}</span>
          <span class="ops-section-count">{{ section.items.length }}</span>
        </div>

        <!-- Section Card -->
        <div class="ops-section-card">
          <div class="ops-grid">
            <div
              v-for="(item, ii) in section.items"
              :key="item.label"
              class="ops-item"
              :style="{ '--item-delay': ii }"
              @click="router.push(item.route)"
            >
              <div class="ops-item-icon" :style="{ '--c': item.color }">
                <div class="ops-item-icon-bg"></div>
                <van-icon :name="item.icon" size="22" class="ops-item-icon-svg" />
              </div>
              <span class="ops-item-label">{{ item.label }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
.ops-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ===== Header ===== */
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 20px 0;
  animation: fadeSlideUp 0.4s both;
}

.page-title {
  font-size: 24px;
  font-weight: 800;
  color: var(--ta-text-primary, #1e293b);
  margin: 0;
  letter-spacing: -0.3px;
}

.page-subtitle {
  font-size: 13px;
  color: var(--ta-text-tertiary, #94a3b8);
  margin: 4px 0 0;
}

.header-badge {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: linear-gradient(135deg, #f0fdfa, #ccfbf1);
  color: #0d9488;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* ===== Quick Stats ===== */
.quick-stats {
  display: flex;
  gap: 8px;
  padding: 14px 20px 4px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  animation: fadeSlideUp 0.4s 0.08s both;

  /* Hide scrollbar */
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.quick-stats::-webkit-scrollbar {
  display: none;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 20px;
  background: var(--ta-bg-card, #fff);
  box-shadow: 0 1px 2px rgba(0,0,0,0.04);
  white-space: nowrap;
  flex-shrink: 0;
}

.stat-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.stat-chip-text {
  font-size: 12px;
  font-weight: 500;
  color: var(--ta-text-secondary, #475569);
}

/* ===== Sections ===== */
.sections-list {
  padding: 8px 16px 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.ops-section {
  animation: fadeSlideUp 0.45s calc(0.15s + var(--section-delay, 0) * 0.06s) both;
}

.ops-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px 10px;
}

.section-accent-line {
  width: 3px;
  height: 14px;
  border-radius: 2px;
  flex-shrink: 0;
}

.ops-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
  flex: 1;
}

.ops-section-count {
  font-size: 11px;
  font-weight: 600;
  color: var(--ta-text-tertiary, #94a3b8);
  background: var(--ta-bg-secondary, #f1f5f9);
  padding: 1px 7px;
  border-radius: 8px;
  min-width: 20px;
  text-align: center;
}

/* ===== Section Card ===== */
.ops-section-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  box-shadow:
    0 1px 3px rgba(0,0,0,0.03),
    0 1px 2px rgba(0,0,0,0.04);
  padding: 6px 4px;
}

/* ===== Grid ===== */
.ops-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
}

.ops-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 14px 4px 12px;
  cursor: pointer;
  border-radius: 12px;
  transition: background 0.15s, transform 0.15s;
  -webkit-tap-highlight-color: transparent;
}

.ops-item:active {
  background: #f8fafc;
  transform: scale(0.94);
}

/* ===== Item Icon ===== */
.ops-item-icon {
  position: relative;
  width: 46px;
  height: 46px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ops-item-icon-bg {
  position: absolute;
  inset: 0;
  border-radius: 14px;
  background: color-mix(in srgb, var(--c) 10%, transparent);
  transition: transform 0.2s, background 0.2s;
}

.ops-item:active .ops-item-icon-bg {
  transform: scale(0.9);
  background: color-mix(in srgb, var(--c) 16%, transparent);
}

.ops-item-icon-svg {
  position: relative;
  z-index: 1;
  color: var(--c);
}

/* ===== Item Label ===== */
.ops-item-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--ta-text-secondary, #475569);
  text-align: center;
  line-height: 1.3;
  max-width: 64px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ===== Notice ===== */
.notice-area {
  padding: 20px 16px 8px;
  animation: fadeSlideUp 0.45s 0.55s both;
}

.notice-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-radius: 12px;
  background: #f0fdfa;
  border: 1px solid #ccfbf1;
}

.notice-text {
  font-size: 12px;
  color: #0d9488;
  font-weight: 500;
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
