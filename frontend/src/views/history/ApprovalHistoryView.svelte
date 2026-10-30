<!-- 결재내역(docs/port/04 5.7). 메뉴는 업무망의 결재자(aprv_yn)에게만 보인다(routes.ts 의 approver 역할). -->
<script lang="ts">
  import HistoryView from './HistoryView.svelte'
</script>

<HistoryView kind="approval" />
