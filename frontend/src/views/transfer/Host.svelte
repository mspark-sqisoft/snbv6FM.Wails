<!--
  전송 영역의 상시 부품(Shell 이 로그인 뒤 저절로 붙인다). 대기열 사건을 store 에 붓고 파일 중복 확인 창을 띄운다.
  - transfer:state  → 목록 전체(200ms 묶음) · transfer:progress → 진행값 묶음 · transfer:open → 서랍 열기
  - transfer:upload-queued → 탐색기 패널의 올리기가 검사를 지나 대기열에 들어갔다는 안내
  - transfer:conflict → 파일 중복 확인 창 · transfer:conflict-close → 그 창 닫기(로그아웃)
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { TransferService, Ev, onAll } from '../../lib/api'
  import { t } from '../../lib/i18n'
  import { toast } from '../../lib/stores/ui'
  import ConflictDialog from './ConflictDialog.svelte'
  import { EV_CONFLICT_CLOSE, EV_OPEN, EV_UPLOAD_QUEUED, applyTicks, drawerOpen, items, type ConflictAsk, type Item, type Tick } from './store'

  let ask = $state<ConflictAsk | null>(null)

  onMount(() => {
    TransferService.Items()
      .then((list) => items.set(list ?? []))
      .catch(() => {})
    return onAll({
      [Ev.transferState]: (list: Item[] | null) => items.set(list ?? []),
      [Ev.transferProgress]: (ticks: Tick[] | null) => applyTicks(ticks),
      [EV_OPEN]: () => drawerOpen.set(true),
      [EV_UPLOAD_QUEUED]: (n: number) => toast(t('local.uploadStarted', { n }), 0),
      [Ev.downloadConflict]: (a: ConflictAsk) => (ask = a),
      [EV_CONFLICT_CLOSE]: (d: { gen: number }) => {
        if (ask && ask.gen === d.gen) ask = null
      },
    })
  })
</script>

<ConflictDialog bind:ask />
