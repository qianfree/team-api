import type { GlobalThemeOverrides } from 'naive-ui'

/**
 * Naive UI 主题覆盖——对齐租户端现有品牌视觉（styles/main.css 的 @theme 主色）。
 * 主色沿用青绿系：#14b8a6（primary-500）→ #0d9488（hover）→ #0f766e（pressed）。
 * 控制件圆角沿用现有 rounded-lg（0.5rem）/ 卡片 rounded-xl（0.75rem）。
 */
export const themeOverrides: GlobalThemeOverrides = {
	common: {
		primaryColor: '#14b8a6',
		primaryColorHover: '#0d9488',
		primaryColorPressed: '#0f766e',
		primaryColorSuppl: '#14b8a6',
		infoColor: '#3B82F6',
		successColor: '#10B981',
		warningColor: '#F59E0B',
		errorColor: '#EF4444',
		borderRadius: '8px',
		borderRadiusSmall: '6px',
		fontWeightStrong: '600',
	},
	// 弹窗卡片（NModal preset=card / NDialog）细化为更大的卡片圆角
	Card: {
		borderRadius: '12px',
	},
	Dialog: {
		borderRadius: '12px',
	},
	Button: {
		borderRadiusMedium: '8px',
		borderRadiusLarge: '10px',
	},
	Select: {
		borderRadius: '8px',
	},
	Pagination: {
		borderRadius: '8px',
	},
	Message: {
		borderRadius: '10px',
	},
}