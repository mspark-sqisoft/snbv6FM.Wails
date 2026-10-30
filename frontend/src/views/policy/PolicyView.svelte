<!--
  [정책] 화면(원본 content/PolicyScrollView.qml ― 메뉴에서는 숨김). 받아 둔 정책을 유형·단어로 걸러 보인다(PolicyService.PolicyList).
  유형: Profile(권한 프로파일) · LinkRedirect(URL 필터 ― 링크 담당이 받는 값이라 비어 있다) · Extension(헤더 패턴 3015) · ExtProfile(확장자 3005).
  조회 시점: 화면을 열 때, 유형을 바꿀 때, 단어 칸 Enter, [조회], 정책을 새로 받았을 때(policy:changed).
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Search from '@lucide/svelte/icons/search'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import SearchX from '@lucide/svelte/icons/search-x'
  import AlertTriangle from '@lucide/svelte/icons/triangle-alert'
  import { Button, DataTable, EmptyState, Input, Page, Select, type Column, type Option } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { Ev, PolicyService, errorText, on, type PolicyRow, type PolicyStatus } from '../../lib/api'

  const KINDS = ['Profile', 'LinkRedirect', 'Extension', 'ExtProfile']

  let kind = $state('Profile')
  let word = $state('')
  let rows = $state<PolicyRow[]>([])
  let status = $state<PolicyStatus | null>(null)
  let loading = $state(false)
  let reloading = $state(false)
  let error = $state('')
  let sortKey = $state('name')
  let sortDir = $state<'asc' | 'desc'>('asc')

  type Line = PolicyRow & { no: number }
  const lines = $derived(rows.map((r, i): Line => ({ ...r, no: i + 1 })))
  const kindOptions: Option<string>[] = $derived(KINDS.map((k) => ({ value: k, label: t(`policy.kind.${k}`) })))
  const columns: Column<Line>[] = $derived([
    { key: 'no', title: t('policy.col.no'), width: 56, align: 'right', sort: (a, b) => a.no - b.no },
    { key: 'type', title: t('policy.col.type'), width: 110 },
    { key: 'name', title: t('policy.col.pname'), width: 220 },
    { key: 'value', title: t('policy.col.value'), width: 260 },
    { key: 'description', title: t('policy.col.desc'), width: 320, grow: true },
  ])
  const subtitle = $derived.by(() => {
    if (!status) return undefined
    if (!status.ready) return t('policy.view.notReady')
    return t('policy.view.summary', {
      method: status.method,
      patt: status.pattStored,
      ext: status.extRows,
      zip: status.insideZip ? t('policy.view.on') : t('policy.view.off'),
    })
  })

  let seq = 0
  async function search() {
    const my = ++seq // 늦게 온 앞 조회 결과는 버린다
    loading = true
    error = ''
    try {
      const [r, s] = await Promise.all([PolicyService.PolicyList(kind, word.trim()), PolicyService.Status()])
      if (my !== seq) return
      rows = r ?? []
      status = s
    } catch (e) {
      if (my !== seq) return
      rows = []
      error = errorText(e)
    } finally {
      if (my === seq) loading = false
    }
  }

  async function reload() {
    reloading = true
    error = ''
    try {
      await PolicyService.Reload() // 끝나면 policy:changed 가 와서 다시 조회한다
    } catch (e) {
      error = errorText(e)
    } finally {
      reloading = false
    }
  }

  // 유형을 바꾸면 조회(단어는 Enter·[조회] 때만). 처음 한 번은 화면을 열 때의 조회다.
  $effect(() => {
    void kind
    untrack(() => void search())
  })

  onMount(() => on(Ev.policyChanged, () => void search()))
</script>

{#snippet emptyState()}
  {#if error}
    <EmptyState icon={AlertTriangle} title={error} />
  {:else}
    <EmptyState icon={SearchX} title={t('policy.view.empty')} />
  {/if}
{/snippet}

<Page title={t('common.menu.policy')} {subtitle} padded={false}>
  {#snippet actions()}
    <Button id="policyReloadButton" icon={RefreshCw} loading={reloading} onclick={reload}>{t('policy.view.reload')}</Button>
  {/snippet}
  {#snippet toolbar()}
    <div class="filters" role="search">
      <Select
        id="policySearchTypeCombo"
        size="sm"
        aria-label={t('policy.view.kind')}
        options={kindOptions}
        bind:value={kind}
      />
      <div class="word">
        <Input
          size="sm"
          icon={Search}
          type="search"
          aria-label={t('policy.view.word')}
          placeholder={t('policy.view.word')}
          bind:value={word}
          onkeydown={(e: KeyboardEvent) => {
            if (e.key === 'Enter' && !e.isComposing) {
              e.preventDefault()
              void search()
            }
          }}
        />
      </div>
      <Button size="sm" variant="primary" loading={loading} onclick={search}>{t('policy.view.search')}</Button>
    </div>
  {/snippet}

  <DataTable
    id="policyTable"
    label={t('policy.view.table')}
    rows={lines}
    {columns}
    rowKey={(r) => `${r.type}\u0000${r.name}\u0000${r.no}`}
    selectable={false}
    bind:sortKey
    bind:sortDir
    storageKey="policy"
    {loading}
    striped
    empty={emptyState}
  />
</Page>

<style>
  .filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-2);
  }
  .word {
    flex: 1 1 220px;
    min-width: 160px;
  }
</style>
