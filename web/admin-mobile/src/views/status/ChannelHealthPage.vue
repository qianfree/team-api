<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import request from '@/utils/request'
import HealthIndicator from '@/components/HealthIndicator.vue'

const router = useRouter()
const loading = ref(false)
const channels = ref<any[]>([])

function ensureArray(data: any): any[] {
  if (Array.isArray(data)) return data
  if (data && typeof data === 'object' && Array.isArray(data.list)) return data.list
  return []
}

async function fetchData() {
  loading.value = true
  try {
    const { data: res } = await request.get('/admin/dashboard/channel-health')
    channels.value = ensureArray(res.data)
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
          v-for="ch in channels"
          :key="ch.id || ch.channel_id"
          :title="ch.name || ch.channel_name"
          is-link
          @click="router.push(`/m/channels/${ch.id || ch.channel_id}`)"
        >
          <template #value>
            <HealthIndicator :score="ch.health_score ?? ch.score ?? 0" />
          </template>
        </van-cell>
        <van-cell v-if="!channels.length" title="暂无数据" />
      </van-cell-group>
    </van-pull-refresh>
  </div>
</template>
