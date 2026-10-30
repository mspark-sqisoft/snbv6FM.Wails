<!-- 빈 상태: 아이콘 + 제목 + 설명 + (children 에) 다음 할 일 단추. compact 는 표 안처럼 좁은 자리용. -->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { Icon } from './types'

  let {
    icon: IconCmp,
    title,
    description,
    compact = false,
    children,
  }: {
    icon?: Icon
    title: string
    description?: string
    compact?: boolean
    children?: Snippet
  } = $props()
</script>

<div class="empty" class:compact>
  {#if IconCmp}
    <span class="icon"><IconCmp size={compact ? 22 : 32} strokeWidth={1.5} aria-hidden="true" /></span>
  {/if}
  <p class="title">{title}</p>
  {#if description}<p class="desc">{description}</p>{/if}
  {#if children}<div class="actions">{@render children()}</div>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--sp-2);
    padding: var(--sp-8) var(--sp-4);
    text-align: center;
    color: var(--color-text-2);
    height: 100%;
  }
  .compact {
    padding: var(--sp-5) var(--sp-3);
    gap: var(--sp-1);
  }
  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 56px;
    height: 56px;
    margin-bottom: var(--sp-1);
    border-radius: 50%;
    background: var(--color-surface);
    color: var(--color-text-3);
  }
  .compact .icon {
    width: 40px;
    height: 40px;
  }
  p {
    margin: 0;
    max-width: 420px;
  }
  .title {
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    color: var(--color-text);
  }
  .compact .title {
    font-size: var(--fs-body);
  }
  .desc {
    font-size: var(--fs-body);
    color: var(--color-text-3);
  }
  .actions {
    display: flex;
    gap: var(--sp-2);
    margin-top: var(--sp-2);
  }
</style>
