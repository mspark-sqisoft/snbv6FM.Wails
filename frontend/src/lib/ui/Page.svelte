<!--
  화면 틀: 위에 제목 줄(제목·부제·오른쪽 단추 actions), 그 아래 toolbar(검색·필터 줄, 선택), 나머지는 본문.
  본문은 남은 높이를 다 쓴다 ― 표(DataTable)를 넣으면 표만 스크롤된다. padded 가 거짓이면 본문 여백 없이 가장자리까지.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    title,
    subtitle,
    actions,
    toolbar,
    children,
    padded = true,
  }: {
    title: string
    subtitle?: string
    actions?: Snippet
    toolbar?: Snippet
    children?: Snippet
    padded?: boolean
  } = $props()
</script>

<section class="page" aria-label={title}>
  <header>
    <div class="titles">
      <h1>{title}</h1>
      {#if subtitle}<p>{subtitle}</p>{/if}
    </div>
    {#if actions}<div class="actions">{@render actions()}</div>{/if}
  </header>
  {#if toolbar}<div class="toolbar">{@render toolbar()}</div>{/if}
  <div class="content" class:padded>{@render children?.()}</div>
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    min-width: 0;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-3);
    min-height: 52px;
    padding: var(--sp-2) var(--sp-5);
  }
  .titles {
    display: flex;
    align-items: baseline;
    gap: var(--sp-3);
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: var(--fs-xl);
    font-weight: var(--fw-bold);
    line-height: var(--lh-tight);
    white-space: nowrap;
  }
  p {
    margin: 0;
    color: var(--color-text-3);
    font-size: var(--fs-body);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .actions {
    margin-left: auto;
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    align-items: center;
    gap: var(--sp-2);
    max-width: 100%;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 0 var(--sp-5) var(--sp-2);
  }
  .content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .content.padded {
    padding: 0 var(--sp-5) var(--sp-4);
  }
</style>
