package svc

// 화면에 내는 사건 이름. 화면은 Events.On(이름, e => e.data) 로 받는다.
// 영역이 새 사건이 필요하면 자기 서비스 파일에 상수를 두고 이 파일은 고치지 않는다(docs/ARCHITECTURE.md).
// 이름은 "영역:무엇" 꼴이다. 원본 qmlSignal 이름은 괄호에 적었다.
const (
	// 전송
	EvTransferProgress = "transfer:progress" // 진행률 []transfer.Tick(200ms 묶음)
	EvTransferState    = "transfer:state"    // 전송 목록 변경(대기→진행→완료/실패/취소)
	EvDownloadConflict = "transfer:conflict" // 받을 폴더에 같은 이름 ― 파일 중복 확인 창(DOWNLOAD_DUPLICATE)

	// 목록 갱신(서버 알림 9001·9002·9004 kind 0·1 의 1초 뒤 등)
	EvServerFilesChanged = "files:changed"    // 서버파일 목록을 다시 받아라
	EvApprovalChanged    = "approval:changed" // 결재 목록을 다시 받아라

	// 정책·검사
	EvPolicyChanged     = "policy:changed"      // 정책(프로파일·확장자·URL 필터)을 새로 받았다
	EvFileCheckOpen     = "policy:check-open"   // 파일 검사 결과 창 열기 {gen}(FILE_CHECK_DLG_POP)
	EvFileCheckRow      = "policy:check-row"    // 검사 결과 행 하나
	EvFileCheckResult   = "policy:check-result" // 검사 끝 {gen, fails, value}(FILE_CHECK_RESULT)
	EvFileCheckClose    = "policy:check-close"  // 검사 창 닫기(FILE_CHECK_DLG_CLOSE)
	EvZipPasswordAsk    = "policy:zip-password" // 압축 비밀번호 입력 창 {gen, name, retry}(ZIP_PASSWORD_ASK)
	EvApproverSelectAsk = "approval:select"     // 결재자 선택 창 요청(APROVER_POP)

	// 안내
	EvNotice = "ui:notice" // 알림 띠 Notice(NOTIFY)
	EvDialog = "ui:dialog" // 확인 창 요청 Dialog(MESSAGE_BOX 등)

	// 세션
	EvSessionEnded       = "session:ended"        // 로그아웃됨 SessionEnded ― 화면은 로그인 화면으로, 열린 창 닫기
	EvDisconnected       = "session:disconnected" // 로그인 상태에서 알림 스트림이 끊김(GRPC_DISCONNECTED)
	EvAutoLogout         = "session:timeout"      // 자동 로그아웃 AutoLogout(SESSION_TIMEOUT)
	EvPasswordChangeAsk  = "session:password"     // 비밀번호 변경 창 요청(기본 비밀번호·만료, PASSWORD_CHANGE_POP)
	EvSecondInstanceArgs = "app:second-instance"  // 두 번째 실행의 인자(--mode upload|download <경로>)

	// 그 밖
	EvSettingsChanged = "settings:changed" // 설정 저장됨 config.Settings
	EvClipCount       = "clip:count"       // 받을 클립보드 건수 {count}
)

// 세션 종료 사유(SessionEnded.Reason).
const (
	ReasonLogout       = "logout"       // 사용자가 로그아웃(또는 다른 망으로 다시 로그인)
	ReasonIdle         = "idle"         // 자동 로그아웃
	ReasonDisconnected = "disconnected" // 알림 스트림 끊김
	ReasonDuplicate    = "duplicate"    // 다른 PC 로그인(9004 kind 6·7)
	ReasonIntegrity    = "integrity"    // 무결성 검사 실패
	ReasonShutdown     = "shutdown"     // 앱 종료
)

// 알림 띠 심각도(원본 Notify.qml).
const (
	SevInfo    = 0
	SevSuccess = 1
	SevWarning = 2
	SevError   = 3
)

// Notice 는 EvNotice 의 값이다. 사용자가 답할 것이 없으면 알림 띠, 있거나 놓치면 안 되는 실패면 확인 창.
type Notice struct {
	Severity int    `json:"severity"`
	Text     string `json:"text"`
}

// Dialog 는 EvDialog 의 값이다. OnClose 는 닫힌 뒤 할 일의 이름("" · "exit" · "logoutProcess")이다(docs/port/09 5.4).
type Dialog struct {
	Title   string `json:"title"` // 비면 "정보"
	Text    string `json:"text"`  // 평문(서식 해석 안 함)
	OnClose string `json:"onClose"`
}

// SessionEnded 는 EvSessionEnded 의 값이다.
type SessionEnded struct {
	Reason string `json:"reason"`
}

// AutoLogout 은 EvAutoLogout 의 값이다. 문구: 「%1분 동안 사용하지 않아 자동으로 로그아웃했습니다.」
type AutoLogout struct {
	Minutes int `json:"minutes"`
}
