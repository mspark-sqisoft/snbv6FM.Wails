<!--
  노선도 로그인 그림(docs/DESIGNS.md): 강으로 갈라진 두 망이 환승역 한 곳에서 만난다. 서울 지하철 노선도의 한강처럼 강이 망의 경계다.
  왼쪽 위가 고른 망(direction 의 앞쪽)이고, 접속 중이면 열차가 환승역으로 들어간다.
-->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import type { ArtProps } from '../registry'

  let { busy, network, direction }: ArtProps = $props()

  const sides = $derived(direction.split('->').map((s) => s.trim()))
  const from = $derived(sides[0] || network)
  const to = $derived(sides[1] ?? '')
  // 망 색은 이름으로 가른다. 방향을 모르면 업무망 색으로 그린다
  const fromInternet = $derived(from.includes(t('common.net.internet')))
  const lineA = $derived(fromInternet ? 'var(--metro-internet)' : 'var(--metro-business)')
  const lineB = $derived(fromInternet ? 'var(--metro-business)' : 'var(--metro-internet)')
</script>

<svg class="art" class:busy viewBox="0 0 600 520" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
  <path class="river" d="M-400 800 C 60 520, 190 380, 300 262 S 520 40, 1000 -300" />

  <path class="line" stroke={lineA} d="M-900 120 H128 Q150 120 166 136 L300 270" />
  <path class="line" stroke={lineB} d="M300 270 L444 414 Q460 430 482 430 H1500" />

  <g class="stops" stroke={lineA}>
    <circle cx="66" cy="120" r="11" />
    <circle cx="215" cy="185" r="11" />
  </g>
  <g class="stops" stroke={lineB}>
    <circle cx="395" cy="365" r="11" />
    <circle cx="534" cy="430" r="11" />
  </g>

  <text x="66" y="96" text-anchor="middle">{t('design-metro.stop.upload')}</text>
  <text x="237" y="172">{t('design-metro.stop.scan')}</text>
  <text x="372" y="382" text-anchor="end">{t('design-metro.stop.send')}</text>
  <text x="534" y="402" text-anchor="middle">{t('design-metro.stop.receive')}</text>

  <text class="terminal" x="24" y="176">{from}</text>
  <text class="terminal" x="576" y="478" text-anchor="end">{to}</text>

  <g class="train">
    <rect x="-26" y="-11" width="52" height="22" rx="11" />
    <circle cx="-14" r="3.5" />
    <circle r="3.5" />
    <circle cx="14" r="3.5" />
  </g>

  <g class="hub">
    <circle class="ring" cx="300" cy="270" r="27" />
    <circle cx="300" cy="270" r="27" />
    <text x="300" y="277" text-anchor="middle">{t('design-metro.stop.approve')}</text>
  </g>
</svg>

<style>
  .art {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible; /* 선과 강이 그림 칸을 넘어 왼쪽 면 끝까지 간다 */
    font-family: var(--font-sans);
  }
  .river {
    fill: none;
    stroke: var(--color-surface);
    stroke-width: 74;
  }
  .line {
    fill: none;
    stroke-width: 16;
    stroke-linejoin: round;
  }
  .stops circle {
    fill: var(--color-bg);
    stroke-width: 7;
  }
  text {
    fill: var(--color-text);
    font-size: 19px;
    font-weight: 700;
  }
  .terminal {
    font-size: 34px;
    font-weight: 900;
    letter-spacing: -0.02em;
  }
  .hub circle {
    fill: var(--color-bg);
    stroke: var(--color-text);
    stroke-width: 9;
  }
  .hub text {
    font-size: 21px;
    font-weight: 900;
  }
  .hub .ring {
    fill: none;
    opacity: 0;
    transform-box: fill-box;
    transform-origin: center;
  }
  .train {
    opacity: 0;
    fill: var(--color-text);
    transform: translate(215px, 185px) rotate(45deg);
  }
  .train circle {
    fill: var(--color-bg);
  }
  .busy .train {
    animation: run 1.3s var(--ease) infinite;
  }
  .busy .ring {
    animation: ping 1.3s ease-out infinite;
  }
  @keyframes run {
    0% {
      opacity: 0;
      transform: translate(185px, 155px) rotate(45deg);
    }
    15% {
      opacity: 1;
    }
    80% {
      opacity: 1;
    }
    100% {
      opacity: 0;
      transform: translate(280px, 250px) rotate(45deg);
    }
  }
  @keyframes ping {
    0% {
      opacity: 0.7;
      transform: scale(1);
    }
    100% {
      opacity: 0;
      transform: scale(1.7);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .busy .train,
    .busy .ring {
      animation: none;
    }
    .busy .train {
      opacity: 1;
      transform: translate(258px, 228px) rotate(45deg);
    }
  }
</style>
