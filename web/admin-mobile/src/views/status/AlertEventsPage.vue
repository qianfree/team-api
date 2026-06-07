<script setup lang="ts">
import { ref, onMounted } from 'vue'
import request from '@/utils/request'

const loading = ref(false)
const events = ref<any[]>([])

function ensureArray(data: any): any[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === 'object' && Array.isArray(data.list)) return data.list
  return []
}

async function fetchData() {
  loading.value = true
  try {
    const { data: res } = await request.get('/admin/dashboard/recent-alerts')
    events.value = ensureArray(res.data)
  } catch {} finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>

<template>
  <div class="p-4">
    <van-pull-refresh v-model="loading" @refresh="fetchData">
      <van-cell-group inset>
        <van-cell
          v-for="ev in events"
          :key="ev.id"
          :title="ev.rule_name || ev.title || '告警'"
          :label="ev.message || ev.description"
        >
          <template #right-icon>
            <van-tag :type="ev.severity === 'critical' ? 'danger' : ev.severity === 'warning' ? 'warning' : 'primary'" size="medium">
              {{ ev.severity === 'critical' ? '严重' : ev.severity === 'warning' ? '警告' : '信息' }}
            </van-tag>
          </template>
        </van-cell>
        <van-cell v-if="!events.length" title="暂无告警事件" />
      </van-cell-group>
    </van-pull-refresh>
  </div>
</template>
