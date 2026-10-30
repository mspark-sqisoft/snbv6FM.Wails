<!--
  단추. 화면 하나에 primary 는 하나(원본 content/README.md).
  variant: primary(주) · secondary(보조, 기본) · danger(위험) · ghost(테두리 없음)
  loading 이면 누를 수 없고 aria-busy 가 켜진다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import type { Icon } from './types'
  import Spinner from './Spinner.svelte'

  type Props = Omit<HTMLButtonAttributes, 'children'> & {
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
    size?: 'sm' | 'md' | 'lg'
    loading?: boolean
    icon?: Icon
    full?: boolean
    children?: Snippet
  }

  let {
    variant = 'secondary',
    size = 'md',
    loading = false,
    icon: IconCmp,
    full = false,
    disabled = false,
    type = 'button',
    children,
    ...rest
  }: Props = $props()
</script>

<button
  {...rest}
  {type}
  class="btn {variant} {size} {rest.class ?? ''}"
  class:full
  disabled={disabled || loading}
  aria-busy={loading || undefined}
>
  {#if loading}
    <Spinner size={size === 'sm' ? 12 : 14} />
  {:else if IconCmp}
    <IconCmp size={size === 'sm' ? 14 : 16} aria-hidden="true" />
  {/if}
  {#if children}<span class="label">{@render children()}</span>{/if}
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--sp-2);
    height: var(--h-md);
    padding: 0 var(--sp-3);
    min-width: 64px;
    border-radius: var(--r-md);
    border: 1px solid transparent;
    font-size: var(--fs-body);
    font-weight: var(--fw-medium);
    white-space: nowrap;
    cursor: pointer;
    transition:
      background-color var(--dur-fast) var(--ease),
      border-color var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
    user-select: none;
  }
  .sm {
    height: var(--h-sm);
    padding: 0 var(--sp-2);
    font-size: var(--fs-sm);
    min-width: 0;
    gap: var(--sp-1);
  }
  .lg {
    height: var(--h-lg);
    padding: 0 var(--sp-4);
    font-size: var(--fs-md);
  }
  .full {
    width: 100%;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .primary {
    background: var(--color-accent-fill);
    color: var(--color-text-on-accent);
  }
  .primary:hover:not(:disabled) {
    background: var(--color-accent-fill-hover);
  }
  .primary:active:not(:disabled) {
    background: var(--color-accent-fill-active);
  }

  .secondary {
    background: var(--color-surface-raised);
    border-color: var(--color-border);
    color: var(--color-text);
  }
  .secondary:hover:not(:disabled) {
    background: var(--color-surface);
    border-color: var(--color-border-strong);
  }
  .secondary:active:not(:disabled) {
    background: var(--color-active);
  }

  .danger {
    background: var(--color-danger-fill);
    color: var(--color-text-on-accent);
  }
  .danger:hover:not(:disabled) {
    background: var(--color-danger-fill-hover);
  }

  .ghost {
    background: transparent;
    color: var(--color-text);
  }
  .ghost:hover:not(:disabled) {
    background: var(--color-hover);
  }
  .ghost:active:not(:disabled) {
    background: var(--color-active);
  }

  .btn:disabled {
    cursor: default;
    opacity: 0.45;
  }
  .btn[aria-busy='true'] {
    opacity: 0.8;
  }
</style>
