<template>
	<div class="landing-page">
		<!-- Navigation -->
		<nav aria-label="主导航" class="landing-nav" :class="{ 'nav-scrolled': isScrolled }">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="flex items-center justify-between h-16">
					<div class="flex items-center gap-2.5">
						<img src="/favicon.png" :alt="siteName" class="h-8 w-8 rounded-lg" />
						<span class="text-lg font-bold tracking-tight" :class="isScrolled ? 'text-gray-900' : 'text-white'">{{ siteName }}</span>
					</div>
					<div class="flex items-center gap-2">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer"
							class="hidden sm:flex items-center gap-1.5 text-sm transition-colors duration-200"
							:class="isScrolled ? 'text-gray-500 hover:text-gray-900' : 'text-white/60 hover:text-white'">
							<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							<span>GitHub</span>
						</a>
						<router-link :to="{ name: 'TenantLogin' }" class="btn-nav-cta text-sm font-medium px-4 py-2 rounded-lg">
							开始使用
						</router-link>
					</div>
				</div>
			</div>
		</nav>

		<main>
			<!-- ============ HERO ============ -->
			<section aria-label="产品介绍" class="hero-section">
				<div class="hero-bg">
					<div class="hero-grid"></div>
					<div class="hero-orb hero-orb-1"></div>
					<div class="hero-orb hero-orb-2"></div>
					<div class="hero-orb hero-orb-3"></div>
					<div class="hero-vignette"></div>
				</div>

				<div class="hero-inner">
					<div class="text-center max-w-3xl mx-auto">
						<h1 class="hero-title">
							一个 API，<span class="hero-title-accent">接入所有大模型</span>
						</h1>

						<p class="hero-subtitle">
							多租户团队管控、精细计费追溯、全链路审计日志、<br />开放平台，可与企业现有OA系统无缝集成。
						</p>

							<div class="flex items-center justify-center gap-3 mt-10">
								<router-link :to="{ name: 'TenantLogin' }" class="btn btn-primary btn-lg min-w-[160px]">
									<Icon name="arrowRight" size="sm" />
									快速开始
								</router-link>
							</div>
					</div>

					<!-- Provider Logo Wall -->
					<div class="hero-logo-wall">
						<div class="logo-wall-inner">
							<div v-for="p in heroProviders" :key="p.name" class="logo-wall-item">
								<div class="logo-wall-icon" :style="{ backgroundColor: p.bgColor, color: p.textColor }">
									{{ p.abbr }}
								</div>
								<span class="logo-wall-name">{{ p.name }}</span>
							</div>
						</div>
						<div class="logo-wall-fade-l"></div>
						<div class="logo-wall-fade-r"></div>
					</div>
				</div>
			</section>

			<!-- ============ 核心价值 ============ -->
			<section id="features" aria-label="核心价值" class="features-section">
				<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 sm:py-28">
					<div class="text-center max-w-2xl mx-auto mb-16">
						<span class="section-tag">核心价值</span>
						<h2 class="section-title">为团队而生的 AI 网关</h2>
						<p class="section-desc">不只是转发请求——从计费到权限、从监控到集成，覆盖企业级 AI 应用的每一个环节。</p>
					</div>

					<div class="features-grid">
						<div v-for="f in coreFeatures" :key="f.title" class="feature-card">
							<div class="feature-card-icon" :style="{ backgroundColor: f.bgColor, color: f.textColor }">
								<Icon :name="f.icon" size="lg" />
							</div>
							<h3 class="feature-card-title">{{ f.title }}</h3>
							<p class="feature-card-sub">{{ f.sub }}</p>
							<p class="feature-card-desc">{{ f.desc }}</p>
						</div>
					</div>
				</div>
			</section>

			<!-- ============ 场景展示 ============ -->
			<section id="scenarios" aria-label="场景展示" class="scenarios-section">
				<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 sm:py-28">
					<div class="text-center max-w-2xl mx-auto mb-14">
						<span class="section-tag">应用场景</span>
						<h2 class="section-title">覆盖从开发到运维的全链路</h2>
					</div>

					<!-- Scenario Tabs -->
					<div class="scenario-tabs">
						<button v-for="(s, i) in scenarios" :key="s.title"
							class="scenario-tab"
							:class="{ 'scenario-tab-active': activeScenario === i }"
							@click="activeScenario = i">
							<Icon :name="s.icon" size="sm" />
							<span>{{ s.tabLabel }}</span>
						</button>
					</div>

					<!-- Scenario Content -->
					<div class="scenario-content">
						<template v-for="(s, i) in scenarios" :key="s.title">
							<div v-show="activeScenario === i" class="scenario-panel">
								<div class="scenario-info">
									<div class="scenario-number">{{ String(i + 1).padStart(2, '0') }}</div>
									<h3 class="scenario-title">{{ s.title }}</h3>
									<p class="scenario-desc">{{ s.desc }}</p>
									<ul class="scenario-points">
										<li v-for="pt in s.points" :key="pt">
											<Icon name="check" size="xs" class="text-primary-500 flex-shrink-0 mt-0.5" />
											<span>{{ pt }}</span>
										</li>
									</ul>
								</div>
								<div class="scenario-visual">
									<!-- Developer: code example -->
									<div v-if="i === 0" class="code-card">
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
  <span class="tk-param">base_url</span><span class="tk-op">=</span><span class="tk-string">"https://your-domain.com/v1"</span><span class="tk-comma">,</span>
  <span class="tk-param">api_key</span><span class="tk-op">=</span><span class="tk-string">"sk-xxx"</span>
<span class="tk-paren">)</span>

<span class="tk-comment"># 像调用 OpenAI 一样调用任意模型</span>
<span class="tk-var">resp</span> <span class="tk-op">=</span> <span class="tk-var">client</span><span class="tk-op">.</span><span class="tk-method">chat</span><span class="tk-op">.</span><span class="tk-method">completions</span><span class="tk-op">.</span><span class="tk-method">create</span><span class="tk-paren">(</span>
  <span class="tk-param">model</span><span class="tk-op">=</span><span class="tk-string">"gpt-4o"</span><span class="tk-comma">,</span>
  <span class="tk-param">messages</span><span class="tk-op">=</span><span class="tk-bracket">[{</span><span class="tk-string">"role"</span><span class="tk-op">:</span> <span class="tk-string">"user"</span><span class="tk-comma">,</span> <span class="tk-string">"content"</span><span class="tk-op">:</span> <span class="tk-string">"Hello!"</span><span class="tk-bracket">}]</span>
<span class="tk-paren">)</span></code></pre>
									</div>

									<!-- Team: member usage dashboard mock -->
									<div v-else-if="i === 1" class="mock-card">
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

									<!-- Ops: monitoring mock -->
									<div v-else-if="i === 2" class="mock-card">
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
						</template>
					</div>
				</div>
			</section>

			<!-- ============ FAQ ============ -->
			<section id="faq" aria-label="常见问题" class="faq-section">
				<div class="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-20 sm:py-28">
					<div class="text-center mb-14">
						<span class="section-tag">常见问题</span>
						<h2 class="section-title">你可能想了解的</h2>
					</div>
					<div class="faq-list">
						<div v-for="(faq, index) in faqItems" :key="faq.question"
							class="faq-item"
							:class="{ 'faq-item-open': openFaq === index }">
							<button class="faq-question" @click="toggleFaq(index)">
								<span>{{ faq.question }}</span>
								<Icon name="chevronDown" size="sm" class="faq-chevron" />
							</button>
							<div class="faq-answer-wrapper">
								<div class="faq-answer">
									<p>{{ faq.answer }}</p>
								</div>
							</div>
						</div>
					</div>
				</div>
			</section>

		</main>

		<!-- Footer -->
		<footer aria-label="页脚" class="bg-gray-950 text-gray-500 border-t border-gray-800">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				<div class="flex flex-col sm:flex-row items-center justify-between gap-4">
					<div class="flex items-center gap-2">
						<img src="/favicon.png" :alt="siteName" class="h-5 w-5 rounded" />
						<span class="text-xs text-gray-600">&copy; 2026 qianfree. Released under AGPL-3.0.</span>
					</div>
					<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer"
						class="text-xs text-gray-600 hover:text-gray-400 transition-colors flex items-center gap-1.5">
						Powered by <span class="text-gray-400 font-medium">Team-API</span>
					</a>
				</div>
			</div>
		</footer>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useSeo } from '@/composables/useSeo'
import { useHead } from '@unhead/vue'
import { usePublicSettings } from '@/composables/usePublicSettings'

const { settings: publicSettings } = usePublicSettings()
const siteName = publicSettings.value.site_name || 'Team-API'

// Nav scroll state
const isScrolled = ref(false)
const handleScroll = () => { isScrolled.value = window.scrollY > 40 }
onMounted(() => window.addEventListener('scroll', handleScroll, { passive: true }))
onUnmounted(() => window.removeEventListener('scroll', handleScroll))

// FAQ accordion
const openFaq = ref<number | null>(null)
const toggleFaq = (index: number) => {
	openFaq.value = openFaq.value === index ? null : index
}

// Scenario tabs
const activeScenario = ref(0)

// SEO
useSeo({
	title: `${siteName} — 一个 API 接入所有大模型 | 企业级多租户 AI 网关`,
	description: publicSettings.value.site_description || `${siteName} 是开源自托管的企业级多租户大模型 API 网关平台。聚合 OpenAI、Claude、Gemini、DeepSeek 等 40+ 供应商，内置计费引擎、团队管理、用量审计与智能渠道调度。完全兼容 OpenAI SDK，只需修改 base_url 即可接入。`,
	siteName,
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
				name: siteName,
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
				name: siteName,
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
				name: siteName,
				url: 'https://team-api.net/',
				description: publicSettings.value.site_description || '企业级多租户大模型 API 网关平台',
				potentialAction: {
					'@type': 'SearchAction',
					target: 'https://github.com/qianfree/team-api/search?q={search_term_string}',
					'query-input': 'required name=search_term_string',
				},
			}),
		},
		{
			type: 'application/ld+json',
			innerHTML: JSON.stringify({
				'@context': 'https://schema.org',
				'@type': 'FAQPage',
				mainEntity: [
					{ '@type': 'Question', name: '什么是 Team-API？', acceptedAnswer: { '@type': 'Answer', text: 'Team-API 是一款开源自托管的企业级多租户大模型 API 网关平台。它聚合了 40+ 大模型供应商（包括 OpenAI、Anthropic Claude、Google Gemini、DeepSeek、阿里云百炼等），提供统一接口、内置计费引擎、团队管理、用量审计与智能渠道调度能力。' } },
					{ '@type': 'Question', name: '如何开始使用 Team-API？', acceptedAnswer: { '@type': 'Answer', text: '最简单的方式是使用 Docker：克隆仓库后执行 docker compose up -d，即可在 http://localhost:3000 访问。完全兼容 OpenAI SDK，只需修改 base_url 即可接入。' } },
					{ '@type': 'Question', name: 'Team-API 支持哪些大模型供应商？', acceptedAnswer: { '@type': 'Answer', text: 'Team-API 内置 40+ 供应商适配器，包括 OpenAI、Anthropic Claude、Google Gemini、DeepSeek、阿里云百炼、百度文心、腾讯混元、智谱AI、Mistral、Moonshot、Ollama 等。' } },
					{ '@type': 'Question', name: 'Team-API 如何计费？', acceptedAnswer: { '@type': 'Answer', text: 'Team-API 采用预扣→结算→退款的原子化计费流程，支持五层额度模型（租户→套餐→成员→项目→API Key），梯度定价，并发预扣防超扣，杜绝超额消费。' } },
					{ '@type': 'Question', name: 'Team-API 是免费的吗？', acceptedAnswer: { '@type': 'Answer', text: '是的，Team-API 采用 AGPL-3.0 开源协议，完全免费使用。大模型 API 调用的费用由各供应商收取，Team-API 本身不收取任何费用。' } },
					{ '@type': 'Question', name: 'Team-API 支持私有化部署吗？', acceptedAnswer: { '@type': 'Answer', text: '完全支持。通过 Docker Compose 一键部署到任何 Linux 服务器。支持 PostgreSQL 数据库和 Redis 缓存，数据完全存储在你自己的基础设施上。' } },
				],
			}),
		},
	],
})

// ============ Data ============

const heroProviders = [
	{ name: 'OpenAI', abbr: 'OA', bgColor: 'rgba(16,163,127,0.15)', textColor: '#10a37f' },
	{ name: 'Claude', abbr: 'CL', bgColor: 'rgba(204,120,50,0.15)', textColor: '#cc7832' },
	{ name: 'Gemini', abbr: 'GE', bgColor: 'rgba(66,133,244,0.15)', textColor: '#4285f4' },
	{ name: 'DeepSeek', abbr: 'DS', bgColor: 'rgba(20,184,166,0.15)', textColor: '#14b8a6' },
	{ name: '阿里百炼', abbr: 'AL', bgColor: 'rgba(255,106,0,0.15)', textColor: '#ff6a00' },
	{ name: '百度文心', abbr: 'BD', bgColor: 'rgba(36,100,230,0.15)', textColor: '#2464e6' },
	{ name: '腾讯混元', abbr: 'TX', bgColor: 'rgba(97,79,230,0.15)', textColor: '#614fe6' },
	{ name: '智谱AI', abbr: 'ZP', bgColor: 'rgba(75,142,240,0.15)', textColor: '#4b8ef0' },
	{ name: 'Mistral', abbr: 'MI', bgColor: 'rgba(255,120,0,0.15)', textColor: '#ff7800' },
	{ name: 'Moonshot', abbr: 'MK', bgColor: 'rgba(30,58,138,0.15)', textColor: '#1e3a8a' },
]

const coreFeatures = [
	{
		icon: 'users',
		title: '多租户管控',
		sub: '一人付费，团队共享',
		desc: '行级数据隔离的多租户架构。Owner 统一充值，按项目、成员分配额度。RBAC 权限精细到按钮级别，团队越大越省心。',
		bgColor: '#eff6ff',
		textColor: '#2563eb',
	},
	{
		icon: 'wallet',
		title: '精细计费',
		sub: '每笔费用都可追溯',
		desc: '每次消费只产生一笔记录，没有预扣退款等干扰项，冻结额度可追溯。成员用量一目了然，账单清晰到每一笔调用。',
		bgColor: '#fffbeb',
		textColor: '#d97706',
	},
	{
		icon: 'link',
		title: '开放平台',
		sub: '无缝对接 OA 系统',
		desc: '开放 API 操作系统数据，支持 Webhook 事件推送。与企业 OA、审批流、BI 看板无缝集成，自动化运维零门槛。',
		bgColor: '#f0fdfa',
		textColor: '#0d9488',
	},
	{
		icon: 'refresh',
		title: '智能调度',
		sub: '渠道故障零感知',
		desc: '多渠道路由引擎，优先级/权重调度 + 健康评分 + 自动故障切换。单渠道挂掉，用户无感切换到备用渠道。',
		bgColor: '#ecfeff',
		textColor: '#0891b2',
	},
	{
		icon: 'shield',
		title: '安全合规',
		sub: '企业级数据保护',
		desc: 'API Key 作用域控制，敏感数据 AES-256 加密存储，全链路操作审计。满足企业数据安全与合规要求。',
		bgColor: '#fff1f2',
		textColor: '#e11d48',
	},
	{
		icon: 'terminal',
		title: '开发者友好',
		sub: '改一行代码即接入',
		desc: '完全兼容 OpenAI SDK，只需修改 base_url。支持流式/非流式、Function Call、多模态、Realtime API 等主流特性。',
		bgColor: '#faf5ff',
		textColor: '#9333ea',
	},
]

const scenarios = [
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

const faqItems = [
	{
		question: '什么是 Team-API？',
		answer: 'Team-API 是一款开源自托管的企业级多租户大模型 API 网关平台。它聚合了 40+ 大模型供应商（包括 OpenAI、Anthropic Claude、Google Gemini、DeepSeek、阿里云百炼等），提供统一接口、内置计费引擎、团队管理、用量审计与智能渠道调度能力。',
	},
	{
		question: '如何开始使用 Team-API？',
		answer: '最简单的方式是使用 Docker：克隆仓库后执行 docker compose up -d，即可在 http://localhost:3000 访问。你也可以直接使用预构建的 Docker 镜像或从源码编译。完全兼容 OpenAI SDK，只需修改 base_url 即可接入。',
	},
	{
		question: 'Team-API 支持哪些大模型供应商？',
		answer: 'Team-API 内置 40+ 供应商适配器，包括 OpenAI、Anthropic Claude、Google Gemini、DeepSeek、阿里云百炼、百度文心、腾讯混元、智谱AI、Mistral、Moonshot、Ollama 等，通过统一接口自动转换协议，支持 SSE 流式转发。',
	},
	{
		question: 'Team-API 如何计费？',
		answer: 'Team-API 采用预扣→结算→退款的原子化计费流程，支持五层额度模型（租户→套餐→成员→项目→API Key），梯度定价，并发预扣防超扣，杜绝超额消费。',
	},
	{
		question: 'Team-API 是免费的吗？',
		answer: '是的，Team-API 采用 AGPL-3.0 开源协议，完全免费使用。你可以自行部署到自己的服务器上，数据完全自主可控。大模型 API 调用的费用由各供应商收取，Team-API 本身不收取任何费用。',
	},
	{
		question: 'Team-API 支持私有化部署吗？',
		answer: '完全支持。Team-API 设计为自托管架构，你可以通过 Docker Compose 一键部署到任何 Linux 服务器。支持 PostgreSQL 数据库和 Redis 缓存，数据完全存储在你自己的基础设施上。',
	},
]
</script>

<style scoped>
/* ================================================
   HERO
   ================================================ */
.hero-section {
	position: relative;
	background: #0a0f1a;
	overflow: hidden;
	min-height: 100vh;
	display: flex;
	align-items: center;
}
.hero-inner {
	position: relative;
	max-width: 80rem;
	margin: 0 auto;
	padding: 6rem 1rem 3rem;
	width: 100%;
}
@media (min-width: 640px) { .hero-inner { padding: 7rem 1.5rem 4rem; } }
@media (min-width: 1024px) { .hero-inner { padding: 8rem 2rem 5rem; } }
.hero-bg { position: absolute; inset: 0; }
.hero-grid {
	position: absolute; inset: 0;
	background-image:
		linear-gradient(rgba(20,184,166,0.05) 1px, transparent 1px),
		linear-gradient(90deg, rgba(20,184,166,0.05) 1px, transparent 1px);
	background-size: 64px 64px;
	mask-image: radial-gradient(ellipse 80% 60% at 50% 40%, black 20%, transparent 70%);
	-webkit-mask-image: radial-gradient(ellipse 80% 60% at 50% 40%, black 20%, transparent 70%);
}
.hero-orb {
	position: absolute; border-radius: 9999px; filter: blur(80px);
	animation: orbFloat 14s ease-in-out infinite;
}
.hero-orb-1 { top: -10%; right: -5%; width: 600px; height: 600px; background: rgba(20,184,166,0.12); }
.hero-orb-2 { bottom: -15%; left: -10%; width: 500px; height: 500px; background: rgba(6,182,212,0.08); animation-delay: -5s; }
.hero-orb-3 { top: 40%; left: 50%; width: 400px; height: 400px; background: rgba(99,102,241,0.06); animation-delay: -10s; }
@keyframes orbFloat {
	0%, 100% { transform: translate(0, 0) scale(1); }
	33% { transform: translate(20px, -30px) scale(1.05); }
	66% { transform: translate(-15px, 15px) scale(0.95); }
}
.hero-vignette {
	position: absolute; inset: 0;
	background: radial-gradient(ellipse at center, transparent 40%, #0a0f1a 100%);
}

.hero-badge {
	display: inline-flex; align-items: center; gap: 8px;
	padding: 6px 16px; border-radius: 9999px;
	background: rgba(20,184,166,0.1); border: 1px solid rgba(20,184,166,0.2);
	margin-bottom: 1.5rem; animation: fadeInUp 0.6s ease-out both;
}
.hero-badge-dot {
	width: 6px; height: 6px; border-radius: 9999px; background: #14b8a6;
	animation: pulse-dot 2s ease-in-out infinite;
}
@keyframes pulse-dot {
	0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(20,184,166,0.4); }
	50% { opacity: 0.8; box-shadow: 0 0 0 6px rgba(20,184,166,0); }
}
.hero-badge span:last-child { font-size: 13px; font-weight: 500; color: rgba(255,255,255,0.7); }

.hero-title {
	font-size: clamp(2.25rem, 6vw, 3.75rem); font-weight: 800;
	line-height: 1.15; letter-spacing: -0.03em; color: #fff;
	margin-bottom: 1.5rem; animation: fadeInUp 0.6s ease-out 0.1s both;
}
.hero-title-accent {
	background: linear-gradient(135deg, #14b8a6, #06b6d4, #6366f1);
	-webkit-background-clip: text; background-clip: text;
	-webkit-text-fill-color: transparent;
}
.hero-subtitle {
	font-size: clamp(1rem, 2vw, 1.15rem); line-height: 1.7;
	color: rgba(255,255,255,0.5); max-width: 520px; margin: 0 auto;
	animation: fadeInUp 0.6s ease-out 0.2s both;
}

.btn-hero-ghost {
	display: inline-flex; align-items: center; justify-content: center; gap: 8px;
	border-radius: 12px; padding: 12px 24px; font-size: 15px; font-weight: 500;
	background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.12);
	color: rgba(255,255,255,0.8); transition: all 0.2s ease;
}
.btn-hero-ghost:hover { background: rgba(255,255,255,0.1); border-color: rgba(255,255,255,0.2); color: #fff; }

/* Logo wall in hero */
.hero-logo-wall {
	position: relative; margin-top: 3.5rem; overflow: hidden;
	animation: fadeInUp 0.6s ease-out 0.35s both;
}
.logo-wall-inner {
	display: flex; gap: 12px; justify-content: center; flex-wrap: wrap;
	max-width: 680px; margin: 0 auto;
}
.logo-wall-item {
	display: flex; align-items: center; gap: 8px;
	padding: 8px 14px; border-radius: 10px;
	background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06);
	transition: all 0.2s ease;
}
.logo-wall-item:hover { background: rgba(255,255,255,0.08); border-color: rgba(20,184,166,0.2); }
.logo-wall-icon {
	width: 28px; height: 28px; border-radius: 6px;
	display: flex; align-items: center; justify-content: center;
	font-size: 10px; font-weight: 800; flex-shrink: 0;
}
.logo-wall-name { font-size: 12px; font-weight: 500; color: rgba(255,255,255,0.55); }
.logo-wall-fade-l, .logo-wall-fade-r { display: none; }

@keyframes fadeInUp {
	from { opacity: 0; transform: translateY(16px); }
	to { opacity: 1; transform: translateY(0); }
}

/* ================================================
   NAV
   ================================================ */
.landing-nav { position: fixed; top: 0; left: 0; right: 0; z-index: 50; transition: all 0.3s ease; }
.landing-nav:not(.nav-scrolled) { background: transparent; }
.landing-nav.nav-scrolled {
	background: rgba(255,255,255,0.85); backdrop-filter: blur(20px); -webkit-backdrop-filter: blur(20px);
	border-bottom: 1px solid rgba(0,0,0,0.06);
}
.nav-logo-icon {
	width: 28px; height: 28px; border-radius: 8px;
	display: flex; align-items: center; justify-content: center; transition: all 0.3s ease;
}
.landing-nav:not(.nav-scrolled) .nav-logo-icon { background: rgba(20,184,166,0.2); box-shadow: 0 0 16px rgba(20,184,166,0.3); }
.landing-nav.nav-scrolled .nav-logo-icon { background: linear-gradient(135deg,#14b8a6,#0d9488); box-shadow: 0 0 12px rgba(20,184,166,0.2); }
.btn-nav-cta { background: linear-gradient(135deg,#14b8a6,#0d9488); color: #fff; transition: all 0.2s ease; }
.btn-nav-cta:hover { box-shadow: 0 0 20px rgba(20,184,166,0.35); transform: translateY(-1px); }

/* ================================================
   SECTION SHARED
   ================================================ */
.section-tag {
	display: inline-block; font-size: 12px; font-weight: 600; color: #14b8a6;
	letter-spacing: 0.08em; text-transform: uppercase; margin-bottom: 0.75rem;
}
.section-title {
	font-size: clamp(1.75rem, 4vw, 2.5rem); font-weight: 800; color: #111827;
	letter-spacing: -0.02em; line-height: 1.2; margin-bottom: 0.75rem;
}
.section-desc { font-size: 16px; color: #6b7280; line-height: 1.6; }

/* ================================================
   核心价值 — FEATURES GRID
   ================================================ */
.features-section { background: #f9fafb; }

.features-grid {
	display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px;
}
.feature-card {
	background: #fff; border-radius: 16px; padding: 28px;
	border: 1px solid rgba(0,0,0,0.06); transition: all 0.3s ease;
}
.feature-card:hover {
	border-color: rgba(20,184,166,0.15); box-shadow: 0 8px 32px rgba(0,0,0,0.06);
	transform: translateY(-2px);
}
.feature-card-icon {
	width: 44px; height: 44px; border-radius: 12px;
	display: flex; align-items: center; justify-content: center; margin-bottom: 16px;
}
.feature-card-title { font-size: 17px; font-weight: 700; color: #111827; margin-bottom: 4px; }
.feature-card-sub { font-size: 13px; font-weight: 600; color: #0d9488; margin-bottom: 10px; }
.feature-card-desc { font-size: 14px; color: #6b7280; line-height: 1.65; }

@media (max-width: 768px) {
	.features-grid { grid-template-columns: 1fr; }
}

/* ================================================
   场景展示 — SCENARIOS
   ================================================ */
.scenarios-section { background: #fff; }

.scenario-tabs {
	display: flex; gap: 8px; justify-content: center; margin-bottom: 2rem;
}
.scenario-tab {
	display: inline-flex; align-items: center; gap: 6px;
	padding: 10px 20px; border-radius: 10px; font-size: 14px; font-weight: 500;
	color: #6b7280; background: #f9fafb; border: 1px solid transparent;
	cursor: pointer; transition: all 0.2s ease;
}
.scenario-tab:hover { color: #111827; background: #f3f4f6; }
.scenario-tab-active {
	color: #0d9488; background: #f0fdfa; border-color: rgba(20,184,166,0.2);
}

.scenario-content { min-height: 380px; }
.scenario-panel {
	display: grid; grid-template-columns: 1fr 1.2fr; gap: 2rem; align-items: flex-start;
	animation: fadeInUp 0.35s ease-out both;
}
.scenario-number {
	font-size: 12px; font-weight: 800; color: #14b8a6; letter-spacing: 0.05em; margin-bottom: 12px;
}
.scenario-title { font-size: 22px; font-weight: 700; color: #111827; margin-bottom: 10px; letter-spacing: -0.01em; }
.scenario-desc { font-size: 14px; color: #6b7280; line-height: 1.65; margin-bottom: 20px; }
.scenario-points { display: flex; flex-direction: column; gap: 10px; }
.scenario-points li {
	display: flex; align-items: flex-start; gap: 8px;
	font-size: 13px; color: #4b5563; line-height: 1.5;
}

/* Code card (shared with provider section) */
.code-card {
	border-radius: 12px; overflow: hidden;
	border: 1px solid rgba(0,0,0,0.08); box-shadow: 0 4px 24px rgba(0,0,0,0.06);
}
.code-card-bar {
	display: flex; align-items: center; padding: 10px 16px;
	background: #f9fafb; border-bottom: 1px solid rgba(0,0,0,0.06);
}
.terminal-dots { display: flex; gap: 6px; }
.terminal-dot { width: 10px; height: 10px; border-radius: 9999px; }
.terminal-dot-red { background: #ef4444; }
.terminal-dot-yellow { background: #eab308; }
.terminal-dot-green { background: #22c55e; }
.code-card-title {
	margin-left: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	font-size: 11px; color: #9ca3af;
}
.code-card-body {
	padding: 20px; background: #1e1e2e;
	font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	font-size: 13px; line-height: 1.8; overflow-x: auto;
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

/* Mock card for team & ops */
.mock-card {
	border-radius: 12px; overflow: hidden;
	border: 1px solid rgba(0,0,0,0.08); box-shadow: 0 4px 24px rgba(0,0,0,0.06);
	background: #fff;
}
.mock-header {
	display: flex; align-items: center; justify-content: space-between;
	padding: 14px 20px; border-bottom: 1px solid rgba(0,0,0,0.06); background: #fafafa;
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
	display: flex; align-items: center; gap: 10px; padding: 10px 20px;
	border-bottom: 1px solid rgba(0,0,0,0.04);
}
.mock-member-avatar { width: 28px; height: 28px; border-radius: 8px; flex-shrink: 0; }
.mock-member-info { display: flex; flex-direction: column; min-width: 60px; }
.mock-member-name { font-size: 12px; font-weight: 600; color: #374151; }
.mock-member-role { font-size: 10px; color: #9ca3af; }
.mock-usage-bar-bg { flex: 1; height: 6px; border-radius: 9999px; background: #f3f4f6; overflow: hidden; }
.mock-usage-bar-fill { height: 100%; border-radius: 9999px; transition: width 0.6s ease; }
.mock-usage-pct { font-size: 11px; font-weight: 600; color: #9ca3af; width: 32px; text-align: right; }

.mock-footer {
	display: flex; align-items: center; justify-content: space-between;
	padding: 12px 20px; background: #fafafa; font-size: 12px; color: #9ca3af;
}
.mock-footer-value { font-weight: 600; color: #374151; }

/* Ops metrics */
.mock-metric-row {
	display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px;
	padding: 16px 20px;
}
.mock-metric { text-align: center; }
.mock-metric-value { display: block; font-size: 20px; font-weight: 700; color: #111827; }
.mock-metric-label { display: block; font-size: 11px; color: #9ca3af; margin-top: 2px; }

.mock-chart {
	display: flex; align-items: flex-end; gap: 4px;
	height: 64px; padding: 0 20px 16px;
}
.mock-chart-bar {
	flex: 1; border-radius: 3px 3px 0 0;
	background: linear-gradient(to top, #14b8a6, #06b6d4); opacity: 0.6;
	min-height: 4px;
}
.mock-chart-bar:nth-child(odd) { opacity: 0.8; }

.mock-alert-row {
	display: flex; align-items: center; gap: 8px;
	padding: 10px 20px; border-top: 1px solid rgba(0,0,0,0.04);
	background: #fffbeb;
}
.mock-alert-text { font-size: 12px; color: #92400e; }

@media (max-width: 768px) {
	.scenario-panel { grid-template-columns: 1fr; }
	.scenario-tabs { flex-wrap: wrap; }
}

/* ================================================
   FAQ
   ================================================ */
.faq-section { background: #fff; }
.faq-list { display: flex; flex-direction: column; gap: 8px; }
.faq-item {
	border-radius: 12px; border: 1px solid rgba(0,0,0,0.06); overflow: hidden; transition: all 0.2s ease;
}
.faq-item:hover { border-color: rgba(20,184,166,0.15); }
.faq-item-open { border-color: rgba(20,184,166,0.2); box-shadow: 0 2px 12px rgba(20,184,166,0.06); }
.faq-question {
	display: flex; align-items: center; justify-content: space-between; width: 100%;
	padding: 18px 20px; text-align: left; font-size: 15px; font-weight: 600;
	color: #111827; background: none; border: none; cursor: pointer; transition: color 0.2s ease;
}
.faq-question:hover { color: #14b8a6; }
.faq-chevron { flex-shrink: 0; transition: transform 0.25s ease; color: #9ca3af; }
.faq-item-open .faq-chevron { transform: rotate(180deg); color: #14b8a6; }
.faq-answer-wrapper { max-height: 0; overflow: hidden; transition: max-height 0.3s ease; }
.faq-item-open .faq-answer-wrapper { max-height: 200px; }
.faq-answer { padding: 0 20px 18px; font-size: 14px; color: #6b7280; line-height: 1.7; }

/* ================================================
   CTA
   ================================================ */
.cta-card {
	position: relative; background: #111827; border-radius: 24px;
	padding: 64px 32px; overflow: hidden;
}
.cta-bg { position: absolute; inset: 0; }
.cta-orb { position: absolute; border-radius: 9999px; filter: blur(80px); }
.cta-orb-1 { top: -30%; right: -10%; width: 400px; height: 400px; background: rgba(20,184,166,0.15); }
.cta-orb-2 { bottom: -30%; left: -10%; width: 300px; height: 300px; background: rgba(99,102,241,0.1); }

/* ================================================
   GLOBAL OVERRIDES
   ================================================ */
.landing-page .btn-primary {
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	color: #fff; box-shadow: 0 0 20px rgba(20,184,166,0.3);
}
.landing-page .btn-primary:hover {
	box-shadow: 0 0 28px rgba(20,184,166,0.4); transform: translateY(-1px);
}
</style>
