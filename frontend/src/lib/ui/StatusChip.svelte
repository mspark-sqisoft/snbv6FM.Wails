<!--
  표 칸에 쓰는 상태 칩: 색 점 + 글자. 면 없이 가볍게 그려 행이 많아도 표가 시끄럽지 않다.
  tone 은 Badge 와 같다. 글자 색은 본문 색, 점만 상태 색이라 색을 못 보는 사람도 글자로 읽는다.
-->
<script lang="ts">
  import type { Tone } from './types'

  let { tone = 'neutral', label, title }: { tone?: Tone; label: string; title?: string } = $props()
</script>

<span class="chip tone-{tone}" title={title ?? label}><span class="dot" aria-hidden="true"></span><span class="text">{label}</span></span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: var(--fs-body);
    color: var(--color-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* inline-flex 의 맨 글자에는 말줄임표가 듣지 않아 글자를 따로 감싼다 */
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--dot);
    box-shadow: 0 0 0 2px var(--halo);
  }
  .tone-neutral {
    --dot: var(--color-text-3);
    --halo: var(--color-neutral-subtle);
  }
  .tone-accent,
  .tone-info {
    --dot: var(--color-info);
    --halo: var(--color-info-subtle);
  }
  .tone-success {
    --dot: var(--color-success);
    --halo: var(--color-success-subtle);
  }
  .tone-warning {
    --dot: var(--color-warning);
    --halo: var(--color-warning-subtle);
  }
  .tone-danger {
    --dot: var(--color-danger);
    --halo: var(--color-danger-subtle);
  }
  .tone-business {
    --dot: var(--color-net-business);
    --halo: var(--color-net-business-subtle);
  }
  .tone-internet {
    --dot: var(--color-net-internet);
    --halo: var(--color-net-internet-subtle);
  }
</style>
