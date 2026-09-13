import type { Component } from 'vue'
import MinimaxMaterialEditor from './pricingSchemes/MinimaxMaterialEditor.vue'

// 计费方案 → 定价编辑器组件映射（特殊计费方案的定价表单在此登记）。
// 后端 pricing JSONB 顶层 scheme 键声明方案名，ModelPricingModal 据此分发内容区；
// 空名/未登记的方案回落内置通用表单。自定义方案建议 custom: 前缀命名，避免与内置方案撞名。
//
// 编辑器契约（新增方案组件只需满足此契约并在下方登记一行）：
//   props:  { modelId: number, schemeConfig: any }  // schemeConfig = 库内已有配置初值（可为 null）
//   expose: save(): Promise<boolean>                // 校验 + PUT 保存；失败自行 Message 提示并返回 false
//   保存成功后的提示与关弹窗由 ModelPricingModal 外壳统一处理，组件无需 emit
const schemeEditors: Record<string, Component> = {
	// MiniMax-H3 素材计费（输出按秒 + 输入图片/视频素材）
	'custom:minimax-material': MinimaxMaterialEditor,
}

// 特殊计费方案选项（新增方案在此登记一行即出现在定价面板的模式下拉中）。
// modelPrefixes 为模型编码前缀白名单（大小写不敏感）：仅命中的模型才显示该特殊计费选项，
// 其余模型只有通用计费，防止把专属方案误配到不适用的模型上。
export interface SchemeEditorOption {
	label: string
	value: string
	modelPrefixes: string[]
}

export const schemeEditorOptions: SchemeEditorOption[] = [
	{ label: '素材计费（MiniMax）', value: 'custom:minimax-material', modelPrefixes: ['MiniMax-H3'] },
]

// availableSchemeOptions 按模型编码过滤可用的特殊计费选项
export function availableSchemeOptions(modelIdStr?: string | null): SchemeEditorOption[] {
	if (!modelIdStr) return []
	const lower = modelIdStr.toLowerCase()
	return schemeEditorOptions.filter((o) => o.modelPrefixes.some((p) => lower.startsWith(p.toLowerCase())))
}

// resolvePricingSchemeEditor 按方案名取编辑器组件；null = 使用通用表单
export function resolvePricingSchemeEditor(scheme?: string | null): Component | null {
	if (!scheme) return null
	return schemeEditors[scheme] ?? null
}
