<!--
  로그인 화면의 잠금문. 닫힌 두 짝 문과 가운데 봉합선을 그리고, 접속 중이면 빗장이 풀린 뒤 문이 벌어진다.
  문짝의 그림(face)은 한 벌을 두 번 그려 각 문짝이 절반씩 잘라 보인다 ― 잠금쇠와 두 망 이름이 봉합선에서 함께 갈라진다.
  고른 망의 이름은 봉합선에 잘리지 않게 문 위에 덧댄 명판에 쓴다.
-->
<script lang="ts">
  import type { ArtProps } from '../registry'

  let { busy, network, direction }: ArtProps = $props()

  const parts = $derived(direction.split('->').map((s) => s.trim()).filter(Boolean))
</script>

{#snippet face()}
  <svg class="hub" viewBox="0 0 120 120" aria-hidden="true">
    <path class="ring" d="M38 4H82L116 38V82L82 116H38L4 82V38Z" />
    <path class="ring" d="M44 18H76L102 44V76L76 102H44L18 76V44Z" />
    <path class="spoke" d="M60 18V40M60 80V102M18 60H40M80 60H102" />
    <path class="core" d="M50 40H70L80 50V70L70 80H50L40 70V50Z" />
  </svg>
  {#if parts.length === 2}
    <span class="route"><span>{parts[0]}</span><span><i>&gt;&gt;</i>{parts[1]}</span></span>
  {/if}
{/snippet}

<div class="door" class:busy>
  <div class="beyond">
    <i></i><i></i><i></i>
    <span class="slit"></span>
  </div>
  <div class="leaf a"><div class="face">{@render face()}</div></div>
  <div class="leaf b"><div class="face">{@render face()}</div></div>
  <span class="seam"></span>
  {#if network}<b class="name">{network}</b>{/if}
  <span class="bolt one"></span>
  <span class="bolt two"></span>
  <span class="bolt three"></span>
  <span class="bolt four"></span>
</div>

<style>
  .door {
    position: relative;
    flex: 1;
    min-height: 300px;
    overflow: hidden;
    background: var(--color-surface-sunken);
  }

  /* 문 너머 */
  .beyond {
    position: absolute;
    inset: 0;
    background: linear-gradient(90deg, var(--color-net-business-subtle), var(--color-surface-sunken) 50%, var(--color-net-internet-subtle));
  }
  /* 깎은 자리에도 선이 이어지게 빗금을 배경으로 그린다(14px 깎기의 대각선 거리는 9.9px) */
  .beyond i {
    position: absolute;
    border: 1px solid;
    border-color: var(--color-net-business-border) var(--color-net-internet-border) var(--color-net-internet-border) var(--color-net-business-border);
    background:
      linear-gradient(135deg, transparent 9.6px, var(--color-net-business-border) 0 10.9px, transparent 0) border-box,
      linear-gradient(315deg, transparent 9.6px, var(--color-net-internet-border) 0 10.9px, transparent 0) border-box;
    clip-path: polygon(14px 0, 100% 0, 100% calc(100% - 14px), calc(100% - 14px) 100%, 0 100%, 0 14px);
  }
  .beyond i:nth-child(1) {
    inset: 8% 12%;
  }
  .beyond i:nth-child(2) {
    inset: 18% 24%;
  }
  .beyond i:nth-child(3) {
    inset: 28% 36%;
  }
  .slit {
    position: absolute;
    top: 28%;
    bottom: 28%;
    left: calc(50% - 1px);
    width: 2px;
    background: linear-gradient(180deg, var(--color-net-business), var(--color-net-internet));
    opacity: 0;
  }
  .busy .slit {
    opacity: 1;
    transition: opacity var(--dur-slow) var(--ease) 400ms;
  }

  /* 문짝 */
  .leaf {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 50%;
    overflow: hidden;
    background:
      linear-gradient(180deg, transparent 24.5%, var(--color-border-subtle) 24.5% 25%, transparent 25% 74.5%, var(--color-border-subtle) 74.5% 75%, transparent 75%),
      var(--color-surface-raised);
    transition: transform 620ms var(--ease);
  }
  .leaf.a {
    left: 0;
  }
  .leaf.b {
    right: 0;
  }
  .busy .leaf {
    transition-delay: 160ms;
  }
  .busy .leaf.a {
    transform: translateX(-86%);
  }
  .busy .leaf.b {
    transform: translateX(86%);
  }
  .face {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 200%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--sp-4);
    padding-top: 44px; /* 명판 자리 */
    background: linear-gradient(90deg, rgba(167, 139, 250, 0.22), transparent 42%, transparent 58%, rgba(45, 212, 191, 0.22));
  }
  .leaf.a .face {
    left: 0;
  }
  .leaf.b .face {
    right: 0;
  }
  .name {
    position: absolute;
    top: 15%;
    left: 50%;
    max-width: 86%;
    padding: 4px 16px;
    overflow: hidden;
    border: 1px solid var(--color-border-strong);
    background: var(--color-bg-chrome);
    color: var(--color-text);
    font-size: 22px;
    font-weight: 800;
    letter-spacing: 0.1em;
    text-indent: 0.1em; /* 자간 때문에 글자가 왼쪽으로 치우치지 않게 */
    text-overflow: ellipsis;
    white-space: nowrap;
    transform: translateX(-50%);
    clip-path: polygon(8px 0, 100% 0, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%, 0 8px);
    transition: opacity var(--dur) var(--ease) 160ms;
  }
  .busy .name {
    opacity: 0;
  }
  .hub {
    width: 118px;
    height: 118px;
  }
  .ring {
    fill: none;
    stroke: var(--color-border-strong);
  }
  .spoke {
    fill: none;
    stroke: var(--color-border-strong);
    stroke-width: 5;
  }
  .core {
    fill: var(--color-bg-chrome);
    stroke: var(--color-text-3);
  }
  /* 내 망과 건너편 망을 봉합선 양쪽에 둔다. 글자 수가 달라도 선이 글자를 가르지 않는다 */
  .route {
    display: grid;
    grid-template-columns: 1fr 1fr;
    column-gap: var(--sp-4);
    align-self: stretch;
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    letter-spacing: 0.04em;
    white-space: nowrap;
    transition: opacity var(--dur) var(--ease) 160ms;
  }
  .busy .route {
    opacity: 0; /* 밀려난 문짝 끝에 글자 조각이 남지 않게 */
  }
  .route > :first-child {
    justify-self: end;
  }
  .route i {
    margin-right: var(--sp-2);
    color: var(--color-text-3);
    font-family: var(--font-mono);
    font-style: normal;
  }

  /* 봉합선과 빗장 */
  .seam {
    position: absolute;
    top: 0;
    bottom: 0;
    left: calc(50% - 1.5px);
    width: 3px;
    background: linear-gradient(90deg, var(--color-bg-chrome) 33%, var(--color-net-business-border) 33% 50%, var(--color-net-internet-border) 50% 67%, var(--color-bg-chrome) 67%);
    transition: opacity var(--dur) var(--ease) 160ms;
  }
  .busy .seam {
    opacity: 0;
  }
  .bolt {
    position: absolute;
    left: calc(50% - 24px);
    width: 48px;
    height: 9px;
    border-block: 1px solid var(--color-bg-chrome);
    background: linear-gradient(180deg, var(--color-text-3), var(--color-border-strong));
    transition:
      transform 140ms var(--ease),
      opacity 140ms var(--ease);
  }
  .bolt.one {
    top: 5%;
  }
  .bolt.two {
    top: 10%;
  }
  .bolt.three {
    bottom: 10%;
  }
  .bolt.four {
    bottom: 5%;
  }
  .busy .bolt {
    transform: scaleX(0);
    opacity: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .leaf,
    .seam,
    .name,
    .route,
    .bolt,
    .busy .slit {
      transition: none;
    }
  }
</style>
