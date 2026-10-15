<script setup lang="ts">
import { h } from 'vue'
import type { Component } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import {
  IconInfoCircle,
  IconGithub,
  IconLink,
  IconStar,
  IconFile,
  IconUserGroup,
  IconApps,
  IconCodeBlock,
  IconBranch,
  IconSafe,
  IconEye,
} from '@arco-design/web-vue/es/icon'

const appVersion = __APP_VERSION__ || 'dev'
const repoUrl = 'https://github.com/qianfree/team-api'
const issuesUrl = `${repoUrl}/issues`
const noticesUrl = `${repoUrl}/blob/main/THIRD_PARTY_NOTICES.md`
const licenseUrl = `${repoUrl}/blob/main/LICENSE`
const readmeUrl = `${repoUrl}#readme`

const projectInfo = [
  { label: '项目名称', value: 'Team-API' },
  { label: '构建版本', value: appVersion },
  { label: '开源许可证', value: 'AGPLv3 / 商业授权另议', url: licenseUrl },
  { label: '代码仓库', value: repoUrl.replace('https://', ''), url: repoUrl },
  { label: '问题反馈', value: 'GitHub Issues', url: issuesUrl },
]

interface Feature {
  icon: Component
  title: string
  desc: string
}

const features: Feature[] = [
  {
    icon: IconUserGroup,
    title: '多租户架构',
    desc: '行级租户隔离；管理后台与租户控制台双独立用户体系，互不越权。',
  },
  {
    icon: IconApps,
    title: '多供应商适配',
    desc: 'OpenAI / Claude / Gemini / DeepSeek / 通义千问 / 智谱 / Ollama 等，按需启用。',
  },
  {
    icon: IconCodeBlock,
    title: 'OpenAI 兼容 API',
    desc: '无缝替换 OpenAI SDK；支持对话补全、向量嵌入、图像生成、语音、实时通信。',
  },
  {
    icon: IconBranch,
    title: '智能渠道调度',
    desc: '优先级 / 权重路由、自动故障转移、健康监控、渠道亲和性，按成本与可用性动态分配。',
  },
  {
    icon: IconSafe,
    title: '三层额度 + 实时计费',
    desc: '租户钱包 → 项目预算 / 成员额度 → Key 额度；预扣 → 转发 → 结算 → 退款，Redis 原子操作。',
  },
  {
    icon: IconEye,
    title: '全链路可观测',
    desc: '请求日志、操作审计、监控告警；Request ID 贯穿全链路，问题定位不再跨服务拼日志。',
  },
]
</script>

<template>
  <div class="about-page">
    <PageHeader
      title="关于"
      description="项目介绍、核心特性、开源许可证与第三方组件声明"
    />

    <div class="about-grid">
      <!-- 项目信息 -->
      <div class="about-card about-card--wide">
        <div class="about-card__header">
          <div class="project-logo">T</div>
          <div>
            <div class="project-name">Team-API</div>
            <div class="project-desc">多租户大模型 API 网关管理平台</div>
          </div>
        </div>
        <div class="about-card__body">
          <div class="info-grid">
            <div
              v-for="item in projectInfo"
              :key="item.label"
              class="info-item"
            >
              <span class="info-item__label">{{ item.label }}</span>
              <a
                v-if="item.url"
                :href="item.url"
                target="_blank"
                rel="noopener noreferrer"
                class="info-item__value info-item__value--link"
              >
                <IconLink />
                {{ item.value }}
              </a>
              <span v-else class="info-item__value">{{ item.value }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 项目介绍 -->
      <div class="about-card about-card--wide about-card--intro">
        <div class="about-card__title">
          <IconApps />
          项目介绍
        </div>
        <div class="about-card__body">
          <p class="intro-text">
            Team-API 是一个多租户大模型 API 网关：把多家大模型供应商封装成统一的 OpenAI 兼容接口，配合计费、限流、监控和多租户隔离，让你专心做产品，不必为每家供应商写一遍适配。
          </p>
          <div class="feature-grid">
            <div
              v-for="feat in features"
              :key="feat.title"
              class="feature"
            >
              <div class="feature__icon">
                <component :is="feat.icon" />
              </div>
              <div class="feature__body">
                <div class="feature__title">{{ feat.title }}</div>
                <div class="feature__desc">{{ feat.desc }}</div>
              </div>
            </div>
          </div>
          <div class="license-actions">
            <a
              :href="readmeUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="license-link"
            >
              <IconFile />
              查看完整 README
            </a>
          </div>
        </div>
      </div>

      <!-- 许可证说明 -->
      <div class="about-card">
        <div class="about-card__title">
          <IconInfoCircle />
          许可证说明
        </div>
        <div class="about-card__body">
          <p class="license-text">
            本项目在 GNU Affero General Public License version 3（AGPLv3）下开源。你可以自由使用、修改、分发本软件的源代码，但必须遵守 AGPLv3 的 copyleft 义务：一旦对外提供网络服务，就需要向用户公开相应的源代码。
          </p>
          <p class="license-text">
            如需 AGPLv3 之外的商业授权（例如 SaaS/OEM/转售、免除 copyleft 义务），请与版权方另行签署书面协议。
          </p>
          <div class="license-actions">
            <a
              :href="licenseUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="license-link"
            >
              <IconFile />
              查看完整 LICENSE
            </a>
          </div>
        </div>
      </div>

      <!-- 第三方组件 -->
      <div class="about-card">
        <div class="about-card__title">
          <IconFile />
          第三方组件声明
        </div>
        <div class="about-card__body">
          <p class="section-text">
            本项目使用了大量优秀的开源组件与库。第三方组件的完整授权信息、版权声明及源码引用可在仓库的 THIRD_PARTY_NOTICES.md 中查阅。
          </p>
          <div class="license-actions">
            <a
              :href="noticesUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="license-link"
            >
              <IconLink />
              查看 THIRD_PARTY_NOTICES.md
            </a>
          </div>
        </div>
      </div>
    </div>

    <!-- Star 引导 -->
    <div class="star-card">
      <div class="star-card__content">
        <div class="star-card__icon">
          <IconGithub />
        </div>
        <div class="star-card__text">
          <div class="star-card__title">开源不易，欢迎支持</div>
          <div class="star-card__desc">
            如果 Team-API 对你有帮助，欢迎在 GitHub 点个 Star，让更多人发现这个项目。
          </div>
        </div>
      </div>
      <a
        :href="repoUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="star-card__btn"
      >
        <IconStar />
        前往 GitHub 点赞
      </a>
    </div>
  </div>
</template>

<style scoped>
.about-page {
  padding-bottom: 24px;
}

.about-grid {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 20px;
  margin-bottom: 20px;
}

.about-card {
  background: var(--ta-bg-card);
  border: 1px solid var(--ta-border);
  border-radius: var(--ta-radius-lg);
  box-shadow: var(--ta-shadow-card);
  overflow: hidden;
}

.about-card--wide {
  grid-column: 1 / -1;
}

.about-card__header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px;
  border-bottom: 1px solid var(--ta-border);
  background: linear-gradient(135deg, var(--ta-primary-light) 0%, var(--ta-bg-card) 100%);
}

.project-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  background: var(--ta-primary-gradient);
  border-radius: var(--ta-radius);
  font-size: 26px;
  font-weight: 700;
  color: #fff;
  box-shadow: var(--ta-shadow-button);
  flex-shrink: 0;
}

.project-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--ta-text-primary);
}

.project-desc {
  font-size: 14px;
  color: var(--ta-text-tertiary);
  margin-top: 2px;
}

.about-card__title {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 20px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
  border-bottom: 1px solid var(--ta-border);
}

.about-card__body {
  padding: 20px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-item__label {
  font-size: 13px;
  color: var(--ta-text-tertiary);
}

.info-item__value {
  font-size: 14px;
  color: var(--ta-text-primary);
  word-break: break-all;
}

.info-item__value--link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--ta-primary);
  text-decoration: none;
  transition: color var(--ta-duration-fast) var(--ta-ease);
}

.info-item__value--link:hover {
  color: var(--ta-primary-hover);
  text-decoration: underline;
}

.license-text,
.section-text {
  font-size: 14px;
  line-height: 1.7;
  color: var(--ta-text-secondary);
  margin: 0 0 12px;
}

.license-text:last-child,
.section-text:last-child {
  margin-bottom: 0;
}

.license-actions {
  margin-top: 16px;
}

.intro-text {
  font-size: 14px;
  line-height: 1.7;
  color: var(--ta-text-secondary);
  margin: 0 0 20px;
}

.feature-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.feature {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px;
  background: var(--ta-bg-secondary);
  border: 1px solid var(--ta-border-light);
  border-radius: var(--ta-radius);
  transition: transform var(--ta-duration-fast) var(--ta-ease),
              box-shadow var(--ta-duration-fast) var(--ta-ease);
}

.feature:hover {
  transform: translateY(-1px);
  box-shadow: var(--ta-shadow-hover);
}

.feature__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 8px;
  background: var(--ta-primary-light);
  color: var(--ta-primary);
  font-size: 18px;
}

.feature__body {
  min-width: 0;
}

.feature__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin-bottom: 4px;
}

.feature__desc {
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--ta-text-tertiary);
}

@media (max-width: 900px) {
  .feature-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .feature-grid {
    grid-template-columns: 1fr;
  }
}

.license-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ta-primary);
  text-decoration: none;
  font-weight: 500;
}

.license-link:hover {
  text-decoration: underline;
}

.star-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
  background: linear-gradient(135deg, var(--ta-primary-light) 0%, var(--ta-bg-card) 100%);
  border: 1px solid var(--ta-border);
  border-radius: var(--ta-radius-lg);
  box-shadow: var(--ta-shadow-card);
}

.star-card__content {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.star-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: #0f172a;
  color: #fff;
  font-size: 22px;
  flex-shrink: 0;
}

.star-card__text {
  min-width: 0;
}

.star-card__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ta-text-primary);
  margin-bottom: 4px;
}

.star-card__desc {
  font-size: 13px;
  color: var(--ta-text-secondary);
}

.star-card__btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  padding: 10px 18px;
  border-radius: var(--ta-radius);
  background: var(--ta-primary-gradient);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
  box-shadow: var(--ta-shadow-button);
  transition: box-shadow var(--ta-duration-fast) var(--ta-ease), transform var(--ta-duration-fast) var(--ta-ease);
}

.star-card__btn:hover {
  box-shadow: var(--ta-shadow-button-hover);
  transform: translateY(-1px);
}

@media (max-width: 768px) {
  .about-grid {
    grid-template-columns: 1fr;
  }

  .about-card--wide {
    grid-column: auto;
  }

  .star-card {
    flex-direction: column;
    align-items: flex-start;
  }

  .star-card__btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
