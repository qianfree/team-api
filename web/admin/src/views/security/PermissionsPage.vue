<script setup lang="ts">
import { ref, computed, onMounted, reactive, h } from 'vue'
import { Message, Modal, Tag, Button, Space, Dropdown, Doption } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import request from '@/utils/request'
import { isSuperAdmin } from '@/utils/permission'

interface TierOption {
  tier: string
  label: string
  permissions: string[]
}
interface ModuleMeta {
  module: string
  label: string
  tiers: TierOption[]
}
interface RoleItem {
  id: number
  code: string
  name: string
  description: string
  is_builtin: boolean
  is_enabled: boolean
  sort: number
  user_count: number
  perm_count: number
}

const loading = ref(false)
const saving = ref(false)
const roles = ref<RoleItem[]>([])
const modules = ref<ModuleMeta[]>([])
const dangerous = ref<Record<string, string>>({})

// 角色管理是权限体系的元操作，硬性限定超管，不参与权限点授权。
// 后端 superAdminOnlyRules + assertSuperAdmin 两道闸门同样拦截，
// 这里的判断只用于隐藏操作入口，不构成安全边界。
const canManage = computed(() => isSuperAdmin())

// ── 编辑态 ─────────────────────────────────────────────────────────────────
const editorVisible = ref(false)
const advancedMode = ref(false)
const editingId = ref<number | null>(null) // null = 新建
const form = reactive({
  code: '',
  name: '',
  description: '',
  sort: 0,
  is_builtin: false,
})
// 权限点集合是唯一真相源：档位视图与高级模式都读写它，两个视图因此永远一致
const selected = ref<Set<string>>(new Set())
// 打开编辑器时的原始权限，用于识别「本次新授予的高危权限」
const originalPerms = ref<Set<string>>(new Set())

const isCreating = computed(() => editingId.value === null)

async function fetchMeta() {
  const res: any = await request.get('/admin/permissions')
  const data = res.data?.data || res.data
  modules.value = data?.modules || []
  dangerous.value = data?.dangerous || {}
}

async function fetchRoles() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/roles')
    const data = res.data?.data || res.data
    roles.value = data?.list || []
  } catch {
    roles.value = []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await Promise.all([fetchMeta(), fetchRoles()])
})

// ── 档位 ↔ 权限点 ──────────────────────────────────────────────────────────

/** 当前某模块选中的档位；不匹配任何档位时返回 '' （显示为「自定义」） */
function currentTier(mod: ModuleMeta): string {
  const owned = [...selected.value].filter(p => p.startsWith(mod.module + ':')).sort()
  if (owned.length === 0) return 'none'
  // 由高到低比对：高档位是低档位的超集，先比高档避免误判
  for (let i = mod.tiers.length - 1; i >= 0; i--) {
    const t = mod.tiers[i]
    if (t.tier === 'none') continue
    const want = [...(t.permissions || [])].sort()
    if (want.length === owned.length && want.every((p, idx) => p === owned[idx])) {
      return t.tier
    }
  }
  return ''
}

/** 选择档位：用该档位的权限点整体替换本模块的现有权限 */
function setTier(mod: ModuleMeta, tier: string) {
  const next = new Set(selected.value)
  for (const p of [...next]) {
    if (p.startsWith(mod.module + ':')) next.delete(p)
  }
  const opt = mod.tiers.find(t => t.tier === tier)
  for (const p of opt?.permissions || []) next.add(p)
  selected.value = next
}

function togglePerm(perm: string, checked: boolean) {
  const next = new Set(selected.value)
  if (checked) next.add(perm)
  else next.delete(perm)
  selected.value = next
}

function isDangerous(perm: string): boolean {
  return Object.prototype.hasOwnProperty.call(dangerous.value, perm)
}

/** 某档位是否包含高危权限（在档位选项上给出提示，避免用户无意中点出资金风险） */
function tierDangerHint(opt: TierOption): string {
  const hits = (opt.permissions || []).filter(isDangerous)
  if (!hits.length) return ''
  return hits.map(p => dangerous.value[p]).join('；')
}

// ── 编辑器 ─────────────────────────────────────────────────────────────────

function resetForm() {
  form.code = ''
  form.name = ''
  form.description = ''
  form.sort = 0
  form.is_builtin = false
  selected.value = new Set()
  originalPerms.value = new Set()
  advancedMode.value = false
}

function openCreate(copyFrom?: RoleItem) {
  resetForm()
  editingId.value = null
  if (copyFrom) {
    form.name = copyFrom.name + ' 副本'
    form.sort = copyFrom.sort
    // 复制是新建角色的主路径：建「实习运营」就是复制「运营」再收掉几个模块
    loadRolePermissions(copyFrom.id)
  }
  editorVisible.value = true
}

async function loadRolePermissions(id: number) {
  const res: any = await request.get(`/admin/roles/${id}`)
  const data = res.data?.data || res.data
  selected.value = new Set<string>(data?.permissions || [])
}

async function openEdit(row: RoleItem) {
  resetForm()
  editingId.value = row.id
  try {
    const res: any = await request.get(`/admin/roles/${row.id}`)
    const data = res.data?.data || res.data
    form.code = data.code
    form.name = data.name
    form.description = data.description
    form.sort = data.sort
    form.is_builtin = data.is_builtin
    selected.value = new Set<string>(data.permissions || [])
    originalPerms.value = new Set<string>(data.permissions || [])
    editorVisible.value = true
  } catch {
    Message.error('加载角色详情失败')
  }
}

/** 本次新授予的高危权限（已有的不重复确认） */
function newlyGrantedDangerous(): string[] {
  return [...selected.value].filter(p => isDangerous(p) && !originalPerms.value.has(p))
}

async function save() {
  if (!form.name.trim()) {
    Message.warning('请输入角色名称')
    return
  }
  if (isCreating.value && !form.code.trim()) {
    Message.warning('请输入角色标识')
    return
  }

  const risky = newlyGrantedDangerous()
  if (risky.length > 0) {
    Modal.confirm({
      title: '确认授予高危权限',
      content: `本次将授予以下权限：\n\n${risky.map(p => `· ${p} —— ${dangerous.value[p]}`).join('\n')}`,
      okText: '确认授予',
      cancelText: '再想想',
      onOk: doSave,
    })
    return
  }
  await doSave()
}

async function doSave() {
  saving.value = true
  try {
    const permissions = [...selected.value].sort()
    if (isCreating.value) {
      await request.post('/admin/roles', {
        code: form.code.trim(),
        name: form.name.trim(),
        description: form.description.trim(),
        sort: form.sort,
        permissions,
      })
      Message.success('角色已创建')
    } else {
      await request.put(`/admin/roles/${editingId.value}`, {
        name: form.name.trim(),
        description: form.description.trim(),
        sort: form.sort,
        permissions,
      })
      Message.success('角色已保存')
    }
    editorVisible.value = false
    await fetchRoles()
  } catch (e: any) {
    Message.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row: RoleItem) {
  const next = !row.is_enabled
  try {
    await request.put(`/admin/roles/${row.id}/status`, { is_enabled: next })
    Message.success(next ? '角色已启用' : '角色已禁用，其权限对关联账号立即失效')
    await fetchRoles()
  } catch (e: any) {
    Message.error(e?.response?.data?.message || '操作失败')
  }
}

function confirmDelete(row: RoleItem) {
  const affected = row.user_count
  Modal.confirm({
    title: `删除角色「${row.name}」`,
    content: affected > 0
      ? `该角色下有 ${affected} 个账号，删除后他们将失去由此角色获得的全部权限。此操作不可撤销。`
      : '该角色当前没有关联账号。此操作不可撤销。',
    okText: '确认删除',
    cancelText: '取消',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      try {
        await request.delete(`/admin/roles/${row.id}`)
        Message.success('角色已删除')
        await fetchRoles()
      } catch (e: any) {
        Message.error(e?.response?.data?.message || '删除失败')
      }
    },
  })
}

function confirmReset(row: RoleItem) {
  Modal.confirm({
    title: `恢复「${row.name}」的默认权限`,
    content: '将把该预置角色的权限重置为系统出厂值，当前的自定义修改会丢失。',
    okText: '恢复默认',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.post(`/admin/roles/${row.id}/reset`)
        Message.success('已恢复默认权限')
        await fetchRoles()
        if (editingId.value === row.id) editorVisible.value = false
      } catch (e: any) {
        Message.error(e?.response?.data?.message || '恢复失败')
      }
    },
  })
}

const selectedCount = computed(() => selected.value.size)

// 超级管理员不入角色表（由 sys_admin_users.role 判定），但它是事实上的一个身份，
// 在列表里缺席反而让人以为漏了。作为固定首行呈现，标记为不可配置。
const SUPER_ADMIN_ROW: any = {
  id: -1,
  code: 'super_admin',
  name: '超级管理员',
  description: '拥有全部权限，由账号属性直接判定，不通过角色授予',
  is_builtin: true,
  is_enabled: true,
  sort: -1,
  user_count: null,
  perm_count: null,
  __fixed: true,
}

const tableData = computed<any[]>(() => [SUPER_ADMIN_ROW, ...roles.value])

const columns: TableColumnData[] = [
  {
    title: '角色',
    dataIndex: 'name',
    width: 200,
    render({ record }) {
      return h('div', {}, [
        h('div', { class: 'role-name' }, record.name),
        h('div', { class: 'role-code' }, record.code),
      ])
    },
  },
  { title: '说明', dataIndex: 'description', ellipsis: true, tooltip: true },
  {
    title: '类型',
    dataIndex: 'is_builtin',
    width: 100,
    render({ record }) {
      if (record.__fixed) return h(Tag, { color: 'red', size: 'small' }, () => '系统内置')
      return record.is_builtin
        ? h(Tag, { color: 'arcoblue', size: 'small' }, () => '预置')
        : h(Tag, { size: 'small' }, () => '自定义')
    },
  },
  {
    title: '状态',
    dataIndex: 'is_enabled',
    width: 90,
    render({ record }) {
      if (record.__fixed) return h('span', { class: 'text-muted' }, '—')
      return record.is_enabled
        ? h(Tag, { color: 'green', size: 'small' }, () => '启用')
        : h(Tag, { color: 'gray', size: 'small' }, () => '已禁用')
    },
  },
  {
    title: '账号数',
    dataIndex: 'user_count',
    width: 90,
    render({ record }) {
      if (record.__fixed) return h('span', { class: 'text-muted' }, '—')
      return h('span', {}, String(record.user_count))
    },
  },
  {
    title: '权限数',
    dataIndex: 'perm_count',
    width: 90,
    render({ record }) {
      if (record.__fixed) return h('span', { class: 'text-muted' }, '全部')
      return h('span', {}, String(record.perm_count))
    },
  },
  {
    title: '操作',
    dataIndex: 'actions',
    width: 180,
    fixed: 'right',
    render({ record }) {
      // 超管行不可配置：它的权限不来自角色表，改不了也没有可改的东西
      if (record.__fixed) {
        return h('span', { class: 'text-muted' }, '不可配置')
      }

      const primary = h(Button, { size: 'small', type: 'primary', onClick: () => openEdit(record) },
        () => canManage.value ? '配置权限' : '查看权限')
      if (!canManage.value) {
        return primary
      }

      // 「配置权限」是日常操作，常驻；复制/启停/恢复默认/删除都是低频动作，收进「更多」，
      // 免得五个按钮把操作列撑到换行
      const items = [
        h(Doption, { onClick: () => openCreate(record) }, () => '复制'),
        h(Doption, { onClick: () => toggleStatus(record) },
          () => record.is_enabled ? '禁用' : '启用'),
      ]
      if (record.is_builtin) {
        items.push(h(Doption, { onClick: () => confirmReset(record) }, () => '恢复默认'))
      }
      // 删除会连带清掉该角色下所有账号的权限，标红
      items.push(h(Doption, { class: 'doption-danger', onClick: () => confirmDelete(record) }, () => '删除'))

      return h(Space, { size: 4 }, () => [
        primary,
        h(Dropdown, { trigger: 'click' }, {
          default: () => h(Button, { size: 'small' }, () => '更多'),
          content: () => items,
        }),
      ])
    },
  },
]
</script>

<template>
  <div class="page-table">
    <PageHeader title="角色权限" description="按岗位分配后台权限。预置角色只是初始值，可自由修改、新建或删除">
      <template #actions>
        <ASpace>
          <AButton size="small" @click="fetchRoles">刷新</AButton>
          <AButton v-if="canManage" type="primary" size="small" @click="openCreate()">新建角色</AButton>
        </ASpace>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <ResponsiveTable
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :scroll="{ x: 1100 }"
        :bordered="false"
        :stripe="true"
        row-key="id"
        card-title-key="name"
        card-subtitle-key="description"
        card-badge-key="is_enabled"
        :card-fields="['is_builtin', 'user_count', 'perm_count']"
      />
      <div class="table-footer">
        <TableStats :total="roles.length">
          <span>{{ roles.filter(r => r.is_enabled).length }} 个已启用</span>
        </TableStats>
      </div>
    </ACard>

    <!-- 权限配置 -->
    <AModal
      v-model:visible="editorVisible"
      :title="isCreating ? '新建角色' : `配置角色：${form.name}`"
      :width="760"
      :ok-loading="saving"
      :ok-button-props="{ disabled: !canManage }"
      ok-text="保存"
      cancel-text="取消"
      @ok="save"
    >
      <AForm :model="form" layout="vertical">
        <ARow :gutter="16">
          <ACol :span="12">
            <AFormItem label="角色标识">
              <AInput
                v-model="form.code"
                :disabled="!isCreating"
                placeholder="小写字母开头，如 ops_l2"
              />
              <template #extra>
                <span v-if="!isCreating">标识创建后不可修改（它是权限缓存与审计日志的稳定标识）</span>
              </template>
            </AFormItem>
          </ACol>
          <ACol :span="12">
            <AFormItem label="角色名称">
              <AInput v-model="form.name" placeholder="如 客户运营" :disabled="!canManage" />
            </AFormItem>
          </ACol>
        </ARow>
        <AFormItem label="角色说明">
          <AInput v-model="form.description" placeholder="这个角色负责什么" :disabled="!canManage" />
        </AFormItem>
      </AForm>

      <ADivider orientation="left" style="margin: 8px 0 12px">
        权限配置
        <ATag size="small" style="margin-left: 8px">已选 {{ selectedCount }} 项</ATag>
      </ADivider>

      <div class="perm-toolbar">
        <ACheckbox v-model="advancedMode">高级模式：按权限点逐个勾选</ACheckbox>
      </div>

      <!-- 档位视图：每个模块一行单选，只渲染该模块实际存在差异的档位 -->
      <div v-if="!advancedMode" class="tier-list">
        <div v-for="mod in modules" :key="mod.module" class="tier-row">
          <div class="tier-row__label">{{ mod.label }}</div>
          <ARadioGroup
            :model-value="currentTier(mod)"
            :disabled="!canManage"
            @change="(v: any) => setTier(mod, v)"
          >
            <ARadio v-for="opt in mod.tiers" :key="opt.tier" :value="opt.tier">
              {{ opt.label }}
              <ATooltip v-if="tierDangerHint(opt)" :content="tierDangerHint(opt)">
                <span class="danger-mark">⚠</span>
              </ATooltip>
            </ARadio>
            <ARadio v-if="currentTier(mod) === ''" value="" disabled>自定义</ARadio>
          </ARadioGroup>
        </div>
      </div>

      <!-- 高级模式：权限点级勾选，与档位视图共用同一份 selected 集合 -->
      <div v-else class="perm-advanced">
        <div v-for="mod in modules" :key="mod.module" class="perm-group">
          <div class="perm-group__label">{{ mod.label }}</div>
          <ASpace wrap size="mini">
            <ACheckbox
              v-for="perm in (mod.tiers[mod.tiers.length - 1]?.permissions || [])"
              :key="perm"
              :model-value="selected.has(perm)"
              :disabled="!canManage"
              @change="(c: any) => togglePerm(perm, c)"
            >
              <span :class="{ 'perm-danger': isDangerous(perm) }">
                {{ perm }}
                <ATooltip v-if="isDangerous(perm)" :content="dangerous[perm]">
                  <span class="danger-mark">⚠</span>
                </ATooltip>
              </span>
            </ACheckbox>
          </ASpace>
        </div>
      </div>
    </AModal>
  </div>
</template>

<style scoped>
.role-name {
  font-weight: 600;
  color: var(--color-text-1);
}

.role-code {
  margin-top: 2px;
  font-family: var(--font-family-mono, monospace);
  font-size: 12px;
  color: var(--color-text-3);
}

.text-muted {
  color: var(--color-text-3);
}

.table-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}

.perm-toolbar {
  margin-bottom: 12px;
}

.tier-list {
  max-height: 46vh;
  overflow-y: auto;
}

.tier-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 0;
  border-bottom: 1px solid var(--color-neutral-2);
}

.tier-row__label {
  flex: 0 0 96px;
  font-size: 13px;
  color: var(--color-text-2);
}

.perm-advanced {
  max-height: 46vh;
  overflow-y: auto;
}

.perm-group {
  padding: 8px 0;
  border-bottom: 1px solid var(--color-neutral-2);
}

.perm-group__label {
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-2);
}

.perm-danger {
  color: rgb(var(--red-6));
}

.danger-mark {
  margin-left: 2px;
  color: rgb(var(--red-6));
}

/* 下拉里的危险动作：颜色是唯一的视觉区分，必须标出来 */
:deep(.doption-danger) {
  color: rgb(var(--red-6));
}
</style>
