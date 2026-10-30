<!--
  탐색기 왼쪽 폴더 트리(원본 LocalDirTreeModel): 바탕 화면 · 내 문서 · 내 컴퓨터 > 드라이브.
  - 노드는 펼칠 때 자식 폴더를 읽는다(Go, 화면은 멈추지 않는다). 「내 컴퓨터」 는 펼칠 때마다 드라이브를 다시 읽는다.
  - 한 번 누름 = 오른쪽 목록에 열기(onOpen), 두 번 누름·[>] = 펼치기/접기.
  - current(오른쪽 목록 폴더)가 바뀌면 부모가 reveal(path) 를 불러 그 노드까지 펼쳐 강조한다.
  - 키보드: ↑↓ 옮기기, → 펼치기/첫 자식, ← 접기/부모, Enter 열기, Home·End.
-->
<script lang="ts">
  import { onMount, tick } from 'svelte'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Monitor from '@lucide/svelte/icons/monitor'
  import FileText from '@lucide/svelte/icons/file-text'
  import Laptop from '@lucide/svelte/icons/laptop'
  import HardDrive from '@lucide/svelte/icons/hard-drive'
  import Usb from '@lucide/svelte/icons/usb'
  import Network from '@lucide/svelte/icons/network'
  import Disc from '@lucide/svelte/icons/disc'
  import Folder from '@lucide/svelte/icons/folder'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import { Spinner, type Icon } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { LocalService, type LocalPlace } from '../../lib/api'

  let { current = '', onOpen }: { current?: string; onOpen: (path: string) => void } = $props()

  type Kind = 'desktop' | 'documents' | 'computer' | 'drive' | 'folder'
  interface TNode {
    key: string
    label: string
    path: string | null
    kind: Kind
    drive?: string
    depth: number
    children: TNode[] | null // null = 아직 안 읽음
    expanded: boolean
    loading: boolean
  }

  const gid = $props.id()
  let roots = $state<TNode[]>([])
  let focusKey = $state<string | null>(null)
  let box: HTMLDivElement

  const low = (p: string) => p.replace(/[\\/]+$/, '').toLowerCase()
  const under = (parent: string, child: string) => {
    const a = low(parent)
    const b = low(child)
    return b === a || b.startsWith(a + '\\') || b.startsWith(a + '/') || (a.endsWith(':') && b.startsWith(a))
  }

  function driveLabel(p: LocalPlace) {
    const letter = p.path.replace(/[\\/]+$/, '')
    const kindName = t(
      p.drive === 'removable' ? 'local.tree.removable' : p.drive === 'network' ? 'local.tree.network' : p.drive === 'cdrom' ? 'local.tree.cdrom' : 'local.tree.drive',
    )
    return `${p.label || kindName} (${letter})`
  }

  const iconOf = (n: TNode): Icon => {
    switch (n.kind) {
      case 'desktop':
        return Monitor
      case 'documents':
        return FileText
      case 'computer':
        return Laptop
      case 'drive':
        return n.drive === 'removable' ? Usb : n.drive === 'network' ? Network : n.drive === 'cdrom' ? Disc : HardDrive
      default:
        return n.expanded ? FolderOpen : Folder
    }
  }

  async function load(n: TNode) {
    n.loading = true
    try {
      if (n.kind === 'computer') {
        const places = (await LocalService.Places()) ?? []
        n.children = places
          .filter((p) => p.kind === 'drive')
          .map((p) => ({
            key: p.path,
            label: driveLabel(p),
            path: p.path,
            kind: 'drive' as Kind,
            drive: p.drive,
            depth: n.depth + 1,
            children: null,
            expanded: false,
            loading: false,
          }))
      } else if (n.path) {
        const res = await LocalService.List(0, n.path)
        n.children = (res?.entries ?? [])
          .filter((e) => e.isDir)
          .map((e) => ({
            key: e.path,
            label: e.name,
            path: e.path,
            kind: 'folder' as Kind,
            depth: n.depth + 1,
            children: null,
            expanded: false,
            loading: false,
          }))
      }
    } catch {
      n.children = [] // 읽을 수 없는 폴더는 펼칠 것이 없다(오른쪽 목록에서 까닭이 보인다)
    } finally {
      n.loading = false
    }
  }

  async function toggle(n: TNode, open = !n.expanded) {
    if (open && (n.children === null || n.kind === 'computer')) await load(n)
    n.expanded = open && (n.children?.length ?? 0) > 0
  }

  /** path 가 있는 노드까지 펼쳐 강조하고 보이게 한다. */
  export async function reveal(path: string) {
    if (!path || !roots.length) return
    const computer = roots.find((r) => r.kind === 'computer')
    const candidates = [...roots.filter((r) => r.path), ...(computer?.children ?? [])]
    let node = candidates
      .filter((c) => c.path && under(c.path, path))
      .sort((a, b) => (b.path?.length ?? 0) - (a.path?.length ?? 0))[0]
    if (!node) return
    if (node.kind === 'drive' && computer && !computer.expanded) computer.expanded = true
    for (let guard = 0; guard < 64 && node.path && low(node.path) !== low(path); guard++) {
      if (node.children === null) await load(node)
      const next: TNode | undefined = node.children?.find((c) => c.path && under(c.path, path))
      if (!next) break
      node.expanded = true
      node = next
    }
    focusKey = node.key
    await tick()
    box?.querySelector(`[data-key="${CSS.escape(node.key)}"] .label`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' }) // 깊은 폴더도 이름이 보이게
  }

  function flatten(ns: TNode[], out: TNode[] = []) {
    for (const n of ns) {
      out.push(n)
      if (n.expanded && n.children) flatten(n.children, out)
    }
    return out
  }
  const visible = $derived(flatten(roots))
  const currentKey = $derived.by(() => {
    if (!current) return null
    return visible.find((n) => n.path && low(n.path) === low(current))?.key ?? null
  })

  function open(n: TNode) {
    focusKey = n.key
    box?.focus({ preventScroll: true })
    if (n.path) onOpen(n.path)
    else void toggle(n)
  }

  function onkeydown(e: KeyboardEvent) {
    const i = visible.findIndex((n) => n.key === focusKey)
    const n = visible[i]
    let next = -1
    switch (e.key) {
      case 'ArrowDown':
        next = Math.min(visible.length - 1, i + 1)
        break
      case 'ArrowUp':
        next = Math.max(0, i - 1)
        break
      case 'Home':
        next = 0
        break
      case 'End':
        next = visible.length - 1
        break
      case 'ArrowRight':
        if (!n) return
        if (!n.expanded && n.children?.length !== 0) void toggle(n, true)
        else if (n.expanded) next = i + 1
        break
      case 'ArrowLeft':
        if (!n) return
        if (n.expanded) void toggle(n, false)
        else {
          for (let j = i - 1; j >= 0; j--)
            if (visible[j].depth < n.depth) {
              next = j
              break
            }
        }
        break
      case 'Enter':
      case ' ':
        if (n) open(n)
        break
      default:
        return
    }
    e.preventDefault()
    if (next >= 0 && visible[next]) {
      focusKey = visible[next].key
      box.querySelector(`[data-key="${CSS.escape(focusKey)}"]`)?.scrollIntoView({ block: 'nearest' })
    }
  }

  onMount(async () => {
    let places: LocalPlace[] = []
    try {
      places = (await LocalService.Places()) ?? []
    } catch {
      /* 브라우저 단독 실행 등 */
    }
    const mk = (p: LocalPlace, kind: Kind, label: string): TNode => ({
      key: kind,
      label,
      path: p.path,
      kind,
      depth: 0,
      children: null,
      expanded: false,
      loading: false,
    })
    const desk = places.find((p) => p.kind === 'desktop')
    const docs = places.find((p) => p.kind === 'documents')
    const list: TNode[] = []
    if (desk) list.push(mk(desk, 'desktop', t('local.tree.desktop')))
    if (docs) list.push(mk(docs, 'documents', t('local.tree.documents')))
    list.push({
      key: 'computer',
      label: t('local.tree.computer'),
      path: null,
      kind: 'computer',
      depth: 0,
      children: null,
      expanded: false,
      loading: false,
    })
    roots = list
    // 열 때 「바탕 화면」·「내 컴퓨터」 를 펼친다(원본 MAIN-101)
    await Promise.all(roots.filter((r) => r.kind === 'desktop' || r.kind === 'computer').map((r) => toggle(r, true)))
    if (current) void reveal(current)
  })
</script>

<div
  class="tree"
  bind:this={box}
  role="tree"
  aria-label={t('local.tree.label')}
  tabindex="0"
  aria-activedescendant={focusKey ? `${gid}-${visible.findIndex((n) => n.key === focusKey)}` : undefined}
  {onkeydown}
>
  {#each visible as n, i (n.key)}
    {@const NodeIcon = iconOf(n)}
    <div
      class="node"
      class:current={n.key === currentKey}
      class:focused={n.key === focusKey}
      id="{gid}-{i}"
      data-key={n.key}
      role="treeitem"
      tabindex="-1"
      aria-level={n.depth + 1}
      aria-selected={n.key === currentKey}
      aria-expanded={n.children?.length === 0 ? undefined : n.expanded}
      style:padding-left="{4 + n.depth * 14}px"
      title={n.path ?? n.label}
      onclick={() => open(n)}
      ondblclick={() => void toggle(n)}
      onkeydown={() => {}}
    >
      <button
        type="button"
        class="chev"
        class:open={n.expanded}
        class:hidden={n.children?.length === 0}
        tabindex="-1"
        aria-hidden="true"
        onclick={(e) => {
          e.stopPropagation()
          void toggle(n)
        }}
      >
        {#if n.loading}<Spinner size={12} />{:else}<ChevronRight size={14} />{/if}
      </button>
      <NodeIcon size={16} aria-hidden="true" />
      <span class="label">{n.label}</span>
    </div>
  {/each}
</div>

<style>
  .tree {
    height: 100%;
    overflow: auto;
    padding: var(--sp-1) 0;
    outline: none;
    font-size: var(--fs-body);
    user-select: none;
  }
  .node {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    min-width: max-content; /* 들여쓰기가 폭을 넘으면 이름을 줄이지 않고 가로로 스크롤한다 */
    height: 26px;
    padding-right: var(--sp-2);
    color: var(--color-text);
    cursor: default;
    white-space: nowrap;
  }
  .node:hover {
    background: var(--color-hover);
  }
  .node.current {
    background: var(--color-selected);
    font-weight: var(--fw-medium);
  }
  .tree:focus-visible .node.focused {
    box-shadow: inset 0 0 0 2px var(--color-focus);
  }
  .node :global(svg) {
    flex: none;
    color: var(--color-text-3);
  }
  .chev {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    padding: 0;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    color: var(--color-text-3);
    cursor: pointer;
    transition: transform var(--dur-fast) var(--ease);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .chev.hidden {
    visibility: hidden;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
