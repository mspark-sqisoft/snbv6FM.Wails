<!--
  결재 영역의 전역 창(로그인 뒤 Shell 이 붙인다): Go 가 approval:select 로 여는 [결재자 선택] 창.
  업로드(전송)·[결재요청] 어느 쪽이 연 창이든 여기서 그린다. approval:select-close 가 오면 답하지 않고 닫는다(부른 쪽이 취소).
  로그아웃하면 이 컴포넌트가 사라지고, Go 는 기다리던 창을 취소로 정리한다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { Ev, onAll } from '../../lib/api'
  import ApproverDialog from './dialogs/ApproverDialog.svelte'
  import type { SelectAsk } from './types'

  let ask = $state<SelectAsk | null>(null)
  let open = $state(false)

  onMount(() =>
    onAll({
      [Ev.approverSelectAsk]: (a: SelectAsk) => {
        ask = a
        open = true
      },
      [Ev.approverSelectClose]: (d: { gen: number }) => {
        if (ask?.gen === d.gen) ask = null
      },
    }),
  )
</script>

{#if ask}
  <ApproverDialog {ask} bind:open />
{/if}
