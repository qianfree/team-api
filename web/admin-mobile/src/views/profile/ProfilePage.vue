<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const user = computed(() => authStore.user)
const displayName = computed(() => user.value?.display_name || user.value?.username || '管理员')
const roleLabel = computed(() => {
  if (user.value?.role === 'super_admin') return '超级管理员'
  if (user.value?.role === 'admin') return '管理员'
  return user.value?.role || '-'
})
const initials = computed(() => {
  const name = displayName.value
  if (!name) return 'A'
  return name.charAt(0).toUpperCase()
})

const accountMenus = [
  { icon: 'lock', label: '修改密码', color: '#6366f1', action: () => {} },
  { icon: 'shield-o', label: '两步验证', color: '#0d9488', action: () => {}, badge: '未开启' },
]

const systemMenus = [
  { icon: 'records-o', label: '会话管理', color: '#3b82f6', action: () => router.push('/m/sessions') },
  { icon: 'desktop-o', label: '桌面版', color: '#64748b', action: () => { window.location.href = '/admin/?desktop=1' } },
]

async function handleLogout() {
  try {
    await showConfirmDialog({
      title: '退出登录',
      message: '确定要退出当前账号吗？',
    })
    await authStore.logout()
    showToast('已退出登录')
    router.replace('/m/login')
  } catch {
    // cancelled
  }
}
</script>

<template>
  <div class="profile-page">
    <!-- Gradient Header -->
    <div class="profile-header">
      <div class="header-bg">
        <div class="header-pattern"></div>
      </div>

      <div class="header-content">
        <!-- Avatar -->
        <div class="avatar-wrapper">
          <div class="avatar-ring">
            <div class="avatar-inner">
              <span class="avatar-text">{{ initials }}</span>
            </div>
          </div>
          <div class="avatar-status-dot"></div>
        </div>

        <!-- User Info -->
        <h2 class="user-name">{{ displayName }}</h2>
        <div class="user-meta">
          <span class="role-badge">
            <van-icon name="shield-o" size="12" />
            {{ roleLabel }}
          </span>
          <span class="username-text">@{{ user?.username }}</span>
        </div>
      </div>

      <!-- Stats Bar -->
      <div class="stats-bar">
        <div class="stat-item">
          <span class="stat-value">128</span>
          <span class="stat-label">操作</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">7</span>
          <span class="stat-label">天在线</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">{{ roleLabel === '超级管理员' ? '∞' : '标准' }}</span>
          <span class="stat-label">权限</span>
        </div>
      </div>
    </div>

    <!-- Menu Sections -->
    <div class="menu-sections">
      <!-- Account Security -->
      <div class="menu-section" style="--delay: 0">
        <div class="section-header">
          <span class="section-icon" style="--c: #6366f1">
            <van-icon name="shield-o" size="14" />
          </span>
          <span class="section-title">账号安全</span>
        </div>
        <div class="section-card">
          <div
            v-for="(item, i) in accountMenus"
            :key="i"
            class="menu-row"
            :class="{ 'has-badge': item.badge }"
            @click="item.action"
          >
            <div class="menu-row-icon" :style="{ '--c': item.color }">
              <van-icon :name="item.icon" size="18" />
            </div>
            <span class="menu-row-label">{{ item.label }}</span>
            <span v-if="item.badge" class="menu-row-badge">{{ item.badge }}</span>
            <van-icon name="arrow" size="14" class="menu-row-arrow" />
          </div>
        </div>
      </div>

      <!-- System -->
      <div class="menu-section" style="--delay: 1">
        <div class="section-header">
          <span class="section-icon" style="--c: #3b82f6">
            <van-icon name="setting-o" size="14" />
          </span>
          <span class="section-title">系统</span>
        </div>
        <div class="section-card">
          <div
            v-for="(item, i) in systemMenus"
            :key="i"
            class="menu-row"
            @click="item.action"
          >
            <div class="menu-row-icon" :style="{ '--c': item.color }">
              <van-icon :name="item.icon" size="18" />
            </div>
            <span class="menu-row-label">{{ item.label }}</span>
            <van-icon name="arrow" size="14" class="menu-row-arrow" />
          </div>
        </div>
      </div>
    </div>

    <!-- Logout -->
    <div class="logout-section">
      <button class="logout-btn" @click="handleLogout">
        <van-icon name="revoke" size="16" />
        <span>退出登录</span>
      </button>
    </div>

    <!-- Version -->
    <div class="version-info">
      <span class="version-dot"></span>
      <span>Team-API Admin v0.1.0</span>
    </div>
  </div>
</template>

<style scoped>
.profile-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(16px + env(safe-area-inset-bottom, 0px));
}

/* ===== Header ===== */
.profile-header {
  position: relative;
  padding: 32px 20px 20px;
  overflow: hidden;
}

.header-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #0d9488 0%, #0f766e 40%, #115e59 100%);
  border-radius: 0 0 28px 28px;
}

.header-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 180px 180px at 20% 30%, rgba(255,255,255,0.12) 0%, transparent 70%),
    radial-gradient(ellipse 120px 120px at 80% 60%, rgba(255,255,255,0.06) 0%, transparent 70%);
  pointer-events: none;
}

.header-pattern {
  position: absolute;
  inset: 0;
  opacity: 0.04;
  background-image:
    radial-gradient(circle at 20% 50%, white 1px, transparent 1px),
    radial-gradient(circle at 80% 20%, white 1px, transparent 1px),
    radial-gradient(circle at 50% 80%, white 0.5px, transparent 0.5px);
  background-size: 60px 60px, 40px 40px, 50px 50px;
}

.header-content {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 8px;
}

/* ===== Avatar ===== */
.avatar-wrapper {
  position: relative;
  margin-bottom: 14px;
  animation: avatarFloat 0.6s cubic-bezier(0.34, 1.56, 0.64, 1) both;
}

@keyframes avatarFloat {
  from {
    opacity: 0;
    transform: scale(0.6) translateY(10px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.avatar-ring {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  padding: 3px;
  background: linear-gradient(135deg, rgba(255,255,255,0.4), rgba(255,255,255,0.1));
  box-shadow:
    0 4px 16px rgba(0,0,0,0.15),
    0 0 0 4px rgba(255,255,255,0.08);
}

.avatar-inner {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: linear-gradient(135deg, #f0fdfa, #ccfbf1);
  display: flex;
  align-items: center;
  justify-content: center;
}

.avatar-text {
  font-size: 32px;
  font-weight: 700;
  background: linear-gradient(135deg, #0d9488, #0f766e);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  line-height: 1;
}

.avatar-status-dot {
  position: absolute;
  bottom: 4px;
  right: 4px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #10b981;
  border: 3px solid #115e59;
  box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.3);
}

/* ===== User Info ===== */
.user-name {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  margin: 0 0 8px;
  letter-spacing: 0.3px;
  animation: fadeSlideUp 0.5s 0.15s both;
}

.user-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  animation: fadeSlideUp 0.5s 0.25s both;
}

.role-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border-radius: 20px;
  background: rgba(255,255,255,0.18);
  color: rgba(255,255,255,0.95);
  font-size: 12px;
  font-weight: 500;
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  border: 1px solid rgba(255,255,255,0.12);
}

.username-text {
  font-size: 12px;
  color: rgba(255,255,255,0.55);
}

/* ===== Stats Bar ===== */
.stats-bar {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  margin: 20px 12px 0;
  padding: 14px 0;
  background: rgba(255,255,255,0.1);
  border-radius: 16px;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(255,255,255,0.08);
  animation: fadeSlideUp 0.5s 0.35s both;
}

.stat-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.stat-value {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.stat-label {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
}

.stat-divider {
  width: 1px;
  height: 24px;
  background: rgba(255,255,255,0.12);
}

/* ===== Menu Sections ===== */
.menu-sections {
  padding: 8px 16px 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.menu-section {
  animation: fadeSlideUp 0.45s calc(0.4s + var(--delay, 0) * 0.08s) both;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px 10px;
}

.section-icon {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--c) 10%, transparent);
  color: var(--c);
  display: flex;
  align-items: center;
  justify-content: center;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ta-text-secondary, #475569);
  letter-spacing: 0.5px;
}

.section-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  box-shadow:
    0 1px 3px rgba(0,0,0,0.03),
    0 1px 2px rgba(0,0,0,0.04);
  overflow: hidden;
}

/* ===== Menu Row ===== */
.menu-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  cursor: pointer;
  transition: background 0.15s;
  position: relative;
}

.menu-row:not(:last-child)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 48px;
  right: 16px;
  height: 1px;
  background: var(--ta-border-light, #f1f5f9);
}

.menu-row:active {
  background: #f8fafc;
}

.menu-row-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--c) 10%, transparent);
  color: var(--c);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: transform 0.2s;
}

.menu-row:active .menu-row-icon {
  transform: scale(0.92);
}

.menu-row-label {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
  color: var(--ta-text-primary, #1e293b);
}

.menu-row-badge {
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  padding: 2px 8px;
  border-radius: 10px;
  background: #f1f5f9;
}

.menu-row-arrow {
  color: var(--ta-text-weakest, #cbd5e1);
  transition: transform 0.15s;
}

.menu-row:active .menu-row-arrow {
  transform: translateX(2px);
}

/* ===== Logout ===== */
.logout-section {
  padding: 28px 20px 8px;
  animation: fadeSlideUp 0.45s 0.7s both;
}

.logout-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 13px 0;
  border: 1.5px solid #fecaca;
  border-radius: 14px;
  background: #fff;
  color: #ef4444;
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}

.logout-btn:active {
  background: #fef2f2;
  border-color: #fca5a5;
  transform: scale(0.98);
}

/* ===== Version ===== */
.version-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 16px 0 4px;
  font-size: 11px;
  color: var(--ta-text-tertiary, #94a3b8);
  animation: fadeSlideUp 0.45s 0.8s both;
}

.version-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #cbd5e1;
}

/* ===== Animations ===== */
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
