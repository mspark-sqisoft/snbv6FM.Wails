// 번역 누락 검사: src 의 t('키') 가 한국어 목록(lib/i18n/*.ko.ts)에 있는가.
// 영어 목록은 Record<keyof typeof ko, string> 타입이라 빠진 키를 npm run check 가 잡는다 ― 여기서는 한국어 쪽만 본다.
// t(`files.col.${k}`) 처럼 글자를 끼운 키는 앞부분(files.col.)으로 시작하는 키가 하나라도 있는지만 본다.
// 실행: node --experimental-strip-types scripts/check-i18n.mjs   (끝 코드 0 = 누락 없음, 1 = 누락 있음)

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { pathToFileURL } from 'node:url'

const root = new URL('../src/', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1')
const i18n = join(root, 'lib', 'i18n')

const keys = new Set()
for (const f of readdirSync(i18n).filter((f) => f.endsWith('.ko.ts'))) {
  const area = f.slice(0, -'.ko.ts'.length)
  const mod = await import(pathToFileURL(join(i18n, f)).href)
  for (const k of Object.keys(mod.default)) keys.add(`${area}.${k}`)
}

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) yield* walk(p)
    else if (/\.(svelte|ts)$/.test(name) && !p.startsWith(i18n)) yield p
  }
}

const missing = []
const re = /\bt\(\s*(['"`])((?:(?!\1).)*)\1\s*(\+)?/g
for (const file of walk(root)) {
  const text = readFileSync(file, 'utf8')
  for (const m of text.matchAll(re)) {
    const key = m[2]
    const prefix = m[3] ? key : m[1] === '`' && key.includes('${') ? key.slice(0, key.indexOf('${')) : null
    const ok = prefix !== null ? [...keys].some((k) => k.startsWith(prefix)) : keys.has(key)
    if (!ok && /^[\w.${}-]+$/.test(key)) {
      const line = text.slice(0, m.index).split('\n').length
      missing.push(`${relative(root, file)}:${line}  ${key}`)
    }
  }
}

if (missing.length) {
  console.log(`번역 키 누락 ${missing.length}건:\n` + missing.join('\n'))
  process.exit(1)
}
console.log(`번역 키 누락 없음 (한국어 키 ${keys.size}개)`)
