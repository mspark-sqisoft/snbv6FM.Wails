<!-- 도는 고리. label 을 주면 화면 읽기 프로그램에 "읽는 중" 같은 이름이 읽힌다(없으면 장식). -->
<script lang="ts">
  let { size = 16, label = '' }: { size?: number; label?: string } = $props()
</script>

<span
  class="spinner"
  style:width="{size}px"
  style:height="{size}px"
  role={label ? 'status' : undefined}
  aria-label={label || undefined}
  aria-hidden={label ? undefined : 'true'}
></span>

<style>
  .spinner {
    display: inline-block;
    flex: none;
    border-radius: 50%;
    border: 2px solid currentColor;
    border-right-color: transparent;
    opacity: 0.8;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
