<template>
	<div class="min-h-screen bg-white">
		<!-- Navigation -->
		<nav class="fixed top-0 left-0 right-0 z-50 glass border-b border-gray-100/50">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="flex items-center justify-between h-16">
					<div class="flex items-center gap-3">
						<div class="h-8 w-8 rounded-lg bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center shadow-glow">
							<Icon name="bolt" size="sm" class="text-white" :stroke-width="2" />
						</div>
						<span class="text-xl font-bold text-gradient">Team-API</span>
					</div>
					<div class="hidden md:flex items-center gap-8">
						<a href="#features" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">系统能力</a>
						<a href="#highlights" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">产品亮点</a>
						<a href="#providers" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">模型生态</a>
						<a href="#pricing" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">授权方案</a>
					</div>
					<div class="flex items-center gap-3">
						<router-link :to="{ name: 'TenantLogin' }" class="btn btn-ghost btn-sm text-gray-600 hover:text-primary-600">
							演示登录
						</router-link>
						<a href="#pricing" class="btn btn-primary btn-sm">获取报价</a>
					</div>
				</div>
			</div>
		</nav>

		<!-- Hero Section -->
		<section class="relative pt-32 pb-20 overflow-hidden">
			<div class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100"></div>
			<div class="absolute top-20 right-0 h-96 w-96 bg-primary-400/15 rounded-full blur-3xl"></div>
			<div class="absolute bottom-0 left-0 h-80 w-80 bg-primary-500/10 rounded-full blur-3xl"></div>
			<div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 h-72 w-72 bg-primary-300/8 rounded-full blur-3xl"></div>

			<div class="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-4xl mx-auto">
					<div class="inline-flex items-center gap-2 rounded-full bg-primary-50 border border-primary-200 px-4 py-1.5 mb-8 animate-fade-in">
						<Icon name="bolt" size="xs" class="text-primary-600" />
						<span class="text-sm font-medium text-primary-700">开箱即用的多租户 AI API 网关系统</span>
					</div>

					<h1 class="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 leading-tight mb-6 animate-slide-up">
						一套系统，开启你的<br />
						<span class="text-gradient">AI API 商业化之路</span>
					</h1>

					<p class="text-lg sm:text-xl text-gray-500 max-w-2xl mx-auto mb-10 animate-slide-up" style="animation-delay: 100ms">
						Team-API 是一套完整的多租户 AI API 网关 SaaS 系统。聚合 40+ 大模型供应商，
						内置计费引擎、用量审计、团队管理、开放平台等能力，助力您快速搭建自己的 AI API 服务。
					</p>

					<div class="flex flex-col sm:flex-row items-center justify-center gap-4 animate-slide-up" style="animation-delay: 200ms">
						<a href="#pricing" class="btn btn-primary btn-lg w-full sm:w-auto">
							<Icon name="arrowRight" size="sm" />
							了解授权方案
						</a>
						<router-link :to="{ name: 'TenantLogin' }" class="btn btn-secondary btn-lg w-full sm:w-auto">
							在线演示
						</router-link>
					</div>

					<!-- Architecture Preview -->
					<div class="mt-16 max-w-3xl mx-auto animate-slide-up" style="animation-delay: 300ms">
						<div class="rounded-2xl overflow-hidden shadow-2xl border border-gray-200 bg-white">
							<div class="flex items-center gap-2 px-4 py-3 bg-gray-800">
								<div class="h-3 w-3 rounded-full bg-red-400"></div>
								<div class="h-3 w-3 rounded-full bg-yellow-400"></div>
								<div class="h-3 w-3 rounded-full bg-green-400"></div>
								<span class="ml-2 text-xs text-gray-400 font-mono">系统架构</span>
							</div>
							<div class="bg-gradient-to-b from-gray-900 to-gray-800 p-6 sm:p-8 text-center">
								<div class="grid grid-cols-3 gap-3 sm:gap-4 max-w-lg mx-auto">
									<div class="rounded-xl bg-primary-500/20 border border-primary-400/30 px-3 py-3 sm:px-4 sm:py-4">
										<Icon name="users" size="lg" class="text-primary-400 mx-auto mb-2" />
										<div class="text-xs sm:text-sm font-medium text-primary-300">管理后台</div>
										<div class="text-[10px] sm:text-xs text-primary-400/60 mt-1">运营 · 监控 · 配置</div>
									</div>
									<div class="rounded-xl bg-amber-500/20 border border-amber-400/30 px-3 py-3 sm:px-4 sm:py-4">
										<Icon name="building" size="lg" class="text-amber-400 mx-auto mb-2" />
										<div class="text-xs sm:text-sm font-medium text-amber-300">租户控制台</div>
										<div class="text-[10px] sm:text-xs text-amber-400/60 mt-1">团队 · 用量 · 财务</div>
									</div>
									<div class="rounded-xl bg-emerald-500/20 border border-emerald-400/30 px-3 py-3 sm:px-4 sm:py-4">
										<Icon name="bolt" size="lg" class="text-emerald-400 mx-auto mb-2" />
										<div class="text-xs sm:text-sm font-medium text-emerald-300">AI 代理层</div>
										<div class="text-[10px] sm:text-xs text-emerald-400/60 mt-1">调度 · 转发 · 计费</div>
									</div>
								</div>
								<div class="mt-4 flex items-center justify-center gap-2 text-gray-500">
									<div class="h-px flex-1 bg-gradient-to-r from-transparent to-gray-600"></div>
									<Icon name="arrowDown" size="sm" class="text-gray-500" />
									<span class="text-xs font-mono text-gray-500">40+ AI 供应商适配</span>
									<Icon name="arrowDown" size="sm" class="text-gray-500" />
									<div class="h-px flex-1 bg-gradient-to-l from-transparent to-gray-600"></div>
								</div>
								<div class="mt-3 flex flex-wrap items-center justify-center gap-2">
									<span v-for="name in ['OpenAI', 'Claude', 'Gemini', 'DeepSeek', '通义', '文心', '混元', '智谱']" :key="name"
										class="text-xs font-mono px-2 py-1 rounded-md bg-white/5 text-gray-400 border border-gray-700">
										{{ name }}
									</span>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Stats -->
		<section class="py-16 bg-white border-y border-gray-100">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="grid grid-cols-2 lg:grid-cols-4 gap-8">
					<div v-for="stat in stats" :key="stat.label" class="text-center">
						<div class="text-3xl sm:text-4xl font-bold text-gradient mb-2">{{ stat.value }}</div>
						<div class="text-sm text-gray-500">{{ stat.label }}</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Features Section -->
		<section id="features" class="py-20 sm:py-28 bg-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">完整的 SaaS 管理能力</h2>
					<p class="text-lg text-gray-500">
						不只是一个 API 转发层，而是一套覆盖运营、管理、计费、监控的完整商业系统。
					</p>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
					<div v-for="feature in features" :key="feature.title" class="card card-hover p-6">
						<div class="h-12 w-12 rounded-xl flex items-center justify-center mb-4" :class="feature.iconBg">
							<Icon :name="feature.icon" size="lg" :class="feature.iconColor" />
						</div>
						<h3 class="text-lg font-semibold text-gray-900 mb-2">{{ feature.title }}</h3>
						<p class="text-sm text-gray-500 leading-relaxed">{{ feature.desc }}</p>
					</div>
				</div>
			</div>
		</section>

		<!-- Highlights Section -->
		<section id="highlights" class="py-20 sm:py-28 bg-gradient-to-b from-gray-50 to-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">为什么选择 Team-API</h2>
					<p class="text-lg text-gray-500">区别于开源方案的差异化优势，让您的业务从第一天就跑在专业级基础设施上。</p>
				</div>

				<div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
					<div v-for="item in highlights" :key="item.title" class="card card-hover p-6 sm:p-8 flex gap-5">
						<div class="h-12 w-12 rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center flex-shrink-0 shadow-md shadow-primary-500/20">
							<Icon :name="item.icon" size="lg" class="text-white" />
						</div>
						<div>
							<h3 class="text-lg font-semibold text-gray-900 mb-2">{{ item.title }}</h3>
							<p class="text-sm text-gray-500 leading-relaxed">{{ item.desc }}</p>
							<ul class="mt-3 flex flex-wrap gap-2">
								<li v-for="tag in item.tags" :key="tag"
									class="text-xs px-2.5 py-1 rounded-full bg-primary-50 text-primary-600 font-medium">
									{{ tag }}
								</li>
							</ul>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Supported Providers -->
		<section id="providers" class="py-20 sm:py-28 bg-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">广泛的模型生态</h2>
					<p class="text-lg text-gray-500">
						内置 40+ 供应商适配器，统一 OpenAI 兼容接口，您的用户无需修改代码即可接入。
					</p>
				</div>

				<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
					<div v-for="provider in providers" :key="provider.name" class="card card-hover p-5 flex items-center gap-4">
						<div class="h-10 w-10 rounded-lg flex items-center justify-center text-lg font-bold flex-shrink-0"
							:style="{ backgroundColor: provider.bgColor, color: provider.textColor }">
							{{ provider.abbr }}
						</div>
						<div>
							<div class="text-sm font-semibold text-gray-900">{{ provider.name }}</div>
							<div class="text-xs text-gray-400">{{ provider.models }}+ 模型</div>
						</div>
					</div>
				</div>

				<!-- Code Example -->
				<div class="mt-12 max-w-2xl mx-auto">
					<p class="text-center text-sm text-gray-500 mb-4">您的用户只需这样调用 —— 完全兼容 OpenAI SDK</p>
					<div class="rounded-2xl overflow-hidden shadow-lg border border-gray-200">
						<div class="flex items-center gap-2 px-4 py-3 bg-gray-800">
							<div class="h-2.5 w-2.5 rounded-full bg-red-400"></div>
							<div class="h-2.5 w-2.5 rounded-full bg-yellow-400"></div>
							<div class="h-2.5 w-2.5 rounded-full bg-green-400"></div>
							<span class="ml-2 text-xs text-gray-400 font-mono">quickstart.py</span>
						</div>
						<div class="bg-gray-900 p-5 text-left">
							<pre class="text-sm font-mono leading-relaxed"><code><span class="text-purple-400">from</span> <span class="text-blue-300">openai</span> <span class="text-purple-400">import</span> <span class="text-blue-300">OpenAI</span>

<span class="text-gray-300">client</span> <span class="text-white">=</span> <span class="text-blue-300">OpenAI</span><span class="text-white">(</span>
  <span class="text-orange-300">base_url</span><span class="text-white">=</span><span class="text-green-400">"https://your-domain.com/v1"</span><span class="text-white">,</span>
  <span class="text-orange-300">api_key</span><span class="text-white">=</span><span class="text-green-400">"sk-xxx"</span>
<span class="text-white">)</span>

<span class="text-gray-300">resp</span> <span class="text-white">=</span> <span class="text-gray-300">client</span><span class="text-white">.</span><span class="text-blue-300">chat</span><span class="text-white">.</span><span class="text-blue-300">completions</span><span class="text-white">.</span><span class="text-blue-300">create</span><span class="text-white">(</span>
  <span class="text-orange-300">model</span><span class="text-white">=</span><span class="text-green-400">"gpt-4o"</span><span class="text-white">,</span>
  <span class="text-orange-300">messages</span><span class="text-white">=[{</span><span class="text-green-400">"role"</span><span class="text-white">:</span> <span class="text-green-400">"user"</span><span class="text-white">,</span> <span class="text-green-400">"content"</span><span class="text-white">:</span> <span class="text-green-400">"Hello!"</span><span class="text-white">}]</span>
<span class="text-white">)</span></code></pre>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Pricing Section -->
		<section id="pricing" class="py-20 sm:py-28 bg-gradient-to-b from-gray-50 to-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">授权方案</h2>
					<p class="text-lg text-gray-500">按需选择适合您的授权方式，源码交付，永久使用。</p>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
					<div v-for="plan in plans" :key="plan.name"
						class="card card-hover p-6 relative"
						:class="{ 'ring-2 ring-primary-500 shadow-glow': plan.highlighted }">
						<div v-if="plan.badge"
							class="absolute -top-3 left-1/2 -translate-x-1/2 bg-gradient-to-r from-primary-500 to-primary-600 text-white text-xs font-medium px-3 py-1 rounded-full shadow-glow">
							{{ plan.badge }}
						</div>
						<div class="text-center mb-6">
							<h3 class="text-lg font-semibold text-gray-900 mb-2">{{ plan.name }}</h3>
							<div class="flex items-baseline justify-center gap-1">
								<span class="text-3xl font-bold text-gray-900">{{ plan.price }}</span>
								<span class="text-sm text-gray-400" v-if="plan.period">{{ plan.period }}</span>
							</div>
							<p class="mt-2 text-sm text-gray-500">{{ plan.desc }}</p>
						</div>
						<ul class="space-y-3 mb-6">
							<li v-for="item in plan.features" :key="item" class="flex items-center gap-2 text-sm text-gray-600">
								<Icon name="check" size="sm" class="text-primary-500 flex-shrink-0" />
								{{ item }}
							</li>
						</ul>
						<a href="#" class="btn w-full text-center" :class="plan.highlighted ? 'btn-primary' : 'btn-secondary'">
							{{ plan.cta }}
						</a>
					</div>
				</div>
			</div>
		</section>

		<!-- CTA Section -->
		<section class="py-20 sm:py-28 relative overflow-hidden">
			<div class="absolute inset-0 bg-gradient-to-br from-primary-600 to-primary-800"></div>
			<div class="absolute inset-0 bg-[radial-gradient(at_30%_20%,rgba(255,255,255,0.1),transparent_50%)]"></div>
			<div class="absolute inset-0 bg-[radial-gradient(at_70%_80%,rgba(255,255,255,0.08),transparent_50%)]"></div>

			<div class="relative max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
				<h2 class="text-3xl sm:text-4xl font-bold text-white mb-4">搭建您自己的 AI API 平台</h2>
				<p class="text-lg text-primary-100 mb-10 max-w-2xl mx-auto">
					一次部署，长期收益。Team-API 帮您省去数月开发周期，即刻上线专业的 AI API 商业服务。
				</p>
				<div class="flex flex-col sm:flex-row items-center justify-center gap-4">
					<a href="#" class="btn bg-white text-primary-700 hover:bg-primary-50 shadow-lg w-full sm:w-auto btn-lg">
						咨询详情
					</a>
					<router-link :to="{ name: 'TenantLogin' }"
						class="btn border-2 border-white/30 text-white hover:bg-white/10 w-full sm:w-auto btn-lg">
						在线体验演示
					</router-link>
				</div>
			</div>
		</section>

		<!-- Footer -->
		<footer class="py-12 bg-gray-900 text-gray-400">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="grid grid-cols-2 md:grid-cols-4 gap-8 mb-8">
					<div class="col-span-2 md:col-span-1">
						<div class="flex items-center gap-2 mb-4">
							<div class="h-7 w-7 rounded-lg bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center">
								<Icon name="bolt" size="xs" class="text-white" :stroke-width="2" />
							</div>
							<span class="text-lg font-bold text-white">Team-API</span>
						</div>
						<p class="text-sm leading-relaxed">开箱即用的<br />多租户 AI API 网关系统</p>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">产品</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="#features" class="hover:text-primary-400 transition-colors">系统能力</a></li>
							<li><a href="#highlights" class="hover:text-primary-400 transition-colors">产品亮点</a></li>
							<li><a href="#pricing" class="hover:text-primary-400 transition-colors">授权方案</a></li>
						</ul>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">技术</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="#providers" class="hover:text-primary-400 transition-colors">支持的模型</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">技术文档</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">部署指南</a></li>
						</ul>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">联系</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="#" class="hover:text-primary-400 transition-colors">商务咨询</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">技术支持</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">关于我们</a></li>
						</ul>
					</div>
				</div>
				<div class="border-t border-gray-800 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4">
					<p class="text-xs">&copy; 2026 Team-API. All rights reserved.</p>
					<div class="flex items-center gap-4 text-xs">
						<a href="#" class="hover:text-primary-400 transition-colors">服务条款</a>
						<a href="#" class="hover:text-primary-400 transition-colors">隐私政策</a>
					</div>
				</div>
			</div>
		</footer>
	</div>
</template>

<script setup lang="ts">
const stats = [
	{ value: '40+', label: 'AI 供应商适配器' },
	{ value: '200+', label: '开箱可用模型' },
	{ value: '5 层', label: '额度管控体系' },
	{ value: '双控制台', label: '管理 + 租户' },
]

const features = [
	{
		icon: 'users',
		title: '多租户与团队管理',
		desc: '完整的多租户数据隔离，支持组织创建、成员邀请、角色权限（Owner/Admin/Member）、项目级资源分配，满足企业级团队协作需求。',
		iconBg: 'bg-blue-50',
		iconColor: 'text-blue-600',
	},
	{
		icon: 'chart',
		title: '成员用量分析',
		desc: '按成员、项目、Key、模型多维度统计 Token 用量与费用。趋势图表、模型分布、用量排名、费用预测，让每一笔 AI 支出透明可控。',
		iconBg: 'bg-primary-50',
		iconColor: 'text-primary-600',
	},
	{
		icon: 'clipboard',
		title: '请求审计日志',
		desc: '全链路请求追踪，记录每次 API 调用的模型、Token 用量、响应状态、耗时、费用。支持按时间、成员、模型检索，满足合规审计要求。',
		iconBg: 'bg-emerald-50',
		iconColor: 'text-emerald-600',
	},
	{
		icon: 'wallet',
		title: '计费引擎与钱包体系',
		desc: '预扣→结算→退款的原子化计费流程，五层额度模型（租户→套餐→成员→项目→Key），梯度定价，双币种（CNY/USD），杜绝超额消费。',
		iconBg: 'bg-amber-50',
		iconColor: 'text-amber-600',
	},
	{
		icon: 'link',
		title: '开放平台与第三方对接',
		desc: '内置 Webhook 事件推送、开放平台应用管理、API 文档中心、SSO 单点登录（SAML/OIDC），无缝对接您的现有业务系统。',
		iconBg: 'bg-purple-50',
		iconColor: 'text-purple-600',
	},
	{
		icon: 'shield',
		title: '安全与权限体系',
		desc: 'RBAC 权限模型，API Key 作用域控制，敏感数据 AES-256 加密，并发预扣防超扣，操作审计日志，保障平台与用户数据安全。',
		iconBg: 'bg-rose-50',
		iconColor: 'text-rose-600',
	},
	{
		icon: 'refresh',
		title: '渠道调度与容灾',
		desc: '多渠道路由引擎，支持优先级/权重调度、健康评分、自动故障切换、渠道亲和性缓存。确保 AI 服务高可用，单渠道故障零感知。',
		iconBg: 'bg-cyan-50',
		iconColor: 'text-cyan-600',
	},
	{
		icon: 'creditCard',
		title: '商业化运营能力',
		desc: '套餐方案管理、在线支付（支付宝/微信/Stripe）、订单与退款、兑换码、优惠券、发票管理。从用户注册到付费收款，全流程闭环。',
		iconBg: 'bg-orange-50',
		iconColor: 'text-orange-600',
	},
	{
		icon: 'cog',
		title: '管理后台与运维',
		desc: '独立的管理控制台，渠道管理、模型配置、系统监控、告警规则、工单系统、状态页面。一套后台掌控平台全局运营状态。',
		iconBg: 'bg-indigo-50',
		iconColor: 'text-indigo-600',
	},
]

const highlights = [
	{
		icon: 'cube',
		title: '真·多租户 SaaS 架构',
		desc: '从数据模型到路由设计全部围绕多租户构建，行级数据隔离，租户间零干扰。支持无限租户接入，每个租户独立控制台、独立成员体系、独立财务账本。',
		tags: ['行级隔离', '独立控制台', '无限租户'],
	},
	{
		icon: 'bolt',
		title: '生产级 AI 代理层',
		desc: '40+ 供应商适配器、SSE 流式转发、OpenAI/Claude/Gemini 多协议自动转换、流式中断资源回收、请求级超时控制。经过大流量验证的转发引擎。',
		tags: ['流式转发', '协议转换', '大流量验证'],
	},
	{
		icon: 'terminal',
		title: '开发者友好',
		desc: '您的用户无需学习新接口，直接使用 OpenAI 官方 SDK 连接。系统自动处理不同供应商的协议差异，包括请求格式转换、响应格式统一、错误码映射。',
		tags: ['OpenAI 兼容', '零迁移成本', '多语言 SDK'],
	},
	{
		icon: 'document',
		title: '完整源码交付',
		desc: 'Go + Vue 3 全栈源码，代码结构清晰，注释完整。包含后端 API、管理后台前端、租户控制台前端、数据库迁移脚本、部署配置，您完全掌控每一行代码。',
		tags: ['全栈源码', 'Go + Vue 3', '可二次开发'],
	},
]

const providers = [
	{ name: 'OpenAI', abbr: 'OA', models: '30', bgColor: '#f0fdfa', textColor: '#0d9488' },
	{ name: 'Anthropic Claude', abbr: 'CL', models: '15', bgColor: '#fef3c7', textColor: '#92400e' },
	{ name: 'Google Gemini', abbr: 'GE', models: '20', bgColor: '#e0f2fe', textColor: '#0369a1' },
	{ name: 'DeepSeek', abbr: 'DS', models: '10', bgColor: '#f0fdf4', textColor: '#15803d' },
	{ name: '阿里云百炼', abbr: 'AL', models: '20', bgColor: '#fff7ed', textColor: '#c2410c' },
	{ name: '百度文心', abbr: 'BD', models: '15', bgColor: '#eff6ff', textColor: '#1d4ed8' },
	{ name: '腾讯混元', abbr: 'TX', models: '10', bgColor: '#f5f3ff', textColor: '#6d28d9' },
	{ name: '智谱 AI', abbr: 'ZP', models: '10', bgColor: '#fdf2f8', textColor: '#be185d' },
]

const plans = [
	{
		name: '标准版',
		price: '¥9,999',
		period: '起',
		desc: '适合快速启动 AI API 业务',
		badge: '',
		highlighted: false,
		cta: '咨询详情',
		features: ['完整源码交付', '多租户架构', '40+ 供应商适配', '基础计费引擎', '管理后台 + 租户控制台', 'PostgreSQL + Redis', '3 个月技术支持'],
	},
	{
		name: '专业版',
		price: '¥29,999',
		period: '起',
		desc: '全功能商业化运营',
		badge: '推荐',
		highlighted: true,
		cta: '咨询详情',
		features: ['标准版全部功能', '支付系统集成', '套餐与订阅管理', '开放平台 & Webhook', 'SSO 单点登录', '工单与帮助中心', '12 个月技术支持', '免费功能更新'],
	},
	{
		name: '企业版',
		price: '定制',
		period: '',
		desc: '私有化部署 + 定制开发',
		badge: '',
		highlighted: false,
		cta: '联系我们',
		features: ['专业版全部功能', '私有化部署支持', '定制功能开发', '品牌白标定制', 'SLA 服务保障', '专属技术对接', 'K8s 部署方案', '优先 Issue 响应'],
	},
]
</script>
