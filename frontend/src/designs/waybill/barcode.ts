// 화면 id 를 Code 39 바코드로 그린다. 판독기로 읽히는 진짜 부호다.
// 표는 A~Z 와 시작·끝 문자 *. 글자마다 막대 5개와 틈 4개가 번갈아 나오고 1 이 굵은 칸이다.
const AZ =
  '100001001 001001001 101001000 000011001 100011000 001011000 000001101 100001100 001001100 000011100 ' +
  '100000011 001000011 101000010 000010011 100010010 001010010 000000111 100000110 001000110 000010110 ' +
  '110000001 011000001 111000000 010010001 110010000 011010000'
const TABLE = AZ.split(' ')
const STAR = '010010100'

/** 화면 제목 옆 스티커가 읽는 CSS 변수를 채운다. design.css 의 .titles::after 가 쓴다. */
export function paintBarcode(id: string) {
  const code = id.toUpperCase().replace(/[^A-Z]/g, '')
  const stops: string[] = []
  let x = 0
  for (const pattern of [STAR, ...[...code].map((c) => TABLE[c.charCodeAt(0) - 65]), STAR]) {
    ;[...pattern].forEach((bit, i) => {
      const end = x + (bit === '1' ? 2 : 1)
      stops.push(`${i % 2 ? 'transparent' : 'var(--wb-ink)'} ${x}px ${end}px`)
      x = end
    })
    stops.push(`transparent ${x}px ${x + 1}px`) // 글자 사이 틈
    x += 1
  }
  const root = document.documentElement.style
  root.setProperty('--wb-barcode', `linear-gradient(90deg, ${stops.join(', ')})`)
  root.setProperty('--wb-barcode-w', `${x}px`)
  root.setProperty('--wb-barcode-text', `"*${code}*"`)
}
