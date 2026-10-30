<!--
  고르기 칸(운영체제 기본 목록을 쓰는 <select>). options 의 value 는 문자열 또는 숫자.
  label 이 없으면 aria-label 을 꼭 넘긴다.
-->
<script lang="ts" generics="V extends string | number">
  import type { HTMLSelectAttributes } from 'svelte/elements'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import type { Option } from './types'

  type Props = Omit<HTMLSelectAttributes, 'value' | 'size'> & {
    value?: V
    options: Option<V>[]
    label?: string
    size?: 'sm' | 'md'
    placeholder?: string
  }

  let { value = $bindable(), options, label, size = 'md', placeholder, id, ...rest }: Props = $props()
  const autoId = $props.id()
  const selId = $derived(id ?? autoId)
</script>

<div class="field">
  {#if label}<label class="label" for={selId}>{label}</label>{/if}
  <div class="box {size}">
    <select {...rest} id={selId} bind:value>
      {#if placeholder}<option value={undefined} disabled>{placeholder}</option>{/if}
      {#each options as o (o.value)}
        <option value={o.value} disabled={o.disabled}>{o.label}</option>
      {/each}
    </select>
    <ChevronDown size={14} aria-hidden="true" class="chev" />
  </div>
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    min-width: 0;
  }
  .label {
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--color-text-2);
  }
  .box {
    position: relative;
    display: flex;
    align-items: center;
    height: var(--h-md);
  }
  .box.sm {
    height: var(--h-sm);
  }
  select {
    appearance: none;
    width: 100%;
    height: 100%;
    padding: 0 28px 0 var(--sp-3);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    font-size: var(--fs-body);
    cursor: pointer;
  }
  .sm select {
    font-size: var(--fs-sm);
    padding-left: var(--sp-2);
  }
  select:hover:not(:disabled) {
    border-color: var(--color-text-3);
  }
  select:focus-visible {
    outline: none;
    border-color: var(--color-focus);
    box-shadow: 0 0 0 1px var(--color-focus);
  }
  select:disabled {
    opacity: 0.5;
    cursor: default;
  }
  option {
    background: var(--color-surface-raised);
    color: var(--color-text);
  }
  .box :global(.chev) {
    position: absolute;
    right: var(--sp-2);
    pointer-events: none;
    color: var(--color-text-3);
  }
</style>
