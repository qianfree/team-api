<template>
  <div class="landing-page">
    <!-- Background Decoration (system mesh + orbs + grid) -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"></div>
      <div class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"></div>
      <div class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"></div>
      <div class="absolute inset-0"
           style="background-image: linear-gradient(rgba(123,143,245,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(123,143,245,0.04) 1px, transparent 1px); background-size: 64px 64px"></div>
    </div>

    <!-- Navigation -->
    <nav aria-label="主导航" class="landing-nav">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between h-16">
          <div class="flex items-center gap-2.5">
            <img src="/favicon.png" :alt="siteName" class="h-8 w-8 rounded-lg" />
            <span class="text-lg font-bold tracking-tight text-gray-900">{{ siteName }}</span>
          </div>
          <div class="flex items-center gap-2">
            <button v-if="announcements.length" @click="openAnnouncements" class="btn-nav-announce" title="查看公告">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0"/></svg>
            </button>
            <a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer"
               class="hidden sm:flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-900 transition-colors duration-200">
              <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
              <span>GitHub</span>
            </a>
          </div>
        </div>
      </div>
    </nav>

    <main class="landing-main">
      <!-- ============ 合并区：产品标题 + 场景展示 ============ -->
      <section aria-label="产品介绍" class="merged-section">
        <div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 w-full">
          <h1 class="hero-title">
            一个 Key，<span class="hero-title-accent">接入所有大模型</span>
          </h1>

          <!-- Scenario Tabs -->
          <div class="scenario-tabs" ref="tabsRef">
            <div class="scenario-tab-slider"
                 :class="{ 'is-ready': sliderReady }"
                 :style="{ transform: `translateX(${sliderLeft}px)`, width: sliderWidth + 'px' }"></div>
            <button v-for="(s, i) in scenarios" :key="s.title"
                    class="scenario-tab"
                    :class="{ 'scenario-tab-active': activeScenario === i }"
                    @click="setScenario(i)">
              <Icon :name="s.icon" size="sm" />
              <span>{{ s.tabLabel }}</span>
            </button>
          </div>

          <!-- Scenario Content -->
          <div class="scenario-content">
            <Transition :name="slideDir">
              <div :key="activeScenario" class="scenario-panel">
                <div class="scenario-info">
                  <h3 class="scenario-title">{{ scenarios[activeScenario].title }}</h3>
                  <p class="scenario-desc">{{ scenarios[activeScenario].desc }}</p>
                  <ul class="scenario-points">
                    <li v-for="pt in scenarios[activeScenario].points" :key="pt">
                      <Icon name="check" size="xs" class="text-primary-500 flex-shrink-0 mt-0.5" />
                      <span>{{ pt }}</span>
                    </li>
                  </ul>
                  <router-link :to="{ name: 'TenantLogin' }" class="btn btn-primary btn-compact mt-6">
                    开始使用
                  </router-link>
                </div>
                <div class="scenario-visual">
                  <!-- Team: member usage dashboard mock -->
                  <div v-if="activeScenario === 0" class="mock-card">
                    <div class="mock-header">
                      <span class="mock-header-title">成员用量概览</span>
                      <span class="mock-header-badge">本月</span>
                    </div>
                    <div class="mock-member-row" v-for="(m, mi) in mockMembers" :key="mi">
                      <div class="mock-member-avatar" :style="{ backgroundColor: m.color }"></div>
                      <div class="mock-member-info">
                        <span class="mock-member-name">{{ m.name }}</span>
                        <span class="mock-member-role">{{ m.role }}</span>
                      </div>
                      <div class="mock-usage-bar-bg">
                        <div class="mock-usage-bar-fill" :style="{ width: m.usage + '%', backgroundColor: m.usage > 85 ? '#ef4444' : '#14b8a6' }"></div>
                      </div>
                      <span class="mock-usage-pct">{{ m.usage }}%</span>
                    </div>
                    <div class="mock-footer">
                      <span>额度总计</span>
                      <span class="mock-footer-value">$128.50 / $200.00</span>
                    </div>
                  </div>

                  <!-- Developer: code example -->
                  <div v-else-if="activeScenario === 1" class="code-card">
                    <div class="code-card-bar">
                      <div class="terminal-dots">
                        <span class="terminal-dot terminal-dot-red"></span>
                        <span class="terminal-dot terminal-dot-yellow"></span>
                        <span class="terminal-dot terminal-dot-green"></span>
                      </div>
                      <span class="code-card-title">quickstart.py</span>
                    </div>
                    <pre class="code-card-body"><code><span class="tk-keyword">from</span> <span class="tk-module">openai</span> <span class="tk-keyword">import</span> <span class="tk-module">OpenAI</span>

<span class="tk-var">client</span> <span class="tk-op">=</span> <span class="tk-module">OpenAI</span><span class="tk-paren">(</span>
  <span class="tk-param">base_url</span><span class="tk-op">=</span><span class="tk-string">"{{ apiBaseUrl }}"</span><span class="tk-comma">,</span>
  <span class="tk-param">api_key</span><span class="tk-op">=</span><span class="tk-string">"sk-xxx"</span>
<span class="tk-paren">)</span>

<span class="tk-comment"># 像调用 OpenAI 一样调用任意模型</span>
<span class="tk-var">resp</span> <span class="tk-op">=</span> <span class="tk-var">client</span><span class="tk-op">.</span><span class="tk-method">chat</span><span class="tk-op">.</span><span class="tk-method">completions</span><span class="tk-op">.</span><span class="tk-method">create</span><span class="tk-paren">(</span>
  <span class="tk-param">model</span><span class="tk-op">=</span><span class="tk-string">"gpt-4o"</span><span class="tk-comma">,</span>
  <span class="tk-param">messages</span><span class="tk-op">=</span><span class="tk-bracket">[{</span><span class="tk-string">"role"</span><span class="tk-op">:</span> <span class="tk-string">"user"</span><span class="tk-comma">,</span> <span class="tk-string">"content"</span><span class="tk-op">:</span> <span class="tk-string">"Hello!"</span><span class="tk-bracket">}]</span>
<span class="tk-paren">)</span></code></pre>
                  </div>

                  <!-- Ops: monitoring mock -->
                  <div v-else-if="activeScenario === 2" class="mock-card">
                    <div class="mock-header">
                      <span class="mock-header-title">渠道健康监控</span>
                      <span class="mock-header-badge-green">全部正常</span>
                    </div>
                    <div class="mock-metric-row">
                      <div class="mock-metric">
                        <span class="mock-metric-value">99.97%</span>
                        <span class="mock-metric-label">请求成功率</span>
                      </div>
                      <div class="mock-metric">
                        <span class="mock-metric-value">23ms</span>
                        <span class="mock-metric-label">平均延迟</span>
                      </div>
                      <div class="mock-metric">
                        <span class="mock-metric-value">1.2M</span>
                        <span class="mock-metric-label">今日请求</span>
                      </div>
                    </div>
                    <div class="mock-chart">
                      <div v-for="(h, hi) in mockChartHeights" :key="hi" class="mock-chart-bar"
                           :style="{ height: h + '%' }"></div>
                    </div>
                    <div class="mock-alert-row">
                      <Icon name="bell" size="xs" class="text-amber-500" />
                      <span class="mock-alert-text">渠道 A 延迟升高 &gt; 200ms · 已自动切换至渠道 B</span>
                    </div>
                  </div>
                </div>
              </div>
            </Transition>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer aria-label="页脚" class="landing-footer">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-5">
        <div class="flex flex-col sm:flex-row items-center justify-between gap-3">
          <span class="text-xs text-gray-400">&copy; 2026 qianfree. Released under AGPL-3.0.</span>
          <a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer"
             class="text-xs text-gray-400 hover:text-gray-600 transition-colors flex items-center gap-1.5">
            Powered by <span class="font-medium text-gray-500">Team-API</span>
          </a>
        </div>
      </div>
    </footer>

    <!-- Announcement Popup Modal -->
    <Teleport to="body">
      <Transition name="announce-modal">
        <div v-if="announcementVisible && announcementItem" class="announce-overlay" @click.self="closeAnnouncement">
          <div class="announce-backdrop"></div>
          <div class="announce-card">
            <!-- Header: icon + title left, close right -->
            <div class="announce-header">
              <div class="announce-header-left">
                <div class="announce-icon-wrap" :class="announcementItem.type === 'important' ? 'announce-icon-warn' : 'announce-icon-info'">
                  <svg v-if="announcementItem.type === 'important'" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/></svg>
                  <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.34 15.84c-.688-.06-1.386-.09-2.09-.09H7.5a4.5 4.5 0 110-9h.75c.704 0 1.402-.03 2.09-.09m0 9.18c.253.962.584 1.892.985 2.783.247.55.06 1.21-.463 1.511l-.657.38c-.551.318-1.26.117-1.527-.461a20.845 20.845 0 01-1.44-4.282m3.102.069a18.03 18.03 0 01-.59-4.59c0-1.586.205-3.124.59-4.59m0 9.18a23.848 23.848 0 018.835 2.535M10.34 6.66a23.847 23.847 0 008.835-2.535m0 0A23.74 23.74 0 0018.795 3m.38 1.125a23.91 23.91 0 011.014 5.395m-1.014 8.855c-.118.38-.245.754-.38 1.125m.38-1.125a23.91 23.91 0 001.014-5.395m0-3.46c.495.413.811 1.035.811 1.73 0 .695-.316 1.317-.811 1.73m0-3.46a24.347 24.347 0 010 3.46"/></svg>
                </div>
                <h2 class="announce-title">{{ announcementItem.title }}</h2>
              </div>
              <button class="announce-close" @click="closeAnnouncement" title="关闭" aria-label="关闭公告">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M12 4L4 12M4 4l8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
              </button>
            </div>

            <!-- Content area -->
            <div class="announce-body">
              <!-- Markdown content -->
              <div class="announce-content" v-html="renderMarkdown(announcementItem.content)"></div>
            </div>

            <!-- Footer -->
            <div class="announce-footer">
              <div class="announce-footer-left">
                <svg class="announce-footer-clock" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>
                <span class="announce-date">{{ announcementItem.created_at }}</span>
              </div>
              <div v-if="announcements.length > 1" class="announce-pagination">
                <button class="announce-page-btn" :disabled="announceIdx === 0" @click="prevAnnouncement">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 19l-7-7 7-7"/></svg>
                </button>
                <span class="announce-page-indicator">{{ announceIdx + 1 }} / {{ announcements.length }}</span>
                <button class="announce-page-btn" :disabled="announceIdx === announcements.length - 1" @click="nextAnnouncement">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 5l7 7-7 7"/></svg>
                </button>
              </div>
              <button class="announce-btn" @click="closeAnnouncement">
                <span>我知道了</span>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useSeo } from '@/composables/useSeo'
import { useHead } from '@unhead/vue'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { marked } from 'marked'
import request from '@/utils/request'

const { settings: publicSettings, fetchSettings } = usePublicSettings()
const siteName = computed(() => publicSettings.value.site_name || 'Team-API')
// 快速开始示例代码中展示的真实接入地址（取当前浏览器访问域名，OpenAI 兼容端点固定为 /v1）
const apiBaseUrl = `${window.location.origin}/v1`

// Announcement popup
const DISMISSED_KEY = 'tenant_dismissed_landing_announcements'
const announcementVisible = ref(false)
const announcementItem = ref<any>(null)
const announcements = ref<any[]>([])
const announceIdx = ref(0)

function loadDismissedIds(): Set<number> {
  try {
    const raw = localStorage.getItem(DISMISSED_KEY)
    if (raw) return new Set(JSON.parse(raw))
  } catch { /* ignore */ }
  return new Set()
}

function saveDismissedIds(ids: Set<number>) {
  try {
    localStorage.setItem(DISMISSED_KEY, JSON.stringify([...ids]))
  } catch { /* ignore */ }
}

function sortAnnouncements(list: any[]) {
  return [...list].sort((a, b) => {
    if (a.is_pinned && !b.is_pinned) return -1
    if (!a.is_pinned && b.is_pinned) return 1
    return 0
  })
}

function showAnnouncementModal(item: any) {
  announcementItem.value = item
  announcementVisible.value = true
}

function closeAnnouncement() {
  if (!announcementItem.value) return
  const dismissed = loadDismissedIds()
  dismissed.add(announcementItem.value.id)
  saveDismissedIds(dismissed)
  announcementVisible.value = false
  setTimeout(() => { announcementItem.value = null }, 300)
}

function prevAnnouncement() {
  if (announceIdx.value > 0) {
    announceIdx.value--
    announcementItem.value = announcements.value[announceIdx.value]
  }
}

function nextAnnouncement() {
  if (announceIdx.value < announcements.value.length - 1) {
    announceIdx.value++
    announcementItem.value = announcements.value[announceIdx.value]
  }
}

function openAnnouncements() {
  if (announcements.value.length) {
    announceIdx.value = 0
    showAnnouncementModal(announcements.value[0])
  }
}

async function fetchAndShowAnnouncement() {
  try {
    const res = await request.get('/settings/announcements', { params: { position: 'login' }, _suppressErrorMsg: true } as any)
    const list: any[] = res.data?.data?.list || []
    if (!list.length) return
    const sorted = sortAnnouncements(list)
    announcements.value = sorted
    const dismissed = loadDismissedIds()
    const latest = sorted.find(a => !dismissed.has(a.id))
    if (latest) {
      announceIdx.value = sorted.indexOf(latest)
      await nextTick()
      showAnnouncementModal(latest)
    }
  } catch { /* silently ignore */ }
}

function renderMarkdown(text: string): string {
  return marked.parse(text) as string
}

onMounted(async () => {
  await fetchSettings()
  fetchAndShowAnnouncement()
  // 首屏测量滑块位置；延迟一帧再开启过渡，避免首屏滑块从 0 展开的动画
  nextTick(() => {
    updateSlider()
    requestAnimationFrame(() => { sliderReady.value = true })
  })
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
})

// Scenario tabs — 顶部滑块指示器 + 左右滑动切换
const activeScenario = ref(0)
const slideDir = ref<'slide-next' | 'slide-prev'>('slide-next')

// 顶部滑块：绝对定位的高亮背景，跟随激活 Tab 平滑左右移动
const tabsRef = ref<HTMLElement>()
const sliderLeft = ref(0)
const sliderWidth = ref(0)
const sliderReady = ref(false)

function updateSlider() {
  const container = tabsRef.value
  if (!container) return
  const tabs = container.querySelectorAll<HTMLElement>('.scenario-tab')
  const el = tabs[activeScenario.value]
  if (el) {
    sliderLeft.value = el.offsetLeft
    sliderWidth.value = el.offsetWidth
  }
}

function setScenario(i: number) {
  if (i === activeScenario.value) return
  slideDir.value = i > activeScenario.value ? 'slide-next' : 'slide-prev'
  activeScenario.value = i
  updateSlider()
}

function onResize() {
  updateSlider()
}

// SEO
useSeo({
  title: `${siteName.value} — 一个 API 接入所有大模型 | 企业级多租户 AI 网关`,
  description: publicSettings.value.site_description || `${siteName.value} 是开源自托管的企业级多租户大模型 API 网关平台。聚合 OpenAI、Claude、Gemini、DeepSeek 等 40+ 供应商，内置计费引擎、团队管理、用量审计与智能渠道调度。完全兼容 OpenAI SDK，只需修改 base_url 即可接入。`,
  siteName: siteName.value,
  keywords: 'Team-API, 大模型网关, API Gateway, 多租户, OpenAI, Claude, Gemini, DeepSeek, 阿里云百炼, 百度文心, 腾讯混元, 智谱AI, AI代理, LLM Gateway, 开源, 自托管, 计费引擎, 团队管理, API管理, SSE流式, 渠道调度',
  canonicalUrl: 'https://team-api.net/',
})

useHead({
  script: [
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: siteName.value,
        description: publicSettings.value.site_description || '企业级多租户大模型 API 网关平台，聚合 40+ 大模型供应商，内置计费引擎、团队管理、用量审计与渠道调度。',
        url: 'https://github.com/qianfree/team-api',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Linux, macOS, Windows',
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: '开源免费，AGPL-3.0 协议' },
        featureList: [
          '多租户与团队协作（RBAC 权限）',
          '统一 AI 代理层（40+ 供应商适配器）',
          '精细化计费引擎（预扣→结算→退款）',
          '智能渠道调度（优先级/权重/健康评分）',
          '安全与权限（AES-256 加密、全链路审计）',
          '开发者友好（兼容 OpenAI SDK）',
        ],
        programmingLanguage: 'Go, Vue, TypeScript',
        license: 'https://opensource.org/licenses/AGPL-3.0',
      }),
    },
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'Organization',
        name: siteName.value,
        url: 'https://github.com/qianfree/team-api',
        logo: 'https://team-api.net/favicon.png',
        sameAs: ['https://github.com/qianfree/team-api'],
      }),
    },
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'WebSite',
        name: siteName.value,
        url: 'https://team-api.net/',
        description: publicSettings.value.site_description || '企业级多租户大模型 API 网关平台',
        potentialAction: {
          '@type': 'SearchAction',
          target: 'https://github.com/qianfree/team-api/search?q={search_term_string}',
          'query-input': 'required name=search_term_string',
        },
      }),
    },
  ],
})

// ============ Data ============

const scenarios = [
  {
    icon: 'users',
    tabLabel: '团队管理',
    title: '额度、权限、用量，一目了然',
    desc: '组织管理员统一充值，按项目和成员分配额度。每个成员的用量实时可查，预算超限自动熔断，杜绝账单意外。',
    points: [
      '五层额度管控：租户→套餐→成员→项目→Key',
      '成员用量排行与明细，实时可查',
      'RBAC 权限：Owner / Admin / Member 三级',
      '预算超限自动熔断，并发预扣防超额',
    ],
  },
  {
    icon: 'terminal',
    tabLabel: '开发者接入',
    title: '快速接入，零改动迁移',
    desc: '如果你已经在用 OpenAI SDK，接入 Team-API 只需要改一行 base_url。协议自动转换、流式透传、错误格式兼容，你的代码一行都不用动。',
    points: [
      '完全兼容 OpenAI Python / Node.js SDK',
      '支持 SSE 流式转发与中断恢复',
      'Function Call、多模态、Embedding 全支持',
      '请求级超时控制，自动重试与降级',
    ],
  },
  {
    icon: 'chart',
    tabLabel: '运维监控',
    title: '渠道健康、请求延迟、告警通知',
    desc: '实时监控渠道成功率和延迟，自动摘除异常渠道。支持自定义告警规则，异常事件第一时间通知到人。',
    points: [
      '渠道健康评分与自动故障切换',
      '请求成功率、延迟、Token 用量实时看板',
      '自定义告警规则：延迟阈值、错误率、额度',
      'Webhook / 邮件多通道告警通知',
    ],
  },
]

// Mock data for scenario visuals
const mockMembers = [
  { name: '张三', role: 'Owner', usage: 72, color: '#14b8a6' },
  { name: '李四', role: 'Admin', usage: 45, color: '#6366f1' },
  { name: '王五', role: 'Member', usage: 91, color: '#f59e0b' },
  { name: '赵六', role: 'Member', usage: 33, color: '#ec4899' },
]

const mockChartHeights = [45, 62, 38, 71, 55, 82, 67, 48, 73, 58, 90, 65, 52, 78, 60, 42, 85, 70, 55, 68]

</script>

<style scoped>
/* ================================================
   页面布局 — 单页全屏，无滚动
   ================================================ */
.landing-page {
  position: relative;
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background:
    radial-gradient(at 12% 8%, rgba(20, 184, 166, 0.10) 0px, transparent 36%),
    radial-gradient(at 72% 0%, rgba(6, 182, 212, 0.08) 0px, transparent 40%),
    radial-gradient(at 95% 70%, rgba(20, 184, 166, 0.06) 0px, transparent 38%),
    radial-gradient(at 45% 100%, rgba(99, 102, 241, 0.04) 0px, transparent 42%),
    #f6f8fc;
}
.landing-main {
  position: relative;
  z-index: 1;
  flex: 1;
  min-height: 0;
  display: flex;
}
.landing-footer {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  background: transparent;
}

/* ================================================
   NAV — 透明导航栏
   ================================================ */
.landing-nav {
  position: relative; z-index: 50; flex-shrink: 0;
}

/* Nav announcement bell — 浅色玻璃按钮 */
.btn-nav-announce {
  width: 36px; height: 36px; border-radius: 10px; border: 1px solid rgba(255,255,255,0.72);
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: all 0.2s ease;
  background: rgba(255,255,255,0.55); color: #64748b;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.7);
}
.btn-nav-announce:hover {
  background: rgba(255,255,255,0.9); color: #0d9488;
  box-shadow: 0 4px 14px rgba(76,91,142,0.1), inset 0 1px 0 rgba(255,255,255,0.9);
}

/* ================================================
   合并区 — 产品标题 + 场景展示
   ================================================ */
.merged-section {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem 0 1.75rem;
}

.hero-title {
  font-size: clamp(2rem, 4vw, 3rem); font-weight: 800;
  line-height: 1.15; letter-spacing: -0.03em; color: #111827;
  text-align: center; margin-bottom: 2.25rem;
  animation: fadeInUp 0.6s ease-out 0.1s both;
}
.hero-title-accent {
  background: linear-gradient(135deg, #0d9488, #14b8a6, #06b6d4);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
}

@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(16px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ================================================
   场景展示 — SCENARIOS
   ================================================ */
.scenario-tabs {
  position: relative; /* 滑块定位锚点 */
  display: flex; gap: 4px; padding: 4px;
  justify-content: center; width: fit-content; margin: 0 auto 1.5rem;
  border-radius: 14px;
  background: rgba(241, 245, 249, 0.8);
  backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}
/* 跟随激活 Tab 平滑移动的滑块（白底高亮背景） */
.scenario-tab-slider {
  position: absolute;
  top: 4px; left: 0;
  height: calc(100% - 8px);
  border-radius: 10px;
  background: #fff;
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 2px 8px rgba(76, 91, 142, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.9);
  z-index: 0;
  pointer-events: none;
}
/* 首屏测量完成后再启用过渡，避免滑块从宽度 0 展开的入场动画 */
.scenario-tab-slider.is-ready {
  transition: transform 0.42s cubic-bezier(0.22, 1, 0.36, 1),
              width 0.42s cubic-bezier(0.22, 1, 0.36, 1);
}
.scenario-tab {
  position: relative; z-index: 1;
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 18px; border-radius: 10px; font-size: 14px; font-weight: 500;
  color: #64748b; border: 1px solid transparent;
  cursor: pointer; transition: color 0.2s ease;
}
.scenario-tab:hover { color: #111827; }
.scenario-tab-active { color: #0f766e; }

.scenario-content {
  position: relative;
  overflow: hidden;
  /* 固定高度：三场景内容区统一高度，切换时不再因面板内容高度差异而上下跳动 */
  height: 440px;
  /* 圆角与内部 scenario-panel 对齐，避免面板阴影被裁剪成直角 */
  border-radius: 1.5rem;
}
.scenario-panel {
  display: grid; grid-template-columns: 1fr 1.2fr; gap: 1.75rem; align-items: stretch;
  height: 440px;
  border-radius: 1.5rem; padding: 2rem 2.25rem;
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(28px) saturate(1.18); -webkit-backdrop-filter: blur(28px) saturate(1.18);
  border: 1px solid rgba(255, 255, 255, 0.82);
  box-shadow: 0 18px 55px rgba(76, 91, 142, 0.11), inset 0 1px 0 rgba(255, 255, 255, 0.9);
}
/* 左侧文案垂直居中、右侧视觉区让卡片撑满，保证三场景视觉高度一致 */
.scenario-info {
  display: flex; flex-direction: column; justify-content: center; min-width: 0;
}
.scenario-visual {
  display: flex; min-width: 0;
}

/* 场景面板左右滑动切换（与容器高度过渡、滑块移动协同） */
.slide-next-enter-active,
.slide-next-leave-active,
.slide-prev-enter-active,
.slide-prev-leave-active {
  transition: transform 0.42s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.32s ease;
}
/* 过渡期间旧面板脱离文档流，与新面板交叠滑动 */
.slide-next-leave-active,
.slide-prev-leave-active {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
}
/* 向后切换（点击右侧 Tab）：新内容右滑入、旧内容左滑出 */
.slide-next-enter-from { transform: translateX(32px); opacity: 0; }
.slide-next-leave-to   { transform: translateX(-32px); opacity: 0; }
/* 向前切换（点击左侧 Tab）：新内容左滑入、旧内容右滑出 */
.slide-prev-enter-from { transform: translateX(-32px); opacity: 0; }
.slide-prev-leave-to   { transform: translateX(32px); opacity: 0; }
.scenario-title { font-size: 22px; font-weight: 700; color: #111827; margin-bottom: 10px; letter-spacing: -0.01em; }
.scenario-desc { font-size: 14px; color: #64748b; line-height: 1.65; margin-bottom: 14px; }
.scenario-points { display: flex; flex-direction: column; gap: 10px; }
.scenario-points li {
  display: flex; align-items: flex-start; gap: 8px;
  font-size: 13px; color: #475569; line-height: 1.5;
}

/* Compact button — 紧凑按钮，不占满容器宽度 */
.btn-compact {
  align-self: flex-start;
}

/* Code card — 深色代码块（对齐系统 code-block） */
.code-card {
  border-radius: 1rem; overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.4);
  background: #0f172a;
  display: flex; flex-direction: column; flex: 1; height: 100%;
}
.code-card-bar {
  display: flex; align-items: center; padding: 10px 16px;
  background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.06);
}
.terminal-dots { display: flex; gap: 6px; }
.terminal-dot { width: 10px; height: 10px; border-radius: 9999px; }
.terminal-dot-red { background: #ef4444; }
.terminal-dot-yellow { background: #eab308; }
.terminal-dot-green { background: #22c55e; }
.code-card-title {
  margin-left: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px; color: #94a3b8;
}
.code-card-body {
  padding: 18px 20px; background: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px; line-height: 1.8; overflow-x: auto;
  flex: 1; min-height: 0;
}

/* Code tokens */
.tk-keyword { color: #c084fc; }
.tk-module { color: #67e8f9; }
.tk-var { color: #e2e8f0; }
.tk-op { color: #94a3b8; }
.tk-param { color: #fbbf24; }
.tk-string { color: #6ee7b7; }
.tk-method { color: #67e8f9; }
.tk-paren { color: #94a3b8; }
.tk-bracket { color: #94a3b8; }
.tk-comma { color: #94a3b8; }
.tk-comment { color: rgba(255,255,255,0.25); }

/* Mock card for team & ops — 浅色卡片 */
.mock-card {
  border-radius: 1rem; overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 14px 34px rgba(76, 91, 142, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.95);
  background: #fff;
  display: flex; flex-direction: column; flex: 1; height: 100%;
}
.mock-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 18px; border-bottom: 1px solid #f1f5f9; background: #fafbfd;
}
.mock-header-title { font-size: 13px; font-weight: 600; color: #374151; }
.mock-header-badge {
  font-size: 11px; font-weight: 600; padding: 2px 10px; border-radius: 9999px;
  background: #f0fdfa; color: #0d9488;
}
.mock-header-badge-green {
  font-size: 11px; font-weight: 600; padding: 2px 10px; border-radius: 9999px;
  background: #f0fdf4; color: #16a34a;
}

.mock-member-row {
  display: flex; align-items: center; gap: 10px; padding: 9px 18px;
  border-bottom: 1px solid #f1f5f9;
  flex: 1;
}
.mock-member-avatar { width: 28px; height: 28px; border-radius: 8px; flex-shrink: 0; }
.mock-member-info { display: flex; flex-direction: column; min-width: 60px; }
.mock-member-name { font-size: 12px; font-weight: 600; color: #374151; }
.mock-member-role { font-size: 10px; color: #94a3b8; }
.mock-usage-bar-bg { flex: 1; height: 6px; border-radius: 9999px; background: #f1f5f9; overflow: hidden; }
.mock-usage-bar-fill { height: 100%; border-radius: 9999px; transition: width 0.6s ease; }
.mock-usage-pct { font-size: 11px; font-weight: 600; color: #94a3b8; width: 32px; text-align: right; }

.mock-footer {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 18px; background: #fafbfd; font-size: 12px; color: #94a3b8;
}
.mock-footer-value { font-weight: 600; color: #374151; }

/* Ops metrics */
.mock-metric-row {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px;
  padding: 14px 18px;
}
.mock-metric { text-align: center; }
.mock-metric-value { display: block; font-size: 20px; font-weight: 700; color: #111827; }
.mock-metric-label { display: block; font-size: 11px; color: #94a3b8; margin-top: 2px; }

.mock-chart {
  display: flex; align-items: flex-end; gap: 4px;
  padding: 0 18px 14px;
  flex: 1; min-height: 60px;
}
.mock-chart-bar {
  flex: 1; border-radius: 3px 3px 0 0;
  background: linear-gradient(to top, #14b8a6, #06b6d4); opacity: 0.6;
  min-height: 4px;
}
.mock-chart-bar:nth-child(odd) { opacity: 0.8; }

.mock-alert-row {
  display: flex; align-items: center; gap: 8px;
  padding: 9px 18px; border-top: 1px solid #f1f5f9;
  background: #fffbeb;
}
.mock-alert-text { font-size: 12px; color: #92400e; }

@media (max-width: 768px) {
  /* 单列堆叠后内容高于视口：放开整页滚动，避免被 overflow:hidden 裁切 */
  .landing-page { height: auto; min-height: 100vh; overflow-y: auto; overflow-x: hidden; }
  .landing-main { flex: none; }
  .merged-section { align-items: flex-start; padding: 1rem 0 1.5rem; }
  /* 移动端版权信息在文档流底部，不固定在屏幕底部，避免与内容重叠 */
  .landing-footer {
    position: relative;
    padding-bottom: env(safe-area-inset-bottom);
  }

  /* 标题强制两行：accent 段落另起一行，字号略减、间距收紧 */
  .hero-title { font-size: 1.75rem; line-height: 1.25; margin-bottom: 1.25rem; }
  .hero-title-accent { display: block; }

  /* Tabs 占满宽度三等分，单行整齐排列 */
  .scenario-tabs { width: 100%; margin: 0 auto 1rem; }
  .scenario-tab { flex: 1 1 0; justify-content: center; padding: 8px 4px; font-size: 12.5px; gap: 4px; }

  /* 面板单列、高度自适应、padding 紧凑 */
  .scenario-content { height: auto; }
  .scenario-panel { grid-template-columns: 1fr; height: auto; gap: 1.25rem; padding: 1.25rem; }
  .scenario-info { justify-content: flex-start; }

  /* 卡片还原自然高度；code-card 限定父容器宽度，超长代码在内部横向自由滚动 */
  .code-card { height: auto; flex: none; width: 100%; min-width: 0; max-width: 100%; }
  .mock-card { height: auto; flex: none; width: 100%; min-width: 0; max-width: 100%; }
  .code-card-body { flex: none; min-width: 0; overflow-x: auto; }
  .mock-member-row { flex: none; }
  .mock-chart { flex: none; min-height: 60px; }
}

/* ================================================
   ANNOUNCEMENT POPUP — 系统浅色模态
   ================================================ */

/* Overlay */
.announce-overlay {
  position: fixed; inset: 0; z-index: 100;
  display: flex; align-items: center; justify-content: center;
  padding: 1.5rem;
}
.announce-backdrop {
  position: absolute; inset: 0;
  background: rgba(15, 23, 42, 0.35);
  backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
}

/* Card */
.announce-card {
  position: relative; width: 100%; max-width: 860px;
  border-radius: 1.5rem; overflow: hidden;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(28px) saturate(1.18); -webkit-backdrop-filter: blur(28px) saturate(1.18);
  border: 1px solid rgba(255, 255, 255, 0.85);
  box-shadow: 0 32px 80px -16px rgba(15, 23, 42, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.95);
  animation: announceEnter 0.4s cubic-bezier(0.16, 1, 0.3, 1) both;
}
@keyframes announceEnter {
  from { opacity: 0; transform: translateY(20px) scale(0.96); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}

/* Header */
.announce-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 20px 28px; gap: 16px;
  border-bottom: 1px solid #f1f5f9;
  position: relative; z-index: 1;
}
.announce-header-left {
  display: flex; align-items: center; gap: 12px; flex: 1; min-width: 0;
}

/* Icon */
.announce-icon-wrap {
  width: 36px; height: 36px; border-radius: 10px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
}
.announce-icon-info { background: #f0fdfa; color: #0d9488; }
.announce-icon-warn { background: #fffbeb; color: #b45309; }

/* Title */
.announce-title {
  font-size: 18px; font-weight: 700; color: #111827;
  line-height: 1.4; letter-spacing: -0.01em; margin: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

/* Close button */
.announce-close {
  width: 32px; height: 32px; border-radius: 8px; border: 1px solid transparent; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: rgba(0,0,0,0.04); color: #94a3b8;
  cursor: pointer; transition: all 0.2s ease;
}
.announce-close:hover {
  background: rgba(0,0,0,0.08); color: #334155;
}

/* Body */
.announce-body {
  position: relative; z-index: 1;
}

/* Content (markdown rendered) */
.announce-content {
  font-size: 15px; line-height: 1.8; color: #475569;
  padding: 20px 28px;
  max-height: 65vh; overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.4) transparent;
}
.announce-content::-webkit-scrollbar { width: 4px; }
.announce-content::-webkit-scrollbar-track { background: transparent; }
.announce-content::-webkit-scrollbar-thumb { background: rgba(148,163,184,0.4); border-radius: 4px; }
.announce-content h1, .announce-content h2, .announce-content h3 {
  color: #111827; font-weight: 600; margin: 1em 0 0.5em;
}
.announce-content h1:first-child, .announce-content h2:first-child, .announce-content h3:first-child,
.announce-content p:first-child { margin-top: 0; }
.announce-content h1 { font-size: 1.15em; }
.announce-content h2 { font-size: 1.08em; }
.announce-content h3 { font-size: 1em; }
.announce-content p { margin: 0.6em 0; }
.announce-content ul, .announce-content ol { padding-left: 1.4em; margin: 0.6em 0; }
.announce-content li { margin: 0.3em 0; }
.announce-content ul li::marker { color: #14b8a6; }
.announce-content a { color: #0d9488; text-decoration: none; border-bottom: 1px solid rgba(13,148,136,0.3); transition: border-color 0.15s ease; }
.announce-content a:hover { border-bottom-color: rgba(13,148,136,0.6); }
.announce-content blockquote {
  border-left: 2px solid rgba(20,184,166,0.4); padding: 0.6em 1em; margin: 0.8em 0;
  background: rgba(20,184,166,0.05); border-radius: 0 10px 10px 0; color: #64748b;
}
.announce-content code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.85em; background: #f1f5f9; padding: 0.15em 0.5em; border-radius: 6px;
  color: #0f766e;
}
.announce-content pre {
  background: #0f172a; color: #e2e8f0; padding: 16px 20px; border-radius: 12px;
  overflow-x: auto; margin: 0.8em 0; border: 1px solid rgba(15,23,42,0.1);
}
.announce-content pre code { background: transparent; padding: 0; color: inherit; border: none; }
.announce-content strong { color: #111827; font-weight: 600; }
.announce-content em { color: #475569; }
.announce-content img { max-width: 100%; border-radius: 12px; margin: 0.8em 0; border: 1px solid rgba(0,0,0,0.06); }
.announce-content hr { border: none; height: 1px; margin: 1.2em 0; background: linear-gradient(90deg, transparent, rgba(0,0,0,0.08), transparent); }
.announce-content table { width: 100%; border-collapse: collapse; margin: 0.8em 0; border-radius: 10px; overflow: hidden; }
.announce-content th { background: #f8fafc; padding: 10px 14px; text-align: left; font-size: 12px; font-weight: 600; color: #64748b; border: 1px solid #e2e8f0; }
.announce-content td { padding: 10px 14px; font-size: 13px; color: #475569; border: 1px solid #e2e8f0; }

/* Footer */
.announce-footer {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 28px; border-top: 1px solid #f1f5f9;
  position: relative; z-index: 1; background: rgba(248, 250, 252, 0.6);
}
.announce-footer-left {
  display: flex; align-items: center; gap: 6px;
}
.announce-footer-clock { color: #94a3b8; flex-shrink: 0; }
.announce-date { font-size: 12px; color: #94a3b8; font-variant-numeric: tabular-nums; }

.announce-btn {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 10px 24px; border-radius: 10px; border: none;
  font-size: 14px; font-weight: 600; cursor: pointer;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  color: #fff;
  box-shadow: 0 6px 20px rgba(20,184,166,0.3);
  transition: all 0.2s ease;
}
.announce-btn:hover {
  box-shadow: 0 10px 28px rgba(20,184,166,0.4);
  transform: translateY(-1px);
}
.announce-btn:active { transform: translateY(0); }

/* Pagination */
.announce-pagination { display: flex; align-items: center; gap: 4px; }
.announce-page-btn {
  width: 28px; height: 28px; border-radius: 6px; border: none;
  display: flex; align-items: center; justify-content: center;
  background: rgba(0,0,0,0.04); color: #64748b;
  cursor: pointer; transition: all 0.15s ease;
}
.announce-page-btn:hover:not(:disabled) { background: rgba(0,0,0,0.08); color: #334155; }
.announce-page-btn:disabled { opacity: 0.3; cursor: default; }
.announce-page-indicator {
  font-size: 12px; color: #94a3b8; min-width: 40px; text-align: center;
  font-variant-numeric: tabular-nums;
}

/* Transition */
.announce-modal-enter-active { transition: opacity 0.3s ease-out; }
.announce-modal-leave-active { transition: opacity 0.2s ease-in; }
.announce-modal-enter-from, .announce-modal-leave-to { opacity: 0; }
.announce-modal-enter-active .announce-card { transition: transform 0.4s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.3s ease-out; }
.announce-modal-leave-active .announce-card { transition: transform 0.2s ease-in, opacity 0.2s ease-in; }
.announce-modal-leave-to .announce-card { transform: translateY(10px) scale(0.97); opacity: 0; }

/* Mobile */
@media (max-width: 480px) {
  .announce-card { border-radius: 16px; max-width: 100%; }
  .announce-header { padding: 16px 20px; }
  .announce-content { padding: 16px 20px; }
  .announce-footer { padding: 12px 20px; }
  .announce-title { font-size: 16px; }
  .announce-content { max-height: 45vh; font-size: 14px; }
}
</style>
