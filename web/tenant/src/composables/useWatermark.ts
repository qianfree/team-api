import { watch, onUnmounted, type Ref } from 'vue'

interface WatermarkOptions {
  text: string
  width?: number
  height?: number
  fontSize?: number
  color?: string
  gapX?: number
  gapY?: number
  rotate?: number
}

function generateWatermarkSvg(opt: WatermarkOptions): string {
  const {
    text,
    width = 200,
    height = 160,
    fontSize = 14,
    color = 'rgba(0, 0, 0, 0.08)',
    gapX = 100,
    gapY = 60,
    rotate = -22,
  } = opt

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width + gapX}" height="${height + gapY}">
    <text
      x="${(width + gapX) / 2}"
      y="${(height + gapY) / 2}"
      font-size="${fontSize}"
      font-family="system-ui, -apple-system, sans-serif"
      fill="${color}"
      text-anchor="middle"
      dominant-baseline="middle"
      transform="rotate(${rotate}, ${(width + gapX) / 2}, ${(height + gapY) / 2})"
    >${text}</text>
  </svg>`

  return `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(svg)))}`
}

export function useWatermark(text: Ref<string>) {
  let observer: MutationObserver | null = null

  function mount(el: HTMLElement) {
    const overlay = document.createElement('div')
    overlay.setAttribute('data-watermark', '')
    Object.assign(overlay.style, {
      position: 'fixed',
      inset: '0',
      'pointer-events': 'none',
      'z-index': '9999',
      'background-repeat': 'repeat',
    })

    const apply = () => {
      const t = text.value || '演示环境'
      overlay.style.backgroundImage = `url("${generateWatermarkSvg({ text: t })}")`
    }
    apply()

    el.appendChild(overlay)

    watch(text, apply)

    observer = new MutationObserver((mutations) => {
      for (const m of mutations) {
        if (m.type === 'childList') {
          for (const node of Array.from(m.removedNodes)) {
            if (node === overlay) {
              el.appendChild(overlay)
              apply()
              return
            }
          }
        }
        if (m.type === 'attributes' && m.target === overlay) {
          apply()
          return
        }
      }
    })
    observer.observe(el, { childList: true, subtree: true })
    observer.observe(overlay, { attributes: true })
  }

  function unmount() {
    observer?.disconnect()
    observer = null
    document.querySelectorAll('[data-watermark]').forEach((el) => el.remove())
  }

  onUnmounted(unmount)

  return { mount, unmount }
}
