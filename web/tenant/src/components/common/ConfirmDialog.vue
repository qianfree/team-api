<template>
  <BaseModal
    :show="visible"
    :title="options.title || '确认操作'"
    width="narrow"
    @close="handleCancel"
  >
    <p class="text-sm text-gray-600">{{ options.message }}</p>

    <template #footer>
      <button @click="handleCancel" class="btn btn-secondary">{{ options.cancelText || '取消' }}</button>
      <button
        @click="handleConfirm"
        :class="options.danger ? 'btn btn-danger' : 'btn btn-primary'"
      >
        {{ options.confirmText || '确认' }}
      </button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from './BaseModal.vue'
import { useConfirm } from '@/composables/useConfirm'

const { visible, options, handleConfirm, handleCancel } = useConfirm()
</script>
