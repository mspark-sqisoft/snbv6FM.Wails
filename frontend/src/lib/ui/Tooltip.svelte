<!--
  툴팁. 감싼 것 위에 마우스를 올리거나 키보드 초점이 오면 delay(ms) 뒤에 text 를 띄운다.
  top layer(popover)에 그려 표·대화상자 안에서도 잘리지 않는다. 화면 읽기 프로그램에는 aria-describedby 로 이어진다.
  아이콘 단추처럼 글자가 없는 것에는 IconButton 이 이미 툴팁을 단다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    text,
    placement = 'bottom',
    delay = 400,
    disabled = false,
    children,
  }: {
    text: string
    placement?: 'top' | 'bottom' | 'left' | 'right'
    delay?: number
    disabled?: boolean
    children: Snippet
  } = $props()

  const id = $props.id()
  let anchor: HTMLSpanElement
  let tip: HTMLDivElement
  let timer: ReturnType<typeof setTimeout> | undefined

  function show() {
    if (disabled || !text) return
    clearTimeout(timer)
    timer = setTimeout(() => {
      if (!tip || !anchor.isConnected) return
      tip.showPopover()
      place()
    }, delay)
  }

  function hide() {
    clearTimeout(timer)
    if (tip?.matches(':popover-open')) tip.hidePopover()
  }

  function place() {
    const a = anchor.getBoundingClientRect()
    const t = tip.getBoundingClientRect()
    const gap = 6
    let x = a.left + a.width / 2 - t.width / 2
    let y = a.bottom + gap
    if (placement === 'top') y = a.top - t.height - gap
    if (placement === 'left') [x, y] = [a.left - t.width - gap, a.top + a.height / 2 - t.height / 2]
    if (placement === 'right') [x, y] = [a.right + gap, a.top + a.height / 2 - t.height / 2]
    if (placement === 'bottom' && y + t.height > innerHeight) y = a.top - t.height - gap
    x = Math.max(4, Math.min(x, innerWidth - t.width - 4))
    y = Math.max(4, Math.min(y, innerHeight - t.height - 4))
    tip.style.left = `${x}px`
    tip.style.top = `${y}px`
  }

  $effect(() => () => clearTimeout(timer))
</script>

<span
  class="anchor"
  bind:this={anchor}
  aria-describedby={text && !disabled ? id : undefined}
  onpointerenter={show}
  onpointerleave={hide}
  onpointerdown={hide}
  onfocusin={show}
  onfocusout={hide}
  role="presentation"
>
  {@render children()}
</span>
<div class="tip" {id} role="tooltip" popover="manual" bind:this={tip}>{text}</div>

<style>
  .anchor {
    display: inline-flex;
    min-width: 0;
  }
  .tip {
    position: fixed;
    inset: auto;
    margin: 0;
    max-width: 320px;
    padding: var(--sp-1) var(--sp-2);
    border: 1px solid var(--color-border);
    border-radius: var(--r-sm);
    background: var(--color-surface-raised);
    color: var(--color-text);
    box-shadow: var(--shadow-2);
    font-size: var(--fs-sm);
    line-height: var(--lh-tight);
    white-space: pre-line;
    pointer-events: none;
  }
</style>
