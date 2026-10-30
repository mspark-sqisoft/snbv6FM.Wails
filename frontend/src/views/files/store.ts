// 파일전송 화면이 떠 있지 않아도 남는 상태와, 화면끼리 함께 쓰는 값.

import { writable } from 'svelte/store'
import { Ev, on } from '../../lib/api'
import type { Tone } from '../../lib/ui'

/** Go files.Kind* → 칩 색. */
export const KIND_TONE: Record<string, Tone> = {
  danger: 'danger',
  pending: 'info',
  success: 'success',
  warning: 'warning',
  accent: 'accent',
  muted: 'neutral',
}

/** 표에서 체크한 행(server_str). 탐색기 「받기」 메뉴가 쓰므로 화면을 떠나도 남긴다. */
export const fileSelection = writable(new Set<string>())

/** 탐색기 「받기」 메뉴가 고른 받을 폴더. Host 가 처리하고 비운다. */
export const pendingDownload = writable<string | null>(null)

// 로그인 직후 Host 가 붙기 전에 올 수 있어 앱이 뜰 때부터 듣는다(로그인 전 요청은 Go 가 들고 있다가 로그인 뒤에 낸다).
on<{ mode: string; path: string }>(Ev.secondInstance, (d) => {
  if (d.mode === 'download') pendingDownload.set(d.path)
})
