import { inject, type Ref } from 'vue'

export function useDemoMode(): Ref<boolean> {
  return inject<Ref<boolean>>('demoMode', { value: false } as Ref<boolean>)
}
