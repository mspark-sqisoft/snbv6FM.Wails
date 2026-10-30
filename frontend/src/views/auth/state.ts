// 인증 영역 화면끼리 주고받는 상태. 로그인 화면(로그인 전)과 Host(로그인 뒤)가 서로 다른 때 붙으므로 store 로 넘긴다.
// 로그아웃 처리 방식 확인 창은 메인 담당의 shell/LogoutDialog.svelte·lib/stores/session.requestLogout 이 맡는다.

import { writable } from 'svelte/store'
import { AuthService } from '../../lib/api'

/**
 * 로그인 뒤 띄울 비밀번호 변경. Host 가 붙으면서 꺼내 쓰고 비운다.
 * default: 기본 비밀번호(변경 창), expire: 만료 예정(먼저 묻는다), manual: 사용자 정보 팝업의 [비밀번호 변경].
 */
export type PasswordPrompt = { kind: 'default' } | { kind: 'expire'; days: number } | { kind: 'manual' }
export const passwordPrompt = writable<PasswordPrompt | null>(null)

/** 사용자 정보 팝업 등에서 비밀번호 변경 창을 연다. */
export function openPasswordChange() {
  passwordPrompt.set({ kind: 'manual' })
}

/** 강제 변경 창의 [취소]: 쥐고 있던 세션을 닫는다(화면은 이어서 앱을 끝낸다). */
export async function cancelPendingLogin() {
  await AuthService.CancelLogin().catch(() => {})
}
