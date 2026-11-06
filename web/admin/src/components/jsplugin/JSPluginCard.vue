<script setup lang="ts">
// JS 插件卡片：列表默认视图。整卡可点开详情，底部操作区阻止冒泡。
// 版式：左状态色条 → 头部（图标/名称/标识/状态）→ 描述 → 适用模型 → 事实行 → 操作。
// 事实行承载版本数、计费项、轮询方式、白名单、作者，避免内容因高度限制被裁掉。
import { computed } from 'vue'
import { Button, Popconfirm } from '@arco-design/web-vue'
import {
  pluginStatus, resolvePluginIcon, fetchModeLabel,
  type JSPlugin,
} from './pluginMeta'

const props = defineProps<{ plugin: JSPlugin }>()
const emit = defineEmits<{
  (e: 'detail'): void
  (e: 'config'): void
  (e: 'toggle', enabled: boolean): void
}>()

const status = computed(() => pluginStatus(props.plugin))
const icon = computed(() => resolvePluginIcon(props.plugin))

const models = computed(() => props.plugin.models || [])
const versionCount = computed(() => (props.plugin.versions || []).length)
const usageCount = computed(() => (props.plugin.usage_schema || []).length)
const hostCount = computed(() => (props.plugin.allowed_hosts || []).length)

const iconUrl = computed(
  () => `/api/admin/js-plugins/${props.plugin.key}/icon?v=${props.plugin.active_version || ''}`,
)

</script>

<template>
  <article class="jpc" :class="`is-${status.key}`" @click="emit('detail')">
    <div class="jpc__head">
      <div class="jpc__icon" :class="[`jpc__icon--${icon.kind}`, `jpc__icon--${plugin.origin}`]">
        <img v-if="icon.kind === 'img'" :src="iconUrl" :alt="plugin.name || plugin.key" loading="lazy" />
        <component :is="icon.comp" v-else-if="icon.kind === 'icon'" :size="20" />
        <span v-else>{{ icon.label }}</span>
      </div>

      <div class="jpc__ident">
        <h3 class="jpc__name" :title="plugin.name || plugin.key">{{ plugin.name || plugin.key }}</h3>
        <div class="jpc__sub">
          <code class="jpc__key" :title="plugin.key">{{ plugin.key }}</code>
          <span class="jpc__origin" :class="`is-${plugin.origin}`">
            {{ plugin.origin === 'builtin' ? '内置' : '上传' }}
          </span>
        </div>
      </div>

      <span class="jpc__status" :class="`is-${status.key}`" :title="status.hint">
        <i class="jpc__status-dot" />{{ status.label }}
      </span>
    </div>

    <p class="jpc__desc" :title="plugin.description || undefined">
      {{ plugin.description || '该插件未填写描述' }}
    </p>

    <div
      class="jpc__models"
      :title="models.length > 3 ? `共 ${models.length} 个适用模型，可区域内滚动查看` : undefined"
    >
      <span v-for="m in models" :key="m" class="jpc__model" :title="m">{{ m }}</span>
      <span v-if="!models.length" class="jpc__model jpc__model--all" title="不限制适用模型">全部模型</span>
    </div>

    <div class="jpc__facts">
      <span class="jpc__fact is-mono" :class="{ 'is-void': !plugin.active_version }">
        {{ plugin.active_version || '无激活版本' }}
      </span>
      <span class="jpc__fact">{{ versionCount }} 个版本</span>
      <span class="jpc__fact">{{ usageCount }} 项计费</span>
      <span class="jpc__fact">{{ fetchModeLabel(plugin.fetch_mode) }}</span>
      <span v-if="hostCount" class="jpc__fact" :title="plugin.allowed_hosts.join('、')">
        白名单 {{ hostCount }}
      </span>
    </div>

    <footer class="jpc__foot" @click.stop>
      <Popconfirm
        v-if="plugin.enabled"
        :content="`禁用「${plugin.key}」？路由会被立即摘除。`"
        @ok="emit('toggle', false)"
      >
        <Button size="small" type="outline" status="warning">禁用</Button>
      </Popconfirm>
      <Popconfirm
        v-else
        :content="plugin.active_version
          ? `启用「${plugin.key}」？将挂载 /pjs/${plugin.key}/ 路由。`
          : `「${plugin.key}」还没有激活版本，启用后不会生效，仍要启用？`"
        @ok="emit('toggle', true)"
      >
        <Button size="small" type="primary">启用</Button>
      </Popconfirm>

      <div class="jpc__links">
        <Button size="small" type="text" @click="emit('detail')">详情</Button>
        <Button size="small" type="text" @click="emit('config')">配置</Button>
      </div>
    </footer>
  </article>
</template>

<style scoped>
.jpc {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 15px 15px 13px 17px;
  background: var(--color-bg-1);
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  box-shadow: var(--ta-shadow-card);
  cursor: pointer;
  overflow: hidden;
  transition:
    transform var(--ta-duration-normal) var(--ta-ease),
    box-shadow var(--ta-duration-normal) var(--ta-ease),
    border-color var(--ta-duration-normal) var(--ta-ease);
}

/* 左侧状态条：贴边 3px，圆角跟随卡片裁切 */
.jpc::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--color-border-3);
  transition: background var(--ta-duration-normal) var(--ta-ease);
}

.jpc.is-live::before {
  background: linear-gradient(180deg, #10b981, #34d399);
}

.jpc.is-pending::before {
  background: linear-gradient(180deg, #f59e0b, #fbbf24);
}

.jpc:hover {
  transform: translateY(-2px);
  border-color: rgba(13, 148, 136, 0.32);
  box-shadow: var(--ta-shadow-hover);
}

/* ── 头部 ─────────────────────────────── */
.jpc__head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.jpc__icon {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  overflow: hidden;
}

.jpc__icon--icon.jpc__icon--builtin,
.jpc__icon--text.jpc__icon--builtin {
  background: linear-gradient(135deg, #7c3aed, #a78bfa);
}

.jpc__icon--icon.jpc__icon--upload,
.jpc__icon--text.jpc__icon--upload {
  background: linear-gradient(135deg, #0d9488, #2dd4bf);
}

.jpc__icon--img {
  background: var(--color-fill-2);
  border: 1px solid var(--color-border-2);
}

.jpc__icon--img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  padding: 4px;
}

.jpc__ident {
  flex: 1;
  min-width: 0;
  padding-top: 1px;
}

.jpc__name {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  line-height: 20px;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jpc__sub {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 2px;
  min-width: 0;
}

.jpc__key {
  min-width: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 11.5px;
  line-height: 16px;
  color: var(--color-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jpc__origin {
  flex-shrink: 0;
  padding: 0 5px;
  border-radius: 3px;
  font-size: 10.5px;
  line-height: 15px;
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jpc__origin.is-builtin {
  color: #7c3aed;
  background: rgba(124, 58, 237, 0.09);
}

.jpc__status {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: 1px;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 11.5px;
  line-height: 19px;
  font-weight: 500;
  color: var(--color-text-3);
  background: var(--color-fill-2);
  white-space: nowrap;
}

.jpc__status-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.jpc__status.is-live {
  color: #059669;
  background: rgba(16, 185, 129, 0.1);
}

.jpc__status.is-pending {
  color: #b45309;
  background: rgba(245, 158, 11, 0.12);
}

/* ── 正文 ─────────────────────────────── */
.jpc__desc {
  margin: 0;
  /* 固定 3 行高度：短描述不留突兀空白，长描述统一裁剪并在 title 里给全文 */
  min-height: 62px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-2);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

/* 最多两行（18px×2 + 4px gap）；模型多的插件改为区域内滚动，不把卡片撑高 */
.jpc__models {
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  gap: 4px;
  max-height: 40px;
  overflow-y: auto;
  /* 预留滚动条宽度，避免出现滚动条时 chip 重新换行抖动 */
  padding-right: 4px;
  scrollbar-width: thin;
  scrollbar-color: var(--color-fill-3) transparent;
}

.jpc__models::-webkit-scrollbar {
  width: 4px;
}

.jpc__models::-webkit-scrollbar-track {
  background: transparent;
}

.jpc__models::-webkit-scrollbar-thumb {
  border-radius: 2px;
  background: var(--color-fill-3);
}

.jpc__models:hover::-webkit-scrollbar-thumb {
  background: var(--color-fill-4);
}

.jpc__model {
  max-width: 100%;
  padding: 0 7px;
  border-radius: 4px;
  font-size: 11.5px;
  line-height: 18px;
  color: var(--color-primary-6);
  background: var(--color-primary-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.jpc__model--all {
  color: var(--color-text-3);
  background: var(--color-fill-2);
}

.jpc__facts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px 12px;
  margin-top: auto;
  padding-top: 2px;
  font-size: 11.5px;
  line-height: 16px;
  color: var(--color-text-3);
  font-variant-numeric: tabular-nums;
}

/* 用间距分组而非分隔点：换行时不会出现悬空的圆点 */
.jpc__fact {
  white-space: nowrap;
}

.jpc__fact.is-mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: var(--color-text-2);
}

.jpc__fact.is-void {
  font-family: inherit;
  color: var(--color-text-3);
  font-style: italic;
}

/* ── 操作区 ───────────────────────────── */
.jpc__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--color-fill-3);
}

.jpc__links {
  display: flex;
  align-items: center;
  gap: 2px;
}

.jpc__links :deep(.arco-btn) {
  padding: 0 6px;
}

/* 单列窄屏（与页面网格断点一致）：描述只给 2 行，避免短描述下方留出整行空白 */
@media (max-width: 768px) {
  /* 单列时每行只有一张卡，描述只给 2 行，避免短描述下方留出整行空白 */
  .jpc__desc {
    min-height: 42px;
    -webkit-line-clamp: 2;
  }
}
</style>
