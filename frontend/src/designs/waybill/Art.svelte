<!--
  로그인 화면 왼쪽: 골판지 상자 옆면에 찍은 스텐실. 오른쪽 입력 칸이 이 상자에 붙은 빈 운송장이다.
  망 이름은 고른 망의 실제 값이고, 상자는 접속 중일 때만 움직이며 「접수 중」 도장이 찍힌다.
  입력 칸의 캡션 글자는 번역을 거쳐야 하므로 여기서 CSS 변수로 넘긴다.
-->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import type { ArtProps } from '../registry'
  import { paintBarcode } from './barcode'

  let { busy, network, direction }: ArtProps = $props()

  const ends = $derived(direction.split('->').map((s) => s.trim()))

  $effect(() => {
    paintBarcode('login')
    const root = document.documentElement.style
    root.setProperty('--wb-cap-from', JSON.stringify(t('design-waybill.from')))
    root.setProperty('--wb-cap-owner', JSON.stringify(t('design-waybill.owner')))
  })
</script>

<div class="side" class:busy>
  <div class="place">
    <small>{t('design-waybill.from')}</small>
    <strong>{ends[0] || network}</strong>
  </div>
  <div class="lane"><span class="crate"><i></i></span></div>
  <div class="place">
    <small>{t('design-waybill.to')}</small>
    <strong>{ends[1] ?? ''}</strong>
  </div>

  <div class="marks">
    <svg viewBox="0 0 40 40"><path d="M5 35h30M13 29V9M7 16l6-8 6 8M27 29V9M21 16l6-8 6 8" /></svg>
    <svg viewBox="0 0 40 40"><path d="M12 5h16l-1 11a7 7 0 0 1-14 0zM20 23v11M13 35h14M21 5l-3 5 4 2-2 4" /></svg>
    <svg viewBox="0 0 40 40"><path d="M5 21a15 15 0 0 1 30 0zM20 21v11a3.5 3.5 0 0 1-7 0M20 6V3" /></svg>
  </div>

  {#if busy}<span class="stamp">{t('design-waybill.accepting')}</span>{/if}
</div>

<style>
  .side {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    width: 100%;
  }
  .place {
    display: flex;
    flex-direction: column;
  }
  small {
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
    color: var(--color-text-2);
  }
  strong {
    min-height: 1.2em;
    font-family: var(--wb-display);
    font-size: 44px;
    font-weight: var(--fw-regular);
    line-height: 1.15;
    color: var(--wb-ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .lane {
    position: relative;
    height: 56px;
    margin-left: 22px;
    border-left: 2px dashed var(--wb-ink);
  }
  .crate {
    position: absolute;
    left: -14px;
    top: 16px;
    width: 26px;
    height: 22px;
    border: 2px solid var(--wb-ink);
    background: var(--wb-orange);
  }
  .crate i {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 4px;
    margin-left: -2px;
    background: var(--wb-ink);
  }
  .busy .crate {
    animation: down 0.9s var(--ease) infinite;
  }
  .marks {
    display: flex;
    gap: var(--sp-2);
    margin-top: var(--sp-4);
  }
  svg {
    width: 44px;
    height: 44px;
    padding: 3px;
    border: 2px solid var(--wb-ink);
    fill: none;
    stroke: var(--wb-ink);
    stroke-width: 2.4;
    stroke-linecap: square;
    stroke-linejoin: miter;
  }
  .stamp {
    position: absolute;
    right: 0;
    top: 62px;
    padding: 2px var(--sp-2);
    border: 3px solid var(--wb-ink);
    color: var(--wb-ink);
    font-size: var(--fs-xl);
    font-weight: 800;
    transform: rotate(-9deg);
    animation: stamp var(--dur-slow) var(--ease);
  }
  @keyframes down {
    from {
      transform: translateY(-14px);
    }
    to {
      transform: translateY(14px);
    }
  }
  @keyframes stamp {
    from {
      opacity: 0;
      transform: rotate(-9deg) scale(1.5);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .busy .crate,
    .stamp {
      animation: none;
    }
  }
</style>
