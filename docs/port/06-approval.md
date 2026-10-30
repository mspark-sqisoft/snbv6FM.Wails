# 06 결재 ― Wails 판 이식 명세(결재요청·결재처리·대리결재자)

원본: snbApp(Qt Quick + C++, `E:\git\snBv6FM`). 현행 명세 `specs/current/06-approval.md`(이하 "현행 명세")의
요건 번호 `APR-xxx` 를 그대로 쓴다. 원본 줄 번호는 2026-10-30 작업트리 기준이다(현행 명세의 줄 번호는 63bbed4 기준이라 어긋날 수 있다).
서버 코드 근거는 `E:\git\snBv6Srv\go\service\approval\`·`approver\` 이다(읽기만 했다).

표기 규칙

- "추정:" 은 코드·실행으로 확인하지 않은 판단이다.
- "검색 0건" 은 무엇을 어디서 찾았는지 함께 적는다.
- 비밀번호·토큰 값은 적지 않는다.

---

## 1. 범위

이 문서가 다루는 것

| 묶음 | 원본 화면·코드 | 하는 일 |
|---|---|---|
| 결재방식 판정 | `loginrespolicy.cpp:114 getAprvType`, `ViewController.cpp checkUpFileMode` | 망(업무망/인터넷망)과 전송 종류(파일/클립보드)로 결재방식 0~7 을 고르고, 결재자를 언제 고를지(`fm_upfile_mode`) 정한다 |
| 결재요청 | `content/ApproverSelectDlg.qml`, `ViewControllerNotifySlot.cpp callApprover·makeAprvTree·setApproverAndCause·approverSelectReply`, `ViewController.cpp approvalRequest` | [결재자 선택] 창(결재자 트리·요청사유), 사전결재 업로드에 결재자 싣기, 사후결재 [결재요청]→결재요청[4127] |
| 결재처리 | `content/ApprovalScrollView.qml`, `content/ApprovalDlg.qml`, `ViewController.cpp processAprv·downloadFileForApproval·verifyDownloadZip` | 결재대상목록[3390], 승인·반려 사유, 결재[3092], 결재 대상 받기와 내용 보기 |
| 대리결재자 | `content/ProxyApproverScrollView.qml`, `ViewController.cpp saveProxyApprover` | 등록된 대리결재자 조회[3174]·등록[3272]·삭제[3276], 후보 목록[3370] |
| 결재 알림 | `FileMoverGrpc.cpp receivedNotify`, `approveResponse` | 9002(결재요청 도착)·9004 kind 0/1/5 받기, 결재 완료 9004 kind 5 보내기(NotifyOnce) |

다루지 않는 것(다른 문서 몫)

- 결재내역(GetAprovalHistory[3360]) → `10-history`.
- 업로드 전송 자체(스트림·대기열·검사 창) → `03-upload`. 이 문서는 업로드 요청에 싣는 결재 칸만 다룬다.
- 서버파일 목록·상태 문구(`getState`) → `04-download`/서버파일 문서. 이 문서는 `결재요청 가능` 판정만 인용한다.
- 클립보드 묶음 형식(`__CLIP__`) → `07-clipboard`.
- 알림 스트림 연결·재연결(NotifyServerStream) → `09-notify`. 이 문서는 결재 관련 알림 번호의 처리만 다룬다.
- 삼성카드 전용 결재요청[3127]·결재자 검색(`getAprvUserSearchList`) ― 빌드에서 빠져 있다(`CMakeLists.txt:670`). 제외.
- WASM 전용 동작(브라우저 폴더 선택, 4127 응답 뒤 서버파일 목록 직접 재조회) ― 제외.

Wails 판의 지금 상태(`E:\git\snBv6FM.Wails\client.go`)

- `aprvType()` 는 `Loc=="1"` 이면 `FmInflupAprv`, 아니면 `FmOtflupAprv` 만 본다. 클립보드 결재방식(`FmInclupAprv`·`FmOtclupAprv`)은 없다.
- `Upload` 는 결재방식 6 이면 결재자 = [자기 ID], `fm_upfile_mode != "1"` 이고 결재방식이 0 이 아니면 오류(결재자 선택 미지원)로 막는다.
  결재자가 있으면 `resv "1"`, 없으면 `"2"`, 결재방식 3 이면 `aprv_kind 3`.
- `pb/` 에 approval·approver·notify 패키지가 아직 없다. `gen.go` 의 `M` 옵션에 `ApprovalService.proto`·`ApproverService.proto`·`NotifyService.proto` 를 더해 만든다.
- 서버 암호 모드는 0(TLS 만)만 지원한다. 이 문서의 RPC 도 그 전제다.

---

## 2. 기능 목록

우선순위: **P1** = 원본과 동등해야 하는 필수, **P2** = 있으면 좋음, **제외** = 이유를 적는다.
난이도: 상/중/하. 현행 명세의 상태(구현·부분·상충)는 비고 첫머리에 적는다.

### 2.1 결재방식과 결재자 창으로 가는 길

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-001 | 결재방식을 망 위치와 전송 종류(파일·클립보드)로 정한다. 로그인 응답의 네 값(`fm_influp_aprv`·`fm_inclup_aprv`·`fm_otflup_aprv`·`fm_otclup_aprv`) | P1 | 하 | 구현. 지금 Wails 판은 파일만 본다 ― 클립보드 두 값을 더한다 |
| APR-003 | 결재방식·결재상태 번호를 서버 값 그대로 다루고 화면 문구로 바꾼다(3.6절 표) | P1 | 하 | 구현. 문구 표를 Go 와 TS 에 한 벌씩 두지 말고 Go 에서 문구까지 만들어 넘긴다 |
| APR-004 | `fm_upfile_mode` 0 = 검사 통과 직후 [결재자 선택](사전결재 모드), 1 = 결재자 없이 업로드 뒤 [결재요청](사후결재 모드). 키가 없으면 0 | P1 | 중 | 구현. 지금 Wails 판은 0 이면 업로드를 막는다 ― 이 막음을 걷어낸다 |
| APR-005 | 사전결재 모드에서 자동 업로드도 검사를 통과하면 [결재자 선택] 창을 띄우고 업로드 목록 주인을 사람으로 바꾼다 | P1 | 중 | 구현. 자동 업로드(`05`)가 들어올 때 함께. 자동 업로드가 없으면 이 행도 없다 |
| APR-006 | [결재요청] 단추는 `fm_upfile_mode` 가 1 이고 목록에 행이 있을 때만 보인다 | P1 | 하 | 구현 |
| APR-007 | 행을 고르지 않고 [결재요청] → 알림 띠 `결재요청할 파일을 선택해 주세요.` | P1 | 하 | 구현 |
| APR-008 | 고른 행 가운데 `결재요청 가능`(결재상태 4 이고 백신·개인정보 통과) 행만 상신. 없으면 `결재요청 가능한 목록이 없습니다.`, 일부만 가능하면 `결재요청이 가능한 파일만 상신합니다.` 창 뒤 진행 | P1 | 중 | 구현. 판정 규칙은 `filemoverpolicy.cpp:161 getState` 의 두 번째 값 4 |
| APR-010 | 파일 행과 클립보드 행(`__CLIP__`)을 섞어 고르면 `파일과 클립보드는 동시에 결재 요청 할 수 없습니다.` 창, 상신 안 함 | P1 | 하 | 구현 |
| APR-011 | 상신할 행으로 업로드 목록을 채운다(클립보드 여부·서버 파일명·이름·크기·개인정보 건수/상세·압축 비밀번호). 목록을 못 잡으면 `앞 업로드를 마친 뒤 다시 시도하세요.` | P1 | 중 | 구현. 업로드 목록 주인 규칙은 `03-upload` 와 공유 |
| APR-012 | 결재자 창을 띄우려고 결재자 목록[3370]과 부서 계층 경로(`setDeptInfo`)를 부른다 | P1 | 중 | 구현. Wails 판은 부서 계층[3250]·조직[3252]을 부르지 않는다(8절 1번) ― 트리는 [3370] 만으로 만든다(`makeAprvTree` 가 `dept` 인자를 쓰지 않는다, `ViewControllerNotifySlot.cpp:1221`) |
| APR-014 | 결재자 목록[3370]을 로그인 때 한 번만 묻는다(조회 중 표시가 내려가지 않음) | P1(고쳐서) | 하 | 구현이지만 원본 결함. Wails 판은 창을 열 때·[대리결재자] [조회] 때마다 새로 묻는다(8절 2번) |

### 2.2 [결재자 선택] 창

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-020 | 결재자 여부(`aprv_state`) 1 인 사용자만 트리에 넣는다. 부서 ID 를 `\|` 로 나눈 계층, 처음 나온 결재자의 부서가 뿌리 | P1 | 중 | 구현. 부모 부서를 못 찾으면 원본은 빈 포인터에 붙여 죽을 수 있다 ― Wails 판은 뿌리에 단다(8절 3번) |
| APR-021 | 대리결재자가 있는 결재자는 `이름(대결:대리결재자)` 로 보이고, 서버로 보내는 이름은 덧붙이지 않은 이름 | P1 | 하 | 구현 |
| APR-022 | 요청자 부서·이름, 결재방식(잠긴 콤보), 파일정보(1건 = 이름, 여럿 = `%1 외 %2건`, 클립보드 = `클립보드`), 전송방향, 크기 합, 파일 표(`#`·파일명·파일 크기) | P1 | 하 | 구현 |
| APR-023 | 창을 열 때 트리를 모두 펼친다. 체크박스로 고르고, 고른 결재자는 트리 순서대로 모인다 | P1 | 하 | 구현 |
| APR-024 | 결재자 0명 → `결재자를 선택해 주세요.`, 사유가 비면 → `결재요청 사유를 입력해 주세요.`. 사유 콤보 `직접입력`(기본)·`결재요청 드립니다.`·`업무상 필요`·`고객에게 전송` | P1 | 하 | 구현 |
| APR-026 | 같은 결재자를 두 번 세지 않는다 | P1 | 하 | 부분. 원본은 트리에 사용자당 한 행이지만 서버 목록의 중복은 거르지 않는다 ― Wails 판은 ID 로 중복을 거른다 |
| APR-030 | `self_approve` 가 `on` 이 아니면 자기 지정을 거절. 창이 닫힌 뒤 알림 띠 `자신을 결재자로 선택할 수 없습니다.`, 업로드 목록을 놓는다 | P1 | 하 | 구현. Wails 판은 창 안에서 막아 다시 고르게 한다(8절 6번) |
| APR-031 | 결재방식 0·6 이면 창 없이 진행. 6 이면 자기 ID 를 결재자로 싣는다 | P1 | 하 | 구현. Wails 판 `Upload` 에 이미 6 처리가 있다 |
| APR-033 | 트리에 넣을 결재자가 없으면 창을 띄우지 않고 목록을 놓는다. 안내 없음 | P1(고쳐서) | 하 | 상충(이전 명세는 "알린다"). Wails 판은 `결재자가 없습니다.` 를 띄운다 ― 원본도 이 상자를 만들기만 하고 안 띄운다(`ViewControllerNotifySlot.cpp:1254-1256`) |
| APR-034 | [취소]·`X`·Esc·로그아웃 정리 어느 길로 닫혀도 취소. 목록을 놓고 [전송 상태] 서랍을 열지 않는다 | P1 | 중 | 구현. Svelte 의 모달 닫힘 이벤트 하나로 모은다 |
| APR-035 | 창이 답을 기다리는 동안 사람의 새 업로드는 `앞 업로드를 마친 뒤 다시 시도하세요.`, 자동 업로드는 그 주기를 건너뛴다 | P1 | 중 | 구현. `03-upload` 의 업로드 목록 주인 규칙과 같은 Go 구조를 쓴다 |

### 2.3 결재요청 보내기

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-040 | 사전결재 모드: 창의 답(결재자 ID·사유)을 업로드 전송마다 싣고 대기열에 넣은 뒤 [전송 상태] 서랍을 연다. 폴더는 먼저 zip 으로 묶는다 | P1 | 중 | 구현. `FileTransInfo.approver_id`·`fcause`·`aprv_type`·`aprv_dept`·`resv "1"` |
| APR-041 | 사후결재 모드: 결재요청[4127]. 서버 파일명·결재자 ID·이름·결재방식·요청자 부서명·사유·묶음 번호·압축 비밀번호. 같은 비밀번호끼리 10개씩 묶는다 | P1 | 중 | 구현. 묶음 규칙은 `ZipPswdGroups.h` |
| APR-042 | 클립보드 행만 고르면 행마다 `클립보드` 로 보이고 클립보드 결재방식으로 보낸다 | P1 | 하 | 구현. 클립보드(`07`)가 들어올 때 함께 |
| APR-043 | 4127 성공이면 화면에 아무것도 안 띄운다. 실패는 `통신 오류 발생` / `서버와의 통신 중에 오류가 발생하였습니다.` | P1(고쳐서) | 하 | 구현. Wails 판은 성공 알림과 서버 사유를 보인다(8절 4번) |

### 2.4 [결재처리] 화면

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-050 | 결재자에게 [결재처리]·[결재내역]·[대리결재자] 메뉴, 대리결재자로 지정된 사람에게 [결재처리] 메뉴. 로그인 응답 `aprv_yn`·`replace_yn` | P1 | 하 | 구현. 인터넷망에서는 결재 메뉴가 안 나온다(서버도 결재를 거절, 3.2절) |
| APR-051 | 화면이 보일 때·[새로고침]·망 변경 때 결재대상목록[3390]. 같은 조회가 도는 동안 겹쳐 보내지 않는다 | P1 | 하 | 구현 |
| APR-052 | 표 열 `#`·선택·전송방향·요청자·파일명·파일크기·요청일시·요청사유·상태(색 알약). 기본 정렬 요청일시 내림차순. 빈 상태 `조회된 데이터가 없습니다.` | P1 | 중 | 구현이지만 원본은 정렬이 화면에 안 먹는다(표가 정렬 모델이 아니라 `ApprovalModel` 을 직접 씀, `ApprovalScrollView.qml:343`). Wails 판은 실제로 정렬한다 |
| APR-053 | 응답을 받으면 회전 표시를 끈다. 실패하면 화면을 떠날 때까지 돈다 | P1(고쳐서) | 하 | 구현. Wails 판은 실패 때도 끄고 사유를 보인다 |
| APR-054 | 행을 누르거나 우클릭하면 그 행만 체크. 여러 건은 체크박스·헤더 전체선택(꺼짐·일부·켜짐) | P1 | 하 | 구현 |
| APR-055 | [승인]·[반려]에서 체크된 행이 없으면 `결재할 항목을 선택해 주세요.`, 있으면 [승인 사유]/[반려 사유] 창 | P1 | 하 | 구현 |
| APR-056 | 사유 콤보(승인: `승인합니다`·`사전에 별도로 승인된 내역`·`단순 파일 승인`·`직접입력`, 반려: `반려합니다`·`목적 불분명`·`설치 진행 미흡`·`직접입력`). 직접입력일 때만 입력(최대 100자). 비면 `결재 사유를 입력해 주세요.` | P1 | 하 | 구현. 원본은 창을 다시 열 때 앞 값이 남는다 ― Wails 판은 열 때마다 초기화 |
| APR-057 | 체크된 건마다 결재[3092]. 20건마다 1.5초 쉰다 | P1 | 중 | 구현. Wails 판은 동시 상한을 둔 병렬로 보낸다(8절 5번) |
| APR-058 | 성공이면 결재대상목록 재조회. 실패는 로그만 | P1(고쳐서) | 하 | 구현. Wails 판은 실패 건수·사유를 보이고 실패해도 목록을 다시 받는다 |

### 2.5 결재 대상 받기와 내용 보기

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-060 | 우클릭 [다운로드]. 체크 없으면 `다운로드할 파일을 선택해 주세요.`, 클립보드는 앱 임시 폴더로 곧바로, 파일은 폴더를 고른 뒤 받는다 | P1 | 중 | 구현. `DownloadForApproval` 이 받기 대기열에 넣어 받고, 클립보드 건은 결재자 클립보드로 넣는다(07 CLIP-040) |
| APR-061 | 결재 받기는 받은 뒤 서버 파일을 지우지 않고(RemoveFile 안 부름) 최대 다운로드 횟수도 적용하지 않는다 | P1 | 하 | 구현. `DownloadForApproval` 은 RemoveFile 을 부르지 않는다 |
| APR-062 | 앞 결재 받기가 대기·진행 중이거나 내용 검사 중이면 `받는 중` / `앞서 요청한 파일을 받는 중입니다.\n받기가 끝난 뒤 다시 받아 주세요.` | P1 | 하 | 구현 |
| APR-063 | 결재 받기 전송이 끝날 때마다 내용 보기(zip 은 풀고 폴더는 훑어 내부 파일 목록 창). 내부 파일 0건이면 창 없음 | P2 | 상 | 구현. 압축 처리(`02`)에 기대므로 그 영역이 먼저다. macOS·Linux 같음 |

### 2.6 결재 알림

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-070 | 9002(결재요청 도착) → 1초 뒤 결재대상목록 재조회. 자가승인(`userid == msg`)은 무시 | P1 | 하 | 구현. 알림 스트림(`09`)이 먼저 있어야 한다 |
| APR-071 | 9002 `dat == "process_replace_approval"` 이면 목록만 갱신. 그 밖이면 트레이 풍선 `결재 요청` / `파일 이동에 대한 승인 요청 건이 있습니다.`(5초), 트레이가 없으면 알림 띠 | P1 | 중 | 구현. macOS·Linux 는 OS 알림 설정에 따라 풍선이 안 뜰 수 있다 |
| APR-072 | 9004 kind 1 → 1초 뒤 결재대상목록. kind 0 → 서버파일 목록 + 결재대상목록(원본은 `break` 가 없어 흘러내림) | P1 | 하 | 구현. Wails 판은 흘러내림 대신 두 조회를 명시적으로 부른다 |
| APR-073 | 9004 kind 5(결재 완료) → `결재 완료` / `결재 완료\n승인 요청하신 파일의 결재가 완료되었습니다.\n요청 결과 : %1`(%1 = `dat=="Y"` 이면 `승인`, 아니면 `반려`) | P1 | 하 | 구현. 서버가 지금 전달하지 않는다(7절 위험 1) |
| APR-074 | 로그아웃 뒤 도착한 결재 관련 알림은 버린다 | P1 | 하 | 구현 |
| APR-077 | 결재 성공 응답이면 요청자에게 9004 kind 5 를 NotifyOnce 로 보낸다(자가 결재는 안 보냄) | P1 | 하 | 부분. 앱은 보내지만 서버 NotifyOnce 가 요청자에게 넘기지 않는다(`svc_notifyonce.go:44`, 서버 수정 대기) |

### 2.7 [대리결재자] 화면

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-080 | 화면이 보일 때·[조회] 때 등록된 대리결재자[3174] 조회와 후보 표 채우기(결재자 목록에서 자신 제외, 열 `#`·ID·이름·부서·직급·직위) | P1 | 하 | 구현. 후보는 `aprv_state` 로 거르지 않는다(`ViewControllerUiSlot.cpp:519`) |
| APR-081 | 등록돼 있으면 이름·`YYYY-MM-DD ~ YYYY-MM-DD`·[삭제], 없으면 `등록된 대리결재자가 없습니다.` | P1 | 하 | 구현 |
| APR-082 | [등록] 막기 순서: 이미 있음 → `등록된 대리결재자가 있습니다.\n삭제하고 등록해 주세요.`, 후보 미선택 → `아래 목록에서 대리결재자를 선택해 주세요.`, 사유 없음 → `대리결재 사유를 입력해 주세요.`, 기간 초과 → `대체결재자 유효기간이 설정된 값을 초과하였습니다. [%1 일]` | P1 | 하 | 구현. 상한 `max_deputy_date`(없으면 5). Wails 판은 종료일 < 시작일도 막는다 |
| APR-083 | 등록[3272]: 결재자 ID·이름, 대리 ID·이름, 시작 `YYYYMMDD000000`, 끝 `YYYYMMDD235959`, 사유. 결과 문구 `대리결재자가 정상적으로 등록되었습니다.` / `대리결재자를 등록하지 못했습니다.` | P1 | 하 | 구현 |
| APR-084 | [삭제]: 없으면 `삭제할 대리결재자가 없습니다.`, 있으면 삭제[3276] 뒤 `대리결재자가 정상적으로 삭제되었습니다.` / `대리결재자를 삭제하지 못했습니다.` | P1 | 하 | 부분. 원본 QML 이 인자 순서를 바꿔 넘겨 사유 칸에 망 번호가 간다 ― Wails 판은 사유 `대리결재자 삭제` 를 바로 싣는다 |
| APR-085 | (서버) 대결 기간 안의 결재요청을 대리결재자 목록에도 내주고, 한쪽이 처리하면 둘 다에서 뺀다. 앱은 받은 대로 보인다 | P1 | 하 | 구현(서버 몫). 앱 쪽 일은 없다 ― 9002 `process_replace_approval` 처리만 |

### 2.8 미구현·보류(한 줄)

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| APR-002·009·013·025·027·028·029·032·059·075·076 | 확장자별 결재(002), 상신 직전 개인정보 상태 경합(009, 보류 050), 부서 정보 없을 때 창이 안 뜸(013, 보류 067·071), 결재자 최소·최대(025), 결재자 유효성 검증 4128(027), 순서 지정(028), 결재자 검색·조직도 깊이(029), 결재방식 4 사전/사후 고르기(032), 결재처리 화면 검출 상세(059), 결재 정책 변경 9008(075), 결재승인 지연 건수 3638(076) | 제외(원본에 없음) | ― | 009·013 은 Wails 판 구조(상신 직전 재조회, 3250 미사용)로 저절로 사라진다. 059·076 은 서버 변경 없이 되므로 P2 후보(8절) |

요건 수: 위 표의 개별 행 51개(구현 47, 부분 3, 상충 1) + 묶음 1줄(미구현 9, 보류 2).
현행 명세의 "요건 수" 표는 부분을 2로 적었지만 행을 세면 APR-026·077·084 의 3개다.

---

## 3. 서버 통신

모든 요청은 기존 `Session` 의 ctx(메타데이터 `auth-token`)를 쓴다. 원본은 단일 요청마다 15초 상한(`callRpc` 의 `RPC_TIMEOUT`)을 둔다 ―
Wails 판도 `context.WithTimeout(ctx, 15*time.Second)` 로 같은 상한을 둔다.

### 3.1 RPC 목록

| 전문 번호 | 서비스.RPC | 요청 메시지 | 응답 메시지 | 언제 |
|---|---|---|---|---|
| 3370/3371 | `ApproverService.GetApproverList` | `ApproverListRequest` | `ApproverHierarchyResponse` | 결재자 창 열기, [대리결재자] 후보 |
| 4127 | `ApprovalService.ApprovalAfterUpload` | `ApprovalAfterUploadRequest` | `AnswerResponse` | 사후결재 [결재요청] |
| 3390/3391 | `ApprovalService.GetAprovalList` | `AprovalListRequest` | `AprovalListResponse` | [결재처리] 목록 |
| 3092/3093 | `ApprovalService.Approve` | `ApprovalRequest` | `ApprovalResponse` | 승인·반려 한 건 |
| 3174/3175 | `ApproverService.GetProxyApprover` | `ProxyApproverRequest` | `ProxyApproverResponse` | [대리결재자] 조회 |
| 3272 | `ApproverService.RegisterProxyApprover` | `ProxyApproverRegRequest` | `AnswerResponse` | [등록] |
| 3276 | `ApproverService.DeleteProxyApprover` | `ProxyApproverRegRequest` | `AnswerResponse` | [삭제] |
| (알림) | `NotifyService.NotifyOnce` | `NotifyMessage` | `Empty` | 결재 성공 뒤 요청자에게 9004 kind 5 |
| 3311 | `ServerFileService.GetServerFileList` | (기존) | (기존) | 사후결재 상신 판정 ― Wails 판에 이미 있음 |
| 3638/3639 | `ApprovalService.GetAprvCount` | `AprvCountRequest{approver, elapsed_days}` | `{count}` | P2(원본은 부르지 않음) |
| 3250/3251·3252/3253 | `GetDeptHierarchy`·`GetOrgInfo` | | | **Wails 판은 부르지 않는다**(8절 1번) |
| 3127·3360·3262 | 삼성카드 결재요청·결재내역·부서 이력 | | | 제외·`10-history`·원본 미사용 |

### 3.2 RPC 별 필드와 순서

**결재자 목록[3370]** (`FileMoverGrpc.cpp:2264 getApproverList`)

- 요청: `req_user_id` = 내 ID, `type` = **1**, `user_name` = `""`. `user_id`·`user_dept_name` 은 비운다.
- 서버의 `type` 뜻(`approver/svc_getapproverlist.go:26-28`): 0 = 조회 사용자 부서 관련, 1 = 전사 사용자. 그 밖은
  `invalid type: %d (0: 조회사용자부서관련, 1: 전사용자)` 오류다(`:139`).
  원본은 `(approverSearchType_==1) ? 2 : 1` 을 보내는데 `approverSearchType_` 를 초기화하지 않는다. 우연히 1 이 되면 2 를 보내 서버가 거절한다.
  Wails 판은 늘 1 을 보낸다.
- 응답: `count`, `list[]` (`user_id, user_name, user_dept_id, user_dept_name, user_rank_code, user_rank, user_duty_code, user_duty_name, user_mail, aprv_state, rname`).
  원본은 `user_id` 가 빈 행을 만나면 반복을 멈춘다(`:2308` 의 `break`). Wails 판도 빈 ID 행에서 멈춘다(추정: 서버의 고정 크기 배열 흔적이라 뒤는 빈 행뿐이다).
- 쓰는 곳 둘
  - 결재자 트리: `aprv_state == 1` 만, 부서 `user_dept_id` 의 `|` 계층(5.1절).
  - 대리결재자 후보: 전체에서 내 ID 만 뺀다(`aprv_state` 로 거르지 않는다).

**결재요청[4127]** (`FileMoverGrpc.cpp:2172 reqAprvAfter`)

- 순서
  1. `pkg_no` = 지금 epoch 초(int32). 모든 묶음이 같은 값을 쓴다.
  2. `(서버 파일명, 압축 비밀번호)` 쌍을 비밀번호로 묶고, 같은 비밀번호 안에서 10개씩 자른다(`ZipPswdGroups.h`).
  3. 묶음마다 요청 하나: `fname_svr[]` = 묶음, `zip_pswd` = 그 비밀번호, `approver_id[]` = 결재자 ID, `approver[]` = 결재자 이름,
     `aprv_type` = 결재방식(파일/클립보드에 따라), `aprv_dept` = 내 부서**명**(로그인 응답 `user_dept_name`), `fcause` = 사유, `pkg_no`.
- 결재자 이름은 트리에서 고른 실제 이름(`RealNameRole`, `(대결:…)` 없음). 개수가 결재자 ID 와 다르면 ID 를 이름으로 쓴다(`setApproverAndCause`).
- 압축 비밀번호는 서버파일 목록 `zip_pswd` 를 **그대로 되돌려 보낸다** ― 서버가 이 값으로 `fm_approve.zip_pswd` 를 덮어써서, 비우면 업로드 때 남긴 비밀번호가 지워진다(`imports/FileMoverGrpc/README.md`).
- 서버 응답
  - 성공: `Res 1`, `Reason "OK~!!!"`(`svc_approvalafterupload.go:247-249`).
  - 결재방식 0: `Res 1`, `Code 1`, `Reason "SKIP"`, `Txt "미결재로 처리되었습니다."`(`:31-38`).
  - 실패: `res` 가 0 인 응답이 아니라 **gRPC 오류**로 온다. 예: `failed to insert/update approval: …`, `affected rows is zero for file %s`, `session not found in context`.
- 원본 처리(`answerResponse` 4127): `res > 0` 이면 `E_NOTIFY_SUCCESS "결재요청이 완료되었습니다."`(처리부가 주석이라 화면에 안 뜸), 그 밖이면 `E_NOTIFY_ERROR(reason)`.
  gRPC 오류면 `setStatus("파일 업로드 후 결재 요청[4127] 실패!!!")` 로그만 남는다.

**결재대상목록[3390]** (`FileMoverGrpc.cpp:1913 getAprvFileList`)

- 요청: `userid` = 내 ID(비면 서버가 `userid is required`).
- 응답: `m_count`, `list[] AprvFileInfo`, `m_pkg_no`. 쓰는 칸
  - 표: `file_course`(0 = 다운로드 out→in, 1 = 업로드 in→out → 전송방향 문구), `file_name_pc`(파일 이름만 보임), `file_size`, `file_user`(요청자 ID), `user_name`(요청자 이름), `file_date`(요청일시), `file_cause`(요청사유), `aprv_state`(상태 문구).
  - 결재·받기에 쓰는 칸: `file_name_svr`, `seqno`, `aprv_rank`, `dir_flag`(문자열 `"0"`/`"1"`), `zip_pswd`.
  - 받지만 원본이 화면에 안 쓰는 칸: `priv_state`·`priv_cnt`·`priv_detail`·`drm_flag`·`user_dept_name`·`aprv_*` ― P2 의 검출 상세(APR-059) 재료.
- 원본은 `approvalListRequesting_` 로 겹쳐 보내지 않고 콜백에서 내린다. 실패면 `결재대상목록 조회[3390] 실패!!!` 로그만.

**결재[3092]** (`FileMoverGrpc.cpp:2110 processAprv`)

- 요청: `seqno`, `file_name_svr`, `file_user` = 요청자 ID, `aprv_cause` = 사유, `aprv_id` = 내 ID, `aprv_user` = 내 이름, `aprv_state` = 1 승인·2 반려, `aprv_rank`.
  원본은 `file_name_pc` 를 채우지 않는다. Wails 판도 비운다(서버가 쓰는지 확인 안 함 ― 추정: 쓰지 않는다. 원본이 비운 채 실 서버에서 통과한다).
- 응답 성공: `Res 1`, `Reason "APPROVE OK !!!"`(`approval/svc_approval.go:311-314`).
- 서버 오류(gRPC 오류)
  - `outer server: approval operation not supported (proxy only)` ― 인터넷망 서버는 결재를 받지 않는다(`:30`).
  - `approval record not found for seqno=%d`(`:68`) ― 다른 결재자·대리결재자가 먼저 처리했을 때 추정.
  - `no rows updated for seqno=%d`(`:123`).
- 성공 뒤 순서(원본 `approveResponse :2138`)
  1. 결재대상목록[3390] 재조회.
  2. 요청자 ID 가 비지 않고 내 ID 와 다르면 `NotifyOnce(NotifyMessage{notify_svcno: 9004, userid: 요청자, kind: 5, side: 0, dat: 승인 "Y" / 반려 "N"})`. 응답은 버린다.
- 원본 실패 처리: 로그 `결재[3093] 응답 >>> 오류 메시지:` 만.

**대리결재자 조회[3174]** (`:2376`)

- 요청 `userid` = 내 ID. 응답 `alt_userid, alt_username, expire_from, expire_to`(`YYYYMMDD`).
  화면은 `YYYY-MM-DD` 로 바꿔 보인다. `alt_userid` 가 비면 "등록 없음".

**대리결재자 등록[3272]** (`:2411`)

- 요청: `userid`·`username` = 나, `alt_userid`·`alt_username` = 고른 후보, `expire_from` = 시작일 `YYYYMMDD` + `000000`, `expire_to` = 끝일 + `235959`, `cause`.
- 서버 성공: `Res 1`, `Reason "Replace Aprv User Changed !!!"`(`svc_registerProxyApprover.go:109-111`). 사유는 이력 행에 남는다(`:61`·`:88`).
- 원본: `res == 1` → `대리결재자가 정상적으로 등록되었습니다.`, 아니면 `대리결재자를 등록하지 못했습니다.`(`answerResponse` 3272).

**대리결재자 삭제[3276]** (`:2436`)

- 요청: `userid`, `username`, `alt_userid`, `alt_username`, `cause`. `expire_*` 는 비운다.
- 서버 성공: `Res 1`, `Reason "Replace Aprv User Delete OK !!!"`(`svc_deleteproxyapprover.go:81-83`). `cause` 를 이력 행에 남긴다(`:61`, 이력 action 1 = 삭제).
- 원본: `res == 1` → `대리결재자가 정상적으로 삭제되었습니다.`, 아니면 `대리결재자를 삭제하지 못했습니다.`.

### 3.3 응답 판정 ― "실패 문구" 가 거의 안 나오는 이유

서버 approval·approver 서비스는 실패를 `res = 0` 응답이 아니라 Go `error`(gRPC 오류, 코드는 대개 `Unknown`)로 돌려준다
(`grep -n "fmt.Errorf" approval/*.go approver/*.go` 로 확인, 3250 만 `status.Errorf(codes.InvalidArgument/Internal, …)`).
원본의 `res != 1` 분기(`대리결재자를 등록하지 못했습니다.` 등)는 그래서 실제로는 거의 안 탄다. gRPC 오류는 `callRpc` 실패 분기의 `setStatus` 로그로만 남는다.

Wails 판 규칙(하나의 함수로 모은다)

- `err != nil` → 실패. 화면 문구는 원본 실패 문구(있으면) + 서버 오류의 `status.Convert(err).Message()` 를 줄을 바꿔 덧붙인다.
- `err == nil && res != 1` → 실패. 원본 실패 문구 + `reason`.
- `err == nil && res == 1` → 성공.

### 3.4 알림 번호(받기)

알림은 `NotifyServerStream` 으로 온다(연결은 `09`). 결재 영역이 처리하는 것(`FileMoverGrpc.cpp:2579 receivedNotify`)

| svcno | kind | 조건 | 원본 동작 | Wails 판 |
|---|---|---|---|---|
| 9001 | ― | `userid` 가 비지 않고 내 ID 와 다르면 무시 | 1초 뒤 서버파일 목록 재조회 | `04` 몫. 결재 성공 뒤 서버가 요청자에게 보낸다 ― 요청자 화면의 상태가 여기로 바뀐다 |
| 9002 | ― | `userid` 가 비지 않고 `userid == msg` 면 무시(자가승인) | 1초 뒤 결재대상목록. `dat == "process_replace_approval"` 이면 끝, 그 밖이면 `E_NOTIFY_APRV_FILE_EXIST` | 같다 |
| 9004 | 0 | `userid` 가 내 ID 와 다르면 무시 | 서버파일 목록 재조회 → (흘러내려) 1초 뒤 결재대상목록 | 두 조회를 명시적으로 |
| 9004 | 1 | 같음 | 1초 뒤 결재대상목록 | 같다 |
| 9004 | 5 | 같음 | `E_NOTIFY_COMPLETE_APRV(dat)` | 같다 |
| 9004 | 6·7 | 같음 | 중복 로그인 처리 | `01` 몫 |

"1초 뒤" 는 서버가 DB 를 커밋하기 전에 알림이 올 수 있어 둔 지연이다(추정: 원본 주석 없음). Wails 판도 1초를 두고, 그사이 같은 조회가 또 오면 하나로 합친다.

### 3.5 알림 보내기

- `NotifyOnce(NotifyMessage)`. 칸: `notify_svcno 9004`, `kind 5`(`MSG_KIND_END_APRV`), `side 0`(`MSG_SIDE_SAME`), `userid` = 요청자 ID, `dat` = `"Y"`/`"N"`.
  나머지 칸(`alim, msg, pid, server_no, ipaddr`)은 원본이 채우지 않는다(`sendNotify`).
- 서버는 지금 이 알림을 요청자에게 넘기지 않는다(`svc_notifyonce.go:44`, 서버 수정 대기). 그래도 보낸다 ― 원본(MOSFM 포함)과 같고, 서버가 고쳐지면 앱을 고치지 않아도 된다.

### 3.6 값 표

결재방식(`ViewController.cpp:479 getAprvTypeText`, 결재자 창 콤보와 같은 순서)

| 값 | 문구 | 창 | 업로드 칸 |
|---|---|---|---|
| 0 | `자동 결재` | 안 띄움 | 결재자 없음, `resv "2"` |
| 1 | `사전 결재` | 사전결재 모드면 띄움 | 결재자 있음, `resv "1"` |
| 2 | `확장자결재` | 띄움 | (원본은 확장자 판정 미구현, APR-002) |
| 3 | `사후 결재` | 띄움 | `aprv_kind 3`, 결재자 있어도 `resv` 는 isAfter 판정 |
| 4 | `요청시 사전사후결정` | 띄움(고르기는 미구현) | |
| 5 | `요청시 결재생략결정` | 띄움 | |
| 6 | `자동자가결재` | 안 띄움 | 결재자 = [내 ID] |
| 7 | `자동,사전,사후` | 띄움 | |
| 그 밖 | `Unknown(n)` | | |

결재상태(`getAprvStateText :445`): 0 `결재 요청`, 1 `결재 승인`, 2 `결재 반려`, 3 `사후 승인`, 4 `결재요청 가능`, 그 밖 `Unknown`.
4 는 서버 스키마 주석에 없지만 실제 DB 에 가장 많은 값이다 ― 결재자 없이 올라온 사후결재 상신 대기(`src/README.md`).

결재요청 가능 판정(`filemoverpolicy.cpp:161 getState`, 두 번째 값이 4 일 때만 상신)

1. `virus_flag`: 1·4 통과. -1 `백신체크 실패`, 0 `바이러스 체크 중`, 2 `바이러스 발견`, 3 `APT 탐지중`, 5 `다운로드 불가`, 21 `CDR 있음`, 22 `암호설정문서파일 차단`, 6 `OLE 검출 파일 차단`, 그 밖 `다운로드 불가`.
2. `priv_state`: 1·3 통과. -1 `개인정보 검색 실패`, 0 `개인정보 검색 중`, 2 `개인정보 검색됨`, 그 밖 `다운로드 불가`.
3. `aprv_state`: 0 `결재 대기중`, 2 `반려됨`, 4 `결재요청 가능`, 1·3 다음 단계.

### 3.7 업로드 요청에 싣는 결재 칸(사전결재 모드)

`FileTransfer.cpp:1118-1173` 기준. 업로드 RPC 는 `03` 에서 다루고, 결재 칸만 적는다.

| 칸 | 값 |
|---|---|
| `approver_id[]` | 창에서 고른 결재자 ID(결재방식 6 이면 [내 ID], 0 이면 비움) |
| `aprv_type` | 결재방식(파일/클립보드별) |
| `aprv_dept` | 내 부서명 |
| `fcause` | 요청사유 |
| `aprv_kind` | 결재방식 3 이면 3 |
| `resv` | 파일: 결재자가 있고 사후가 아니면 `"1"`, 그 밖 `"2"`. 클립보드: 사후면 `"2"`, 아니면 `"1"` |
| `zip_pswd` | 사람이 맞힌 암호 압축 비밀번호(128 글자 이하일 때만) |
| `pkg_no` | 파일일 때 |

---

## 4. 정책 키

### 4.1 권한 프로파일(`fm_profile2`, 정책 조회[3007])에서 읽는 키

| 키 | 뜻 | 없을 때 | 근거 |
|---|---|---|---|
| `fm_upfile_mode` | 0 = 사전결재 모드(업로드 전에 결재자 고름), 1 = 사후결재 모드([결재요청]) | 0 | `FileMoverGrpc.cpp:1194 profile2Response` |
| `self_approve` | `on` 이면 자기를 결재자로 고를 수 있다 | 허용 안 함 | `ViewControllerNotifySlot.cpp:1520 approverSelectReply` |
| `max_deputy_date` | 대리 기간 상한(일). 판정 `시작~끝 일수 + 1 > 상한` 이면 막음 | 5 | `ViewController.cpp:2742 saveProxyApprover`, `filemoverutil.cpp:262` |

정책 값이 없는 것과 0 을 가르는 규칙: 원본은 `isValid()` 로 먼저 가른다. Wails 판 `Session.Profile`(map)도 `v, ok := m[key]` 로 가른다.

### 4.2 로그인 응답(`LoginResponse`)에서 읽는 값

| 칸(필드 번호) | 뜻 |
|---|---|
| `fm_influp_aprv`(6) | 업무망 파일 업로드 결재방식 |
| `fm_inclup_aprv`(7) | 업무망 클립보드 업로드 결재방식 |
| `fm_otflup_aprv`(8) | 인터넷망 파일 업로드 결재방식 |
| `fm_otclup_aprv`(9) | 인터넷망 클립보드 업로드 결재방식 |
| `aprv_yn`(21) | 결재자 여부 → [결재처리]·[결재내역]·[대리결재자] 메뉴 |
| `replace_yn`(45) | 대리결재자 여부 → [결재처리] 메뉴 |
| `user_name`(37) | 결재[3092] `aprv_user`, 대리결재자 `username`, 결재자 창 요청자 이름 |
| `user_dept_name`(39) | 결재요청 `aprv_dept`, 결재자 창 요청자 부서 |
| `user_dept`(38) | 원본은 [3250] 요청에만 쓴다. Wails 판은 쓰지 않는다 |

망 위치는 `fmenv.ini` 의 서버 위치(`Session.Server.Loc`: `"1"` 업무망, `"0"` 인터넷망)다.

### 4.3 원본이 읽지 않는 키

`fm_aprv_min_*`·`fm_aprv_max_*`·`MULTI_APRV`·`or_approve`·`fm_use_aprv_search`·`fm_use_aprv_valid`·`fm_aprv_depth`
― `src`·`imports` 에서 검색 0건(현행 명세 "정책 키" 절). Wails 판도 읽지 않는다(2.8절).

---

## 5. 화면 동작

### 5.1 [결재자 선택] 창(`content/ApproverSelectDlg.qml`)

크기·닫기

- 모달. 크기 `min(520, 부모-40) x min(620, 부모-40)`. 제목 `결재자 선택`, 제목줄 `X`.
- 닫기 규칙: Esc 로 닫힌다(`closePolicy: CloseOnEscape`). `onClosed` 에서 답하지 않았으면 `reply(true)`(취소). 바깥 누름으로는 안 닫힌다.
- 열 때(`show()`): `replied = false`, 트리를 전부 펼친다(`expandRecursively`).

영역(위에서 아래로)

1. **요청자 정보**: 부서 + 이름.
2. **결재자 정보**: 트리. 부서 행은 부서 이름, 결재자 행은 체크박스 + `이름` 또는 `이름(대결:대리결재자)`.
   접근성 이름 `<화면 이름> (<ID>)`(동명이인 구분). 글자는 서식 태그를 해석하지 않는다(`textFormat: Text.PlainText`).
3. **신청 정보**: 결재방법(잠긴 콤보, `currentIndex = aprvType`), 파일정보, 전송방향, 파일크기, 요청사유(콤보 + 입력란).
   요청사유 콤보가 0(`직접입력`)이 아니면 입력란을 잠그고 비운다. placeholder `직접입력인 경우만 입력 가능합니다.`
4. **파일 표**: ` # `·`파일명`·`파일 크기`.
5. 단추: [결재요청](objectName `approverRequestButton`) · [취소].

[결재요청] 판정 순서

1. 체크된 결재자 0명 → `결재자를 선택해 주세요.`
2. 사유 = 직접입력이면 입력란, 아니면 콤보 글자. `trim` 해서 비면 → `결재요청 사유를 입력해 주세요.`
3. `reply(false, cause)` → C++ `approverSelectReply`:
   - `self_approve != "on"` 이고 고른 결재자에 내 ID 가 있으면 창은 이미 닫힌 뒤 알림 띠(경고) `자신을 결재자로 선택할 수 없습니다.`, 목록을 놓는다.
   - 그 밖이면 `setApproverAndCause` → 사전결재(업로드 대기열) / 사후결재(4127).

트리 만들기(`ViewControllerNotifySlot.cpp:1221 makeAprvTree`)

1. 결재자 목록을 서버 순서대로 돈다. `aprv_state != 1` 은 건너뛴다.
2. 첫 결재자의 부서를 뿌리로 만들고 그 아래에 결재자를 단다.
3. 다음부터: 부서 ID 가 이미 있으면 그 아래에 단다. 없으면 새 부서 행을 만들고 부모 = `deptId` 의 마지막 `|` 앞부분으로 찾는다.
   부모가 없으면 원본은 빈 포인터에 붙인다(죽을 수 있다).
4. 0명이면 `false` → 원본은 로그 `결재자선택 >>> 결재자 트리가 비어 창을 띄우지 않습니다. 업로드 목록을 놓습니다.` 뒤 목록을 놓는다.

창 데이터 채우기(`callApprover :1355`)

- 모든 행이 클립보드면 클립보드 결재방식, 아니면 파일 결재방식.
- 파일정보: 클립보드면 행마다 `클립보드`, 합계 설명 `클립보드`. 파일 1건 = 파일명, 여럿 = `%1 외 %2건`(첫 파일명, 나머지 수).
- 결재방식 0·6 이면 창 없이 `setApproverAndCause` (6 이면 결재자 = [내 ID]).

Wails 판 구성(Svelte)

- 창은 `<dialog>` 하나. `close` 이벤트 하나에서 "답하지 않았으면 취소" 를 처리해 [취소]·`X`·Esc·로그아웃이 한 길로 간다.
- 트리는 Go 가 만든 중첩 구조(`[]DeptNode{Name, Users[], Children[]}`)를 받아 `<details open>` 로 그린다. 펼침은 기본이 열림이라 따로 코드가 필요 없다.
- 고른 순서 = 트리 순서(원본 `getSelectedApprovers` 와 같다). 순회 한 번으로 모은다.

### 5.2 [결재처리] 화면(`content/ApprovalScrollView.qml`)

- 보일 때(`onVisibleChanged`) 회전 표시를 켜고 결재대상목록 조회. 숨을 때 [전송 상태] 서랍을 닫고 회전 표시를 끈다.
- 단추: [승인](approve 아이콘), [반려], [새로고침], [상태보기](서랍 토글).
- 표
  - 헤더 ` # `·`선택`·`전송방향`·`요청자`·`파일명`·`파일크기`·`요청일시`·`요청사유`·`상태`. 열 폭 `[50, 40, 200, 80, 350, 80, 150, 250, 250]`.
  - 헤더 체크박스는 `checkAll(checkState !== Checked)` ― 일부·꺼짐이면 전체 켬, 켜짐이면 전체 끔.
  - 행을 누르면(현재 행 변경) 그 행만 체크(`selectRowAndCheck`). 우클릭도 같고 메뉴 [승인]·[반려]·[다운로드].
  - 상태 칸: 색 알약(`CustomStatusChip`, 톤 `FileStatus.approvalTone(aprvStateCode)`).
  - 빈 상태: `조회된 데이터가 없습니다.`
  - 접근성: 표 `approvalTable` / `결재처리 목록`, 파일명 칸 `Accessible.id = <서버파일명>.fileNameCell`.
- 신호: `APPROVAL_LIST` → 회전 끔, `CHANGED_NETWORK` → 재조회, `TRANSFER_STATUS` → 서랍 열기.
- [승인]/[반려]: 체크 0 → `결재할 항목을 선택해 주세요.`(경고 띠), 아니면 사유 창.
- [다운로드]: 체크 0 → `다운로드할 파일을 선택해 주세요.` → 겹침 판정(`받는 중`) → 클립보드면 곧바로, 아니면 폴더 고르기 → 받기.

### 5.3 [승인 사유]/[반려 사유] 창(`content/ApprovalDlg.qml`)

- 크기 350x250, 제목 `승인 사유`/`반려 사유`.
- 콤보 index 3(`직접입력`)일 때만 입력란이 열린다. 다른 항목을 고르면 입력란 = 콤보 글자.
- 입력란 최대 100자, placeholder `직접입력인 경우만 사유 입력이 가능합니다.`
- [확인]: `trim` 해서 비면 `결재 사유를 입력해 주세요.`, 아니면 `processAprv(serverIndex, isApproval, text)`.
- [취소].
- 원본 결함: `show()` 가 콤보·입력을 되돌리지 않아 앞 상태가 남는다. Wails 판은 열 때마다 index 0 과 그 글자로 되돌린다.

결재 보내기(`ViewController.cpp:1476 processAprv`)

- 체크된 행의 `seq, serverFileName, requesterId, aprvRank` 를 모은다. 비면 `선택된 결재 항목이 없습니다.`
- 작업 스레드에서 건마다 [3092]. 20건마다 1.5초 쉰다(서버 부하 배려 추정).

### 5.4 결재 받기와 내용 보기

- `warnIfApprovalDownloadBusy :957`: 결재 받기가 대기·진행 중이거나 검사 중이면 창 `받는 중` / `앞서 요청한 파일을 받는 중입니다.\n받기가 끝난 뒤 다시 받아 주세요.`
- `downloadFileForApproval :972`: `aprvDownload = true`. 클립보드는 앱 임시 폴더의 `__CLIP__` 로, 파일은 `폴더/이름`(`isDir = dir_flag`).
- 받은 뒤 지우지 않는다(APR-061).
- 끝나면 `verifyDownloadZip :1925`
  - 폴더는 그대로 훑고, zip 은 `checkZipFile(…, "결재 내용 보기")` 로 푼다.
  - 상한·디스크 부족으로 멈추면 그 사유를 적은 창.
  - 내부 파일이 있으면 [파일 검사 결과] 창: 위 표 `이름`·`폴더 확인` 또는 `압축 해제 성공`·`내부 파일 %1개`, 아래 표 `정상 파일`·이름·크기.
  - 임시 폴더는 지운다.
- 결과 창이 떠 있는 동안에는 겹침 잠금을 걸지 않는다(모달이라 뒤 표를 못 누른다).

### 5.5 [대리결재자] 화면(`content/ProxyApproverScrollView.qml`)

- 보일 때 `searchProxy()` = 등록 조회[3174] + 후보 목록[3370]. 후보 정렬 기본 3열(이름) 오름차순.
- 위 영역 `대리결재자 조회` + [조회]
  - 등록돼 있으면 이름, `대리 기간` 시작 `~` 끝, [삭제].
  - 없으면 `등록된 대리결재자가 없습니다.`
- 아래 영역 `대리결재자 등록`
  - `nameField`(placeholder `대리결재자`, 읽기 전용 ― 후보 표에서 고르면 채워짐).
  - `대리 기간` 시작/종료 날짜 고르기(종료 기본값 = 오늘 + 4일, 버튼 글자 `종료일 선택`).
  - `causeField`(placeholder `대리결재 사유`). [등록].
- 후보 표 헤더 ` # `·`ID`·`이름`·`부서`·`직급`·`직위`(접근성 `proxyApproverTable` / `대리결재자 목록`, 칸 `nameCell`).
- 결과 신호 `PROXY_APPROVER_CHANGED`/`DELETED` → 메시지 창에 결과 문구 → 다시 `searchProxy()`.

Wails 판: 날짜는 `<input type="date">`(네이티브)로 충분하다. 종료일 `min` 을 시작일로 두어 앞선 날을 고를 수 없게 한다.

### 5.6 메뉴(`content/HomePage.qml`)

| 계정 | 메뉴 |
|---|---|
| 결재자(`aprv_yn`) | 파일전송·전송내역·결재처리·결재내역·대리결재자·로그·설정 (정책은 숨김) |
| 대리결재자(`replace_yn`) | 파일전송·전송내역·결재처리·로그·설정 |
| 일반 | 파일전송·전송내역·로그·설정 |

### 5.7 트레이·메시지

- 결재요청 도착: 트레이가 있으면 풍선 `결재 요청` / `파일 이동에 대한 승인 요청 건이 있습니다.`(정보, 5초), 없으면 알림 띠(정보).
- 결재 완료: 트레이가 있으면 풍선 `결재 완료` / 본문(3.4절), 없으면 메시지 창.
- 오류(`E_NOTIFY_ERROR :829`): 기본 제목 `통신 오류 발생`, 기본 문구 `서버와의 통신 중에 오류가 발생하였습니다.`.
  `msg` 앞 4글자가 숫자면 제목 `요청 처리 오류`, 문구는 `\` 뒤 부분.

Wails 판: Wails v3 의 시스템 트레이와 알림 서비스(`pkg/services/notifications`)로 OS 알림을 띄우고, 실패하거나 권한이 없으면 앱 안 알림 띠로 바꾼다.
macOS 는 알림 권한을 처음에 물어야 하고, Linux 는 알림 데몬(DBus `org.freedesktop.Notifications`)이 없으면 실패한다 ― 둘 다 앱 안 알림 띠로 떨어진다.

---

## 6. 경계·오류 상황

| 상황 | 원본 | Wails 판 |
|---|---|---|
| 부서 정보[3250] 실패·0건 상태에서 [결재요청] | 창이 안 뜨고 업로드 목록 주인이 안 풀려, 로그아웃 전까지 사람의 업로드가 거절된다(APR-013, 보류 067·071) | [3250] 을 부르지 않으므로 생기지 않는다. 창 준비가 어떤 이유로든 실패하면 늘 목록을 놓고 사유를 보인다 |
| 결재자 목록이 로그인 시점 것 | 로그인 뒤 등록된 대리결재자가 다시 로그인할 때까지 안 보인다(APR-014) | 창을 열 때마다 새로 받는다 |
| 트리 부모 부서 없음 | 빈 포인터에 붙여 앱이 죽을 수 있다 | 뿌리에 단다. 같은 부서 ID 는 한 행 |
| 결재자 0명 | 안내 없이 목록을 놓는다(APR-033) | `결재자가 없습니다.` 창 뒤 목록을 놓는다 |
| 자기 지정 | 창이 닫힌 뒤 거절, [결재요청]부터 다시 | 창 안에서 막고 다시 고르게 한다 |
| 결재요청[4127] 성공 | 화면 변화 없음 | `결재요청이 완료되었습니다.` 알림 띠 + 서버파일 목록 재조회 |
| 결재요청[4127] 실패 | `통신 오류 발생` 기본 문구 또는 로그만 | 실패 묶음 수와 서버 사유. 이미 성공한 묶음은 되돌리지 않는다(서버에 취소 RPC 없음, `ApprovalService.proto` 검색) |
| 결재요청 상신 직전 상태가 바뀜(개인정보 검사가 끝나지 않음 등) | 마지막으로 받은 목록으로 판정해 경합(APR-009) | 상신 직전에 서버파일 목록[3311]을 새로 받아 판정한다 |
| 결재처리 실패 | 조용함, 목록도 안 바뀜(APR-058) | 실패 건수·사유를 보이고 목록을 다시 받는다 |
| 결재처리 다수 | 상한 없음, 20건마다 1.5초 | 동시 4건, 상한 없음. 100건 기준 시간은 7절 측정 항목 |
| 결재대상목록 조회 실패 | 회전 표시가 화면을 떠날 때까지 돈다(APR-053) | 끄고 사유를 보인다 |
| 결재 받기 겹침 | `받는 중` 창 | 같다 |
| 결재자가 암호 압축을 받음 | 비밀번호 없이 풀다 실패하면 창 없음 | 같다(P2 에서 결재대상목록의 `zip_pswd` 로 풀기 검토) |
| 대리 기간 종료 < 시작 | 막지 않는다(음수 일수는 초과 아님) | 날짜 입력 `min` 과 판정 둘 다로 막는다. 문구 `종료일이 시작일보다 앞섭니다.`(새 문구) |
| 대리결재자 삭제 인자 순서 | 사유 칸에 망 번호(`"0"` 추정)가 가고 망 번호에 사유 글자가 간다 | 사유 `대리결재자 삭제` 를 싣는다 |
| 인터넷망 결재 | 메뉴가 안 나온다. 서버도 `outer server: approval operation not supported (proxy only)` | 같다. 메뉴 판정을 망 위치까지 본다(인터넷망 서버도 `aprv_yn` 을 참으로 준다) |
| 로그아웃 뒤 늦은 알림·응답 | 버린다(APR-074) | 세션 세대 번호로 버린다 |
| `approverSearchType_` 미초기화 | 서버가 `invalid type: 2` 로 거절할 수 있다 | 늘 1 |
| 결재요청 압축 비밀번호 128자 초과 | 업로드 때 싣지 않음 | 같다 |
| 결재 완료 알림 | 서버가 요청자에게 안 넘김 | 같다(서버 수정 대기). 요청자 화면은 9001 로 상태가 바뀐다 |

---

## 7. 로컬 처리와 Go 패키지 후보

### 7.1 파일 배치(지금 저장소 구조를 따른다 ― main 패키지 평면)

| 파일 | 하는 일 |
|---|---|
| `approval.go` | RPC 감싸기: `ApproverList`, `RequestApproval`(4127 묶음), `ApprovalList`, `Approve`(병렬), `ProxyGet/Register/Delete`, `notifyEndApproval`. 응답 판정 함수 하나(3.3절) |
| `approval_rules.go` | 순수 함수: `aprvType(loc, isClip)`, `aprvTypeText`, `aprvStateText`, `canRequest(virus, priv, aprv)`, `buildApproverTree(list)`, `groupByZipPswd(names, pswds, 10)`, `deputyDaysOK(from, to, limit)` |
| `approval_rules_test.go` | 위 순수 함수의 표 시험(트리 부모 없음, 중복 ID, 10개 묶기, 날짜 경계 `+1`) |
| `app.go` | 화면이 부르는 메서드 추가(결재자 창 준비·답, 결재처리, 대리결재자) |
| `frontend/src/` | `ApproverDialog.svelte`, `ApprovalView.svelte`, `CauseDialog.svelte`, `ProxyApproverView.svelte` |

새 인터페이스·추상화는 만들지 않는다. 화면 상태(업로드 목록 주인 등)는 `03-upload` 가 만드는 것을 그대로 쓴다.

### 7.2 로컬 처리 규칙

- **결재방식**: `aprvType(loc, isClip)` ― `loc=="1"` 이면 업무망 값, 아니면 인터넷망 값. 파일/클립보드 둘. 다운로드에는 결재방식이 없다(-1).
- **결재요청 가능**: 3.6절의 세 단계. `virus_flag` → `priv_state` → `aprv_state==4`.
- **트리**: `aprv_state==1` 만, 서버 순서. 부서 ID 맵. 부모 = 마지막 `|` 앞. 없으면 뿌리. 사용자 ID 중복은 첫 번째만.
- **비밀번호 묶기**: 같은 비밀번호끼리 순서를 지키며 10개씩. 빈 비밀번호도 하나의 묶음 열쇠다.
- **대리 기간**: `일수 = (끝 - 시작) + 1`, `일수 > 상한` 이면 막음. 형식이 틀리면 원본은 막지 않는다(false) ― Wails 판은 날짜 입력이 형식을 보장하므로 형식 오류는 막는다.
- **날짜 문자열**: 서버 `YYYYMMDD` ↔ 화면 `YYYY-MM-DD`. `time.Parse("20060102", …)`.
- **글자 인코딩**: gRPC 는 UTF-8 이다. 원본의 EUC-KR 변환(`safeStrCopy`)은 고정 칸 때문이라 Go 에는 필요 없다(`golang.org/x/text/encoding/korean` 불필요).
- **결재 병렬**: 버퍼 채널(크기 4)을 세마포어로 쓰는 `sync.WaitGroup` 루프. 건별 결과(성공/실패 사유)를 모아 한 번에 화면에 돌려준다.
  `ponytail:` 동시 4건 고정 ― 서버 부하가 보이면 정책 키로 뺀다.
- **알림 1초 지연 합치기**: `time.AfterFunc` 하나를 다시 걸어 마지막 요청만 돈다.
- **세션 세대**: 로그인마다 늘리는 정수. 응답·알림 처리 시작에서 비교해 다르면 버린다.

### 7.3 외부 Go 패키지 후보

| 패키지 | 쓰는 곳 | 판단 |
|---|---|---|
| `google.golang.org/grpc`, `google.golang.org/protobuf` | 모든 RPC | 이미 있음 |
| `github.com/wailsapp/wails/v3`(트레이, `pkg/services/notifications`, 폴더 고르기 창) | 결재 알림 풍선, 결재 받기 폴더 | 이미 있음(같은 모듈) |
| 표준 `archive/zip`, `path/filepath`, `time`, `sync` | 내용 보기·날짜·병렬 | 표준 |
| `golang.org/x/sync/errgroup` | 결재 병렬 | 쓰지 않는다 ― 건별 실패를 다 모아야 해서 첫 오류에 멈추는 errgroup 보다 WaitGroup + 채널이 맞다 |
| `github.com/yeka/zip` | P2: 결재자가 받은 암호 zip(ZipCrypto·AES) 내용 보기 | P2 때만. `02` 압축 영역과 같은 결정 |
| `github.com/bodgit/sevenzip` | P2: 7z 내용 보기 | 원본 `ZipExtractor`(bit7z)가 7z 를 푼다. `02` 영역 결정에 따른다 |

---

## 8. 원본보다 낫게 할 곳

1. **부서 계층[3250]·조직[3252] 조회를 없앤다.** 원본에서 이 둘은 창을 띄울지 말지의 문(門) 역할만 하고 결과는 트리에 쓰지 않는다
   (`makeAprvTree(dept, approvers)` 의 `dept` 미사용, `orgInfoResponse` 는 빈 반복문). 이것이 APR-013(창이 안 뜨고 업로드가 막힘)의 원인이다.
   트리는 [3370] 의 `user_dept_id`/`user_dept_name` 만으로 만든다. 서버 [3370] 은 [3250] 과 독립된 쿼리다(`svc_getapproverlist.go`).
2. **결재자 목록을 창을 열 때마다 새로 받는다**(APR-014). 대리결재자 표시 `(대결:…)` 가 다음 로그인까지 늦는 문제가 사라진다. 응답 크기는 전사 결재자 수에 비례한다 ― 추정: 수천 명 이하라 1초 안. 9절에서 잰다.
3. **트리 부모가 없어도 죽지 않는다.** 뿌리에 달고 로그를 남긴다.
4. **결재요청 결과를 보인다.** 성공 띠 `결재요청이 완료되었습니다.`(원본에 문구는 있으나 주석) + 서버파일 목록 재조회. 실패는 서버 사유를 보인다.
5. **결재처리를 병렬로 보내고 결과를 모아 보인다.** 원본은 20건마다 1.5초 쉬며 직렬이고 실패는 조용하다. Wails 판은 동시 4건, 끝나면 `승인 N건 완료, 실패 M건` + 실패 사유 목록, 그리고 목록 재조회 한 번(원본은 건마다 재조회).
6. **자기 지정을 창 안에서 막는다.** 창이 닫힌 뒤 처음부터 다시 하게 하지 않는다. 판정 규칙(`self_approve`)은 같다.
7. **상신 직전 서버파일 목록을 새로 받는다**(APR-009 경합 해소). 한 번의 [3311] 로 상태를 다시 판정한다.
8. **정렬이 실제로 먹는다.** 요청일시 내림차순 기본, 헤더 눌러 바꾸기.
9. **회전 표시가 실패에도 꺼진다**(APR-053).
10. **사유 창이 열 때마다 초기화된다.**
11. **대리결재자 삭제 인자를 바로 싣고, 종료일 < 시작일을 막는다.**
12. **결재자 없음 안내를 띄운다**(APR-033).
13. **결재 알림 조회 합치기.** 9002 가 몰려 와도 1초 창 안의 조회는 한 번.
14. P2 후보(서버 변경 없음)
    - 결재처리 화면에 개인정보 건수·상세 열(APR-059) ― [3390] 응답에 이미 있다.
    - 결재자 로그인 때 지연 건수[3638](APR-076) ― 문구는 원본 `자료승인 요청 지연이 %1건 있습니다.\n요청일로부터 %2일 이내에 결재 처리하지 않을 경우\n경영평가 및 국정원 실태평가에 감점 사항이 됩니다.`
    - 결재자가 받은 암호 압축을 [3390] 의 `zip_pswd` 로 풀어 보이기.

---

## 9. 실 서버 검증 방법

비밀번호는 문서에 적지 않는다. 계정 비밀번호는 원본 저장소 `tools/.secrets/login.json` 에 있고, Wails 판 실 서버 시험은 환경변수로 받는다.

### 9.1 준비

- 서버: 업무망(결재는 업무망에서만). `fmenv.ini`·`root_ca.pem` 은 원본 개발 빌드 것을 쓴다: `$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'`.
- 계정 역할(`tools/e2e/README.md`): 요청자 `test<N>_1`, 결재자 `test<N>_2`(요청자의 결재선에 있어야 한다), 대리결재자 `test<N>_3`.
- 서버 정책은 SBM 으로 바꾸는 `tools/srv_settings.py` 를 쓴다(권한 프로파일 단위, 바꾼 뒤 다시 로그인해야 먹는다). `fm_upfile_mode` 0/1 두 모드를 다 본다.
- 공용 자원 주의: autoqa e2e 가 같은 서버·시험 계정 5개(프로파일 `E2E-QA`)를 쓴다. 그 실행과 겹치지 않게 시각을 피한다.
- 대리결재 지정은 다음 실행까지 남는다. 시험 끝에 삭제하거나 원본 E2E 의 `55-reset-proxy` 로 지운다.
- 인터넷망 서버(.158)에는 mysql 이 없다. DB 확인은 업무망(.157)에서 한다.

### 9.2 Go 실 서버 시험(`live_test.go` 에 더함, `SNB_LIVE_ID` 가 있을 때만)

| 시험 | 순서 | 통과 기준 |
|---|---|---|
| 결재자 목록 | 요청자로 로그인 → [3370] type 1 | 오류 없음, `aprv_state==1` 인 행에 결재자 계정 ID 가 있다, 트리 함수가 뿌리 1개 이상을 만든다 |
| 사후결재 상신 | `fm_upfile_mode=1` 요청자로 업로드 → 서버파일 목록에서 상태 4 행 → [4127] | `res==1`, 서버파일 목록에서 그 행 상태가 0(`결재 요청`)이 된다 |
| 결재대상 | 결재자로 로그인 → [3390] | 방금 상신한 `file_name_svr` 가 목록에 있다 |
| 승인 | 결재자 → [3092] `aprv_state=1` | `res==1`, `Reason "APPROVE OK !!!"`, 다시 [3390] 하면 그 행이 없다, 요청자의 서버파일 목록 상태가 1 |
| 반려 | 위와 같고 `aprv_state=2` | 요청자의 상태가 2(`반려됨`) |
| 대리결재 | 결재자 → [3272](오늘~오늘+4) → [3174] → [3276] → [3174] | 등록 뒤 `alt_userid` = 대리결재자 ID, 삭제 뒤 빈 값 |
| 기간 상한 | 순수 함수 시험 | 상한 5 에서 5일 통과, 6일 막힘, 끝<시작 막힘 |

```powershell
$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'; $env:SNB_LIVE_ID = '…'; $env:SNB_LIVE_PW = '…'
go test -run 'Live.*(Approv|Proxy)' -v -count=1 .
go test -run 'Aprv|Approver|Deputy|ZipPswd' -v .   # 순수 함수(서버 없이)
```

시험이 서버에 남기는 것: 업로드한 `WAILS_<시각>.txt` 와 결재 이력. 대리결재 등록은 시험 안에서 지운다.

### 9.3 화면 확인(Wails 판을 띄워 직접)

1. 사후결재 요청자: `결재요청 가능` 행을 골라 [결재요청] → 창이 뜬다. 결재자 없이 [결재요청] → `결재자를 선택해 주세요.` 사유 비움 → `결재요청 사유를 입력해 주세요.`
2. 같은 창에서 Esc → 창이 닫히고 바로 다른 파일 업로드가 된다(업로드가 막히지 않음).
3. 결재자: 원본 앱과 Wails 판을 같은 계정으로 나란히 보지 않는다(중복 로그인). 요청자 쪽에서 상신 직후 결재자 Wails 판에 풍선 `결재 요청` 이 뜨고 [결재처리] 목록이 1초쯤 뒤 바뀐다.
4. 결재자: 2건 체크 → [승인] → 사유 `직접입력` + 빈 칸 → `결재 사유를 입력해 주세요.` → 사유 입력 → 목록에서 2건이 사라진다.
5. [대리결재자]: 종료일을 시작일 앞으로 고를 수 없다. 6일 이상이면 `대체결재자 유효기간이 설정된 값을 초과하였습니다. [5 일]`.
6. 사전결재 요청자(`fm_upfile_mode=0`): 파일 업로드 → 검사 통과 → 결재자 창 → 고르고 [결재요청] → [전송 상태] 서랍이 열리고 업로드된다. 결재자 목록에 그 건이 나온다.

### 9.4 원본과 대조

- 같은 시나리오를 원본 E2E 로 돌려 기준을 잡는다: `.\tools\run-e2e.ps1 -Launch -Desktop -Auto -Only request,decide,decide-proxy,proxy`(원본 저장소). 결과의 `전체/통과/실패/건너뜀` 과 결과 JSON 을 본다.
- 서버 쪽 판정은 MCP 도구(`mcp__snb__login`·`list_approvers`·`request_approval`·`list_pending_approvals`·`approve`)로 Wails 판이 만든 결재 건을 원본 경로로 읽어 볼 수 있다(같은 서버, 같은 RPC).
- 측정: 결재자 창 열기(3370 왕복)·결재 20건 처리 시간을 원본(20건 = 직렬 + 쉼 없음 구간)과 Wails 판(동시 4)으로 각 5회 재서 중앙값을 남긴다.
