// 앱 안 끌어 놓기: 로컬 탐색기 목록의 행을 서버파일 표로 끌어 놓으면 업로드(원본 MAIN-113).
// 탐색기(LocalPanel)가 끌기를 시작하고, 서버파일 화면(FilesView)이 놓는 자리에서 아래 함수로 받는다:
//
//   <div ondragover={(e) => acceptLocalDrag(e)} ondrop={(e) => { const p = localDragPaths(e); if (p) void uploadLocalPaths(p) }}>
//
// 운영체제 탐색기에서 창으로 끌어 오는 파일(Wails 파일 놓기)과는 다른 길이다 ― 그쪽은 전송·서버파일 담당이 맡는다.

import { TransferService, errorText } from '../../lib/api'
import { t } from '../../lib/i18n'
import { toast } from '../../lib/stores/ui'

/** dataTransfer 형식 이름(앱 안에서만 쓴다). 값은 경로 JSON 배열. */
export const LOCAL_DRAG_TYPE = 'application/x-snb-local-paths'

/** 끌기 시작: 경로를 싣고 끌기 그림 글자(「이름 외 N개」)를 붙인다. */
export function startLocalDrag(e: DragEvent, paths: string[], names: string[]) {
  if (!e.dataTransfer || !paths.length) return
  e.dataTransfer.effectAllowed = 'copy'
  e.dataTransfer.setData(LOCAL_DRAG_TYPE, JSON.stringify(paths))
  e.dataTransfer.setData('text/plain', paths.join('\n'))
  const ghost = document.createElement('div')
  ghost.textContent = names.length > 1 ? t('local.dragLabel', { name: names[0], n: names.length - 1 }) : names[0]
  ghost.className = 'snb-drag-ghost'
  document.body.appendChild(ghost)
  e.dataTransfer.setDragImage(ghost, -8, -8)
  setTimeout(() => ghost.remove(), 0)
}

/** 탐색기에서 끈 것인가(dragover·dragenter 에서 — 내용은 drop 때만 읽힌다). */
export function hasLocalDrag(e: DragEvent): boolean {
  return !!e.dataTransfer?.types.includes(LOCAL_DRAG_TYPE)
}

/** dragover 에서 부르면 탐색기에서 끈 것일 때만 놓을 수 있게 한다. 받았으면 참. */
export function acceptLocalDrag(e: DragEvent): boolean {
  if (!hasLocalDrag(e)) return false
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
  return true
}

/** drop 에서 경로를 꺼낸다. 탐색기에서 끈 것이 아니면 null. */
export function localDragPaths(e: DragEvent): string[] | null {
  const raw = e.dataTransfer?.getData(LOCAL_DRAG_TYPE)
  if (!raw) return null
  e.preventDefault()
  try {
    const v = JSON.parse(raw)
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : null
  } catch {
    return null
  }
}

/** 고른 로컬 파일·폴더를 올린다(TransferService.UploadPaths ― 검사·결재·진행은 전송 담당, 대기열에 들어간 안내는 transfer/Host). 실패는 알림 띠. */
export async function uploadLocalPaths(paths: string[]): Promise<boolean> {
  if (!paths.length) return false
  try {
    await TransferService.UploadPaths(paths)
    return true
  } catch (e) {
    toast(errorText(e), 3)
    return false
  }
}
