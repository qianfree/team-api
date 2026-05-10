<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import {
  Tag, Space,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const groups = ref<any[]>([])

const columns: TableColumnData[] = [
  { title: '模块', dataIndex: 'name', width: 140 },
  { title: '标签', dataIndex: 'label', width: 140 },
  {
    title: '权限点',
    dataIndex: 'permissions',
    render({ record }) {
      return h(Space, { size: [4, 8], wrap: true }, () =>
        record.permissions.map((p: string) =>
          h(Tag, { size: 'small', color: 'green' }, () => p)
        )
      )
    },
  },
  {
    title: '数量',
    dataIndex: 'count',
    width: 80,
    render({ record }) { return record.permissions.length },
  },
]

async function fetchPermissions() {
  loading.value = true
  try {
    const res: any = await request.get('/admin/permissions')
    const raw = res.data?.data || res.data
    groups.value = Array.isArray(raw?.groups) ? raw.groups : (Array.isArray(raw) ? raw : [])
  } catch {
    groups.value = []
  } finally {
    loading.value = false
  }
}

onMounted(fetchPermissions)
</script>

<template>
  <div class="page-table">
    <PageHeader title="权限管理" description="查看系统权限点分组列表">
      <template #actions>
        <AButton size="small" @click="fetchPermissions">刷新</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <ATable
        :columns="columns"
        :data="groups"
        :loading="loading"
        :bordered="false"
        :stripe="true"
        :pagination="false"
        row-key="id"
      />
      <div class="table-footer">
        <span class="text-sm" style="color: var(--ta-text-tertiary)">
          共 {{ groups.length }} 个权限组，{{ groups.reduce((s, g) => s + g.permissions.length, 0) }} 个权限点
        </span>
      </div>
    </ACard>
  </div>
</template>

<style scoped>
.table-footer {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--ta-border-light);
}
</style>
