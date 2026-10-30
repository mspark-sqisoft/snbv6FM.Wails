<!--
  운송장 머리 띠: 보내는 곳 ─ 화물이 지나는 길 ─ 받는 곳 · 담당 · 화물 건수 · 현재 시각.
  값은 세션과 전송 대기열의 실제 값이다. 전송이 없으면 상자도 건수도 그리지 않는다.
  상자의 자리는 전송 상태 서랍의 전체 진행률과 같다.
-->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import { session } from '../../lib/stores/session'
  import { route } from '../../lib/stores/ui'
  import { items, summary } from '../../views/transfer/store'
  import { paintBarcode } from './barcode'

  let now = $state(new Date())
  $effect(() => {
    const timer = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(timer)
  })
  $effect(() => paintBarcode($route))

  const ends = $derived($session.direction.split('->').map((s) => s.trim()))
  const running = $derived($summary.pending + $summary.active)
  const overall = $derived($summary.total > 0 ? Math.floor(($summary.bytes / $summary.total) * 100) : 0)
  const date = $derived(now.toLocaleDateString('sv')) // 연도까지 다 쓴 값은 툴팁에 둔다
</script>

<div class="strip" id="waybillStrip">
  <span class="feed" aria-hidden="true"></span>
  <dl>
    <div class="cell from">
      <dt>{t('design-waybill.from')}</dt>
      <dd>{ends[0] || $session.networkName}</dd>
    </div>
    <div class="lane" class:moving={running > 0} aria-hidden="true">
      {#if running}
        <span class="crate" style:transform="translateX({overall}%)"><i></i></span>
        <b>{t('transfer.overall', { p: overall })}</b>
      {/if}
    </div>
    <div class="cell to">
      <dt>{t('design-waybill.to')}</dt>
      <dd>{ends[1] ?? ''}</dd>
    </div>
    <div class="cell owner">
      <dt>{t('design-waybill.owner')}</dt>
      <dd title={t(`common.role.${$session.role}`)}>{t(`common.role.${$session.role}`)}</dd>
    </div>
    {#if $items.length}
      <div class="cell">
        <dt>{t('design-waybill.cargo')}</dt>
        <dd class="small">{t('transfer.summary', { a: $summary.active, p: $summary.pending, d: $summary.done, f: $summary.failed })}</dd>
      </div>
    {/if}
    <div class="cell" title={date}>
      <dt>{t('design-waybill.clock')}</dt>
      <dd class="small mono"><span class="day">{`${date.slice(5)} `}</span>{now.toLocaleTimeString('sv')}</dd>
    </div>
  </dl>
  <span class="feed" aria-hidden="true"></span>
</div>

<style>
  .strip {
    flex: none;
    display: flex;
    height: 42px;
    min-width: 0;
    background: var(--color-surface-raised);
    border-bottom: 2px solid var(--wb-ink);
    overflow: hidden;
  }
  .feed {
    flex: none;
    width: 16px;
    background: var(--wb-hole) 0 -3px / 16px 24px repeat-y;
  }
  dl {
    flex: 1;
    display: flex;
    min-width: 0;
    margin: 0;
    border-inline: 1px dashed var(--wb-ink);
  }
  .cell {
    flex: none;
    display: flex;
    flex-direction: column;
    justify-content: center;
    min-width: 0;
    padding: 0 var(--sp-3);
    border-left: 1px solid var(--wb-ink);
  }
  .from {
    border-left: 0;
  }
  dt {
    font-size: var(--fs-caption);
    font-weight: var(--fw-bold);
    line-height: 1.1;
    color: var(--color-text-3);
    white-space: nowrap;
  }
  dd {
    min-height: 1.3em;
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-bold);
    line-height: 1.3;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* 서버가 준 망 이름이라 어떤 글자가 올지 모른다. 장식 글꼴은 이 크기에서 속이 막히므로 쓰지 않는다 */
  .from dd,
  .to dd {
    font-weight: 800;
    font-size: var(--fs-lg);
  }
  .small {
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
  }
  .mono {
    font-family: var(--font-mono);
  }
  /* 화물 건수는 줄이지 않는다. 띠가 모자라면 담당 글자가 줄어든다 */
  .owner {
    flex: 0 1 auto;
  }
  /* 화물이 지나는 길. 띠가 도는 것은 전송 중일 때뿐이고 상자의 자리는 실제 전체 진행률이다 */
  .lane {
    --crate: 18px;
    position: relative;
    flex: 1 1 60px;
    min-width: 60px;
    margin: 0 var(--sp-2);
    overflow: hidden;
  }
  .lane::before {
    content: '';
    position: absolute;
    left: -14px;
    right: 0;
    top: 50%;
    height: 2px;
    margin-top: -1px;
    background: repeating-linear-gradient(90deg, var(--wb-ink) 0 8px, transparent 8px 14px);
  }
  .lane::after {
    content: '';
    position: absolute;
    right: 0;
    top: 50%;
    width: 8px;
    height: 8px;
    border-top: 2px solid var(--wb-ink);
    border-right: 2px solid var(--wb-ink);
    transform: translateY(-50%) rotate(45deg);
    background: var(--color-surface-raised);
  }
  .moving::before {
    animation: belt 0.7s linear infinite;
  }
  .crate {
    position: absolute;
    inset: 0 calc(var(--crate) + 8px) 0 0;
    transition: transform var(--dur-slow) var(--ease);
  }
  .crate i {
    position: absolute;
    left: 0;
    top: 50%;
    width: var(--crate);
    height: 16px;
    margin-top: -8px;
    border: 2px solid var(--wb-ink);
    background: linear-gradient(90deg, transparent 6px, var(--wb-ink) 6px 8px, transparent 8px), var(--wb-orange);
  }
  .lane b {
    position: absolute;
    right: var(--sp-3);
    top: 2px;
    padding: 0 var(--sp-1);
    background: var(--color-surface-raised);
    font-family: var(--font-mono);
    font-size: var(--fs-caption);
    font-weight: var(--fw-medium);
    line-height: 1.2;
  }
  @keyframes belt {
    to {
      transform: translateX(14px);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .moving::before {
      animation: none;
    }
  }
  /* 좁거나 낮은 창: 칸 이름과 값을 한 줄로 놓아 표에 높이를 돌려준다 */
  @media (max-width: 880px), (max-height: 640px) {
    .strip {
      height: 28px;
    }
    .cell {
      flex-direction: row;
      align-items: center;
      gap: var(--sp-1);
      padding: 0 var(--sp-1);
    }
    dd {
      min-height: 0;
    }
    /* 날짜는 툴팁에 있다. 그 폭을 화물 건수에 준다 */
    .day {
      display: none;
    }
    /* 길이 짧아 진행률을 왼쪽에 두고 상자는 그 뒤로만 움직인다. 겹치면 상자가 글자 밑으로 사라진다 */
    .lane {
      min-width: 128px;
    }
    .lane b {
      left: 0;
      right: auto;
      top: 6px;
    }
    .crate {
      left: 84px;
    }
  }
</style>
