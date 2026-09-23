<template>
  <div ref="rootEl" class="lp" :data-lp-theme="theme">

    <!-- ===================== 背景 ===================== -->
    <div class="bg" aria-hidden="true">
      <div class="bg__wash"></div>
      <div class="bg__grid"></div>
      <div class="bg__noise"></div>
      <div class="bg__ring bg__ring--1"></div>
      <div class="bg__ring bg__ring--2"></div>
    </div>

    <!-- ===================== 导航 ===================== -->
    <nav class="nav">
      <div class="brand">
        <div class="brand__mark">
          <svg viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round">
            <path d="M12 3v18M3 12h18"/>
          </svg>
        </div>
        <span class="brand__name">{{ siteName }}</span>
        <span class="brand__tag">AI Gateway</span>
      </div>

      <div class="nav__links">
        <router-link class="nav__link" :to="{ name: 'Marketplace' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <path d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z"/>
          </svg>
          模型广场
        </router-link>
        <router-link class="nav__link" :to="{ name: 'TenantHelpCenter' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"/>
          </svg>
          文档
        </router-link>
        <a class="nav__link" href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 .5C5.73.5.5 5.73.5 12c0 5.02 3.26 9.28 7.78 10.79.57.1.78-.25.78-.55v-2.1c-3.17.69-3.84-1.35-3.84-1.35-.52-1.32-1.27-1.67-1.27-1.67-1.03-.71.08-.7.08-.7 1.14.08 1.74 1.17 1.74 1.17 1.02 1.75 2.67 1.24 3.32.95.1-.74.4-1.24.72-1.53-2.53-.29-5.2-1.27-5.2-5.63 0-1.24.44-2.26 1.17-3.06-.12-.29-.51-1.45.11-3.02 0 0 .95-.3 3.13 1.17a10.9 10.9 0 015.7 0c2.17-1.47 3.12-1.17 3.12-1.17.62 1.57.23 2.73.12 3.02.73.8 1.17 1.82 1.17 3.06 0 4.37-2.68 5.34-5.22 5.62.41.35.77 1.05.77 2.12v3.14c0 .3.2.66.79.55A11.51 11.51 0 0023.5 12C23.5 5.73 18.27.5 12 .5z"/>
          </svg>
          GitHub
        </a>
      </div>

      <div class="nav__actions">
        <button v-if="announcements.length" class="icon-btn" title="查看公告" aria-label="查看公告" @click="openAnnouncements">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0"/></svg>
        </button>
        <button class="icon-btn" title="切换明暗主题" aria-label="切换明暗主题" @click="toggleTheme">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round">
            <circle cx="12" cy="12" r="4.2"/>
            <path d="M12 2.6v2.2M12 19.2v2.2M2.6 12h2.2M19.2 12h2.2M5.4 5.4l1.6 1.6M17 17l1.6 1.6M18.6 5.4L17 7M7 17l-1.6 1.6"/>
          </svg>
        </button>
        <router-link class="btn btn--ghost" :to="{ name: 'TenantLogin' }">登录</router-link>
        <router-link class="btn btn--solid" :to="{ name: 'TenantRegister' }">
          免费开始
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 12h13M12.5 5.5L19 12l-6.5 6.5"/>
          </svg>
        </router-link>
      </div>
    </nav>

    <!-- ===================== 主区 ===================== -->
    <main class="main">

      <!-- ---------- Hero ---------- -->
      <section class="hero">
        <div class="pill">
          <span class="pill__dot"></span>
          <span>开源自托管 · AGPL-3.0</span>
          <span class="pill__sep">/</span>
          <span>五层额度 · 预算熔断</span>
        </div>

        <h1 class="title">
          <span class="title__line"><span>一个 Key，</span></span>
          <span class="title__line"><span class="title__grad">接入所有大模型</span></span>
        </h1>

        <p class="sub">
          企业级多租户 AI 网关。<b>团队统一充值、按人按项目分额、用量实时入账</b> ——
          谁花了多少、花在哪个项目，一眼看清；预算触线自动熔断，账单不再失控。
          你的代码一行都不用改。
        </p>

        <div class="cta-row">
          <router-link class="btn btn--solid btn--lg" :to="{ name: 'TenantRegister' }">
            免费开始使用
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M5 12h13M12.5 5.5L19 12l-6.5 6.5"/>
            </svg>
          </router-link>
          <router-link class="btn btn--ghost btn--lg" :to="{ name: 'TenantHelpCenter' }">查看接入文档</router-link>
          <span class="hint">无需信用卡</span>
        </div>

        <div class="snippet">
          <span class="snippet__prompt">&gt;_</span>
          <span class="snippet__code">base_url = <b>"{{ apiBaseUrl }}"</b></span>
          <button class="snippet__copy" title="复制" aria-label="复制接入地址" @click="copyBaseUrl">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <path d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 019.75 12.375v-1.5m6 7.875a3.375 3.375 0 01-3.375 3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-4.5a1.125 1.125 0 011.125-1.125h1.5a3.375 3.375 0 013.375 3.375z"/>
            </svg>
          </button>
        </div>

        <div class="stats">
          <div class="stat">
            <div class="stat__num"><span>实时</span></div>
            <div class="stat__label">用量与账单</div>
          </div>
          <div class="stat">
            <div class="stat__num"><span data-count="99.97" data-decimals="2">99.97</span><i>%</i></div>
            <div class="stat__label">网关可用性</div>
          </div>
          <div class="stat">
            <div class="stat__num"><span data-count="1" data-decimals="0">1</span><i>行</i></div>
            <div class="stat__label">代码改动量</div>
          </div>
          <div class="stat">
            <div class="stat__num"><span data-count="5" data-decimals="0">5</span><i>层</i></div>
            <div class="stat__label">额度管控模型</div>
          </div>
        </div>
      </section>

      <!-- ---------- 实时路由可视化 ---------- -->
      <section class="stage" aria-label="实时路由演示">
        <div class="stage__inner">
          <div class="panel">
            <div class="panel__scan"></div>

            <header class="panel__head">
              <div class="panel__title">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M13 3l-1.5 7.5H17l-6 10.5 1.5-7.5H7L13 3z"/>
                </svg>
                用量与路由
                <span class="tag-live"><i></i>LIVE</span>
              </div>
            </header>

            <div class="panel__body">
              <svg viewBox="0 0 900 460" preserveAspectRatio="xMidYMid meet">
                <defs>
                  <linearGradient id="litGrad" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0" stop-color="var(--teal)" stop-opacity=".12"/>
                    <stop offset=".5" stop-color="var(--cyan)"/>
                    <stop offset="1" stop-color="var(--violet)"/>
                  </linearGradient>
                  <radialGradient id="coreInner">
                    <stop offset="0" stop-color="var(--teal)" stop-opacity=".16"/>
                    <stop offset="1" stop-color="var(--violet)" stop-opacity=".05"/>
                  </radialGradient>
                  <radialGradient id="coreGlow">
                    <stop offset="0" stop-color="var(--teal)" stop-opacity=".5"/>
                    <stop offset=".5" stop-color="var(--cyan)" stop-opacity=".15"/>
                    <stop offset="1" stop-color="var(--teal)" stop-opacity="0"/>
                  </radialGradient>
                  <linearGradient id="coreFill" x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stop-color="var(--teal)"/>
                    <stop offset=".55" stop-color="var(--cyan)"/>
                    <stop offset="1" stop-color="var(--violet)"/>
                  </linearGradient>
                  <filter id="soft" x="-70%" y="-70%" width="240%" height="240%">
                    <feGaussianBlur stdDeviation="4"/>
                  </filter>
                  <filter id="softer" x="-90%" y="-90%" width="280%" height="280%">
                    <feGaussianBlur stdDeviation="10"/>
                  </filter>

                  <path id="pIn" d="M126 230 C 176 230, 206 230, 264 230"/>
                  <path id="p1" d="M396 230 C 500 230, 536 34, 638 34"/>
                  <path id="p2" d="M396 230 C 500 230, 540 132, 638 132"/>
                  <path id="p3" d="M396 230 C 500 230, 540 230, 638 230"/>
                  <path id="p4" d="M396 230 C 500 230, 540 328, 638 328"/>
                  <path id="p5" d="M396 230 C 500 230, 536 426, 638 426"/>
                </defs>

                <!-- 连接线 -->
                <use href="#pIn" class="link-path"/>
                <use href="#pIn" class="link-path link-path--lit"/>
                <use href="#p1" class="link-path"/>
                <use href="#p2" class="link-path"/>
                <use href="#p3" class="link-path"/>
                <use href="#p4" class="link-path"/>
                <use href="#p5" class="link-path"/>
                <use href="#p1" class="link-path link-path--lit" style="animation-delay:-.25s"/>
                <use href="#p2" class="link-path link-path--lit" style="animation-delay:-.9s"/>
                <use href="#p4" class="link-path link-path--lit" style="animation-delay:-1.35s"/>
                <use href="#p5" class="link-path link-path--lit" style="animation-delay:-.6s"/>

                <!-- 调用方 -->
                <g>
                  <rect x="16" y="196" width="110" height="68" rx="16" fill="var(--node-bg)" stroke="var(--node-line)" stroke-width="var(--stroke-w)"/>
                  <text x="71" y="223" text-anchor="middle" class="app-label">YOUR APP</text>
                  <text x="71" y="242" text-anchor="middle" class="core__sub" style="font-size:9px">OpenAI SDK</text>
                </g>

                <!-- 管控连线：球顶向上指向仪表盘。
                     信息已上移到 HTML 仪表盘行，这里只留一条指向它的虚线。 -->
                <path d="M330 118 L330 0" class="link-dashed"/>

                <!-- 中心：产品本体。不放抽象的 "Gateway"，直接放品牌 mark + 名字 -->
                <g>
                  <circle cx="330" cy="230" r="134" fill="url(#coreGlow)" class="core__glow"/>
                  <circle cx="330" cy="230" r="84" fill="none" stroke="var(--panel-line)" stroke-width="1"/>
                  <circle cx="330" cy="230" r="98" fill="none" stroke="var(--teal)" stroke-width="1.3"
                          stroke-dasharray="3 11" opacity=".55" class="core__ring-a"/>
                  <circle cx="330" cy="230" r="112" fill="none" stroke="var(--violet)" stroke-width="1"
                          stroke-dasharray="1 15" opacity=".4" class="core__ring-b"/>

                  <!-- 主体圆：内圈渐变底 + 本体 + 外描边发光 -->
                  <circle cx="330" cy="230" r="72" fill="var(--node-bg)"/>
                  <circle cx="330" cy="230" r="72" fill="url(#coreInner)"/>
                  <circle cx="330" cy="230" r="72" fill="none" stroke="url(#coreFill)" stroke-width="1.8"/>
                  <circle cx="330" cy="230" r="72" fill="none" stroke="url(#coreFill)" stroke-width="9" opacity=".2" filter="url(#soft)"/>

                  <!-- 品牌 mark：与顶部导航 logo 同一套语言 -->
                  <rect x="316" y="194" width="28" height="28" rx="9" fill="url(#coreFill)"/>
                  <path d="M330 201.5v13M323.5 208h13" stroke="#fff" stroke-width="2.2" stroke-linecap="round"/>

                  <text x="330" y="248" text-anchor="middle" class="core__label">Team-API</text>
                  <text x="330" y="266" text-anchor="middle" class="core__sub">AI GATEWAY</text>
                </g>

                <!-- 模型节点 -->
                <g class="sup-node">
                  <rect x="638" y="4" width="246" height="60" rx="14"/>
                  <circle cx="664" cy="34" r="4.5" fill="var(--teal)" class="sup-dot"/>
                  <text x="684" y="25" class="sup-name">GPT-4o</text>
                  <text x="684" y="45" class="sup-sub">OpenAI</text>
                  <text x="866" y="25" text-anchor="end" class="sup-lat">112ms</text>
                  <text x="866" y="45" text-anchor="end" class="sup-sub">健康 98</text>
                </g>
                <g class="sup-node">
                  <rect x="638" y="102" width="246" height="60" rx="14"/>
                  <circle cx="664" cy="132" r="4.5" fill="var(--teal)" class="sup-dot" style="animation-delay:.4s"/>
                  <text x="684" y="123" class="sup-name">Claude Sonnet</text>
                  <text x="684" y="143" class="sup-sub">Anthropic</text>
                  <text x="866" y="123" text-anchor="end" class="sup-lat">204ms</text>
                  <text x="866" y="143" text-anchor="end" class="sup-sub">健康 96</text>
                </g>
                <g class="sup-node">
                  <rect x="638" y="200" width="246" height="60" rx="14"/>
                  <circle cx="664" cy="230" r="4.5" fill="var(--teal)" class="sup-dot" style="animation-delay:.8s"/>
                  <text x="684" y="221" class="sup-name">Gemini 2.5 Pro</text>
                  <text x="684" y="241" class="sup-sub">Google</text>
                  <text x="866" y="221" text-anchor="end" class="sup-lat">163ms</text>
                  <text x="866" y="241" text-anchor="end" class="sup-sub">健康 97</text>
                </g>
                <g class="sup-node sup-node--warn">
                  <rect x="638" y="298" width="246" height="60" rx="14"/>
                  <circle cx="664" cy="328" r="4.5" fill="var(--amber)" class="sup-dot" style="animation-delay:1.2s"/>
                  <text x="684" y="319" class="sup-name">DeepSeek V3</text>
                  <text x="684" y="339" class="sup-sub">DeepSeek · 已降权</text>
                  <text x="866" y="319" text-anchor="end" class="sup-lat">1 280ms</text>
                  <text x="866" y="339" text-anchor="end" class="sup-sub">健康 71</text>
                </g>
                <g class="sup-node">
                  <rect x="638" y="396" width="246" height="60" rx="14"/>
                  <circle cx="664" cy="426" r="4.5" fill="var(--teal)" class="sup-dot" style="animation-delay:1.6s"/>
                  <text x="684" y="417" class="sup-name">Qwen-Max</text>
                  <text x="684" y="437" class="sup-sub">阿里云</text>
                  <text x="866" y="417" text-anchor="end" class="sup-lat">141ms</text>
                  <text x="866" y="437" text-anchor="end" class="sup-sub">健康 95</text>
                </g>

                <!-- 流动数据包 -->
                <circle r="4.5" fill="var(--teal)" filter="url(#soft)"><animateMotion dur="2.6s" repeatCount="indefinite"><mpath href="#pIn"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="2.6s" repeatCount="indefinite"><mpath href="#pIn"/></animateMotion></circle>

                <circle r="4.5" fill="var(--teal)" filter="url(#soft)"><animateMotion dur="3.2s" begin="0s" repeatCount="indefinite"><mpath href="#p1"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="3.2s" begin="0s" repeatCount="indefinite"><mpath href="#p1"/></animateMotion></circle>

                <circle r="4.5" fill="var(--cyan)" filter="url(#soft)"><animateMotion dur="3.6s" begin="-1.4s" repeatCount="indefinite"><mpath href="#p2"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="3.6s" begin="-1.4s" repeatCount="indefinite"><mpath href="#p2"/></animateMotion></circle>

                <circle r="4.5" fill="var(--violet)" filter="url(#soft)"><animateMotion dur="3.1s" begin="-.6s" repeatCount="indefinite"><mpath href="#p3"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="3.1s" begin="-.6s" repeatCount="indefinite"><mpath href="#p3"/></animateMotion></circle>

                <circle r="4.5" fill="var(--amber)" filter="url(#soft)"><animateMotion dur="4.2s" begin="-2.4s" repeatCount="indefinite"><mpath href="#p4"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="4.2s" begin="-2.4s" repeatCount="indefinite"><mpath href="#p4"/></animateMotion></circle>

                <circle r="4.5" fill="var(--cyan)" filter="url(#soft)"><animateMotion dur="3.8s" begin="-1s" repeatCount="indefinite"><mpath href="#p5"/></animateMotion></circle>
                <circle r="2.6" fill="#fff"><animateMotion dur="3.8s" begin="-1s" repeatCount="indefinite"><mpath href="#p5"/></animateMotion></circle>
              </svg>
            </div>

            <div class="logs" data-logs>
              <div class="log-line"><span>成员 <b>王五</b> 额度 91%</span><span class="warn">已触发预警</span></div>
              <div class="log-line"><span>预扣 {{ currencySymbol }}0.0123 → 结算 {{ currencySymbol }}0.0098</span><span class="ok">退回 {{ currencySymbol }}0.0025</span></div>
              <div class="log-line"><span>项目「客服机器人」预算 {{ currencySymbol }}86.2/{{ currencySymbol }}100</span><span class="warn">接近上限</span></div>
            </div>
          </div>

          <!-- 看板：组织 / 额度 / 管控。要求是「用虚线连接的独立框」，
               而不是铺成面板顶部的一整行 —— 所以它是浮层，不占 grid 行。 -->
    <div class="dash">
                <div class="dash__col">
                  <div class="dash__label">组织与权限</div>
                  <div class="dash__row"><span class="dot dot--rose"></span><b>Owner</b><span>全部</span></div>
                  <div class="dash__row"><span class="dot dot--violet"></span><b>Admin</b><span>计费·成员</span></div>
                  <div class="dash__row"><span class="dot dot--teal"></span><b>Member</b><span>仅调用</span></div>
                </div>

                <div class="dash__col">
                  <div class="dash__label">五层额度</div>
                  <!-- 递减横条 = 一层套一层。比用 › 连成一串准确：那是包含关系，不是流程。 -->
                  <div class="dash__quota">
                    <span>租户钱包</span><i style="--w:100%"></i>
                    <span>套餐额度</span><i style="--w:80%"></i>
                    <span>成员额度</span><i style="--w:60%"></i>
                    <span>项目预算</span><i style="--w:42%"></i>
                    <span>Key 限额</span><i style="--w:26%"></i>
                  </div>
                </div>

                <div class="dash__col">
                  <div class="dash__label">本月消耗</div>
                  <div class="dash__spend">
                    <span data-spend>{{ currencySymbol }}128.50</span><small>/ {{ currencySymbol }}200.00</small><em data-pct>64%</em>
                  </div>
                  <div class="dash__track"><span class="dash__fill"></span></div>
                  <div class="dash__meta">
                    <span>熔断 <b>0</b> 次</span>
                    <span>成员 <b>12</b> 人</span>
                  </div>
                </div>
              </div>

          <!-- 故障转移提示 -->
          <div class="toast" data-toast>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 9v3.75m0 3.75h.008M10.34 3.94L2.7 17.06A1.5 1.5 0 004 19.3h16a1.5 1.5 0 001.3-2.24L13.66 3.94a1.5 1.5 0 00-2.6 0z"/>
            </svg>
            <span>成员 赵六 本月预算超限</span>
            <em>→ 请求已熔断</em>
          </div>
        </div>
      </section>
    </main>

    <!-- ===================== 能力带 =====================
         原为「支持的供应商」名单。但「支持 N 家供应商」是同类产品的通用能力，
         不构成差异点；改成本项目的真实能力清单，重心落在团队管理与预算掌控上。 -->
    <footer class="band" aria-label="平台能力">
      <span class="band__label">Capabilities</span>
      <div class="band__viewport">
        <div class="band__track" data-band>
          <span class="vendor"><i></i>团队统一充值</span>
          <span class="vendor"><i></i>五层额度管控</span>
          <span class="vendor"><i></i>按成员分配额度</span>
          <span class="vendor"><i></i>项目预算</span>
          <span class="vendor"><i></i>Key 限额</span>
          <span class="vendor"><i></i>预算超限熔断</span>
          <span class="vendor"><i></i>并发预扣防超额</span>
          <span class="vendor"><i></i>实时计费结算</span>
          <span class="vendor"><i></i>成员用量排行</span>
          <span class="vendor"><i></i>用量明细可查</span>
          <span class="vendor"><i></i>请求日志审计</span>
          <span class="vendor"><i></i>操作审计</span>
          <span class="vendor"><i></i>RBAC 三级权限</span>
          <span class="vendor"><i></i>渠道健康监控</span>
          <span class="vendor"><i></i>多渠道告警</span>
          <span class="vendor"><i></i>用量报表导出</span>
          <span class="vendor"><i></i>OpenAI 协议兼容</span>
          <span class="vendor"><i></i>自托管部署</span>
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
import { currencySymbol } from '@/composables/useCurrency'
import { marked } from 'marked'
import request from '@/utils/request'

const { settings: publicSettings, fetchSettings } = usePublicSettings()
const siteName = computed(() => publicSettings.value.site_name || 'Team-API')
// 快速开始示例里展示的真实接入地址（取当前访问域名，OpenAI 兼容端点固定为 /v1）
const apiBaseUrl = `${window.location.origin}/v1`

const rootEl = ref<HTMLElement | null>(null)

/* ==================== 主题 ====================
   主题挂在落地页容器上（.lp[data-lp-theme]），**不写 <html>** ——
   租户端其他页面是浅色体系，不能被落地页的选择带着走。 */
type LpTheme = 'dark' | 'light'
const THEME_KEY = 'tenant_landing_theme'
const theme = ref<LpTheme>('dark')

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  try {
    localStorage.setItem(THEME_KEY, theme.value)
  } catch {
    /* 隐私模式下 localStorage 可能不可写，忽略 */
  }
}

/* ==================== 公告弹窗 ==================== */

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

/* ==================== SEO ==================== */
useSeo({
  title: `${siteName.value} — 团队统一充值、按人分额、用量实时入账 | 企业级多租户 AI 网关`,
  description:
    publicSettings.value.site_description ||
    `${siteName.value} 是开源自托管的企业级多租户 AI 网关。团队统一充值、按成员与项目分配额度、用量实时入账，预算触线自动熔断 —— 完全兼容 OpenAI SDK，一个 Key 接入所有大模型，只需改一行 base_url。`,
  siteName: siteName.value,
  keywords:
    'Team-API, 大模型网关, API Gateway, 多租户, 额度管控, 预算熔断, 团队管理, 用量计费, RBAC 权限, 请求审计, 操作审计, 开源, 自托管, OpenAI 兼容, 渠道调度, SSE 流式',
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
        description:
          publicSettings.value.site_description ||
          '企业级多租户 AI 网关：团队统一充值、按人按项目分额、用量实时入账、预算超限自动熔断，完全兼容 OpenAI SDK。',
        url: 'https://github.com/qianfree/team-api',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Linux, macOS, Windows',
        offers: {
          '@type': 'Offer',
          price: '0',
          priceCurrency: 'USD',
          description: '社区版开源免费，采用 AGPL-3.0-or-later 协议',
        },
        featureList: [
          '团队与额度管理（租户 → 套餐 → 成员 → 项目 → Key 五层）',
          '预算管控（超限自动熔断、并发预扣防超额）',
          '实时计费引擎（预扣 → 结算 → 退款）',
          'RBAC 三级权限与全链路审计',
          '统一网关（一个 Key 接入所有大模型，兼容 OpenAI SDK）',
          '智能渠道调度（优先级 / 权重 / 健康评分）',
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
        description: publicSettings.value.site_description || '企业级多租户 AI 网关平台',
        potentialAction: {
          '@type': 'SearchAction',
          target: 'https://github.com/qianfree/team-api/search?q={search_term_string}',
          'query-input': 'required name=search_term_string',
        },
      }),
    },
  ],
})

/* ==================== 面板上的动态演示 ====================
   纯装饰性的实时效果：日志滚动、消耗累加、故障熔断提示、能力跑马灯。
   定时器统一收集，卸载时清理，避免路由切走后还在跑。 */
const timers: number[] = []
let disposed = false

function setupPanelFx() {
  const root = rootEl.value
  if (!root) return
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (reduce) return

  /* 指标数字递增 */
  root.querySelectorAll<HTMLElement>('[data-count]').forEach((el) => {
    const target = parseFloat(el.getAttribute('data-count') || '0')
    const decimals = parseInt(el.getAttribute('data-decimals') || '0', 10)
    let start: number | null = null
    const duration = 1400
    const step = (ts: number) => {
      if (disposed) return
      if (start === null) start = ts
      const p = Math.min((ts - start) / duration, 1)
      el.textContent = (target * (1 - Math.pow(1 - p, 3))).toFixed(decimals)
      if (p < 1) requestAnimationFrame(step)
    }
    window.setTimeout(() => requestAnimationFrame(step), 660)
  })

  /* 日志滚动：口径围绕成员额度 / 项目预算 / 熔断 */
  const logPool = [
    '<span>成员 <b>王五</b> 额度 91%</span><span class="warn">已触发预警</span>',
    `<span>预扣 ${currencySymbol.value}0.0310 → 结算 ${currencySymbol.value}0.0287</span><span class="ok">退回 ${currencySymbol.value}0.0023</span>`,
    `<span>项目「客服机器人」预算 ${currencySymbol.value}86.2/${currencySymbol.value}100</span><span class="warn">接近上限</span>`,
    `<span>Owner 为 <b>李四</b> 追加额度</span><span class="ok">+${currencySymbol.value}50.00</span>`,
    '<b>POST /v1/chat/completions</b><span class="ok">→ claude-sonnet-4</span>',
    '<span>Key「prod-01」本月用量 85%</span><span class="warn">预警</span>',
    '<span>成员 <b>赵六</b> 预算超限</span><span class="warn">请求已熔断</span>',
    `<span>9 月用量报表已导出</span><span class="ok">CSV · 1 284 行</span>`,
    `<span>套餐额度已重置</span><span class="ok">团队 ${currencySymbol.value}2.00 万</span>`,
    '<span>RBAC：<b>Admin</b> 修改成员额度</span><span class="ok">已记录审计</span>',
  ]
  const logs = root.querySelector<HTMLElement>('[data-logs]')
  let li = 0
  if (logs) {
    timers.push(
      window.setInterval(() => {
        const el = document.createElement('div')
        el.className = 'log-line is-enter'
        el.innerHTML = logPool[li % logPool.length]
        li++
        logs.appendChild(el)
        while (logs.children.length > 4 && logs.firstElementChild) {
          logs.removeChild(logs.firstElementChild)
        }
        requestAnimationFrame(() => requestAnimationFrame(() => el.classList.remove('is-enter')))
      }, 2200),
    )
  }

  /* 本月消耗实时累加：金额 / 百分比 / 进度条三处联动，让「实时计费」可见 */
  const spendEl = root.querySelector<HTMLElement>('[data-spend]')
  const pctEl = root.querySelector<HTMLElement>('[data-pct]')
  const fillEl = root.querySelector<HTMLElement>('.dash__fill')
  if (spendEl) {
    let spent = 128.5
    const budget = 200
    timers.push(
      window.setInterval(() => {
        spent += Math.random() * 0.05
        const pct = (spent / budget) * 100
        spendEl.textContent = `${currencySymbol.value}${spent.toFixed(2)}`
        if (pctEl) pctEl.textContent = `${pct.toFixed(0)}%`
        if (fillEl) fillEl.style.width = `${pct.toFixed(1)}%`
      }, 2000),
    )
  }

  /* 熔断提示轮播 */
  const toast = root.querySelector<HTMLElement>('[data-toast]')
  if (toast) {
    const loopToast = () => {
      const t = window.setTimeout(() => {
        if (disposed) return
        toast.classList.add('is-on')
        const t2 = window.setTimeout(() => {
          if (disposed) return
          toast.classList.remove('is-on')
          loopToast()
        }, 4200)
        timers.push(t2)
      }, 3600)
      timers.push(t)
    }
    loopToast()
  }

  /* 能力跑马灯：复制一份实现无缝滚动 */
  const track = root.querySelector<HTMLElement>('[data-band]')
  if (track) track.innerHTML += track.innerHTML
}

/* 接入地址一键复制 */
const copied = ref(false)
async function copyBaseUrl() {
  try {
    await navigator.clipboard.writeText(apiBaseUrl)
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1400)
  } catch {
    /* 非安全上下文下 clipboard 不可用，静默失败 */
  }
}

onMounted(async () => {
  try {
    const saved = localStorage.getItem(THEME_KEY)
    if (saved === 'light' || saved === 'dark') theme.value = saved
  } catch { /* ignore */ }

  await fetchSettings()
  fetchAndShowAnnouncement()
  setupPanelFx()
})

onBeforeUnmount(() => {
  disposed = true
  timers.forEach((t) => window.clearInterval(t))
  timers.length = 0
})
</script>


<style scoped>
/* ==========================================================================
   0. Design Tokens
   ========================================================================== */
.lp {
  --teal: #2dd4bf;
  --cyan: #22d3ee;
  --violet: #8b5cf6;
  --amber: #fbbf24;
  --rose: #fb7185;

  --font-sans: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto,
    "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  --font-mono: ui-monospace, "SF Mono", "JetBrains Mono", "Cascadia Code",
    Consolas, "Liberation Mono", monospace;

  --ease-out: cubic-bezier(.16, 1, .3, 1);
  --ease-io: cubic-bezier(.65, 0, .35, 1);
  --shell-w: 1760px;
  --nav-h: 70px;
  --band-h: 76px;
}

/* --- Dark（默认） --- */
.lp[data-lp-theme="dark"] {
  --bg: #04060d;
  --text: #eef2fa;
  --text-dim: #9aa6c2;
  --text-mute: #5e6a87;

  --panel: rgba(255, 255, 255, .035);
  --panel-solid: rgba(9, 13, 25, .88);
  --panel-line: rgba(255, 255, 255, .09);
  --panel-line-soft: rgba(255, 255, 255, .055);
  --node-bg: rgba(13, 19, 34, .94);
  --node-line: rgba(255, 255, 255, .11);

  --grid: rgba(120, 165, 255, .05);
  --glow-a: 45, 212, 191;
  --glow-b: 139, 92, 246;
  --glow-c: 34, 211, 238;
  --glow-strength: 1.4;
  --noise: .04;
  --noise-blend: overlay;
  --scan-o: .2;
  --title-glow: 0 0 30px rgba(45, 212, 191, .32);
  --core-glow-mul: 1;

  --board-bg: rgba(12, 18, 32, .95);
  --btn-solid-text: #032019;
  --shadow-panel:
    0 64px 160px -70px rgba(var(--glow-a), .62),
    0 26px 80px -42px rgba(0, 0, 0, .74),
    inset 0 0 0 1px rgba(255, 255, 255, .06);
  --stroke-w: 1.2;
}

/* --- Light --- */
.lp[data-lp-theme="light"] {
  --bg: #eef3fa;
  --text: #08101f;
  --text-dim: #46516c;
  --text-mute: #8791a9;

  --teal: #0d9488;
  --cyan: #0891b2;
  --violet: #7c3aed;

  --panel: rgba(255, 255, 255, .82);
  --panel-solid: rgba(255, 255, 255, .97);
  --panel-line: rgba(13, 32, 76, .16);
  --panel-line-soft: rgba(13, 32, 76, .09);
  --node-bg: #ffffff;
  --node-line: rgba(13, 32, 76, .16);

  --grid: rgba(30, 70, 150, .07);
  --glow-a: 13, 148, 136;
  --glow-b: 124, 58, 237;
  --glow-c: 8, 145, 178;
  --glow-strength: .36;   /* 浅底上大面积染色会显脏，压低 */
  --noise: .018;
  --noise-blend: multiply;
  --scan-o: .075;         /* 扫描带在浅底上只要一丝痕迹 */
  --title-glow: none;     /* 浅底上青色 drop-shadow 会发灰，关掉 */
  --core-glow-mul: .55;

  --board-bg: rgba(255, 255, 255, .98);
  --btn-solid-text: #ffffff;
  --shadow-panel:
    0 46px 110px -52px rgba(13, 148, 136, .34),
    0 14px 40px -24px rgba(20, 45, 96, .2),
    inset 0 0 0 1px rgba(13, 32, 76, .085);
  --stroke-w: 1;
}

/* ==========================================================================
   1. Base
   ========================================================================== */
/* 只在落地页容器内重置，不动全局 —— 租户端其他页面是浅色体系 */
.lp *, .lp *::before, .lp *::after {
  box-sizing: border-box; margin: 0; padding: 0;
}

a { color: inherit; text-decoration: none; }
button { font: inherit; color: inherit; background: none; border: 0; cursor: pointer; }

.lp {
  position: relative;
  height: 100dvh;            /* 一屏，不滚动 —— 只作用在落地页自己身上 */
  min-height: 560px;
  margin: 0;
  display: grid;
  grid-template-rows: var(--nav-h) minmax(0, 1fr) var(--band-h);
  isolation: isolate;
  font-family: var(--font-sans);
  background: var(--bg);
  color: var(--text);
  overflow: hidden;
  -webkit-font-smoothing: antialiased;
  transition: background .5s var(--ease-io), color .5s var(--ease-io);
}

/* ==========================================================================
   2. Background
   ========================================================================== */
.bg { position: absolute; inset: 0; z-index: 0; pointer-events: none; overflow: hidden; }

.bg__wash {
  position: absolute; inset: -22%;
  background:
    radial-gradient(58% 50% at 14% 12%, rgba(var(--glow-a), calc(.24 * var(--glow-strength))) 0%, transparent 64%),
    radial-gradient(52% 46% at 88% 18%, rgba(var(--glow-b), calc(.2 * var(--glow-strength))) 0%, transparent 66%),
    radial-gradient(64% 52% at 60% 100%, rgba(var(--glow-c), calc(.16 * var(--glow-strength))) 0%, transparent 68%);
  filter: blur(10px);
  animation: washDrift 26s var(--ease-io) infinite alternate;
}
@keyframes washDrift {
  from { transform: translate3d(-1.2%, -1%, 0) scale(1); }
  to   { transform: translate3d(1.6%, 1.4%, 0) scale(1.07); }
}

.bg__grid {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(var(--grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--grid) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(circle at 52% 44%, #000 0%, transparent 80%);
  -webkit-mask-image: radial-gradient(circle at 52% 44%, #000 0%, transparent 80%);
  animation: gridPan 40s linear infinite;
}
@keyframes gridPan { to { background-position: 56px 56px, 56px 56px; } }

/* 噪点：消除大片纯色的塑料感 */
.bg__noise {
  position: absolute; inset: 0;
  opacity: var(--noise);
  mix-blend-mode: var(--noise-blend);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.82' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='180' height='180' filter='url(%23n)'/%3E%3C/svg%3E");
}

.bg__ring { position: absolute; border-radius: 50%; left: 50%; top: 44%; }
.bg__ring--1 {
  width: 1240px; height: 1240px; margin: -620px 0 0 -620px;
  border: 1px dashed rgba(var(--glow-a), .17);
  animation: spin 130s linear infinite;
}
.bg__ring--2 {
  width: 1680px; height: 1680px; margin: -840px 0 0 -840px;
  border: 1px solid rgba(var(--glow-b), .12);
  animation: spin 210s linear infinite reverse;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ==========================================================================
   3. Navigation
   ========================================================================== */
.nav {
  position: relative; z-index: 5;
  width: 100%; max-width: var(--shell-w); margin: 0 auto;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 clamp(20px, 4vw, 60px);
}

.brand { display: flex; align-items: center; gap: 12px; }
.brand__mark {
  position: relative; width: 36px; height: 36px; border-radius: 11px;
  display: grid; place-items: center;
  background: linear-gradient(140deg, var(--teal), var(--cyan) 55%, var(--violet));
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, .16), 0 10px 28px -8px rgba(var(--glow-a), .8);
}
.brand__mark svg { width: 19px; height: 19px; }
.brand__mark::after {
  content: ""; position: absolute; inset: -7px; border-radius: 18px;
  border: 1px solid rgba(var(--glow-a), .3);
  animation: markPulse 3.4s var(--ease-io) infinite;
}
@keyframes markPulse {
  0%, 100% { opacity: .5; transform: scale(.94); }
  50%      { opacity: 0; transform: scale(1.16); }
}
.brand__name { font-size: 17px; font-weight: 700; letter-spacing: -.02em; }
.brand__tag {
  font-family: var(--font-mono); font-size: 10px; letter-spacing: .12em; text-transform: uppercase;
  color: var(--text-mute); padding-left: 12px; margin-left: 2px;
  border-left: 1px solid var(--panel-line);
}

.nav__links { display: flex; align-items: center; gap: 4px; }
.nav__link {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 13px; border-radius: 9px;
  font-size: 13.5px; color: var(--text-dim);
  transition: color .2s, background .2s;
}
.nav__link:hover { color: var(--text); background: var(--panel); }
.nav__link svg { width: 14px; height: 14px; opacity: .75; }

.nav__actions { display: flex; align-items: center; gap: 10px; }
.icon-btn {
  width: 34px; height: 34px; border-radius: 10px;
  display: grid; place-items: center;
  border: 1px solid var(--panel-line); color: var(--text-dim);
  transition: color .2s, border-color .2s, background .2s, transform .3s var(--ease-out);
}
.icon-btn:hover { color: var(--text); background: var(--panel); border-color: rgba(var(--glow-a), .45); transform: rotate(22deg); }
.icon-btn svg { width: 16px; height: 16px; }

.btn {
  position: relative; overflow: hidden;
  display: inline-flex; align-items: center; justify-content: center; gap: 7px;
  padding: 9px 19px; border-radius: 10px;
  font-size: 13.5px; font-weight: 600; letter-spacing: -.01em; white-space: nowrap;
  transition: transform .25s var(--ease-out), box-shadow .25s, background .25s, border-color .25s;
}
.btn svg { width: 15px; height: 15px; }
.btn--ghost { color: var(--text-dim); border: 1px solid var(--panel-line); }
.btn--ghost:hover { color: var(--text); border-color: rgba(var(--glow-a), .45); background: var(--panel); }
.btn--solid {
  color: var(--btn-solid-text);
  background: linear-gradient(115deg, var(--teal), var(--cyan));
  box-shadow: 0 10px 32px -13px rgba(var(--glow-a), .95), inset 0 0 0 1px rgba(255, 255, 255, .16);
}
.btn--solid:hover { transform: translateY(-1.5px); box-shadow: 0 20px 46px -16px rgba(var(--glow-a), 1); }
.btn--solid::after {
  content: ""; position: absolute; top: 0; bottom: 0; width: 40%;
  background: linear-gradient(100deg, transparent, rgba(255, 255, 255, .6), transparent);
  transform: translateX(-170%) skewX(-18deg);
  animation: btnSheen 4.8s var(--ease-io) infinite;
}
@keyframes btnSheen {
  0%, 56% { transform: translateX(-170%) skewX(-18deg); }
  78%, 100% { transform: translateX(340%) skewX(-18deg); }
}
.btn--lg { padding: 15px 28px; font-size: 15px; border-radius: 12px; }
.btn--lg svg { width: 17px; height: 17px; }

/* ==========================================================================
   4. Main
   ========================================================================== */
.main {
  position: relative; z-index: 3;
  width: 100%; max-width: var(--shell-w); margin: 0 auto;
  padding: 0 clamp(20px, 4vw, 60px);
  display: grid;
  grid-template-columns: minmax(0, .9fr) minmax(0, 1.1fr);
  gap: clamp(26px, 3.4vw, 66px);
  align-items: center;
  min-height: 0;
}

/* ---------- 4.1 Hero ---------- */
.hero { display: flex; flex-direction: column; gap: clamp(16px, 2.3vh, 28px); min-width: 0; }

.pill {
  display: inline-flex; align-items: center; gap: 9px; align-self: flex-start;
  padding: 7px 14px 7px 12px; border-radius: 999px;
  background: var(--panel); border: 1px solid var(--panel-line);
  backdrop-filter: blur(10px);
  font-size: 12.5px; color: var(--text-dim);
  animation: riseIn .8s var(--ease-out) backwards;
}
.pill__dot {
  width: 6px; height: 6px; border-radius: 50%; background: var(--teal);
  animation: liveDot 2.2s var(--ease-io) infinite;
}
@keyframes liveDot {
  0%   { box-shadow: 0 0 0 0 rgba(var(--glow-a), .75); }
  70%  { box-shadow: 0 0 0 9px rgba(var(--glow-a), 0); }
  100% { box-shadow: 0 0 0 0 rgba(var(--glow-a), 0); }
}
.pill__sep { color: var(--text-mute); opacity: .5; }

.title {
  font-size: clamp(40px, min(5vw, 7.6vh), 84px);
  font-weight: 800;
  line-height: 1.05;
  letter-spacing: -.04em;
}
.title__line { display: block; overflow: hidden; padding-bottom: .08em; }
.title__line > span { display: inline-block; animation: titleRise 1.05s var(--ease-out) backwards; }
.title__line:nth-child(1) > span { animation-delay: .1s; }
.title__line:nth-child(2) > span { animation-delay: .22s; }
@keyframes titleRise {
  from { transform: translateY(106%); opacity: 0; }
  to   { transform: translateY(0); opacity: 1; }
}
.title__grad {
  background: linear-gradient(96deg, var(--teal) 0%, var(--cyan) 32%, var(--violet) 66%, var(--teal) 100%);
  background-size: 280% 100%;
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent; color: transparent;
  animation: titleRise 1.05s var(--ease-out) .22s backwards, gradFlow 9s linear infinite;
  filter: drop-shadow(var(--title-glow));
}
@keyframes gradFlow { to { background-position: 280% 0; } }

.sub {
  max-width: 31em;
  font-size: clamp(14px, 1.12vw, 17px);
  line-height: 1.78;
  color: var(--text-dim);
  animation: riseIn .9s var(--ease-out) .34s backwards;
}
.sub b { color: var(--text); font-weight: 600; }

.cta-row {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  animation: riseIn .9s var(--ease-out) .44s backwards;
}
.cta-row .hint { font-family: var(--font-mono); font-size: 11px; color: var(--text-mute); margin-left: 4px; }

/* 一行接入代码条 */
.snippet {
  display: flex; align-items: center; gap: 12px;
  align-self: flex-start; max-width: 100%;
  padding: 11px 14px; border-radius: 12px;
  background: var(--panel); border: 1px solid var(--panel-line);
  backdrop-filter: blur(10px);
  font-family: var(--font-mono); font-size: 12.5px;
  animation: riseIn .9s var(--ease-out) .52s backwards;
}
.snippet__prompt { color: var(--teal); flex: none; }
.snippet__code { color: var(--text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.snippet__code b { color: var(--text); font-weight: 500; }
.snippet__copy {
  flex: none; display: grid; place-items: center;
  width: 24px; height: 24px; border-radius: 7px;
  color: var(--text-mute);
  transition: color .2s, background .2s;
}
.snippet__copy:hover { color: var(--teal); background: rgba(var(--glow-a), .1); }
.snippet__copy svg { width: 14px; height: 14px; }

.stats {
  display: grid; grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  margin-top: clamp(2px, 1vh, 12px);
  padding-top: clamp(16px, 2.4vh, 26px);
  border-top: 1px solid var(--panel-line-soft);
  animation: riseIn .9s var(--ease-out) .62s backwards;
}
.stat { position: relative; padding-right: 10px; }
.stat + .stat { padding-left: clamp(12px, 1.4vw, 24px); }
.stat + .stat::before {
  content: ""; position: absolute; left: 0; top: 5px; bottom: 5px; width: 1px;
  background: var(--panel-line-soft);
}
.stat__num {
  font-family: var(--font-mono);
  font-size: clamp(20px, 1.95vw, 29px);
  font-weight: 600; letter-spacing: -.035em; color: var(--text);
  display: flex; align-items: baseline; gap: 2px;
}
.stat__num i { font-style: normal; font-size: .6em; color: var(--teal); }
.stat__label { margin-top: 6px; font-size: 12px; color: var(--text-mute); white-space: nowrap; }

@keyframes riseIn {
  from { opacity: 0; transform: translateY(15px); }
  to   { opacity: 1; transform: translateY(0); }
}

/* ---------- 4.2 Stage ---------- */
.stage {
  position: relative;
  height: min(100%, 640px);
  min-height: 400px;
}
.stage__inner {
  position: relative; height: 100%;
  animation: stageIn 1.15s var(--ease-out) .18s backwards;
}
@keyframes stageIn {
  from { opacity: 0; transform: translateY(28px) scale(.965); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}

.panel {
  position: relative; height: 100%;
  display: grid; grid-template-rows: auto minmax(0, 1fr);
  border-radius: 26px;
  background:
    radial-gradient(120% 84% at 46% 2%, rgba(var(--glow-a), calc(.09 * var(--glow-strength))), transparent 64%),
    radial-gradient(92% 72% at 86% 102%, rgba(var(--glow-b), calc(.075 * var(--glow-strength))), transparent 68%),
    var(--panel);
  /* 不用 1px 实边框：改为内描边 + 彩色外辉光，让面板「浮在光里」，
     不再被一圈线框住（反馈：大卡片与整体有点突兀）。 */
  box-shadow: var(--shadow-panel);
  overflow: hidden;
}
/* 面板内部铺一层与页面同尺寸(56px)的网格 —— 让面板读起来像页面背景的延伸，而非贴上去的板子 */
.panel::after {
  content: ""; position: absolute; inset: 0; z-index: 0; pointer-events: none;
  background-image:
    linear-gradient(var(--grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--grid) 1px, transparent 1px);
  background-size: 56px 56px;
  opacity: .7;
  /* 中心下移：网格在 head + 仪表盘区域淡出，不去干扰那里的文字 */
  mask-image: radial-gradient(120% 78% at 46% 64%, #000 16%, transparent 76%);
  -webkit-mask-image: radial-gradient(120% 78% at 46% 64%, #000 16%, transparent 76%);
}
/* 注意：此处**故意不用** backdrop-filter。
   Chrome 对大面积 backdrop-filter 采样会按 tile 分块，背后又是大范围渐变光晕时，
   面板中部会浮现一块矩形亮斑（已在 1366×768 实测复现，去掉后消失）。
   玻璃感改由「渐变内底 + 1px 描边 + 顶部高光边」承担，观感基本无损，性能更好。 */
.panel::before {
  content: ""; position: absolute; inset: 0 0 auto 0; height: 1px;
  background: linear-gradient(90deg, transparent, rgba(var(--glow-a), .75), rgba(var(--glow-b), .6), transparent);
}

/* 扫描带：随面板高度自适应 */
.panel__scan {
  position: absolute; inset: 0; pointer-events: none; z-index: 1;
  background: linear-gradient(180deg,
    transparent 0%, transparent 43.6%,
    rgba(var(--glow-c), var(--scan-o)) 49.7%, rgba(var(--glow-c), var(--scan-o)) 50.3%,
    transparent 56.4%, transparent 100%);
  background-size: 100% 260%;
  background-repeat: no-repeat;
  opacity: 0;
  animation: scanSweep 9.5s var(--ease-io) infinite;
}
@keyframes scanSweep {
  0%        { background-position: 0 -135%; opacity: 0; }
  10%, 86%  { opacity: 1; }
  100%      { background-position: 0 135%; opacity: 0; }
}

.panel__head {
  position: relative; z-index: 2;
  display: flex; align-items: center; justify-content: space-between; gap: 14px;
  padding: 14px 20px 12px;
  background: linear-gradient(180deg, var(--panel-solid), transparent);
}
.panel__title { display: flex; align-items: center; gap: 9px; font-size: 13.5px; font-weight: 600; }
.panel__title svg { width: 16px; height: 16px; color: var(--teal); }
.tag-live {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 3px 9px; border-radius: 999px;
  font-family: var(--font-mono); font-size: 10px; letter-spacing: .08em;
  color: var(--teal); background: rgba(var(--glow-a), .1);
  border: 1px solid rgba(var(--glow-a), .3);
}
.tag-live i { width: 5px; height: 5px; border-radius: 50%; background: var(--teal); animation: liveDot 2s var(--ease-io) infinite; }

.panel__body { position: relative; min-height: 0; z-index: 2; }
.panel__body svg { display: block; width: 100%; height: 100%; }

/* SVG 元素 */
.link-path { fill: none; stroke: var(--panel-line); stroke-width: var(--stroke-w); }
.link-path--lit {
  stroke: url(#litGrad); stroke-width: 1.5;
  stroke-dasharray: 5 13;
  animation: dashFlow 1.6s linear infinite;
}
@keyframes dashFlow { to { stroke-dashoffset: -36; } }

.sup-node rect { fill: var(--node-bg); stroke: var(--node-line); stroke-width: var(--stroke-w); }
.sup-name { font-family: var(--font-sans); font-size: 13.5px; font-weight: 600; fill: var(--text); }
.sup-sub { font-family: var(--font-mono); font-size: 11px; fill: var(--text-mute); }
.sup-lat { font-family: var(--font-mono); font-size: 11.5px; fill: var(--teal); }
.sup-dot { transform-box: fill-box; transform-origin: center; animation: supBlink 2.8s var(--ease-io) infinite; }
@keyframes supBlink { 0%, 100% { opacity: 1; } 50% { opacity: .35; } }
.sup-node--warn rect { stroke: rgba(251, 191, 36, .5); }
.sup-node--warn .sup-lat { fill: var(--amber); }
.sup-node--warn .sup-sub { fill: var(--amber); opacity: .85; }

/* 管控连线：虚线缓慢流动 —— 比数据流(1.6s)慢，读作「持续校验」而非「传数据」 */
.link-dashed {
  fill: none; stroke: var(--teal); stroke-width: 1.5;
  stroke-dasharray: 4 7; opacity: .62;
  animation: dashFlowSlow 3.4s linear infinite;
}
@keyframes dashFlowSlow { to { stroke-dashoffset: -22; } }

.core__ring-a { transform-origin: 330px 230px; animation: spin 8s linear infinite; }
.core__ring-b { transform-origin: 330px 230px; animation: spin 15s linear infinite reverse; }
.core__glow { transform-origin: 330px 230px; filter: opacity(var(--core-glow-mul)); animation: corePulse 4.2s var(--ease-io) infinite; }
@keyframes corePulse {
  0%, 100% { opacity: .5; transform: scale(1); }
  50%      { opacity: 1; transform: scale(1.08); }
}
.core__label { font-family: var(--font-sans); font-size: 14.5px; font-weight: 700; fill: var(--text); letter-spacing: -.025em; }
.core__sub { font-family: var(--font-mono); font-size: 9.5px; fill: var(--text-mute); letter-spacing: .12em; }
.app-label { font-family: var(--font-mono); font-size: 11.5px; font-weight: 600; fill: var(--text); letter-spacing: .04em; }

/* 日志浮层：只占左侧 ~2/3，右侧留给模型节点，避免遮挡 */
.logs {
  position: absolute; left: 0; bottom: 0; z-index: 3;
  width: 68%; height: 116px;
  padding: 24px 26px 15px 22px;
  display: flex; flex-direction: column; justify-content: flex-end; gap: 5px;
  overflow: hidden;
  pointer-events: none;
  background: linear-gradient(180deg, transparent 0%, var(--panel-solid) 44%, var(--panel-solid) 100%);
  mask-image: linear-gradient(90deg, #000 0%, #000 84%, transparent 100%);
  -webkit-mask-image: linear-gradient(90deg, #000 0%, #000 84%, transparent 100%);
  font-family: var(--font-mono); font-size: 11.5px; line-height: 1.55;
}
.logs::before {
  content: "request log"; position: absolute; left: 22px; top: 8px;
  font-family: var(--font-mono); font-size: 9.5px; letter-spacing: .14em;
  color: var(--text-mute); opacity: .7; text-transform: uppercase;
}
.log-line {
  display: flex; align-items: center; gap: 9px;
  color: var(--text-mute); white-space: nowrap;
  transition: opacity .4s, transform .4s var(--ease-out);
}
.log-line.is-enter { opacity: 0; transform: translateY(12px); }
.log-line b { color: var(--text-dim); font-weight: 500; }
.log-line .ok { color: var(--teal); }
.log-line .warn { color: var(--amber); }

/* ==========================================================================
   看板：组织 / 额度 / 管控。一块被虚线牵在网关正上方的独立板子。
   —— 不占 grid 行：铺成整行会读成「面板的标题栏」，而不是「连在网关上的看板」。
   left 取球心 330/900 = 36.7%，与 SVG 里那条虚线对齐。
   ========================================================================== */
.dash {
  position: absolute; z-index: 4;
  left: 36.7%; top: 56px;
  transform: translateX(-50%);
  width: min(520px, 68%);
  display: grid;
  /* 第三列内容最长（金额 + 进度条 + 两个小指标），给它足够宽度避免换行把看板撑高 */
  grid-template-columns: .8fr 1.25fr 1.2fr;
  border-radius: 16px;
  background: var(--board-bg);
  border: 1px solid var(--node-line);
  box-shadow: 0 22px 60px -26px rgba(0, 0, 0, .8);
}
.dash__col { position: relative; padding: 9px 12px 10px; min-width: 0; }
.dash__col:first-child { padding-left: 16px; }
.dash__col:last-child { padding-right: 16px; }
.dash__col + .dash__col::before {
  content: ""; position: absolute; left: 0; top: 9px; bottom: 9px; width: 1px;
  background: var(--panel-line-soft);
}
.dash__label {
  font-family: var(--font-mono); font-size: 9px; letter-spacing: .09em;
  color: var(--text-mute); margin-bottom: 7px; text-transform: uppercase;
}

/* 列1 · 组织与权限 */
.dash__row { display: flex; align-items: center; gap: 6px; font-size: 10.5px; margin-top: 4px; }
.dash__row:first-of-type { margin-top: 0; }
.dash__row b {
  font-family: var(--font-mono); font-size: 10.5px; font-weight: 600;
  color: var(--text); min-width: 42px;
}
.dash__row > span:last-child {
  color: var(--text-mute); font-size: 10px;
  white-space: nowrap;   /* 换行会撑高看板 → 挤到球顶，宁可让它自己收窄 */
}
.dot { width: 5px; height: 5px; border-radius: 50%; flex: none; }
.dot--rose { background: var(--rose); }
.dot--violet { background: var(--violet); }
.dot--teal { background: var(--teal); }

/* 列2 · 五层额度
   递减横条把「一层套一层」画出来 —— 用 › 连成一串会被读成流程，那是错的。 */
/* 两列排布（租户/套餐 | 成员/项目 | Key）：单列 5 行会把看板撑高到压住球顶 */
.dash__quota {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto minmax(0, 1fr);
  gap: 4px 7px; align-items: center;
}
.dash__quota span { font-size: 10px; color: var(--text-dim); white-space: nowrap; }
.dash__quota i {
  display: block; height: 4px; border-radius: 99px;
  width: var(--w, 50%);
  background: linear-gradient(90deg, var(--teal), var(--cyan));
  opacity: .85;
  transform-origin: left center;
  animation: barIn .85s var(--ease-out) backwards;
}
.dash__quota i:nth-of-type(2) { animation-delay: .08s; }
.dash__quota i:nth-of-type(3) { animation-delay: .16s; }
.dash__quota i:nth-of-type(4) { animation-delay: .24s; }
.dash__quota i:nth-of-type(5) { animation-delay: .32s; }
@keyframes barIn { from { transform: scaleX(0); } }

/* 列3 · 本月消耗与管控 */
.dash__spend { display: flex; align-items: baseline; gap: 5px; white-space: nowrap; }
.dash__spend > span {
  font-family: var(--font-mono); font-size: 14px; font-weight: 600; letter-spacing: -.025em;
}
.dash__spend small { font-family: var(--font-mono); font-size: 10.5px; color: var(--text-mute); }
.dash__spend em {
  margin-left: auto; font-style: normal;
  font-family: var(--font-mono); font-size: 11px; color: var(--teal);
}
.dash__track {
  height: 4px; border-radius: 99px; background: var(--panel-line);
  overflow: hidden; margin-top: 7px;
}
.dash__fill {
  display: block; height: 100%; width: 64%; border-radius: 99px;
  background: linear-gradient(90deg, var(--teal), var(--cyan));
  box-shadow: 0 0 10px rgba(var(--glow-a), .8);
  animation: fillGrow 2.1s var(--ease-out) .9s backwards;
}
@keyframes fillGrow { from { width: 0; } }
.dash__meta {
  display: flex; gap: 13px;
  margin-top: 7px; font-size: 10px; color: var(--text-mute);
}
.dash__meta b { font-family: var(--font-mono); font-weight: 600; color: var(--text-dim); }
.chain { display: inline-flex; align-items: center; gap: 7px; }
.chain--short { display: none; }

.toast {
  position: absolute; right: 20px; bottom: 132px; z-index: 4;
  display: flex; align-items: center; gap: 9px;
  padding: 10px 14px; border-radius: 12px;
  background: var(--panel-solid);
  border: 1px solid rgba(251, 191, 36, .36);
  box-shadow: 0 18px 46px -18px rgba(0, 0, 0, .75);
  font-size: 11.5px; color: var(--text-dim); white-space: nowrap;
  opacity: 0; transform: translateY(12px) scale(.96);
  transition: opacity .45s var(--ease-out), transform .45s var(--ease-out);
  pointer-events: none;
}
.toast.is-on { opacity: 1; transform: translateY(0) scale(1); }
.toast svg { width: 15px; height: 15px; color: var(--amber); flex: none; }
.toast em { font-style: normal; font-family: var(--font-mono); color: var(--amber); }
.toast span { font-family: var(--font-mono); color: var(--text); }

/* ==========================================================================
   5. 底部能力带
   ========================================================================== */
.band {
  position: relative; z-index: 4;
  width: 100%; max-width: var(--shell-w); margin: 0 auto;
  display: flex; align-items: center; gap: clamp(18px, 2.6vw, 44px);
  padding: 0 clamp(20px, 4vw, 60px);
  overflow: hidden;
}
.band::before {
  content: ""; position: absolute; inset: 0 0 auto 0; height: 1px;
  background: linear-gradient(90deg, transparent, var(--panel-line), transparent);
}
.band__label {
  flex: none;
  font-family: var(--font-mono); font-size: 10.5px; letter-spacing: .12em;
  color: var(--text-mute); text-transform: uppercase;
}
.band__viewport {
  position: relative; flex: 1; min-width: 0; overflow: hidden;
  mask-image: linear-gradient(90deg, transparent, #000 7%, #000 93%, transparent);
  -webkit-mask-image: linear-gradient(90deg, transparent, #000 7%, #000 93%, transparent);
}
.band__track {
  display: flex; gap: clamp(30px, 3.6vw, 62px); width: max-content;
  animation: marquee 44s linear infinite;
}
.band:hover .band__track { animation-play-state: paused; }
@keyframes marquee { to { transform: translateX(-50%); } }
.vendor {
  display: flex; align-items: center; gap: 8px;
  font-size: 13px; font-weight: 500; color: var(--text-mute);
  white-space: nowrap; transition: color .25s;
}
.vendor:hover { color: var(--text-dim); }
.vendor i { width: 5px; height: 5px; border-radius: 50%; background: var(--teal); opacity: .75; }
.vendor:nth-child(4n+2) i { background: var(--cyan); }
.vendor:nth-child(4n+3) i { background: var(--violet); }
.vendor:nth-child(4n+4) i { background: var(--amber); }

/* ==========================================================================
   6. 响应式 / 降级
   ========================================================================== */
/* 中屏：面板变矮、球上移，看板必须跟着缩小才留得住「虚线连接」那段距离。
   这里**不能**让第三列折行 —— 折行会把看板撑高一倍，直接压到球上。 */
@media (max-width: 1340px) {
  .dash { top: 50px; width: min(520px, 78%); }
  .dash__col { padding: 8px 10px 9px; }
  .dash__label { font-size: 8.5px; margin-bottom: 6px; }
  .dash__row { font-size: 10px; margin-top: 3px; }
  .dash__row b { font-size: 10px; min-width: 38px; }
  .dash__row > span:last-child { font-size: 9.5px; }
  .dash__quota { gap: 3px 6px; }
  .dash__quota span { font-size: 9.5px; }
  .dash__spend > span { font-size: 13px; }
  .dash__spend small { font-size: 9.5px; }
  .dash__track { margin-top: 6px; }
  .dash__meta { margin-top: 6px; font-size: 9.5px; }
}

@media (max-width: 1180px) {
  .main { grid-template-columns: minmax(0, .92fr) minmax(0, 1.08fr); gap: 28px; }
  .toast { right: 14px; bottom: 126px; }
}

@media (max-width: 960px) {
  .lp { height: auto; min-height: 100dvh; overflow: visible; grid-template-rows: auto auto auto; }
  .nav__links { display: none; }
  .main { grid-template-columns: minmax(0, 1fr); gap: 36px; padding-top: 26px; padding-bottom: 38px; }

  /* 窄屏：面板改为「头 + 额度条 + 图 + 日志」四段自然堆叠，
     日志不再浮层（否则必遮住底部节点）。 */
  .stage { height: auto; min-height: 0; }
  .panel { height: auto; grid-template-rows: auto auto auto; }
  .panel__body { aspect-ratio: 900 / 460; }
  .logs {
    position: static; width: 100%; height: auto;
    padding: 13px 20px 15px;
    background: none;
    border-top: 1px solid var(--panel-line-soft);
    mask-image: none;
    -webkit-mask-image: none;
  }
  .logs::before { display: none; }
  /* 窄屏：浮层看板会盖住球，改为排在面板下方的一块静态卡片 */
  .link-dashed { display: none; }
  .dash {
    position: static; transform: none;
    width: auto; margin: 14px 18px 0;
    grid-template-columns: minmax(0, 1fr);
  }
  .dash__col { padding: 12px 16px; }
  .dash__col + .dash__col {
    border-top: 1px solid var(--panel-line-soft);
  }
  .dash__col + .dash__col::before { display: none; }
  .dash__col:nth-child(3) { border-top: 1px solid var(--panel-line-soft); padding-top: 12px; }
}

@media (max-width: 620px) {
  .stats { grid-template-columns: repeat(2, 1fr); row-gap: 18px; }
  .stat:nth-child(3)::before { display: none; }
  .brand__tag { display: none; }
  .toast { right: 10px; left: 10px; bottom: 118px; justify-content: center; }
  .snippet { font-size: 11.5px; }
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: .001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: .001ms !important;
  }
  .title__line > span, .title__grad { animation: none; opacity: 1; transform: none; }
  .panel__scan, .bg__wash, .bg__grid, .bg__ring { animation: none; }
  .panel__scan { opacity: .7; }
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
  backdrop-filter: blur(8px);
}

/* Card */
.announce-card {
  position: relative; width: 100%; max-width: 860px;
  border-radius: 1.5rem; overflow: hidden;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(28px) saturate(1.18);
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
