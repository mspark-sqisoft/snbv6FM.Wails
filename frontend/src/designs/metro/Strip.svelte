<!--
  노선도 띠(docs/DESIGNS.md): 객차 문 위의 노선 안내처럼 올리기 → 검사 → 결재(환승) → 전송 → 받기 를 한 줄로 그린다.
  - 왼쪽 원은 지금 망의 노선 표지다. 여기서 내려가는 선이 왼쪽 메뉴의 세로 노선으로 이어진다.
  - 앱이 실제로 아는 것은 이 PC 의 올리기·받기뿐이다. 검사·결재·전송은 서버에서 일어나므로 역만 그린다.
  - 올리기는 지금 망 구간에 건수와 진행률을 싣고 열차를 움직인다.
  - 받기는 이 PC 로 들어오는 파일이다. 환승역 너머는 반대편 망 색이라 거기에 그리면 저쪽으로 달리는 것처럼 보이므로,
    노선 밖에 지금 망 색 역 하나로 적는다.
  - 오른쪽 두 칸은 받기·실패 글자의 자리를 늘 잡아 두어 글자가 나타나도 노선의 역이 움직이지 않는다.
  - html 에 망(data-metro-net)과 역명판 표지(--metro-bullet)를 얹는다. design.css 가 읽는다.
-->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import { session } from '../../lib/stores/session'
  import { items, live, summary, type Item } from '../../views/transfer/store'

  const STOPS = ['upload', 'scan', 'approve', 'send', 'receive']

  /** store.ts summary 와 같은 셈을 올리기·받기로 나눈다. */
  function leg(list: Item[], dir: string) {
    let n = 0
    let done = 0
    let total = 0
    for (const it of list) {
      if (it.dir !== dir || !live(it)) continue
      const size = it.total || it.size
      n++
      total += size
      done += it.phase === 'send' || it.phase === 'recv' || it.phase === 'commit' ? Math.min(it.done, size) : 0
    }
    return { n, pct: total > 0 ? Math.floor((done / total) * 100) : 0 }
  }

  const up = $derived(leg($items, 'upload'))
  const down = $derived(leg($items, 'download'))
  const netName = $derived(t(`common.net.${$session.isBusiness ? 'business' : 'internet'}`))
  const otherName = $derived(t(`common.net.${$session.isBusiness ? 'internet' : 'business'}`))
  const toward = $derived($session.direction.split('->')[1]?.trim() || otherName)

  const clock = () => new Date().toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false })
  let now = $state(clock())
  $effect(() => {
    const timer = setInterval(() => (now = clock()), 5000)
    return () => clearInterval(timer)
  })

  $effect(() => {
    const root = document.documentElement
    root.dataset.metroNet = $session.isBusiness ? 'business' : 'internet'
    root.style.setProperty('--metro-bullet', JSON.stringify(netName.slice(0, 1)))
    return () => {
      delete root.dataset.metroNet
      root.style.removeProperty('--metro-bullet')
    }
  })
</script>

<div class="strip" role="group" aria-label={t('design-metro.strip.label')}>
  <span class="bullet" aria-hidden="true">{netName.slice(0, 1)}</span>
  <div class="route">
    <ol>
      {#each STOPS as s, i (s)}
        {@const busy = s === 'upload' && !!up.n}
        <li class:hub={s === 'approve'} class:busy style:left="{i * 25}%">
          <span class="stop" aria-hidden="true"></span>
          <span class="name">
            {t(`design-metro.stop.${s}`)}{#if busy}<b>{up.n} · {up.pct}%</b>{/if}
          </span>
        </li>
      {/each}
    </ol>
    {#if up.n}
      <span class="leg" aria-hidden="true"><i style:transform="translateX({up.pct}%)"><u></u></i></span>
    {/if}
  </div>
  <div class="slot near">
    <span class="toward">{t('design-metro.toward', { name: toward })}</span>
    <span class="incoming" class:idle={!down.n}>
      <i aria-hidden="true"></i>{t('design-metro.stop.receive')}<b>{down.n} · {down.pct}%</b>
    </span>
  </div>
  <div class="slot far">
    <time class="clock">{now}</time>
    <span class="failed" class:idle={!$summary.failed}>{t('transfer.state.failed')} {$summary.failed}</span>
  </div>
</div>

<style>
  .strip {
    position: relative;
    display: flex;
    align-items: center;
    flex: none;
    gap: var(--sp-3);
    height: 46px;
    min-width: 0;
    padding: 0 var(--sp-4) 0 14px;
    background: var(--color-bg);
    border-bottom: 2px solid var(--color-text);
  }
  /* 왼쪽 메뉴의 세로 노선으로 내려가는 선 */
  .strip::after {
    content: '';
    position: absolute;
    left: 23px;
    top: 16px;
    bottom: -2px;
    width: var(--metro-w);
    background: var(--metro-line);
  }
  .bullet {
    position: absolute;
    left: 14px;
    top: 4px;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--metro-line);
    color: var(--metro-on-line);
    font-size: var(--fs-sm);
    font-weight: 800;
    line-height: 1;
  }
  .route {
    position: relative;
    flex: 1;
    align-self: stretch;
    min-width: 0;
    margin: 0 var(--sp-5) 0 44px;
    container-type: inline-size;
  }
  /* 지나온 쪽은 지금 망, 결재(환승역) 너머는 반대편 망의 색 */
  .route::before,
  .route::after {
    content: '';
    position: absolute;
    top: 13px;
    height: var(--metro-w);
  }
  .route::before {
    left: -30px;
    right: 50%;
    background: var(--metro-line);
  }
  .route::after {
    left: 50%;
    right: 0;
    background: var(--metro-cross);
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    position: absolute;
    top: 0;
    z-index: 1;
    width: 0;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .stop {
    flex: none;
    box-sizing: border-box;
    width: 14px;
    height: 14px;
    margin-top: 9px;
    border: 3.5px solid var(--color-text);
    border-radius: 50%;
    background: var(--color-bg);
  }
  .hub .stop {
    width: 22px;
    height: 22px;
    margin-top: 5px;
    border-width: 5px;
  }
  .busy .stop {
    background: var(--color-text);
  }
  .name {
    display: flex;
    align-items: baseline;
    gap: var(--sp-1);
    margin-top: 2px;
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    line-height: 1.2;
    white-space: nowrap;
  }
  .hub .name {
    position: absolute;
    top: 28px;
    margin-top: 0;
    color: var(--color-text);
    font-weight: 800;
  }
  /* 양 끝 역의 이름은 안쪽으로 붙여 표지 원과 겹치지 않게 */
  li:first-child .name,
  li:last-child .name {
    position: absolute;
    top: 25px;
    margin-top: 0;
  }
  li:first-child .name {
    left: -7px;
  }
  li:last-child .name {
    right: -7px;
  }
  .name b,
  .incoming b {
    font-family: var(--font-mono);
    font-size: var(--fs-caption);
    font-weight: var(--fw-medium);
    color: var(--color-text);
  }
  /* 건수·진행률은 실제 값이라 남긴다. 자리가 모자라면 고정 글자인 중간 역 이름을 읽기 프로그램에만 남긴다 */
  @container (max-width: 480px) {
    ol:has(.busy) li:not(.hub, :first-child, :last-child) .name {
      position: absolute;
      width: 1px;
      height: 1px;
      margin: -1px;
      overflow: hidden;
      clip: rect(0 0 0 0);
    }
  }
  /* 열차 중심이 다니는 구간. 양쪽 역을 가리지 않게 역 반지름 7px + 열차 절반 14px + 여유 3px 만큼 안쪽이다 */
  .leg {
    position: absolute;
    z-index: 1;
    top: 10px;
    left: 24px;
    width: calc(25% - 48px);
    height: 12px;
    pointer-events: none;
  }
  .leg i {
    display: block;
    width: 100%;
    transition: transform var(--dur-slow) linear;
  }
  /* 열차: 창 세 칸이 있는 먹색 차체 */
  .leg u {
    display: block;
    width: 28px;
    height: 12px;
    margin-left: -14px;
    border-radius: 6px;
    background:
      radial-gradient(circle, var(--color-bg) 0 1.8px, transparent 2.2px) 3px 0 / 7px 100% repeat-x,
      var(--color-text);
    background-clip: content-box, border-box;
    padding: 0 3px;
  }
  /* 첫 줄은 노선 높이, 둘째 줄은 역 이름 높이에 맞춘다 */
  .slot {
    flex: none;
    align-self: stretch;
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding-top: 8px;
    line-height: 16px;
    white-space: nowrap;
  }
  .near {
    max-width: 150px;
  }
  .far {
    align-items: flex-end;
  }
  /* 값이 없어도 글자 폭만큼 자리를 잡아 둔다. 언어가 바뀌어도 폭을 다시 정할 일이 없다 */
  .idle {
    visibility: hidden;
  }
  .toward {
    color: var(--color-text);
    font-size: var(--fs-sm);
    font-weight: 800;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .incoming {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
  }
  /* 건수·진행률이 늘어도 폭이 변하지 않게 '99 · 100%' 자리를 잡는다 */
  .incoming b {
    min-width: 9ch;
  }
  .incoming i {
    flex: none;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--metro-line);
  }
  .failed {
    color: var(--color-danger);
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
  }
  .clock {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    font-weight: var(--fw-medium);
    color: var(--color-text);
  }
  @media (prefers-reduced-motion: reduce) {
    .leg i {
      transition: none;
    }
  }
</style>
