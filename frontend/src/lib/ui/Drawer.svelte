<!--
  아래쪽 서랍(모달 아님). 머리줄은 늘 보이고, open 이면 본문이 height(px) 만큼 열린다.
  머리줄 위 가장자리를 끌거나(마우스) 초점을 둔 채 위·아래 화살표로 높이를 바꾼다(minHeight ~ 창 높이의 maxRatio).
  끌기를 마치면 onResizeEnd(height) ― 높이 저장은 쓰는 쪽이 한다(예: 설정 transferDrawerHeight).
  title 옆 summary snippet 에 요약(진행 건수·전체 진행률 등), actions 에 머리줄 오른쪽 단추를 넣는다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import ChevronUp from '@lucide/svelte/icons/chevron-up'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import IconButton from './IconButton.svelte'
  import { t } from '../i18n'

  let {
    open = $bindable(false),
    height = $bindable(240),
    minHeight = 120,
    maxRatio = 0.7,
    title,
    summary,
    actions,
    children,
    onResizeEnd,
    id,
  }: {
    open?: boolean
    height?: number
    minHeight?: number
    maxRatio?: number
    title: string
    summary?: Snippet
    actions?: Snippet
    children?: Snippet
    onResizeEnd?: (height: number) => void
    id?: string
  } = $props()

  const bodyId = $props.id()
  let startY = 0
  let startH = 0
  let dragging = $state(false)

  const clamp = (h: number) => Math.round(Math.max(minHeight, Math.min(h, innerHeight * maxRatio)))

  function down(e: PointerEvent) {
    if (!open) return
    dragging = true
    startY = e.clientY
    startH = height
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function move(e: PointerEvent) {
    if (dragging) height = clamp(startH + (startY - e.clientY))
  }
  function up() {
    if (!dragging) return
    dragging = false
    onResizeEnd?.(height)
  }
  function key(e: KeyboardEvent) {
    if (!open || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return
    e.preventDefault()
    height = clamp(height + (e.key === 'ArrowUp' ? 16 : -16))
    onResizeEnd?.(height)
  }
</script>

<section class="drawer" class:open {id} aria-label={title}>
  {#if open}
    <!-- 초점을 받는 separator 는 ARIA 창 나누기(window splitter) 위젯이다 ― svelte 규칙이 그 쓰임을 모른다 -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="grip"
      class:dragging
      role="separator"
      aria-orientation="horizontal"
      aria-label={t('common.resize')}
      aria-valuenow={height}
      aria-valuemin={minHeight}
      tabindex="0"
      onpointerdown={down}
      onpointermove={move}
      onpointerup={up}
      onpointercancel={up}
      onkeydown={key}
    ></div>
  {/if}
  <header>
    <button class="title" type="button" aria-expanded={open} aria-controls={bodyId} onclick={() => (open = !open)}>
      {title}
    </button>
    {#if summary}<div class="summary">{@render summary()}</div>{/if}
    <div class="actions">
      {@render actions?.()}
      <IconButton
        size="sm"
        icon={open ? ChevronDown : ChevronUp}
        label={open ? t('common.collapse') : t('common.expand')}
        onclick={() => (open = !open)}
        tooltipPlacement="top"
      />
    </div>
  </header>
  <div class="body" id={bodyId} style:height={open ? `${clamp(height)}px` : '0px'} hidden={!open}>
    {@render children?.()}
  </div>
</section>

<style>
  .drawer {
    position: relative;
    z-index: var(--z-drawer);
    flex: none;
    border-top: 1px solid var(--color-border);
    background: var(--color-surface-raised);
  }
  .drawer.open {
    box-shadow: var(--shadow-2);
  }
  .grip {
    position: absolute;
    top: -3px;
    left: 0;
    right: 0;
    height: 6px;
    cursor: ns-resize;
  }
  .grip:hover,
  .grip.dragging,
  .grip:focus-visible {
    background: var(--color-focus);
    outline: none;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    height: 34px;
    padding: 0 var(--sp-2) 0 var(--sp-3);
  }
  .title {
    padding: 0;
    border: 0;
    background: none;
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--color-text);
    cursor: pointer;
  }
  .summary {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  .actions {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: var(--sp-1);
  }
  .body {
    overflow: auto;
    border-top: 1px solid var(--color-border-subtle);
  }
</style>
