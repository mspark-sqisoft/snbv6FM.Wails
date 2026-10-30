<!--
  정책 영역의 전역 창(로그인 뒤 Shell 이 붙인다): Go 가 여는 [파일 검사 결과] 창과 [압축 비밀번호 입력] 창.
  업로드 검사(전송 영역이 PolicyChecker.CheckUpload 로 부른다)와 결재자의 다운로드 검사가 같은 창을 쓴다.
  행은 Go 가 120ms 마다 모아 보내고, 같은 key 는 바꾼다. Go 가 닫으라 하면(밀려남·로그아웃·시간 초과) 답하지 않고 내린다.
  질문마다 {#key} 로 창을 새로 붙여 앞 질문의 입력·답 상태가 남지 않게 한다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { Ev, onAll } from '../../lib/api'
  import FileCheckResultDialog from './dialogs/FileCheckResultDialog.svelte'
  import ZipPasswordDialog from './dialogs/ZipPasswordDialog.svelte'
  import type { CheckOpen, CheckResult, CheckRow, CheckRows, ZipAsk } from './types'

  let check = $state<CheckOpen | null>(null)
  let checkOpen = $state(false)
  let rows = $state<CheckRow[]>([])
  let done = $state(0)
  let total = $state(0)
  let current = $state('')
  let result = $state<CheckResult | null>(null)

  let zip = $state<ZipAsk | null>(null)
  let zipOpen = $state(false)
  let zipSeq = $state(0)

  // 행 반영: key 로 바꾸거나 더하고 order 로 줄 세운다(원본 upsertRow).
  function merge(add: CheckRow[]) {
    const byKey = new Map(rows.map((r) => [r.key, r]))
    for (const r of add) byKey.set(r.key, r)
    rows = [...byKey.values()].sort((a, b) => a.order - b.order)
  }

  onMount(() =>
    onAll({
      [Ev.fileCheckOpen]: (o: CheckOpen) => {
        rows = []
        done = 0
        total = o.total
        current = ''
        result = null
        check = o
        checkOpen = true
      },
      [Ev.fileCheckRow]: (d: CheckRows) => {
        if (check?.gen !== d.gen) return
        if (d.rows?.length) merge(d.rows)
        done = d.done
        total = d.total
        current = d.current
      },
      [Ev.fileCheckResult]: (r: CheckResult) => {
        if (check?.gen === r.gen) result = r
      },
      [Ev.fileCheckClose]: (d: { gen: number }) => {
        if (check?.gen === d.gen) check = null
      },
      [Ev.zipPasswordAsk]: (a: ZipAsk) => {
        if (!a.name) {
          if (zip?.gen === a.gen) zip = null
          return
        }
        zip = a
        zipSeq++
        zipOpen = true
      },
    }),
  )
</script>

{#if check}
  {#key check.gen}
    <FileCheckResultDialog {check} {rows} {done} {total} {current} {result} bind:open={checkOpen} />
  {/key}
{/if}
{#if zip}
  {#key zipSeq}
    <ZipPasswordDialog ask={zip} bind:open={zipOpen} />
  {/key}
{/if}
