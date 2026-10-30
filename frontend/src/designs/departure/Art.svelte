<!--
  로그인 그림: 빈 출발 안내판에 망 목록이 항공편처럼 뜬다. 칸은 24칸 고정이고, 판 폭에 맞춰 칸 크기가 정해진다.
  행은 실제 망 목록(AuthService.Servers)이고, 나머지는 빈 판이다. 고른 망만 상태 칸에 대기·접속 중이 뜬다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { AuthService, type Server } from '../../lib/api'
  import { t } from '../../lib/i18n'
  import type { ArtProps } from '../registry'
  import Flap from './Flap.svelte'

  let { busy, network, direction }: ArtProps = $props()

  const ROWS = 6
  const EMPTY = ' '.repeat(24)

  let servers = $state<Server[]>([])
  let now = $state(new Date())
  const pad = (n: number) => String(n).padStart(2, '0')
  const clock = $derived(`${pad(now.getHours())}:${pad(now.getMinutes())}`)

  /** n 칸에 맞춘다(넘치면 자르고 모자라면 빈 칸). */
  const fit = (s: string, n: number) => {
    const c = [...s].slice(0, n)
    return [...c, ...Array(n - c.length).fill(' ')].join('')
  }

  const lines = $derived.by(() => {
    const list = servers.map((s, i) => {
      const on = s.name === network
      const route = (on && direction ? direction : s.network).replace(/\s*->\s*/, '→')
      const state = on ? t(busy ? 'design-departure.art.busy' : 'design-departure.art.ready') : ''
      return { on, text: `${pad(i + 1)} ${fit(s.name, 6)} ${fit(route, 8)} ${fit(state, 5)}` }
    })
    return Array.from({ length: Math.max(ROWS, list.length) }, (_, i) => list[i] ?? { on: false, text: EMPTY })
  })

  onMount(() => {
    AuthService.Servers()
      .then((v) => (servers = v ?? []))
      .catch(() => {}) // 못 읽으면 빈 안내판이 남는다(오류는 로그인 폼이 보인다)
    const timer = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(timer)
  })
</script>

<div class="wrap">
  <div class="board" class:busy>
    <div class="cap">
      <span class="title">{t('design-departure.art.title')}</span>
      <span class="time"><Flap text={clock} /></span>
    </div>
    <div class="cols">
      <span style:grid-column="1 / 3">{t('design-departure.art.no')}</span>
      <span style:grid-column="4 / 10">{t('design-departure.art.net')}</span>
      <span style:grid-column="11 / 19">{t('design-departure.art.route')}</span>
      <span style:grid-column="20 / 25">{t('design-departure.art.state')}</span>
    </div>
    {#each lines as l, i (i)}
      <div class="line" class:on={l.on}>
        <Flap text={l.text} delay={14} />
      </div>
    {/each}
  </div>
</div>

<style>
  .wrap {
    width: 100%;
    container-type: inline-size;
  }
  .board {
    --m: calc((100cqi - 23 * 2px) / 24);
    --mw: var(--m);
    --mh: calc(var(--m) * 1.5);
    display: flex;
    flex-direction: column;
    gap: 3px;
    color: var(--color-text);
  }
  .cap {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--sp-2);
  }
  .title {
    font-family: var(--dep-dot);
    font-size: 24px;
    font-weight: var(--fw-regular);
    line-height: 1;
    color: var(--color-accent);
    text-shadow: 0 0 2px rgba(255, 176, 0, 0.3);
  }
  .time {
    --mw: calc(var(--m) * 1.1);
    --mh: calc(var(--m) * 1.5);
    font-size: calc(var(--m) * 0.9);
    color: var(--color-accent);
  }
  .cols {
    display: grid;
    grid-template-columns: repeat(24, 1fr);
    gap: 2px;
    padding-bottom: var(--sp-1);
    border-bottom: 2px solid var(--color-accent);
    font-family: var(--dep-dot);
    font-size: var(--fs-sm);
    font-weight: var(--fw-regular);
    color: var(--color-accent);
    white-space: nowrap;
  }
  .line {
    display: flex;
    font-size: calc(var(--m) * 0.78);
    color: var(--color-text);
  }
  /* 고른 망은 폼의 고른 칸과 같은 노란 윤곽. 앰버는 숫자·입력값 색이라 망 이름에 쓰지 않는다 */
  .line.on {
    outline: 1px solid var(--color-accent-fill);
    outline-offset: 1px;
  }
</style>
