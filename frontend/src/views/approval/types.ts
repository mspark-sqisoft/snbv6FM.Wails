// 결재 화면이 받는 Go 사건 값(internal/svc/approval.go 의 SelectAsk·approval.Dept). 사건 값은 바인딩이 만들지 않아 여기 둔다.

import type { ApproverUser } from '../../lib/api'

export interface ApproverDept {
  id: string
  name: string
  users: ApproverUser[] | null
  children: ApproverDept[] | null
}

export interface SelectAsk {
  gen: number
  tree: ApproverDept[] | null
  myId: string
  selfApprove: boolean
  reqDept: string
  reqName: string
  aprvType: number
  typeOptions: string[] | null
  fileDesc: string
  direction: string
  totalSize: number
  clip: boolean
  files: { name: string; size: number }[] | null
}
