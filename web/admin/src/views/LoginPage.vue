<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import type { FormInstance } from '@arco-design/web-vue'
import { useAuthStore } from '@/stores/auth'
import { extractApiError } from '@/utils/request'
import request from '@/utils/request'
import SlideCaptcha from '@/components/SlideCaptcha.vue'
import { useSiteName } from '@/composables/useSiteName'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const { siteName } = useSiteName()

const form = reactive({
  username: '',
  password: '',
})

// 2FA state
const show2FA = ref(false)
const provisionalToken = ref('')
const totpCode = ref('')
const totpLoading = ref(false)

// Captcha state — always required
const captcha = reactive({ captchaKey: '', captchaX: 0 })
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)

async function handleLogin() {
  try {
    const errors = await formRef.value?.validate()
    if (errors) return
  } catch {
    return
  }

  loading.value = true
  try {
    const res = await authStore.login(form.username, form.password, { captchaKey: captcha.captchaKey, captchaX: captcha.captchaX })
    // Check if 2FA is required
    if (res?.totp_required) {
      provisionalToken.value = res.provisional_token
      show2FA.value = true
      return
    }
    Message.success('登录成功')
    const redirect = (route.query.redirect as string) || '/admin'
    router.push(redirect)
  } catch (err: any) {
    const apiErr = extractApiError(err)
    if (apiErr?.code === 10058) {
      Message.error('滑块验证失败，请重新拖动')
    }
    captchaRef.value?.resetCaptcha()
  } finally {
    loading.value = false
  }
}

async function handle2FAVerify() {
  if (!totpCode.value) {
    Message.warning('请输入验证码')
    return
  }
  totpLoading.value = true
  try {
    const res = await request.post('/api/admin/auth/2fa/verify', {
      provisional_token: provisionalToken.value,
      code: totpCode.value,
    })
    // If we get here, code === 0 (interceptor rejects otherwise)
    authStore.applyTokensFrom2FA(res.data.data)
    Message.success('登录成功')
    const redirect = (route.query.redirect as string) || '/admin'
    router.push(redirect)
  } catch {
    // error toast already shown by interceptor
  } finally {
    totpLoading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <!-- Left: Branding Panel -->
    <div class="login-brand">
      <!-- Floating orbs -->
      <div class="login-orb login-orb--1" />
      <div class="login-orb login-orb--2" />
      <div class="login-orb login-orb--3" />

      <div class="login-brand__content">
        <img src="/favicon.png" :alt="siteName" class="login-brand__logo" />
        <h1 class="login-brand__title">{{ siteName }}</h1>
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

      <div class="login-brand__footer">
        <span>&copy; {{ new Date().getFullYear() }} qianfree. Licensed under AGPL-3.0.</span>
      </div>
    </div>

    <!-- Right: Login Form -->
    <div class="login-form-wrapper">
      <!-- Mobile Brand (hidden on desktop) -->
      <div class="login-mobile-brand">
        <img src="/favicon.png" :alt="siteName" class="login-brand__logo" />
        <span class="login-mobile-brand__name">{{ siteName }}</span>
      </div>

      <div class="login-form glass animate-slideUp">
        <div class="login-form__header">
          <h2 class="login-form__title">欢迎回来</h2>
          <p class="login-form__subtitle">请登录管理后台</p>
        </div>

        <AForm
          ref="formRef"
          :model="form"
          layout="vertical"
          size="large"
        >
          <AFormItem
            field="username"
            label="用户名"
            :rules="[{ required: true, message: '请输入用户名' }]"
          >
            <AInput
              v-model="form.username"
              placeholder="请输入用户名"
              @keydown.enter="handleLogin"
            />
          </AFormItem>
          <AFormItem
            field="password"
            label="密码"
            :rules="[{ required: true, message: '请输入密码' }]"
          >
            <AInput
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              @keydown.enter="handleLogin"
            />
          </AFormItem>
          <!-- Captcha -->
          <div style="margin-bottom: 20px;">
            <SlideCaptcha ref="captchaRef" v-model="captcha" />
          </div>
          <AFormItem>
            <AButton
              type="primary"
              long
              size="large"
              :loading="loading"
              @click="handleLogin"
            >
              登 录
            </AButton>
          </AFormItem>
        </AForm>

        <!-- 2FA Verification -->
        <div v-if="show2FA" class="totp-form">
          <div class="login-form__header">
            <h2 class="login-form__title">双因素认证</h2>
            <p class="login-form__subtitle">请输入身份验证器中的 6 位数字码</p>
          </div>
          <AFormItem>
            <AInput
              v-model="totpCode"
              placeholder="6 位数字验证码"
              maxlength="6"
              size="large"
              @keydown.enter="handle2FAVerify"
            />
          </AFormItem>
          <AButton
            type="primary"
            long
            size="large"
            :loading="totpLoading"
            @click="handle2FAVerify"
          >
            验证并登录
          </AButton>
          <AButton
            type="text"
            long
            size="small"
            class="mt-2"
            @click="show2FA = false; totpCode = ''"
          >
            返回登录
          </AButton>
        </div>
      </div>

      <!-- Mobile Footer (hidden on desktop) -->
      <div class="login-mobile-footer">
        <a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="login-mobile-footer__link">Powered by Team-API</a>
        <span>&copy; {{ new Date().getFullYear() }} qianfree. Licensed under AGPL-3.0.</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  min-height: 100vh;
}

/* ===== Branding Panel ===== */
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

/* Floating orbs */
.login-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  pointer-events: none;
}

.login-orb--1 {
  width: 300px;
  height: 300px;
  background: rgba(13, 148, 136, 0.12);
  top: 10%;
  right: -5%;
  animation: float 8s ease-in-out infinite;
}

.login-orb--2 {
  width: 250px;
  height: 250px;
  background: rgba(6, 182, 212, 0.08);
  bottom: 20%;
  left: -5%;
  animation: float 10s ease-in-out infinite reverse;
}

.login-orb--3 {
  width: 200px;
  height: 200px;
  background: rgba(20, 184, 166, 0.06);
  top: 50%;
  right: 30%;
  animation: float 12s ease-in-out infinite;
}

.login-brand__content {
  position: relative;
  z-index: 1;
  max-width: 400px;
}

.login-brand__logo {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  margin-bottom: 24px;
  object-fit: contain;
}

.login-brand__title {
  font-size: 32px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin-bottom: 8px;
}

.login-brand__desc {
  font-size: 16px;
  color: rgba(255, 255, 255, 0.6);
  margin-bottom: 48px;
}

.login-brand__features {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.login-brand__feature {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.login-brand__feature-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 7px;
}

.login-brand__feature-title {
  font-size: 15px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
  margin-bottom: 2px;
}

.login-brand__feature-desc {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.45);
}

.login-brand__footer {
  position: relative;
  z-index: 1;
  margin-top: auto;
  padding-top: 40px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.3);
}

/* ===== Form Panel ===== */
.login-form-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: #f8fafc;
}

.login-form {
  width: 100%;
  max-width: 400px;
  padding: 36px;
  border-radius: var(--ta-radius-xl);
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.login-form__header {
  margin-bottom: 32px;
}

.login-form__title {
  font-size: 26px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 6px;
}

.login-form__subtitle {
  font-size: 15px;
  color: #94a3b8;
}

/* Mobile brand & footer — hidden on desktop */
.login-mobile-brand {
  display: none;
  justify-content: center;
  align-items: center;
  gap: 10px;
  margin-bottom: 24px;
}

.login-mobile-brand__name {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
}

.login-mobile-footer {
  display: none;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  position: absolute;
  bottom: 16px;
  left: 0;
  right: 0;
  font-size: 11px;
  color: #94a3b8;
}

.login-mobile-footer__link {
  color: #64748b;
  text-decoration: none;
  transition: color 0.15s;
}

.login-mobile-footer__link:hover {
  color: #0f172a;
}

/* ===== Responsive: hide branding on mobile ===== */
@media (max-width: 768px) {
  .login-page {
    flex-direction: column;
  }

  .login-brand {
    display: none;
  }

  .login-form-wrapper {
    position: relative;
    min-height: 100vh;
    background: linear-gradient(to bottom, #f0fdfa, #f8fafc, #f0fdf4);
  }

  .login-mobile-brand {
    display: flex;
  }

  .login-mobile-brand .login-brand__logo {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    margin-bottom: 0;
  }

  .login-mobile-brand__name {
    font-size: 32px;
  }

  .login-mobile-footer {
    display: flex;
  }
}

.totp-form {
  margin-top: 16px;
}

.mt-2 {
  margin-top: 8px;
}
</style>
