// 전송 영역 화면 상태: 대기열 행(Go transfer.Item 사본)·서랍 열림. Host 가 Go 사건으로 채우고 서랍·서버파일 화면이 읽는다.

import { derived, writable } from 'svelte/store'
import type { TransferService } from '../../lib/api'
import { t } from '../../lib/i18n'

/** Go internal/transfer.Item(서랍 한 줄). */
export type Item = NonNullable<Awaited<ReturnType<typeof TransferService.Items>>>[number]

/** Go internal/transfer.Tick(200ms 묶음의 진행값). */
export interface Tick {
  id: number
  phase: string
  done: number
  total: number
  speed: number
}

/** Go svc.ConflictAsk(파일 중복 확인 질문). */
export interface ConflictAsk {
  gen: number
  name: string
  path: string
  folder: boolean
  index: number
  total: number
}

/** Go internal/svc/transfer.go 의 전송 영역 사건(events.go 밖). */
export const EV_OPEN = 'transfer:open'
export const EV_UPLOAD_QUEUED = 'transfer:upload-queued'
export const EV_CONFLICT_CLOSE = 'transfer:conflict-close'

export const items = writable<Item[]>([])
export const drawerOpen = writable(false)

/** 진행값 묶음을 행에 붙인다(목록 전체는 transfer:state 로 따로 온다). */
export function applyTicks(ticks: Tick[] | null) {
  if (!ticks?.length) return
  const byId = new Map(ticks.map((k) => [k.id, k]))
  items.update((list) =>
    list.map((it) => {
      const k = byId.get(it.id)
      return k ? { ...it, phase: k.phase, done: k.done, total: k.total, speed: k.speed } : it
    }),
  )
}

export const live = (it: Item) => it.state === 'pending' || it.state === 'active'

export const pctOf = (it: Item) => {
  const total = it.total || it.size
  return total > 0 ? Math.floor((Math.min(it.done, total) / total) * 100) : 0
}

/** 서랍과 띠의 상태 글자. active 는 실제로 보내거나 받는 중에 쓸 문구다. */
export function statusText(it: Item, active = `${pctOf(it)}%`): string {
  switch (it.state) {
    case 'done':
      return t('transfer.state.done')
    case 'failed':
      return t('transfer.state.failed')
    case 'cancelled':
      return t('transfer.state.cancelled')
  }
  if (it.phase === 'zip') return t('transfer.state.zip', { p: pctOf(it) })
  if (it.state === 'pending') {
    if (it.phase === 'hash') return t('transfer.state.waitHash')
    if (it.wait === 'serial') return t('transfer.state.waitSerial')
    return it.dir === 'download' ? t('transfer.state.waitDownload') : t('transfer.state.wait')
  }
  if (it.phase === 'commit') return t('transfer.state.commit')
  if (it.phase === 'extract') return t('transfer.state.extract', { p: pctOf(it) })
  if (it.phase === 'hash' || it.done === 0) return t('transfer.state.prepare')
  return active
}

/** 머리줄 요약: 대기·진행·완료·실패·취소 건수와 진행 중인 것의 전체 진행률. */
export const summary = derived(items, (list) => {
  const s = { pending: 0, active: 0, done: 0, failed: 0, cancelled: 0, bytes: 0, total: 0 }
  for (const it of list) {
    s[it.state as 'pending' | 'active' | 'done' | 'failed' | 'cancelled']++
    if (live(it)) {
      const total = it.total || it.size
      s.total += total
      s.bytes += it.phase === 'send' || it.phase === 'recv' || it.phase === 'commit' ? Math.min(it.done, total) : 0
    }
  }
  return s
})

/** 어떤 서버 파일 쪽 전송이 끝났는가를 알기 위한 끝난 건수(서버파일 화면이 바뀌면 목록을 다시 받는다). */
export const finishedCount = derived(summary, (s) => s.done + s.failed + s.cancelled)
