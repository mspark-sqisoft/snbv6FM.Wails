<!--
  전송 상태 서랍(원본 TransferProgressView.qml·TransferDrawer, docs/port/03 5.5). Shell 이 본문 아래에 늘 둔다.
  - 행은 Host 가 Go 사건(transfer:state·transfer:progress, 200ms 묶음)으로 채운 store 를 그린다 ― 1만 행도 가상 스크롤.
  - 머리줄: 전체 진행률·건수 요약, [목록 지우기](끝난 행만 지운다 ― 진행 중인 전송은 계속된다), [모두 취소].
  - 행마다 [취소](대기·진행 모두. 올리던 것은 서버가, 받던 것은 앱이 지운다), 실패·취소 사유는 상태 칸 툴팁.
  - 높이는 끌어서 바꾸고 설정 transferDrawerHeight 에 저장한다. 닫을 때 끝난 행을 지운다.
-->
<script lang="ts">
  import { untrack } from 'svelte'
  import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import FolderIcon from '@lucide/svelte/icons/folder'
  import X from '@lucide/svelte/icons/x'
  import ListX from '@lucide/svelte/icons/list-x'
  import { Button, DataTable, Drawer, EmptyState, IconButton, Progress, StatusChip, Tooltip, formatBytes, type Column, type Tone } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { TransferService, errorText } from '../../lib/api'
  import { confirm, settings, toast, updateSettings } from '../../lib/stores/ui'
  import { drawerOpen, items, live, statusText, summary as counts, type Item } from './store'

  let height = $state(untrack(() => $settings?.transferDrawerHeight || 220))
  let synced = false
  $effect(() => {
    const h = $settings?.transferDrawerHeight
    if (h && !synced) {
      height = h
      synced = true
    }
  })

  // 닫을 때 끝난 행을 지운다(원본: 서랍을 닫으면 완료 행 정리)
  let wasOpen = false
  $effect(() => {
    const o = $drawerOpen
    if (wasOpen && !o && $items.some((it) => !live(it))) TransferService.Clear().catch(() => {})
    wasOpen = o
  })

  const TONE: Record<string, Tone> = { done: 'success', failed: 'danger', cancelled: 'neutral', pending: 'info', active: 'accent' }

  function eta(it: Item): string {
    if (it.state !== 'active' || !it.speed || !(it.phase === 'send' || it.phase === 'recv')) return ''
    const left = Math.max(0, (it.total || it.size) - it.done) / it.speed
    const m = Math.floor(left / 60)
    const s = Math.round(left % 60)
    return `${formatBytes(it.speed)}/s · ${m}:${String(s).padStart(2, '0')}`
  }

  const columns: Column<Item>[] = $derived([
    { key: 'id', title: '#', width: 50, align: 'right', value: (r) => r.id },
    { key: 'name', title: t('transfer.col.name'), width: 220, grow: true, cell: nameCell },
    { key: 'state', title: t('transfer.col.state'), width: 220, value: statusText, cell: stateCell, sortable: false },
    { key: 'size', title: t('transfer.col.size'), width: 90, align: 'right', value: (r) => r.total || r.size, cell: sizeCell },
    { key: 'dir', title: t('transfer.col.dir'), width: 80, value: (r) => (r.dir === 'upload' ? t('transfer.dir.upload') : t('transfer.dir.download')) },
    { key: 'speed', title: t('transfer.col.speed'), width: 140, value: eta, sortable: false },
    { key: 'act', title: t('transfer.col.action'), width: 60, align: 'center', sortable: false, cell: actCell },
  ])

  async function cancel(it: Item) {
    try {
      await TransferService.Cancel(it.id)
    } catch (e) {
      toast(errorText(e), 3)
    }
  }

  async function cancelAll() {
    const yes = await confirm({ title: t('transfer.cancelAll'), text: t('transfer.cancelAllConfirm'), okLabel: t('transfer.cancelAll'), danger: true })
    if (yes) TransferService.CancelAll().catch((e) => toast(errorText(e), 3))
  }

  const overall = $derived($counts.total > 0 ? Math.floor(($counts.bytes / $counts.total) * 100) : 0)
  const running = $derived($counts.pending + $counts.active)
</script>

{#snippet nameCell(r: Item)}
  <span class="name">
    {#if r.dir === 'upload'}<ArrowUp size={14} aria-hidden="true" />{:else}<ArrowDown size={14} aria-hidden="true" />{/if}
    {#if r.phase === 'zip' || r.name.endsWith('.zip')}<FolderIcon size={14} aria-hidden="true" />{/if}
    <span class="ellipsis" data-testid="transfer.{r.id}.name">{r.name}</span>
    {#if r.auto}<span class="auto">{t('transfer.auto')}</span>{/if}
  </span>
{/snippet}
{#snippet stateCell(r: Item)}
  <span class="state" title={r.error || undefined} data-testid="transfer.{r.id}.state">
    {#if r.state === 'active' && (r.phase === 'send' || r.phase === 'recv' || r.phase === 'zip' || r.phase === 'extract') && r.done > 0}
      <span class="bar"><Progress size="sm" value={r.done} max={r.total || r.size} label={r.name} /></span>
      <span class="num">{statusText(r)}</span>
    {:else if r.state === 'pending' && r.phase === 'zip'}
      <span class="bar"><Progress size="sm" value={r.done} max={r.total || r.size} label={r.name} /></span>
      <span class="num">{statusText(r)}</span>
    {:else}
      <StatusChip tone={TONE[r.state] ?? 'neutral'} label={statusText(r)} />
      {#if r.error && r.state !== 'done'}<span class="err ellipsis">{r.error}</span>{/if}
    {/if}
  </span>
{/snippet}
{#snippet sizeCell(r: Item)}
  <span class="num">{formatBytes(r.total || r.size)}</span>
{/snippet}
{#snippet actCell(r: Item)}
  {#if live(r)}
    <IconButton size="sm" icon={X} label={t('transfer.cancelOne')} tooltipPlacement="left" onclick={() => cancel(r)} />
  {/if}
{/snippet}

<Drawer
  id="transferDrawer"
  title={t('transfer.title')}
  bind:open={$drawerOpen}
  bind:height
  onResizeEnd={(h) => updateSettings((s) => (s.transferDrawerHeight = h))}
>
  {#snippet summary()}
    {#if running}
      <span class="overall"><Progress size="sm" value={overall} max={100} label={t('transfer.overall', { p: overall })} /></span>
      <span class="num">{t('transfer.overall', { p: overall })}</span>
    {/if}
    {#if $items.length}
      <span class="counts" data-testid="transferSummary">
        {t('transfer.summary', { a: $counts.active, p: $counts.pending, d: $counts.done, f: $counts.failed })}
      </span>
    {/if}
  {/snippet}
  {#snippet actions()}
    {#if running}
      <IconButton danger size="sm" icon={X} label={t('transfer.cancelAll')} tooltipPlacement="top" onclick={cancelAll} />
    {/if}
    {#if $items.some((it) => !live(it))}
      <Tooltip text={t('transfer.clearHint')} placement="top">
        <Button size="sm" variant="ghost" icon={ListX} onclick={() => TransferService.Clear()}>{t('transfer.clear')}</Button>
      </Tooltip>
    {/if}
  {/snippet}
  {#if $items.length}
    <DataTable id="transferTable" label={t('transfer.title')} rows={$items} {columns} rowKey={(r) => String(r.id)} selectable={false} storageKey="transfer" rowHeight={30} />
  {:else}
    <EmptyState compact icon={ArrowUpDown} title={t('transfer.empty')} />
  {/if}
</Drawer>

<style>
  .name {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-1);
    min-width: 0;
  }
  .name :global(svg) {
    flex: none;
    color: var(--color-text-3);
  }
  .auto {
    flex: none;
    font-size: var(--fs-sm);
    color: var(--color-text-3);
  }
  .state {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    min-width: 0;
    width: 100%;
  }
  .state :global(.chip) {
    flex: none; /* 긴 오류 글자가 상태 글자를 밀어내지 않게 */
  }
  .bar {
    flex: 1;
    min-width: 60px;
  }
  .err {
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .num {
    flex: none;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-2);
  }
  .overall {
    width: 120px;
  }
  .counts {
    white-space: nowrap;
  }
</style>
