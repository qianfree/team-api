<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '@/stores/auth'
import { extractApiError } from '@/utils/request'
import request from '@/utils/request'
import SlideCaptcha from '@/components/SlideCaptcha.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const loading = ref(false)

const form = reactive({
  username: '',
  password: '',
})

// 2FA state
const show2FA = ref(false)
const provisionalToken = ref('')
const totpCode = ref('')
const totpLoading = ref(false)

// Captcha state
const captcha = reactive({ captchaKey: '', captchaX: 0 })
const captchaRef = ref<InstanceType<typeof SlideCaptcha> | null>(null)

async function handleLogin() {
  if (!form.username) {
    showToast('请输入用户名')
    return
  }
  if (!form.password) {
    showToast('请输入密码')
    return
  }

  loading.value = true
  try {
    const res = await authStore.login(form.username, form.password, {
      captchaKey: captcha.captchaKey,
      captchaX: captcha.captchaX,
    })
    // Check if 2FA is required
    if (res?.totp_required) {
      provisionalToken.value = res.provisional_token
      show2FA.value = true
      return
    }
    showToast('登录成功')
    const redirect = (route.query.redirect as string) || '/m/'
    router.push(redirect)
  } catch (err: any) {
    const apiErr = extractApiError(err)
    if (apiErr?.code === 10058) {
      showToast('滑块验证失败，请重新拖动')
    }
    captchaRef.value?.resetCaptcha()
  } finally {
    loading.value = false
  }
}

async function handle2FAVerify() {
  if (!totpCode.value) {
    showToast('请输入验证码')
    return
  }
  totpLoading.value = true
  try {
    const res = await request.post('/api/admin/auth/2fa/verify', {
      provisional_token: provisionalToken.value,
      code: totpCode.value,
    })
    authStore.applyTokensFrom2FA(res.data.data)
    showToast('登录成功')
    const redirect = (route.query.redirect as string) || '/m/'
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
    <!-- Header -->
    <div class="login-header">
      <img src="/favicon.png" alt="Team-API" class="login-logo" />
      <h1 class="login-title">Team-API</h1>
      <p class="login-subtitle">管理后台</p>
    </div>

    <!-- Login Form -->
    <div v-if="!show2FA" class="login-form">
      <van-cell-group inset>
        <van-field
          v-model="form.username"
          label="用户名"
          placeholder="请输入用户名"
          :border="true"
          autocomplete="username"
        />
        <van-field
          v-model="form.password"
          type="password"
          label="密码"
          placeholder="请输入密码"
          :border="true"
          autocomplete="current-password"
          @keyup.enter="handleLogin"
        />
      </van-cell-group>

      <div class="login-captcha">
        <SlideCaptcha ref="captchaRef" v-model="captcha" />
      </div>

      <div class="login-actions">
        <van-button
          type="primary"
          block
          round
          :loading="loading"
          loading-text="登录中..."
          @click="handleLogin"
        >
          登 录
        </van-button>
      </div>
    </div>

    <!-- 2FA Form -->
    <div v-else class="login-form">
      <div class="totp-header">
        <h3 class="totp-title">双因素认证</h3>
        <p class="totp-subtitle">请输入身份验证器中的 6 位数字码</p>
      </div>

      <van-cell-group inset>
        <van-field
          v-model="totpCode"
          center
          type="digit"
          label="验证码"
          placeholder="6 位数字"
          maxlength="6"
          @keyup.enter="handle2FAVerify"
        />
      </van-cell-group>

      <div class="login-actions">
        <van-button
          type="primary"
          block
          round
          :loading="totpLoading"
          loading-text="验证中..."
          @click="handle2FAVerify"
        >
          验证并登录
        </van-button>
        <van-button
          block
          round
          plain
          size="small"
          class="mt-3"
          @click="show2FA = false; totpCode = ''"
        >
          返回登录
        </van-button>
      </div>
    </div>

    <!-- Footer -->
    <div class="login-footer">
      <span>&copy; {{ new Date().getFullYear() }} qianfree</span>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(to bottom, #f0fdfa, #f8fafc);
  padding: 0 16px;
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 20vh;
  margin-bottom: 32px;
}

.login-logo {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  margin-bottom: 12px;
}

.login-title {
  font-size: 28px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.02em;
}

.login-subtitle {
  font-size: 14px;
  color: #94a3b8;
  margin-top: 4px;
}

.login-form {
  flex: 1;
}

.login-captcha {
  padding: 0 16px;
  margin: 16px 0;
}

.login-actions {
  padding: 0 16px;
  margin-top: 24px;
}

.mt-3 {
  margin-top: 12px;
}

.totp-header {
  text-align: center;
  margin-bottom: 24px;
}

.totp-title {
  font-size: 20px;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 4px;
}

.totp-subtitle {
  font-size: 13px;
  color: #94a3b8;
}

.login-footer {
  text-align: center;
  padding: 24px 0;
  font-size: 11px;
  color: #94a3b8;
}
</style>
