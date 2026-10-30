<!-- 앱 표지: 테마 색을 따르는 그림 + 글자(원본 MAIN-040 의 그림/글자 상충을 SVG 하나로). -->
<script lang="ts">
  let { size = 22, word = true }: { size?: number; word?: boolean } = $props()
</script>

<span class="logo" style:--size="{size}px">
  <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
    <rect x="1" y="1" width="22" height="22" rx="6" class="bg" />
    <path d="M6.5 9.5h9.5l-2.6-2.6M17.5 14.5H8l2.6 2.6" class="fg" />
  </svg>
  {#if word}<span class="word"><b>snB</b> App</span>{/if}
</span>

<style>
  .logo {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    user-select: none;
  }
  .bg {
    fill: var(--color-accent-fill);
  }
  .fg {
    fill: none;
    stroke: var(--color-text-on-accent);
    stroke-width: 1.9;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .word {
    font-size: calc(var(--size) * 0.72);
    font-weight: var(--fw-medium);
    letter-spacing: -0.01em;
    color: var(--color-text-2);
    white-space: nowrap;
  }
  b {
    font-weight: var(--fw-bold);
    color: var(--color-text);
  }
</style>
