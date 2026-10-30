<!--
  로컬 탐색기 패널(원본 LocalFileExplorerPanel.qml, docs/port/12 5.7). 독립 컴포넌트 ― 서버파일 화면(FilesView)이 아래나 옆에 끼운다.

  props:
    serverSelection  위 서버파일 표에서 고른 서버 파일 이름(serverName)들. [다운로드] 가 이것을 지금 폴더로 받는다.
    onTransfer       업로드·다운로드를 맡긴 뒤 부른다(서버 목록 새로 고침 등, 선택).

  위 줄: [상위] · 경로 입력(Enter 로 이동) · [업로드] · [다운로드] · [새 폴더] · [숨김 파일] · [새로고침].
  왼쪽 트리(경계 끌기, 폭 저장) · 오른쪽 목록(「상위 폴더」 늘 첫 줄, 폴더 먼저, 정렬 저장, 다중 선택, 두 번 누름=폴더 열기).
  목록 행을 서버파일 표로 끌어 놓으면 업로드한다(dnd.ts). 읽기가 300ms 를 넘으면 「폴더를 읽고 있습니다...」.
  키보드: F5 새로고침, Backspace 상위 폴더(목록에 초점), Enter 열기.
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import UploadIcon from '@lucide/svelte/icons/upload'
  import DownloadIcon from '@lucide/svelte/icons/download'
  import FolderPlus from '@lucide/svelte/icons/folder-plus'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import Folder from '@lucide/svelte/icons/folder'
  import FileIcon from '@lucide/svelte/icons/file'
  import FileSymlink from '@lucide/svelte/icons/file-symlink'
  import CornerLeftUp from '@lucide/svelte/icons/corner-left-up'
  import FolderX from '@lucide/svelte/icons/folder-x'
  import {
    Button,
    DataTable,
    Dialog,
    EmptyState,
    IconButton,
    Input,
    Spinner,
    SplitPane,
    Tooltip,
    type Column,
  } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { LocalService, TransferService, errorText, type LocalEntry } from '../../lib/api'
  import { settings, toast, updateSettings } from '../../lib/stores/ui'
  import Tree from './Tree.svelte'
  import { startLocalDrag, uploadLocalPaths } from './dnd'

  let { serverSelection = [], onTransfer }: { serverSelection?: string[]; onTransfer?: () => void } = $props()

  type Row = LocalEntry & { up?: boolean }
  const UP_KEY = '\u0000up'

  let path = $state('') // 지금 보이는 폴더
  let parent = $state('')
  let input = $state('') // 경로 입력란
  let entries = $state<LocalEntry[]>([])
  let selected = $state(new Set<string>())
  let errKey = $state<string | null>(null) // 읽기 오류 번역 키(또는 날 글자)
  let reading = $state(false) // 300ms 넘게 읽는 중
  let busy = $state(false)
  let tree: Tree | undefined = $state()

  let sortKey = $state<string | undefined>(untrack(() => $settings?.localSortKey || undefined))
  let sortDir = $state<'asc' | 'desc'>(untrack(() => ($settings?.localSortDesc ? 'desc' : 'asc')))
  let treeW = $state(untrack(() => $settings?.localTreeWidth || 190))
  const showHidden = $derived($settings?.localShowHidden ?? false)

  let req = 0
  let lastSaveTimer: ReturnType<typeof setTimeout> | undefined

  const rows = $derived<Row[]>(
    parent
      ? [{ name: '..', path: parent, isDir: true, isLink: false, size: 0, sizeText: '', ext: '', type: '', modTime: '', modUnix: 0, up: true }, ...entries]
      : entries,
  )
  const keyOf = (r: Row) => (r.up ? UP_KEY : r.path)
  const picked = $derived(entries.filter((e) => selected.has(e.path)))
  const pickedFiles = $derived(picked.filter((e) => !e.isDir).length)
  const pickedDirs = $derived(picked.length - pickedFiles)

  // ── 오류 글자: Go 는 원본 한국어 문구를 낸다 → 번역 키로 ──
  const ERRS: Record<string, string> = {
    '폴더를 찾을 수 없습니다.': 'local.err.notFound',
    '폴더를 읽을 권한이 없습니다.': 'local.err.permission',
    '먼저 폴더를 선택해 주세요.': 'local.err.noParent',
    '폴더 이름을 입력해 주세요.': 'local.err.emptyName',
    '폴더 이름에 다음 문자는 쓸 수 없습니다.': 'local.err.badChars',
    '사용할 수 없는 폴더 이름입니다.': 'local.err.badName',
    '같은 이름의 폴더나 파일이 이미 있습니다.': 'local.err.exists',
    '폴더를 만들 수 없습니다. 쓰기 권한을 확인해 주세요.': 'local.err.cannotWrite',
  }
  function errMsg(e: unknown) {
    const s = errorText(e)
    for (const [ko, key] of Object.entries(ERRS)) if (s.includes(ko)) return t(key)
    return s
  }

  const typeText = (r: Row) =>
    r.up
      ? ''
      : r.isLink
        ? t('local.type.link')
        : r.isDir
          ? t('local.type.folder')
          : r.type || (r.ext ? t('local.type.ext', { ext: r.ext }) : t('local.type.file'))

  const columns: Column<Row>[] = $derived([
    { key: 'name', title: t('local.col.name'), width: 260, grow: true, cell: nameCell, value: (r) => r.name },
    { key: 'size', title: t('local.col.size'), width: 90, align: 'right', value: (r) => (r.isDir ? -1 : r.size), cell: sizeCell },
    { key: 'type', title: t('local.col.type'), width: 130, value: typeText },
    { key: 'modTime', title: t('local.col.modified'), width: 130, value: (r) => r.modUnix, cell: modCell },
  ])

  // 「상위 폴더」 는 늘 맨 위, 폴더는 늘 파일 앞(정렬 방향과 무관 ― 원본 parentEntry_isAlwaysFirst·directoriesBeforeFiles)
  const rank = (r: Row) => (r.up ? 0 : r.isDir ? 1 : 2)

  async function go(dir: string, keepSelection = false) {
    const my = ++req
    const slow = setTimeout(() => my === req && (reading = true), 300)
    try {
      const res = await LocalService.List(my, dir)
      if (!res || res.req !== req) return // 늦게 온 옛 결과
      const same = res.path === path
      path = res.path
      parent = res.parent
      input = res.path
      entries = res.entries ?? []
      errKey = null
      if (!same && !keepSelection) selected = new Set()
      void tree?.reveal(res.path)
      clearTimeout(lastSaveTimer)
      lastSaveTimer = setTimeout(() => {
        if ($settings?.lastFolder !== res.path) void updateSettings((s) => (s.lastFolder = res.path))
      }, 800)
    } catch (e) {
      if (my !== req) return
      path = dir
      input = dir
      parent = ''
      entries = []
      selected = new Set()
      errKey = errMsg(e)
    } finally {
      clearTimeout(slow)
      if (my === req) reading = false
    }
  }

  const refresh = () => (path ? go(path, true) : Promise.resolve())
  const goUp = () => parent && go(parent)

  function activate(r: Row) {
    if (r.isDir && !r.isLink) void go(r.path)
  }

  async function upload() {
    const paths = picked.map((e) => e.path)
    if (!paths.length) return
    busy = true
    try {
      if (await uploadLocalPaths(paths)) onTransfer?.()
    } finally {
      busy = false
    }
  }

  async function download() {
    if (!serverSelection.length || !path) return
    busy = true
    try {
      await TransferService.DownloadTo(serverSelection, path)
      toast(t('local.downloadStarted', { n: serverSelection.length, path }), 0)
      onTransfer?.()
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      busy = false
    }
  }

  function dragStart(e: DragEvent, keys: Set<string>) {
    const items = entries.filter((x) => keys.has(x.path))
    if (!items.length) {
      e.preventDefault()
      return
    }
    startLocalDrag(
      e,
      items.map((x) => x.path),
      items.map((x) => x.name),
    )
  }

  // ── 새 폴더 ──
  let mkOpen = $state(false)
  let mkName = $state('')
  let mkError = $state('')
  function openMakeDir() {
    mkName = t('local.newFolder')
    mkError = ''
    mkOpen = true
  }
  async function makeDir() {
    try {
      const p = await LocalService.MakeDir(path, mkName)
      await go(path, true)
      selected = new Set([p])
    } catch (e) {
      mkError = errMsg(e)
      return false
    }
  }

  function toggleHidden() {
    const next = !showHidden
    void updateSettings((s) => (s.localShowHidden = next)).then(refresh)
  }

  // 정렬이 바뀌면 저장(원본은 저장하지 않았다 ― MAIN-118 개선)
  $effect(() => {
    const k = sortKey ?? ''
    const desc = sortDir === 'desc'
    untrack(() => {
      const s = $settings
      if (s && (s.localSortKey !== k || s.localSortDesc !== desc))
        void updateSettings((x) => {
          x.localSortKey = k
          x.localSortDesc = desc
        })
    })
  })

  function onkeydown(e: KeyboardEvent) {
    const inInput = (e.target as HTMLElement).closest('input, textarea')
    if (e.key === 'F5') {
      e.preventDefault()
      void refresh()
    } else if (e.key === 'Backspace' && !inInput && parent) {
      e.preventDefault()
      void goUp()
    }
  }

  onMount(() => {
    void (async () => {
      try {
        const s = await LocalService.StartFolder()
        if (s?.path) await go(s.path)
      } catch (e) {
        errKey = errMsg(e)
      }
    })()
    return () => clearTimeout(lastSaveTimer)
  })
</script>

{#snippet nameCell(r: Row)}
  <span class="name" class:up={r.up}>
    {#if r.up}
      <CornerLeftUp size={15} aria-hidden="true" /><span class="ellipsis">{t('local.up')}</span>
    {:else}
      {#if r.isLink}<FileSymlink size={15} aria-hidden="true" />{:else if r.isDir}<Folder size={15} aria-hidden="true" class="dir" />{:else}<FileIcon
          size={15}
          aria-hidden="true"
        />{/if}
      <span class="ellipsis">{r.name}</span>
    {/if}
  </span>
{/snippet}
{#snippet sizeCell(r: Row)}
  <span class="num">{r.sizeText}</span>
{/snippet}
{#snippet modCell(r: Row)}
  <span class="num">{r.modTime}</span>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<section class="local" id="localExplorer" aria-label={t('local.title')} {onkeydown}>
  <div class="bar">
    <IconButton icon={ArrowUp} label={t('local.up')} size="sm" disabled={!parent} onclick={goUp} />
    <form
      class="path"
      onsubmit={(e) => {
        e.preventDefault()
        if (input.trim()) void go(input.trim())
      }}
    >
      <Input
        size="sm"
        bind:value={input}
        placeholder={t('local.pathPlaceholder')}
        aria-label={t('local.path')}
        id="localPathField"
        onkeydown={(e: KeyboardEvent) => e.key === 'Escape' && (input = path)}
      />
    </form>
    <Tooltip text={picked.length ? t('local.uploadTip', { files: pickedFiles, dirs: pickedDirs }) : t('local.uploadNone')}>
      <Button size="sm" variant="primary" icon={UploadIcon} disabled={!picked.length || busy} onclick={upload} id="localUploadButton">
        {t('local.upload')}
      </Button>
    </Tooltip>
    <Tooltip text={serverSelection.length ? t('local.downloadTip') : t('local.downloadNone')}>
      <Button size="sm" icon={DownloadIcon} disabled={!serverSelection.length || !path || !!errKey || busy} onclick={download} id="localDownloadButton">
        {t('local.download')}
      </Button>
    </Tooltip>
    <IconButton icon={FolderPlus} label={t('local.newFolderTip')} size="sm" disabled={!path || !!errKey} onclick={openMakeDir} />
    <IconButton icon={showHidden ? Eye : EyeOff} label={t('local.showHidden')} size="sm" pressed={showHidden} onclick={toggleHidden} />
    <IconButton icon={RefreshCw} label="{t('common.refresh')} (F5)" size="sm" loading={reading} onclick={refresh} />
  </div>

  <div class="split">
    <SplitPane bind:size={treeW} min={120} max={480} minRight={240} onResizeEnd={(w) => void updateSettings((s) => (s.localTreeWidth = w))}>
      {#snippet left()}
        <Tree bind:this={tree} current={path} onOpen={(p) => void go(p)} />
      {/snippet}
      {#snippet right()}
        <div class="list">
          {#if errKey}
            <div class="state">
              <EmptyState compact icon={FolderX} title={errKey} description={path}>
                <Button size="sm" onclick={refresh}>{t('common.retry')}</Button>
              </EmptyState>
            </div>
          {:else if !path && !reading}
            <div class="state"><EmptyState compact icon={Folder} title={t('local.emptyNoFolder')} /></div>
          {:else}
            <DataTable
              id="localFileList"
              label={t('local.list.label')}
              {rows}
              {columns}
              rowKey={keyOf}
              bind:selected
              bind:sortKey
              bind:sortDir
              {rank}
              storageKey="localFiles"
              emptyText={t('local.emptyFolder')}
              onActivate={activate}
              ondragstart={dragStart}
            />
            {#if reading}
              <div class="reading" role="status"><Spinner size={14} /> {t('local.reading')}</div>
            {/if}
          {/if}
        </div>
      {/snippet}
    </SplitPane>
  </div>

  <div class="status" aria-live="polite">
    {#if !errKey && path}
      {picked.length
        ? t('local.statusSelected', { n: entries.length, files: pickedFiles, dirs: pickedDirs })
        : t('local.status', { n: entries.length })}
    {/if}
  </div>
</section>

<Dialog
  bind:open={mkOpen}
  title={t('local.newFolder')}
  okLabel={t('common.ok')}
  cancelLabel={t('common.cancel')}
  okDisabled={!mkName.trim()}
  onOk={makeDir}
>
  <form
    class="mk"
    onsubmit={(e) => {
      e.preventDefault()
      void makeDir().then((r) => r !== false && (mkOpen = false))
    }}
  >
    <p class="where">{t('local.newFolderAt', { path })}</p>
    <Input
      bind:value={mkName}
      placeholder={t('local.folderName')}
      aria-label={t('local.folderName')}
      error={mkError || undefined}
      autofocus
      oninput={() => (mkError = '')}
    />
  </form>
</Dialog>

<style>
  .local {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    background: var(--color-bg);
    overflow: hidden;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    padding: var(--sp-1) var(--sp-2);
    border-bottom: 1px solid var(--color-border-subtle);
    background: var(--color-surface);
    flex: none;
  }
  .path {
    flex: 1;
    min-width: 120px;
    margin: 0 var(--sp-1);
  }
  .split {
    flex: 1;
    min-height: 0;
  }
  .list {
    position: relative;
    height: 100%;
    min-height: 0;
  }
  .state {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .reading {
    position: absolute;
    left: 50%;
    top: 40px;
    transform: translateX(-50%);
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-1) var(--sp-3);
    border: 1px solid var(--color-border);
    border-radius: var(--r-full);
    background: var(--color-surface-raised);
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    box-shadow: var(--shadow-2);
  }
  .status {
    flex: none;
    height: 24px;
    padding: 0 var(--sp-3);
    display: flex;
    align-items: center;
    border-top: 1px solid var(--color-border-subtle);
    color: var(--color-text-3);
    font-size: var(--fs-sm);
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
  .name :global(svg.dir) {
    color: var(--color-accent);
  }
  .name.up {
    color: var(--color-text-2);
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
  .mk {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .where {
    margin: 0;
    color: var(--color-text-3);
    font-size: var(--fs-sm);
    word-break: break-all;
  }
  :global(.snb-drag-ghost) {
    position: fixed;
    top: -1000px;
    left: -1000px;
    padding: var(--sp-1) var(--sp-3);
    border-radius: var(--r-md);
    background: var(--color-accent-fill);
    color: var(--color-text-on-accent);
    font-size: var(--fs-sm);
    white-space: nowrap;
  }
</style>
