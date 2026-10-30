// 표에 쓰는 크기 글자(1024 단위). 원본의 화면별 규칙(탐색기·권한 팝업)은 서로 달라 그 화면 담당이 따로 둔다(docs/port/12 7.3).
export function formatBytes(b: number): string {
  if (b < 1024) return `${b} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = b / 1024
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v.toFixed(1)} ${units[u]}`
}
