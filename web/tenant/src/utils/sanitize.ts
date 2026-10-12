import DOMPurify from 'dompurify'

// 所有 v-html 渲染点的统一净化入口：marked 默认放行原始 HTML，
// 内容即使来自管理员配置或通知模板，也必须先过白名单再进 v-html，防存储型 XSS
export function sanitizeHtml(html: string): string {
	if (!html) return ''
	return DOMPurify.sanitize(html)
}
