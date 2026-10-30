// Go 서비스 바인딩을 한 곳에서 다시 내보내고, 오류 글자 다듬기·이벤트 구독 도우미를 둔다.
// 화면은 bindings/ 를 직접 import 하지 않고 여기서 가져온다(바인딩 경로가 바뀌어도 이 파일만 고친다).

import { Events } from '@wailsio/runtime'
import { t } from './i18n'

export * from '../../bindings/snbapp/internal/svc'
export type { AutoTransfer, Hotkey, Server, Settings } from '../../bindings/snbapp/internal/config'
export type { User as ApproverUser } from '../../bindings/snbapp/internal/approval'

/**
 * Go 가 내는 이벤트 이름(internal/svc 의 상수와 같은 글자).
 * Go 쪽에서 이벤트를 RegisterEvent 로 등록하지 않아 타입이 생성되지 않으므로 여기 손으로 둔다.
 */
export const Ev = {
  transferProgress: 'transfer:progress',
  transferState: 'transfer:state',
  downloadConflict: 'transfer:conflict',
  transferOpen: 'transfer:open',
  conflictClose: 'transfer:conflict-close',
  serverFilesChanged: 'files:changed',
  fileList: 'files:list',
  approvalChanged: 'approval:changed',
  policyChanged: 'policy:changed',
  fileCheckOpen: 'policy:check-open',
  fileCheckRow: 'policy:check-row',
  fileCheckResult: 'policy:check-result',
  fileCheckClose: 'policy:check-close',
  zipPasswordAsk: 'policy:zip-password',
  approverSelectAsk: 'approval:select',
  approverSelectClose: 'approval:select-close',
  notice: 'ui:notice',
  dialog: 'ui:dialog',
  sessionEnded: 'session:ended',
  disconnected: 'session:disconnected',
  autoLogout: 'session:timeout',
  passwordChangeAsk: 'session:password',
  secondInstance: 'app:second-instance',
  settingsChanged: 'settings:changed',
  clipCount: 'clip:count',
  trayPolicyReload: 'tray:policy-reload',
} as const

/** events.go 의 값 구조체(화면 쪽 사본). */
export interface Notice {
  severity: 0 | 1 | 2 | 3
  text: string
}
export interface GoDialog {
  id: number
  title: string
  text: string
  onClose: '' | 'exit' | 'logoutProcess'
}
export interface SessionEnded {
  reason: 'logout' | 'idle' | 'disconnected' | 'duplicate' | 'integrity' | 'shutdown'
}
export interface AutoLogout {
  minutes: number
}

/**
 * Go 이벤트 하나를 듣는다. 돌려준 함수를 부르면 그만 듣는다 ―
 * 화면에서는 `onMount(() => on(Ev.serverFilesChanged, refresh))` 처럼 쓰면 화면이 사라질 때 저절로 끊긴다.
 */
export function on<T = unknown>(name: string, cb: (data: T) => void): () => void {
  return Events.On(name, (e) => cb(e.data as T))
}

/** 여러 이벤트를 한꺼번에 듣고, 한 번에 끊는 함수를 돌려준다. */
export function onAll(handlers: Record<string, (data: never) => void>): () => void {
  const offs = Object.entries(handlers).map(([n, cb]) => on(n, cb as (d: unknown) => void))
  return () => offs.forEach((off) => off())
}

/**
 * 바인딩 호출이 던진 오류를 사람이 읽을 문구로 바꾼다.
 * Go 가 화면용으로 만든 한국어 문구(로그인 실패 등)는 그대로 두고, gRPC 상태 글자·래핑 접두어·Go 내부 문구만 다듬는다.
 */
export function errorText(e: unknown): string {
  let s = (e instanceof Error ? e.message : typeof e === 'string' ? e : '').trim()
  if (!s && e && typeof e === 'object' && 'message' in e) s = String((e as { message: unknown }).message)
  if (!s) return t('common.error.unknown')

  if (s.includes('아직 구현하지 않았습니다')) return t('common.error.notImplemented') // svc.ErrNotImplemented
  if (s.includes('로그인하지 않았습니다')) return t('common.error.notLoggedIn') // svc.ErrNotLoggedIn

  // gRPC: "rpc error: code = Unavailable desc = …"
  const g = s.match(/rpc error: code = (\w+) desc = (.*)$/s)
  if (g) {
    const [, code, desc] = g
    if (code === 'Unavailable') return t('common.error.connect')
    if (code === 'DeadlineExceeded') return t('common.error.timeout')
    s = desc.trim() || code
  }
  if (/context deadline exceeded/i.test(s)) return t('common.error.timeout')
  if (/connection refused|no such host|actively refused/i.test(s)) return t('common.error.connect')

  // "Service.Method: 실제 문구" 꼴의 래핑 접두어를 걷는다(한국어 문구가 뒤에 있을 때만)
  const wrapped = s.match(/^[A-Za-z][\w.]*:\s+(.*[가-힣].*)$/s)
  if (wrapped) s = wrapped[1]
  return s
}
