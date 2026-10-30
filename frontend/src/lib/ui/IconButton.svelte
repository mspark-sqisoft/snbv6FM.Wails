<!--
  아이콘만 있는 단추. label 은 필수다 ― 툴팁과 접근성 이름(aria-label)에 함께 쓴다.
  pressed 를 주면 켜고 끄는 단추(aria-pressed)가 된다.
-->
<script lang="ts">
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import type { Icon } from './types'
  import Tooltip from './Tooltip.svelte'
  import Spinner from './Spinner.svelte'

  type Props = Omit<HTMLButtonAttributes, 'children'> & {
    icon: Icon
    label: string
    variant?: 'ghost' | 'secondary' | 'primary'
    size?: 'sm' | 'md' | 'lg'
    pressed?: boolean
    loading?: boolean
    /** 삭제·모두 취소 같은 위험 동작. 기본 생김새는 같고 디자인이 표시를 얹는다. */
    danger?: boolean
    tooltipPlacement?: 'top' | 'bottom' | 'left' | 'right'
  }

  let {
    icon: IconCmp,
    label,
    variant = 'ghost',
    size = 'md',
    pressed,
    loading = false,
    danger = false,
    tooltipPlacement = 'bottom',
    disabled = false,
    type = 'button',
    ...rest
  }: Props = $props()

  const iconPx = { sm: 14, md: 16, lg: 20 }
</script>

<Tooltip text={label} placement={tooltipPlacement}>
  <button
    {...rest}
    {type}
    class="ib {variant} {size}"
    class:danger
    aria-label={label}
    aria-pressed={pressed}
    aria-busy={loading || undefined}
    disabled={disabled || loading}
  >
    {#if loading}
      <Spinner size={iconPx[size] - 2} />
    {:else}
      <IconCmp size={iconPx[size]} aria-hidden="true" />
    {/if}
  </button>
</Tooltip>

<style>
  .ib {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: var(--h-md);
    height: var(--h-md);
    padding: 0;
    border-radius: var(--r-md);
    border: 1px solid transparent;
    background: transparent;
    color: var(--color-text-2);
    cursor: pointer;
    transition:
      background-color var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
  }
  .sm {
    width: var(--h-sm);
    height: var(--h-sm);
    border-radius: var(--r-sm);
  }
  .lg {
    width: var(--h-lg);
    height: var(--h-lg);
  }
  .ib:hover:not(:disabled) {
    background: var(--color-hover);
    color: var(--color-text);
  }
  .ib:active:not(:disabled) {
    background: var(--color-active);
  }
  .ib[aria-pressed='true'] {
    background: var(--color-accent-subtle);
    color: var(--color-accent);
  }
  .secondary {
    border-color: var(--color-border);
    background: var(--color-surface-raised);
  }
  .primary {
    background: var(--color-accent-fill);
    color: var(--color-text-on-accent);
  }
  .primary:hover:not(:disabled) {
    background: var(--color-accent-fill-hover);
    color: var(--color-text-on-accent);
  }
  .ib:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
