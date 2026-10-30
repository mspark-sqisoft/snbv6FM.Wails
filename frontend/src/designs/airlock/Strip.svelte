<!--
  에어록 띠: 왼쪽 업무망 · 잠금문 두 짝 · 오른쪽 인터넷망. 양쪽 문은 동시에 열리지 않는다(인터록).
  전송이 실제로 돌 때만 문이 번갈아 열리고 화물이 지나간다. 올리기는 내 망에서 반대편으로, 받기는 반대편에서 내 망으로 흐른다.
  data-net 은 design.css 가 읽어 강조색과 벽의 밝기를 고른다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { t } from '../../lib/i18n'
  import { session } from '../../lib/stores/session'
  import { items, summary } from '../../views/transfer/store'

  let now = $state(new Date())
  onMount(() => {
    const timer = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(timer)
  })

  const biz = $derived($session.isBusiness)
  const parts = $derived($session.direction.split('->').map((s) => s.trim()))
  const named = $derived(parts.length === 2)
  const left = $derived(named ? parts[biz ? 0 : 1] : t('common.net.business'))
  const right = $derived(named ? parts[biz ? 1 : 0] : t('common.net.internet'))

  const up = $derived($items.some((i) => i.state === 'active' && i.dir === 'upload'))
  const down = $derived($items.some((i) => i.state === 'active' && i.dir === 'download'))
  const ltr = $derived(biz ? up : down)
  const rtl = $derived(biz ? down : up)
  const flowing = $derived(up || down)
  const idle = $derived(!flowing && !$summary.pending)
  const pct = $derived($summary.total > 0 ? Math.floor(($summary.bytes / $summary.total) * 100) : 0)
</script>

{#snippet packet(cls: string, y: number)}
  <g class="packet {cls}">
    <rect x="136" {y} width="6" height="6" />
    <rect x="147" {y} width="6" height="6" />
    <rect x="158" {y} width="6" height="6" />
  </g>
{/snippet}

<div class="al-strip" data-net={biz ? 'business' : 'internet'} class:flowing class:ltr class:rtl>
  <div class="wing a">
    <span class="role">{t(`common.role.${$session.role}`)}</span>
    <span class="plate biz" class:here={biz}>
      {#if biz}<small>{t('design-airlock.here')}</small>{/if}<b>{left}</b>
    </span>
    <span class="duct" aria-hidden="true"><i></i></span>
  </div>

  <div class="lock">
  <svg viewBox="0 0 300 44" aria-hidden="true">
    <defs>
      <pattern id="al-hazard" width="8" height="8" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
        <rect width="8" height="8" class="haz-b" />
        <rect width="4" height="8" class="haz-a" />
      </pattern>
    </defs>
    <rect class="room" x="80" y="12" width="140" height="20" />
    <path class="wall" d="M0 11.5H300M0 32.5H300" />
    <g class="bands">
      <rect x="80" y="12" width="140" height="3" />
      <rect x="80" y="29" width="140" height="3" />
    </g>
    {#if ltr}{@render packet('ltr', rtl ? 15 : 19)}{/if}
    {#if rtl}{@render packet('rtl', ltr ? 23 : 19)}{/if}
    <g class="door a">
      <path class="leaf up" d="M66 12H76V20L71 23L66 20Z" />
      <path class="leaf down" d="M66 21L71 24L76 21V32H66Z" />
    </g>
    <g class="door b">
      <path class="leaf up" d="M224 12H234V20L229 23L224 20Z" />
      <path class="leaf down" d="M224 21L229 24L234 21V32H224Z" />
    </g>
    <path
      class="jamb"
      d="M62.5 .5H80.5V11.5H62.5ZM62.5 32.5H80.5V43.5H62.5ZM219.5 .5H237.5V11.5H219.5ZM219.5 32.5H237.5V43.5H219.5Z"
    />
    <path class="post" d="M62.5 11.5V32.5M80.5 11.5V32.5M219.5 11.5V32.5M237.5 11.5V32.5" />
    <rect class="lamp" x="66" y="4" width="10" height="4" />
    <rect class="lamp" x="224" y="4" width="10" height="4" />
  </svg>
  {#if idle}<span class="sealed">{t('design-airlock.sealed')}</span>{/if}
  </div>

  <div class="wing b">
    <span class="duct" aria-hidden="true"><i></i></span>
    <span class="plate inet" class:here={!biz}>
      <b>{right}</b>{#if !biz}<small>{t('design-airlock.here')}</small>{/if}
    </span>
    <span class="state">
      {#if $summary.active}<span>{t('design-airlock.passing', { n: $summary.active })}</span>{/if}
      {#if $summary.pending}<span>{t('design-airlock.queued', { n: $summary.pending })}</span>{/if}
      {#if $summary.total}<span class="pct">{pct}%</span>{/if}
      {#if $summary.failed}<span class="bad">{t('design-airlock.failed', { n: $summary.failed })}</span>{/if}
    </span>
    <time class="clock">{now.toTimeString().slice(0, 8)}</time>
  </div>
</div>

<style>
  .al-strip {
    display: flex;
    align-items: center;
    flex: none;
    height: 45px;
    min-width: 0;
    overflow: hidden;
    padding: 0 var(--sp-3);
    background: var(--color-bg-chrome);
    border-bottom: 1px solid var(--color-border);
    font-size: var(--fs-sm);
    white-space: nowrap;
  }
  .wing {
    flex: 1 1 0; /* 넓은 창에서는 문이 창 한가운데에 온다 */
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--sp-2);
  }
  .duct {
    position: relative;
    flex: 1 1 8px;
    min-width: 8px;
    height: 22px;
    overflow: hidden;
    border-block: 1px solid var(--color-border-strong);
  }
  .wing.a .duct {
    margin-right: calc(-1 * var(--sp-2));
    background: linear-gradient(90deg, var(--color-net-business-subtle), transparent);
    color: var(--color-net-business);
  }
  .wing.b .duct {
    margin-left: calc(-1 * var(--sp-2));
    background: linear-gradient(270deg, var(--color-net-internet-subtle), transparent);
    color: var(--color-net-internet);
  }
  /* 통로의 바닥 눈금. 전송 중이면 흐르는 쪽으로 움직인다 */
  .duct i {
    position: absolute;
    inset: 6px -12px;
    background: repeating-linear-gradient(90deg, currentColor 0 2px, transparent 2px 12px);
    opacity: 0;
  }
  .flowing .duct i {
    opacity: 0.55;
  }
  @keyframes tick-ltr {
    to {
      transform: translateX(12px);
    }
  }
  @keyframes tick-rtl {
    to {
      transform: translateX(-12px);
    }
  }
  .role {
    flex: none;
    color: var(--color-text-3);
  }
  .plate {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex: none;
    height: 26px;
    padding: 0 var(--sp-3);
    border: 1px solid var(--edge);
    color: var(--lit);
    clip-path: polygon(7px 0, 100% 0, 100% calc(100% - 7px), calc(100% - 7px) 100%, 0 100%, 0 7px);
  }
  .plate.biz {
    --edge: var(--color-net-business-border);
    --lit: var(--color-net-business);
    --wash: var(--color-net-business-subtle);
  }
  .plate.inet {
    --edge: var(--color-net-internet-border);
    --lit: var(--color-net-internet);
    --wash: var(--color-net-internet-subtle);
  }
  /* 현재 망은 표시등이 켜진 판이다. 꽉 찬 강조 면은 누를 수 있는 단추만 쓴다 */
  .plate.here {
    border-color: var(--lit);
    background: var(--wash);
    box-shadow: inset 4px 0 0 var(--lit);
  }
  .plate b {
    font-weight: var(--fw-bold);
    letter-spacing: 0.03em;
  }
  .plate small {
    flex: none;
    font-size: var(--fs-caption);
    font-weight: var(--fw-medium);
  }
  .state {
    display: inline-flex;
    flex-wrap: wrap;
    gap: 0 var(--sp-2);
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    color: var(--color-text-2);
    line-height: 1.2;
  }
  .flowing .state {
    color: var(--color-text);
  }
  .pct {
    color: var(--color-text);
    font-family: var(--font-mono);
    font-weight: var(--fw-medium);
  }
  .bad {
    order: -1; /* 폭이 모자라도 실패가 먼저 보이게 */
    color: var(--color-danger);
    font-weight: var(--fw-semibold);
  }
  .clock {
    flex: none;
    margin-left: auto;
    color: var(--color-text-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }

  /* 잠금문 */
  .lock {
    position: relative;
    flex: none;
    width: 300px;
    height: 44px;
  }
  .lock svg {
    display: block;
    width: 300px;
    height: 100%;
  }
  .sealed {
    position: absolute;
    top: 12px;
    left: calc(50% - 70px); /* 두 문 사이의 방 */
    width: 140px;
    overflow: hidden;
    color: var(--color-text-2);
    font-size: var(--fs-caption);
    letter-spacing: 0.04em;
    line-height: 20px;
    text-align: center;
    text-overflow: ellipsis;
  }
  .room {
    fill: var(--color-surface-sunken);
  }
  .wall {
    fill: none;
    stroke: var(--color-border-strong);
  }
  .jamb {
    fill: var(--color-surface-raised);
    stroke: var(--color-text-3);
  }
  .post {
    fill: none;
    stroke: var(--color-text-3);
  }
  .bands {
    fill: url(#al-hazard);
    opacity: 0;
  }
  .haz-a {
    fill: var(--color-danger-fill);
  }
  .haz-b {
    fill: var(--color-bg-chrome);
  }
  .door.a .leaf {
    fill: var(--color-net-business);
  }
  .door.b .leaf {
    fill: var(--color-net-internet);
  }
  .lamp {
    fill: var(--color-text-3);
  }
  .flowing .bands {
    opacity: 1;
  }
  .flowing .door .leaf {
    fill: url(#al-hazard);
  }
  .flowing .lamp {
    fill: var(--color-danger-fill);
  }
  .packet.ltr {
    fill: var(--color-net-business);
  }
  .packet.rtl {
    fill: var(--color-net-internet);
  }
  /* 인터록: 앞문이 닫혀야 뒷문이 열린다. 화물은 방에서 기다렸다가 건너간다. 움직임을 줄인 설정이면 문은 닫힌 채 화물만 방에 보인다 */
  @media (prefers-reduced-motion: no-preference) {
    .ltr .duct i {
      animation: tick-ltr 700ms linear infinite;
    }
    .rtl:not(.ltr) .duct i {
      animation: tick-rtl 700ms linear infinite;
    }
    .packet.ltr {
      animation: pass-ltr 3.4s linear infinite;
    }
    .packet.rtl {
      animation: pass-rtl 3.4s linear infinite;
    }
    .flowing .door.a .leaf.up {
      animation: open-a-up 3.4s var(--ease) infinite;
    }
    .flowing .door.a .leaf.down {
      animation: open-a-down 3.4s var(--ease) infinite;
    }
    .flowing .door.b .leaf.up {
      animation: open-b-up 3.4s var(--ease) infinite;
    }
    .flowing .door.b .leaf.down {
      animation: open-b-down 3.4s var(--ease) infinite;
    }
  }
  @keyframes open-a-up {
    6%,
    30% {
      transform: translateY(-11px);
    }
    0%,
    36%,
    100% {
      transform: none;
    }
  }
  @keyframes open-a-down {
    6%,
    30% {
      transform: translateY(11px);
    }
    0%,
    36%,
    100% {
      transform: none;
    }
  }
  @keyframes open-b-up {
    56%,
    80% {
      transform: translateY(-11px);
    }
    0%,
    50%,
    86%,
    100% {
      transform: none;
    }
  }
  @keyframes open-b-down {
    56%,
    80% {
      transform: translateY(11px);
    }
    0%,
    50%,
    86%,
    100% {
      transform: none;
    }
  }
  @keyframes pass-ltr {
    0% {
      transform: translateX(-150px);
      opacity: 0;
    }
    4% {
      opacity: 1;
    }
    32%,
    52% {
      transform: none;
    }
    86% {
      opacity: 1;
    }
    90%,
    100% {
      transform: translateX(150px);
      opacity: 0;
    }
  }
  @keyframes pass-rtl {
    0% {
      transform: none;
    }
    28% {
      opacity: 1;
    }
    32% {
      transform: translateX(-150px);
      opacity: 0;
    }
    33%,
    50% {
      transform: translateX(150px);
      opacity: 0;
    }
    54% {
      opacity: 1;
    }
    85%,
    100% {
      transform: none;
    }
  }

  /* 창이 좁아지면 그림을 줄이지 않고(복도 선이 덕트와 어긋난다) 문 바깥 벽만 잘라 낸다. 날개는 내용대로 폭을 받아 상태 글자가 먼저 산다 */
  @media (max-width: 1099.98px) {
    .wing {
      flex: 1 1 auto;
    }
    .wing.a {
      flex-shrink: 0;
    }
    .lock {
      width: 176px;
      overflow: hidden;
    }
    .lock svg {
      margin-left: -62px;
    }
  }
  @media (max-width: 879.98px) {
    .role,
    .plate small {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
    .plate {
      padding: 0 var(--sp-2);
    }
  }
</style>
