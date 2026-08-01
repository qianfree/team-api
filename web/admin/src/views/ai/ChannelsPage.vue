<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { Tag, Button, Space, Popconfirm, Message, Radio } from '@arco-design/web-vue'
import { IconInfoCircle } from '@arco-design/web-vue/es/icon'
import type { TableColumnData } from '@arco-design/web-vue'
import type { FormInstance } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import { hasPermission } from '@/utils/permission'
import { providerTypeOptions, providerTypeName, filterProviderOption } from '@/constants/channel'

const router = useRouter()
const message = Message

const providerTagColor: Record<number, string> = {
  1: 'green', 2: 'arcoblue', 3: 'orangered', 8: 'green', 15: 'arcoblue',
}

// === Channel List ===
const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })
const filterType = ref<number | undefined>(undefined)
const filterStatus = ref<string | undefined>(undefined)
const filterSearch = ref('')

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }, { label: '测试中', value: 'testing' },
]
const statusTagColor: Record<string, string> = { active: 'green', disabled: 'orangered', testing: 'arcoblue' }
const statusLabel: Record<string, string> = { active: '启用', disabled: '禁用', testing: '测试中' }
const tierLabel: Record<string, string> = { primary: '首选', secondary: '备用', reserve: '保底' }
const tierTagColor: Record<string, string> = { primary: 'arcoblue', secondary: 'orange', reserve: 'gray' }
const tierOptions = [
  { label: '首选（承接主要流量）', value: 'primary' },
  { label: '备用（零星保温流量，主力饱和时溢出承接）', value: 'secondary' },
  { label: '保底（仅前两层不可用时使用）', value: 'reserve' },
]

function healthColor(score: number | null | undefined) {
  if (score === null || score === undefined) return '#94a3b8'
  if (score >= 80) return '#10b981'
  if (score >= 50) return '#f59e0b'
  return '#ef4444'
}

function openDetail(row: any) {
  router.push({ name: 'AdminChannelDetail', params: { id: row.id } })
}

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '名称', dataIndex: 'name', width: 140 },
  {
    title: '类型', dataIndex: 'type', width: 140,
    render({ record }) {
      const label = providerTypeName[record.type] || `Unknown(${record.type})`
      return h(Tag, { color: providerTagColor[record.type], size: 'small' }, () => label)
    },
  },
  { title: 'Base URL', dataIndex: 'base_url', width: 220, ellipsis: true },
  {
    title: '状态', dataIndex: 'status', width: 80,
    render({ record }) { return h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusLabel[record.status] || record.status) },
  },
  { title: '优先级', dataIndex: 'priority', width: 70 },
  { title: '权重', dataIndex: 'weight', width: 70 },
  {
    title: '层级', dataIndex: 'tier', width: 80,
    render({ record }) {
      return h(Tag, { color: tierTagColor[record.tier] || 'gray', size: 'small' }, () => tierLabel[record.tier] || record.tier || '-')
    },
  },
  {
    title: '健康度', dataIndex: 'health_score', width: 90,
    render({ record }) {
      if (record.health_score === null || record.health_score === undefined) return h('span', { style: 'color:#94a3b8' }, 'N/A')
      return h('span', { style: { color: healthColor(record.health_score), fontWeight: 600 } }, record.health_score.toFixed(0))
    },
  },
  {
    title: 'VIP', dataIndex: 'is_vip', width: 70,
    render({ record }) {
      return h(Tag, { color: record.is_vip ? 'gold' : undefined, size: 'small' }, () => record.is_vip ? 'VIP' : '-')
    },
  },
  { title: '创建时间', dataIndex: 'created_at', width: 170 },
  {
    title: '操作', dataIndex: 'actions', width: 210, fixed: 'right',
    render({ record }) {
      const isActive = record.status === 'active'
      const action = isActive ? '禁用' : '启用'
      // 破坏性写操作按钮按权限点渲染（super_admin 在 hasPermission 内直接放行）。
      const nodes = [
        hasPermission('channel:edit')
          ? h(Popconfirm, {
              content: `确定${action}该渠道？`,
              onOk: () => toggleStatus(record),
            }, () => h(Button, {
              size: 'small',
              status: isActive ? 'normal' : 'success',
            }, () => action))
          : null,
        h(Button, { size: 'small', type: 'primary', onClick: () => openDetail(record) }, () => '详情'),
        hasPermission('channel:delete')
          ? h(Popconfirm, { content: '确定删除该渠道？', onOk: () => deleteChannel(record) }, () => h(Button, { size: 'small', status: 'danger' }, () => '删除'))
          : null,
      ].filter(Boolean)
      return h(Space, { size: 4 }, () => nodes)
    },
  },
]

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = { page: pagination.current, page_size: pagination.pageSize }
    if (filterType.value) params.type = filterType.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterSearch.value) params.search = filterSearch.value
    const res: any = await request.get('/admin/channels', { params })
    data.value = res.data?.data?.list || res.data?.list || []
    pagination.total = res.data?.data?.total || res.data?.total || 0
  } catch {
    data.value = []; pagination.total = 0
  } finally { loading.value = false }
}

function handleFilter() { pagination.current = 1; fetchData() }

// === Create/Edit Modal ===
const showModal = ref(false)
const formLoading = ref(false)
const formRef = ref<FormInstance>()

const formRules = {
  name: [{ required: true, message: '请输入渠道名称' }],
  type: [{ required: true, message: '请选择供应商类型' }],
  api_key: [{ required: true, message: '请输入 API Key' }],
}

const providerDefaultURLs: Record<number, string> = {
  1: 'https://api.openai.com',
  2: 'https://api.anthropic.com',
  3: 'https://generativelanguage.googleapis.com',
  4: 'https://dashscope.aliyuncs.com/compatible-mode',
  6: 'https://hunyuan.tencentcloudapi.com',
  7: 'https://open.bigmodel.cn/api/paas',
  8: 'https://api.deepseek.com',
  9: 'https://api.moonshot.cn',
  10: 'https://ark.cn-beijing.volces.com/api',
  11: 'https://bedrock-runtime.us-east-1.amazonaws.com',
  12: 'https://{resource}.openai.azure.com',
  13: 'https://us-central1-aiplatform.googleapis.com',
  15: 'https://api.mistral.ai',
  16: 'https://api.x.ai',
  17: 'https://ai.360.cn',
  18: 'https://api.lingyiwanwu.com',
  19: 'https://qianfan.baidubce.com',
  20: 'https://api.cloudflare.com/client/v4/accounts/{account_id}',
  22: 'http://localhost:11434',
  25: 'https://api.siliconflow.cn',
  26: 'https://spark-api.xf-yun.com',
  27: 'https://openrouter.ai/api',
  28: 'http://localhost:9997',
  29: 'https://api.minimax.chat',
  30: '',
  32: 'https://api.coze.cn',
  33: 'https://api.dify.ai/v1',
  34: 'https://jimeng.jianying.com',
  35: 'https://api.openai.com',
}

const form = reactive({ name: '', type: 1, base_url: '', api_key: '', priority: 0, weight: 100, tier: 'primary', strict_capacity: false, test_model: '', remark: '', status: 'active', is_vip: false, use_proxy: false, sharing_threshold: null as number | null, preemption_threshold: null as number | null, borrowing_cooldown_seconds: null as number | null })

function openCreate() {
  Object.assign(form, { name: '', type: 1, base_url: '', api_key: '', priority: 0, weight: 100, tier: 'primary', strict_capacity: false, test_model: '', remark: '', status: 'active', is_vip: false, use_proxy: false, sharing_threshold: null, preemption_threshold: null, borrowing_cooldown_seconds: null })
  showModal.value = true
}

async function handleSubmit(done: () => void) {
  const errors = await formRef.value?.validate()
  if (errors) { return false }
  formLoading.value = true
  try {
    await request.post('/admin/channels', form)
    message.success('创建成功')
    done(); fetchData()
  } catch { return false } finally { formLoading.value = false }
}

async function deleteChannel(row: any) {
  try { await request.delete(`/admin/channels/${row.id}`); message.success('删除成功'); fetchData() }
  catch { /* interceptor handles error toast */ }
}

// 快捷启用/禁用渠道：复用 PUT /admin/channels/{id}。
// 后端 UpdateChannel 会无条件写入 priority/weight，故必须带上当前值，否则会被重置为 0。
// status 为 active → 禁用；其它（disabled/testing）→ 启用为 active。
async function toggleStatus(record: any) {
  if (record._toggling) return
  const isActive = record.status === 'active'
  const nextStatus = isActive ? 'disabled' : 'active'
  const prevStatus = record.status
  record._toggling = true
  record.status = nextStatus // 乐观更新，状态 Tag 即刻变化
  try {
    await request.put(`/admin/channels/${record.id}`, {
      status: nextStatus,
      priority: record.priority,
      weight: record.weight,
    })
    message.success(nextStatus === 'active' ? '已启用' : '已禁用')
  } catch {
    record.status = prevStatus // 失败回滚，错误提示由 request 拦截器统一处理
  } finally {
    record._toggling = false
  }
}

onMounted(fetchData)

// === OAuth ===
const showOAuthDialog = ref(false)
const oauthStep2 = ref(false)
const oauthLoading = ref(false)
const oauthForm = reactive({
  platform: 'claude' as string,
  auth_mode: 'browser' as string,
  scope: 'full' as string,
  oauth_type: 'code_assist' as string,
  session_key: '',
  code: '',
  state: '',
  session_id: '',
})

async function handleOAuthStart() {
  oauthLoading.value = true
  try {
    const res: any = await request.post('/admin/channels/oauth/auth-url', {
      platform: oauthForm.platform,
      auth_mode: oauthForm.auth_mode,
      scope: oauthForm.scope,
      oauth_type: oauthForm.oauth_type,
      session_key: oauthForm.session_key || undefined,
    })
    const data = res.data?.data || res.data
    if (data?.auth_url) {
      // 浏览器模式：打开授权链接，进入第二步
      oauthForm.session_id = data.session_id
      window.open(data.auth_url, '_blank')
      oauthForm.code = ''
      oauthForm.state = ''
      oauthStep2.value = true
    } else if (data?.key_id) {
      // Cookie 模式：已完成
      message.success('Cookie 授权成功，渠道已创建')
      showOAuthDialog.value = false
      fetchData()
    }
  } catch { /* interceptor handles error */ }
  finally { oauthLoading.value = false }
}

async function handleOAuthExchange() {
  if (!oauthForm.code) { message.warning('请输入授权码'); return }
  oauthLoading.value = true
  try {
    await request.post('/admin/channels/oauth/exchange', {
      session_id: oauthForm.session_id,
      code: oauthForm.code,
      state: oauthForm.state || undefined,
    })
    message.success('OAuth 授权成功，渠道已创建')
    showOAuthDialog.value = false
    oauthStep2.value = false
    fetchData()
  } catch { /* interceptor handles error */ }
  finally { oauthLoading.value = false }
}

const showSchedulingGuide = ref(false)

const { exporting, exportFile } = useExport({
  url: '/admin/channels/export',
  getFilters: () => ({
    type: filterType.value,
    status: filterStatus.value,
    search: filterSearch.value,
  }),
})
</script>

<template>
  <div class="page-table">
    <PageHeader title="渠道管理" description="管理 AI 模型供应商渠道（每渠道一个 Key）">
      <template #actions>
        <AButton type="primary" @click="openCreate">创建渠道</AButton>
        <AButton type="outline" status="success" @click="showOAuthDialog = true">官方渠道授权</AButton>
        <ADropdown trigger="hover">
          <AButton :loading="exporting">导出</AButton>
          <template #content>
            <ADoption @click="exportFile('csv')">导出 CSV</ADoption>
            <ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
          </template>
        </ADropdown>
      </template>
    </PageHeader>

    <!-- 渠道调度说明（点击弹窗） -->
    <div class="scheduling-hint" @click="showSchedulingGuide = true">
      <IconInfoCircle /> 了解优先级、权重、亲和与容量如何共同调度 →
    </div>

    <!-- 调度说明弹窗 -->
    <AModal v-model:visible="showSchedulingGuide" title="新版渠道调度说明" :width="680" :footer="false">
      <div class="scheduling-guide">
        <p class="guide-intro">新版渠道调度开启时，请求按以下规则选择渠道：</p>

        <section class="guide-section">
          <div class="guide-title"><span class="guide-index">1</span>候选渠道</div>
          <p>先筛选已启用、支持当前模型、符合租户范围且存在可用 Key 的渠道。健康度低于 20 的渠道通常会被排除；如果全部低于 20，则保留作为降级候选。</p>
        </section>

        <section class="guide-section">
          <div class="guide-title"><span class="guide-index">2</span>优先级与权重</div>
          <p>数字越大优先级越高，仅最高可用优先级组参与分配。同组通过加权稳定选路建立新亲和，<code>权重=0</code> 的渠道不参与调度；健康度 50-79 时有效权重减半，20-49 时降至 1/4。</p>
        </section>

        <section class="guide-section">
          <div class="guide-title"><span class="guide-index">3</span>渠道亲和</div>
          <p>首次请求或亲和失效后，调度器按权重稳定选择渠道。请求成功后，同一租户、用户、API Key 和模型会尽量复用该渠道，默认有效期为 30 分钟，后续成功会刷新有效期，以提高上游缓存命中率。</p>
        </section>

        <section class="guide-section">
          <div class="guide-title"><span class="guide-index">4</span>故障与容量</div>
          <p>亲和渠道发生硬失败时会清除绑定并重新选择。渠道达到并发上限时，仅当前请求临时溢出到其他可用渠道，不覆盖原亲和；后续请求仍会优先尝试原渠道。</p>
        </section>

        <div class="guide-example">
          <div class="guide-example-title">配置示例</div>
          <p><code>优先级=100</code> 的主力渠道可用时，不会选择 <code>优先级=50</code> 的备用渠道；主力组不可用后才会降级。</p>
          <p>同一优先级内设置 <code>权重=70</code> 和 <code>权重=30</code>，表示大量新亲和主体约按 7:3 建立绑定。由于后续请求保持亲和，用户请求量不同会使总请求比例偏离 7:3。</p>
        </div>
      </div>
    </AModal>

    <!-- Filters -->
    <ACard :bordered="false" class="mb-4">
      <ASpace>
        <ASelect v-model="filterType" :options="[{ label: '全部类型', value: '' }, ...providerTypeOptions]" placeholder="供应商类型" allow-clear allow-search style="width: 180px" @change="handleFilter" />
        <ASelect v-model="filterStatus" :options="statusOptions" placeholder="状态" allow-clear style="width: 120px" @change="handleFilter" />
        <AInput v-model="filterSearch" placeholder="搜索渠道名..." allow-clear style="width: 200px" @keydown.enter="handleFilter" @clear="handleFilter" />
        <AButton @click="handleFilter">搜索</AButton>
      </ASpace>
    </ACard>

    <!-- Table -->
    <ACard :bordered="false">
      <TableStats :total="pagination.total" />
      <ATable :columns="columns" :data="data" :loading="loading" :scroll="{ x: 1400 }" :bordered="false" :stripe="true" :pagination="false" row-key="id" />
      <div class="table-footer">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchData" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchData() }" />
      </div>
    </ACard>

    <!-- Create Channel Modal -->
    <AModal v-model:visible="showModal" title="创建渠道" :width="640" :mask-closable="false" :on-before-ok="handleSubmit" :ok-loading="formLoading">
      <AForm ref="formRef" :model="form" :rules="formRules" :auto-label-width="true" layout="vertical">
        <!-- 基本信息 -->
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem field="name" label="渠道名称" required><AInput v-model="form.name" placeholder="例如：OpenAI 主力" /></AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem field="type" label="供应商类型" required><ASelect v-model="form.type" :options="providerTypeOptions" placeholder="搜索或选择" allow-search :filter-option="filterProviderOption" /></AFormItem>
          </ACol>
        </ARow>
        <AFormItem label="Base URL">
          <AInput v-model="form.base_url" :placeholder="providerDefaultURLs[form.type] || 'https://api.openai.com'" />
          <template #extra>
            <span v-if="providerDefaultURLs[form.type]" style="color:var(--color-text-3);font-size:12px">
              {{ form.base_url ? '留空则自动使用默认地址' : '将使用默认地址: ' + providerDefaultURLs[form.type] }}
            </span>
          </template>
        </AFormItem>
        <AFormItem field="api_key" label="API Key" required>
          <AInput v-model="form.api_key" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="sk-xxxx..." />
        </AFormItem>
        <!-- 调度 & 状态 -->
        <ARow :gutter="16">
          <ACol :span="8">
            <AFormItem label="调度层级">
              <ASelect v-model="form.tier" :options="tierOptions" />
              <template #extra><span class="field-help">三档固定层级，替代旧版数值优先级</span></template>
            </AFormItem>
          </ACol>
          <ACol :span="8">
            <AFormItem label="权重">
              <AInputNumber v-model="form.weight" :min="0" :max="100" class="w-full" />
              <template #extra><span class="field-help">同层级内按权重比例分配</span></template>
            </AFormItem>
          </ACol>
          <ACol :span="8">
            <AFormItem label="状态">
              <ASelect v-model="form.status" :options="[{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }, { label: '测试中', value: 'testing' }]" />
            </AFormItem>
          </ACol>
        </ARow>
        <ARow :gutter="16">
          <ACol :span="8">
            <AFormItem label="测试模型"><AInput v-model="form.test_model" placeholder="gpt-4o-mini" /></AFormItem>
          </ACol>
          <ACol :span="8">
            <AFormItem label="使用代理">
              <ASwitch v-model="form.use_proxy" />
              <template #extra><span style="color:var(--color-text-3);font-size:12px">需先在系统设置中配置代理</span></template>
            </AFormItem>
          </ACol>
          <ACol :span="8">
            <AFormItem label="严格容量">
              <ASwitch v-model="form.strict_capacity" />
              <template #extra><span style="color:var(--color-text-3);font-size:12px">Redis 故障时保守限流（高成本渠道）</span></template>
            </AFormItem>
          </ACol>
        </ARow>
        <AFormItem label="备注"><AInput v-model="form.remark" type="textarea" :auto-size="{ minRows: 2, maxRows: 4 }" placeholder="备注（选填）" /></AFormItem>
        <!-- VIP 设置 -->
        <ACollapse :bordered="false" :default-active-key="form.is_vip ? ['vip'] : []">
          <ACollapseItem header="VIP 设置" key="vip" :header-style="{ fontSize: '13px' }">
            <ARow :gutter="16">
              <ACol :span="8">
                <AFormItem label="VIP 渠道"><ASwitch v-model="form.is_vip" /></AFormItem>
              </ACol>
              <ACol v-if="form.is_vip" :span="8">
                <AFormItem label="共享阈值"><AInputNumber v-model="form.sharing_threshold" :min="0" :max="100" placeholder="共享健康分阈值" class="w-full" /></AFormItem>
              </ACol>
              <ACol v-if="form.is_vip" :span="8">
                <AFormItem label="抢占阈值"><AInputNumber v-model="form.preemption_threshold" :min="0" :max="100" placeholder="抢占优先级阈值" class="w-full" /></AFormItem>
              </ACol>
            </ARow>
            <ARow v-if="form.is_vip" :gutter="16">
              <ACol :span="8">
                <AFormItem label="借用冷却(秒)"><AInputNumber v-model="form.borrowing_cooldown_seconds" :min="0" placeholder="非VIP冷却时间" class="w-full" /></AFormItem>
              </ACol>
            </ARow>
          </ACollapseItem>
        </ACollapse>
      </AForm>
    </AModal>

    <!-- OAuth Dialog -->
    <AModal v-model:visible="showOAuthDialog" title="官方渠道 OAuth 授权" :width="560" :footer="false" :mask-closable="false">
      <!-- Step 1: Choose platform & mode -->
      <div v-if="!oauthStep2">
        <AForm :model="oauthForm" layout="vertical">
          <AFormItem label="选择平台" required>
            <ARadioGroup v-model="oauthForm.platform">
              <Radio value="claude">Claude</Radio>
              <Radio value="openai">OpenAI</Radio>
              <Radio value="codex">Codex</Radio>
              <Radio value="gemini">Gemini</Radio>
            </ARadioGroup>
          </AFormItem>
          <AFormItem v-if="oauthForm.platform === 'claude'" label="授权模式">
            <ARadioGroup v-model="oauthForm.auth_mode">
              <Radio value="browser">浏览器授权</Radio>
              <Radio value="cookie">Cookie 授权 (sessionKey)</Radio>
            </ARadioGroup>
          </AFormItem>
          <AFormItem v-if="oauthForm.auth_mode === 'cookie'" label="Session Key">
            <AInput v-model="oauthForm.session_key" type="textarea" :auto-size="{ minRows: 2 }" placeholder="粘贴 claude.ai 的 sessionKey cookie 值" />
          </AFormItem>
          <AFormItem v-if="oauthForm.platform === 'claude'" label="授权范围">
            <ARadioGroup v-model="oauthForm.scope">
              <Radio value="full">完整权限</Radio>
              <Radio value="inference">仅推理</Radio>
            </ARadioGroup>
          </AFormItem>
          <AButton type="primary" :loading="oauthLoading" long @click="handleOAuthStart">
            {{ oauthForm.auth_mode === 'cookie' ? '执行 Cookie 授权' : '生成授权链接' }}
          </AButton>
        </AForm>
      </div>
      <!-- Step 2: Enter code -->
      <div v-else>
        <AAlert type="info" class="mb-4">请在弹出窗口中完成授权，将回调 URL 中的 code 参数粘贴到下方。</AAlert>
        <AForm :model="oauthForm" layout="vertical">
          <AFormItem label="Authorization Code" required>
            <AInput v-model="oauthForm.code" placeholder="粘贴 code 参数值" />
          </AFormItem>
          <AFormItem label="State（可选）">
            <AInput v-model="oauthForm.state" placeholder="粘贴 state 参数值（如有）" />
          </AFormItem>
          <ASpace direction="vertical" fill class="w-full">
            <AButton type="primary" :loading="oauthLoading" long @click="handleOAuthExchange">换取令牌</AButton>
            <AButton long @click="oauthStep2 = false">返回上一步</AButton>
          </ASpace>
        </AForm>
      </div>
    </AModal>
  </div>
</template>

<style scoped>
.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}
.scheduling-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 16px;
  font-size: 13px;
  color: var(--color-text-3);
  cursor: pointer;
  transition: color 0.2s;
  user-select: none;
}
.scheduling-hint:hover {
  color: rgb(var(--arcoblue-6));
}
.scheduling-guide {
  color: var(--color-text-1);
  font-size: 14px;
  line-height: 1.7;
}
.guide-intro {
  margin: 0 0 14px;
  color: var(--color-text-2);
}
.guide-section {
  margin-bottom: 14px;
}
.guide-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-weight: 600;
}
.guide-index {
  display: inline-flex;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  flex: 0 0 22px;
  border-radius: 50%;
  color: rgb(var(--arcoblue-6));
  background: var(--color-primary-light-1);
  font-size: 12px;
}
.guide-section p,
.guide-example p {
  margin: 0;
}
.guide-example {
  padding: 14px 16px;
  border-radius: 6px;
  background: var(--color-fill-2);
}
.guide-example-title {
  margin-bottom: 6px;
  font-weight: 600;
}
.guide-example p + p {
  margin-top: 8px;
}
.scheduling-guide code {
  padding: 1px 4px;
  border-radius: 3px;
  color: rgb(var(--arcoblue-6));
  background: var(--color-primary-light-1);
}
.field-help {
  color: var(--color-text-3);
  font-size: 12px;
}
</style>
