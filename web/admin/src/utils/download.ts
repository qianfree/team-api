/**
 * 通用「文本内容 → 文件下载」：前端生成 Blob 触发浏览器保存，无后端参与。
 * 适用于源码模板、源码导出等小文本；大文件/服务端生成文件请走 useExport（blob 响应）。
 */
export function downloadTextFile(
  content: string,
  fileName: string,
  mimeType = 'text/plain;charset=utf-8',
): void {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
