// 메뉴 모델: 망·역할별 메뉴 항목과 화면 연결(docs/port/12 5.2, MAIN-020·021).
//   업무망 결재자  : 파일전송·전송내역·결재처리·결재내역·대리결재자·로그·설정 (7, 정책은 숨김)
//   업무망 대리결재자: 파일전송·전송내역·결재처리·로그·설정 (5)
//   일반 사용자, 그리고 인터넷망의 모든 사용자: 파일전송·전송내역·로그·설정 (4)
// 역할은 로그인 응답 aprv_yn(isApprover)·replace_yn(isProxyApprover), 망은 fmenv.ini LOC(1 업무망).
// 결재요청·정책 화면은 메뉴에 없다(결재요청은 파일전송에서 여는 흐름, 정책은 원본도 메뉴에서 숨김) ― 화면 id 로만 연다.

import type { Component } from 'svelte'
import type { Icon } from './lib/ui/types'
import type { Role } from './lib/stores/session'

import FolderSync from '@lucide/svelte/icons/folder-sync'
import FileClock from '@lucide/svelte/icons/file-clock'
import Stamp from '@lucide/svelte/icons/stamp'
import ClipboardCheck from '@lucide/svelte/icons/clipboard-check'
import UserCheck from '@lucide/svelte/icons/user-check'
import ScrollText from '@lucide/svelte/icons/scroll-text'
import Shield from '@lucide/svelte/icons/shield'
import Settings from '@lucide/svelte/icons/settings'
import Send from '@lucide/svelte/icons/send'

import FilesView from './views/files/FilesView.svelte'
import RequestView from './views/approval/RequestView.svelte'
import ApproveView from './views/approval/ApproveView.svelte'
import ProxyView from './views/approval/ProxyView.svelte'
import TransHistoryView from './views/history/TransHistoryView.svelte'
import ApprovalHistoryView from './views/history/ApprovalHistoryView.svelte'
import PolicyView from './views/policy/PolicyView.svelte'
import LogView from './views/log/LogView.svelte'
import SettingsView from './views/settings/SettingsView.svelte'

export type RouteId =
  | 'files'
  | 'request'
  | 'approve'
  | 'proxy'
  | 'transHistory'
  | 'approvalHistory'
  | 'policy'
  | 'log'
  | 'settings'

export interface Route {
  id: RouteId
  /** 번역 키(t(labelKey)) */
  labelKey: string
  icon: Icon
  component: Component
}

export const routes: Record<RouteId, Route> = {
  files: { id: 'files', labelKey: 'common.menu.files', icon: FolderSync, component: FilesView },
  request: { id: 'request', labelKey: 'common.menu.request', icon: Send, component: RequestView },
  approve: { id: 'approve', labelKey: 'common.menu.approve', icon: Stamp, component: ApproveView },
  proxy: { id: 'proxy', labelKey: 'common.menu.proxy', icon: UserCheck, component: ProxyView },
  transHistory: { id: 'transHistory', labelKey: 'common.menu.transHistory', icon: FileClock, component: TransHistoryView },
  approvalHistory: {
    id: 'approvalHistory',
    labelKey: 'common.menu.approvalHistory',
    icon: ClipboardCheck,
    component: ApprovalHistoryView,
  },
  policy: { id: 'policy', labelKey: 'common.menu.policy', icon: Shield, component: PolicyView },
  log: { id: 'log', labelKey: 'common.menu.log', icon: ScrollText, component: LogView },
  settings: { id: 'settings', labelKey: 'common.menu.settings', icon: Settings, component: SettingsView },
}

const MENUS: Record<Role, RouteId[]> = {
  approver: ['files', 'transHistory', 'approve', 'approvalHistory', 'proxy', 'log', 'settings'],
  proxy: ['files', 'transHistory', 'approve', 'log', 'settings'],
  user: ['files', 'transHistory', 'log', 'settings'],
}

/** 역할의 메뉴(인터넷망이면 session store 가 이미 role 을 'user' 로 준다). */
export function menuFor(role: Role): Route[] {
  return MENUS[role].map((id) => routes[id])
}

/** 로그인 직후 화면(MAIN-025). */
export const HOME: RouteId = 'files'
