<!--
  파일전송 화면: 서버파일 표(12칸) + [파일 업로드]·[파일 다운로드]·[삭제]·[새로고침]·[탐색기] (docs/port/04 5.1).
  - 목록은 Go(FilesService)가 들고, 바뀐 때만 files:list 로 보낸다 ― 표가 깜빡이지 않고 선택(server_str 열쇠)이 남는다.
    검사 중 행은 Go 가 스스로 다시 조회한다(1·2·4·8초, 최대 40회).
  - 받을 수 없는 사유(Block)와 상태 글자·칩 색은 Go 가 판정한다(원본 getState·classifyDownload).
  - 올리기·받기는 전송 대기열(TransferService)에 넣고 곧 돌아온다. 진행·실패는 전송 상태 서랍이, 끝난 뒤 목록 갱신은 Go 가
    files:list 로 한다. 표 자리는 운영체제 탐색기에서 끌어 온 파일(Wails 파일 놓기, data-file-drop-target)과
    아래 탐색기(LocalPanel)에서 끈 행을 받는다.
  - 탐색기를 켜면 표 아래에 세로로 나눠 붙인다. 높이는 경계를 끌어 바꾸고 설정 localPanelHeight 에 저장한다.
  - 사후결재 망(preMode 거짓)에서 표에 행이 있으면 [결재요청] 단추로 결재요청 화면에 간다.
-->
<script lang="ts">
  import { onMount, untrack, type Component } from 'svelte'
  import { get } from 'svelte/store'
  import UploadIcon from '@lucide/svelte/icons/upload'
  import FolderUp from '@lucide/svelte/icons/folder-up'
  import Send from '@lucide/svelte/icons/send'
  import DownloadIcon from '@lucide/svelte/icons/download'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import FileIcon from '@lucide/svelte/icons/file'
  import FolderIcon from '@lucide/svelte/icons/folder'
  import Trash2 from '@lucide/svelte/icons/trash-2'
  import Search from '@lucide/svelte/icons/search'
  import ShieldAlert from '@lucide/svelte/icons/shield-alert'
  import Bug from '@lucide/svelte/icons/bug'
  import ScanSearch from '@lucide/svelte/icons/scan-search'
  import CloudOff from '@lucide/svelte/icons/cloud-off'
  import {
    Button,
    ContextMenu,
    DataTable,
    EmptyState,
    IconButton,
    Input,
    Page,
    StatusChip,
    formatBytes,
    type Column,
    type MenuItem,
  } from '../../lib/ui'
  import { t, tx } from '../../lib/i18n'
  import { ApprovalService, Ev, FilesService, TransferService, errorText, onAll, type FileRow } from '../../lib/api'
  import { alert, confirm, navigate, settings, toast, updateSettings } from '../../lib/stores/ui'
  import LocalPanel from '../local/LocalPanel.svelte'
  import { acceptLocalDrag, hasLocalDrag, localDragPaths, uploadLocalPaths } from '../local/dnd'
  import { KIND_TONE, fileSelection } from './store'

  // 클립보드 단추·건수(클립보드 담당 파일 ― 없으면 그리지 않는다)
  const ClipActions = Object.values(
    import.meta.glob<{ default: Component }>('../clip/ClipActions.svelte', { eager: true }),
  )[0]?.default

  let rows = $state.raw<FileRow[]>([])
  let selected = $state(get(fileSelection))
  $effect(() => fileSelection.set(selected))
  let loading = $state(false)
  let loadError = $state('')
  let uploading = $state(false)
  let downloading = $state(false)
  let deleting = $state(false)
  let explorer = $state(false)
  let filter = $state('')
  let menu = $state({ open: false, x: 0, y: 0 })
  let preMode = $state(true) // 모르면 [결재요청]을 숨긴다
  let dropOver = $state(false)

  // 탐색기 높이(settings.localPanelHeight, 0 = 기본 260)
  let panelH = $state(untrack(() => $settings?.localPanelHeight || 260))
  let box: HTMLDivElement
  let drag: { y: number; h: number } | null = null
  const clampH = (h: number) => Math.round(Math.max(120, Math.min(h, (box?.clientHeight ?? 800) - 160)))
  function gripDown(e: PointerEvent) {
    drag = { y: e.clientY, h: panelH }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }
  function gripMove(e: PointerEvent) {
    if (drag) panelH = clampH(drag.h - (e.clientY - drag.y))
  }
  function gripUp() {
    if (!drag) return
    drag = null
    void updateSettings((s) => (s.localPanelHeight = panelH))
  }
  function gripKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    e.preventDefault()
    panelH = clampH(panelH + (e.key === 'ArrowUp' ? 16 : -16))
    void updateSettings((s) => (s.localPanelHeight = panelH))
  }

  const shown = $derived.by(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return rows
    return rows.filter((r) => r.name.toLowerCase().includes(q) || r.serverName.toLowerCase().includes(q))
  })
  const picked = $derived(rows.filter((r) => selected.has(r.serverName)))
  const busy = $derived(uploading || downloading || deleting)

  const columns: Column<FileRow>[] = $derived([
    { key: 'no', title: t('files.col.no'), width: 45, align: 'right', value: (r) => r.no },
    { key: 'name', title: t('files.col.name'), width: 200, grow: true, cell: nameCell },
    { key: 'status', title: t('files.col.state'), width: 150, value: (r) => r.statusText, cell: statusCell },
    { key: 'size', title: t('files.col.size'), width: 90, align: 'right', value: (r) => r.size, cell: sizeCell },
    { key: 'uploadTime', title: t('files.col.uploadTime'), width: 165 },
    { key: 'courseName', title: t('files.col.course'), width: 110 },
    { key: 'reqCause', title: t('files.col.reqCause'), width: 130 },
    { key: 'approver', title: t('files.col.approver'), width: 100 },
    { key: 'aprvCause', title: t('files.col.aprvCause'), width: 130 },
    { key: 'isDir', title: t('files.col.isDir'), width: 80, value: (r) => (r.isDir ? t('files.folder') : '') },
    { key: 'serverName', title: t('files.col.serverName'), width: 150 },
  ])

  function setRows(list: FileRow[] | null) {
    rows = list ?? [] // Go 의 빈 목록(nil)은 null 로 온다
  }

  async function refresh() {
    loading = true
    try {
      setRows(await FilesService.List())
      loadError = ''
    } catch (e) {
      loadError = errorText(e)
    } finally {
      loading = false
    }
  }

  /** 파일·폴더 고르기 → 검사·결재자 선택 → 대기열(Go). 대기열에 넣으면 돌아오고 진행은 서랍이 보인다. */
  async function upload(folder = false) {
    uploading = true
    try {
      await (folder ? TransferService.UploadFolder() : TransferService.Upload())
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      uploading = false
    }
  }

  function dropLocal(e: DragEvent) {
    dropOver = false
    const p = localDragPaths(e)
    if (p) void uploadLocalPaths(p)
  }

  /** 받을 수 없어 뺀 파일 안내(원본 popDownloadBlocked): 하나면 알림 띠, 여럿이면 사유별 정보 창. */
  async function explainBlocked(list: FileRow[]) {
    if (list.length === 1) {
      toast(t(`files.block.${list[0].block}`, { name: list[0].name }), 2)
      return
    }
    const lines = [t('files.blocked.head')]
    for (const b of [1, 2, 3, 4]) {
      const names = list.filter((r) => r.block === b).map((r) => r.name)
      if (!names.length) continue
      lines.push('', t(`files.blocked.${b}`, { n: names.length }), ...names.map((n) => `  ${n}`))
    }
    await alert({ text: lines.join('\n') })
  }

  async function download(keys: string[] = [...selected]) {
    const set = new Set(keys)
    const list = rows.filter((r) => set.has(r.serverName))
    if (!list.length) {
      toast(t('files.download.none'), 2)
      return
    }
    const ok = list.filter((r) => r.block === 0)
    const blocked = list.filter((r) => r.block !== 0)
    if (blocked.length) await explainBlocked(blocked)
    if (!ok.length) return // 받을 것이 없으면 폴더 선택 창을 띄우지 않는다
    downloading = true
    try {
      await TransferService.Download(ok.map((r) => r.serverName))
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      downloading = false
    }
  }

  async function remove(keys: string[] = [...selected]) {
    if (!keys.length) {
      toast(t('files.delete.none'), 2)
      return
    }
    const yes = await confirm({
      title: t('files.delete.title'),
      text: t('files.delete.confirm', { n: keys.length }),
      okLabel: t('files.delete'),
      danger: true,
    })
    if (!yes) return
    deleting = true
    try {
      const failed = await FilesService.Delete(keys)
      if (failed) toast(t('files.delete.failed', { n: failed }), 3)
      else toast(t('files.delete.done', { n: keys.length }), 1)
      selected = new Set()
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      deleting = false
    }
  }

  /** 검출 조회: 0 개인정보, 1 APT, 2 바이러스. 선택이 정확히 하나여야 한다. */
  async function detection(kind: 0 | 1 | 2) {
    if (selected.size !== 1) {
      toast(t('files.detect.one'), 2)
      return
    }
    try {
      const text = await FilesService.DetectionDetail([...selected][0], kind)
      await alert({ title: t(`files.detect.${kind}`), text })
    } catch (e) {
      toast(errorText(e), 3)
    }
  }

  const menuItems: MenuItem[] = $derived([
    { label: t('files.menu.download'), icon: DownloadIcon, disabled: busy, onSelect: () => download() },
    { label: t('files.menu.delete'), icon: Trash2, hint: 'Del', danger: true, disabled: busy, onSelect: () => remove() },
    { separator: true },
    { label: t('files.menu.priv'), icon: ScanSearch, disabled: selected.size !== 1, onSelect: () => detection(0) },
    { label: t('files.menu.virus'), icon: Bug, disabled: selected.size !== 1, onSelect: () => detection(2) },
    { label: t('files.menu.apt'), icon: ShieldAlert, disabled: selected.size !== 1, onSelect: () => detection(1) },
    { separator: true },
    { label: t('common.refresh'), icon: RefreshCw, hint: 'F5', onSelect: refresh },
  ])

  function onkey(e: KeyboardEvent) {
    if (e.key === 'F5') {
      e.preventDefault()
      void refresh()
    } else if (e.key === 'Delete' && !busy && selected.size) {
      const el = e.target as HTMLElement
      if (el.closest('input, textarea, select, [contenteditable="true"], dialog')) return
      e.preventDefault()
      void remove()
    }
  }

  onMount(() => {
    void refresh()
    ApprovalService.Mode()
      .then((m) => (preMode = m?.preMode ?? true))
      .catch(() => {})
    return onAll({ [Ev.fileList]: (list: FileRow[] | null) => setRows(list) })
  })
</script>

<svelte:window onkeydown={onkey} />

{#snippet nameCell(r: FileRow)}
  <span class="name">
    {#if r.isDir}<FolderIcon size={15} aria-hidden="true" />{:else}<FileIcon size={15} aria-hidden="true" />{/if}
    <span class="ellipsis" data-testid="{r.serverName}.fileNameCell">{r.name}</span>
  </span>
{/snippet}
{#snippet statusCell(r: FileRow)}
  <StatusChip tone={KIND_TONE[r.statusKind] ?? 'neutral'} label={tx(r.statusText)} />
{/snippet}
{#snippet sizeCell(r: FileRow)}
  <span class="num" title={formatBytes(r.size)}>{r.sizeText}</span>
{/snippet}

<Page
  title={t('common.menu.files')}
  subtitle={selected.size ? t('common.selected', { n: selected.size }) : t('common.total', { n: rows.length })}
>
  {#snippet actions()}
    <Button variant="primary" icon={UploadIcon} loading={uploading} disabled={busy} onclick={() => upload()}>{t('files.upload')}</Button>
    <Button icon={FolderUp} disabled={busy} onclick={() => upload(true)}>{t('files.uploadFolder')}</Button>
    <Button
      icon={DownloadIcon}
      loading={downloading}
      disabled={busy || !selected.size}
      aria-label={t('files.download')}
      onclick={() => download()}
    >
      {t('files.download')}{picked.length ? ` (${picked.length})` : ''}
    </Button>
    {#if !preMode && rows.length}
      <Button icon={Send} onclick={() => navigate('request')}>{t('files.request')}</Button>
    {/if}
    {#if ClipActions}<ClipActions />{/if}
    <IconButton danger icon={Trash2} label={t('files.delete')} loading={deleting} disabled={busy || !selected.size} onclick={() => remove()} />
    <IconButton icon={RefreshCw} label={t('common.refresh')} {loading} onclick={refresh} />
    <IconButton
      icon={FolderOpen}
      label={explorer ? t('files.explorerClose') : t('files.explorerOpen')}
      pressed={explorer}
      onclick={() => (explorer = !explorer)}
    />
  {/snippet}
  {#snippet toolbar()}
    <div class="filter">
      <Input size="sm" icon={Search} bind:value={filter} placeholder={t('files.filter')} aria-label={t('files.filter')} type="search" />
    </div>
  {/snippet}

  <div class="split" bind:this={box}>
  <div
    class="table"
    class:over={dropOver}
    id="serverFileDrop"
    data-file-drop-target
    role="region"
    aria-label={t('files.dropHint')}
    title={t('files.dropHint')}
    ondragenter={(e) => (dropOver = hasLocalDrag(e))}
    ondragover={(e) => acceptLocalDrag(e)}
    ondragleave={(e) => {
      if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) dropOver = false
    }}
    ondrop={dropLocal}
  >
    <DataTable
      id="serverFileTable"
      label={t('files.table')}
      rows={shown}
      {columns}
      rowKey={(r) => r.serverName}
      bind:selected
      storageKey="serverFiles"
      loading={loading && !rows.length}
      onActivate={(r) => download([r.serverName])}
      oncontextmenu={(e, r) => r && (menu = { open: true, x: e.clientX, y: e.clientY })}
    >
      {#snippet empty()}
        {#if loadError}
          <EmptyState compact icon={CloudOff} title={loadError}>
            <Button size="sm" icon={RefreshCw} onclick={refresh}>{t('common.retry')}</Button>
          </EmptyState>
        {:else if filter.trim() && rows.length}
          <EmptyState compact icon={Search} title={t('files.noMatch')} />
        {:else}
          <EmptyState compact icon={CloudOff} title={t('files.empty')} description={t('files.emptyHint')} />
        {/if}
      {/snippet}
    </DataTable>
  </div>

  {#if explorer}
    <!-- 초점을 받는 separator 는 ARIA 창 나누기 위젯이다(lib/ui SplitPane 과 같은 예외) -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="grip"
      role="separator"
      aria-orientation="horizontal"
      aria-label={t('files.splitHint')}
      aria-valuenow={panelH}
      tabindex="0"
      onpointerdown={gripDown}
      onpointermove={gripMove}
      onpointerup={gripUp}
      onpointercancel={gripUp}
      onkeydown={gripKey}
    ></div>
    <div class="explorer" style:height="{panelH}px"><LocalPanel serverSelection={[...selected]} /></div>
  {/if}
  </div>
</Page>

<ContextMenu bind:open={menu.open} x={menu.x} y={menu.y} items={menuItems} label={t('files.menu')} />

<style>
  .split {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .table {
    flex: 1 1 0;
    min-height: 120px; /* 서랍을 펴도 서버파일 표가 사라지지 않게 탐색기 쪽이 줄어든다 */
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  /* 탐색기 행을 끌어 올 때(over), 운영체제 파일을 끌어 올 때(Wails 가 붙이는 file-drop-target-active) */
  .table.over,
  .table:global(.file-drop-target-active) {
    border-color: var(--color-accent);
    box-shadow: 0 0 0 1px var(--color-accent);
  }
  .grip {
    flex: none;
    height: var(--sp-2);
    cursor: row-resize;
    position: relative;
  }
  .grip::after {
    content: '';
    position: absolute;
    left: 50%;
    top: 50%;
    width: 32px;
    height: 2px;
    transform: translate(-50%, -50%);
    border-radius: 1px;
    background: var(--color-border);
  }
  .grip:hover::after,
  .grip:focus-visible::after {
    background: var(--color-accent);
  }
  .grip:focus-visible {
    outline: none;
  }
  .filter {
    width: min(280px, 100%);
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
  .explorer {
    flex: 0 1 auto;
    min-height: 120px;
  }
</style>
