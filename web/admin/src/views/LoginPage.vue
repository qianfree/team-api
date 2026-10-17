<script setup lang="ts">
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import type { FormInstance } from '@arco-design/web-vue'
import { marked } from 'marked'
import { sanitizeHtml } from '@/utils/sanitize'
import { useAuthStore } from '@/stores/auth'
import type { PendingAgreement } from '@/stores/auth'
import { extractApiError } from '@/utils/request'
import request from '@/utils/request'
import { getDeviceId } from '@/utils/deviceId'
import SlideCaptcha from '@/components/SlideCaptcha.vue'
import OtpInput from '@/components/OtpInput.vue'
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
  remember: true,
})

// 2FA state
const show2FA = ref(false)
const provisionalToken = ref('')
const totpCode = ref('')
const totpLoading = ref(false)
const totpError = ref('')
const otpRef = ref<InstanceType<typeof OtpInput> | null>(null)

// Captcha state — always required
const captcha = reactive({ captchaKey: '', captchaX: 0 })
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)

// Pending agreements state
const showAgreements = ref(false)
const agreements = ref<PendingAgreement[]>([])
const selectedAgreement = ref<PendingAgreement | null>(null)
const agreementContent = ref('')
const agreementLoading = ref(false)
const acceptLoading = ref(false)
const allChecked = ref(false)

const agreementContentHtml = computed(() => {
  if (!agreementContent.value) return ''
  return sanitizeHtml(marked(agreementContent.value) as string)
})

function proceedAfterLogin() {
  const pending = authStore.pendingAgreements
  if (pending.length > 0) {
    agreements.value = pending
    selectedAgreement.value = pending[0]
    loadAgreementContent(pending[0])
    showAgreements.value = true
    return
  }
  Message.success('登录成功')
  const redirect = (route.query.redirect as string) || '/admin'
  router.push(redirect)
}

async function loadAgreementContent(item: PendingAgreement) {
  selectedAgreement.value = item
  agreementContent.value = ''
  agreementLoading.value = true
  try {
    const res = await request.get(`/admin/agreements/${item.id}`)
    agreementContent.value = res.data?.data?.content || ''
  } catch {
    agreementContent.value = '加载失败，请重试'
  } finally {
    agreementLoading.value = false
  }
}

async function acceptAgreements() {
  if (!allChecked.value) {
    Message.warning('请先阅读并勾选同意所有协议')
    return
  }
  acceptLoading.value = true
  try {
    await request.post('/admin/agreements/accept', {
      agreement_ids: agreements.value.map(a => a.id),
    })
    authStore.clearPendingAgreements()
    showAgreements.value = false
    Message.success('登录成功')
    const redirect = (route.query.redirect as string) || '/admin'
    router.push(redirect)
  } catch {
    // error auto-displayed
  } finally {
    acceptLoading.value = false
  }
}

// Check if already logged in on mount
onMounted(() => {
  // If already logged in, redirect to dashboard
  if (authStore.isLoggedIn) {
    const redirect = (route.query.redirect as string) || '/admin'
    router.replace(redirect)
  }
})
async function handleLogin() {
  try {
    const errors = await formRef.value?.validate()
    if (errors) return
  } catch {
    return
  }

  loading.value = true
  try {
    const res = await authStore.login(form.username, form.password, { captchaKey: captcha.captchaKey, captchaX: captcha.captchaX }, form.remember)
    // Check if 2FA is required
    if (res?.totp_required) {
      provisionalToken.value = res.provisional_token
      totpError.value = ''
      show2FA.value = true
      nextTick(() => otpRef.value?.focusFirst())
      return
    }
    proceedAfterLogin()
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
  if (totpLoading.value) return
  if (totpCode.value.length !== 6) {
    totpError.value = '请输入完整的 6 位验证码'
    return
  }
  totpError.value = ''
  totpLoading.value = true
  try {
    // 行内提示错误原因，故抑制全局 toast，避免重复信息
    const res = await request.post(
      '/admin/auth/2fa/verify',
      {
        provisional_token: provisionalToken.value,
        code: totpCode.value,
        device_id: getDeviceId(),
      },
      { _suppressErrorMsg: true } as any,
    )
    authStore.applyTokensFrom2FA(res.data.data)
    proceedAfterLogin()
  } catch (err: any) {
    const apiErr = extractApiError(err)
    totpError.value = apiErr?.message || '验证码不正确，请重新输入'
    totpCode.value = ''
    nextTick(() => otpRef.value?.focusFirst())
  } finally {
    totpLoading.value = false
  }
}

// 输入即清除错误态（错误由 handle2FAVerify 显式写入，避免清空验证码时被误清）
function onTotpInput(v: string) {
  totpCode.value = v.replace(/\D/g, '')
  if (totpError.value) totpError.value = ''
}

// 返回登录：清临时凭据并重置滑块——密码步成功时滑块已被消耗，直接再点登录必然报过期
function backToLogin() {
  show2FA.value = false
  totpCode.value = ''
  totpError.value = ''
  provisionalToken.value = ''
  captchaRef.value?.resetCaptcha()
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
        <span>&copy; {{ new Date().getFullYear() }} qianfree. Licensed under AGPL-3.0-or-later.</span>
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
        <!-- 2FA Verification：整卡切换为验证码表单（与租户端一致），而非在登录表单下方追加 -->
        <template v-if="show2FA">
          <div class="twofa">
            <div class="twofa__badge">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 3l7 3v5.4c0 4.3-2.9 7.8-7 9.1-4.1-1.3-7-4.8-7-9.1V6l7-3z" />
                <path d="M9.4 12.1l1.9 1.9 3.4-3.7" />
              </svg>
            </div>

            <h2 class="twofa__title">双因素认证</h2>
            <p class="twofa__subtitle">打开身份验证器 App，输入当前 6 位动态验证码</p>

            <OtpInput
              ref="otpRef"
              class="twofa__otp"
              :model-value="totpCode"
              :invalid="!!totpError"
              :disabled="totpLoading"
              described-by="twofa-error"
              autofocus
              @update:model-value="onTotpInput"
              @complete="handle2FAVerify"
            />

            <div class="twofa__message">
              <transition name="twofa-msg">
                <p v-if="totpError" id="twofa-error" role="alert" class="twofa__message-text">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 8v4.5M12 16h.01" />
                  </svg>
                  <span>{{ totpError }}</span>
                </p>
              </transition>
            </div>

            <AButton
              class="twofa__submit"
              type="primary"
              long
              size="large"
              :loading="totpLoading"
              @click="handle2FAVerify"
            >
              验证并登录
            </AButton>

            <button type="button" class="twofa__back" @click="backToLogin">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M15 18l-6-6 6-6" />
              </svg>
              返回登录
            </button>
          </div>
        </template>

        <template v-else>
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
            <!-- Remember Me -->
            <AFormItem>
              <ACheckbox v-model="form.remember">
                记住登录状态（7天免登录）
              </ACheckbox>
            </AFormItem>
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
        </template>
      </div>

      <!-- Mobile Footer (hidden on desktop) -->
      <div class="login-mobile-footer">
        <a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="login-mobile-footer__link">Powered by Team-API</a>
        <span>&copy; {{ new Date().getFullYear() }} qianfree. Licensed under AGPL-3.0-or-later.</span>
      </div>
    </div>

    <!-- Pending Agreements Modal -->
    <AModal
      v-model:visible="showAgreements"
      :closable="false"
      :mask-closable="false"
      :footer="false"
      :width="680"
      title-align="start"
    >
      <template #title>
        <div class="flex items-center gap-2">
          <svg class="w-5 h-5 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span>请阅读并同意以下协议</span>
        </div>
      </template>

      <!-- Agreement tabs -->
      <div v-if="agreements.length > 1" class="agreement-tabs">
        <div
          v-for="item in agreements"
          :key="item.id"
          class="agreement-tab"
          :class="{ 'agreement-tab--active': selectedAgreement?.id === item.id }"
          @click="loadAgreementContent(item)"
        >
          {{ item.title }} ({{ item.version }})
        </div>
      </div>

      <!-- Agreement content -->
      <div class="agreement-modal-content">
        <ASpin :loading="agreementLoading" style="width: 100%">
          <div class="agreement-body" v-html="agreementContentHtml" />
        </ASpin>
      </div>

      <!-- Accept checkbox + button -->
      <div class="agreement-footer">
        <ACheckbox v-model="allChecked">
          我已阅读并同意以上{{ agreements.length > 1 ? '所有' : '' }}协议
        </ACheckbox>
        <AButton
          type="primary"
          :loading="acceptLoading"
          :disabled="!allChecked"
          @click="acceptAgreements"
        >
          同意并继续
        </AButton>
      </div>
    </AModal>
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

/* ===== 2FA 验证码 ===== */
.twofa {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.twofa__badge {
  display: grid;
  place-items: center;
  width: 56px;
  height: 56px;
  margin-bottom: 18px;
  border-radius: 18px;
  color: var(--ta-primary);
  background: linear-gradient(135deg, rgba(20, 184, 166, 0.14), rgba(13, 148, 136, 0.05));
  box-shadow: inset 0 0 0 1px rgba(13, 148, 136, 0.14);
}

.twofa__badge svg {
  width: 26px;
  height: 26px;
}

.twofa__title {
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: #0f172a;
  margin: 0 0 8px;
}

.twofa__subtitle {
  max-width: 300px;
  margin: 0 0 28px;
  font-size: 14px;
  line-height: 1.6;
  color: #94a3b8;
}

.twofa__otp {
  width: 100%;
}

/* 预留固定高度，避免错误提示出现时按钮跳动 */
.twofa__message {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 18px;
  margin-top: 8px;
}

.twofa__message-text {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--ta-danger);
}

.twofa__message-text svg {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
}

.twofa-msg-enter-active,
.twofa-msg-leave-active {
  transition: opacity 0.18s var(--ta-ease), transform 0.18s var(--ta-ease);
}

.twofa-msg-enter-from,
.twofa-msg-leave-to {
  opacity: 0;
  transform: translateY(-3px);
}

.twofa__submit {
  margin-top: 2px;
}

.twofa__back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 16px;
  padding: 6px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #64748b;
  font-family: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
}

.twofa__back svg {
  width: 14px;
  height: 14px;
}

.twofa__back:hover {
  color: var(--ta-primary);
  background: var(--ta-primary-light);
}

/* ===== Agreements Modal ===== */
.agreement-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.agreement-tab {
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 13px;
  color: #64748b;
  cursor: pointer;
  transition: all 0.15s;
}

.agreement-tab:hover {
  background: #f1f5f9;
  color: #334155;
}

.agreement-tab--active {
  background: #0d9488;
  color: #fff;
}

.agreement-tab--active:hover {
  background: #0d9488;
  color: #fff;
}

.agreement-modal-content {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 20px;
  max-height: 400px;
  overflow-y: auto;
  background: #f8fafc;
  line-height: 1.8;
  font-size: 14px;
  color: #334155;
}

.agreement-body :deep(h1),
.agreement-body :deep(h2),
.agreement-body :deep(h3) {
  margin-top: 16px;
  margin-bottom: 8px;
  color: #0f172a;
}

.agreement-body :deep(h1) { font-size: 20px; }
.agreement-body :deep(h2) { font-size: 17px; }
.agreement-body :deep(h3) { font-size: 15px; }

.agreement-body :deep(ul),
.agreement-body :deep(ol) {
  padding-left: 20px;
}

.agreement-body :deep(p) {
  margin-bottom: 8px;
}

.agreement-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid #e5e7eb;
}
</style>
