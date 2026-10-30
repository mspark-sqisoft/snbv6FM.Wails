<!--
  공문의 문서 머리. 머리줄 아래 띠에 발신·수신·시행·전송을 칸으로 적고, 머리줄 오른쪽 구석에 결재란을 띄운다.
  모든 값은 실제 상태다: 망 방향($session), 시각, 전송 대기열($summary). 값이 없으면 칸을 비운다.
  결재란의 도장은 실제로 일어난 일(로그인)에만 찍는다. 권한과 전송 현황은 글자로 적는다. 승인 칸은 권한이지 승인 사실이 아니다.
  같은 값이 머리줄·전송 서랍에 있으므로 띠 전체는 장식(aria-hidden)이다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { t, locale } from '../../lib/i18n'
  import { session } from '../../lib/stores/session'
  import { summary } from '../../views/transfer/store'
  import { seal } from './seal'

  let now = $state(new Date())
  let stamping = $state(seal.armed)
  seal.armed = false

  onMount(() => {
    const timer = setInterval(() => (now = new Date()), 15000)
    return () => clearInterval(timer)
  })

  const dir = $derived($session.direction.split('->').map((s) => s.trim()))
  const when = $derived(
    new Intl.DateTimeFormat($locale === 'ko' ? 'ko-KR' : 'en-GB', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      weekday: 'short',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(now),
  )
  const running = $derived($summary.pending + $summary.active)
  const anyItem = $derived(running + $summary.done + $summary.failed + $summary.cancelled > 0)
  const overall = $derived($summary.total > 0 ? Math.floor(($summary.bytes / $summary.total) * 100) : 0)
  const name = $derived(($session.user?.userName ?? '').slice(0, 3))
  // 전송 칸: 실패가 있으면 실패 건수(색과 글자의 근거가 같아야 한다), 없으면 진행 건수, 끝났으면 완료, 대기열이 비었으면 빈칸.
  // 진행 건수는 띠의 전송 칸에도 있다.
  const review = $derived(
    $summary.failed
      ? t('design-gongmun.failed', { n: $summary.failed })
      : running
        ? t('design-gongmun.count', { n: running })
        : $summary.done
          ? t('design-gongmun.done')
          : '',
  )
  const authority = $derived(
    $session.role === 'approver' ? t('design-gongmun.approver') : $session.role === 'proxy' ? t('design-gongmun.proxy') : '',
  )
</script>

<div class="strip" aria-hidden="true">
  <div class="f"><span class="k">{t('design-gongmun.from')}</span><span class="v">{dir[0] ?? ''}</span></div>
  <div class="f"><span class="k">{t('design-gongmun.to')}</span><span class="v">{dir[1] ?? ''}</span></div>
  <div class="f date"><span class="k">{t('design-gongmun.date')}</span><span class="v">{when}</span></div>
  {#if anyItem}
    <div class="f flow">
      <span class="k">{t('design-gongmun.flow')}</span>
      <span class="v">
        {#if running}<span class="rule"><i style:transform="scaleX({overall / 100})"></i></span><b>{overall}%</b>{/if}
        <span class="sum">{t('transfer.summary', { a: $summary.active, p: $summary.pending, d: $summary.done, f: $summary.failed })}</span>
      </span>
    </div>
  {/if}
</div>

<div class="sign" aria-hidden="true">
  <span class="spine">{t('design-gongmun.box')}</span>
  <div class="c" title={t('design-gongmun.staffTip')}>
    <span class="k">{t('design-gongmun.staff')}</span>
    <span class="s"><span class="seal">{name}</span></span>
  </div>
  <div class="c" class:void={!review} class:bad={$summary.failed > 0} class:live={!$summary.failed && running > 0} title={t('design-gongmun.reviewTip')}>
    <span class="k">{t('design-gongmun.review')}</span>
    <span class="s">{review}</span>
  </div>
  <div class="c" class:void={!authority} title={t('design-gongmun.approveTip')}>
    <span class="k">{t('design-gongmun.approve')}</span>
    <span class="s">{authority}</span>
  </div>
</div>

{#if stamping}
  <div class="landing" aria-hidden="true" onanimationend={() => (stamping = false)}>
    <span class="big" class:en={$locale === 'en'}>
      <small>{$session.networkName}</small>
      <b>{t('design-gongmun.seal')}</b>
      <small>{$session.user?.userName ?? ''}</small>
    </span>
  </div>
{/if}

<style>
  .strip {
    flex: none;
    display: flex;
    align-items: stretch;
    height: 28px;
    min-width: 0;
    padding-right: var(--gm-box-w);
    overflow: hidden;
    background: var(--color-bg);
    border-bottom: 3px double var(--color-text);
    font-size: var(--fs-sm);
    white-space: nowrap;
  }
  .f {
    flex: none;
    display: flex;
    align-items: stretch;
    border-right: 1px solid var(--color-border);
  }
  .k {
    display: flex;
    align-items: center;
    padding: 0 var(--sp-2) 2px; /* 고운바탕이 낮게 앉아 값 글자와 기준선을 맞춘다 */
    background: var(--color-surface);
    border-right: 1px solid var(--color-border);
    color: var(--color-text-2);
    font-family: var(--gm-serif);
    font-weight: var(--fw-bold);
    letter-spacing: 0.08em;
  }
  .v {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 0 var(--sp-3);
    color: var(--color-text);
  }
  .f:first-child .k {
    padding-left: var(--sp-4);
  }
  .date .v {
    font-variant-numeric: tabular-nums;
  }
  .flow {
    flex: 1;
    min-width: 0;
    border-right: 0;
  }
  .flow .v {
    min-width: 0;
    overflow: hidden;
  }
  .flow b {
    flex: none;
    font-weight: var(--fw-semibold);
    font-variant-numeric: tabular-nums;
  }
  .sum {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .rule {
    flex: none;
    position: relative;
    width: 90px;
    height: 8px;
    border: 1px solid var(--color-text);
    background: repeating-linear-gradient(90deg, transparent 0 8px, var(--color-border) 8px 9px);
  }
  .rule i {
    position: absolute;
    inset: 0;
    background: var(--color-accent-fill);
    transform-origin: left;
  }

  /* 머리줄과 띠 두 줄에 걸친다. 띠 아래 이중선 바로 위까지다. */
  .sign {
    position: absolute;
    top: 0;
    right: 0;
    z-index: 1;
    display: flex;
    width: var(--gm-box-w);
    height: calc(var(--topbar-h) + 25px);
    border-left: 1px solid var(--color-text);
    background: var(--color-bg);
    font-size: var(--fs-sm);
  }
  .spine {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    background: var(--color-surface);
    border-right: 1px solid var(--color-text);
    color: var(--color-text-2);
    font-family: var(--gm-serif);
    font-weight: var(--fw-bold);
    writing-mode: vertical-rl;
    letter-spacing: 0.25em;
  }
  .c {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--color-border);
  }
  .c:last-child {
    border-right: 0;
  }
  .c > .k {
    flex: none;
    justify-content: center;
    height: 21px;
    padding: 0;
    border-right: 0;
    border-bottom: 1px solid var(--color-text);
    letter-spacing: 0.04em;
  }
  .s {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    font-variant-numeric: tabular-nums;
    line-height: 1.2;
    text-align: center;
    white-space: pre-line;
    word-break: keep-all; /* 좁은 창에서 '실패 1건' 이 숫자와 단위 사이에서 끊기지 않게 */
  }
  .bad .s {
    background: var(--color-danger-subtle);
    color: var(--color-danger);
  }
  .live .s {
    color: var(--color-accent);
  }
  .seal {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border: 1.5px solid var(--color-text);
    border-radius: 50%;
    color: var(--color-text);
    font-family: var(--gm-serif);
    font-size: 12px;
    font-weight: var(--fw-bold);
    line-height: 1;
    letter-spacing: -0.04em;
    white-space: nowrap;
    transform: rotate(-8deg);
  }
  /* 쓰지 않는 칸에는 사선을 긋는다(결재란 관행) */
  .void .s {
    background: linear-gradient(
      to top right,
      transparent calc(50% - 0.5px),
      var(--color-border-strong) calc(50% - 0.5px) calc(50% + 0.5px),
      transparent calc(50% + 0.5px)
    );
  }

  /* 결재란 바로 아래에 찍는다. 로그인 직후 가운데에 뜨는 안내 창에 가리지 않는다 */
  .landing {
    position: fixed;
    top: calc(var(--topbar-h) + 36px);
    right: var(--sp-6);
    z-index: var(--z-overlay);
    pointer-events: none;
  }
  .big {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    width: 188px;
    height: 188px;
    border: 7px solid var(--color-accent-fill);
    border-radius: 50%;
    outline: 2px solid var(--color-accent-fill);
    outline-offset: -14px;
    color: var(--color-accent-fill);
    font-family: var(--gm-serif);
    font-weight: var(--fw-bold);
    mix-blend-mode: multiply;
    animation: gm-land 1300ms var(--ease) both;
  }
  .big b {
    font-size: 46px;
    line-height: 1.1;
    letter-spacing: 0.06em;
    white-space: nowrap;
  }
  .big.en b {
    font-size: 30px;
    letter-spacing: 0;
  }
  .big small {
    font-size: 14px;
    letter-spacing: 0.14em;
  }
  @keyframes gm-land {
    0% {
      opacity: 0;
      transform: rotate(-16deg) scale(2.4);
    }
    14% {
      opacity: 1;
      transform: rotate(-9deg) scale(0.96);
    }
    20% {
      transform: rotate(-9deg) scale(1);
    }
    72% {
      opacity: 0.95;
      transform: rotate(-9deg) scale(1);
    }
    100% {
      opacity: 0;
      transform: rotate(-9deg) scale(1);
    }
  }
  @media (max-width: 879px) {
    /* 눈금은 장식이다. 같은 값이 바로 옆 백분율에 있고, 잘리는 쪽은 실패 건수다 */
    .rule {
      display: none;
    }
    .seal {
      width: 36px;
      height: 36px;
      font-size: 10px;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .landing {
      display: none;
    }
  }
</style>
