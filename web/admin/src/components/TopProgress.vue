<script setup lang="ts">
import { useTopProgress } from '@/composables/useTopProgress'

const { progress, visible, smooth } = useTopProgress()
</script>

<template>
  <div class="top-progress-container" :class="{ 'is-visible': visible }">
    <div class="top-progress-bar" :class="{ 'no-anim': !smooth }" :style="{ width: progress + '%' }" />
  </div>
</template>

<style scoped>
.top-progress-container {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 3px;
  z-index: 9999;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.3s ease;
}
.top-progress-container.is-visible {
  opacity: 1;
}
.top-progress-bar {
  height: 100%;
  background: var(--ta-primary-gradient);
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
