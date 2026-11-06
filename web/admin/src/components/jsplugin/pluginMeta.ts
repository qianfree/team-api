import type { Component } from 'vue'
import {
  IconApps, IconVideoCamera, IconMusic, IconImage, IconRobot, IconCode,
  IconThunderbolt, IconFire, IconCloud, IconTool, IconBulb, IconPalette,
  IconStorage, IconNotification, IconDashboard, IconCommand,
} from '@arco-design/web-vue/es/icon'

/** 管理后台 JS 任务插件列表项（对应 api/admin/v1.JSPluginItem） */
export interface JSPluginVersion {
  id: number
  version: string
  origin: string // builtin=内置（不可删除）| upload
  active: boolean
  enabled: boolean
  remark: string
  created_at: string
}

export interface JSPluginUsageField {
  key: string
  type: string // number（可计费）| enum
  unit?: string
  description?: string
}

export interface JSPlugin {
  key: string
  name: string
  icon: string
  has_image: boolean
  description: string
  author: string
  website: string
  origin: string // 激活版本来源：builtin | upload
  active_version: string
  enabled: boolean
  in_service: boolean
  models: string[]
  fetch_mode: string
  allowed_hosts: string[]
  usage_schema: JSPluginUsageField[]
  versions: JSPluginVersion[]
}

export interface JSPluginRoute {
  method: string
  path: string
  type: string
}

export interface JSPluginMetricsHook {
  calls: number
  errors: number
  error_rate: number
  avg_ms: number
  last_err?: string
  last_err_at?: string
}

export interface JSPluginMetricsItem {
  key: string
  in_service: boolean
  live_instances: number
  total_calls: number
  total_errors: number
  avg_ms: number
  hooks: Record<string, JSPluginMetricsHook>
  tasks_total: number
  tasks_success: number
  tasks_active: number
  tasks_failed: number
  artifacts_stored: number
  artifacts_failed: number
}

// ── 卡片图标 ──────────────────────────────────────────────
// manifest.icon 是「图标名 / text / text:label」，上传时也可存 SVG/PNG 图标
const PLUGIN_ICON_MAP: Record<string, Component> = {
  video: IconVideoCamera,
  'video-camera': IconVideoCamera,
  music: IconMusic,
  image: IconImage,
  robot: IconRobot,
  ai: IconRobot,
  code: IconCode,
  app: IconApps,
  apps: IconApps,
  thunderbolt: IconThunderbolt,
  fire: IconFire,
  cloud: IconCloud,
  tool: IconTool,
  bulb: IconBulb,
  palette: IconPalette,
  storage: IconStorage,
  notification: IconNotification,
  dashboard: IconDashboard,
  command: IconCommand,
}

export interface ResolvedIcon {
  kind: 'img' | 'icon' | 'text'
  comp?: Component
  label: string
}

export function initialOf(p: Pick<JSPlugin, 'key' | 'name'>): string {
  const name = p.name || p.key || '?'
  return name.replace(/[^a-zA-Z一-龥]/g, '').slice(0, 1).toUpperCase() || '?'
}

export function resolvePluginIcon(p: JSPlugin): ResolvedIcon {
  if (p.has_image) return { kind: 'img', label: '' }
  const icon = String(p.icon || '')
  if (icon.startsWith('text:')) return { kind: 'text', label: icon.slice(5) }
  if (icon === 'text') return { kind: 'text', label: initialOf(p) }
  const comp = PLUGIN_ICON_MAP[icon]
  if (comp) return { kind: 'icon', comp, label: '' }
  return { kind: 'text', label: initialOf(p) }
}

// ── 运行状态 ──────────────────────────────────────────────
// 三态来自 registry 的双开关：enabled（是否进 generation）+ 激活版本是否存在。
export type PluginStatusKey = 'live' | 'pending' | 'off'

export interface PluginStatusMeta {
  key: PluginStatusKey
  label: string
  hint: string
  tagColor?: string
}

export function pluginStatus(
  p: Pick<JSPlugin, 'in_service' | 'enabled' | 'active_version'>,
): PluginStatusMeta {
  if (p.in_service) {
    return { key: 'live', label: '服务中', hint: '路由已挂载，客户端可直接调用', tagColor: 'green' }
  }
  if (p.enabled) {
    return {
      key: 'pending',
      label: '待生效',
      hint: p.active_version
        ? '已启用但 generation 尚未重载，稍候会自动生效'
        : '已启用但没有激活版本，无法生成路由',
      tagColor: 'orange',
    }
  }
  return { key: 'off', label: '未启用', hint: '未加入 generation，/pjs 下无对应路由', tagColor: undefined }
}

export function fetchModeLabel(mode?: string): string {
  return mode === 'batch' ? '批量轮询' : '逐任务轮询'
}

/** 从 metrics 里取最近一条钩子错误，用于「最近错误」列 */
export function latestHookError(m: JSPluginMetricsItem): { hook: string; message: string } | null {
  const entries = Object.entries(m.hooks || {}).filter(([, v]) => v?.last_err)
  if (entries.length === 0) return null
  entries.sort((a, b) => String(b[1].last_err_at || '').localeCompare(String(a[1].last_err_at || '')))
  const [hook, v] = entries[0]
  return { hook, message: v.last_err || '' }
}
