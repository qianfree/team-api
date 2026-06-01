import { useSeoMeta, useHead } from '@unhead/vue'

interface SeoOptions {
	title: string
	description: string
	keywords?: string
	canonicalUrl?: string
	ogImage?: string
	ogType?: 'website' | 'article' | 'profile' | 'book'
}

const SITE_NAME = 'Team-API'
const DEFAULT_OG_IMAGE = '/og-image.png'

/**
 * 集中管理页面 SEO 头部信息
 * 封装 @unhead/vue 的 useSeoMeta 和 useHead，统一管理 title、description、OG、Twitter Card、canonical URL
 */
export function useSeo(options: SeoOptions) {
	const {
		title,
		description,
		keywords,
		canonicalUrl,
		ogImage = DEFAULT_OG_IMAGE,
		ogType = 'website',
	} = options

	useSeoMeta({
		title,
		ogTitle: title,
		description,
		ogDescription: description,
		keywords,
		ogType,
		ogSiteName: SITE_NAME,
		ogLocale: 'zh_CN',
		ogImage,
		twitterCard: 'summary_large_image',
		twitterTitle: title,
		twitterDescription: description,
		twitterImage: ogImage,
	})

	if (canonicalUrl) {
		useHead({
			link: [
				{ rel: 'canonical', href: canonicalUrl },
			],
		})
	}
}
