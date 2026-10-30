<!--
  파일전송 영역의 상시 부품(Shell 이 로그인 뒤 저절로 붙인다). 탐색기 「받기」 메뉴(두 번째 실행 --mode download)를 처리한다.
  파일전송 화면으로 옮긴 뒤 체크한 행을 그 폴더로 받는다. 체크한 행이 없으면 원본처럼 안내 없이 아무것도 받지 않는다.
-->
<script lang="ts">
  import { onDestroy } from 'svelte'
  import { get } from 'svelte/store'
  import { TransferService, errorText } from '../../lib/api'
  import { navigate, toast } from '../../lib/stores/ui'
  import { fileSelection, pendingDownload } from './store'

  $effect(() => {
    const path = $pendingDownload
    if (!path) return
    pendingDownload.set(null)
    navigate('files')
    const keys = [...get(fileSelection)]
    if (keys.length) TransferService.DownloadTo(keys, path).catch((e) => toast(errorText(e), 3))
  })

  onDestroy(() => fileSelection.set(new Set())) // 로그아웃
</script>
