<script setup lang="ts">
import { useTopProgress } from '@/composables/useTopProgress'

const { progress, visible, smooth } = useTopProgress()
</script>

<template>
  <div
    class="top-progress-container fixed top-0 left-0 z-[9999] h-[3px] w-full pointer-events-none"
    :class="{ 'is-visible': visible }"
  >
    <div
      class="top-progress-bar h-full bg-gradient-to-r from-primary-500 to-primary-600"
      :class="{ 'no-anim': !smooth }"
      :style="{ width: progress + '%' }"
    />
  </div>
</template>

<style scoped>
.top-progress-container {
  opacity: 0;
  transition: opacity 0.3s ease;
}
.top-progress-container.is-visible {
  opacity: 1;
}
.top-progress-bar {
  transition: width 0.2s ease;
}
.top-progress-bar.no-anim {
  transition: none;
}
@media (prefers-reduced-motion: reduce) {
  .top-progress-container,
  .top-progress-bar {
    transition-duration: 1ms;
  }
}
</style>
