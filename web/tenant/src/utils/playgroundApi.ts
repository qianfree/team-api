import axios from 'axios'
import type { AxiosInstance } from 'axios'

/**
 * createPlaygroundApi 创建一个独立的 axios 实例，用于 Playground 直接调用 /v1/* 端点。
 *
 * 与主 request.ts 的区别：
 * - 使用 API Key（sk-xxx）认证，而非 JWT
 * - 不走统一响应格式拦截（/v1/* 返回原生 OpenAI 格式）
 * - 错误格式为 {error: {type, message}}
 */
export function createPlaygroundApi(apiKey: string): AxiosInstance {
	const instance = axios.create({
		timeout: 60_000,
		headers: {
			'Content-Type': 'application/json',
			'Authorization': `Bearer ${apiKey}`,
		},
	})

	instance.interceptors.response.use(
		(response) => response,
		(error) => {
			let data = error.response?.data
			// arraybuffer 请求（如 /v1/audio/speech 的 TTS）的错误响应同为 ArrayBuffer，
			// 需先解码为文本再解析 JSON，否则真实错误原因丢失、只剩 axios 通用文案
			if (data instanceof ArrayBuffer) {
				try {
					data = JSON.parse(new TextDecoder().decode(data))
				} catch {
					data = undefined // 非 JSON 错误体，走通用错误分支
				}
			}
			// 解析 OpenAI 错误格式: {error: {type, message, code}}
			if (data?.error) {
				const relayError = data.error
				const msg = relayError.message || '请求失败'
				const err = new Error(msg)
				;(err as any).relayError = relayError
				return Promise.reject(err)
			}
			// 网络或其它错误
			if (!error.response) {
				return Promise.reject(new Error('网络连接异常，请稍后重试'))
			}
			return Promise.reject(error)
		},
	)

	return instance
}
