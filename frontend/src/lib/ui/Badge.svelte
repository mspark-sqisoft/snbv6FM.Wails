<!--
  작은 꼬리표. tone 으로 색(neutral·accent·success·warning·danger·info·business 업무망·internet 인터넷망),
  variant: soft(옅은 면, 기본) · outline(테두리만) · solid(진한 면). 글자는 children 또는 label.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { Icon, Tone } from './types'

  let {
    tone = 'neutral',
    variant = 'soft',
    size = 'md',
    icon: IconCmp,
    label,
    title,
    children,
  }: {
    tone?: Tone
    variant?: 'soft' | 'outline' | 'solid'
    size?: 'sm' | 'md'
    icon?: Icon
    label?: string
    title?: string
    children?: Snippet
  } = $props()
</script>

<span class="badge tone-{tone} {variant} {size}" {title}>
  {#if IconCmp}<IconCmp size={size === 'sm' ? 11 : 13} aria-hidden="true" />{/if}
  {#if label}{label}{:else}{@render children?.()}{/if}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-1);
    height: 22px;
    padding: 0 var(--sp-2);
    border: 1px solid transparent;
    border-radius: var(--r-full);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    line-height: 1;
    white-space: nowrap;
    color: var(--tone-fg);
  }
  .sm {
    height: 18px;
    padding: 0 6px;
    font-size: var(--fs-caption);
  }
  .soft {
    background: var(--tone-bg);
    border-color: var(--tone-border);
  }
  .outline {
    border-color: var(--tone-border);
  }
  .solid {
    background: var(--tone-fg);
    color: var(--color-bg);
  }
  .tone-neutral {
    --tone-fg: var(--color-neutral);
    --tone-bg: var(--color-neutral-subtle);
    --tone-border: var(--color-neutral-border);
  }
  .tone-accent,
  .tone-info {
    --tone-fg: var(--color-info);
    --tone-bg: var(--color-info-subtle);
    --tone-border: var(--color-info-border);
  }
  .tone-success {
    --tone-fg: var(--color-success);
    --tone-bg: var(--color-success-subtle);
    --tone-border: var(--color-success-border);
  }
  .tone-warning {
    --tone-fg: var(--color-warning);
    --tone-bg: var(--color-warning-subtle);
    --tone-border: var(--color-warning-border);
  }
  .tone-danger {
    --tone-fg: var(--color-danger);
    --tone-bg: var(--color-danger-subtle);
    --tone-border: var(--color-danger-border);
  }
  .tone-business {
    --tone-fg: var(--color-net-business);
    --tone-bg: var(--color-net-business-subtle);
    --tone-border: var(--color-net-business-border);
  }
  .tone-internet {
    --tone-fg: var(--color-net-internet);
    --tone-bg: var(--color-net-internet-subtle);
    --tone-border: var(--color-net-internet-border);
  }
</style>
