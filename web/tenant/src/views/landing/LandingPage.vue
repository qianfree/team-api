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
						<span class="hidden sm:inline-flex items-center text-xs px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-600 font-medium border border-emerald-200">Open Source</span>
					</div>
					<div class="hidden md:flex items-center gap-8">
						<a href="#features" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">功能特性</a>
						<a href="#architecture" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">技术架构</a>
						<a href="#providers" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">模型生态</a>
						<a href="#quickstart" class="text-sm text-gray-600 hover:text-primary-600 transition-colors">快速开始</a>
					</div>
					<div class="flex items-center gap-3">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="hidden sm:flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 transition-colors">
							<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							<span>GitHub</span>
							<span class="text-xs px-1.5 py-0.5 rounded-md bg-gray-100 text-gray-600 font-mono">Star</span>
						</a>
						<router-link :to="{ name: 'TenantLogin' }" class="btn btn-ghost btn-sm text-gray-600 hover:text-primary-600">
							在线演示
						</router-link>
						<a href="#quickstart" class="btn btn-primary btn-sm">开始使用</a>
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
			<div class="absolute inset-0 opacity-[0.03]" style="background-image: radial-gradient(circle, #14b8a6 1px, transparent 1px); background-size: 24px 24px;"></div>

			<div class="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-4xl mx-auto">
					<div class="inline-flex items-center gap-3 mb-8 animate-fade-in">
						<span class="inline-flex items-center gap-2 rounded-full bg-emerald-50 border border-emerald-200 px-4 py-1.5">
							<span class="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
							<span class="text-sm font-medium text-emerald-700">开源自托管 · AGPL-3.0</span>
						</span>
					</div>

					<h1 class="text-4xl sm:text-5xl lg:text-6xl font-bold text-gray-900 leading-tight mb-6 animate-slide-up">
						自托管的多租户<br />
						<span class="text-gradient">AI API 网关平台</span>
					</h1>

					<p class="text-lg sm:text-xl text-gray-500 max-w-2xl mx-auto mb-10 animate-slide-up" style="animation-delay: 100ms">
						聚合 40+ 大模型供应商，内置计费引擎、团队管理、用量审计与渠道调度。
						完整源码开放，Docker 一键部署，数据完全自主掌控。
					</p>

					<div class="flex flex-col sm:flex-row items-center justify-center gap-4 animate-slide-up" style="animation-delay: 200ms">
						<a href="#quickstart" class="btn btn-primary btn-lg w-full sm:w-auto">
							<Icon name="arrowRight" size="sm" />
							快速开始
						</a>
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-lg w-full sm:w-auto gap-2">
							<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							GitHub
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

		<!-- Architecture Section -->
		<section id="architecture" class="py-20 sm:py-28 bg-gradient-to-b from-gray-50 to-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">系统架构</h2>
					<p class="text-lg text-gray-500">三层解耦设计，职责清晰，易于扩展和维护。</p>
				</div>

				<div class="max-w-4xl mx-auto">
					<!-- Client Layer -->
					<div class="card p-4 sm:p-5 mb-3">
						<div class="flex items-center justify-between flex-wrap gap-3">
							<div class="flex items-center gap-3">
								<div class="h-10 w-10 rounded-lg bg-blue-50 flex items-center justify-center">
									<Icon name="terminal" size="lg" class="text-blue-500" />
								</div>
								<div>
									<div class="text-sm font-semibold text-gray-900">客户端层</div>
									<div class="text-xs text-gray-400">通过标准 OpenAI 接口接入</div>
								</div>
							</div>
							<div class="flex flex-wrap items-center gap-2">
								<span class="text-xs px-2 py-1 rounded-md bg-gray-50 text-gray-500 font-mono">OpenAI SDK</span>
								<span class="text-xs px-2 py-1 rounded-md bg-gray-50 text-gray-500 font-mono">Claude SDK</span>
								<span class="text-xs px-2 py-1 rounded-md bg-gray-50 text-gray-500 font-mono">HTTP</span>
								<span class="text-xs px-2 py-1 rounded-md bg-gray-50 text-gray-500 font-mono">SSE</span>
							</div>
						</div>
					</div>

					<!-- Arrow -->
					<div class="flex justify-center my-1">
						<Icon name="arrowDown" size="sm" class="text-gray-300" />
					</div>

					<!-- Gateway Layer -->
					<div class="card p-5 sm:p-6 mb-3">
						<div class="flex items-center gap-3 mb-4">
							<div class="h-10 w-10 rounded-lg bg-primary-50 flex items-center justify-center">
								<Icon name="bolt" size="lg" class="text-primary-500" />
							</div>
							<div>
								<div class="text-sm font-semibold text-gray-900">Team-API 网关层</div>
								<div class="text-xs text-gray-400">认证 · 计费 · 调度 · 审计 · 限流</div>
							</div>
						</div>
						<div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
							<div v-for="mod in gatewayModules" :key="mod.name" class="flex items-center gap-2 px-3 py-2 rounded-lg bg-gray-50">
								<div class="h-2 w-2 rounded-full" :class="mod.dotColor"></div>
								<span class="text-xs font-medium text-gray-600">{{ mod.name }}</span>
							</div>
						</div>
					</div>

					<!-- Arrow -->
					<div class="flex justify-center my-1">
						<Icon name="arrowDown" size="sm" class="text-gray-300" />
					</div>

					<!-- Provider Layer -->
					<div class="card p-4 sm:p-5">
						<div class="flex items-center justify-between flex-wrap gap-3">
							<div class="flex items-center gap-3">
								<div class="h-10 w-10 rounded-lg bg-emerald-50 flex items-center justify-center">
									<Icon name="cube" size="lg" class="text-emerald-500" />
								</div>
								<div>
									<div class="text-sm font-semibold text-gray-900">AI 供应商层</div>
									<div class="text-xs text-gray-400">40+ 供应商适配器，统一转发</div>
								</div>
							</div>
							<div class="flex flex-wrap items-center gap-1.5">
								<span v-for="name in ['OpenAI', 'Claude', 'Gemini', 'DeepSeek', '通义', '文心', '混元', '智谱']" :key="name"
									class="text-xs font-mono px-2 py-0.5 rounded bg-gray-50 text-gray-500">
									{{ name }}
								</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Tech Stack -->
		<section class="py-20 sm:py-28 bg-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">技术栈</h2>
					<p class="text-lg text-gray-500">基于成熟稳定的技术选型，代码结构清晰，便于二次开发。</p>
				</div>

				<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4 max-w-4xl mx-auto">
					<div v-for="tech in techStack" :key="tech.name" class="card card-hover p-5 text-center">
						<div class="h-12 w-12 rounded-xl mx-auto mb-3 flex items-center justify-center text-lg font-bold"
							:style="{ backgroundColor: tech.bgColor, color: tech.textColor }">
							{{ tech.abbr }}
						</div>
						<div class="text-sm font-semibold text-gray-900">{{ tech.name }}</div>
						<div class="text-xs text-gray-400 mt-1">{{ tech.desc }}</div>
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

		<!-- Quick Start Section -->
		<section id="quickstart" class="py-20 sm:py-28 bg-white">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="text-center max-w-3xl mx-auto mb-16">
					<h2 class="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">三步部署</h2>
					<p class="text-lg text-gray-500">Docker Compose 一键启动，五分钟即可拥有自己的 AI API 网关。</p>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
					<div v-for="(step, idx) in quickStartSteps" :key="step.title" class="card card-hover p-6">
						<div class="h-10 w-10 rounded-full bg-primary-500 text-white flex items-center justify-center text-lg font-bold mb-4">
							{{ idx + 1 }}
						</div>
						<h3 class="text-lg font-semibold text-gray-900 mb-2">{{ step.title }}</h3>
						<p class="text-sm text-gray-500 mb-4">{{ step.desc }}</p>
						<div class="rounded-xl bg-gray-900 p-4">
							<code class="text-sm font-mono text-emerald-400 whitespace-pre-wrap">{{ step.code }}</code>
						</div>
					</div>
				</div>

				<div class="text-center mt-10">
					<p class="text-sm text-gray-400 mb-4">更详细的部署指南请参考文档</p>
					<div class="flex flex-col sm:flex-row items-center justify-center gap-3">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">
							<Icon name="bookOpen" size="sm" />
							阅读文档
						</a>
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn btn-ghost text-gray-600">
							<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							GitHub 仓库
						</a>
					</div>
				</div>
			</div>
		</section>

		<!-- Community CTA Section -->
		<section class="py-20 sm:py-28 relative overflow-hidden">
			<div class="absolute inset-0 bg-gradient-to-br from-gray-900 to-gray-800"></div>
			<div class="absolute inset-0 bg-[radial-gradient(at_30%_20%,rgba(20,184,166,0.15),transparent_50%)]"></div>
			<div class="absolute inset-0 bg-[radial-gradient(at_70%_80%,rgba(20,184,166,0.1),transparent_50%)]"></div>

			<div class="relative max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
				<h2 class="text-3xl sm:text-4xl font-bold text-white mb-4">加入社区，共建 Team-API</h2>
				<p class="text-lg text-gray-400 mb-10 max-w-2xl mx-auto">
					开源不仅是代码，更是社区。欢迎 Star、Fork、提交 Issue 和 PR，一起打造更好的 AI API 网关。
				</p>
				<div class="flex flex-col sm:flex-row items-center justify-center gap-4">
					<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="btn bg-white text-gray-900 hover:bg-gray-100 shadow-lg w-full sm:w-auto btn-lg gap-2">
						<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
						Star on GitHub
					</a>
					<a href="https://github.com/qianfree/team-api/issues" target="_blank" rel="noopener noreferrer" class="btn border-2 border-gray-600 text-gray-300 hover:bg-white/5 w-full sm:w-auto btn-lg">
						参与贡献
					</a>
				</div>
			</div>
		</section>

		<!-- Footer -->
		<footer class="py-12 bg-gray-900 text-gray-400 border-t border-gray-800">
			<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="grid grid-cols-2 md:grid-cols-4 gap-8 mb-8">
					<div class="col-span-2 md:col-span-1">
						<div class="flex items-center gap-2 mb-4">
							<div class="h-7 w-7 rounded-lg bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center">
								<Icon name="bolt" size="xs" class="text-white" :stroke-width="2" />
							</div>
							<span class="text-lg font-bold text-white">Team-API</span>
						</div>
						<p class="text-sm leading-relaxed">开源自托管<br />多租户 AI API 网关</p>
						<div class="mt-3">
							<span class="text-xs px-2 py-0.5 rounded bg-gray-800 text-gray-500 font-mono">AGPL-3.0 License</span>
						</div>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">项目</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="#features" class="hover:text-primary-400 transition-colors">功能特性</a></li>
							<li><a href="#architecture" class="hover:text-primary-400 transition-colors">技术架构</a></li>
							<li><a href="#providers" class="hover:text-primary-400 transition-colors">模型生态</a></li>
							<li><a href="#quickstart" class="hover:text-primary-400 transition-colors">快速开始</a></li>
						</ul>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">资源</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="#" class="hover:text-primary-400 transition-colors">技术文档</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">部署指南</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">更新日志</a></li>
							<li><a href="#" class="hover:text-primary-400 transition-colors">API 参考</a></li>
						</ul>
					</div>
					<div>
						<h4 class="text-sm font-semibold text-white mb-3">社区</h4>
						<ul class="space-y-2 text-sm">
							<li><a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="hover:text-primary-400 transition-colors">GitHub</a></li>
							<li><a href="https://github.com/qianfree/team-api/blob/main/CONTRIBUTING.md" target="_blank" rel="noopener noreferrer" class="hover:text-primary-400 transition-colors">贡献指南</a></li>
							<li><a href="https://github.com/qianfree/team-api/issues" target="_blank" rel="noopener noreferrer" class="hover:text-primary-400 transition-colors">问题反馈</a></li>
							<li><a href="https://github.com/qianfree/team-api/discussions" target="_blank" rel="noopener noreferrer" class="hover:text-primary-400 transition-colors">讨论区</a></li>
						</ul>
					</div>
				</div>
				<div class="border-t border-gray-800 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4">
					<p class="text-xs">&copy; 2026 Team-API. Released under the AGPL-3.0 License.</p>
					<div class="flex items-center gap-4 text-xs">
						<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="hover:text-primary-400 transition-colors flex items-center gap-1.5">
							<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
							GitHub
						</a>
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
	{ value: 'AGPL-3.0', label: '开源协议' },
	{ value: '全栈', label: '完整源码交付' },
]

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

const gatewayModules = [
	{ name: 'API Key 认证', dotColor: 'bg-blue-400' },
	{ name: '渠道调度', dotColor: 'bg-primary-400' },
	{ name: '计费引擎', dotColor: 'bg-amber-400' },
	{ name: '协议转换', dotColor: 'bg-emerald-400' },
	{ name: '限流控制', dotColor: 'bg-red-400' },
	{ name: '用量审计', dotColor: 'bg-purple-400' },
]

const techStack = [
	{ name: 'Go', abbr: 'Go', desc: '后端语言', bgColor: '#e0f2fe', textColor: '#0369a1' },
	{ name: 'Vue 3', abbr: 'V3', desc: '前端框架', bgColor: '#f0fdf4', textColor: '#15803d' },
	{ name: 'PostgreSQL', abbr: 'PG', desc: '数据库', bgColor: '#eff6ff', textColor: '#1d4ed8' },
	{ name: 'Redis', abbr: 'Rd', desc: '缓存', bgColor: '#fef2f2', textColor: '#b91c1c' },
	{ name: 'Docker', abbr: 'Dk', desc: '容器部署', bgColor: '#e0f2fe', textColor: '#0284c7' },
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

const quickStartSteps = [
	{
		title: '克隆项目',
		desc: '从 GitHub 克隆源码到本地',
		code: 'git clone https://github.com/\nqianfree/team-api.git\ncd team-api',
	},
	{
		title: '配置环境',
		desc: '复制环境变量配置文件并修改',
		code: 'cp .env.example .env\n# 编辑 .env 配置数据库\n# 和 Redis 连接信息',
	},
	{
		title: '启动服务',
		desc: 'Docker Compose 一键启动',
		code: 'docker compose up -d\n# 访问 http://localhost:3000',
	},
]
</script>
