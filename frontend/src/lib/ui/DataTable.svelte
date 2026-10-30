<!--
  업무 표. 보이는 행만 그린다(가상 스크롤, 행 높이 고정) ― 1만 행에서도 스크롤이 부드럽다.

  - columns: Column<T>[](lib/ui/types.ts) ― 키·제목·폭·정렬·정렬 함수·값·셀 snippet.
  - rowKey: 행의 변하지 않는 열쇠. 선택은 열쇠로 기억해서 목록을 새로 받아도 같은 행이 선택된 채 남는다
    (없어진 행·비활성이 된 행의 열쇠는 저절로 빠진다).
  - selected(bind): 선택된 열쇠 Set. 바꿀 때마다 새 Set 으로 갈아 끼운다. onSelectionChange(keys) 도 부른다.
  - 선택: 누름=하나만, Ctrl+누름·체크 칸=넣고 빼기, Shift+누름=범위, 머리 체크=전체(3상태).
  - 키보드: ↑↓ PageUp PageDown Home End 로 옮기며 선택(Shift 범위, Ctrl 은 초점만), Space 넣고 빼기,
    Ctrl+A 전체, Esc 선택 해제, Enter·두 번 누름 = onActivate(row), 메뉴 키·Shift+F10 = oncontextmenu.
  - disabled(row): 참 또는 까닭 글자면 그 행은 흐리게, 선택할 수 없고, 까닭은 행 툴팁으로 보인다.
  - storageKey: 끌어 바꾼 컬럼 폭을 설정(columnWidths[storageKey])에 저장하고 다음에 되살린다.
  - sortKey·sortDir(bind): 머리 누름(또는 머리 단추에 초점을 두고 Enter·Space)으로 오름 ↔ 내림. 비교는 col.sort, 없으면 값의 한국어 자연 정렬.
  - rank(row): 정렬 방향과 무관하게 늘 앞에 둘 무리 번호(작을수록 앞). 탐색기의 「상위 폴더」·폴더 먼저 같은 것.
  - ondragstart(e, keys): 주면 행을 끌 수 있다. 선택 밖의 행을 끌면 그 행만 고른 뒤 부른다 ― e.dataTransfer 는 쓰는 쪽이 채운다.
  - 넘친 글자는 … 로 줄이고, 마우스를 올리면 전체 글자를 툴팁(title)으로 보인다.
  - oncontextmenu(e, row|null, keys): 우클릭. 선택 밖의 행을 누르면 그 행만 고른 뒤 부른다. 빈 곳이면 row 가 null.
  - 빈 목록이면 empty snippet, 없으면 emptyText 로 EmptyState. loading 이면 도는 표시.
-->
<script lang="ts" generics="T">
  import { untrack, type Snippet } from 'svelte'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import Inbox from '@lucide/svelte/icons/inbox'
  import type { Column } from './types'
  import Checkbox from './Checkbox.svelte'
  import EmptyState from './EmptyState.svelte'
  import Spinner from './Spinner.svelte'
  import { t } from '../i18n'
  import { getColumnWidths, saveColumnWidths } from '../stores/ui'

  let {
    rows,
    columns,
    rowKey,
    label,
    selectable = true,
    multiple = true,
    selected = $bindable(new Set<string>()),
    disabled,
    storageKey,
    sortKey = $bindable(),
    sortDir = $bindable('asc'),
    rowHeight = 30,
    loading = false,
    emptyText,
    empty,
    striped = false,
    onActivate,
    onSelectionChange,
    oncontextmenu,
    rank,
    ondragstart,
    id,
  }: {
    rows: T[]
    columns: Column<T>[]
    rowKey: (row: T) => string
    label: string
    selectable?: boolean
    multiple?: boolean
    selected?: Set<string>
    disabled?: (row: T) => boolean | string | null | undefined
    storageKey?: string
    sortKey?: string
    sortDir?: 'asc' | 'desc'
    rowHeight?: number
    loading?: boolean
    emptyText?: string
    empty?: Snippet
    striped?: boolean
    onActivate?: (row: T) => void
    onSelectionChange?: (keys: Set<string>) => void
    oncontextmenu?: (e: MouseEvent, row: T | null, keys: Set<string>) => void
    rank?: (row: T) => number
    ondragstart?: (e: DragEvent, keys: Set<string>) => void
    id?: string
  } = $props()

  const gid = $props.id()
  const OVERSCAN = 8
  const SEL_W = 36
  const collator = new Intl.Collator('ko', { numeric: true, sensitivity: 'base' })

  let scroller: HTMLDivElement
  let scrollTop = $state(0)
  let viewH = $state(400)
  let headH = $state(32)
  let focusKey = $state<string | null>(null)
  let anchorKey: string | null = null

  // ── 컬럼 폭 ──
  function loadWidths(): Record<string, number> {
    const saved = storageKey ? getColumnWidths(storageKey) : undefined
    const out: Record<string, number> = {}
    if (saved) columns.forEach((c, i) => saved[i] > 0 && (out[c.key] = saved[i]))
    return out
  }
  let custom = $state<Record<string, number>>(untrack(loadWidths))
  const widthOf = (c: Column<T>) => custom[c.key] ?? c.width ?? 120
  const growKey = $derived((columns.find((c) => c.grow) ?? columns[columns.length - 1])?.key)
  const template = $derived(
    (selectable ? `${SEL_W}px ` : '') +
      columns.map((c) => (c.key === growKey ? `minmax(${widthOf(c)}px, 1fr)` : `${widthOf(c)}px`)).join(' '),
  )
  const totalW = $derived(columns.reduce((s, c) => s + widthOf(c), selectable ? SEL_W : 0))

  // ── 값·정렬 ──
  const valueOf = (c: Column<T>, r: T) => (c.value ? c.value(r) : (r as unknown as Record<string, unknown>)[c.key])
  function compare(a: unknown, b: unknown) {
    if (a == null || a === '') return b == null || b === '' ? 0 : 1
    if (b == null || b === '') return -1
    if (typeof a === 'number' && typeof b === 'number') return a - b
    return collator.compare(String(a), String(b))
  }
  const sorted = $derived.by(() => {
    const col = sortKey ? columns.find((c) => c.key === sortKey) : undefined
    if (!col) return rows
    const dir = sortDir === 'desc' ? -1 : 1
    const cmp = col.sort ?? ((a: T, b: T) => compare(valueOf(col, a), valueOf(col, b)))
    return rows.slice().sort((a, b) => (rank ? rank(a) - rank(b) : 0) || dir * cmp(a, b))
  })

  function toggleSort(c: Column<T>) {
    if (c.sortable === false) return
    if (sortKey === c.key) sortDir = sortDir === 'asc' ? 'desc' : 'asc'
    else {
      sortKey = c.key
      sortDir = 'asc'
    }
  }

  // ── 비활성·선택 ──
  const offOf = (r: T) => (disabled ? disabled(r) : false)
  const selectableKeys = $derived(sorted.filter((r) => !offOf(r)).map(rowKey))
  const allChecked = $derived(selected.size > 0 && selected.size >= selectableKeys.length)
  const someChecked = $derived(selected.size > 0 && !allChecked)

  // 목록이 바뀌면 사라진 행·비활성이 된 행을 선택에서 뺀다
  $effect(() => {
    const ok = new Set(selectableKeys)
    const cur = untrack(() => selected)
    let drop = false
    for (const k of cur) if (!ok.has(k)) drop = true
    if (drop) setSelection(new Set([...cur].filter((k) => ok.has(k))))
  })

  function setSelection(next: Set<string>) {
    selected = next
    onSelectionChange?.(next)
  }

  function selectRange(fromKey: string | null, toIdx: number, add: boolean) {
    const fromIdx = fromKey == null ? toIdx : Math.max(0, sorted.findIndex((r) => rowKey(r) === fromKey))
    const [a, b] = fromIdx < toIdx ? [fromIdx, toIdx] : [toIdx, fromIdx]
    const next = add ? new Set(selected) : new Set<string>()
    for (let i = a; i <= b; i++) if (!offOf(sorted[i])) next.add(rowKey(sorted[i]))
    setSelection(next)
  }

  function toggleKey(k: string) {
    const next = new Set(multiple ? selected : [])
    if (selected.has(k)) next.delete(k)
    else next.add(k)
    setSelection(next)
  }

  function headerToggle() {
    setSelection(allChecked ? new Set() : new Set(selectableKeys))
  }

  // ── 가상 스크롤 ──
  const start = $derived(Math.max(0, Math.floor(scrollTop / rowHeight) - OVERSCAN))
  const end = $derived(Math.min(sorted.length, Math.ceil((scrollTop + viewH) / rowHeight) + OVERSCAN))
  const visible = $derived(sorted.slice(start, end))

  function ensureVisible(i: number) {
    const top = i * rowHeight
    const bodyH = scroller.clientHeight - headH
    if (top < scroller.scrollTop) scroller.scrollTop = top
    else if (top + rowHeight > scroller.scrollTop + bodyH) scroller.scrollTop = top + rowHeight - bodyH
  }

  // ── 이벤트(표 하나에 모아 받는다) ──
  const rowAt = (e: Event) => {
    const el = (e.target as HTMLElement).closest<HTMLElement>('[data-index]')
    return el ? Number(el.dataset.index) : -1
  }

  function onclick(e: MouseEvent) {
    const i = rowAt(e)
    if (i < 0) return
    const r = sorted[i]
    const k = rowKey(r)
    focusKey = k
    if (!selectable || offOf(r)) return
    const onCheck = !!(e.target as HTMLElement).closest('.selcell')
    if (e.shiftKey && multiple) selectRange(anchorKey, i, e.ctrlKey || e.metaKey)
    else if (onCheck || ((e.ctrlKey || e.metaKey) && multiple)) {
      toggleKey(k)
      anchorKey = k
    } else {
      setSelection(new Set([k]))
      anchorKey = k
    }
  }

  function ondblclick(e: MouseEvent) {
    const i = rowAt(e)
    if (i >= 0 && !offOf(sorted[i]) && !(e.target as HTMLElement).closest('.selcell')) onActivate?.(sorted[i])
  }

  function onctx(e: MouseEvent) {
    if (!oncontextmenu) return
    e.preventDefault()
    const i = rowAt(e)
    const r = i >= 0 ? sorted[i] : null
    if (r) {
      const k = rowKey(r)
      focusKey = k
      if (selectable && !offOf(r) && !selected.has(k)) {
        setSelection(new Set([k]))
        anchorKey = k
      }
    }
    oncontextmenu(e, r, selected)
  }

  function ondrag(e: DragEvent) {
    if (!ondragstart) return
    const i = rowAt(e)
    if (i < 0 || offOf(sorted[i])) {
      e.preventDefault()
      return
    }
    const k = rowKey(sorted[i])
    let keys = selected
    if (!selected.has(k)) {
      keys = new Set([k])
      setSelection(keys)
      anchorKey = k
    }
    focusKey = k
    ondragstart(e, keys)
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.target as HTMLElement).closest('.head')) return // 머리의 단추·체크는 제 키를 쓴다
    const n = sorted.length
    if (!n) return
    const cur = focusKey == null ? -1 : sorted.findIndex((r) => rowKey(r) === focusKey)
    const page = Math.max(1, Math.floor((scroller.clientHeight - headH) / rowHeight) - 1)
    let next = -1
    switch (e.key) {
      case 'ArrowDown':
        next = Math.min(n - 1, cur + 1)
        break
      case 'ArrowUp':
        next = Math.max(0, cur < 0 ? 0 : cur - 1)
        break
      case 'PageDown':
        next = Math.min(n - 1, cur + page)
        break
      case 'PageUp':
        next = Math.max(0, cur - page)
        break
      case 'Home':
        next = 0
        break
      case 'End':
        next = n - 1
        break
      case ' ':
        if (cur >= 0 && selectable && !offOf(sorted[cur])) {
          e.preventDefault()
          toggleKey(rowKey(sorted[cur]))
          anchorKey = focusKey
        }
        return
      case 'Enter':
        if (cur >= 0 && !offOf(sorted[cur])) onActivate?.(sorted[cur])
        return
      case 'Escape':
        if (selected.size) setSelection(new Set())
        return
      case 'a':
      case 'A':
        if ((e.ctrlKey || e.metaKey) && selectable && multiple) {
          e.preventDefault()
          setSelection(new Set(selectableKeys))
        }
        return
      case 'ContextMenu':
      case 'F10':
        if (e.key === 'F10' && !e.shiftKey) return
        if (cur >= 0 && oncontextmenu) {
          e.preventDefault()
          const el = scroller.querySelector<HTMLElement>(`[data-index="${cur}"]`)
          const b = el?.getBoundingClientRect() ?? scroller.getBoundingClientRect()
          oncontextmenu(new MouseEvent('contextmenu', { clientX: b.left + 24, clientY: b.bottom }), sorted[cur], selected)
        }
        return
      default:
        return
    }
    e.preventDefault()
    const r = sorted[next]
    focusKey = rowKey(r)
    ensureVisible(next)
    if (!selectable || e.ctrlKey || e.metaKey) return
    if (e.shiftKey && multiple) selectRange(anchorKey ?? focusKey, next, false)
    else if (!offOf(r)) {
      setSelection(new Set([focusKey]))
      anchorKey = focusKey
    }
  }

  // 넘친 칸에만 전체 글자 툴팁
  function onpointerover(e: PointerEvent) {
    const cell = (e.target as HTMLElement).closest<HTMLElement>('.cell')
    if (!cell || cell.title) return
    const inner = (cell.firstElementChild as HTMLElement | null) ?? cell
    if (inner.scrollWidth > inner.clientWidth || cell.scrollWidth > cell.clientWidth) cell.title = cell.textContent?.trim() ?? ''
  }

  // ── 컬럼 폭 끌기 ──
  let resizing: { key: string; x: number; w: number; min: number } | null = null
  function onpointerdown(e: PointerEvent) {
    const h = (e.target as HTMLElement).closest<HTMLElement>('.resize')
    if (!h) return
    const c = columns.find((c) => c.key === h.dataset.key)
    if (!c) return
    e.preventDefault()
    h.setPointerCapture(e.pointerId)
    resizing = { key: c.key, x: e.clientX, w: widthOf(c), min: c.minWidth ?? 50 }
  }
  function onpointermove(e: PointerEvent) {
    if (!resizing) return
    custom[resizing.key] = Math.max(resizing.min, Math.round(resizing.w + e.clientX - resizing.x))
  }
  function onpointerup() {
    if (!resizing) return
    resizing = null
    if (storageKey) saveColumnWidths(storageKey, columns.map(widthOf))
  }

  const focusIdx = $derived(focusKey == null ? -1 : sorted.findIndex((r) => rowKey(r) === focusKey))
  const activeId = $derived(focusIdx >= start && focusIdx < end ? `${gid}-r${focusIdx}` : undefined)
  const fmt = (v: unknown) => (v == null ? '' : String(v))
</script>

<div
  class="dt"
  class:striped
  bind:this={scroller}
  bind:clientHeight={viewH}
  {id}
  role="grid"
  aria-label={label}
  aria-rowcount={sorted.length + 1}
  aria-colcount={columns.length + (selectable ? 1 : 0)}
  aria-multiselectable={selectable && multiple ? true : undefined}
  aria-activedescendant={activeId}
  aria-busy={loading || undefined}
  tabindex="0"
  onscroll={() => (scrollTop = scroller.scrollTop)}
  {onclick}
  {ondblclick}
  oncontextmenu={onctx}
  ondragstart={ondrag}
  {onkeydown}
  {onpointerover}
  {onpointerdown}
  {onpointermove}
  {onpointerup}
  onpointercancel={onpointerup}
  style:--row-h="{rowHeight}px"
>
  <div class="inner" style:min-width="{totalW}px">
    <div class="head" role="row" aria-rowindex={1} style:grid-template-columns={template} bind:clientHeight={headH}>
      {#if selectable}
        <div class="hcell selcell" role="columnheader">
          {#if multiple}
            <Checkbox
              checked={allChecked}
              indeterminate={someChecked}
              disabled={!selectableKeys.length}
              aria-label={t('common.selectAll')}
              onchange={headerToggle}
            />
          {/if}
        </div>
      {/if}
      {#each columns as c (c.key)}
        {@const sorting = sortKey === c.key}
        <div
          class="hcell {c.align ?? 'left'}"
          role="columnheader"
          aria-sort={sorting ? (sortDir === 'asc' ? 'ascending' : 'descending') : undefined}
        >
          {#if c.sortable === false}
            <span class="htext">{c.title}</span>
          {:else}
            <button type="button" class="hbtn" onclick={() => toggleSort(c)}>
              <span class="htext">{c.title}</span>
              {#if sorting}
                {#if sortDir === 'asc'}<ArrowUp size={12} aria-hidden="true" />{:else}<ArrowDown size={12} aria-hidden="true" />{/if}
              {/if}
            </button>
          {/if}
          <span class="resize" data-key={c.key} aria-hidden="true"></span>
        </div>
      {/each}
    </div>

    <div class="body" style:height="{sorted.length * rowHeight}px">
      {#each visible as r, j (rowKey(r))}
        {@const i = start + j}
        {@const k = rowKey(r)}
        {@const off = offOf(r)}
        {@const sel = selected.has(k)}
        <div
          class="row"
          class:sel
          class:off={!!off}
          class:focused={focusKey === k}
          class:alt={i % 2 === 1}
          id="{gid}-r{i}"
          role="row"
          data-index={i}
          aria-rowindex={i + 2}
          aria-selected={selectable ? sel : undefined}
          aria-disabled={off ? true : undefined}
          draggable={ondragstart && !off ? 'true' : undefined}
          title={typeof off === 'string' ? off : undefined}
          style:transform="translateY({i * rowHeight}px)"
          style:grid-template-columns={template}
        >
          {#if selectable}
            <div class="cell selcell" role="gridcell">
              <span class="box" class:on={sel} aria-hidden="true">
                {#if sel}<svg viewBox="0 0 16 16"><path d="M3.5 8.5l3 3 6-7" /></svg>{/if}
              </span>
            </div>
          {/if}
          {#each columns as c (c.key)}
            <div class="cell {c.align ?? 'left'}" role="gridcell">
              {#if c.cell}{@render c.cell(r)}{:else}<span class="txt">{fmt(valueOf(c, r))}</span>{/if}
            </div>
          {/each}
        </div>
      {/each}
    </div>

    {#if loading && !sorted.length}
      <div class="overlay"><Spinner size={20} label={t('common.loading')} /></div>
    {:else if !sorted.length}
      <div class="overlay">
        {#if empty}{@render empty()}{:else}<EmptyState compact icon={Inbox} title={emptyText ?? t('common.empty')} />{/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .dt {
    position: relative;
    container-type: inline-size; /* 빈 표 안내가 보이는 폭을 안다 */
    height: 100%;
    min-height: 0; /* 좁은 칸에서도 표 안에서 스크롤한다(최소 높이는 놓는 쪽이 정한다) */
    overflow: auto;
    scrollbar-gutter: stable;
    background: var(--color-bg);
    font-size: var(--fs-body);
    outline: none;
  }
  .inner {
    position: relative;
    min-height: 100%;
    display: flex;
    flex-direction: column;
  }
  .head {
    position: sticky;
    top: 0;
    z-index: var(--z-sticky);
    display: grid;
    height: 32px;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
  }
  .hcell {
    position: relative;
    display: flex;
    align-items: center;
    min-width: 0;
    padding: 0 var(--sp-2);
    color: var(--color-text-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
  }
  .hbtn {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    min-width: 0;
    max-width: 100%;
    height: 24px;
    margin: 0 -4px;
    padding: 0 4px;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  .hbtn:hover {
    background: var(--color-hover);
    color: var(--color-text);
  }
  .hbtn:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }
  .hcell.right {
    justify-content: flex-end;
  }
  .hcell.right .hbtn {
    margin-left: auto;
    flex-direction: row-reverse;
  }
  .hcell.center {
    justify-content: center;
  }
  .htext {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resize {
    position: absolute;
    top: 6px;
    bottom: 6px;
    right: 0;
    width: 7px;
    transform: translateX(3px);
    z-index: 1;
    cursor: col-resize;
    touch-action: none;
  }
  .resize::after {
    content: '';
    position: absolute;
    left: 3px;
    top: 0;
    bottom: 0;
    width: 1px;
    background: var(--color-border);
  }
  .hcell:last-child .resize {
    transform: none; /* 마지막 손잡이가 표 밖으로 나가 가로 스크롤을 만들지 않게 */
  }
  .resize:hover::after {
    width: 2px;
    left: 2px;
    background: var(--color-focus);
  }
  .body {
    position: relative;
    flex: none;
  }
  .row {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    display: grid;
    height: var(--row-h);
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
    cursor: default;
    user-select: none;
  }
  .striped .row.alt {
    background: var(--color-row-alt);
  }
  .row:hover {
    background: var(--color-hover);
  }
  .row.sel {
    background: var(--color-selected);
  }
  .row.sel:hover {
    background: var(--color-selected-hover);
  }
  .row.off {
    color: var(--color-text-disabled);
  }
  .dt:focus-visible .row.focused {
    box-shadow: inset 0 0 0 2px var(--color-focus);
  }
  .cell {
    display: flex;
    align-items: center;
    min-width: 0;
    padding: 0 var(--sp-2);
    overflow: hidden;
    white-space: nowrap;
  }
  .cell.right {
    justify-content: flex-end;
  }
  .cell.center {
    justify-content: center;
  }
  .txt {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .selcell {
    justify-content: center;
    padding: 0;
  }
  .box {
    display: inline-flex;
    width: 16px;
    height: 16px;
    border: 1px solid var(--color-border-strong);
    border-radius: var(--r-sm);
    background: var(--color-surface-raised);
  }
  .box.on {
    background: var(--color-accent-fill);
    border-color: var(--color-accent-fill);
  }
  .off .box {
    opacity: 0.4;
  }
  .box svg {
    width: 100%;
    height: 100%;
    fill: none;
    stroke: var(--color-text-on-accent);
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  /* 표가 창보다 넓어도 안내는 보이는 폭의 가운데에 있고, 가로로 밀어도 따라온다 */
  .overlay {
    position: sticky;
    left: 0;
    flex: 1;
    width: min(100cqi, 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }
  .overlay :global(button) {
    pointer-events: auto;
  }
</style>
