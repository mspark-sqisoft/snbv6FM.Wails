<!--
  플립 보드 글자판 묶음. text 의 글자마다 위아래 두 쪽 판을 그리고, 글자가 바뀐 칸만 판이 넘어간다.
  처음 그릴 때는 빈 판에서 넘어온다. delay 는 칸마다 늘어나는 시작 지연(ms)이다.
  화면 읽기 프로그램에는 글자 하나로 읽히게 text 를 숨은 글로 두고 판은 aria-hidden 이다.
-->
<script lang="ts">
  let { text, delay = 0 }: { text: string; delay?: number } = $props()

  const chars = $derived([...text])
  // 바뀌기 전 글자. 그린 뒤에 옮기며 반응형이 아니다 ― 넘어가는 판이 옛 글자를 읽어야 한다
  const last = { chars: [] as string[] }
  $effect(() => {
    last.chars = chars
  })
  // 한글·전각은 넓은 칸
  const wide = (c: string) => c > '⹿'
</script>

<span class="flap">
  <span class="sr-only">{text}</span>
  <span class="ms" aria-hidden="true">
    {#each chars as c, i (i)}
      <span class="m" class:wide={wide(c)} class:gap={c === ' '} style:--d="{i * delay}ms">
        <span class="h t"><i>{c}</i></span>
        <span class="h b"><i>{c}</i></span>
        {#key c}
          {@const p = last.chars[i] ?? ' '}
          {#if p !== c}
            <span class="h b old"><i>{p}</i></span>
            <span class="h t fold"><i>{p}</i></span>
            <span class="h b drop"><i>{c}</i></span>
          {/if}
        {/key}
      </span>
    {/each}
  </span>
</span>

<style>
  .flap,
  .ms {
    display: inline-flex;
    min-width: 0;
  }
  .ms {
    gap: 2px;
  }
  .m {
    --h: var(--mh, 1.75em);
    position: relative;
    flex: none;
    width: var(--mw, 0.86em);
    height: var(--h);
    font-family: var(--dep-table);
    font-weight: var(--fw-medium);
    font-size: inherit;
    line-height: 1;
  }
  .m.wide {
    width: var(--mw, 1.2em);
  }
  /* 칸 가운데의 경첩. 글자 위를 지나므로 옅게 둔다 ― 진하면 0 이 8 로 보인다 */
  .m::after {
    content: '';
    position: absolute;
    z-index: 3;
    left: 0;
    right: 0;
    top: calc(50% - 0.5px);
    height: 1px;
    background: var(--dep-hinge);
    opacity: 0.4;
  }
  .h {
    position: absolute;
    left: 0;
    right: 0;
    height: 50%;
    overflow: hidden;
    background: var(--dep-flap-top);
    backface-visibility: hidden;
  }
  .h.t {
    top: 0;
    border-radius: 2px 2px 0 0;
  }
  .h.b {
    bottom: 0;
    border-radius: 0 0 2px 2px;
    background: var(--dep-flap-bot);
  }
  .h i {
    position: absolute;
    left: 0;
    right: 0;
    height: var(--h);
    line-height: var(--h);
    text-align: center;
    font-style: normal;
  }
  .t i {
    top: 0;
  }
  .b i {
    bottom: 0;
  }
  .old {
    z-index: 1;
    animation: dep-old 260ms linear var(--d, 0ms) both;
  }
  .fold {
    z-index: 2;
    transform-origin: 50% 100%;
    animation: dep-fold 130ms ease-in var(--d, 0ms) both;
  }
  .drop {
    z-index: 2;
    transform-origin: 50% 0;
    animation: dep-drop 130ms ease-out calc(var(--d, 0ms) + 130ms) both;
  }
  @keyframes dep-old {
    99% {
      opacity: 1;
    }
    100% {
      opacity: 0;
    }
  }
  @keyframes dep-fold {
    from {
      transform: perspective(90px) rotateX(0);
    }
    to {
      transform: perspective(90px) rotateX(-90deg);
    }
  }
  @keyframes dep-drop {
    from {
      transform: perspective(90px) rotateX(90deg);
    }
    to {
      transform: perspective(90px) rotateX(0);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .old,
    .fold,
    .drop {
      display: none;
    }
  }
</style>
