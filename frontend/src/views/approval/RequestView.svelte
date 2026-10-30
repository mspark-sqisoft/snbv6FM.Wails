<!--
  [결재요청] 화면(사후결재 모드, fm_upfile_mode 1 ― docs/port/06 2.1·2.3).
  이 망에서 올린 서버 파일을 보이고, `결재요청 가능` 행만 고를 수 있다(나머지는 까닭을 툴팁으로).
  [결재요청] → Go 가 서버파일 목록을 새로 받아 다시 판정 → 결재자 창(Host) → 결재요청[4127](비밀번호별 10개 묶음).
  메뉴에는 없고 파일전송 화면에서 navigate('request') 로 연다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import Send from '@lucide/svelte/icons/send'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import FileIcon from '@lucide/svelte/icons/file'
  import ClipboardIcon from '@lucide/svelte/icons/clipboard'
  import CloudOff from '@lucide/svelte/icons/cloud-off'
  import Info from '@lucide/svelte/icons/info'
  import { Button, DataTable, EmptyState, IconButton, Page, StatusChip, formatBytes, type Column } from '../../lib/ui'
  import { t, tx } from '../../lib/i18n'
  import { ApprovalService, Ev, errorText, onAll, type ApprovalMode, type RequestRow } from '../../lib/api'
  import { alert, toast } from '../../lib/stores/ui'
  import { KIND_TONE } from '../files/store'

  let rows = $state<RequestRow[]>([])
  let selected = $state(new Set<string>())
  let loading = $state(false)
  let sending = $state(false)
  let loadError = $state('')
  let mode = $state<ApprovalMode | null>(null)

  const columns: Column<RequestRow>[] = $derived([
    { key: 'name', title: t('approval.request.col.name'), width: 320, grow: true, cell: nameCell },
    { key: 'size', title: t('approval.request.col.size'), width: 96, align: 'right', value: (r) => r.size, cell: sizeCell },
    { key: 'state', title: t('approval.request.col.state'), width: 150, value: (r) => r.block || r.stateText, cell: stateCell },
    { key: 'cause', title: t('approval.request.col.cause'), width: 200 },
  ])

  async function refresh() {
    if (loading) return
    loading = true
    try {
      ;[rows, mode] = await Promise.all([ApprovalService.RequestList().then((r) => r ?? []), ApprovalService.Mode()])
      loadError = ''
    } catch (e) {
      loadError = errorText(e)
    } finally {
      loading = false
    }
  }

  async function request() {
    const keys = [...selected]
    if (!keys.length) return toast(t('approval.request.select'), 2)
    const picked = rows.filter((r) => selected.has(r.serverName))
    const clips = picked.filter((r) => r.clip).length
    if (clips && clips < picked.length) return void alert({ text: t('approval.request.mixed') })
    if (!picked.some((r) => r.canRequest)) return toast(t('approval.request.none'), 2)
    sending = true
    try {
      const r = await ApprovalService.RequestApproval({ serverNames: keys })
      if (!r || r.canceled) return
      if (r.fails?.length) {
        void alert({ title: t('approval.request.failTitle'), text: `${t('approval.request.fail')}\n\n${r.fails.join('\n')}` })
      }
      if (r.sent) toast(t('approval.request.doneN', { n: r.sent }), 1)
      selected = new Set()
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      sending = false
      void refresh()
    }
  }

  onMount(() => {
    void refresh()
    return onAll({ [Ev.serverFilesChanged]: () => void refresh() })
  })
</script>

{#snippet nameCell(r: RequestRow)}
  <span class="name">
    {#if r.clip}<ClipboardIcon size={15} aria-hidden="true" />{:else}<FileIcon size={15} aria-hidden="true" />{/if}
    <span class="ellipsis">{r.name}</span>
  </span>
{/snippet}
{#snippet sizeCell(r: RequestRow)}
  <span class="num">{formatBytes(r.size)}</span>
{/snippet}
{#snippet stateCell(r: RequestRow)}
  <StatusChip tone={KIND_TONE[r.statusKind] ?? 'neutral'} label={tx(r.block || r.stateText)} />
{/snippet}

<Page
  title={t('approval.request.title')}
  subtitle={selected.size ? t('common.selected', { n: selected.size }) : t('approval.request.subtitle')}
>
  {#snippet actions()}
    <Button variant="primary" icon={Send} loading={sending} disabled={!selected.size} onclick={request}>{t('approval.request.button')}</Button>
    <IconButton icon={RefreshCw} label={t('common.refresh')} {loading} onclick={refresh} />
  {/snippet}

  {#if mode?.preMode}
    <p class="note"><Info size={15} aria-hidden="true" />{t('approval.request.preMode')}</p>
  {/if}
  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {/if}
  <div class="table">
    <DataTable
      id="requestTable"
      label={t('approval.request.table')}
      {rows}
      {columns}
      rowKey={(r) => r.serverName}
      bind:selected
      disabled={(r) => (r.canRequest ? false : tx(r.block || r.stateText))}
      storageKey="approvalRequest"
      {loading}
      onActivate={() => void request()}
    >
      {#snippet empty()}
        <EmptyState compact icon={CloudOff} title={t('approval.request.empty')} description={t('approval.request.emptyHint')} />
      {/snippet}
    </DataTable>
  </div>
</Page>

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
  .note,
  .error {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    margin: 0 0 var(--sp-2);
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r-md);
    font-size: var(--fs-sm);
  }
  .note {
    border: 1px solid var(--color-info-border);
    background: var(--color-info-subtle);
    color: var(--color-info);
  }
  .error {
    border: 1px solid var(--color-danger-border);
    background: var(--color-danger-subtle);
    color: var(--color-danger);
  }
</style>
