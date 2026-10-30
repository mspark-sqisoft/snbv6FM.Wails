<!--
  [로그] 화면: 그날 앱 로그(<데이터 폴더>/log/yyyyMMdd.v2.log)를 날짜·수준·분류·단어로 걸러 보인다(docs/port/09 LOG-038~042).
  거르기는 Go 가 파일을 한 번 훑으며 한다(LogService.Search) ― 큰 파일도 화면은 결과만 받는다. 표는 가상 스크롤.
  조회 시점: 화면을 열 때, 조건(날짜·수준·이상·분류)을 바꿀 때, 단어 칸 Enter, [조회]. 원본과 달리 날짜를 바꿔도 곧바로 조회한다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import Search from '@lucide/svelte/icons/search'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import FileX from '@lucide/svelte/icons/file-x'
  import SearchX from '@lucide/svelte/icons/search-x'
  import AlertTriangle from '@lucide/svelte/icons/triangle-alert'
  import { Badge, Button, Checkbox, DataTable, EmptyState, Input, Page, Select, type Column, type Option, type Tone } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { LogService, errorText, type LogRow } from '../../lib/api'
  import { alert } from '../../lib/stores/ui'

  const CATEGORIES = [
    'snb.app',
    'snb.auth',
    'snb.transfer',
    'snb.transfer.upload',
    'snb.transfer.download',
    'snb.transfer.clipboard',
    'snb.transfer.auto',
    'snb.approval',
    'snb.check',
    'snb.settings',
    'snb.grpc',
    'snb.ui',
    'default',
    'qml',
  ]
  const LEVEL_TONE: Record<string, Tone> = { DEBUG: 'neutral', INFO: 'info', WARN: 'warning', ERROR: 'danger', FATAL: 'danger' }

  function today() {
    const d = new Date()
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
  }

  let date = $state(today())
  let level = $state('INFO') // 원본 기본: INFO 만
  let atLeast = $state(false)
  let category = $state('')
  let word = $state('')
  let rows = $state.raw<LogRow[]>([])
  let summary = $state({ total: 0, matched: 0, ms: 0, truncated: false, missing: false })
  let loading = $state(false)
  let error = $state('')
  let fileOn = $state(true)
  let selected = $state(new Set<string>())
  let sortKey = $state('time')
  let sortDir = $state<'asc' | 'desc'>('desc') // 처음은 시간 내림차순(최근 줄이 위)

  const levelOptions: Option<string>[] = $derived([
    { value: '', label: t('log.level.all') },
    ...['DEBUG', 'INFO', 'WARN', 'ERROR'].map((v) => ({ value: v, label: v })),
  ])
  const categoryOptions: Option<string>[] = $derived([
    { value: '', label: t('log.category.all') },
    ...CATEGORIES.map((v) => ({ value: v, label: v })),
  ])

  const byNo = (a: LogRow, b: LogRow) => a.no - b.no // 파일 순서 = 시간 순서(날짜 하루치)
  const columns: Column<LogRow>[] = $derived([
    { key: 'no', title: t('log.col.no'), width: 64, align: 'right', sort: byNo, cell: noCell },
    { key: 'time', title: t('log.col.time'), width: 110, sort: byNo, cell: monoTime },
    { key: 'level', title: t('log.col.level'), width: 76, cell: levelCell },
    { key: 'account', title: t('log.col.account'), width: 130 },
    { key: 'category', title: t('log.col.category'), width: 170 },
    { key: 'message', title: t('log.col.message'), width: 600, grow: true },
    { key: 'file', title: t('log.col.file'), width: 200 },
  ])

  let seq = 0
  async function search() {
    const my = ++seq // 늦게 온 앞 조회 결과는 버린다
    loading = true
    error = ''
    try {
      const lv = level && atLeast ? `${level}+` : level
      const [r, on] = await Promise.all([LogService.Search(date, lv, category, word.trim()), LogService.LogFileEnabled()])
      if (my !== seq) return
      rows = r?.rows ?? []
      summary = {
        total: r?.total ?? 0,
        matched: r?.matched ?? 0,
        ms: r?.ms ?? 0,
        truncated: !!r?.truncated,
        missing: !!r?.missing,
      }
      fileOn = on
      selected = new Set() // 다시 조회하면 강조가 풀린다(원본)
    } catch (e) {
      if (my !== seq) return
      rows = []
      error = errorText(e)
    } finally {
      if (my === seq) loading = false
    }
  }

  // 조건이 바뀌면 조회(단어는 Enter·[조회] 때만)
  let ready = false
  $effect(() => {
    void [date, level, atLeast, category]
    if (ready) void search()
  })

  function showRow(r: LogRow) {
    void alert({
      title: t('log.detail', { time: r.time }),
      text: `${r.level}  ${r.account}  ${r.category}  ${r.file}\n\n${r.message.replaceAll('\\n', '\n')}`,
    })
  }

  onMount(() => {
    ready = true
    void search()
  })
</script>

{#snippet noCell(r: LogRow)}
  <span class="num">{r.no}</span>
{/snippet}
{#snippet monoTime(r: LogRow)}
  <span class="num">{r.time}</span>
{/snippet}
{#snippet levelCell(r: LogRow)}
  <Badge size="sm" tone={LEVEL_TONE[r.level] ?? 'neutral'} label={r.level} />
{/snippet}
{#snippet emptyState()}
  {#if error}
    <EmptyState icon={AlertTriangle} title={t('log.failed')} description={error} />
  {:else if summary.missing}
    <EmptyState icon={FileX} title={t('log.missing')} description={fileOn ? undefined : t('log.missingOff')} />
  {:else}
    <EmptyState icon={SearchX} title={t('log.empty')} description={t('log.emptyHint')} />
  {/if}
{/snippet}

<Page
  title={t('common.menu.log')}
  subtitle={summary.missing || error ? undefined : t('log.summary', { total: summary.total, matched: summary.matched, ms: summary.ms })}
  padded={false}
>
  {#snippet actions()}
    <Button icon={RefreshCw} loading={loading} onclick={search}>{t('log.search')}</Button>
  {/snippet}
  {#snippet toolbar()}
    <div class="filters" role="search">
      <div class="date"><Input type="date" size="sm" aria-label={t('log.date')} bind:value={date} required /></div>
      <Select id="logTypeCombo" size="sm" aria-label={t('log.level')} options={levelOptions} bind:value={level} />
      <Checkbox label={t('log.atLeast')} bind:checked={atLeast} disabled={!level} />
      <Select id="logCategoryCombo" size="sm" aria-label={t('log.category')} options={categoryOptions} bind:value={category} />
      <div class="word">
        <Input
          size="sm"
          icon={Search}
          type="search"
          aria-label={t('log.word')}
          placeholder={t('log.wordPlaceholder')}
          bind:value={word}
          onkeydown={(e: KeyboardEvent) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              void search()
            }
          }}
        />
      </div>
    </div>
    {#if summary.truncated}
      <p class="note" role="status">{t('log.truncated', { n: rows.length.toLocaleString() })}</p>
    {/if}
  {/snippet}

  <DataTable
    id="logTable"
    label={t('log.table')}
    {rows}
    {columns}
    rowKey={(r) => String(r.no)}
    multiple={false}
    bind:selected
    bind:sortKey
    bind:sortDir
    storageKey="log"
    {loading}
    striped
    empty={emptyState}
    onActivate={showRow}
  />
</Page>

<style>
  .filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-2);
  }
  .date {
    width: 150px;
  }
  .word {
    flex: 1 1 220px;
    min-width: 160px;
  }
  .note {
    margin: var(--sp-1) 0 0;
    color: var(--color-warning);
    font-size: var(--fs-caption);
  }
  .num {
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
  }
</style>
