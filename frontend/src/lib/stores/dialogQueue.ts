// 확인 창 줄의 순수 계산(node 로 시험한다 ― scripts/check-dialogs.mjs).

/** 줄에 같은 제목의 덮어쓸 창이 있으면 글만 바꾼 새 줄을, 없으면 null 을 돌려준다. */
export function mergeAlert<T extends { title: string; text: string; merge?: boolean }>(
  q: T[],
  title: string,
  text: string,
): T[] | null {
  if (!q.some((d) => d.merge && d.title === title)) return null
  return q.map((d) => (d.merge && d.title === title ? { ...d, text } : d))
}
