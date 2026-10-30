<!--
  진행 막대(role="progressbar"). value/max 로 비율, indeterminate 면 오가는 막대(얼마 남았는지 모를 때).
  label 은 접근성 이름(예: 파일 이름), showValue 면 오른쪽에 백분율 글자.
-->
<script lang="ts">
  let {
    value = 0,
    max = 100,
    indeterminate = false,
    label,
    tone = 'accent',
    size = 'md',
    showValue = false,
  }: {
    value?: number
    max?: number
    indeterminate?: boolean
    label?: string
    tone?: 'accent' | 'success' | 'warning' | 'danger'
    size?: 'sm' | 'md'
    showValue?: boolean
  } = $props()

  const pct = $derived(max > 0 ? Math.max(0, Math.min(100, (value / max) * 100)) : 0)
</script>

<div class="wrap">
  <div
    class="bar {tone} {size}"
    class:indeterminate
    role="progressbar"
    aria-label={label}
    aria-valuemin={0}
    aria-valuemax={indeterminate ? undefined : 100}
    aria-valuenow={indeterminate ? undefined : Math.round(pct)}
  >
    <div class="fill" style:width={indeterminate ? undefined : `${pct}%`}></div>
  </div>
  {#if showValue && !indeterminate}<span class="val">{Math.floor(pct)}%</span>{/if}
</div>

<style>
  .wrap {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    min-width: 0;
    flex: 1;
  }
  .bar {
    position: relative;
    flex: 1;
    height: 6px;
    border-radius: var(--r-full);
    background: var(--color-border-subtle);
    overflow: hidden;
  }
  .sm {
    height: 3px;
  }
  .fill {
    height: 100%;
    border-radius: inherit;
    background: var(--fill);
    transition: width var(--dur) var(--ease);
  }
  .accent {
    --fill: var(--color-accent-fill);
  }
  .success {
    --fill: var(--color-success);
  }
  .warning {
    --fill: var(--color-warning);
  }
  .danger {
    --fill: var(--color-danger);
  }
  .indeterminate .fill {
    position: absolute;
    width: 35%;
    animation: slide 1.1s var(--ease) infinite;
  }
  .val {
    flex: none;
    min-width: 34px;
    text-align: right;
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  @keyframes slide {
    from {
      left: -35%;
    }
    to {
      left: 100%;
    }
  }
</style>
