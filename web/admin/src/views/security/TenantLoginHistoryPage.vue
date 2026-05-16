<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const list = ref<any[]>([])
const loading = ref(false)
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showTotal: true,
})

const searchForm = reactive({
  tenant_id: undefined as number | undefined,
  username: '',
  ip_address: '',
  success: undefined as boolean | undefined,
  login_method: '',
  start_time: '',
  end_time: '',
})

const successOptions = [
  { label: '全部', value: undefined },
  { label: '成功', value: true },
  { label: '失败', value: false },
]

const methodOptions = [
  { label: '全部', value: '' },
  { label: '密码', value: 'password' },
  { label: '双因素', value: 'totp' },
  { label: '单点登录', value: 'sso' },
  { label: '恢复码', value: 'backup_code' },
]

onMounted(() => {
  fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    if (searchForm.tenant_id) params.tenant_id = searchForm.tenant_id
    if (searchForm.username) params.username = searchForm.username
    if (searchForm.ip_address) params.ip_address = searchForm.ip_address
    if (searchForm.success !== undefined) params.success = searchForm.success
    if (searchForm.login_method) params.login_method = searchForm.login_method
    if (searchForm.start_time) params.start_time = searchForm.start_time
    if (searchForm.end_time) params.end_time = searchForm.end_time

    const res = await request.get('/admin/security/tenant-login-history', { params })
    if (res.data?.code === 0) {
      list.value = res.data.data.list || []
      pagination.total = res.data.data.total || 0
    }
  } finally {
    loading.value = false
  }
}

function onPageChange(page: number) {
  pagination.current = page
  fetchData()
}

function onSearch() {
  pagination.current = 1
  fetchData()
}

function onReset() {
  searchForm.tenant_id = undefined
  searchForm.username = ''
  searchForm.ip_address = ''
  searchForm.success = undefined
  searchForm.login_method = ''
  searchForm.start_time = ''
  searchForm.end_time = ''
  pagination.current = 1
  fetchData()
}

function methodLabel(method: string) {
  const map: Record<string, string> = {
    password: '密码',
    totp: '双因素',
    sso: '单点登录',
    backup_code: '恢复码',
  }
  return map[method] || method
}

function methodColor(method: string) {
  const map: Record<string, string> = {
    password: 'blue',
    totp: 'green',
    sso: 'purple',
    backup_code: 'orange',
  }
  return map[method] || 'gray'
}

function onDateChange(dateString: string | undefined, type: 'start' | 'end') {
  if (type === 'start') {
    searchForm.start_time = dateString || ''
  } else {
    searchForm.end_time = dateString || ''
  }
}
</script>

<template>
  <div class="page-table">
    <PageHeader title="租户登录历史" description="查看所有租户用户的登录记录" />

    <ACard :bordered="false" style="margin-bottom: 16px">
      <AForm :model="searchForm" layout="inline" @submit="onSearch">
        <AFormItem label="租户ID">
          <AInputNumber v-model="searchForm.tenant_id" placeholder="租户ID" allow-clear style="width: 120px" />
        </AFormItem>
        <AFormItem label="用户名">
          <AInput v-model="searchForm.username" placeholder="搜索用户名" allow-clear style="width: 140px" />
        </AFormItem>
        <AFormItem label="IP 地址">
          <AInput v-model="searchForm.ip_address" placeholder="搜索 IP" allow-clear style="width: 140px" />
        </AFormItem>
        <AFormItem label="状态">
          <ASelect v-model="searchForm.success" :options="successOptions" placeholder="全部" allow-clear style="width: 100px" />
        </AFormItem>
        <AFormItem label="登录方式">
          <ASelect v-model="searchForm.login_method" :options="methodOptions" placeholder="全部" allow-clear style="width: 120px" />
        </AFormItem>
        <AFormItem label="开始日期">
          <ADatePicker v-model="searchForm.start_time" style="width: 150px" @change="(v: any) => onDateChange(v, 'start')" />
        </AFormItem>
        <AFormItem label="结束日期">
          <ADatePicker v-model="searchForm.end_time" style="width: 150px" @change="(v: any) => onDateChange(v, 'end')" />
        </AFormItem>
        <AFormItem>
          <ASpace>
            <AButton type="primary" html-type="submit">搜索</AButton>
            <AButton @click="onReset">重置</AButton>
          </ASpace>
        </AFormItem>
      </AForm>
    </ACard>

    <ACard :bordered="false">
      <ATable
        :data="list"
        :loading="loading"
        :bordered="false"
        :stripe="true"
        row-key="id"
        :pagination="{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total: pagination.total,
          showTotal: true,
          onChange: onPageChange,
        }"
      >
        <template #columns>
          <ATableColumn title="时间" data-index="created_at" :width="180" />
          <ATableColumn title="租户ID" data-index="tenant_id" :width="90" />
          <ATableColumn title="用户" :width="160">
            <template #cell="{ record }">
              <span>{{ record.display_name || record.username }}</span>
              <span v-if="record.display_name" style="color: var(--ta-text-tertiary); margin-left: 4px">({{ record.username }})</span>
            </template>
          </ATableColumn>
          <ATableColumn title="登录方式" data-index="login_method" :width="110">
            <template #cell="{ record }">
              <ATag :color="methodColor(record.login_method)" size="small">{{ methodLabel(record.login_method) }}</ATag>
            </template>
          </ATableColumn>
          <ATableColumn title="IP 地址" data-index="ip_address" :width="150" />
          <ATableColumn title="设备" data-index="user_agent" :width="200" ellipsis />
          <ATableColumn title="状态" :width="80">
            <template #cell="{ record }">
              <ATag :color="record.success ? 'green' : 'red'" size="small">{{ record.success ? '成功' : '失败' }}</ATag>
            </template>
          </ATableColumn>
          <ATableColumn title="新设备" :width="80">
            <template #cell="{ record }">
              <ATag v-if="record.is_new_device" color="orangered" size="small">新设备</ATag>
              <span v-else>-</span>
            </template>
          </ATableColumn>
          <ATableColumn title="失败原因" data-index="fail_reason" :width="180" ellipsis>
            <template #cell="{ record }">
              <span v-if="record.fail_reason">{{ record.fail_reason }}</span>
              <span v-else>-</span>
            </template>
          </ATableColumn>
        </template>
      </ATable>
    </ACard>
  </div>
</template>
