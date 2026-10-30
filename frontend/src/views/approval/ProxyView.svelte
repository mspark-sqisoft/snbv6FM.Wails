<!--
  [대리결재자] 화면(원본 content/ProxyApproverScrollView.qml, docs/port/06 5.5).
  위: 등록된 대리결재자[3174](이름·기간·[삭제]). 아래: 후보 표(결재자 목록[3370] 에서 나를 뺀 것)에서 골라 기간·사유를 넣고 [등록][3272].
  막는 규칙(이미 있음·미선택·사유 없음·기간 초과·끝<시작)은 Go 가 판정하고, 화면은 날짜 입력 min/max 로 미리 막는다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import UserCheck from '@lucide/svelte/icons/user-check'
  import UserX from '@lucide/svelte/icons/user-x'
  import Trash2 from '@lucide/svelte/icons/trash-2'
  import Users from '@lucide/svelte/icons/users'
  import { Button, DataTable, EmptyState, Input, Page, Spinner, type Column } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { ApprovalService, errorText, type ApproverUser, type ProxyState } from '../../lib/api'
  import { alert, confirm } from '../../lib/stores/ui'

  let info = $state<ProxyState | null>(null)
  let loading = $state(false)
  let loadError = $state('')
  let selected = $state(new Set<string>())
  let sortKey = $state<string | undefined>('name')
  let sortDir = $state<'asc' | 'desc'>('asc')
  let saving = $state(false)
  let deleting = $state(false)

  const today = () => new Date(Date.now() - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 10)
  const addDays = (d: string, n: number) => {
    const x = new Date(`${d}T00:00:00`)
    x.setDate(x.getDate() + n)
    return new Date(x.getTime() - x.getTimezoneOffset() * 60000).toISOString().slice(0, 10)
  }
  let from = $state(today())
  let to = $state(addDays(today(), 4)) // 원본 종료 기본값 = 오늘 + 4일
  let cause = $state('')

  const rows = $derived((info?.candidates ?? []) as ApproverUser[])
  const chosen = $derived(rows.find((r) => selected.has(r.id)) ?? null)
  const maxDays = $derived(info?.maxDays ?? 5)
  const maxTo = $derived(addDays(from, maxDays - 1))
  const indexOf = $derived(new Map(rows.map((r, i) => [r.id, i + 1])))

  const columns: Column<ApproverUser>[] = $derived([
    { key: 'no', title: '#', width: 50, align: 'right', sortable: false, value: (r) => indexOf.get(r.id) },
    { key: 'id', title: t('approval.proxy.col.id'), width: 120 },
    { key: 'name', title: t('approval.proxy.col.name'), width: 140, cell: nameCell },
    { key: 'dept', title: t('approval.proxy.col.dept'), width: 200, grow: true },
    { key: 'rank', title: t('approval.proxy.col.rank'), width: 100 },
    { key: 'duty', title: t('approval.proxy.col.duty'), width: 100 },
  ])

  async function refresh() {
    if (loading) return
    loading = true
    try {
      info = await ApprovalService.Proxy()
      loadError = ''
    } catch (e) {
      loadError = errorText(e)
    } finally {
      loading = false
    }
  }

  function onFrom() {
    if (to < from) to = from
    if (to > maxTo) to = maxTo
  }

  async function register() {
    saving = true
    try {
      await ApprovalService.RegisterProxy({
        id: chosen?.id ?? '',
        name: chosen?.name ?? '',
        dept: chosen?.dept ?? '',
        from,
        to,
        cause,
      })
      await alert({ text: t('approval.proxy.registered') })
      cause = ''
      selected = new Set()
    } catch (e) {
      await alert({ text: errorText(e) })
    } finally {
      saving = false
      void refresh()
    }
  }

  async function remove() {
    const cur = info?.current
    if (!cur) return void alert({ text: t('approval.proxy.none') })
    if (!(await confirm({ text: t('approval.proxy.deleteConfirm', { name: cur.name }), danger: true, okLabel: t('approval.proxy.delete') })))
      return
    deleting = true
    try {
      await ApprovalService.DeleteProxy()
      await alert({ text: t('approval.proxy.deleted') })
    } catch (e) {
      await alert({ text: errorText(e) })
    } finally {
      deleting = false
      void refresh()
    }
  }

  onMount(() => void refresh())
</script>

{#snippet nameCell(r: ApproverUser)}
  <span class="ellipsis" id="{r.id}.nameCell">{r.name}</span>
{/snippet}

<Page title={t('common.menu.proxy')}>
  {#snippet actions()}
    <Button icon={RefreshCw} {loading} onclick={refresh}>{t('approval.proxy.search')}</Button>
  {/snippet}

  <div class="wrap">
    {#if loadError}<p class="error" role="alert">{loadError}</p>{/if}

    <section class="card" aria-labelledby="proxyCurrentTitle">
      <h2 id="proxyCurrentTitle">{t('approval.proxy.current')}</h2>
      {#if !info && loading}
        <Spinner size={18} label={t('common.loading')} />
      {:else if info?.current}
        <div class="current">
          <UserCheck size={18} aria-hidden="true" />
          <strong>{info.current.name}</strong>
          <span class="muted">({info.current.id})</span>
          <span class="period"><span class="muted">{t('approval.proxy.period')}</span> {info.current.from} ~ {info.current.to}</span>
          <Button size="sm" variant="danger" icon={Trash2} loading={deleting} onclick={remove}>{t('approval.proxy.delete')}</Button>
        </div>
      {:else}
        <div class="current muted"><UserX size={18} aria-hidden="true" />{t('approval.proxy.none')}</div>
      {/if}
    </section>

    <section class="card grow" aria-labelledby="proxyRegisterTitle">
      <h2 id="proxyRegisterTitle">{t('approval.proxy.register')}</h2>
      <div class="form">
        <Input id="nameField" size="sm" value={chosen ? `${chosen.name} (${chosen.id})` : ''} readonly placeholder={t('approval.proxy.name')} label={t('approval.proxy.name')} />
        <label class="date">
          <span>{t('approval.proxy.from')}</span>
          <input type="date" bind:value={from} min={today()} onchange={onFrom} />
        </label>
        <span class="tilde" aria-hidden="true">~</span>
        <label class="date">
          <span>{t('approval.proxy.to')}</span>
          <input type="date" bind:value={to} min={from} max={maxTo} />
        </label>
        <span class="muted hint">{t('approval.proxy.maxDays', { n: maxDays })}</span>
        <Input id="causeField" size="sm" bind:value={cause} maxlength={100} placeholder={t('approval.proxy.cause')} label={t('approval.proxy.cause')} />
        <Button variant="primary" loading={saving} disabled={!!info?.current} onclick={register}>{t('approval.proxy.registerButton')}</Button>
      </div>
      <div class="table">
        <DataTable
          id="proxyApproverTable"
          label={t('approval.proxy.table')}
          {rows}
          {columns}
          rowKey={(r) => r.id}
          multiple={false}
          bind:selected
          bind:sortKey
          bind:sortDir
          storageKey="proxyApprover"
          loading={loading && !info}
        >
          {#snippet empty()}
            <EmptyState compact icon={Users} title={t('approval.proxy.empty')} />
          {/snippet}
        </DataTable>
      </div>
    </section>
  </div>
</Page>

<style>
  .wrap {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    height: 100%;
    min-height: 0;
  }
  .card {
    padding: var(--sp-3) var(--sp-4);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
  }
  .card.grow {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  h2 {
    margin: 0 0 var(--sp-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
    color: var(--color-text-2);
  }
  .card.grow h2 {
    margin: 0;
  }
  .current {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    flex-wrap: wrap;
  }
  .period {
    margin: 0 var(--sp-2);
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--color-text-3);
  }
  .form {
    display: flex;
    align-items: flex-end;
    gap: var(--sp-2);
    flex-wrap: wrap;
  }
  .date {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  .date input {
    height: var(--h-sm);
    padding: 0 var(--sp-2);
    border: 1px solid var(--color-border);
    border-radius: var(--r-sm);
    background: var(--color-bg);
    color: var(--color-text);
    font: inherit;
  }
  .tilde {
    padding-bottom: 6px;
    color: var(--color-text-3);
  }
  .hint {
    padding-bottom: 6px;
    font-size: var(--fs-sm);
  }
  .table {
    flex: 1;
    min-height: 120px;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .error {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--color-danger-border);
    border-radius: var(--r-md);
    background: var(--color-danger-subtle);
    color: var(--color-danger);
    font-size: var(--fs-sm);
  }
</style>
