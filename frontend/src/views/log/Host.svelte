<!--
  로그 영역의 상시 부품(화면에 그리는 것은 없다). 메인 틀이 로그인 뒤 한 번 올려 둔다.
  GUI 멈춤 감시(LOG-044): SNB_FREEZE_WATCH=1 로 띄웠으면 200ms 시계가 직전 틱부터 500ms 넘게 늦게 불릴 때
  그 간격을 Go 로 보내 앱 로그에 `GUI 멈춤 >>> N ms`(snb.app WARN)로 남긴다. 원본의 GUI 스레드 감시를 WebView 의 JS 스레드로 옮긴 것.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { LogService } from '../../lib/api'

  const TICK = 200
  const LIMIT = 500

  onMount(() => {
    let timer: ReturnType<typeof setInterval> | undefined
    let stopped = false
    LogService.FreezeWatchEnabled()
      .then((on) => {
        if (!on || stopped) return
        let last = performance.now()
        timer = setInterval(() => {
          const now = performance.now()
          const gap = now - last
          last = now
          // 창이 가려져 브라우저가 시계를 늦춘 것은 멈춤이 아니다
          if (gap > LIMIT && document.visibilityState === 'visible') LogService.Freeze(Math.round(gap)).catch(() => {})
        }, TICK)
      })
      .catch(() => {})
    return () => {
      stopped = true
      clearInterval(timer)
    }
  })
</script>
