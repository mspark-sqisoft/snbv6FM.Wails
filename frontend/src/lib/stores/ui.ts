// 화면 전역 상태: 설정 사본·테마·언어·알림 띠·확인 창 줄·전역 바쁨 표시.

import { get, writable } from 'svelte/store'
import { SettingsService, errorText, type Settings } from '../api'
import { locale, t, type Locale } from '../i18n'
import type { Severity } from '../ui/types'
import type { RouteId } from '../../routes'
import { designOf, type DesignId } from '../../designs/registry'
import { mergeAlert } from './dialogQueue'

// ── 설정 ──────────────────────────────────────────────

/** Go 설정 저장소의 사본(loadSettings 뒤부터 값이 있다). 직접 set 하지 말고 updateSettings 를 쓴다. */
export const settings = writable<Settings | null>(null)

/** 설정을 읽어 테마·언어를 적용한다(앱 시작 때 한 번, settings:changed 이벤트 때). */
export async function loadSettings() {
  try {
    applySettings(await SettingsService.Get())
  } catch (e) {
    console.warn('설정을 읽지 못했다', e) // 브라우저 단독 실행(npm run dev) 등 ― 기본값으로 그린다
  }
}

export function applySettings(s: Settings) {
  settings.set(s)
  theme.set(normTheme(s.theme))
  design.set(designOf(s.design).id)
  applyLook()
  locale.set(s.language === 1 ? 'en' : 'ko')
}

let chain: Promise<unknown> = Promise.resolve()

/**
 * 설정 하나를 고쳐 저장한다. Go 의 최신 값을 읽어 fn 으로 고친 뒤 통째로 쓴다(차례대로, 겹치지 않게).
 * 실패하면 알림 띠로 알린다(원본 SET-104 해소).
 */
export function updateSettings(fn: (s: Settings) => void): Promise<void> {
  const run = chain.then(async () => {
    try {
      const s = await SettingsService.Get()
      fn(s)
      await SettingsService.Set(s)
      settings.set(s)
    } catch (e) {
      toast(`${t('common.error.settingsSave')} ${errorText(e)}`, 3)
    }
  })
  chain = run
  return run
}

// ── 테마 ──────────────────────────────────────────────

export type Theme = 'system' | 'light' | 'dark'
export const theme = writable<Theme>('system')

const normTheme = (v: string): Theme => (v === 'light' || v === 'dark' ? v : v === 'system' ? 'system' : 'dark')

/** html 의 data-design·data-theme 를 지금 값에 맞춘다. 디자인이 밝기를 정하면 테마 설정은 저장된 채 쉰다. */
function applyLook() {
  const root = document.documentElement
  const d = designOf(get(design))
  if (d.scheme) {
    root.dataset.design = d.id
    root.dataset.theme = d.scheme
    return
  }
  delete root.dataset.design
  const v = get(theme)
  if (v === 'system') delete root.dataset.theme
  else root.dataset.theme = v
}

export function setTheme(v: Theme) {
  theme.set(v)
  applyLook()
  void updateSettings((s) => (s.theme = v))
}

// ── 디자인 ────────────────────────────────────────────

export const design = writable<DesignId>('standard')

export function setDesign(id: DesignId) {
  design.set(id)
  applyLook()
  void updateSettings((s) => (s.design = id))
}

/** 시스템 → 라이트 → 다크 → 시스템 차례로 바꾼다(머리줄 단추). */
export function cycleTheme() {
  const order: Theme[] = ['system', 'light', 'dark']
  setTheme(order[(order.indexOf(get(theme)) + 1) % order.length])
}

// ── 언어 ──────────────────────────────────────────────

export function setLanguage(l: Locale) {
  locale.set(l)
  void updateSettings((s) => (s.language = l === 'en' ? 1 : 0))
}

// ── 컬럼 폭(DataTable storageKey) ─────────────────────

export function getColumnWidths(key: string): number[] | undefined {
  return get(settings)?.columnWidths?.[key] ?? undefined
}

export function saveColumnWidths(key: string, widths: number[]) {
  void updateSettings((s) => {
    s.columnWidths = { ...(s.columnWidths ?? {}), [key]: widths }
  })
}

// ── 알림 띠 ───────────────────────────────────────────

export interface ToastNotice {
  id: number
  severity: Severity
  text: string
}

/** 지금 보이는 알림 띠 하나(App.svelte 가 Toast 로 그린다). */
export const currentToast = writable<ToastNotice | null>(null)
let toastSeq = 0
let toastTimer: ReturnType<typeof setTimeout> | undefined

/** 알림 띠를 띄운다. 떠 있으면 바꿔치기하고 4초 시계를 다시 잰다(원본 Notify.qml, 줄 세우지 않는다). */
export function toast(text: string, severity: Severity = 0) {
  clearTimeout(toastTimer)
  currentToast.set({ id: ++toastSeq, severity, text })
  toastTimer = setTimeout(closeToast, 4000)
}

export function closeToast() {
  clearTimeout(toastTimer)
  currentToast.set(null)
}

// ── 확인 창 줄 ────────────────────────────────────────

export interface DialogRequest {
  id: number
  title: string
  text: string
  okLabel?: string
  /** null 이면 [확인] 하나(안내 창) */
  cancelLabel?: string | null
  danger?: boolean
  /** 거짓이면 Esc·바깥 누름으로 닫히지 않는다 */
  dismissible?: boolean
  /** 같은 제목의 안내가 또 오면 새 창 대신 글을 바꾼다(goAlert) */
  merge?: boolean
  resolve: (ok: boolean) => void
}

/** 떠야 할 확인 창들. 맨 앞 하나만 보이고, 닫히면 다음 것이 뜬다(앞 안내를 놓치지 않게 줄 세운다). */
export const dialogs = writable<DialogRequest[]>([])
let dialogSeq = 0

type DialogOptions = Omit<DialogRequest, 'id' | 'resolve' | 'title'> & { title?: string }

/** 확인·취소를 묻는다. 확인이면 true. */
export function confirm(o: DialogOptions): Promise<boolean> {
  return new Promise((resolve) => {
    dialogs.update((q) => [...q, { title: t('common.dialog.info'), ...o, id: ++dialogSeq, resolve }])
  })
}

/** 안내 창([확인] 하나). 닫히면 끝난다. */
export async function alert(o: Omit<DialogOptions, 'cancelLabel'>): Promise<void> {
  await confirm({ ...o, cancelLabel: null })
}

/**
 * Go 가 보낸 안내 창. 같은 제목의 창이 줄에 있으면 글만 바꾼다(원본 확인 창은 하나라 뒤 안내가 앞을 덮는다 ―
 * 자동 전송 실패가 주기마다 쌓이지 않게). 닫힌 뒤 할 일이 있는 창에는 쓰지 않는다.
 */
export function goAlert(title: string, text: string) {
  const merged = mergeAlert(get(dialogs), title, text)
  if (merged) dialogs.set(merged)
  else void alert({ title, text, merge: true })
}

/** 맨 앞 창을 닫는다(App.svelte 가 부른다). */
export function closeDialog(id: number, ok: boolean) {
  const q = get(dialogs)
  const d = q.find((x) => x.id === id)
  dialogs.set(q.filter((x) => x.id !== id))
  d?.resolve(ok)
}

/** 로그아웃 때 떠 있는 창을 모두 취소로 닫는다(원본 closeSessionPopups). */
export function clearDialogs() {
  const q = get(dialogs)
  dialogs.set([])
  q.forEach((d) => d.resolve(false))
}

// ── 지금 화면 ─────────────────────────────────────────

/** 지금 본문에 보이는 화면 id(routes.ts 의 RouteId). 화면끼리 옮길 때 navigate 를 쓴다. */
export const route = writable<RouteId>('files')

export function navigate(id: RouteId) {
  route.set(id)
}

// ── 전역 바쁨 ─────────────────────────────────────────

/** 0 보다 크면 머리줄 아래에 얇은 진행 막대를 보인다. */
export const busy = writable(0)

/** fn 이 도는 동안 전역 바쁨 표시를 켠다. */
export async function withBusy<R>(fn: () => Promise<R>): Promise<R> {
  busy.update((n) => n + 1)
  try {
    return await fn()
  } finally {
    busy.update((n) => n - 1)
  }
}
