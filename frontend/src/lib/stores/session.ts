// 로그인한 사용자·망·권한. 로그인 화면이 login() 으로 채우고, 로그아웃·세션 종료 이벤트가 비운다.

import { derived, get, writable } from 'svelte/store'
import { Application, Window } from '@wailsio/runtime'
import { AuthService, type LoginInfo } from '../api'
import { settings } from './ui'

/** 망 구분(fmenv.ini LOC): 1 업무망, 0 인터넷망. */
export const LOC_BUSINESS = 1
export const LOC_INTERNET = 0

/** 지금 로그인한 사용자(없으면 null). */
export const user = writable<LoginInfo | null>(null)

export type Role = 'approver' | 'proxy' | 'user'

/** 화면이 자주 묻는 것을 한 곳에서 판정한다. */
export const session = derived(user, (u) => {
  const loc = u?.server.loc ?? -1
  const isBusiness = loc === LOC_BUSINESS
  // 원본 HomePage.qml: location === 1 ? (결재자 ? 결재자 : 대리 ? 대리 : 일반) : 일반
  const role: Role = !isBusiness ? 'user' : u?.isApprover ? 'approver' : u?.isProxyApprover ? 'proxy' : 'user'
  return {
    user: u,
    loggedIn: !!u,
    loc,
    isBusiness,
    networkName: u?.server.name ?? '',
    /** 전송 방향 글자(fmenv.ini NETWORK, 예: 업무망->인터넷망) */
    direction: u?.server.network ?? '',
    role,
  }
})

export function setUser(u: LoginInfo | null) {
  user.set(u)
}

/** 참이면 Shell 이 「로그아웃 처리 방식 확인」 창(shell/LogoutDialog.svelte)을 띄운다. */
export const logoutAsk = writable(false)

/**
 * 머리줄 [로그아웃] 단추: 설정 logoutConfirm 이 켜져 있으면 확인 창을 먼저, 아니면 바로 logoutProcess(원본 MAIN-044).
 */
export function requestLogout() {
  if (get(settings)?.logoutConfirm ?? true) logoutAsk.set(true)
  else void logoutProcess()
}

/**
 * 로그아웃 처리 방식(설정 logoutType)을 따른다(원본 logoutProcess, docs/port/12 5.5):
 * 0 로그아웃(로그인 화면), 1 로그아웃 뒤 프로그램 종료, 2 창만 숨김(로그인 유지, 트레이로 되찾는다).
 * 자동 로그아웃·로그인 실패에는 쓰지 않는다(그때는 AuthService.Logout 만). 연결 끊김 안내 뒤에는 쓴다(확인 창 없이).
 */
export async function logoutProcess(type: number = get(settings)?.logoutType ?? 0) {
  if (type === 2) {
    await Window.Hide()
    return
  }
  await AuthService.Logout()
  setUser(null)
  if (type === 1) await Application.Quit()
}
