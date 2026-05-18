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
					<div class="flex items-center gap-3">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="hidden sm:flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 transition-colors">
							<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							<span>GitHub</span>
						</a>
						<router-link :to="{ name: 'TenantLogin' }" class="btn btn-ghost btn-sm text-gray-600 hover:text-primary-600">
							在线演示
						</router-link>
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn btn-primary btn-sm">开始使用</a>
					</div>
				</div>
			</div>
		</nav>

		<!-- Hero Section -->
			<section class="relative min-h-screen flex items-center overflow-hidden">
			<div class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100"></div>
				<div class="absolute top-10 right-0 h-[500px] w-[500px] bg-primary-400/15 rounded-full blur-3xl"></div>
				<div class="absolute bottom-0 left-0 h-[400px] w-[400px] bg-primary-500/10 rounded-full blur-3xl"></div>
				<div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 h-96 w-96 bg-primary-300/8 rounded-full blur-3xl"></div>
			<div class="absolute inset-0 opacity-[0.03]" style="background-image: radial-gradient(circle, #14b8a6 1px, transparent 1px); background-size: 24px 24px;"></div>

				<div class="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24 w-full">
				<div class="text-center max-w-4xl mx-auto">
					<div class="inline-flex items-center gap-3 mb-8 animate-fade-in">
						<span class="inline-flex items-center gap-2 rounded-full bg-emerald-50 border border-emerald-200 px-4 py-1.5">
							<span class="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
							<span class="text-sm font-medium text-emerald-700">开源自托管 · AGPL-3.0</span>
						</span>
					</div>

					<h1 class="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 leading-tight mb-6 animate-slide-up">
						企业级·多租户<br />
						<span class="text-gradient">大模型 API 网关平台</span>
					</h1>

					<p class="text-lg sm:text-xl text-gray-500 max-w-2xl mx-auto mb-10 animate-slide-up" style="animation-delay: 100ms">
						聚合 40+ 大模型供应商，内置计费引擎、团队管理、用量审计与渠道调度。
					</p>

					<div class="flex flex-col sm:flex-row items-center justify-center gap-4 animate-slide-up" style="animation-delay: 200ms">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn btn-primary btn-lg w-full sm:w-auto">
							<Icon name="arrowRight" size="sm" />
							快速开始
						</a>
						<router-link :to="{ name: 'TenantLogin' }" class="btn btn-ghost btn-lg w-full sm:w-auto text-gray-600">
							在线演示
						</router-link>
					</div>

					<!-- Terminal Preview -->
					<div class="mt-16 max-w-2xl mx-auto animate-slide-up" style="animation-delay: 300ms">
						<div class="rounded-2xl overflow-hidden shadow-2xl border border-gray-200 bg-white">
							<div class="flex items-center gap-2 px-4 py-3 bg-gray-800">
								<div class="h-3 w-3 rounded-full bg-red-400"></div>
								<div class="h-3 w-3 rounded-full bg-yellow-400"></div>
								<div class="h-3 w-3 rounded-full bg-green-400"></div>
								<span class="ml-2 text-xs text-gray-400 font-mono">terminal</span>
							</div>
							<div class="bg-gray-900 p-5 sm:p-6 text-left">
								<pre class="text-sm font-mono leading-loose"><code><span class="text-gray-500"># 克隆项目并启动</span>
<span class="text-emerald-400">$</span> <span class="text-gray-300">git clone https://github.com/qianfree/team-api.git</span>
<span class="text-emerald-400">$</span> <span class="text-gray-300">cd team-api</span>
<span class="text-emerald-400">$</span> <span class="text-gray-300">cp .env.example .env</span>
<span class="text-emerald-400">$</span> <span class="text-gray-300">docker compose up -d</span>

<span class="text-gray-500"># 访问 http://localhost:3000 即可使用</span></code></pre>
							</div>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Features Section -->
		<section id="features" class="py-20 sm:py-28 bg-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">核心能力</h2>
					<p class="text-lg text-gray-500">
						不只是 API 转发，而是一套完整的多租户 AI 网关基础设施。
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


		<!-- Supported Providers -->
		<section id="providers" class="py-20 sm:py-28 bg-gradient-to-b from-gray-50 to-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">广泛的模型生态</h2>
					<p class="text-lg text-gray-500">
						内置 40+ 供应商适配器，统一 OpenAI 兼容接口，用户无需修改代码即可接入。
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
					<p class="text-center text-sm text-gray-500 mb-4">完全兼容 OpenAI SDK，只需修改 base_url 即可接入</p>
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

		<!-- Footer -->
		<footer class="bg-gray-950 text-gray-500">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
				<div class="flex flex-col sm:flex-row items-center justify-between gap-4">
					<p class="text-xs text-gray-600">&copy; 2026 qianfree. Released under the AGPL-3.0 License.</p>
					<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="text-xs text-gray-600 hover:text-gray-400 transition-colors flex items-center gap-1.5">Powered by <span class="text-gray-400 font-medium">Team-API</span></a>
				</div>
			</div>
		</footer>
	</div>
</template>

<script setup lang="ts">

const features = [
	{
		icon: 'users',
		title: '多租户与团队协作',
		desc: '行级数据隔离的多租户架构，支持组织创建、成员邀请、RBAC 权限（Owner/Admin/Member）、项目级资源分配。',
		iconBg: 'bg-blue-50',
		iconColor: 'text-blue-600',
	},
	{
		icon: 'bolt',
		title: '统一 AI 代理层',
		desc: '40+ 供应商适配器，SSE 流式转发，OpenAI/Claude/Gemini 多协议自动转换，请求级超时控制，流式中断资源回收。',
		iconBg: 'bg-primary-50',
		iconColor: 'text-primary-600',
	},
	{
		icon: 'wallet',
		title: '精细化计费引擎',
		desc: '预扣→结算→退款的原子化计费流程，五层额度模型（租户→套餐→成员→项目→Key），梯度定价，杜绝超额消费。',
		iconBg: 'bg-amber-50',
		iconColor: 'text-amber-600',
	},
	{
		icon: 'refresh',
		title: '智能渠道调度',
		desc: '多渠道路由引擎，支持优先级/权重调度、健康评分、自动故障切换、渠道亲和性缓存，单渠道故障零感知。',
		iconBg: 'bg-cyan-50',
		iconColor: 'text-cyan-600',
	},
	{
		icon: 'shield',
		title: '安全与权限',
		desc: 'RBAC 权限模型，API Key 作用域控制，敏感数据 AES-256 加密，并发预扣防超扣，全链路操作审计。',
		iconBg: 'bg-rose-50',
		iconColor: 'text-rose-600',
	},
	{
		icon: 'terminal',
		title: '开发者友好',
		desc: '完全兼容 OpenAI SDK，只需修改 base_url 即可接入。支持流式/非流式、Function Call、多模态等主流特性。',
		iconBg: 'bg-purple-50',
		iconColor: 'text-purple-600',
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
</script>
