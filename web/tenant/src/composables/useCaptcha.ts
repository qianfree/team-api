import { ref } from 'vue'
import request from '@/utils/request'

export interface CaptchaData {
	captcha_key: string
	master_image: string
	tile_image: string
	tile_y: number
}

export type CaptchaStatus = 'idle' | 'verifying' | 'success' | 'failed'

export function useCaptcha() {
	const captchaData = ref<CaptchaData | null>(null)
	const loading = ref(false)
	const error = ref('')
	const status = ref<CaptchaStatus>('idle')

	async function fetchCaptcha() {
		loading.value = true
		error.value = ''
		status.value = 'idle'
		try {
			const { data } = await request.get('/captcha')
			if (data.code === 0) {
				captchaData.value = data.data
			} else {
				error.value = data.message || '获取验证码失败'
			}
		} catch (e: any) {
			error.value = '获取验证码失败'
		} finally {
			loading.value = false
		}
	}

	async function verify(captchaKey: string, captchaX: number): Promise<boolean> {
		status.value = 'verifying'
		try {
			const { data } = await request.post('/captcha/verify', {
				captcha_key: captchaKey,
				captcha_x: captchaX,
			})
			if (data.code === 0 && data.data?.verified) {
				status.value = 'success'
				return true
			}
			status.value = 'failed'
			return false
		} catch (e: any) {
			status.value = 'failed'
			return false
		}
	}

	function resetCaptcha() {
		fetchCaptcha()
	}

	function reset() {
		captchaData.value = null
		status.value = 'idle'
		error.value = ''
	}

	return {
		captchaData,
		loading,
		error,
		status,
		fetchCaptcha,
		verify,
		resetCaptcha,
		reset,
	}
}
