<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Popconfirm, Message } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useIsMobile } from '@/composables/useIsMobile'
import InfoTab from './detail/InfoTab.vue'
import ModelsTab from './detail/ModelsTab.vue'
import WalletTab from './detail/WalletTab.vue'
import TransactionsTab from './detail/TransactionsTab.vue'

const route = useRoute()
const router = useRouter()
const isMobile = useIsMobile()

const loading = ref(false)
const detail = ref<any>(null)
const activeTab = ref('info')

const tenantId = route.params.id as string

// ModelsTab 实例引用，供 ATabs #extra「查看可用模型」按钮打开预览弹窗
const modelsTabRef = ref<InstanceType<typeof ModelsTab>>()

async function fetchDetail() {
  loading.value = true
  try {
    const res: any = await request.get(`/admin/tenants/${tenantId}`)
    detail.value = res.data?.data || res.data
  } catch {
  } finally {
    loading.value = false
  }
}

async function updateStatus(status: string) {
  try {
    await request.put(`/admin/tenants/${tenantId}/status`, { status })
    Message.success('状态更新成功')
    fetchDetail()
  } catch {
  }
}

function goBack() {
  router.push({ name: 'AdminTenants' })
}

onMounted(fetchDetail)
</script>

<template>
  <div class="tenant-detail-page">
    <PageHeader :title="detail ? detail.name : '租户详情'" :description="detail ? `租户代码: ${detail.code}` : ''">
      <template #actions>
        <ASpace :wrap="isMobile" :size="isMobile ? 'small' : 'medium'">
          <AButton @click="goBack">返回列表</AButton>
          <template v-if="detail">
            <Popconfirm
              v-if="detail.status === 'active'"
              content="确定暂停该租户？暂停后租户下所有成员将无法使用服务"
              @ok="updateStatus('suspended')"
            >
              <AButton status="warning">暂停</AButton>
            </Popconfirm>
            <Popconfirm
              v-if="detail.status === 'suspended'"
              content="确定恢复该租户？"
              @ok="updateStatus('active')"
            >
              <AButton status="success">恢复</AButton>
            </Popconfirm>
            <Popconfirm
              v-if="detail.status !== 'closed'"
              content="确定关闭该租户？此操作不可逆"
              @ok="updateStatus('closed')"
            >
              <AButton status="danger">关闭</AButton>
            </Popconfirm>
          </template>
        </ASpace>
      </template>
    </PageHeader>

    <ASpin :loading="loading" class="w-full">
      <template v-if="detail">
        <!-- lazy-load：未激活的 Tab 不渲染，各 Tab 组件自管数据加载 -->
        <ATabs v-model:active-key="activeTab" lazy-load>
          <template #extra>
            <AButton v-if="activeTab === 'models'" type="outline" size="small" @click="modelsTabRef?.openPreviewModal()">查看可用模型</AButton>
          </template>
          <ATabPane key="info" title="基本信息">
            <InfoTab :tenant-id="tenantId" :detail="detail" @refresh-detail="fetchDetail" />
          </ATabPane>
          <ATabPane key="models" title="模型分配">
            <ModelsTab ref="modelsTabRef" :tenant-id="tenantId" :active="activeTab === 'models'" />
          </ATabPane>
          <ATabPane key="wallet" title="钱包与计费">
            <WalletTab :tenant-id="tenantId" :active="activeTab === 'wallet'" @refresh-detail="fetchDetail" />
          </ATabPane>
          <ATabPane key="transactions" title="交易记录">
            <TransactionsTab :tenant-id="tenantId" :active="activeTab === 'transactions'" />
          </ATabPane>
        </ATabs>
      </template>
    </ASpin>
  </div>
</template>
