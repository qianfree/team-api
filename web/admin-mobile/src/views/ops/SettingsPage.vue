<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { showToast, showConfirmDialog } from 'vant'
import request from '@/utils/request'

/* ── Types ── */
interface Category {
  key: string
  label: string
  icon: string
  order: number
}

interface SettingItem {
  key: string
  value: any
  type: 'text' | 'number' | 'boolean' | 'select' | 'textarea'
  label: string
  description: string
  sensitive: boolean
  validation: string
  default: any
}

/* ── State ── */
const categories = ref<Category[]>([])
const activeCategory = ref('')
const settings = ref<SettingItem[]>([])
const originalSettings = ref<Record<string, any>>({})
const formSettings = ref<Record<string, any>>({})
const loadingCategories = ref(false)
const loadingSettings = ref(false)
const saving = ref(false)
const revealMap = ref<Record<string, boolean>>({})

// Picker state for select fields
const showPicker = ref(false)
const pickerColumns = ref<{ text: string; value: string }[]>([])
const pickerTargetKey = ref('')
const pickerTargetLabel = ref('')

/* ── Computed ── */
const changedKeys = computed(() => {
  const keys: string[] = []
  for (const k of Object.keys(formSettings.value)) {
    if (formSettings.value[k] !== originalSettings.value[k]) {
      keys.push(k)
    }
  }
  return keys
})

const hasChanges = computed(() => changedKeys.value.length > 0)

/* ── Fetch categories ── */
async function fetchCategories() {
  loadingCategories.value = true
  try {
    const { data: res } = await request.get('/admin/settings/categories')
    categories.value = res.data?.categories || []
    if (categories.value.length > 0) {
      activeCategory.value = categories.value[0].key
      await fetchSettings(activeCategory.value)
    }
  } catch {
    // handled by interceptor
  } finally {
    loadingCategories.value = false
  }
}

/* ── Fetch settings for a category ── */
async function fetchSettings(category: string) {
  loadingSettings.value = true
  settings.value = []
  formSettings.value = {}
  originalSettings.value = {}
  revealMap.value = {}

  try {
    const { data: res } = await request.get(`/admin/settings/${category}`)
    const list: SettingItem[] = res.data?.settings || []
    settings.value = list

    const form: Record<string, any> = {}
    const orig: Record<string, any> = {}
    for (const item of list) {
      form[item.key] = item.value ?? item.default ?? ''
      orig[item.key] = item.value ?? item.default ?? ''
    }
    formSettings.value = form
    originalSettings.value = orig
  } catch {
    // handled by interceptor
  } finally {
    loadingSettings.value = false
  }
}

/* ── Select category ── */
async function selectCategory(key: string) {
  if (key === activeCategory.value) return
  activeCategory.value = key
  await fetchSettings(key)
}

/* ── Parse select options ── */
function getSelectOptions(item: SettingItem): string[] {
  // Try parsing validation field as JSON array
  if (item.validation) {
    try {
      const parsed = JSON.parse(item.validation)
      if (Array.isArray(parsed)) return parsed.map(String)
      if (parsed.options && Array.isArray(parsed.options)) return parsed.options.map(String)
    } catch {
      // Not JSON, fall through
    }
  }
  // Fall back to comma-separated description
  if (item.description) {
    const parts = item.description
      .split(/[,，、;；]/)
      .map(s => s.trim())
      .filter(Boolean)
    if (parts.length > 1) return parts
  }
  return []
}

/* ── Open picker for select field ── */
function openPicker(item: SettingItem) {
  const options = getSelectOptions(item)
  if (options.length === 0) return
  pickerColumns.value = options.map(o => ({ text: o, value: o }))
  pickerTargetKey.value = item.key
  pickerTargetLabel.value = item.label
  showPicker.value = true
}

function onPickerConfirm({ selectedOptions }: any) {
  const val = selectedOptions[0]?.text ?? selectedOptions[0]?.value ?? ''
  formSettings.value[pickerTargetKey.value] = val
  showPicker.value = false
}

/* ── Toggle sensitive field visibility ── */
function toggleReveal(key: string) {
  revealMap.value[key] = !revealMap.value[key]
}

/* ── Save ── */
async function handleSave() {
  if (!hasChanges.value) {
    showToast('没有修改需要保存')
    return
  }

  try {
    await showConfirmDialog({
      title: '保存设置',
      message: '确定保存设置？',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
  } catch {
    return // cancelled
  }

  saving.value = true
  try {
    const changed: Record<string, any> = {}
    for (const k of changedKeys.value) {
      changed[k] = formSettings.value[k]
    }

    await request.put(`/admin/settings/${activeCategory.value}`, {
      settings: changed,
    })

    // Update original to match current
    for (const k of changedKeys.value) {
      originalSettings.value[k] = formSettings.value[k]
    }
    showToast('保存成功')
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false
  }
}

/* ── Skeleton helpers ── */
const skeletonItems = Array.from({ length: 5 }, (_, i) => i)

onMounted(() => fetchCategories())
</script>

<template>
  <div class="settings-page">

    <!-- Hero Header -->
    <div class="hero">
      <div class="hero-bg">
        <div class="hero-orb hero-orb--1" />
        <div class="hero-orb hero-orb--2" />
      </div>
      <div class="hero-content">
        <div class="hero-icon-wrap">
          <van-icon name="setting" size="22" color="#fff" />
        </div>
        <h1 class="hero-title">系统设置</h1>
        <p class="hero-subtitle">配置平台运行参数</p>
      </div>
    </div>

    <!-- Category Chips -->
    <div v-if="loadingCategories" class="chips-skeleton">
      <div v-for="i in 4" :key="i" class="chip-skeleton" :style="{ animationDelay: `${i * 0.06}s` }" />
    </div>
    <div v-else-if="categories.length" class="category-chips">
      <div
        v-for="cat in categories"
        :key="cat.key"
        class="category-chip"
        :class="{ 'category-chip--active': activeCategory === cat.key }"
        @click="selectCategory(cat.key)"
      >
        <van-icon v-if="cat.icon" :name="cat.icon" size="14" class="chip-icon" />
        <span>{{ cat.label }}</span>
      </div>
    </div>

    <!-- Settings Content -->
    <div class="settings-body">

      <!-- Loading Skeleton -->
      <div v-if="loadingSettings" class="settings-list">
        <div
          v-for="i in skeletonItems"
          :key="i"
          class="setting-card setting-card--skeleton"
          :style="{ animationDelay: `${i * 0.05}s` }"
        >
          <div class="skeleton-bar skeleton-bar--title" />
          <div class="skeleton-bar skeleton-bar--desc" />
          <div class="skeleton-bar skeleton-bar--input" />
        </div>
      </div>

      <!-- Settings Form -->
      <div v-else-if="settings.length" class="settings-list">

        <div
          v-for="(item, idx) in settings"
          :key="item.key"
          class="setting-card"
          :style="{ animationDelay: `${Math.min(idx, 10) * 0.04}s` }"
        >
          <!-- Label & Description -->
          <div class="setting-header">
            <span class="setting-label">{{ item.label }}</span>
            <span v-if="item.sensitive" class="setting-badge">敏感</span>
          </div>
          <p v-if="item.description && item.type !== 'select'" class="setting-desc">{{ item.description }}</p>

          <!-- Text -->
          <div v-if="item.type === 'text'" class="setting-control">
            <van-field
              v-model="formSettings[item.key]"
              :type="item.sensitive && !revealMap[item.key] ? 'password' : 'text'"
              :placeholder="`请输入${item.label}`"
              class="setting-input"
            >
              <template v-if="item.sensitive" #right-icon>
                <van-icon
                  :name="revealMap[item.key] ? 'eye-o' : 'closed-eye'"
                  size="18"
                  color="#94a3b8"
                  @click="toggleReveal(item.key)"
                />
              </template>
            </van-field>
          </div>

          <!-- Number -->
          <div v-else-if="item.type === 'number'" class="setting-control">
            <van-field
              v-model="formSettings[item.key]"
              type="digit"
              :placeholder="`请输入${item.label}`"
              class="setting-input"
            />
          </div>

          <!-- Boolean -->
          <div v-else-if="item.type === 'boolean'" class="setting-control setting-control--switch">
            <van-switch
              :model-value="!!formSettings[item.key]"
              size="20px"
              active-color="#0d9488"
              @update:model-value="formSettings[item.key] = $event"
            />
          </div>

          <!-- Select -->
          <div v-else-if="item.type === 'select'" class="setting-control">
            <van-field
              :model-value="formSettings[item.key]"
              is-link
              readonly
              :placeholder="`请选择${item.label}`"
              class="setting-input"
              @click="openPicker(item)"
            />
          </div>

          <!-- Textarea -->
          <div v-else-if="item.type === 'textarea'" class="setting-control">
            <van-field
              v-model="formSettings[item.key]"
              type="textarea"
              rows="3"
              :placeholder="`请输入${item.label}`"
              class="setting-input setting-input--textarea"
              autosize
            />
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="empty-state">
        <van-icon name="setting-o" size="36" color="#cbd5e1" />
        <span>请选择一个分类查看设置项</span>
      </div>
    </div>

    <!-- Floating Save Button -->
    <transition name="slide-up">
      <div v-if="hasChanges && !loadingSettings" class="save-bar">
        <div class="save-bar-inner">
          <span class="save-bar-info">{{ changedKeys.length }} 项已修改</span>
          <van-button
            type="primary"
            round
            size="small"
            :loading="saving"
            loading-text="保存中..."
            class="save-btn"
            @click="handleSave"
          >
            保存设置
          </van-button>
        </div>
      </div>
    </transition>

    <!-- Picker Popup -->
    <van-popup v-model:show="showPicker" position="bottom" round>
      <van-picker
        :columns="pickerColumns"
        :title="`选择${pickerTargetLabel}`"
        @confirm="onPickerConfirm"
        @cancel="showPicker = false"
      />
    </van-popup>

  </div>
</template>

<style scoped>
.settings-page {
  min-height: 100vh;
  background: var(--ta-bg-page, #f8fafc);
  padding-bottom: calc(80px + env(safe-area-inset-bottom, 0px));
}

/* ═══ Hero ═══ */
.hero {
  position: relative;
  overflow: hidden;
  padding-bottom: 20px;
}

.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(160deg, #475569 0%, #64748b 40%, #94a3b8 70%, #475569 100%);
  border-radius: 0 0 28px 28px;
}

.hero-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  pointer-events: none;
}
.hero-orb--1 {
  width: 200px;
  height: 200px;
  background: rgba(148, 163, 184, 0.3);
  top: -40px;
  right: -30px;
}
.hero-orb--2 {
  width: 160px;
  height: 160px;
  background: rgba(100, 116, 139, 0.25);
  bottom: 10px;
  left: -20px;
}

.hero-content {
  position: relative;
  z-index: 1;
  padding: 24px 20px 0;
  display: flex;
  align-items: center;
  gap: 14px;
  animation: fadeSlideUp 0.5s both;
}

.hero-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.hero-title {
  font-size: 22px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
  margin: 0;
}

.hero-subtitle {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
  margin: 2px 0 0;
}

/* ═══ Category Chips ═══ */
.category-chips {
  display: flex;
  gap: 8px;
  padding: 14px 16px 6px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  -ms-overflow-style: none;
  animation: fadeSlideUp 0.45s 0.08s both;
}
.category-chips::-webkit-scrollbar {
  display: none;
}

.category-chip {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 14px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 600;
  background: var(--ta-bg-card, #fff);
  color: var(--ta-text-secondary, #475569);
  border: 1.5px solid #e2e8f0;
  cursor: pointer;
  transition: all 0.2s;
  -webkit-tap-highlight-color: transparent;
}
.category-chip:active {
  transform: scale(0.96);
}
.category-chip--active {
  background: #0d9488;
  color: #fff;
  border-color: #0d9488;
}
.chip-icon {
  position: relative;
  top: -0.5px;
}

/* Chips skeleton */
.chips-skeleton {
  display: flex;
  gap: 8px;
  padding: 14px 16px 6px;
  animation: fadeSlideUp 0.45s 0.08s both;
}
.chip-skeleton {
  width: 72px;
  height: 32px;
  border-radius: 20px;
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  flex-shrink: 0;
}

/* ═══ Settings Body ═══ */
.settings-body {
  padding: 8px 12px 0;
}

.settings-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* ═══ Setting Card ═══ */
.setting-card {
  background: var(--ta-bg-card, #fff);
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03), 0 2px 8px rgba(0, 0, 0, 0.04);
  animation: cardIn 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.setting-card--skeleton {
  pointer-events: none;
}

.setting-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.setting-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary, #1e293b);
}

.setting-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  background: #fef3c7;
  color: #d97706;
  flex-shrink: 0;
}

.setting-desc {
  font-size: 12px;
  color: var(--ta-text-tertiary, #94a3b8);
  margin: 0 0 8px;
  line-height: 1.5;
}

.setting-control {
  margin-top: 4px;
}

.setting-control--switch {
  display: flex;
  justify-content: flex-end;
  padding-top: 2px;
}

.setting-input {
  --van-field-input-text-color: #0f172a;
  --van-field-placeholder-text-color: #94a3b8;
}

.setting-input--textarea :deep(.van-field__control) {
  min-height: 72px;
}

/* ═══ Skeleton Bars ═══ */
.skeleton-bar {
  border-radius: 6px;
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}
.skeleton-bar--title {
  width: 40%;
  height: 14px;
  margin-bottom: 8px;
}
.skeleton-bar--desc {
  width: 70%;
  height: 10px;
  margin-bottom: 10px;
}
.skeleton-bar--input {
  width: 100%;
  height: 36px;
  background: #f8fafc;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* ═══ Empty State ═══ */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 56px 0 24px;
  color: var(--ta-text-tertiary, #94a3b8);
  font-size: 13px;
}

/* ═══ Save Bar ═══ */
.save-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 100;
  padding: 0 16px calc(8px + env(safe-area-inset-bottom, 0px));
  pointer-events: none;
}

.save-bar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--ta-bg-card, #fff);
  border-radius: 16px;
  padding: 10px 16px;
  box-shadow:
    0 -2px 12px rgba(0, 0, 0, 0.06),
    0 4px 16px rgba(0, 0, 0, 0.08);
  pointer-events: auto;
}

.save-bar-info {
  font-size: 13px;
  font-weight: 500;
  color: var(--ta-text-secondary, #475569);
}

.save-btn {
  --van-button-primary-background: #0d9488;
  --van-button-primary-border-color: #0d9488;
  min-width: 96px;
}

/* Save bar transition */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.25s;
}
.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

/* ═══ Animations ═══ */
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

@keyframes cardIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
