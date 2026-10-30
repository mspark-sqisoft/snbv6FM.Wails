// 번역. 영역마다 lib/i18n/<영역>.ko.ts·<영역>.en.ts 를 두면 여기서 저절로 모은다(import.meta.glob).
// 키는 "<영역>.<파일 안 키>" 다 ― files.ko.ts 의 'col.name' 은 t('files.col.name').
// 한국어가 원문이다(원본 문구 그대로). 영어에 없는 키는 한국어로, 한국어에도 없으면 키 그대로 보이고 개발 모드에서 경고한다.
// t() 는 언어 상태($state)를 읽으므로 화면에서 {t('…')} 로 쓰면 언어를 바꿀 때 저절로 다시 그려진다.

import { writable } from 'svelte/store'
import serverEn from './server-text'

export type Locale = 'ko' | 'en'
type Dict = Record<string, string>

const catalogs: Record<Locale, Dict> = { ko: {}, en: {} }
const modules = import.meta.glob<{ default: Dict }>('./*.{ko,en}.ts', { eager: true })
for (const [path, mod] of Object.entries(modules)) {
  const m = path.match(/^\.\/(.+)\.(ko|en)\.ts$/)
  if (!m) continue
  const [, area, lang] = m
  for (const [k, v] of Object.entries(mod.default)) catalogs[lang as Locale][`${area}.${k}`] = v
}

/** 지금 언어. 바꾸려면 lib/stores/ui.ts 의 setLanguage(설정에도 저장) 를 쓴다. */
export const locale = writable<Locale>('ko')

let current = $state<Locale>('ko')
locale.subscribe((v) => {
  current = v
  if (typeof document !== 'undefined') document.documentElement.lang = v
})

const warned = new Set<string>()
function warnOnce(key: string, why: string) {
  if (!import.meta.env.DEV || warned.has(key)) return
  warned.add(key)
  console.warn(`[i18n] ${why}: ${key}`)
}

/** key 의 글자. params 의 {이름} 자리를 값으로 바꾼다: t('files.done', { n: 3 }). */
export function t(key: string, params?: Record<string, string | number>): string {
  let s = catalogs[current][key]
  if (s === undefined) {
    s = catalogs.ko[key]
    warnOnce(key, s === undefined ? '번역 키가 없습니다' : `${current} 번역이 없어 한국어로 보입니다`)
  }
  if (s === undefined) return key
  if (!params) return s
  return s.replace(/\{(\w+)\}/g, (all, p: string) => (p in params ? String(params[p]) : all))
}

/** Go 가 보낸 고정 상태 글자(한국어)를 지금 언어로. 목록에 없으면 그대로 둔다. */
export function tx(text: string): string {
  return current === 'en' ? (serverEn[text] ?? text) : text
}

/** 지금 언어(템플릿에서 읽으면 바뀔 때 다시 그려진다). */
export function currentLocale(): Locale {
  return current
}
