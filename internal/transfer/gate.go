package transfer

// Owner 는 업로드 입구(검사·결재자 창까지)의 지금 주인이다(원본 src/UploadGate.h, 057).
type Owner int

const (
	OwnerNone Owner = iota
	OwnerAuto
	OwnerHuman
)

// Admit 은 새 업로드를 입구가 어떻게 받는가다.
type Admit int

const (
	Take        Admit = iota // 받는다
	SkipAuto                 // 자동 업로드는 이번 주기를 건너뛴다(파일은 폴더에 남는다)
	RejectHuman              // 사람이 답을 기다리는 중 ― 새 사람 업로드는 안내하고 받지 않는다
	PreemptAuto              // 자동 업로드 검사를 멈추고 사람의 업로드를 받는다
)

// AdmitUpload 는 원본 admitUpload 표 그대로다.
func AdmitUpload(owner Owner, auto bool) Admit {
	switch {
	case owner == OwnerNone:
		return Take
	case auto:
		return SkipAuto
	case owner == OwnerAuto:
		return PreemptAuto
	}
	return RejectHuman
}
