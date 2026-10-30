<!--
  [파일 검사 결과] 창(원본 content/FileCheckResultDlg.qml). Go 가 policy:check-open 으로 열고 행·결과를 보낸다.
  - 결과 전: "검사가 진행중입니다." + [취소](검사를 멈추고 업로드 취소).
  - 모두 정상: 2초 뒤 스스로 FileCheckReply(gen, false)(다운로드 검사는 답하지 않는다).
  - 폴더에서 막힌 파일만 있으면([SKIP:n]) [차단파일 제외 업로드], 그 밖의 오류면 [취소] 만.
  - 결재자 다운로드 검사(mode DOWNLOAD)는 [확인] 하나, 답을 보내지 않는다.
  - Esc·바깥 누름으로 닫히지 않는다. 답 없이 닫히면 취소로 답한다(뒤 업로드가 답을 기다리며 막히지 않게).
  Host 가 검사마다 새로 붙이므로 앞 검사의 상태(자동 닫기 타이머·다운로드 모드)가 남지 않는다.
-->
<script lang="ts">
  import { Button, DataTable, Dialog, Progress, StatusChip, type Column } from '../../../lib/ui'
  import { t } from '../../../lib/i18n'
  import { PolicyService } from '../../../lib/api'
  import type { CheckOpen, CheckResult, CheckRow } from '../types'

  let {
    check,
    rows,
    done,
    total,
    current,
    result,
    open = $bindable(false),
  }: {
    check: CheckOpen
    rows: CheckRow[]
    done: number
    total: number
    current: string
    result: CheckResult | null
    open?: boolean
  } = $props()

  let replied = false
  const download = $derived(check.mode === 'DOWNLOAD' || result?.value === 'DOWNLOAD')

  type Line = CheckRow & { no: number }
  const top = $derived(rows.filter((r) => r.top).map((r, i): Line => ({ ...r, no: i + 1 })))
  const inner = $derived(rows.filter((r) => !r.top).map((r, i): Line => ({ ...r, no: i + 1 })))

  const status = $derived.by(() => {
    if (!result) return { text: t('policy.check.running'), tone: 'success' as const }
    if (result.fails === 0) return { text: t('policy.check.allOk'), tone: 'success' as const }
    if (result.skip > 0) return { text: t('policy.check.failsSkip', { n: result.fails, skip: result.skip }), tone: 'danger' as const }
    return { text: t('policy.check.fails', { n: result.fails }), tone: 'danger' as const }
  })
  const showOk = $derived(!!result && (download || result.fails === 0 || result.skip > 0))
  const showCancel = $derived(!download || !result)

  function reply(reject: boolean) {
    if (replied) return
    replied = true
    if (!download) void PolicyService.FileCheckReply(check.gen, reject).catch(() => {})
    open = false
  }

  // 모두 정상이면 결과를 2초 보여 주고 스스로 진행한다(원본 closeTimer). 창이 내려가면 타이머도 멈춘다.
  $effect(() => {
    if (!result || result.fails !== 0 || download || !open) return
    const id = setTimeout(() => reply(false), 2000)
    return () => clearTimeout(id)
  })

  function onClose() {
    if (!replied) reply(true)
  }

  const topColumns: Column<Line>[] = $derived([
    { key: 'no', title: t('policy.col.no'), width: 44, align: 'right', sortable: false },
    { key: 'name', title: t('policy.col.name'), width: 220, sortable: false },
    { key: 'result', title: t('policy.col.result'), width: 150, sortable: false, cell: resultCell },
    { key: 'detail', title: t('policy.col.detail'), width: 200, grow: true, sortable: false },
  ])
  const innerColumns: Column<Line>[] = $derived([
    { key: 'no', title: t('policy.col.no'), width: 44, align: 'right', sortable: false },
    { key: 'parent', title: t('policy.col.name'), width: 180, sortable: false },
    { key: 'name', title: t('policy.col.inner'), width: 240, sortable: false },
    { key: 'result', title: t('policy.col.result'), width: 150, sortable: false, cell: resultCell },
    { key: 'detail', title: t('policy.col.detail'), width: 200, grow: true, sortable: false },
  ])
</script>

{#snippet resultCell(r: Line)}
  <StatusChip tone={r.ok ? 'success' : 'danger'} label={r.result} title={r.detail || r.result} />
{/snippet}

<Dialog bind:open title={t('policy.check.title')} width={inner.length ? 1000 : 700} dismissible={false} {onClose}>
  <div class="body">
    <p class="status tone-{status.tone}" id="checkResultLabel" role="status" aria-live="polite">{status.text}</p>
    {#if !result}
      <div class="progress">
        <Progress value={done} max={total || 1} indeterminate={total === 0} label={t('policy.check.progressLabel')} size="sm" />
        <span class="count">{t('policy.check.progress', { done, total })}</span>
        {#if current}<span class="current" title={current}>{current}</span>{/if}
      </div>
    {/if}

    <div class="table" style:height="{Math.min(260, 40 + Math.max(top.length, 3) * 28)}px">
      <DataTable
        id="fileCheckTable"
        label={t('policy.check.table')}
        rows={top}
        columns={topColumns}
        rowKey={(r) => r.key}
        selectable={false}
        rowHeight={28}
        emptyText={t('policy.check.empty')}
      />
    </div>

    {#if inner.length}
      <h3>{t('policy.check.innerTitle')}</h3>
      <div class="table" style:height="{Math.min(260, 40 + Math.max(inner.length, 3) * 28)}px">
        <DataTable
          id="zipCheckTable"
          label={t('policy.check.innerTable')}
          rows={inner}
          columns={innerColumns}
          rowKey={(r) => r.key}
          selectable={false}
          rowHeight={28}
          emptyText={t('policy.check.empty')}
        />
      </div>
    {/if}
  </div>

  {#snippet footer()}
    {#if showOk}
      <Button id="okButton" variant="primary" onclick={() => reply(false)}>
        {result && result.skip > 0 && !download ? t('policy.check.skipUpload') : t('policy.check.ok')}
      </Button>
    {/if}
    {#if showCancel}
      <Button id="cancelButton" onclick={() => reply(true)}>{t('policy.check.cancel')}</Button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    color: var(--color-text);
  }
  .status {
    margin: 0;
    font-size: var(--fs-body);
    font-weight: var(--fw-medium);
  }
  .tone-success {
    color: var(--color-success);
  }
  .tone-danger {
    color: var(--color-danger);
  }
  .progress {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: var(--sp-1) var(--sp-2);
    align-items: center;
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  .count {
    font-variant-numeric: tabular-nums;
  }
  .current {
    grid-column: 1 / -1;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--color-text-3);
  }
  .table {
    min-height: 120px;
  }
  h3 {
    margin: var(--sp-1) 0 0;
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
  }
</style>
