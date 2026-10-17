<script setup lang="ts">
import { computed, ref, watch } from 'vue'

/**
 * 分段式一次性验证码输入框（TOTP / OTP）。
 * - 每位一个格子，输入即自动前进、退格自动回退
 * - 支持整段粘贴（`123456` / `123 456`）与方向键移动
 * - 填满后触发 complete，父组件可直接提交
 */
const props = withDefaults(
  defineProps<{
    modelValue: string
    /** 位数，默认 6 */
    length?: number
    disabled?: boolean
    /** 错误态：红框 + 抖动 */
    invalid?: boolean
    /** 挂载后聚焦第一格 */
    autofocus?: boolean
  }>(),
  { length: 6, disabled: false, invalid: false, autofocus: false },
)

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
  (e: 'complete', v: string): void
}>()

const cells = computed(() => Array.from({ length: props.length }, (_, i) => i))
const digits = ref<string[]>(Array.from({ length: props.length }, () => ''))
const shaking = ref(false)
const inputEls: (HTMLInputElement | null)[] = []

function setInputEl(el: any, i: number) {
  inputEls[i] = (el as HTMLInputElement) ?? null
}

function setDigit(i: number, v: string) {
  const next = digits.value.slice()
  next[i] = v
  digits.value = next
}

/** 同步外部值（清空、回填）。只在值真的变化时重排，避免打断用户输入 */
function syncFromValue(v: string) {
  const clean = (v || '').replace(/\D/g, '').slice(0, props.length)
  if (clean === digits.value.join('')) return
  digits.value = Array.from({ length: props.length }, (_, i) => clean[i] ?? '')
}

watch(() => props.modelValue, syncFromValue, { immediate: true })

// 触发抖动，动画结束后自动复位
watch(
  () => props.invalid,
  (v) => {
    if (!v) return
    shaking.value = false
    requestAnimationFrame(() => {
      shaking.value = true
      window.setTimeout(() => (shaking.value = false), 460)
    })
  },
)

function commit() {
  const v = digits.value.join('')
  emit('update:modelValue', v)
  return v
}

function focusAt(i: number) {
  const idx = Math.max(0, Math.min(props.length - 1, i))
  const el = inputEls[idx]
  if (!el) return
  el.focus()
  el.select()
}

function maybeComplete(v: string) {
  // 每格最多 1 位，总长等于格数即代表已填满
  if (v.length === props.length) emit('complete', v)
}

/** 从 start 位开始批量填入（粘贴 / 移动端一次性输入） */
function fillFrom(start: number, nums: string) {
  const next = digits.value.slice()
  for (let k = 0; k < nums.length && start + k < props.length; k++) {
    next[start + k] = nums[k]
  }
  digits.value = next
  const v = commit()
  const empty = next.findIndex((d) => !d)
  focusAt(empty === -1 ? props.length - 1 : empty)
  maybeComplete(v)
}

function onInput(index: number, e: Event) {
  const el = e.target as HTMLInputElement
  let nums = el.value.replace(/\D/g, '')

  if (!nums) {
    setDigit(index, '')
    el.value = ''
    commit()
    return
  }

  if (nums.length > 1) {
    // 覆盖输入时浏览器可能留下「旧值+新值」，取后一位
    const current = digits.value[index]
    if (current && nums.startsWith(current)) {
      nums = nums.slice(1)
    } else if (nums.length === props.length) {
      fillFrom(0, nums)
      return
    } else {
      fillFrom(index, nums)
      return
    }
    if (!nums) {
      setDigit(index, '')
      el.value = ''
      commit()
      return
    }
  }

  setDigit(index, nums)
  el.value = nums
  const v = commit()
  if (index < props.length - 1) focusAt(index + 1)
  maybeComplete(v)
}

function onKeydown(index: number, e: KeyboardEvent) {
  const key = e.key
  const el = e.target as HTMLInputElement

  if (key === 'Backspace') {
    e.preventDefault()
    if (digits.value[index]) {
      setDigit(index, '')
      el.value = ''
      commit()
      return
    }
    if (index > 0) {
      setDigit(index - 1, '')
      commit()
      focusAt(index - 1)
    }
    return
  }

  if (key === 'Delete') {
    e.preventDefault()
    setDigit(index, '')
    el.value = ''
    commit()
    return
  }

  if (key === 'ArrowLeft') {
    e.preventDefault()
    focusAt(index - 1)
    return
  }

  if (key === 'ArrowRight') {
    e.preventDefault()
    focusAt(index + 1)
    return
  }

  if (key === 'Enter') {
    e.preventDefault()
    maybeComplete(digits.value.join(''))
  }
}

function firstEmptyIndex() {
  return digits.value.findIndex((d) => !d)
}

function onPaste(e: ClipboardEvent) {
  e.preventDefault()
  const text = e.clipboardData?.getData('text') || ''
  const nums = text.replace(/\D/g, '')
  if (!nums) return
  if (nums.length >= props.length) {
    fillFrom(0, nums.slice(0, props.length))
    return
  }
  const empty = firstEmptyIndex()
  fillFrom(empty === -1 ? 0 : empty, nums)
}

function onFocus(e: Event) {
  ;(e.target as HTMLInputElement).select()
}

defineExpose({
  focusFirst: () => focusAt(0),
  focus: () => focusAt(firstEmptyIndex() === -1 ? 0 : firstEmptyIndex()),
  clear: () => {
    digits.value = Array.from({ length: props.length }, () => '')
    commit()
    focusAt(0)
  },
})
</script>

<template>
  <div
    class="otp"
    :class="{
      'otp--invalid': invalid,
      'otp--disabled': disabled,
      'otp--shake': shaking,
    }"
  >
    <input
      v-for="i in cells"
      :key="i"
      :ref="(el) => setInputEl(el, i)"
      :value="digits[i]"
      class="otp__cell"
      :class="{ 'otp__cell--filled': !!digits[i] }"
      type="text"
      inputmode="numeric"
      :autocomplete="i === 0 ? 'one-time-code' : 'off'"
      :aria-label="`验证码第 ${i + 1} 位`"
      maxlength="1"
      :disabled="disabled"
      :autofocus="autofocus && i === 0"
      @input="onInput(i, $event)"
      @keydown="onKeydown(i, $event)"
      @paste="onPaste"
      @focus="onFocus"
    />
  </div>
</template>

<style scoped>
.otp {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
}

.otp__cell {
  flex: 1 1 0;
  width: 100%;
  min-width: 0;
  max-width: 56px;
  height: 56px;
  padding: 0;
  border: 1.5px solid var(--ta-border, #e2e8f0);
  border-radius: 12px;
  background: #fff;
  color: var(--ta-text-primary, #0f172a);
  font-family: inherit;
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  text-align: center;
  caret-color: var(--ta-primary, #0d9488);
  transition: border-color 0.18s var(--ta-ease, ease), box-shadow 0.18s var(--ta-ease, ease),
    background 0.18s var(--ta-ease, ease);
}

.otp__cell:hover:not(:disabled) {
  border-color: var(--ta-text-weakest, #cbd5e1);
}

.otp__cell--filled {
  background: #f8fafc;
}

.otp__cell:focus {
  outline: none;
  border-color: var(--ta-primary, #0d9488);
  background: #fff;
  box-shadow: 0 0 0 3px rgba(13, 148, 136, 0.14);
}

.otp__cell:disabled {
  cursor: not-allowed;
  background: #f8fafc;
  color: var(--ta-text-tertiary, #94a3b8);
}

/* 聚焦时整格选中，选中底色跟随主题而非系统蓝 */
.otp__cell::selection {
  background: rgba(13, 148, 136, 0.18);
  color: inherit;
}

.otp--invalid .otp__cell {
  border-color: var(--ta-danger, #ef4444);
  background: #fef2f2;
  color: #b91c1c;
}

.otp--invalid .otp__cell:focus {
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.14);
}

@keyframes otpShake {
  10%,
  90% {
    transform: translateX(-1px);
  }
  20%,
  80% {
    transform: translateX(2px);
  }
  30%,
  50%,
  70% {
    transform: translateX(-4px);
  }
  40%,
  60% {
    transform: translateX(4px);
  }
}

.otp--shake {
  animation: otpShake 0.44s cubic-bezier(0.36, 0.07, 0.19, 0.97) both;
}

@media (max-width: 420px) {
  .otp {
    gap: 5px;
  }

  .otp__cell {
    height: 46px;
    border-radius: 9px;
    font-size: 18px;
  }
}
</style>
