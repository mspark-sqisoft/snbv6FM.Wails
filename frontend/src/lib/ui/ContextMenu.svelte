<!--
  우클릭 메뉴(role="menu"). open·x·y 로 띄운다(화면 밖으로 나가면 안으로 당긴다).
  위·아래 화살표·Home·End 로 옮기고 Enter·Space 로 고르며, Esc·바깥 누름·고르기 뒤 닫힌다(bind:open, onClose).
  보통은 DataTable 의 oncontextmenu(e, row, keys) 에서 x=e.clientX, y=e.clientY 로 연다.
-->
<script lang="ts">
  import type { MenuItem } from './types'

  let {
    open = $bindable(false),
    x = 0,
    y = 0,
    items,
    label,
    onClose,
  }: {
    open?: boolean
    x?: number
    y?: number
    items: MenuItem[]
    label: string
    onClose?: () => void
  } = $props()

  let el: HTMLDivElement
  let returnFocus: HTMLElement | null = null

  $effect(() => {
    if (open) {
      returnFocus = document.activeElement as HTMLElement | null
      el.showPopover()
      const r = el.getBoundingClientRect()
      el.style.left = `${Math.max(4, Math.min(x, innerWidth - r.width - 4))}px`
      el.style.top = `${Math.max(4, Math.min(y, innerHeight - r.height - 4))}px`
      buttons()[0]?.focus()
      const outside = (e: PointerEvent) => {
        if (!el.contains(e.target as Node)) close()
      }
      const t = setTimeout(() => addEventListener('pointerdown', outside, true))
      addEventListener('blur', close)
      addEventListener('resize', close)
      return () => {
        clearTimeout(t)
        removeEventListener('pointerdown', outside, true)
        removeEventListener('blur', close)
        removeEventListener('resize', close)
        if (el.matches(':popover-open')) el.hidePopover()
      }
    }
  })

  const buttons = () => [...el.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')]

  function close() {
    if (!open) return
    open = false
    onClose?.()
    returnFocus?.focus?.()
  }

  function choose(item: MenuItem) {
    close()
    item.onSelect?.()
  }

  function key(e: KeyboardEvent) {
    const bs = buttons()
    const i = bs.indexOf(document.activeElement as HTMLButtonElement)
    let n = -1
    if (e.key === 'ArrowDown') n = (i + 1) % bs.length
    else if (e.key === 'ArrowUp') n = (i - 1 + bs.length) % bs.length
    else if (e.key === 'Home') n = 0
    else if (e.key === 'End') n = bs.length - 1
    else if (e.key === 'Escape' || e.key === 'Tab') {
      e.preventDefault()
      close()
      return
    }
    if (n >= 0) {
      e.preventDefault()
      bs[n]?.focus()
    }
  }
</script>

<div bind:this={el} class="menu" popover="manual" role="menu" aria-label={label} tabindex="-1" onkeydown={key}>
  {#if open}
    {#each items as item, i (i)}
      {#if item.separator}
        <div class="sep" role="separator"></div>
      {:else}
        <button type="button" role="menuitem" class:danger={item.danger} disabled={item.disabled} onclick={() => choose(item)}>
          <span class="ico">{#if item.icon}<item.icon size={15} aria-hidden="true" />{/if}</span>
          <span class="lbl">{item.label}</span>
          {#if item.hint}<span class="hint">{item.hint}</span>{/if}
        </button>
      {/if}
    {/each}
  {/if}
</div>

<style>
  .menu {
    position: fixed;
    inset: auto;
    margin: 0;
    min-width: 180px;
    max-width: 320px;
    padding: var(--sp-1);
    border: 1px solid var(--color-border);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    color: var(--color-text);
    box-shadow: var(--shadow-3);
  }
  .menu:focus {
    outline: none;
  }
  button {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    width: 100%;
    height: 30px;
    padding: 0 var(--sp-2);
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    font-size: var(--fs-body);
    text-align: left;
    cursor: pointer;
  }
  button:hover:not(:disabled),
  button:focus-visible {
    background: var(--color-hover);
    outline: none;
  }
  button:disabled {
    color: var(--color-text-disabled);
    cursor: default;
  }
  .danger {
    color: var(--color-danger);
  }
  .ico {
    display: flex;
    width: 16px;
    color: var(--color-text-3);
  }
  .danger .ico {
    color: inherit;
  }
  .lbl {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
  .sep {
    height: 1px;
    margin: var(--sp-1) 0;
    background: var(--color-border-subtle);
  }
</style>
