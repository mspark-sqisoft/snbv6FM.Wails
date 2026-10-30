<!--
  공문의 로그인 그림: 기안 용지의 머리와 빈 결재란. 발신·수신은 고른 망의 방향이고, 접속 중이면 빈 날인 자리가 깜박인다.
  도장은 성공한 뒤에만 찍힌다. 그 도장은 Strip 이 찍는다(seal.ts).
-->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import type { ArtProps } from '../registry'
  import { seal } from './seal'

  let { busy, network, direction }: ArtProps = $props()

  const dir = $derived(direction.split('->').map((s) => s.trim()))

  $effect(() => {
    seal.armed = busy
  })
</script>

<div class="sheet">
  <p class="name">{t('design-gongmun.form')}</p>
  <div class="rows">
    <div class="r"><span class="k">{t('design-gongmun.network')}</span><span class="v">{network}</span></div>
    <div class="r"><span class="k">{t('design-gongmun.from')}</span><span class="v">{dir[0] ?? ''}</span></div>
    <div class="r"><span class="k">{t('design-gongmun.to')}</span><span class="v">{dir[1] ?? ''}</span></div>
  </div>
  <div class="box">
    <span class="spine">{t('design-gongmun.box')}</span>
    <div class="c">
      <span class="k">{t('design-gongmun.staff')}</span>
      <span class="s"><span class="slot" class:busy>{t('design-gongmun.sealSlot')}</span></span>
    </div>
    <div class="c"><span class="k">{t('design-gongmun.review')}</span><span class="s"></span></div>
    <div class="c"><span class="k">{t('design-gongmun.approve')}</span><span class="s"></span></div>
  </div>
</div>

<style>
  .sheet {
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
    width: 100%;
    margin: var(--sp-4) 0;
  }
  .name {
    margin: 0;
    padding-bottom: var(--sp-2);
    border-bottom: 3px double var(--color-text);
    color: var(--color-text);
    font-family: var(--gm-serif);
    font-size: var(--fs-2xl);
    font-weight: var(--fw-bold);
    letter-spacing: 0.34em;
    word-spacing: -0.3em;
    text-align: center;
    white-space: nowrap;
  }
  .rows,
  .box {
    border: 1px solid var(--color-text);
    background: var(--color-bg);
  }
  .r {
    display: flex;
    height: 30px;
    border-bottom: 1px solid var(--color-border);
  }
  .r:last-child {
    border-bottom: 0;
  }
  .k {
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-surface);
    color: var(--color-text-2);
    font-family: var(--gm-serif);
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
    letter-spacing: 0.08em;
  }
  .r .k {
    flex: none;
    width: 68px;
    border-right: 1px solid var(--color-text);
  }
  .v {
    display: flex;
    align-items: center;
    min-width: 0;
    padding: 0 var(--sp-3);
    color: var(--color-text);
    font-size: var(--fs-body);
  }
  .box {
    display: flex;
    height: 92px;
  }
  .spine {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    background: var(--color-surface);
    border-right: 1px solid var(--color-text);
    color: var(--color-text-2);
    font-family: var(--gm-serif);
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
    letter-spacing: 0.3em;
    writing-mode: vertical-rl;
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
  .c .k {
    flex: none;
    height: 24px;
    border-bottom: 1px solid var(--color-text);
  }
  .s {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .slot {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border: 1px dashed var(--color-border-strong);
    border-radius: 50%;
    color: var(--color-text-3);
    font-size: var(--fs-caption);
  }
  .slot.busy {
    animation: gm-wait 600ms var(--ease) infinite alternate;
  }
  @keyframes gm-wait {
    to {
      opacity: 0.25;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .slot.busy {
      animation: none;
    }
  }
</style>
