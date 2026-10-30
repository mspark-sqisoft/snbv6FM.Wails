<!--
  탭 줄(role="tablist"). value 가 고른 탭 id(bind:value), 바뀌면 onchange(id).
  왼쪽·오른쪽 화살표·Home·End 로 옮긴다(옮기면 바로 고른다). 탭 본문은 쓰는 쪽이 value 로 그린다 ―
  본문 요소에 id="{idPrefix}-panel-{tab.id}" 를 주면 aria-controls 가 이어진다.
  variant: line(밑줄, 화면 안 탭) · nav(상단 메뉴 탭, 원본 topTabBar)
-->
<script lang="ts">
  import type { TabItem } from './types'

  let {
    items,
    value = $bindable(),
    label,
    variant = 'line',
    idPrefix,
    onchange,
    id,
  }: {
    items: TabItem[]
    value?: string
    label: string
    variant?: 'line' | 'nav'
    idPrefix?: string
    onchange?: (id: string) => void
    id?: string
  } = $props()

  const auto = $props.id()
  const prefix = $derived(idPrefix ?? auto)
  let list: HTMLDivElement

  function pick(tab: TabItem) {
    if (tab.disabled || tab.id === value) return
    value = tab.id
    onchange?.(tab.id)
  }

  function key(e: KeyboardEvent) {
    const enabled = items.filter((i) => !i.disabled)
    const i = enabled.findIndex((t) => t.id === value)
    let n = -1
    if (e.key === 'ArrowRight') n = (i + 1) % enabled.length
    else if (e.key === 'ArrowLeft') n = (i - 1 + enabled.length) % enabled.length
    else if (e.key === 'Home') n = 0
    else if (e.key === 'End') n = enabled.length - 1
    if (n < 0) return
    e.preventDefault()
    pick(enabled[n])
    list.querySelector<HTMLElement>(`[data-tab="${enabled[n].id}"]`)?.focus()
  }
</script>

<div class="tabs {variant}" role="tablist" aria-label={label} bind:this={list} {id} tabindex="-1" onkeydown={key}>
  {#each items as tab (tab.id)}
    {@const sel = tab.id === value}
    <button
      type="button"
      role="tab"
      id="{prefix}-tab-{tab.id}"
      data-tab={tab.id}
      aria-selected={sel}
      aria-controls="{prefix}-panel-{tab.id}"
      tabindex={sel ? 0 : -1}
      disabled={tab.disabled}
      onclick={() => pick(tab)}
    >
      {#if tab.icon}<tab.icon size={16} aria-hidden="true" />{/if}
      <span>{tab.label}</span>
      {#if tab.count != null}<span class="count">{tab.count}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: var(--sp-1);
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tabs:focus {
    outline: none;
  }
  button {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    height: 36px;
    padding: 0 var(--sp-3);
    border: 0;
    background: none;
    color: var(--color-text-2);
    font-size: var(--fs-body);
    font-weight: var(--fw-medium);
    white-space: nowrap;
    cursor: pointer;
    border-radius: var(--r-md) var(--r-md) 0 0;
    transition:
      color var(--dur-fast) var(--ease),
      background-color var(--dur-fast) var(--ease);
  }
  button:hover:not(:disabled) {
    color: var(--color-text);
    background: var(--color-hover);
  }
  button:disabled {
    opacity: 0.4;
    cursor: default;
  }
  button[aria-selected='true'] {
    color: var(--color-text);
    font-weight: var(--fw-semibold);
  }
  button[aria-selected='true']::after {
    content: '';
    position: absolute;
    left: var(--sp-2);
    right: var(--sp-2);
    bottom: 0;
    height: 2px;
    border-radius: 2px;
    background: var(--color-accent-fill);
  }
  button[aria-selected='true'] :global(svg) {
    color: var(--color-accent);
  }
  .nav {
    padding: 0 var(--sp-3);
    border-bottom: 1px solid var(--color-border-subtle);
    background: var(--color-bg-chrome);
  }
  .line {
    border-bottom: 1px solid var(--color-border-subtle);
  }
  .count {
    min-width: 18px;
    padding: 0 5px;
    border-radius: var(--r-full);
    background: var(--color-neutral-subtle);
    color: var(--color-text-2);
    font-size: var(--fs-caption);
    line-height: 16px;
    text-align: center;
  }
</style>
