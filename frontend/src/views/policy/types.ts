// 정책 영역 이벤트 값(Go internal/svc/policy.go 의 FileCheck*·ZipPasswordAsk, internal/policy/upload.go 의 Row).
// 이벤트로만 오가는 값이라 바인딩 생성기가 만들지 않는다 ― Go 구조체를 고치면 여기도 고친다.

/** 검사 결과 한 행. top 이면 위 표(파일·폴더), 아니면 아래 표(압축·폴더 안 파일). 같은 key 는 바꾼다. */
export interface CheckRow {
  key: string
  top: boolean
  order: number
  parent: string
  name: string
  result: string
  detail: string
  ok: boolean
}

export type CheckMode = 'UPLOAD' | 'DOWNLOAD'

export interface CheckOpen {
  gen: number
  mode: CheckMode
  total: number
}

export interface CheckRows {
  gen: number
  rows: CheckRow[] | null
  done: number
  total: number
  current: string
}

/** value: "AUTO"(자동 업로드) · "SKIP:n"(폴더에서 n건 빼고 올린다) · "DOWNLOAD" · "" */
export interface CheckResult {
  gen: number
  fails: number
  value: string
  skip: number
}

/** name 이 비면 창을 닫는다(답하지 않는다 ― Go 가 기다림을 정리했다). */
export interface ZipAsk {
  gen: number
  name: string
  retry: boolean
}
