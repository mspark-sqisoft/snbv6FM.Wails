<!--
  [결재처리] 화면(원본 content/ApprovalScrollView.qml, docs/port/06 5.2).
  들어올 때·[새로고침](F5)·approval:changed(알림 9002·9004 의 1초 뒤) 에 결재대상목록[3390]. 조회가 도는 동안은 겹쳐 보내지 않는다.
  [승인]/[반려] → 사유 창 → 고른 건 결재[3092](Go 가 동시 4건) → 결과 요약 → 목록 다시 받기. 우클릭 [다운로드] 는 결재 받기(서버에서 지우지 않음).
  받기 진행은 전송 서랍이 보인다. 받는 중에 또 누르면 「받는 중」 안내(APR-062).
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import Check from '@lucide/svelte/icons/check'
  import X from '@lucide/svelte/icons/x'
  import DownloadIcon from '@lucide/svelte/icons/download'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Inbox from '@lucide/svelte/icons/inbox'
  import FileIcon from '@lucide/svelte/icons/file'
  import FolderIcon from '@lucide/svelte/icons/folder'
  import ClipboardIcon from '@lucide/svelte/icons/clipboard'
  import {
    Badge,
    Button,
    ContextMenu,
    DataTable,
    EmptyState,
    IconButton,
    Page,
    StatusChip,
    formatBytes,
    type Column,
    type MenuItem,
    type Tone,
  } from '../../lib/ui'
  import { t, tx } from '../../lib/i18n'
  import { ApprovalService, Ev, errorText, on, type ApprovalItem } from '../../lib/api'
  import { alert, toast } from '../../lib/stores/ui'
  import CauseDialog from './dialogs/CauseDialog.svelte'

  type Row = ApprovalItem & { no: number } // 번호는 받을 때 매긴다 ― 칸마다 indexOf 로 찾으면 행 수의 제곱이 든다

  let rows = $state.raw<Row[]>([])
  let selected = $state(new Set<string>())
  let loading = $state(false)
  let loadError = $state('')
  let sortKey = $state<string | undefined>('requestTime')
  let sortDir = $state<'asc' | 'desc'>('desc')
  let menu = $state({ open: false, x: 0, y: 0 })
  let cause = $state({ open: false, approve: true })
  let downloading = $state(false)

  const key = (r: ApprovalItem) => String(r.seqno)
  const picked = $derived(rows.filter((r) => selected.has(key(r))))

  // 결재상태 톤(원본 FileStatus.approvalTone)
  const TONE: Record<number, Tone> = { 0: 'info', 1: 'success', 2: 'danger', 3: 'success', 4: 'neutral' }

  const columns: Column<Row>[] = $derived([
    { key: 'no', title: '#', width: 50, align: 'right', sortable: false, value: (r) => r.no },
    { key: 'course', title: t('approval.approve.col.course'), width: 170, value: (r) => r.courseText, cell: courseCell },
    { key: 'requester', title: t('approval.approve.col.requester'), width: 100, value: (r) => r.userName || r.requester },
    { key: 'fileName', title: t('approval.approve.col.fileName'), width: 300, grow: true, value: (r) => r.fileName, cell: nameCell },
    { key: 'fileSize', title: t('approval.approve.col.fileSize'), width: 90, align: 'right', value: (r) => r.fileSize, cell: sizeCell },
    { key: 'requestTime', title: t('approval.approve.col.time'), width: 165 },
    { key: 'cause', title: t('approval.approve.col.cause'), width: 220 },
    { key: 'state', title: t('approval.approve.col.state'), width: 120, value: (r) => r.stateText, cell: stateCell },
  ])

  async function refresh() {
    if (loading) return // 같은 조회가 도는 동안 겹쳐 보내지 않는다
    loading = true
    try {
      rows = ((await ApprovalService.Pending()) ?? []).map((r, i) => ({ ...r, no: i + 1 }))
      loadError = ''
    } catch (e) {
      loadError = errorText(e) // 실패해도 회전 표시를 끈다(원본은 계속 돌았다)
    } finally {
      loading = false
    }
  }

  function ask(approve: boolean) {
    if (!picked.length) return toast(t('approval.approve.selectFirst'), 2)
    cause = { open: true, approve }
  }

  async function submit(text: string) {
    const items = picked
    const approve = cause.approve
    try {
      const res = (await ApprovalService.Approve(items, approve, text)) ?? []
      const ok = res.filter((r) => r.ok).length
      const fails = res.filter((r) => !r.ok)
      const done = t(approve ? 'approval.approve.doneApprove' : 'approval.approve.doneReject', { n: ok })
      if (fails.length) {
        const byKey = new Map(items.map((r) => [r.seqno, r.fileName]))
        void alert({
          title: t('approval.approve.failTitle'),
          text: `${done}, ${t('approval.approve.failed', { n: fails.length })}\n\n${fails.map((f) => `${byKey.get(f.seqno) ?? f.seqno}: ${f.reason}`).join('\n')}`,
        })
      } else {
        toast(done, 1)
      }
      selected = new Set()
    } catch (e) {
      toast(errorText(e), 3)
    }
    await refresh() // 실패가 있어도 목록을 다시 받는다
  }

  async function download(items = picked) {
    if (!items.length) return toast(t('approval.approve.selectDownload'), 2)
    if (downloading) return void alert({ title: t('approval.approve.busyTitle'), text: t('approval.approve.busy') })
    downloading = true
    try {
      const r = await ApprovalService.DownloadForApproval(items)
      if (!r || r.canceled) return
      if (r.busy) return void alert({ title: t('approval.approve.busyTitle'), text: t('approval.approve.busy') })
      if (r.saved?.length) toast(t('approval.approve.downloaded', { names: r.saved.join(', ') }), 1)
      if (r.fails?.length) void alert({ title: t('approval.approve.downloadFailTitle'), text: r.fails.join('\n') })
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      downloading = false
    }
  }

  const menuItems: MenuItem[] = $derived([
    { label: t('approval.approve.approve'), icon: Check, disabled: !picked.length, onSelect: () => ask(true) },
    { label: t('approval.approve.reject'), icon: X, disabled: !picked.length, danger: true, onSelect: () => ask(false) },
    { separator: true },
    { label: t('approval.approve.download'), icon: DownloadIcon, disabled: !picked.length, onSelect: () => download() },
    { separator: true },
    { label: t('common.refresh'), icon: RefreshCw, hint: 'F5', onSelect: refresh },
  ])

  function onkey(e: KeyboardEvent) {
    if (e.key === 'F5') {
      e.preventDefault()
      void refresh()
    }
  }

  onMount(() => {
    void refresh()
    return on(Ev.approvalChanged, () => void refresh())
  })
</script>

<svelte:window onkeydown={onkey} />

{#snippet nameCell(r: ApprovalItem)}
  <span class="name" id="{r.serverName}.fileNameCell">
    {#if r.clip}<ClipboardIcon size={15} aria-hidden="true" />{:else if r.dirFlag}<FolderIcon size={15} aria-hidden="true" />{:else}<FileIcon
        size={15}
        aria-hidden="true"
      />{/if}
    <span class="ellipsis">{r.fileName}</span>
    {#if r.privCnt > 0}<Badge size="sm" tone="warning" label={t('approval.approve.priv', { n: r.privCnt })} />{/if}
  </span>
{/snippet}
{#snippet sizeCell(r: ApprovalItem)}
  <span class="num">{formatBytes(r.fileSize)}</span>
{/snippet}
{#snippet courseCell(r: ApprovalItem)}
  <Badge size="sm" tone={r.course === 1 ? 'business' : 'internet'} label={r.courseText} />
{/snippet}
{#snippet stateCell(r: ApprovalItem)}
  <StatusChip tone={TONE[r.aprvState] ?? 'neutral'} label={tx(r.stateText)} />
{/snippet}

<Page
  title={t('common.menu.approve')}
  subtitle={selected.size ? t('common.selected', { n: selected.size }) : t('common.total', { n: rows.length })}
>
  {#snippet actions()}
    <Button variant="primary" icon={Check} disabled={!picked.length} onclick={() => ask(true)}>{t('approval.approve.approve')}</Button>
    <Button variant="danger" icon={X} disabled={!picked.length} onclick={() => ask(false)}>{t('approval.approve.reject')}</Button>
    <Button icon={DownloadIcon} disabled={!picked.length} onclick={() => download()}>{t('approval.approve.download')}</Button>
    <IconButton icon={RefreshCw} label="{t('common.refresh')} (F5)" {loading} onclick={refresh} />
  {/snippet}

  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {/if}
  <div class="table">
    <DataTable
      id="approvalTable"
      label={t('approval.approve.table')}
      {rows}
      {columns}
      rowKey={key}
      bind:selected
      bind:sortKey
      bind:sortDir
      storageKey="approvalList"
      {loading}
      oncontextmenu={(e) => (menu = { open: true, x: e.clientX, y: e.clientY })}
    >
      {#snippet empty()}
        <EmptyState compact icon={Inbox} title={t('approval.approve.empty')} description={t('approval.approve.emptyHint')} />
      {/snippet}
    </DataTable>
  </div>
</Page>

<ContextMenu bind:open={menu.open} x={menu.x} y={menu.y} items={menuItems} label={t('approval.approve.menu')} />
<CauseDialog bind:open={cause.open} approve={cause.approve} count={picked.length} onSubmit={submit} />

<style>
  .table {
    flex: 1;
    min-height: 0;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .name {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    min-width: 0;
  }
  .name :global(svg) {
    flex: none;
    color: var(--color-text-3);
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .num {
    font-variant-numeric: tabular-nums;
    color: var(--color-text-2);
  }
  .error {
    margin: 0 0 var(--sp-2);
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--color-danger-border);
    border-radius: var(--r-md);
    background: var(--color-danger-subtle);
    color: var(--color-danger);
    font-size: var(--fs-sm);
  }
</style>
