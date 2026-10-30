// 디자인 토큰 검사: 각 디자인이 색 토큰을 빠짐없이 정의했고 글자 대비가 기준을 넘는가(docs/DESIGNS.md).
// 실행: node scripts/check-designs.mjs   (끝 코드 0 = 통과, 1 = 문제 있음)

import { readFileSync } from 'node:fs'

const src = new URL('../src/', import.meta.url)
const read = (p) => readFileSync(new URL(p, src), 'utf8')

const IDS = ['airlock', 'gongmun', 'departure', 'waybill', 'metro']
const required = [...new Set([...read('lib/ui/tokens.css').split(/:root\[data-theme/)[0].matchAll(/(--color-[\w-]+):/g)].map((m) => m[1]))]

// [글자 토큰들, 바탕 토큰들, 최소 대비]
const PAIRS = [
  [['text', 'text-2', 'text-3'], ['bg', 'bg-chrome', 'surface', 'surface-raised', 'surface-sunken', 'row-alt', 'selected'], 4.5],
  [['text-on-accent'], ['accent-fill', 'accent-fill-hover', 'danger-fill'], 4.5],
  [['accent'], ['bg', 'surface-raised', 'accent-subtle'], 4.5],
  [['success'], ['bg', 'success-subtle'], 4.5],
  [['warning'], ['bg', 'warning-subtle'], 4.5],
  [['danger'], ['bg', 'danger-subtle'], 4.5],
  [['info'], ['bg', 'info-subtle'], 4.5],
  [['neutral'], ['bg', 'neutral-subtle'], 4.5],
  [['net-business'], ['bg-chrome', 'net-business-subtle'], 4.5],
  [['net-internet'], ['bg-chrome', 'net-internet-subtle'], 4.5],
  [['focus'], ['bg', 'surface-raised'], 3],
]

function rgb(v) {
  const m = v.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i)
  if (!m) return null
  const h = m[1].length === 3 ? [...m[1]].map((c) => c + c).join('') : m[1]
  return [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
}

function lum(c) {
  const [r, g, b] = c.map((x) => {
    const s = x / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function ratio(a, b) {
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

let bad = 0
const fail = (m) => {
  bad++
  console.log(m)
}

for (const id of IDS) {
  const css = read(`designs/${id}/design.css`)
  const head = css.match(new RegExp(`:root\\[data-design='${id}'\\]\\s*\\{([^}]*)\\}`))
  if (!head) {
    fail(`${id}: :root[data-design='${id}'] 토큰 블록이 없다`)
    continue
  }
  const tok = Object.fromEntries([...head[1].matchAll(/(--color-[\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]))
  for (const k of required) if (!tok[k]) fail(`${id}: ${k} 가 없다`)
  for (const [fgs, bgs, min] of PAIRS) {
    for (const f of fgs) {
      for (const b of bgs) {
        const [fv, bv] = [rgb(tok[`--color-${f}`] ?? ''), rgb(tok[`--color-${b}`] ?? '')]
        if (!fv || !bv) {
          fail(`${id}: ${f}/${b} 는 #rrggbb 값이어야 잴 수 있다`)
          continue
        }
        const r = ratio(fv, bv)
        if (r < min) fail(`${id}: ${f} 위 ${b} 대비 ${r.toFixed(2)} < ${min}`)
      }
    }
  }
}

if (bad) {
  console.log(`문제 ${bad}건`)
  process.exit(1)
}
console.log(`디자인 ${IDS.length}종 토큰 ${required.length}개와 대비 통과`)
