<script setup lang="ts">
// JS 插件上传弹窗：拖拽/选择 plugin.js → 读取源码 → 可选品牌图标与备注 → 上传校验。
// 上传只做 esbuild 编译 + manifest 契约校验，入库后默认未启用（需在列表激活 + 启用）。
// 上传前可先「试运行」（dry-run 端点）：不落库的自检报告，存在 fail 项时上传按钮
// 弱化为警示色并要求二次确认（不强制阻止，最终决定权留给操作者）。
import { computed, ref, watch } from 'vue'
import { Button, Message, Modal } from '@arco-design/web-vue'
import {
  IconCheckCircle, IconClose, IconCloseCircle, IconDown, IconDownload,
  IconExclamationCircle, IconExperiment, IconFile, IconUpload,
} from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { downloadTextFile } from '@/utils/download'
import { fetchModeLabel } from './pluginMeta'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'uploaded', manifest: any): void
}>()

const source = ref('')
const remark = ref('')
const fileName = ref('')
const fileSize = ref(0)
const iconURI = ref('')
const iconName = ref('')
const uploading = ref(false)
const dragging = ref(false)
const showMore = ref(false)

const fileInput = ref<HTMLInputElement | null>(null)
const iconInput = ref<HTMLInputElement | null>(null)

const lineCount = computed(() => (source.value ? source.value.split(/\r?\n/).length : 0))
const canSubmit = computed(() => source.value.trim().length > 0)

const prettySize = computed(() => {
  if (!fileSize.value) return ''
  return fileSize.value < 1024
    ? `${fileSize.value} B`
    : `${(fileSize.value / 1024).toFixed(1)} KiB`
})

watch(
  () => props.visible,
  (v) => {
    if (v) reset()
  },
)

// ── 试运行（dry-run）──────────────────────────────────────
interface DryRunCheck {
  name: string
  status: 'pass' | 'warn' | 'fail'
  detail: string
}

interface DryRunResult {
  key?: string
  name?: string
  version?: string
  models?: string[]
  fetch_mode?: string
  routes?: { method: string; path: string; type: string }[]
  checks: DryRunCheck[]
  pass: boolean
}

const dryRun = ref<DryRunResult | null>(null)
const dryRunning = ref(false)
const templateLoading = ref(false)

const failCount = computed(() => dryRun.value?.checks.filter((c) => c.status === 'fail').length || 0)
const warnCount = computed(() => dryRun.value?.checks.filter((c) => c.status === 'warn').length || 0)
const passCount = computed(() => dryRun.value?.checks.filter((c) => c.status === 'pass').length || 0)
const hasFail = computed(() => failCount.value > 0)
// 报告整体色调：全过=绿 / 仅有提示=橙 / 有缺陷=红
const dryRunVariant = computed(() => {
  if (!dryRun.value) return ''
  if (hasFail.value) return 'is-fail'
  if (warnCount.value) return 'is-warn'
  return 'is-ok'
})

// 源码一旦改动，上一份试运行报告即失效，立即清空避免拿着旧结论上传
watch(source, () => {
  dryRun.value = null
})

async function runDryRun() {
  if (!canSubmit.value) {
    Message.warning('请先选择 plugin.js 文件或粘贴源码')
    return
  }
  dryRunning.value = true
  try {
    const res: any = await request.post('/admin/js-plugins/dry-run', { source: source.value })
    dryRun.value = res.data?.data || null
    if (dryRun.value?.pass) Message.success('试运行通过，可以上传')
    else if (dryRun.value) Message.warning(`试运行发现 ${failCount.value} 项缺陷，建议先修复`)
  } catch {
    // 拦截器已提示（编译失败等场景后端也可能以 code!=0 返回）
  } finally {
    dryRunning.value = false
  }
}

// 下载作者模板：GET template → 前端存为 file_name（后端建议名，兜底 plugin.template.js）
async function downloadTemplate() {
  templateLoading.value = true
  try {
    const res: any = await request.get('/admin/js-plugins/template')
    const d = res.data?.data || {}
    if (!d.source) {
      Message.warning('模板内容为空')
      return
    }
    downloadTextFile(
      String(d.source),
      d.file_name || 'plugin.template.js',
      'text/javascript;charset=utf-8',
    )
  } catch {
    // 拦截器已提示
  } finally {
    templateLoading.value = false
  }
}

function reset() {
  source.value = ''
  remark.value = ''
  fileName.value = ''
  fileSize.value = 0
  iconURI.value = ''
  iconName.value = ''
  dragging.value = false
  showMore.value = false
  dryRun.value = null
}

function close() {
  emit('update:visible', false)
}

const MAX_SOURCE = 2 * 1024 * 1024

function readSource(file: File) {
  if (!/\.(js|mjs|cjs|ts)$/i.test(file.name)) {
    Message.warning('仅支持 .js / .mjs / .cjs / .ts 源码文件')
    return
  }
  if (file.size > MAX_SOURCE) {
    Message.warning('源码文件需 ≤ 2 MiB')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    source.value = String(reader.result || '')
    fileName.value = file.name
    fileSize.value = file.size
  }
  reader.onerror = () => Message.error('读取文件失败')
  reader.readAsText(file)
}

function pickFile() {
  fileInput.value?.click()
}

function onFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) readSource(file)
  input.value = ''
}

function onDrop(ev: DragEvent) {
  dragging.value = false
  const file = ev.dataTransfer?.files?.[0]
  if (file) readSource(file)
}

function clearFile() {
  source.value = ''
  fileName.value = ''
  fileSize.value = 0
}

function pickIcon() {
  iconInput.value?.click()
}

function onIconChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!/\.(svg|png)$/i.test(file.name)) {
    Message.warning('图标仅支持 SVG / PNG')
    return
  }
  if (file.size > 512 * 1024) {
    Message.warning('图标需 ≤ 512 KiB')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    iconURI.value = String(reader.result || '')
    iconName.value = file.name
  }
  reader.onerror = () => {
    // 读取失败必须提示：静默会让用户以为图标已选上，提交后却没有图标
    Message.warning('图标读取失败，请重试')
    iconURI.value = ''
    iconName.value = ''
  }
  reader.readAsDataURL(file)
}

function clearIcon() {
  iconURI.value = ''
  iconName.value = ''
}

async function submit(): Promise<boolean> {
  if (!canSubmit.value) {
    Message.warning('请选择 plugin.js 文件或粘贴源码')
    return false
  }
  uploading.value = true
  try {
    const res: any = await request.post('/admin/js-plugins/upload', {
      source: source.value,
      icon: iconURI.value || undefined,
      remark: remark.value,
    })
    const manifest = res.data?.data?.manifest || {}
    Message.success(`上传成功：${manifest.key || '未知'} @ ${manifest.version || '未知版本'}`)
    emit('uploaded', manifest)
    close()
    return true
  } catch {
    // 拦截器已提示 esbuild / 契约校验错误原文
    return false
  } finally {
    uploading.value = false
  }
}

// 试运行存在 fail 时不强制阻止上传：按钮弱化为警示色，点击给二次确认
function onUploadClick() {
  if (hasFail.value) {
    Modal.confirm({
      title: '试运行发现缺陷，仍要上传？',
      content: `当前源码试运行有 ${failCount.value} 项缺陷（fail），上传大概率会被契约校验拒绝或生成不可用插件。建议先按报告修复。`,
      okText: '仍要上传',
      cancelText: '返回修改',
      onOk: () => submit(),
    })
    return
  }
  submit()
}
</script>

<template>
  <Modal
    :visible="visible"
    title="上传 JS 任务插件"
    :width="800"
    :mask-closable="false"
    @cancel="close"
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <div class="up-banner">
      上传只做 <strong>esbuild 编译 + manifest 契约校验</strong>，入库后默认未启用；
      需要先在列表里激活版本并启用，才会挂载到 <code>/pjs/{key}/</code>。
    </div>

    <!-- 源码：拖拽区（未选文件时）/ 文件条（已选文件时） -->
    <div v-if="!fileName" class="up-drop" :class="{ 'is-dragging': dragging }"
      @click="pickFile"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <input ref="fileInput" type="file" accept=".js,.mjs,.cjs,.ts" hidden @change="onFileChange" />
      <div class="up-drop__icon"><IconUpload :size="22" /></div>
      <div class="up-drop__title">拖入 <code>plugin.js</code>，或<span class="up-drop__link">点击选择文件</span></div>
      <div class="up-drop__hint">支持 .js / .mjs / .cjs / .ts，单文件 ≤ 2 MiB</div>
    </div>

    <div v-else class="up-file">
      <div class="up-file__icon"><IconFile :size="18" /></div>
      <div class="up-file__meta">
        <span class="up-file__name">{{ fileName }}</span>
        <span class="up-file__size">{{ prettySize }} · {{ lineCount }} 行</span>
      </div>
      <Button size="mini" type="text" @click="pickFile">换一个</Button>
      <Button size="mini" type="text" status="danger" @click="clearFile">
        <template #icon><IconClose /></template>
      </Button>
    </div>

    <div class="up-editor">
      <div class="up-editor__head">
        <span>源码</span>
        <span class="up-editor__tools">
          <button
            type="button"
            class="up-editor__tpl"
            :disabled="templateLoading"
            title="下载内置的插件作者模板（golden 参考实现）"
            @click="downloadTemplate"
          >
            <IconDownload />{{ templateLoading ? '获取中…' : '下载模板' }}
          </button>
          <span class="up-editor__meta">
            {{ lineCount }} 行 · {{ source.length.toLocaleString() }} 字符
            <em v-if="fileName">（已从文件载入，可直接修改）</em>
          </span>
        </span>
      </div>
      <textarea
        v-model="source"
        class="up-editor__area"
        spellcheck="false"
        placeholder="// 在此粘贴 plugin.js 源码，或从上方拖入文件…"
      />
    </div>

    <!-- 试运行报告：manifest 摘要 + 检查项平铺；源码改动即清空 -->
    <div v-if="dryRun" class="up-dryrun" :class="dryRunVariant">
      <div class="up-dryrun__head">
        <span class="up-dryrun__title">试运行报告</span>
        <span class="up-dryrun__verdict">
          {{ passCount }} 项通过<template v-if="warnCount"> · {{ warnCount }} 项提示</template><template v-if="failCount"> · {{ failCount }} 项缺陷</template>
        </span>
      </div>
      <!-- 编译失败时后端只回 checks，无 manifest 摘要 -->
      <div v-if="dryRun.key" class="up-dryrun__manifest">
        <code class="up-dryrun__key">{{ dryRun.key }}</code>
        <span class="up-dryrun__name">{{ dryRun.name }}</span>
        <span class="up-dryrun__ver">v{{ dryRun.version }}</span>
        <span class="up-dryrun__sep">·</span>
        <span>模型 {{ (dryRun.models || []).length }} 个</span>
        <span class="up-dryrun__sep">·</span>
        <span>路由 {{ (dryRun.routes || []).length }} 条</span>
        <span class="up-dryrun__sep">·</span>
        <span>{{ fetchModeLabel(dryRun.fetch_mode) }}</span>
      </div>
      <ul class="up-dryrun__checks">
        <li v-for="c in dryRun.checks" :key="c.name" class="up-dryrun__check" :class="`is-${c.status}`">
          <IconCheckCircle v-if="c.status === 'pass'" class="up-dryrun__ico" />
          <IconExclamationCircle v-else-if="c.status === 'warn'" class="up-dryrun__ico" />
          <IconCloseCircle v-else class="up-dryrun__ico" />
          <span class="up-dryrun__cname">{{ c.name }}</span>
          <span class="up-dryrun__cdetail" :title="c.detail">{{ c.detail }}</span>
        </li>
      </ul>
    </div>

    <button type="button" class="up-more" @click="showMore = !showMore">
      <span>{{ showMore ? '收起' : '展开' }}图标与备注（可选）</span>
      <IconDown class="up-more__chev" :class="{ 'is-open': showMore }" />
    </button>

    <div v-if="showMore" class="up-more__body">
      <div class="up-field">
        <label class="up-field__label">品牌图标</label>
        <div class="up-field__row">
          <input ref="iconInput" type="file" accept=".svg,.png" hidden @change="onIconChange" />
          <div class="up-icon-preview" :class="{ 'is-empty': !iconURI }">
            <img v-if="iconURI" :src="iconURI" alt="图标预览" />
            <span v-else>无</span>
          </div>
          <div class="up-field__col">
            <div class="up-field__buttons">
              <Button size="small" type="secondary" @click="pickIcon">
                {{ iconName ? '重新选择' : '选择 SVG / PNG' }}
              </Button>
              <Button v-if="iconURI" size="small" type="text" status="danger" @click="clearIcon">移除</Button>
            </div>
            <span class="up-field__hint">≤ 512 KiB；不传则用 manifest.icon（图标名或首字）</span>
          </div>
        </div>
      </div>

      <div class="up-field">
        <label class="up-field__label">版本备注</label>
        <input v-model="remark" class="up-input" maxlength="200" placeholder="本次改动说明，便于在版本历史里辨认" />
      </div>
    </div>

    <!-- 自定义底部：试运行（左）+ 取消/上传（右）；有 fail 项时上传弱化为警示色 -->
    <template #footer>
      <div class="up-footer">
        <Button :loading="dryRunning" :disabled="!canSubmit || uploading" @click="runDryRun">
          <template #icon><IconExperiment /></template>试运行
        </Button>
        <span v-if="hasFail" class="up-footer__warn">试运行有 {{ failCount }} 项缺陷</span>
        <span class="up-footer__spacer" />
        <Button :disabled="uploading || dryRunning" @click="close">取消</Button>
        <Button
          :type="hasFail ? 'outline' : 'primary'"
          :status="hasFail ? 'warning' : undefined"
          :loading="uploading"
          :disabled="!canSubmit || dryRunning"
          @click="onUploadClick"
        >上传并校验</Button>
      </div>
    </template>
  </Modal>
</template>

<style scoped>
.up-banner {
  margin-bottom: 14px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--color-primary-2);
  background: var(--color-primary-1);
  font-size: 12.5px;
  line-height: 1.65;
  color: var(--color-text-2);
}

.up-banner code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--color-primary-6);
}

/* 拖拽区 */
.up-drop {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 22px 16px;
  border: 1.5px dashed var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
  cursor: pointer;
  transition: border-color var(--ta-duration-fast) var(--ta-ease),
    background var(--ta-duration-fast) var(--ta-ease);
}

.up-drop:hover,
.up-drop.is-dragging {
  border-color: var(--color-primary-6);
  background: var(--color-primary-1);
}

.up-drop__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  color: var(--color-primary-6);
  background: var(--color-primary-1);
}

.up-drop__title {
  font-size: 13.5px;
  color: var(--color-text-1);
}

.up-drop__title code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: var(--color-primary-6);
}

.up-drop__link {
  margin-left: 4px;
  color: var(--color-primary-6);
  font-weight: 500;
}

.up-drop__hint {
  font-size: 12px;
  color: var(--color-text-3);
}

/* 已选文件条 */
.up-file {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
}

.up-file__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  color: var(--color-primary-6);
  background: var(--color-bg-1);
}

.up-file__meta {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.up-file__name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.up-file__size {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--color-text-3);
}

/* 源码编辑器 */
.up-editor {
  margin-top: 10px;
  border: 1px solid var(--color-border-2);
  border-radius: var(--ta-radius);
  overflow: hidden;
  background: var(--color-bg-1);
}

.up-editor__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 12px;
  border-bottom: 1px solid var(--color-border-2);
  background: var(--color-fill-1);
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-2);
}

.up-editor__meta {
  font-weight: 400;
  color: var(--color-text-3);
}

.up-editor__meta em {
  font-style: normal;
  color: var(--color-primary-6);
}

.up-editor__tools {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

/* 下载模板：编辑器头部的小动作按钮，与 meta 同行 */
.up-editor__tpl {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  padding: 1px 8px;
  border: none;
  border-radius: 5px;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  color: var(--color-text-2);
  transition: color var(--ta-duration-fast) var(--ta-ease),
    background var(--ta-duration-fast) var(--ta-ease);
}

.up-editor__tpl:hover:not(:disabled) {
  color: var(--color-primary-6);
  background: var(--color-primary-1);
}

.up-editor__tpl:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* ── 试运行报告 ─────────────────────────────────── */
.up-dryrun {
  margin-top: 12px;
  border: 1px solid var(--color-border-2);
  border-left: 3px solid rgb(var(--green-6));
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
  overflow: hidden;
}

.up-dryrun.is-warn {
  border-left-color: rgb(var(--orange-6));
}

.up-dryrun.is-fail {
  border-left-color: rgb(var(--red-6));
}

.up-dryrun__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border-2);
}

.up-dryrun__title {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--color-text-1);
}

.up-dryrun__verdict {
  font-size: 12px;
  color: var(--color-text-3);
}

.up-dryrun__manifest {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 12px;
  border-bottom: 1px dashed var(--color-border-2);
  font-size: 12px;
  color: var(--color-text-2);
}

.up-dryrun__key {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: var(--color-primary-6);
}

.up-dryrun__name {
  font-weight: 500;
  color: var(--color-text-1);
}

.up-dryrun__ver {
  color: var(--color-text-3);
}

.up-dryrun__sep {
  color: var(--color-text-4);
}

.up-dryrun__checks {
  list-style: none;
  margin: 0;
  padding: 6px 12px;
}

.up-dryrun__check {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 4px 0;
  font-size: 12.5px;
  line-height: 1.6;
}

.up-dryrun__check .up-dryrun__ico {
  flex-shrink: 0;
  align-self: center;
}

.up-dryrun__check.is-pass .up-dryrun__ico {
  color: rgb(var(--green-6));
}

.up-dryrun__check.is-warn .up-dryrun__ico {
  color: rgb(var(--orange-6));
}

.up-dryrun__check.is-fail .up-dryrun__ico {
  color: rgb(var(--red-6));
}

.up-dryrun__cname {
  flex-shrink: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-2);
}

.up-dryrun__cdetail {
  min-width: 0;
  color: var(--color-text-3);
  word-break: break-all;
}

/* ── 底部操作条 ─────────────────────────────────── */
.up-footer {
  display: flex;
  align-items: center;
  gap: 10px;
}

.up-footer__warn {
  font-size: 12px;
  color: rgb(var(--orange-6));
}

.up-footer__spacer {
  flex: 1;
}

.up-editor__area {
  display: block;
  width: 100%;
  min-height: 180px;
  max-height: 400px;
  padding: 12px;
  border: none;
  outline: none;
  resize: vertical;
  background: transparent;
  color: var(--color-text-1);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.65;
  tab-size: 2;
}

.up-editor__area::placeholder {
  color: var(--color-text-3);
}

/* 更多设置 */
.up-more {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 12.5px;
  color: var(--color-primary-6);
}

.up-more__chev {
  display: inline-block;
  transition: transform var(--ta-duration-fast) var(--ta-ease);
}

.up-more__chev.is-open {
  transform: rotate(180deg);
}

.up-more__body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 12px;
  padding: 14px;
  border-radius: var(--ta-radius);
  background: var(--color-fill-1);
}

.up-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.up-field__label {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--color-text-2);
}

.up-field__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.up-field__col {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.up-field__buttons {
  display: flex;
  align-items: center;
  gap: 4px;
}

.up-field__hint {
  font-size: 11.5px;
  color: var(--color-text-3);
}

.up-icon-preview {
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  border: 1px solid var(--color-border-2);
  background: var(--color-bg-1);
  overflow: hidden;
}

.up-icon-preview.is-empty {
  border-style: dashed;
  color: var(--color-text-3);
  font-size: 12px;
}

.up-icon-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  padding: 4px;
}

.up-input {
  width: 100%;
  padding: 6px 11px;
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  background: var(--color-bg-1);
  color: var(--color-text-1);
  font-size: 13px;
  outline: none;
  transition: border-color var(--ta-duration-fast) var(--ta-ease),
    box-shadow var(--ta-duration-fast) var(--ta-ease);
}

.up-input:focus {
  border-color: var(--color-primary-6);
  box-shadow: 0 0 0 3px rgba(13, 148, 136, 0.1);
}
</style>
