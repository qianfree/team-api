// 模型分类共享常量（管理后台模型管理 + 租户详情模型分配共用）
// value 与后端 mdl_models.category 枚举一一对应（api/admin/v1/model.go 的 v:"in:chat,embedding,image,audio,rerank,video" 校验）。
// 新增分类时需同步：后端枚举校验、本文件。

export const modelCategoryOptions: { label: string; value: string }[] = [
  { label: '对话', value: 'chat' },
  { label: '向量', value: 'embedding' },
  { label: '图像', value: 'image' },
  { label: '音频', value: 'audio' },
  { label: '视频', value: 'video' },
  { label: '重排', value: 'rerank' },
]

// value -> label 映射，表格列展示用
export const modelCategoryLabelMap: Record<string, string> = {}
modelCategoryOptions.forEach(o => { modelCategoryLabelMap[o.value] = o.label })

// value -> Arco Tag 预设色映射
export const modelCategoryTagColor: Record<string, string> = {
  chat: 'arcoblue',
  embedding: 'green',
  image: 'orangered',
  audio: 'red',
  video: 'purple',
  rerank: 'arcoblue',
}
