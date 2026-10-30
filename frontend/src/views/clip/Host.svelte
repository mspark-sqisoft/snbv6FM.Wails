<!--
  클립보드 영역의 상시 부품(Shell 이 로그인 뒤 저절로 붙인다). 진행은 전송 서랍의 '클립보드' 행이 보이고, 여기서는 끝난 뒤 안내만 한다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { on } from '../../lib/api'
  import { t } from '../../lib/i18n'
  import { toast } from '../../lib/stores/ui'

  onMount(() => on('clip:received', () => toast(t('clip.received'))))
</script>
