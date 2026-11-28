<script setup lang="ts">
// JS 插件详情抽屉：摘要 + 快捷操作 + 基本信息 + 计费单位 + 路由 + 版本历史。
// 数据以 props.plugin 为单一来源（父页面列表对象），激活/启停后 emit('changed')
// 由父页面刷新列表，prop 更新后自动重算，无需抽屉内维护副本。
//
// 布局取向：顶部「名片卡」承载描述 / 主操作 / 运行提示，其余信息按
// 只读规格表 + 路由清单 + 版本列表逐段下沉；版本历史用自定义列表而非 Table
// —— 抽屉定宽 720（内容 ~680），固定列宽表格必然横向溢出、操作列被裁。
import { computed, ref, toRef, watch } from 'vue'
import { Button, Drawer, Message, Modal, Popconfirm } from '@arco-design/web-vue'
import { IconClose, IconCopy, IconLink, IconRobot } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { downloadTextFile } from '@/utils/download'
import JSPluginSourceModal from './JSPluginSourceModal.vue'
import {
  pluginStatus, resolvePluginIcon, fetchModeLabel,
  type JSPlugin, type JSPluginRoute, type JSPluginVersion,
} from './pluginMeta'
import { usePluginIcon } from './usePluginIcon'
import { hasPermission } from '@/utils/permission'
import { useIsMobile } from '@/composables/useIsMobile'

const props = defineProps<{
  visible: boolean
  plugin: JSPlugin | null
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'toggle', enabled: boolean): void
  (e: 'config'): void
  (e: 'changed'): void
  (e: 'aiUpgrade', payload: { key: string; version: string; source: string }): void
}>()

const routes = ref<JSPluginRoute[]>([])
const routesLoading = ref(false)
// 行内操作态按 `${version}:activate|remove` 记账：激活与删除互不顶掉 loading
const busy = ref('')
function isBusy(version: string, action: 'activate' | 'remove') {
  return busy.value === `${version}:${action}`
}
// 源码下载单独记账：与激活/删除并行时不互相清空 loading
const downloadingVersion = ref('')
const isMobile = useIsMobile()

// 源码查看弹窗状态：sourceVersion 记录待查看版本号
const sourceVisible = ref(false)
const sourceVersion = ref('')

function viewSource(version: string) {
  sourceVersion.value = version
  sourceVisible.value = true
}

// AI 升级：拉取激活版本源码交给父页面带入生成工作台（新会话 + 基线版本 + 聊天修改）
const aiUpgrading = ref(false)
async function startAiUpgrade() {
  const p = props.plugin
  if (!p?.active_version || aiUpgrading.value) return
  aiUpgrading.value = true
  try {
    const res: any = await request.get(`/admin/js-plugins/${p.key}/versions/${p.active_version}/source`)
    const d = res.data?.data
    if (!d?.source) {
      Message.warning('未获取到源码内容')
      return
    }
    emit('aiUpgrade', { key: p.key, version: p.active_version, source: d.source })
  } catch {
    // 拦截器已提示
  } finally {
    aiUpgrading.value = false
  }
}

const status = computed(() => (props.plugin ? pluginStatus(props.plugin) : null))
const icon = computed(() => (props.plugin ? resolvePluginIcon(props.plugin) : null))
const versions = computed<JSPluginVersion[]>(() => props.plugin?.versions || [])

// 品牌图标经 blob 加载（端点要求 Authorization 头，<img> 直连必 401）
const iconURL = usePluginIcon(toRef(props, 'plugin'))

// 与后端 rbac 对齐：激活/启停需 plugin:manage，删除版本需 plugin:upload
const canManage = computed(() => hasPermission('plugin:manage'))
const canUpload = computed(() => hasPermission('plugin:upload'))

// 纯上传插件（所有版本均非内置）才提供整删入口；含内置版本的 key 只能逐个删除
// 版本（上传版本 + 过期内置版本，当前二进制内置版本不可删）
const canPurge = computed(() =>
  versions.value.length > 0 && versions.value.every((v) => v.origin !== 'builtin'),
)

// 版本删除确认文案：过期内置版本强调「不会随重启复活」，上传版本强调「源码不可恢复」
function removeConfirmText(v: JSPluginVersion) {
  if (v.origin === 'builtin') {
    return `删除过期内置版本 ${v.version}？该版本已不在当前程序内，删除后不会随重启恢复。`
  }
  return `删除版本 ${v.version}？源码不可恢复。`
}

// ── 整删插件 ──────────────────────────────────────────────
// 点击先取预检（版本数 + 在途任务数），确认弹窗展示后果后才发 DELETE
const purgeVisible = ref(false)
const purgeChecking = ref(false)
const purgeLoading = ref(false)
const purgeCheck = ref<{ version_count: number; pending_tasks: number } | null>(null)

async function openPurge() {
  if (!props.plugin) return
  purgeChecking.value = true
  try {
    const res: any = await request.get(`/admin/js-plugins/${props.plugin.key}/delete-check`)
    const d = res.data?.data
    if (!d?.allowed) {
      Message.warning(d?.reason || '当前状态不允许删除该插件')
      return
    }
    purgeCheck.value = d
    purgeVisible.value = true
  } catch {
    // 拦截器已提示
  } finally {
    purgeChecking.value = false
  }
}

async function confirmPurge() {
  if (!props.plugin) return
  purgeLoading.value = true
  try {
    await request.delete(`/admin/js-plugins/${props.plugin.key}`)
    Message.success('已删除该插件（含全部版本与配置）')
    purgeVisible.value = false
    // 插件已不存在：列表刷新后详情数据源为空，直接关抽屉
    emit('update:visible', false)
    emit('changed')
  } catch {
    // 拦截器已提示
  } finally {
    purgeLoading.value = false
  }
}

// 版本时间只保留到分钟：秒对「哪次上传」没有区分意义，却会把一行撑长
function fmtTime(s?: string) {
  if (!s) return '—'
  return s.replace(/:\d{2}$/, '')
}

const latestUpload = computed(() => {
  const list = versions.value.map((v) => v.created_at || '').filter(Boolean).sort()
  return list.length ? list[list.length - 1] : ''
})

// 状态不入基本信息：抽屉头部已有同源状态药丸，重复列一次是噪声；
// 换成「最近上传」——版本时间线里最有用的那一格。
const basicItems = computed(() => {
  const p = props.plugin
  if (!p) return []
  return [
    { label: '插件标识', value: p.key, mono: true },
    { label: '激活版本', value: p.active_version || '—', mono: true },
    { label: '版本数量', value: `${versions.value.length} 个` },
    { label: '轮询模式', value: fetchModeLabel(p.fetch_mode) },
    { label: '作者', value: p.author || '—' },
    { label: '最近上传', value: fmtTime(latestUpload.value) || '—', mono: true },
  ]
})

async function loadRoutes(key: string) {
  routes.value = []
  routesLoading.value = true
  try {
    const res: any = await request.get(`/admin/js-plugins/${key}/routes`)
    routes.value = res.data?.data?.list || res.data?.list || []
  } catch {
    routes.value = []
  } finally {
    routesLoading.value = false
  }
}

watch(
  () => [props.visible, props.plugin?.key, props.plugin?.active_version] as const,
  ([visible, key]) => {
    if (visible && key) loadRoutes(key)
    else routes.value = []
    // 抽屉关闭同步复位嵌套源码弹窗：unmount-on-close 只卸载 body 槽内容，
    // 弹窗开着关抽屉时 sourceVisible 残留 true，下次开抽屉会莫名弹出旧源码
    if (!visible) {
      sourceVisible.value = false
      sourceVersion.value = ''
    }
  },
  { immediate: true },
)

function close() {
  emit('update:visible', false)
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Message.success('已复制')
  } catch {
    Message.warning('复制失败，请手动选中')
  }
}

async function activate(version: string) {
  if (!props.plugin) return
  busy.value = `${version}:activate`
  try {
    await request.post(`/admin/js-plugins/${props.plugin.key}/versions/${version}/activate`)
    Message.success(`已激活 ${version}`)
    emit('changed')
  } catch {
    // 拦截器已提示
  } finally {
    busy.value = ''
  }
}

async function removeVersion(version: string) {
  if (!props.plugin) return
  busy.value = `${version}:remove`
  try {
    await request.delete(`/admin/js-plugins/${props.plugin.key}/versions/${version}`)
    Message.success('已删除该版本')
    emit('changed')
  } catch {
    // 拦截器已提示
  } finally {
    busy.value = ''
  }
}

// 版本列表行内下载：拉取源码全文直接存为 {key}-{version}.js（不打开源码弹窗）
async function downloadSource(version: string) {
  if (!props.plugin) return
  downloadingVersion.value = version
  try {
    const res: any = await request.get(`/admin/js-plugins/${props.plugin.key}/versions/${version}/source`)
    const d = res.data?.data
    if (!d?.source) {
      Message.warning('未获取到源码内容')
      return
    }
    downloadTextFile(
      d.source,
      `${d.key || props.plugin.key}-${d.version || version}.js`,
      'text/javascript;charset=utf-8',
    )
  } catch {
    // 拦截器已提示
  } finally {
    downloadingVersion.value = ''
  }
}
</script>

<template>
  <Drawer
    :visible="visible"
    :width="isMobile ? '100%' : 720"
    :footer="false"
    :header="false"
    :body-style="{ padding: '0' }"
    unmount-on-close
    @cancel="close"
  >
    <div v-if="plugin" class="jd-body">
      <!-- 自绘头部：Arco 原生 header 写死 48px、padding 0 16px，46px 图标 + 两行标识
           必然溢出被裁；且 Teleport 内的节点拿不到 scoped 属性，无法用 :deep 覆盖。
           自绘还能让头部固定、正文独立滚动。 -->
      <header class="jd-top">
        <div class="jd-icon" :class="[`jd-icon--${icon?.kind}`, `jd-icon--${plugin.origin}`]">
          <img v-if="icon?.kind === 'img' && iconURL" :src="iconURL" :alt="plugin.name" />
          <component :is="icon?.comp" v-else-if="icon?.kind === 'icon'" :size="24" />
          <span v-else>{{ icon?.label || (plugin.name || plugin.key).slice(0, 1) }}</span>
        </div>
        <div class="jd-ident">
          <div class="jd-name-row">
            <span class="jd-name">{{ plugin.name || plugin.key }}</span>
            <span class="jd-pill" :class="plugin.origin === 'builtin' ? 'is-builtin' : 'is-muted'">
              {{ plugin.origin === 'builtin' ? '内置' : '上传' }}
            </span>
          </div>
          <button
            type="button"
            class="jd-key"
            :title="`复制插件标识 ${plugin.key}`"
            @click="copy(plugin.key)"
          >
            <code>{{ plugin.key }}</code>
            <IconCopy class="jd-key__icon" />
          </button>
        </div>
        <span v-if="status" class="jd-status" :class="`is-${status.key}`" :title="status.hint">
          <i />{{ status.label }}
        </span>
        <button type="button" class="jd-close" title="关闭" aria-label="关闭" @click="close">
          <IconClose />
        </button>
      </header>

      <div class="jd-main">
        <!-- 名片卡：描述 / 可直接执行的动作 / 运行态说明 -->
        <div class="jd-hero">
          <p class="jd-desc">{{ plugin.description || '该插件未填写描述。' }}</p>

          <div class="jd-actions">
            <div class="jd-actions__main">
              <template v-if="canManage">
                <Popconfirm
                  v-if="plugin.enabled"
                  content="禁用后路由会被立即摘除，确定？"
                  @ok="emit('toggle', false)"
                >
                  <Button type="outline" status="warning">禁用插件</Button>
                </Popconfirm>
                <Button v-else type="primary" @click="emit('toggle', true)">启用插件</Button>
              </template>
              <Button
                v-if="plugin.active_version"
                type="secondary"
                @click="viewSource(plugin.active_version)"
              >查看源码</Button>
              <Button
                v-if="plugin.active_version"
                type="secondary"
                :loading="aiUpgrading"
                title="带入 AI 插件生成工作台：以当前源码为基线，通过聊天描述升级需求，AI 定点修改"
                @click="startAiUpgrade"
              >
                <template #icon><IconRobot /></template>AI 升级
              </Button>
              <Button type="secondary" @click="emit('config')">配置</Button>
            </div>
            <div class="jd-actions__aux">
              <Button
                v-if="plugin.website"
                type="text"
                :href="plugin.website"
                target="_blank"
                rel="noopener"
              >
                <template #icon><IconLink /></template>官网
              </Button>
              <span v-if="canPurge && canUpload" class="jd-actions__split" />
              <Button
                v-if="canPurge && canUpload"
                type="text"
                status="danger"
                :loading="purgeChecking"
                @click="openPurge"
              >删除插件</Button>
            </div>
          </div>

          <p v-if="status" class="jd-hint" :class="`is-${status.key}`">
            <i />{{ status.hint }}
          </p>
        </div>

        <section class="jd-section">
          <h4 class="jd-title">基本信息</h4>
          <div class="jd-spec">
            <div v-for="item in basicItems" :key="item.label" class="jd-cell">
              <span class="jd-cell__label">{{ item.label }}</span>
              <span class="jd-cell__value" :class="{ 'is-mono': item.mono }" :title="item.value">
                {{ item.value }}
              </span>
            </div>
          </div>
        </section>

        <section class="jd-section">
          <h4 class="jd-title">
            支持的上游模型
            <span class="jd-title__hint">渠道模型映射须落在此清单内，同一上游模型可由多插件提供</span>
          </h4>
          <div v-if="(plugin.models || []).length" class="jd-chips">
            <span v-for="m in plugin.models" :key="m" class="jd-chip">{{ m }}</span>
          </div>
          <p v-else class="jd-empty">未声明上游模型。</p>
        </section>

        <section v-if="(plugin.allowed_hosts || []).length" class="jd-section">
          <h4 class="jd-title">额外出网白名单</h4>
          <div class="jd-chips">
            <span v-for="host in plugin.allowed_hosts" :key="host" class="jd-chip">{{ host }}</span>
          </div>
        </section>

        <section class="jd-section">
          <h4 class="jd-title">调用路由</h4>
          <div v-if="routesLoading" class="jd-empty">加载中…</div>
          <div v-else-if="!routes.length" class="jd-empty">该插件未声明路由。</div>
          <ul v-else class="jd-routes">
            <li v-for="r in routes" :key="`${r.method}${r.path}`" class="jd-route">
              <span class="jd-route__type" :class="`is-${r.type}`">{{ r.type }}</span>
              <span class="jd-route__method">{{ r.method }}</span>
              <code class="jd-route__path" :title="r.path">{{ r.path }}</code>
              <Button
                size="mini"
                type="text"
                class="jd-route__copy"
                title="复制路由"
                @click="copy(r.path)"
              >
                <template #icon><IconCopy /></template>
              </Button>
            </li>
          </ul>
          <p class="jd-note">
            <template v-if="plugin.in_service">客户端按上述地址调用，鉴权方式与 /v1 一致（Bearer API Key）。</template>
            <template v-else>路由声明取自插件源码 manifest；插件当前未启用，<code>/pjs/{{ plugin.key }}/</code> 下暂未挂载，启用后即按上述地址生效。</template>
          </p>
        </section>

        <section class="jd-section">
          <h4 class="jd-title">
            版本历史
            <span class="jd-title__hint">同 key + version 源码一致才允许重复上传</span>
          </h4>
          <ul v-if="versions.length" class="jd-vers">
            <li
              v-for="v in versions"
              :key="v.id"
              class="jd-ver"
              :class="{ 'is-current': v.active }"
            >
              <div class="jd-ver__info">
                <div class="jd-ver__head">
                  <span class="jd-ver__no">{{ v.version }}</span>
                  <span v-if="v.active" class="jd-pill is-live">激活中</span>
                  <span v-else-if="v.enabled" class="jd-pill is-muted">已启用</span>
                  <span v-else class="jd-pill is-idle">未使用</span>
                </div>
                <div class="jd-ver__meta" :title="v.remark || ''">
                  <span :class="v.origin === 'builtin' ? 'is-builtin' : ''">
                    {{ v.origin === 'builtin' ? '内置' : '上传' }}
                  </span>
                  <span v-if="v.stale" class="is-stale">已过期</span>
                  · {{ fmtTime(v.created_at) }}<template v-if="v.remark"> · {{ v.remark }}</template>
                </div>
              </div>
              <div class="jd-ver__ops">
                <Button size="mini" type="text" @click="viewSource(v.version)">源码</Button>
                <Button
                  size="mini"
                  type="text"
                  :loading="downloadingVersion === v.version"
                  @click="downloadSource(v.version)"
                >下载</Button>
                <template v-if="!v.active">
                  <Popconfirm
                    v-if="canManage"
                    :content="`激活 ${v.version}？当前激活版本将被替换。`"
                    @ok="activate(v.version)"
                  >
                    <Button
                      size="mini"
                      type="primary"
                      :loading="isBusy(v.version, 'activate')"
                    >激活</Button>
                  </Popconfirm>
                  <Popconfirm
                    v-if="v.origin !== 'builtin' || v.stale"
                    :content="removeConfirmText(v)"
                    @ok="removeVersion(v.version)"
                  >
                    <Button
                      size="mini"
                      type="text"
                      status="danger"
                      :loading="isBusy(v.version, 'remove')"
                    >删除</Button>
                  </Popconfirm>
                </template>
              </div>
            </li>
          </ul>
          <p v-else class="jd-empty">暂无版本记录。</p>
        </section>
      </div>
    </div>

    <JSPluginSourceModal
      :visible="sourceVisible"
      :plugin-key="plugin?.key || ''"
      :version="sourceVersion"
      @update:visible="(v: boolean) => (sourceVisible = v)"
    />

    <!-- 整删确认：预检数据（版本数 / 在途任务数）在此展示，确认后才发 DELETE -->
    <Modal
      :visible="purgeVisible"
      title="删除插件"
      ok-text="确认删除"
      :ok-loading="purgeLoading"
      :ok-button-props="{ status: 'danger' }"
      :mask-closable="false"
      @ok="confirmPurge"
      @cancel="purgeVisible = false"
    >
      <p class="jp-purge__lead" v-if="plugin">
        将删除插件 <b>{{ plugin.name || plugin.key }}</b>（<code>{{ plugin.key }}</code>）的
        <b>{{ purgeCheck?.version_count ?? 0 }}</b> 个版本，源码、图标与配置一并清除，不可恢复。
      </p>
      <p v-if="(purgeCheck?.pending_tasks || 0) > 0" class="jp-purge__warn">
        有 {{ purgeCheck?.pending_tasks }} 个在途任务：删除后轮询等不到插件回归，
        将由 30 分钟超时网判为失败并退款。
      </p>
      <p class="jp-purge__note">历史任务、计费与用量记录保留，不受影响。</p>
    </Modal>
  </Drawer>
</template>

<style scoped>
/* ── 骨架：固定头部 + 独立滚动正文 ─────────────────────── */
.jd-body {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--color-bg-1);
}

.jd-top {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px 14px 20px;
  border-bottom: 1px solid var(--color-border);
}

.jd-main {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 20px 24px;
}

.jd-close {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  margin-left: 2px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--color-text-3);
  font-size: 14px;
  cursor: pointer;
  transition: background var(--ta-duration-fast) var(--ta-ease),
    color var(--ta-duration-fast) var(--ta-ease);
}

.jd-close:hover {
  background: var(--color-fill-2);
  color: var(--color-text-1);
}

/* ── 头部 ─────────────────────────────────────────────── */
.jd-icon {
  width: 46px;
  height: 46px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: #fff;
  font-size: 19px;
  font-weight: 700;
  overflow: hidden;
}

.jd-icon--icon,
.jd-icon--text {
  background: linear-gradient(135deg, #0d9488, #2dd4bf);
}

.jd-icon--icon.jd-icon--builtin,
.jd-icon--text.jd-icon--builtin {
  background: linear-gradient(135deg, #7c3aed, #a78bfa);
}

.jd-icon--img {
  background: var(--color-fill-2);
  border: 1px solid var(--color-border);
}

.jd-icon--img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  padding: 4px;
}

.jd-ident {
  flex: 1;
  min-width: 0;
}

.jd-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.jd-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* key 做成可点击的复制 chip：标识是排查问题时最常被复制的一项 */
.jd-key {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin-top: 3px;
  padding: 0;
  border: 0;
  background: none;
  cursor: pointer;
  color: var(--color-text-3);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
}

.jd-key code {
  font-family: inherit;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-key__icon {
  flex-shrink: 0;
  font-size: 11px;
  opacity: 0;
  transition: opacity var(--ta-duration-fast) var(--ta-ease);
}

.jd-key:hover {
  color: var(--color-text-2);
}

.jd-key:hover .jd-key__icon {
  opacity: 1;
}

.jd-status {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jd-status i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.jd-status.is-live {
  color: #059669;
  background: rgba(16, 185, 129, 0.12);
}

.jd-status.is-pending {
  color: #b45309;
  background: rgba(245, 158, 11, 0.14);
}

/* ── 通用药丸（来源 / 版本状态）───────────────────────── */
.jd-pill {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  height: 19px;
  padding: 0 7px;
  border-radius: 5px;
  font-size: 11.5px;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jd-pill.is-live {
  color: #059669;
  background: rgba(16, 185, 129, 0.12);
}

.jd-pill.is-builtin {
  color: #7c3aed;
  background: rgba(124, 58, 237, 0.09);
}

.jd-pill.is-muted {
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jd-pill.is-idle {
  color: var(--color-text-4);
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--color-border);
}

/* ── 名片卡 ───────────────────────────────────────────── */
.jd-hero {
  padding: 14px 16px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--ta-radius);
}

.jd-desc {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--color-text-2);
  /* 中文长句末行容易只剩一两个字（如「口，异步轮询获取结果」），让浏览器择优断行 */
  text-wrap: pretty;
}

.jd-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px 12px;
}

.jd-actions__main,
.jd-actions__aux {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

/* 危操作与常规操作之间留一道细竖线，避免「删除」混在按钮串里被误点 */
.jd-actions__split {
  width: 1px;
  height: 16px;
  background: var(--color-border);
}

.jd-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 10px 0 0;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border);
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-3);
}

.jd-hint i {
  flex-shrink: 0;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.jd-hint.is-live {
  color: #059669;
}

.jd-hint.is-pending {
  color: #b45309;
}

/* ── 分区标题 ─────────────────────────────────────────── */
.jd-section {
  margin-top: 20px;
}

.jd-title {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 6px 8px;
  margin: 0 0 9px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jd-title::before {
  content: '';
  align-self: center;
  width: 3px;
  height: 12px;
  border-radius: 2px;
  background: var(--ta-primary);
}

.jd-title__hint {
  margin-left: auto;
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  text-align: right;
  color: var(--color-text-3);
}

/* ── 规格表（基本信息）────────────────────────────────── */
.jd-spec {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--color-border);
  border-radius: var(--ta-radius);
  overflow: hidden;
}

.jd-cell {
  min-width: 0;
  padding: 9px 14px 10px;
  border-right: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
}

.jd-cell:nth-child(3n) {
  border-right: 0;
}

.jd-cell:nth-last-child(-n + 3) {
  border-bottom: 0;
}

.jd-cell__label {
  display: block;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-3);
}

.jd-cell__value {
  display: block;
  margin-top: 1px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-cell__value.is-mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12.5px;
}

/* ── 模型 / 白名单 chips ─────────────────────────────── */
.jd-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.jd-chip {
  padding: 2px 9px;
  border-radius: 6px;
  background: var(--color-fill-2);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-text-2);
}

.jd-empty {
  margin: 0;
  padding: 12px 14px;
  border: 1px dashed var(--color-border);
  border-radius: var(--ta-radius);
  font-size: 12.5px;
  color: var(--color-text-3);
}

/* ── 路由清单 ─────────────────────────────────────────── */
.jd-routes {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.jd-route {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px 7px 10px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  transition: border-color var(--ta-duration-fast) var(--ta-ease);
}

.jd-route:hover {
  border-color: var(--color-border);
}

.jd-route__type {
  flex-shrink: 0;
  min-width: 54px;
  padding: 1px 7px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  text-align: center;
  color: #0f766e;
  background: rgba(13, 148, 136, 0.1);
}

/* 路由类型差异化配色：submit=主色 / query=天蓝 / dynamic=紫（契约枚举只有这三种） */
.jd-route__type.is-query {
  color: #0369a1;
  background: rgba(6, 182, 212, 0.12);
}

.jd-route__type.is-dynamic {
  color: #7c3aed;
  background: rgba(124, 58, 237, 0.1);
}

.jd-route__method {
  flex-shrink: 0;
  width: 38px;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-3);
}

.jd-route__path {
  flex: 1;
  min-width: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 复制按钮常态压低，悬停整行才点亮：一行里最不缺的就是图标噪声 */
.jd-route__copy {
  flex-shrink: 0;
  opacity: 0.45;
  transition: opacity var(--ta-duration-fast) var(--ta-ease);
}

.jd-route:hover .jd-route__copy {
  opacity: 1;
}

.jd-note {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-3);
}

/* ── 版本列表 ─────────────────────────────────────────── */
.jd-vers {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.jd-ver {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 8px 9px 14px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  transition: border-color var(--ta-duration-fast) var(--ta-ease);
}

.jd-ver:hover {
  border-color: var(--color-border);
}

/* 当前激活版本：左侧色条 + 极淡底，扫一眼就知道线上跑的是哪个 */
.jd-ver.is-current {
  background: rgba(13, 148, 136, 0.045);
  border-color: rgba(13, 148, 136, 0.28);
}

.jd-ver.is-current::before {
  content: '';
  position: absolute;
  left: 0;
  top: 8px;
  bottom: 8px;
  width: 2px;
  border-radius: 0 2px 2px 0;
  background: var(--ta-primary);
}

.jd-ver__info {
  flex: 1;
  min-width: 0;
}

.jd-ver__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.jd-ver__no {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-1);
}

.jd-ver__meta {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jd-ver__meta .is-builtin {
  color: #7c3aed;
}

.jd-ver__meta .is-stale {
  margin-left: 4px;
  padding: 0 4px;
  border-radius: 2px;
  background: var(--color-fill-3);
  color: var(--color-text-2);
}

.jd-ver__ops {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}

/* ── 整删确认弹窗（Modal teleport 到 body，scoped attr 仍随内容携带，可命中） */
.jp-purge__lead {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.7;
}

.jp-purge__lead code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
}

.jp-purge__warn {
  margin: 0 0 8px;
  font-size: 12.5px;
  line-height: 1.7;
  color: rgb(var(--danger-6));
}

.jp-purge__note {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-3);
}

/* ── 暗色适配 ─────────────────────────────────────────────
   状态色 / 类型色都是按浅底挑的深色值，直接落到 #0f172a 上对比度只有 3 出头，
   这里按主题换一套亮值（主色令牌在 :root 里不随主题切换，无法复用）。 */
:global(body.arco-theme-dark) .jd-status.is-live,
:global(body.arco-theme-dark) .jd-pill.is-live,
:global(body.arco-theme-dark) .jd-hint.is-live {
  color: #34d399;
}

:global(body.arco-theme-dark) .jd-status.is-pending,
:global(body.arco-theme-dark) .jd-hint.is-pending {
  color: #fbbf24;
}

:global(body.arco-theme-dark) .jd-pill.is-builtin,
:global(body.arco-theme-dark) .jd-ver__meta .is-builtin {
  color: #c4b5fd;
}

:global(body.arco-theme-dark) .jd-route__type {
  color: #5eead4;
  background: rgba(45, 212, 191, 0.14);
}

:global(body.arco-theme-dark) .jd-route__type.is-query {
  color: #67e8f9;
  background: rgba(6, 182, 212, 0.18);
}

:global(body.arco-theme-dark) .jd-route__type.is-dynamic {
  color: #c4b5fd;
  background: rgba(124, 58, 237, 0.24);
}

:global(body.arco-theme-dark) .jd-ver.is-current {
  background: rgba(13, 148, 136, 0.1);
}

@media (max-width: 640px) {
  .jd-spec {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .jd-cell:nth-child(3n) {
    border-right: 1px solid var(--color-border);
  }
  .jd-cell:nth-child(2n) {
    border-right: 0;
  }
  .jd-cell:nth-last-child(-n + 3) {
    border-bottom: 1px solid var(--color-border);
  }
  .jd-cell:nth-last-child(-n + 2) {
    border-bottom: 0;
  }
  /* 窄屏版本行折行：信息在上、操作在下，避免按钮被挤出可视区 */
  .jd-ver {
    flex-wrap: wrap;
  }
  .jd-ver__ops {
    width: 100%;
    justify-content: flex-end;
  }
  .jd-title__hint {
    margin-left: 0;
    width: 100%;
    text-align: left;
  }
}
</style>
