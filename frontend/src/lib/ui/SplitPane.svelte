<!--
  좌우로 나눈 두 칸. 가운데 경계를 끌거나 초점을 둔 채 왼쪽·오른쪽 화살표로 왼쪽 폭(size, px)을 바꾼다.
  min·max 는 왼쪽 폭의 한계, 오른쪽은 minRight 아래로 줄지 않는다. 끝나면 onResizeEnd(size) ― 저장은 쓰는 쪽이 한다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import { t } from '../i18n'

  let {
    size = $bindable(240),
    min = 120,
    max = 800,
    minRight = 200,
    left,
    right,
    onResizeEnd,
    label,
  }: {
    size?: number
    min?: number
    max?: number
    minRight?: number
    left: Snippet
    right: Snippet
    onResizeEnd?: (size: number) => void
    label?: string
  } = $props()

  let box: HTMLDivElement
  let dragging = $state(false)
  let startX = 0
  let startSize = 0

  function clamp(v: number) {
    const limit = Math.min(max, (box?.clientWidth ?? Infinity) - minRight)
    return Math.round(Math.max(min, Math.min(v, limit)))
  }
  function down(e: PointerEvent) {
    dragging = true
    startX = e.clientX
    startSize = size
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function move(e: PointerEvent) {
    if (dragging) size = clamp(startSize + e.clientX - startX)
  }
  function up() {
    if (!dragging) return
    dragging = false
    onResizeEnd?.(size)
  }
  function key(e: KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    size = clamp(size + (e.key === 'ArrowRight' ? 16 : -16))
    onResizeEnd?.(size)
  }
</script>

<div class="split" class:dragging bind:this={box}>
  <div class="pane" style:width="{size}px">{@render left()}</div>
  <!-- 초점을 받는 separator 는 ARIA 창 나누기(window splitter) 위젯이다 ― svelte 규칙이 그 쓰임을 모른다 -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div
    class="gutter"
    role="separator"
    aria-orientation="vertical"
    aria-label={label ?? t('common.resize')}
    aria-valuenow={size}
    aria-valuemin={min}
    aria-valuemax={max}
    tabindex="0"
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={up}
    onkeydown={key}
  ></div>
  <div class="pane rest">{@render right()}</div>
</div>

<style>
  .split {
    display: flex;
    min-height: 0;
    height: 100%;
  }
  .dragging {
    cursor: ew-resize;
    user-select: none;
  }
  .pane {
    flex: none;
    min-width: 0;
    overflow: hidden;
  }
  .rest {
    flex: 1;
  }
  .gutter {
    flex: none;
    position: relative;
    width: 1px;
    background: var(--color-border-subtle);
    cursor: ew-resize;
  }
  .gutter::before {
    content: '';
    position: absolute;
    inset: 0 -3px;
  }
  .gutter:hover,
  .dragging .gutter,
  .gutter:focus-visible {
    background: var(--color-focus);
    outline: none;
    box-shadow: 0 0 0 1px var(--color-focus);
  }
</style>
