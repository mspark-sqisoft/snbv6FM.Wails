<!--
  체크 상자. indeterminate 가 참이면 세 번째 상태(일부 선택, 가로줄)로 그린다 ― 표 머리의 전체 선택처럼.
  label 이 없으면 aria-label 을 꼭 넘긴다. 바뀌면 onchange(checked) 를 부른다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    checked = $bindable(false),
    indeterminate = false,
    disabled = false,
    label,
    children,
    onchange,
    'aria-label': ariaLabel,
    id,
    title,
  }: {
    checked?: boolean
    indeterminate?: boolean
    disabled?: boolean
    label?: string
    children?: Snippet
    onchange?: (checked: boolean) => void
    'aria-label'?: string
    id?: string
    title?: string
  } = $props()

  const autoId = $props.id()
</script>

<label class="cb" class:disabled {title}>
  <input
    type="checkbox"
    id={id ?? autoId}
    bind:checked
    {indeterminate}
    {disabled}
    aria-label={ariaLabel}
    onchange={() => onchange?.(checked)}
  />
  <span class="box" aria-hidden="true">
    {#if indeterminate}
      <svg viewBox="0 0 16 16"><path d="M4 8h8" /></svg>
    {:else if checked}
      <svg viewBox="0 0 16 16"><path d="M3.5 8.5l3 3 6-7" /></svg>
    {/if}
  </span>
  {#if label}<span class="text">{label}</span>{:else if children}<span class="text">{@render children()}</span>{/if}
</label>

<style>
  .cb {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    cursor: pointer;
    user-select: none;
    min-width: 0;
  }
  .cb.disabled {
    cursor: default;
    opacity: 0.45;
  }
  input {
    position: absolute;
    opacity: 0;
    width: 16px;
    height: 16px;
    margin: 0;
    pointer-events: none;
  }
  .box {
    display: inline-flex;
    flex: none;
    width: 16px;
    height: 16px;
    border: 1px solid var(--color-border-strong);
    border-radius: var(--r-sm);
    background: var(--color-surface-raised);
    transition:
      background-color var(--dur-fast) var(--ease),
      border-color var(--dur-fast) var(--ease);
  }
  input:checked + .box,
  input:indeterminate + .box {
    background: var(--color-accent-fill);
    border-color: var(--color-accent-fill);
  }
  input:focus-visible + .box {
    outline: 2px solid var(--color-focus);
    outline-offset: 1px;
  }
  svg {
    width: 100%;
    height: 100%;
    fill: none;
    stroke: var(--color-text-on-accent);
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .text {
    font-size: var(--fs-body);
    color: var(--color-text);
  }
</style>
