<!--
  머리줄 아래의 출발 안내 띠: 출발 망 → 도착 망 · 지금 건너가는 파일 · 탑승 현황 건수 · 역할 · 실제 시각.
  값은 전부 세션과 전송 대기열에서 읽는다. 표의 상태 글자가 바뀌면 그 칸을 뒤집는다(감시자 하나로 모은다 ― 행마다 두지 않는다).
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import PlaneTakeoff from '@lucide/svelte/icons/plane-takeoff'
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import { currentLocale, t } from '../../lib/i18n'
  import { session } from '../../lib/stores/session'
  import { items, live, pctOf, statusText, summary, type Item } from '../../views/transfer/store'
  import Flap from './Flap.svelte'

  const pad = (n: number) => String(n).padStart(2, '0')

  let now = $state(new Date())
  const clock = $derived(`${pad(now.getHours())}:${pad(now.getMinutes())}`)
  const date = $derived(
    `${pad(now.getMonth() + 1)}.${pad(now.getDate())} ${now.toLocaleDateString(currentLocale(), { weekday: 'short' })}`,
  )

  const dir = $derived($session.direction.split('->').map((s) => s.trim()))
  const from = $derived(dir[0] || $session.networkName)
  const to = $derived(dir[1] ?? '')

  const running = $derived($summary.active + $summary.pending)
  const cur = $derived($items.find((i) => i.state === 'active') ?? $items.find(live))
  // 실제로 보내거나 받는 중일 때만 숫자가 따로 붙는다. 압축·해제는 문구 안에 들어 있다
  const pct = $derived(cur?.state === 'active' && (cur.phase === 'send' || cur.phase === 'recv') && cur.done > 0 ? pctOf(cur) : null)

  // 서랍의 상태 글자와 같다. 전송 중 문구만 서랍은 숫자, 띠는 방향을 적는다
  const phaseText = (it: Item) => statusText(it, t(`design-departure.${it.dir === 'upload' ? 'up' : 'down'}`))

  const counts = $derived([
    { id: 'active', n: $summary.active },
    { id: 'pending', n: $summary.pending },
    { id: 'done', n: $summary.done },
    { id: 'failed', n: $summary.failed },
  ])

  // 상태 글자가 바뀐 칸을 뒤집는다. 새로 그려지는 행(스크롤)은 글자 변경이 아니어서 걸리지 않는다.
  function watchStatus(root: Element) {
    const still = matchMedia('(prefers-reduced-motion: reduce)')
    const mo = new MutationObserver((list) => {
      if (still.matches) return
      for (const m of list) {
        const chip = m.target.parentElement?.closest('.chip')
        if (!chip) continue
        ;(chip.closest('.cell') ?? chip).animate(
          [
            { transform: 'perspective(240px) rotateX(-90deg)' },
            { transform: 'perspective(240px) rotateX(10deg)', offset: 0.7 },
            { transform: 'perspective(240px) rotateX(0)' },
          ],
          { duration: 320, easing: 'ease-out' },
        )
      }
    })
    mo.observe(root, { subtree: true, characterData: true })
    return () => mo.disconnect()
  }

  onMount(() => {
    const timer = setInterval(() => (now = new Date()), 1000)
    const shell = document.querySelector('.shell')
    const stop = shell ? watchStatus(shell) : undefined
    return () => {
      clearInterval(timer)
      stop?.()
    }
  })
</script>

<section class="strip" aria-label={t('design-departure.strip')}>
  <div class="row">
    <div class="route">
      <span class="pic" aria-hidden="true"><PlaneTakeoff size={18} strokeWidth={2.2} /></span>
      <span class="lab">{t('design-departure.from')}</span>
      <span class="net" class:biz={$session.isBusiness}><Flap text={from} delay={24} /></span>
      <ArrowRight size={14} aria-hidden="true" />
      {#if to}
        <span class="lab">{t('design-departure.to')}</span>
        <span class="net" class:biz={!$session.isBusiness}><Flap text={to} delay={24} /></span>
      {/if}
    </div>

    <div class="now" class:on={!!cur}>
      {#if cur}
        <span class="lamp" class:wait={cur.state === 'pending'} aria-hidden="true"></span>
        {#if cur.dir === 'upload'}<ArrowUp size={14} aria-hidden="true" />{:else}<ArrowDown size={14} aria-hidden="true" />{/if}
        <span class="what" class:plain={pct !== null}>{phaseText(cur)}</span>
        <span class="file" title={cur.name}>{cur.name}</span>
        {#if pct !== null}<span class="pct">{pct}%</span>{/if}
        {#if running > 1}<span class="more">{t('design-departure.more', { n: running - 1 })}</span>{/if}
      {:else}
        <span class="idle">{t('design-departure.idle')}</span>
      {/if}
    </div>

    <dl class="counts">
      {#each counts as c (c.id)}
        <div class="c {c.id}" class:hit={c.n > 0}>
          <dt>{t(`design-departure.${c.id}`)}</dt>
          <dd><Flap text={String(c.n).padStart(2, '0')} /></dd>
        </div>
      {/each}
    </dl>

    <div class="role">
      <span class="lab">{t('design-departure.role')}</span>
      <span class="val">{t(`common.role.${$session.role}`)}</span>
    </div>

    <div class="clock">
      <span class="date">{date}</span>
      <time><Flap text={clock} /></time>
    </div>
  </div>
</section>

<style>
  .strip {
    flex: none;
    height: 44px;
    padding: 0 var(--sp-3) 0 var(--sp-5);
    overflow: hidden;
    container-type: inline-size;
    background: var(--dep-grid), var(--color-bg);
    background-size: var(--dep-grid-size);
    border-top: 1px solid var(--color-border-subtle);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--sp-4);
    height: 100%;
    min-width: 0;
    font-size: 15px;
    color: var(--color-accent);
  }
  .route,
  .role,
  .clock {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--sp-2);
  }
  /* 망 이름이 길면 이쪽이 잘리고 시계는 밀리지 않는다 */
  .route {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
  }
  .route :global(svg),
  .route .pic,
  .route .lab {
    flex: none;
  }
  /* 망 이름은 머리줄 배지와 같은 망 색이다. 앰버는 표의 숫자·입력값 색이라 섞이면 구분이 사라진다 */
  .net {
    display: inline-flex;
    min-width: 0;
    color: var(--color-net-internet);
  }
  .net.biz {
    color: var(--color-net-business);
  }
  .pic {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: var(--color-accent-fill);
    color: var(--color-text-on-accent);
  }
  .lab,
  dt,
  .date,
  .what {
    font-family: var(--dep-dot);
    font-size: var(--fs-sm);
    line-height: 1;
    color: var(--color-text-3);
    white-space: nowrap;
  }
  .role .val {
    font-size: var(--fs-body);
    font-weight: var(--fw-semibold);
    color: var(--color-text);
    white-space: nowrap;
  }
  .now {
    flex: 1 1 0;
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    min-width: 0;
    height: 28px;
    padding: 0 var(--sp-3);
    overflow: hidden;
    background: var(--color-surface-sunken);
    border: 1px solid var(--color-border-subtle);
    color: var(--color-text-3);
  }
  .now.on {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }
  .now :global(svg) {
    flex: none;
  }
  /* 자리가 모자라면 파일 이름이 먼저 준다. 단계 문구가 띠의 본론이라 %까지 다 보여야 한다. 44px 는 앞의 등·화살표 몫 */
  .now .what {
    flex: none;
    max-width: calc(100% - 44px);
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 1.2;
  }
  .now.on .what {
    color: var(--color-accent);
  }
  .idle {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    font-family: var(--dep-dot);
    font-size: var(--fs-sm);
    white-space: nowrap;
  }
  /* 색은 표·서랍의 상태와 같다: 진행 앰버, 대기 안내색 */
  .lamp {
    --lamp: var(--color-accent);
    flex: none;
    width: 6px;
    height: 12px;
    background: var(--lamp);
    box-shadow: 0 0 6px var(--lamp);
    animation: dep-lamp 1.4s steps(2, jump-none) infinite;
  }
  .lamp.wait {
    --lamp: var(--color-info);
    animation: none;
  }
  .file {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--dep-table);
    font-size: var(--fs-body);
    color: var(--color-text);
  }
  .pct,
  .more {
    flex: none;
    font-family: var(--dep-table);
    font-size: var(--fs-body);
    white-space: nowrap;
  }
  .more {
    color: var(--color-text-3);
  }
  .counts {
    display: flex;
    flex: none;
    gap: var(--sp-3);
    margin: 0;
  }
  .c {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    color: var(--color-text-3);
  }
  .c dd {
    margin: 0;
    display: flex;
    font-size: var(--fs-lg);
  }
  .c.active.hit {
    color: var(--color-accent);
  }
  .c.pending.hit {
    color: var(--color-info);
  }
  .c.done.hit {
    color: var(--color-success);
  }
  .c.failed.hit {
    color: var(--color-danger);
  }
  .clock {
    font-size: 17px;
    color: var(--color-accent);
  }
  .clock time {
    display: flex;
  }
  @keyframes dep-lamp {
    from {
      opacity: 1;
    }
    to {
      opacity: 0.35;
    }
  }
  /* 좁은 창: 이름표를 먼저 줄이고 숫자는 남긴다. 줄인 것도 화면 읽기 프로그램에는 남는다 */
  @container (max-width: 860px) {
    .row {
      gap: var(--sp-3);
    }
    .route .lab,
    .role .lab,
    .date,
    .now .what.plain,
    .more,
    .c dt:lang(en) {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
    .route {
      gap: var(--sp-1);
    }
  }
  /* 단계 문구 옆에 글자 한둘만 남는 파일 이름은 뺀다(전체 이름은 서랍에 있다). 보내고 받는 중에는 문구가 빠져 이름이 남는다 */
  @container (max-width: 780px) {
    .now .what:not(.plain) + .file {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
  }
  @container (max-width: 700px) {
    .idle,
    .role {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .lamp {
      animation: none;
    }
  }
</style>
