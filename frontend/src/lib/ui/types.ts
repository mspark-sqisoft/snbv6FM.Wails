import type { Component, Snippet } from 'svelte'
import type { LucideProps } from '@lucide/svelte'

/** 아이콘 컴포넌트(@lucide/svelte 의 아이콘 하나, 예: `import Upload from '@lucide/svelte/icons/upload'`). */
export type Icon = Component<LucideProps>

/** 색 묶음. Badge·StatusChip·Toast 가 함께 쓴다. */
export type Tone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info' | 'business' | 'internet'

/** 알림 띠 심각도(Go svc.SevInfo~SevError 와 같은 값). */
export type Severity = 0 | 1 | 2 | 3

export const severityTone: Record<Severity, Tone> = { 0: 'info', 1: 'success', 2: 'warning', 3: 'danger' }

/** DataTable 컬럼 정의. */
export interface Column<T> {
  /** 컬럼 열쇠. 정렬 상태·폭 저장의 이름이고, value 가 없으면 row[key] 를 값으로 쓴다 */
  key: string
  /** 머리 글자(번역한 글자를 넘긴다) */
  title: string
  /** 기본 폭(px). 끌어서 바꾸면 storageKey 로 저장된다. 기본 120 */
  width?: number
  /** 최소 폭(px). 기본 50(원본 표 규칙) */
  minWidth?: number
  align?: 'left' | 'center' | 'right'
  /** 머리 누름으로 정렬할 수 있는가(기본 참) */
  sortable?: boolean
  /** 정렬 비교 함수. 없으면 value(또는 row[key])를 한국어 자연 정렬로 비교한다 */
  sort?: (a: T, b: T) => number
  /** 셀의 값(정렬·말줄임 툴팁·기본 표시에 쓴다) */
  value?: (row: T) => string | number | null | undefined
  /** 셀을 직접 그린다. 없으면 value 를 글자로 보인다 */
  cell?: Snippet<[T]>
  /** 마지막 남는 폭을 이 컬럼이 차지한다(하나만). 없으면 마지막 컬럼 */
  grow?: boolean
}

/** ContextMenu 항목. separator 면 나머지는 무시한다. */
export interface MenuItem {
  label?: string
  icon?: Icon
  /** 오른쪽 작은 글자(단축키 등) */
  hint?: string
  disabled?: boolean
  danger?: boolean
  separator?: boolean
  onSelect?: () => void
}

/** Select 의 선택지. */
export interface Option<V = string | number> {
  value: V
  label: string
  disabled?: boolean
}

/** Tabs 의 탭 하나. */
export interface TabItem {
  id: string
  label: string
  icon?: Icon
  disabled?: boolean
  /** 탭 옆 작은 숫자(건수) */
  count?: number
}
