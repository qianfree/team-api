<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import ConversationDrawer from '@/views/security/ConversationDrawer.vue'

// 报告详情抽屉：概览（风险/来源/数据级别/去重统计）+ 发现明细 + 会话还原。
// 访问报告证据属敏感读取（后端已写敏感访问日志）；会话还原复用审计端
// ConversationDrawer，数据走插件自己的端点（不依赖 audit:view 权限）。

const props = defineProps<{
  visible: boolean
  reportId: number | null
  canDelete?: boolean
}>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'deleted'): void
}>()

const loading = ref(false)
const deleting = ref(false)
const detail = ref<any>(null)

watch(
  () => [props.visible, props.reportId] as const,
  ([visible, id]) => {
    if (visible && id) fetchDetail(id)
    if (!visible) detail.value = null
  },
)

async function fetchDetail(id: number) {
  loading.value = true
  detail.value = null
  try {
    const res: any = await request.get(`/admin/plugins/ai-req-analyze/reports/${id}`)
    detail.value = res.data?.data || null
  } catch {
    detail.value = null
    Message.error('加载报告详情失败')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!detail.value) return
  deleting.value = true
  try {
    await request.delete(`/admin/plugins/ai-req-analyze/reports/${detail.value.id}`)
    Message.success('报告已删除')
    emit('update:visible', false)
    emit('deleted')
  } catch {
    // 拦截器已提示
  } finally {
    deleting.value = false
  }
}

// ── 标签映射 ────────────────────────────────────────────────────────────────

const riskLabel: Record<string, string> = {
  high: '高风险', medium: '中风险', low: '低风险', clean: '未发现风险', unknown: '未知',
}
const riskColor: Record<string, string> = {
  high: 'red', medium: 'orange', low: 'arcoblue', clean: 'green', unknown: 'gray',
}
const sourceLabel: Record<string, string> = { manual: '手动分析', patrol: '定时巡查' }
const auditLevelLabel: Record<string, string> = {
  full: '完整记录', full_text: '纯文本', masked: '脱敏记录', question_only: '仅提问', meta_only: '仅留痕', none: '关闭',
}
const severityLabel: Record<string, string> = { high: '高危', medium: '可疑', low: '待复核' }
const severityColor: Record<string, string> = { high: 'red', medium: 'orange', low: 'arcoblue' }
const dimensionLabel: Record<string, string> = { compliance: '合规', leakage: '泄露' }
const dimensionColor: Record<string, string> = { compliance: 'orangered', leakage: 'purple' }
const categoryLabel: Record<string, string> = {
  phishing: '钓鱼', fraud: '诈骗', malware: '恶意代码', illegal_content: '违法违规内容',
  evasion: '绕过审查', credential_leak: '凭证泄露', source_code_leak: '源代码泄露',
  pii_leak: '个人信息泄露', internal_doc_leak: '内部文档泄露', other: '其他',
}

function fmtTime(s?: string | null): string {
  if (!s) return '-'
  return String(s).replace('T', ' ').slice(0, 19)
}

// 去重率：1 - 分析单元数/原始日志数（无日志或无压缩空间时不展示）
const dedupRatio = computed(() => {
  const d = detail.value
  if (!d || !d.log_count) return null
  const units = (d.session_count || 0) + (d.standalone_count || 0)
  if (!units || units >= d.log_count) return null
  return `${Math.round((1 - units / d.log_count) * 100)}%`
})

// ── 会话还原（数据端点指向插件自己的会话接口，finding_id 作为记录定位） ────

const convVisible = ref(false)
const convFindingId = ref<number | null>(null)
const convFetchUrl = computed(() => {
  const reportId = detail.value?.id
  return (findingId: number) =>
    `/admin/plugins/ai-req-analyze/reports/${reportId}/conversation?finding_id=${findingId}`
})

function openConversation(findingId: number) {
  convFindingId.value = findingId
  convVisible.value = true
}
</script>

<template>
  <ADrawer
    :visible="visible"
    title="分析报告详情"
    :width="720"
    unmount-on-close
    @update:visible="emit('update:visible', $event)"
    @cancel="emit('update:visible', false)"
  >
    <ASpin :loading="loading" style="width: 100%">
      <template v-if="detail">
        <!-- 概览 -->
        <div class="detail-header">
          <div class="detail-tags">
            <ATag :color="riskColor[detail.risk_level]" size="large">
              {{ riskLabel[detail.risk_level] || detail.risk_level }}
            </ATag>
            <ATag>{{ sourceLabel[detail.source] || detail.source }}</ATag>
            <ATag v-if="detail.target_username">成员：{{ detail.target_username }}</ATag>
            <ATag v-else>全员</ATag>
            <ATag color="gray">数据级别：{{ auditLevelLabel[detail.audit_level] || detail.audit_level || '-' }}</ATag>
          </div>
          <div class="detail-window">
            {{ detail.tenant_name }} · {{ fmtTime(detail.time_start) }} ~ {{ fmtTime(detail.time_end) }}
          </div>
          <div class="detail-summary">{{ detail.summary }}</div>
          <div class="detail-stats">
            <span>原始日志 {{ detail.log_count }} 条</span>
            <span>会话单元 {{ detail.session_count }}</span>
            <span>独立单元 {{ detail.standalone_count }}</span>
            <span v-if="dedupRatio">去重率 {{ dedupRatio }}</span>
            <span>AI 消耗 {{ detail.ai_tokens_used }} tokens（{{ detail.ai_model }}）</span>
          </div>
          <AAlert v-if="['masked', 'question_only', 'meta_only'].includes(detail.audit_level)"
            type="warning" :closable="false">
            {{ detail.audit_level === 'meta_only'
              ? '本报告基于仅留痕级别的审计数据生成，请求与回复内容均未记录，泄露检测/合规判定无证据可用。'
              : `本报告基于${auditLevelLabel[detail.audit_level]}级别的审计数据生成，部分内容可能已脱敏或缺失（无 AI 回答侧），泄露检测/合规判定的证据完整性受限。` }}
          </AAlert>
        </div>

        <!-- 发现明细 -->
        <div class="findings-title">发现明细（{{ detail.findings?.length || 0 }} 项）</div>
        <AEmpty v-if="!detail.findings?.length" description="未发现风险项" />
        <div v-for="f in detail.findings" :key="f.id" class="finding-card">
          <div class="finding-head">
            <ATag :color="dimensionColor[f.dimension]" size="small">
              {{ dimensionLabel[f.dimension] || f.dimension }}
            </ATag>
            <ATag :color="severityColor[f.severity]" size="small">
              {{ severityLabel[f.severity] || f.severity }}
            </ATag>
            <ATag size="small">{{ categoryLabel[f.category] || f.category }}</ATag>
            <ATag v-if="f.unit_type === 'session'" color="cyan" size="small">会话</ATag>
            <AButton size="mini" type="text" @click="openConversation(f.id)">查看完整会话</AButton>
          </div>
          <pre class="finding-evidence">{{ f.evidence }}</pre>
          <div class="finding-explanation">{{ f.explanation }}</div>
        </div>
      </template>
    </ASpin>

    <template #footer>
      <ASpace>
        <AButton @click="emit('update:visible', false)">关闭</AButton>
        <AButton v-if="canDelete && detail" status="danger" :loading="deleting" @click="handleDelete">
          删除报告
        </AButton>
      </ASpace>
    </template>

    <!-- 会话还原抽屉（复用审计会话组件，数据走插件端点） -->
    <ConversationDrawer
      v-model:visible="convVisible"
      :record-id="convFindingId"
      :fetch-url="convFetchUrl"
    />
  </ADrawer>
</template>

<style scoped>
.detail-header {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--color-border-2);
  margin-bottom: 16px;
}
.detail-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}
.detail-window {
  font-size: 13px;
  color: var(--color-text-2);
}
.detail-summary {
  font-size: 13px;
  line-height: 1.7;
  color: var(--color-text-2);
  background: var(--color-fill-1);
  border-radius: 4px;
  padding: 10px 12px;
}
.detail-stats {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--color-text-3);
}
.findings-title {
  font-weight: 600;
  margin-bottom: 10px;
}
.finding-card {
  border: 1px solid var(--color-border-2);
  border-radius: 6px;
  padding: 12px;
  margin-bottom: 10px;
}
.finding-head {
  display: flex;
  gap: 6px;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.finding-evidence {
  background: var(--color-fill-2);
  border-radius: 4px;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0 0 6px;
  max-height: 200px;
  overflow-y: auto;
}
.finding-explanation {
  font-size: 12px;
  color: var(--color-text-3);
}
</style>
