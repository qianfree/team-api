<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import axios from 'axios'
import { markSystemInitialized } from '@/router'
import { IconGithub, IconStar } from '@arco-design/web-vue/es/icon'

const router = useRouter()
const repoUrl = 'https://github.com/qianfree/team-api'
const loading = ref(false)
const checking = ref(true)

const form = reactive({
  username: '',
  displayName: '',
  password: '',
  confirmPassword: '',
  currency: 'USD',
})

onMounted(async () => {
  try {
    const res = await axios.get('/api/setup/status', { timeout: 5000 })
    if (res.data?.code === 0 && res.data?.data?.initialized) {
      router.replace('/admin/login')
    }
  } catch {
    // status endpoint might be blocked — if so, system is already initialized
  } finally {
    checking.value = false
  }
})

async function handleSetup() {
  if (!form.username) {
    Message.error('请输入用户名')
    return
  }
  if (!form.password) {
    Message.error('请输入密码')
    return
  }
  if (form.password !== form.confirmPassword) {
    Message.error('两次输入的密码不一致')
    return
  }

  loading.value = true
  try {
    await request.post('/setup/initialize', {
      username: form.username.trim(),
      displayName: form.displayName.trim(),
      password: form.password,
      confirmPassword: form.confirmPassword,
      currency: form.currency,
    })
    Message.success('初始化成功，正在跳转登录页...')
    markSystemInitialized()
    router.replace('/admin/login')
  } catch {
    // error toast already shown by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <!-- Left: Branding Panel -->
    <div class="login-brand">
      <div class="login-orb login-orb--1" />
      <div class="login-orb login-orb--2" />
      <div class="login-orb login-orb--3" />

      <div class="login-brand__content">
        <div class="login-brand__logo">T</div>
        <h1 class="login-brand__title">Team-API</h1>
        <p class="login-brand__desc">多租户 大模型 API 网关管理平台</p>

        <div class="login-brand__features">
          <div class="login-brand__feature">
            <div class="login-brand__feature-dot" style="background: #0d9488" />
            <div>
              <div class="login-brand__feature-title">统一接口</div>
              <div class="login-brand__feature-desc">OpenAI / Claude / Gemini 一键接入</div>
            </div>
          </div>
          <div class="login-brand__feature">
            <div class="login-brand__feature-dot" style="background: #10b981" />
            <div>
              <div class="login-brand__feature-title">智能计费</div>
              <div class="login-brand__feature-desc">实时用量统计，精准费用核算</div>
            </div>
          </div>
          <div class="login-brand__feature">
            <div class="login-brand__feature-dot" style="background: #f59e0b" />
            <div>
              <div class="login-brand__feature-title">安全可控</div>
              <div class="login-brand__feature-desc">多租户隔离，细粒度权限管控</div>
            </div>
          </div>
        </div>
      </div>

      <div class="login-brand__star">
        <div class="setup-star">
          <div class="setup-star__icon">
            <IconGithub />
          </div>
          <div class="setup-star__text">
            <div class="setup-star__title">开源不易，欢迎 Star 支持</div>
            <div class="setup-star__desc">如果这个项目对你有帮助，欢迎在 GitHub 点赞</div>
          </div>
          <a
            :href="repoUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="setup-star__btn"
          >
            <IconStar />
            前往点赞
          </a>
        </div>
      </div>

      <div class="login-brand__footer">
        <span>Team-API &copy; {{ new Date().getFullYear() }}</span>
      </div>
    </div>

    <!-- Right: Setup Form -->
    <div class="login-form-wrapper">
      <div v-if="checking" class="login-form" style="text-align: center; padding: 80px 36px;">
        <ASpin size="32" />
      </div>
      <div v-else class="login-form">
        <div class="login-form__header">
          <h2 class="login-form__title">系统初始化</h2>
          <p class="login-form__subtitle">首次使用，请创建管理员账号</p>
        </div>

        <AForm
          :model="form"
          layout="vertical"
          size="large"
          @submit="handleSetup"
        >
          <AFormItem
            field="username"
            label="用户名"
            :rules="[
              { required: true, message: '请输入用户名' },
              { match: /^[a-zA-Z0-9_]{3,20}$/, message: '仅支持字母、数字和下划线，长度3-20' },
            ]"
          >
            <AInput
              v-model="form.username"
              placeholder="3-20位，字母/数字/下划线"
              autocomplete="off"
              @keydown.enter="handleSetup"
            />
          </AFormItem>
          <AFormItem
            field="displayName"
            label="显示名称"
            extra="可选，用于界面显示"
          >
            <AInput
              v-model="form.displayName"
              placeholder="管理员显示名称"
              autocomplete="off"
            />
          </AFormItem>
          <AFormItem
            field="password"
            label="密码"
            :rules="[
              { required: true, message: '请输入密码' },
              { minLength: 8, message: '密码长度不能少于8位' },
            ]"
          >
            <AInputPassword
              v-model="form.password"
              placeholder="至少8位，含字母和数字"
              @keydown.enter="handleSetup"
            />
          </AFormItem>
          <AFormItem
            field="confirmPassword"
            label="确认密码"
            :rules="[
              { required: true, message: '请再次输入密码' },
              { validator: (value: string, callback: (err?: string) => void) => {
                if (value !== form.password) callback('两次输入的密码不一致')
                else callback()
              }},
            ]"
          >
            <AInputPassword
              v-model="form.confirmPassword"
              placeholder="再次输入密码"
              @keydown.enter="handleSetup"
            />
          </AFormItem>
          <AFormItem
            field="currency"
            label="本位币"
            extra="决定系统内部记账与全站显示货币（模型定价按本位币配置），初始化后不可更改"
          >
            <ARadioGroup v-model="form.currency">
              <ARadio value="USD">USD（美元）· 与上游模型报价对齐</ARadio>
              <ARadio value="CNY">CNY（人民币）· 面向国内运营</ARadio>
            </ARadioGroup>
          </AFormItem>
          <AFormItem>
            <AButton
              type="primary"
              long
              size="large"
              :loading="loading"
              @click="handleSetup"
            >
              初始化系统
            </AButton>
          </AFormItem>
        </AForm>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  min-height: 100vh;
}

.login-brand {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 60px;
  background: linear-gradient(135deg, #020617 0%, #0f172a 30%, #042f2e 60%, #0f172a 100%);
  color: #fff;
  position: relative;
  overflow: hidden;
}

.login-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  pointer-events: none;
}

.login-orb--1 {
  width: 300px; height: 300px;
  background: rgba(13, 148, 136, 0.12);
  top: 10%; right: -5%;
  animation: float 8s ease-in-out infinite;
}

.login-orb--2 {
  width: 250px; height: 250px;
  background: rgba(6, 182, 212, 0.08);
  bottom: 20%; left: -5%;
  animation: float 10s ease-in-out infinite reverse;
}

.login-orb--3 {
  width: 200px; height: 200px;
  background: rgba(20, 184, 166, 0.06);
  top: 50%; right: 30%;
  animation: float 12s ease-in-out infinite;
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-20px); }
}

.login-brand__content {
  position: relative;
  z-index: 1;
  max-width: 400px;
}

.login-brand__logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px; height: 48px;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  border-radius: 12px;
  font-size: 24px; font-weight: 700;
  margin-bottom: 24px;
  box-shadow: 0 4px 16px rgba(13, 148, 136, 0.4);
}

.login-brand__title {
  font-size: 32px; font-weight: 700;
  letter-spacing: -0.02em; margin-bottom: 8px;
}

.login-brand__desc {
  font-size: 16px; color: rgba(255, 255, 255, 0.6);
  margin-bottom: 48px;
}

.login-brand__features {
  display: flex; flex-direction: column; gap: 24px;
}

.login-brand__feature {
  display: flex; align-items: flex-start; gap: 14px;
}

.login-brand__feature-dot {
  flex-shrink: 0; width: 8px; height: 8px;
  border-radius: 50%; margin-top: 7px;
}

.login-brand__feature-title {
  font-size: 15px; font-weight: 600;
  color: rgba(255, 255, 255, 0.9); margin-bottom: 2px;
}

.login-brand__feature-desc {
  font-size: 13px; color: rgba(255, 255, 255, 0.45);
}

.login-brand__footer {
  position: relative; z-index: 1;
  padding-top: 16px;
  font-size: 13px; color: rgba(255, 255, 255, 0.3);
}

.login-brand__star {
  position: relative;
  z-index: 1;
  margin-top: auto;
  padding-top: 40px;
}

.setup-star {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.setup-star__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 18px;
  flex-shrink: 0;
}

.setup-star__text {
  flex: 1;
  min-width: 0;
}

.setup-star__title {
  font-size: 14px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
  margin-bottom: 2px;
}

.setup-star__desc {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.45);
}

.setup-star__btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  padding: 8px 14px;
  border-radius: 8px;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  box-shadow: 0 4px 12px rgba(13, 148, 136, 0.35);
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.setup-star__btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(13, 148, 136, 0.45);
}

@media (max-width: 768px) {
  .setup-star {
    flex-direction: column;
    align-items: flex-start;
  }

  .setup-star__btn {
    width: 100%;
    justify-content: center;
  }
}

.login-form-wrapper {
  flex: 1; display: flex; align-items: center;
  justify-content: center; padding: 40px; background: #f8fafc;
}

.login-form {
  width: 100%; max-width: 400px;
  padding: 36px; border-radius: var(--ta-radius-xl);
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(24px); -webkit-backdrop-filter: blur(24px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.login-form__header {
  margin-bottom: 32px;
}

.login-form__title {
  font-size: 26px; font-weight: 700;
  color: #0f172a; margin-bottom: 6px;
}

.login-form__subtitle {
  font-size: 15px; color: #94a3b8;
}

@media (max-width: 768px) {
  .login-page { flex-direction: column; }
  .login-brand { display: none; }
  .login-form-wrapper { min-height: 100vh; }
}
</style>
