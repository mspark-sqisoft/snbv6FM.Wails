<!--
  [전송내역]·[결재내역] 공용 화면(docs/port/04 5.7, HIST-002). kind 로 이름·조회를 가른다.
  - 열면 오늘−7일 ~ 오늘로 한 번 조회한다. 날짜를 바꾸면 곧바로 다시 조회한다(시작일 > 종료일이면 막는다).
  - 단어 거르기는 서버에 다시 묻지 않고 받은 행에서 즉시 거른다(대소문자 없이 포함).
  - 열 때 [요청일시] 내림차순, 머리글로 정렬. 1만 건도 표는 보이는 행만 그린다.
  - [엑셀저장]은 화면에 보이는 순서(정렬·거르기) 그대로, 제목에는 마지막으로 조회한 기간·적용한 단어.
  - 늦게 온 앞 조회 결과는 요청 번호로 버린다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import Search from '@lucide/svelte/icons/search'
  import FileSpreadsheet from '@lucide/svelte/icons/file-spreadsheet'
  import Inbox from '@lucide/svelte/icons/inbox'
  import CloudOff from '@lucide/svelte/icons/cloud-off'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import { Button, DataTable, EmptyState, IconButton, Input, Page, Select, type Column, type Option } from '../../lib/ui'
  import { t, tx } from '../../lib/i18n'
  import { HistoryService, errorText, type HistoryRow } from '../../lib/api'
  import { alert, toast } from '../../lib/stores/ui'

  let { kind }: { kind: 'transfer' | 'approval' } = $props()

  type Row = HistoryRow & { no: number }

  const screen = $derived(kind === 'transfer' ? t('common.menu.transHistory') : t('common.menu.approvalHistory'))
  // 엑셀 제목·파일 이름은 원본처럼 한국어 화면 이름으로 쓴다
  const screenKo = $derived(kind === 'transfer' ? '전송내역' : '결재내역')

  const ymd = (d: Date) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  const today = new Date()
  const weekAgo = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 7)

  let from = $state(ymd(weekAgo))
  let to = $state(ymd(today))
  let field = $state<keyof HistoryRow>('fileName')
  let word = $state('')
  let rows = $state.raw<Row[]>([])
  let loading = $state(false)
  let loadError = $state('')
  let queried = $state({ from: '', to: '' })
  let sortKey = $state<string | undefined>('requestTime')
  let sortDir = $state<'asc' | 'desc'>('desc')
  let exporting = $state(false)
  let seq = 0

  const dash = (v: string) => (v === '-' ? '' : v)

  const fields: Option<string>[] = $derived(
    (
      [
        ['fileName', 'col.fileName'],
        ['fileSizeText', 'col.fileSize'],
        ['requester', 'col.requester'],
        ['requestTime', 'col.requestTime'],
        ['requestCause', 'col.requestCause'],
        ['courseText', 'col.course'],
        ['aprvStateText', 'col.aprvState'],
        ['aprvTypeText', 'col.aprvType'],
        ['approver', 'col.approver'],
        ['aprvTime', 'col.aprvTime'],
        ['aprvCause', 'col.aprvCause'],
        ['downTime', 'col.downTime'],
      ] as const
    ).map(([value, key]) => ({ value, label: t(`history.${key}`) })),
  )

  const columns: Column<Row>[] = $derived([
    { key: 'no', title: t('history.col.no'), width: 50, align: 'right', value: (r) => r.no },
    { key: 'dept', title: t('history.col.dept'), width: 100 },
    { key: 'fileName', title: t('history.col.fileName'), width: 300 },
    { key: 'fileSize', title: t('history.col.fileSize'), width: 100, align: 'right', value: (r) => r.fileSize, cell: sizeCell },
    { key: 'requester', title: t('history.col.requester'), width: 100 },
    { key: 'requestTime', title: t('history.col.requestTime'), width: 165 },
    { key: 'requestCause', title: t('history.col.requestCause'), width: 200 },
    { key: 'courseText', title: t('history.col.course'), width: 100 },
    { key: 'aprvState', title: t('history.col.aprvState'), width: 100, value: (r) => r.aprvStateText, cell: stateCell },
    { key: 'aprvTypeText', title: t('history.col.aprvType'), width: 100, value: (r) => tx(r.aprvTypeText) },
    { key: 'approver', title: t('history.col.approver'), width: 100, value: approverText },
    { key: 'aprvTime', title: t('history.col.aprvTime'), width: 165, value: (r) => dash(r.aprvTime) },
    { key: 'aprvCause', title: t('history.col.aprvCause'), width: 200, value: (r) => dash(r.aprvCause) },
    { key: 'downTime', title: t('history.col.downTime'), width: 165, grow: true },
  ])

  function approverText(r: Row) {
    const name = dash(r.approver)
    return r.actApprover ? t('history.actual', { name, act: r.actApprover }) : name
  }

  const shown = $derived.by(() => {
    const q = word.trim().toLowerCase()
    if (!q) return rows
    const k = field
    return rows.filter((r) => String(r[k] ?? '').toLowerCase().includes(q))
  })

  async function query() {
    if (from > to) {
      toast(t('history.badRange'), 2)
      return
    }
    const my = ++seq
    const q = { from, to }
    loading = true
    try {
      const list =
        (kind === 'transfer' ? await HistoryService.TransferHistory(q.from, q.to) : await HistoryService.ApprovalHistory(q.from, q.to)) ??
        []
      if (my !== seq) return
      rows = list.map((r, i) => ({ ...r, no: i + 1 }))
      queried = q
      loadError = ''
      sortKey = 'requestTime'
      sortDir = 'desc'
    } catch (e) {
      if (my !== seq) return
      loadError = errorText(e)
      rows = []
    } finally {
      if (my === seq) loading = false
    }
  }

  // 화면과 같은 순서로 엑셀에 쓴다(DataTable 의 정렬 규칙과 같다)
  const collator = new Intl.Collator('ko', { numeric: true, sensitivity: 'base' })
  function visibleOrder(): Row[] {
    const col = columns.find((c) => c.key === sortKey)
    if (!col) return shown
    const val = (r: Row) => (col.value ? col.value(r) : (r as unknown as Record<string, unknown>)[col.key])
    const dir = sortDir === 'desc' ? -1 : 1
    return shown.slice().sort((a, b) => {
      const x = val(a)
      const y = val(b)
      if (x == null || x === '') return y == null || y === '' ? 0 : 1
      if (y == null || y === '') return -1
      return dir * (typeof x === 'number' && typeof y === 'number' ? x - y : collator.compare(String(x), String(y)))
    })
  }

  async function exportExcel() {
    if (!shown.length) {
      toast(t('history.exportEmpty'), 2)
      return
    }
    exporting = true
    try {
      const path = await HistoryService.ExportExcel({
        screen: screenKo,
        from: queried.from,
        to: queried.to,
        word: word.trim(),
        rows: visibleOrder(),
      })
      if (path) await alert({ text: t('history.saved', { path }) })
    } catch (e) {
      await alert({ text: errorText(e) })
    } finally {
      exporting = false
    }
  }

  onMount(() => {
    void query()
  })
</script>

{#snippet sizeCell(r: Row)}
  <span class="num">{r.fileSizeText}</span>
{/snippet}
{#snippet stateCell(r: Row)}
  <span class="aprv-state" class:ok={r.aprvState === 1} class:bad={r.aprvState === 2}>{tx(r.aprvStateText)}</span>
{/snippet}

<Page
  title={screen}
  subtitle={word.trim() ? t('history.filtered', { n: shown.length, total: rows.length }) : t('history.count', { n: rows.length })}
>
  {#snippet actions()}
    <IconButton icon={FileSpreadsheet} label={t('history.excel')} loading={exporting} disabled={loading} onclick={exportExcel} />
  {/snippet}
  {#snippet toolbar()}
    <div class="bar">
      <div class="date">
        <Input size="sm" type="date" bind:value={from} max={to} aria-label={t('history.from')} onchange={query} />
      </div>
      <span class="tilde" aria-hidden="true">~</span>
      <div class="date">
        <Input size="sm" type="date" bind:value={to} min={from} aria-label={t('history.to')} onchange={query} />
      </div>
      <div class="field">
        <Select size="sm" options={fields} bind:value={field} id="historySearchTypeCombo" aria-label={t('history.searchType')} />
      </div>
      <div class="word">
        <Input size="sm" type="search" icon={Search} bind:value={word} placeholder={t('history.word')} aria-label={t('history.word')} />
      </div>
      <Button size="sm" icon={Search} loading={loading} onclick={query}>{t('history.search')}</Button>
    </div>
  {/snippet}

  <div class="table">
    <DataTable
      id={kind === 'transfer' ? 'transHistoryTable' : 'approvalHistoryTable'}
      label={t(`history.table.${kind}`)}
      rows={shown}
      {columns}
      rowKey={(r) => String(r.no)}
      selectable={false}
      striped
      bind:sortKey
      bind:sortDir
      storageKey={kind === 'transfer' ? 'transHistory' : 'approvalHistory'}
      loading={loading && !rows.length}
    >
      {#snippet empty()}
        {#if loadError}
          <EmptyState compact icon={CloudOff} title={loadError}>
            <Button size="sm" icon={RefreshCw} onclick={query}>{t('common.retry')}</Button>
          </EmptyState>
        {:else if word.trim() && rows.length}
          <EmptyState compact icon={Search} title={t('history.noMatch')} />
        {:else}
          <EmptyState compact icon={Inbox} title={t('history.empty')} />
        {/if}
      {/snippet}
    </DataTable>
  </div>
</Page>

<style>
  .bar {
    flex: 1;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-2);
  }
  .date {
    width: 150px;
  }
  .field {
    width: 130px;
  }
  .word {
    width: min(240px, 100%);
  }
  .tilde {
    color: var(--color-text-3);
  }
  .table {
    flex: 1;
    min-height: 0;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .num {
    font-variant-numeric: tabular-nums;
    color: var(--color-text-2);
  }
  .ok {
    color: var(--color-success);
    font-weight: var(--fw-medium);
  }
  .bad {
    color: var(--color-danger);
    font-weight: var(--fw-medium);
  }
</style>
