<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

// 分析配置抽屉（手写界面而非设置页通用渲染——布局自由、可内嵌连通性测试
// 与公网地址警示这两个通用表单做不到的能力）。
//
// 字段清单与后端设置注册表（internal/logic/common/settings_registry.go 的
// ai_analysis_* 键）一一对应，后端增删字段时需同步本组件。读写走功能自有的
// 配置端点（/admin/ai-analysis/config，sys_options 存储）：加密字段
// （ai_analysis_api_key）回显为掩码，留空/保持掩码提交即不改动现值。

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void
  (e: 'saved'): void
}>()

// 与后端 ConfigSchema Default 一致的本地默认值
const defaults = () => ({
  ai_analysis_base_url: '',
  ai_analysis_endpoint_mode: 'base',
  ai_analysis_api_key: '',
  ai_analysis_model: '',
  ai_analysis_timeout_ms: 60000,
  ai_analysis_max_tokens: 4096,
  ai_analysis_agent_prompt: '',
  ai_analysis_units_per_batch: 6,
  ai_analysis_snippet_max_len: 4000,
  ai_analysis_custom_rules: '',
  ai_analysis_patrol_enabled: false,
  ai_analysis_patrol_cron: '0 3 * * *',
  ai_analysis_patrol_scope: '{"tenants":[]}',
  ai_analysis_patrol_window_h: 24,
  ai_analysis_retention_days: 90,
})

const loading = ref(false)
const saving = ref(false)
const form = reactive(defaults())

const endpointModeOptions = [
  { label: '基地址（自动拼接 /chat/completions）', value: 'base' },
  { label: '完整路径（非标准端点）', value: 'full' },
]

watch(
  () => props.visible,
  (visible) => {
    if (visible) load()
  },
)

async function load() {
  loading.value = true
  Object.assign(form, defaults())
  try {
    const res: any = await request.get('/admin/ai-analysis/config')
    const items: any[] = res.data?.data?.list || []
    // 掩码值原样回显：提交掩码 = 保持现值；未配置的键用本地默认
    for (const item of items) {
      if (item.value !== undefined && item.value !== null && (form as any)[item.key] !== undefined) {
        ;(form as any)[item.key] = item.value
      }
    }
  } catch {
    Message.error('加载配置失败')
    emit('update:visible', false)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await request.put('/admin/ai-analysis/config', { settings: { ...form } })
    Message.success('配置已保存')
    emit('saved')
  } catch {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

// ── 公网地址警示：分析内容是敏感数据，端点指向公网时提示二次泄露风险 ────────

function isPrivateHost(url: string): boolean {
  try {
    const host = new URL(url).hostname
    if (host === 'localhost' || host.endsWith('.local') || host.endsWith('.internal') || host.endsWith('.lan')) {
      return true
    }
    return /^127\./.test(host) || /^10\./.test(host) || /^192\.168\./.test(host) ||
      /^172\.(1[6-9]|2\d|3[01])\./.test(host)
  } catch {
    return true // 非法 URL 不警示（保存/测试时会有明确报错）
  }
}

const publicEndpointWarn = computed(() => {
  const url = String(form.ai_analysis_base_url || '').trim()
  if (!url) return false
  return !isPrivateHost(url)
})

// ── 连通性测试：测的是【已保存】的配置（后端从库加载），表单改动需保存后测试 ──

const testing = ref(false)
const testResult = ref<any>(null)

async function testConnection() {
  testing.value = true
  testResult.value = null
  try {
    const res: any = await request.post('/admin/ai-analysis/test')
    testResult.value = res.data?.data || null
  } catch {
    testResult.value = null
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <ADrawer
    :visible="visible"
    title="AI请求分析 - 分析配置"
    :width="620"
    unmount-on-close
    @update:visible="emit('update:visible', $event)"
    @cancel="emit('update:visible', false)"
  >
    <ASpin :loading="loading" style="width: 100%">
      <AForm :model="form" layout="vertical">
        <!-- ── 分析 AI 连接 ── -->
        <div class="group-title">分析 AI 连接</div>
        <div class="grid2">
          <AFormItem label="分析 AI 端点" required
            extra="OpenAI 兼容端点地址（填法取决于右侧「端点模式」）">
            <AInput v-model="form.ai_analysis_base_url" placeholder="如 http://ollama:11434/v1" allow-clear />
          </AFormItem>
          <AFormItem label="端点模式"
            extra="base：填基地址自动拼 /chat/completions；full：填完整请求地址">
            <ASelect v-model="form.ai_analysis_endpoint_mode" :options="endpointModeOptions" />
          </AFormItem>
          <AFormItem label="分析 AI 密钥"
            :extra="form.ai_analysis_api_key ? '已配置（回显为掩码，留空或保持掩码提交即不改动）' : '加密存储；内网无鉴权部署（如 Ollama）可留空'">
            <AInputPassword v-model="form.ai_analysis_api_key" allow-clear />
          </AFormItem>
          <AFormItem label="分析模型" required extra="用于分析的模型名">
            <AInput v-model="form.ai_analysis_model" placeholder="如 qwen2.5:32b、gpt-4o-mini" allow-clear />
          </AFormItem>
          <AFormItem label="调用超时（毫秒）">
            <AInputNumber v-model="form.ai_analysis_timeout_ms" :min="1000" :step="1000" style="width: 100%" />
          </AFormItem>
          <AFormItem label="单次输出上限" extra="单次分析调用的 max_tokens">
            <AInputNumber v-model="form.ai_analysis_max_tokens" :min="64" :step="256" style="width: 100%" />
          </AFormItem>
        </div>
        <AAlert v-if="publicEndpointWarn" type="warning" class="mb-16">
          端点指向公网地址：分析内容（用户对话原文）将发送至外部服务，存在二次泄露风险，建议接入内网自部署模型。
        </AAlert>

        <!-- ── 连通性测试 ── -->
        <div class="conn-test">
          <AButton size="small" :loading="testing" @click="testConnection">测试连接</AButton>
          <span class="field-help">基于<strong>已保存</strong>的配置测试，表单修改后请先保存</span>
          <template v-if="testResult">
            <ATag v-if="testResult.ok" color="green" size="small">
              连通正常 · {{ testResult.latency_ms }}ms · {{ testResult.tokens_used }} tokens
            </ATag>
            <ATag v-else color="red" size="small">失败：{{ testResult.error || '未知原因' }}</ATag>
          </template>
        </div>

        <!-- ── 智能体提示词 ── -->
        <div class="group-title">智能体提示词</div>
        <AFormItem label="自定义智能体提示词"
          extra="替换内置的角色设定与双维度定义；判定质量要求与 JSON 输出格式由系统自动拼接，无需编写">
          <ATextarea
            v-model="form.ai_analysis_agent_prompt"
            :auto-size="{ minRows: 6, maxRows: 16 }"
            placeholder="留空使用内置提示词。自定义分析智能体的角色设定与审计维度，例如：&#10;你是医疗行业的合规审计专家，重点审计药品广告违规与患者隐私外发……"
            allow-clear
          />
        </AFormItem>

        <!-- ── 分析行为 ── -->
        <div class="group-title">分析行为</div>
        <div class="grid2">
          <AFormItem label="每批分析单元数" extra="每批送入 AI 的会话/独立单元数">
            <AInputNumber v-model="form.ai_analysis_units_per_batch" :min="1" style="width: 100%" />
          </AFormItem>
          <AFormItem label="单元截断长度（字符）" extra="单个分析单元送入 AI 的最大长度">
            <AInputNumber v-model="form.ai_analysis_snippet_max_len" :min="200" :step="200" style="width: 100%" />
          </AFormItem>
        </div>
        <AFormItem label="自定义合规规则" extra="与自定义智能体提示词可共存">
          <ATextarea
            v-model="form.ai_analysis_custom_rules"
            :auto-size="{ minRows: 3, maxRows: 8 }"
            placeholder="追加到分析提示词的额外规则（如行业监管要求），可选"
            allow-clear
          />
        </AFormItem>

        <!-- ── 定时巡查 ── -->
        <div class="group-title">定时巡查</div>
        <div class="grid2">
          <AFormItem label="启用定时巡查" extra="按计划自动对巡查范围内的租户产出分析报告">
            <ASwitch v-model="form.ai_analysis_patrol_enabled" />
          </AFormItem>
          <AFormItem label="巡查回看窗口（小时）" extra="每次巡查分析的回看时间窗口">
            <AInputNumber v-model="form.ai_analysis_patrol_window_h" :min="1" style="width: 100%" />
          </AFormItem>
          <AFormItem label="巡查计划" extra="cron 表达式；保存后自动重排生效">
            <AInput v-model="form.ai_analysis_patrol_cron" placeholder="0 3 * * *" allow-clear />
          </AFormItem>
          <AFormItem label="报告保留天数" extra="超期由清理任务自动删除">
            <AInputNumber v-model="form.ai_analysis_retention_days" :min="1" style="width: 100%" />
          </AFormItem>
        </div>
        <AFormItem label="巡查范围" extra="指定租户 ID 列表，空数组表示全部活跃租户">
          <ATextarea
            v-model="form.ai_analysis_patrol_scope"
            :auto-size="{ minRows: 2, maxRows: 4 }"
            placeholder='{"tenants":[]}'
          />
        </AFormItem>
      </AForm>
    </ASpin>

    <template #footer>
      <ASpace>
        <AButton @click="emit('update:visible', false)">取消</AButton>
        <AButton type="primary" :loading="saving" @click="save">保存配置</AButton>
      </ASpace>
    </template>
  </ADrawer>
</template>

<style scoped>
.group-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-2);
  margin: 4px 0 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--color-border-2);
}

.group-title:not(:first-child) {
  margin-top: 20px;
}

.grid2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 16px;
}

@media (max-width: 640px) {
  .grid2 {
    grid-template-columns: 1fr;
  }
}

.field-help {
  font-size: 12px;
  color: var(--color-text-3);
  margin-top: 2px;
  line-height: 1.5;
}

.conn-test {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin: 12px 0 4px;
}

.mb-16 {
  margin-bottom: 16px;
}
</style>
