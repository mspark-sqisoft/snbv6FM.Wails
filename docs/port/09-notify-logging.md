# 이식 설계: 다중 망·서버 알림·앱 로그 (영역 09·11)

원본: `E:\git\snBv6FM` (Qt Quick + C++, 기준 커밋 117020c, 2026-10-30 읽음)
대상: `E:\git\snBv6FM.Wails` (Go + Wails v3 + Svelte 5)
원본 기준 문서: `specs/current/09-multi-network-notify.md`, `specs/current/11-logging.md`

이 문서는 원본 소스를 다시 읽지 않고 구현할 수 있게 쓴다. 근거는 `원본 파일:줄` 로 붙인다(줄 번호는 위 커밋 기준).
명세의 줄 번호는 그 뒤 코드가 밀려 어긋난 곳이 많아, 아래 줄 번호는 이번에 직접 연 소스의 것이다.
추측은 **추정:** 으로 표시한다.

---

## 1. 범위

### 이 문서가 맡는 것

- **접속 대상(망) 목록**: `fmenv.ini` 의 `[SERVERnn]` 읽기, `fmlocal.ini` 로 이 PC 의 망을 맨 앞에 두기, 로그인 화면 망 콤보·저장, 툴바 접속 망 배지, 전송 방향 문구.
- **다중 망 세션**: 원본은 한 번에 한 망만 로그인한다. Wails 판에서 서버(망)마다 세션 객체를 따로 두는 구조와, 두 망을 동시에 여는 확장(P2).
- **서버 알림 스트림**: `NotifyService.NotifyServerStream`(서버 → 앱) 열기·닫기, 전문 번호(9001·9002·9004·9005·9007)·`kind` 별 처리, 연결 끊김 창, 앱 → 서버 `NotifyService.NotifyOnce`.
- **공통 안내 부품**: 확인 창(하나뿐인 `msgBox`), 화면 아래 알림 띠(InfoBar, 4초), 트레이 풍선 알림, 서버 글자를 서식 없이 그리기.
- **단일 요청 응답 상한 15초**와 공통 오류 창(`E_NOTIFY_ERROR`).
- **앱 로그 파일**: 위치·이름·한 줄 형식(`FmLog.h`)·수준·분류·반복 억제·첫 줄·여러 프로세스 덧붙이기·보관/용량 정리·남기면 안 되는 값·설정 변경 기록·종료 경로 기록.
- **[로그] 화면**(`LogScrollView.qml`): 날짜·수준·분류·단어 조회, 표, 정렬, 빈 상태.
- **GUI 멈춤 감시**(`SNB_FREEZE_WATCH=1`).

### 다른 문서로 넘기는 것

| 무엇 | 문서(영역) |
|---|---|
| PreLogin·Login 전문, 로그인 실패 문구, 로그인 응답 `MultiLogin`, 세션 만료(자동 로그아웃) 타이머 | 01 auth-session |
| 9001·9004 kind 0 이 부르는 서버 파일 목록 다시 조회(`GetServerFileList`) | 04 download |
| 9002·9004 kind 1 이 부르는 결재 목록 다시 조회, 결재 완료 9004 kind 5 를 **보내는** 결재 처리, 대결 | 06 approval |
| 9004 kind 4 URL 전송으로 브라우저 열기(링크연동) | 08 link-external |
| 9005 관리자 수동 무결성 검사(검사유형 2), `root_ca.pem` 검증 | 14 integrity-security |
| 트레이 아이콘 자체·트레이 메뉴·툴바 배치·[설정] 의 나머지 항목(로그아웃 시 동작 콤보 등) | 12 main-window, 13 settings |
| 설치본에 담는 `fmenv.ini`·`fmlocal.<망>.ini` | 15 build-deploy |
| 헬퍼(업데이터·버전메이커)의 로그 파일 | 15 build-deploy (형식만 이 문서 7.3 을 재사용) |
| 기능마다 무슨 문장을 로그로 남기는가(전문 번호·값) | 각 영역 문서 |

---

## 2. 기능 목록

우선순위: **P1** 원본과 동등 필수 / **P2** 있으면 좋음 / **제외** + 이유. 난이도: 상·중·하.
원본 상태가 '구현'·'부분'·'상충' 인 요건은 전부 넣었다(09: 44건, 11: 43건, 합계 87건).

### 2.1 다중 망 (09 의 NET-001~017, 064)

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| NET-001 | `[SERVER01]` 부터 번호를 올려 읽고 `IP` 가 빈 첫 구획에서 멈춤. `IP` 는 `_` 로 여럿, `PORT` 없으면 29001, `NAME`·`NETWORK` 없으면 `NONAME` | P1 | 하 | **Wails 의 `readServers` 는 지금 다르다**: `[SERVER` 로 시작하는 모든 구획을 파일 순서로 읽고, 기본 포트 29002, `_` 를 가르지 않고, `NETWORK` 를 읽지 않는다. 고친다(7.1) |
| NET-002 | `LOC` 가 비었거나 0·1 이 아니면 경고 후 1(업무망) | P1 | 하 | **Wails 는 빈 `LOC` 를 인터넷망으로 다룬다**(`s.Server.Loc == "1"` 비교, `client.go:157`) ― 로그인 `in_out` 과 결재 방식이 뒤바뀐다. 읽을 때 정규화한다 |
| NET-003 | `IP` 를 갈랐는데 남는 주소가 없으면 그 구획을 건너뛰고 경고(다음 번호로 계속) | P1 | 하 | |
| NET-004 | `fmenv.ini` 옆 `fmlocal.ini [NETWORK] LOC` 와 같은 망의 서버를 목록 맨 앞에. 없거나 0·1 이 아니면 파일 순서 그대로 | P1 | 하 | 이 순서가 콤보 순번·세션 배열·저장 순번의 축이다 |
| NET-005 | 접속을 정하는 구획(`[SERVERnn]`·`[GRPC]`·`[WASM]`·`[CONFIG]`)을 사용자 설정으로 덮지 못함 | P1 | 하 | Wails 는 사용자 설정 파일에서 이 키를 아예 읽지 않으면 끝(구조로 충족). 사용자 설정에 그 키가 있으면 경고 한 줄 |
| NET-006 | 로그인 화면 망 콤보(이름만, IP 안 보임), 고른 순번을 저장해 다음 실행 기본값 | P1 | 하 | 원본은 레지스트리 `AppSettings.serverIndex`. Wails 는 사용자 설정 파일(13 영역과 같은 저장소) |
| NET-007 | 로그인 때 콤보 순번의 서버에 붙음. 범위 밖이면 저장값, 그것도 밖이면 0 | P1 | 하 | |
| NET-008 | 로그인마다 서버별 통신 객체를 새로 만들고 같은 순번의 IP·이름·LOC·포트를 복사 → 화면 이름과 실제 대상이 같은 순번 | P1 | 하 | Wails: `Session` 이 `Server` 값을 복사해 든다(이미 그렇다) |
| NET-009 | 고른 서버 하나에만 로그인 | P1 | 하 | 두 망 동시 세션은 P2(5.2·8.1) |
| NET-010 | IP 목록을 앞에서부터 시도, 처음 성공한 것 사용(원본은 채널이 지연 연결이라 사실상 첫 IP 만 씀) | P2 | 중 | Wails 는 실제 연결 성공(PreLogin 성공)으로 판정해 다음 IP 로 넘어가게 하면 원본보다 낫다(8.3) |
| NET-011 | 늘 TLS. `root_ca.pem` 을 못 읽으면 붙지 않고 실패 | P1 | 하 | **명세의 "TLS=0 이면 http" 는 지금 코드와 다르다** ― `grpcTlsEnabled()` 가 `TLS=0` 을 경고만 남기고 무시하고 늘 참(`FileMoverGrpc.cpp:388-396`). Wails 는 이미 늘 TLS |
| NET-012 | 고른 서버의 IP 가 없거나 포트 0 이거나 채널 생성 실패면 "서버 접속에 실패했습니다. - N" → "서버정보가 없어 로그인에 실패했습니다." → 앱 종료 | P1 | 하 | 원본은 두 안내를 같은 창에 연달아 덮어쓰고 곧바로 종료(읽을 틈 없음). Wails 는 안내를 닫은 뒤 종료(8.4) |
| NET-013 | 로그인 상태에서 툴바에 접속 서버 `NAME` 배지 | P1 | 하 | 배치는 12 |
| NET-014 | 전송 방향 문구 = 그 `LOC` 를 가진 서버의 `NETWORK` 값, 없으면 `Unknown` | P1 | 하 | [사용자 정보] 팝업·목록 [전송방향] 칸이 같은 함수 |
| NET-015 | 로그인 뒤 망을 바꾸는 콤보를 보이지 않음. 바꾸려면 로그아웃 → 로그인 화면 | P1 | 하 | 원본은 화면마다 콤보가 `visible:false` 로 숨어 있다. Wails 는 만들지 않는다 |
| NET-016 | 화면에서 부를 길 없는 망 전환 함수(`changeNetwork`): 앞 망 로그아웃 → 같은 ID·비밀번호로 새 망 로그인 | 제외 | ― | 비밀번호를 로그인 뒤까지 들고 있어야 해 원본의 비밀번호 즉시 소거(087)와 충돌. 화면 경로도 없다 |
| NET-017 | WASM 은 `[WASM] INNER/OUTER_ENVOY_ADDRESS` 로 접속 | 제외 | ― | WASM 전용 |
| NET-064 | 망 이름은 서버에서 받지 않고 `fmenv.ini` 의 `NAME`·`NETWORK` 그대로(언어를 바꿔도 그대로) | P1 | 하 | 상태 '부분' 은 이전 목표 명세(서버에서 망 이름 받기) 대비. 원본 동작 그대로 옮긴다 |

### 2.2 알림 스트림·전문 처리 (NET-020~044)

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| NET-020 | 로그인 응답을 받으면 `NotifyServerStream` 을 연다(로그인 응답 직후, 프로파일 조회 전) | P1 | 중 | `FileMoverGrpc.cpp:996-997` |
| NET-021 | 로그인 요청에 `cancel_9003="1"` 로 9003 을 받지 않음. 9001·9002·9004·9005·9007 밖은 로그만 | P1 | 하 | Wails 로그인은 이미 `Cancel_9003: "1"` |
| NET-022 | 알림마다 `Notify 수신: 전문번호[..] kind[..] side[..] alim[..] msg[..] userid[..] ipaddr[..] dat[..]` 한 줄 | P1 | 하 | E2E 가 이 줄로 판정한다 ― 글자 그대로 |
| NET-023 | 로그아웃 때 로그인 상태를 먼저 내리고 스트림을 취소 → 그 종료를 끊김으로 보지 않음 | P1 | 중 | Go: `loggedIn.Store(false)` 후 `cancel()` |
| NET-024 | 로그인 상태에서 스트림이 끝나면 재연결 없이 끊김 처리: 계정 칸 비움 → 로그인 상태 내림 → "연결 끊김" 창. 창을 닫으면 [설정] 의 로그아웃 시 동작(0 로그아웃·1 종료·2 트레이로) | P1 | 중 | 데스크톱 재연결은 원본에 없다. Wails 의 선택적 재연결은 P2(8.2) |
| NET-025 | WASM 스트림 재연결(최대 5번, 2·4·8·16·32초), 인증 만료면 멈춤 | 제외 | ― | WASM 전용. 단, 데스크톱 재연결을 넣는다면 이 간격 규칙을 그대로 쓴다(8.2) |
| NET-026 | WASM 서버 스트림을 `fetch` 읽기 스트림으로 | 제외 | ― | WASM 전용 |
| NET-027 | 로그아웃 뒤 도착한 세션 알림 12종을 버리고 `로그아웃 뒤 도착한 알림을 버립니다` | P1 | 하 | 목록은 3.6 |
| NET-028 | 앱 → 서버 알림 한 건(`NotifyOnce`) | P1 | 하 | **명세는 "부르는 곳 없음" 이지만 지금 코드는 결재 승인·반려 성공 때 요청자에게 9004 kind 5 를 보낸다**(`FileMoverGrpc.cpp:2140-2157`, 106). 함수는 여기, 부르는 쪽은 06 |
| NET-030 | 9001: `userid` 가 비지 않고 내 ID 와 다르면 무시, 아니면 1초 뒤 서버 파일 목록 다시 조회 | P1 | 하 | |
| NET-031 | 9002: `userid`==`msg`(자가승인)면 무시. 아니면 1초 뒤 결재 목록 다시 조회. `dat`=="process_replace_approval" 이면 거기서 끝, 아니면 결재 요청 알림(트레이 풍선 5초, 트레이 없으면 알림 띠) | P1 | 중 | 트레이 풍선 = Wails notifications 서비스(7.6) |
| NET-032 | 9004 `userid` 가 남의 것이면 무시. kind 6(다중로그인 종료)·side 0 이면 "중복 로그인" 창 → **창을 닫으면** 앱 종료 | P1 | 중 | **명세의 "창을 기다리지 않고 종료" 는 옛 동작** ― 지금은 닫힘 콜백 + `m_exitWhenNoticeClosed`(`ViewControllerNotifySlot.cpp:750-758`, 2026-10-26) |
| NET-033 | 9004 kind 7 도 kind 6 과 같이 종료 | P1 | 하 | 상충(MOSFM 은 무시). 원본 snbApp 과 같게 종료로 옮기고 비고에 남긴다. 6절 |
| NET-034 | 9004 kind 0: 1초 뒤 서버 파일 목록 **과** 결재 목록 둘 다(`break` 누락으로 1 로 이어짐). kind 1: 결재 목록만 | P1 | 하 | 결과가 해롭지 않아 같게 둔다. 6절 |
| NET-035 | 9004 kind 2·3(메일)·모르는 kind 는 로그만 | P1 | 하 | |
| NET-036 | 9004 kind 4: `dat` 의 URL 로 링크연동 알림 | P1 | 하 | 동작은 08 |
| NET-037 | 9004 kind 5: 결재 완료 ― `dat`=="Y" 승인, 아니면 반려. 트레이 풍선(없으면 확인 창) | P1 | 하 | |
| NET-038 | 9005: 관리자 요청 무결성 검사(검사유형 2) 시작 | P1 | 하 | 검사 자체는 14 |
| NET-039 | 9007 전체공지: `msg`(제목)+"_"+`dat`(본문)로 이어 넘기고, 창 제목 = 첫 `_` 앞, 본문 = 그 뒤 전부. 밑줄 없어도 안 죽음. `userid`·`kind`·`alim` 안 봄 | P1 | 하 | Wails 는 이어 붙이지 말고 제목·본문을 따로 넘기는 편이 낫다(제목에 `_` 가 있어도 안 잘림). 8.5 |
| NET-040 | 실서버가 지금 9007 을 못 보냄(SR034 미발송) → 시험 진입점으로만 확인 | P1 | 하 | Wails 에도 개발용 알림 주입 진입점을 둔다(9절) |
| NET-041 | 확인 창은 하나(`msgBox`). 열린 채 새 것이 오면 제목·본문 덮어쓰기, 닫힘 콜백도 하나 | P1 | 중 | Wails 는 줄 세우기(큐)로 원본보다 낫게(8.5). 단 "닫으면 종료" 콜백이 덮여 사라지지 않게 한다 |
| NET-042 | 화면 아래 알림 띠: 4초 뒤 닫힘, 새 안내는 덮어쓰고 시간 다시 잼, 모달보다 위, 뒤 클릭 안 막음, 심각도 4종은 아이콘·테두리 색만 | P1 | 하 | 5.5 |
| NET-043 | 서버·상대망 글자를 서식 해석 없이 그림(공통 부품 5종) | P1 | 하 | Svelte 는 `{text}` 가 기본 이스케이프. **`{@html}` 금지** 규칙 하나로 충족 |
| NET-044 | 공통 부품 밖 자리(결재자 선택창 이름, 중복 확인 파일명, 탐색기, 로그인 실패 사유, 카드, 업데이터 경로)도 서식 해석 끔 | P1 | 하 | 위와 같은 규칙 |

### 2.3 응답 상한·공통 오류 (NET-050·051)

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| NET-050 | 단일 요청마다 호출 시점부터 15초 상한, 넘으면 취소·실패. 스트림에는 안 걸음. 상태 코드는 남김 | P1 | 하 | Wails 는 이미 `rpcTimeout` 15초(`client.go:36`). 스트림은 `ctx(0)` |
| NET-051 | 요청 실패 알림: 내용이 `<전문번호>\<문구>` 면 "요청 처리 오류" 창에 문구, 아니면 "통신 오류 발생" / "서버와의 통신 중에 오류가 발생하였습니다." | P1 | 하 | 문구는 첫 `\` 뒤 **둘째 칸 하나만**(원본 결함: `3127\1\결재요청이 실패하였습니다.` 가 `1` 로 보인다). Wails 는 Go 에서 구조체로 넘겨 이 문제를 피한다(8.6) |

### 2.4 앱 로그 (11 의 LOG-001~046)

| ID | 기능 | 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| LOG-001 | 로그를 사용자 데이터 폴더 아래 `log\` 에 씀(Windows `%LOCALAPPDATA%\SQIsoft\snB App\log`). 실행 파일 옆에는 안 씀 | P1 | 하 | macOS `~/Library/Application Support/SQIsoft/snB App`, Linux `~/.local/share/SQIsoft/snB App` |
| LOG-002 | 개발 빌드가 `SNB_INSTANCE=<영숫자·-·_>` 로 뜨면 앱 이름 `snB App <이름>` 으로 폴더 분리. 설치본은 무시하고 경고 | P2 | 하 | 개발 빌드 판정은 Wails 에 맞게 새로(추정: `-tags dev` 또는 `buildinfo`). E2E 다중 자리를 쓸 때 필요 |
| LOG-003 | 로그 폴더가 있을 때만 씀. 폴더가 생기면 재시작 없이 다음 줄부터 | P1 | 하 | |
| LOG-004 | [설정] [로그 저장] 켜면 폴더를 먼저 만들고 값 저장, 끄면 파일 닫기 + 폴더 통째 삭제를 로그 잠금 하나 안에서. 실패면 WARN. 기본 꺼짐 | P1 | 중 | 체크상자 배치는 13 |
| LOG-005 | 새 형식을 `yyyyMMdd.v2.log` 에 UTF-8·LF. 경로는 한 함수에서만 | P1 | 하 | 이름은 `.v2.log` 그대로(읽는 도구들이 이 이름을 본다, 7.3) |
| LOG-006 | 옛 형식 `yyyyMMdd.log`(`^^` 구분, CRLF)도 나란히 씀 | 제외 | ― | 읽는 앱 기능이 없고 원본도 037 3단계에서 지울 예정. 디스크 두 배 |
| LOG-008 | 한 줄 = `날짜 시각 수준 프로세스 계정@망 분류 파일:줄 메시지`, 머리 7칸 공백 없음(공백→`_`, 빈 값→`-`), 로컬 시각 `yyyy-MM-dd hh:mm:ss.zzz` | P1 | 하 | 7.3 |
| LOG-009 | 수준 5자 고정폭 `DEBUG`·`INFO `·`WARN `·`ERROR`·`FATAL` | P1 | 하 | |
| LOG-010 | 프로세스 칸 = PID, 시험 인스턴스면 `이름:PID`. 시작 때 한 번 정함 | P1 | 하 | |
| LOG-011 | 계정 칸: 로그인 완료(중복 로그인 분기 뒤)에 `ID@망이름`, 끊김·로그인 실패·세션 만료·로그아웃에서 비움, 비면 `-` | P1 | 하 | 두 망 동시 세션(P2)이면 5.2 의 규칙 |
| LOG-012 | 분류 11개(`snb.app`·`snb.auth`·`snb.transfer.upload`·`.download`·`.clipboard`·`.auto`·`snb.approval`·`snb.check`·`snb.settings`·`snb.grpc`·`snb.ui`), 분류 없으면 `default`, 화면 쪽은 `qml` | P1 | 하 | Svelte 쪽 로그는 `qml` 대신 같은 이름 `qml` 을 쓴다(필터 호환). 7.3 |
| LOG-013 | 파일:줄 칸은 파일 이름만(`/`·`\` 둘 다 구분자). 정보 없으면 `-:0` | P1 | 하 | Go `runtime.Caller` |
| LOG-014 | 메시지의 큰따옴표 전부 삭제, `\r` 삭제, 줄바꿈 → `\n` 두 글자, 4000자 넘으면 자르고 `…(잘림, 원래 N자)` | P1 | 하 | 상충(옛 명세는 따옴표 유지)이지만 코드·형식 문서·E2E 가 모두 "전부 삭제" 기준이라 그대로 |
| LOG-015 | 같은 수준·분류·메시지가 1초 안에 다시 오면 세기만. 다음 다른 줄/날짜 변경/닫기 때 1회면 원래 줄 한 번 더, 2회 이상이면 `(같은 줄 N 회 더, hh:mm:ss.zzz 까지)` | P1 | 중 | 원본의 알려진 구멍(마지막 접힘은 다음 줄이 밀어냄)을 타이머로 메운다(8.8) |
| LOG-016 | 새 파일을 열 때마다 `snb.app` INFO 첫 줄: 처음 `앱 시작 >>> `, 날짜 바뀌면 `로그 파일을 새로 열었다 >>> ` + 버전·빌드·수준·저장 위치·보관·상한 | P1 | 하 | E2E LF-05 |
| LOG-017 | 켜 둔 채 날짜가 바뀌면 접힌 반복을 내보내고 새 날짜 파일로 | P1 | 하 | |
| LOG-018 | 시작 직후 식별 줄들(시험 인스턴스 WARN, 개발 빌드 WARN, 명령줄 DEBUG, 실행 모드 DEBUG, 시험 진입점 WARN, 마지막 `프로그램이 실행되었습니다.` INFO) | P1 | 하 | Wails 에 맞는 것만 |
| LOG-019 | 모든 스레드를 잠금 하나로 직렬화, 파일 열어 둔 채 줄마다 flush, Windows 는 `FILE_APPEND_DATA` 만 준 핸들(여러 프로세스가 같은 파일에 써도 안 찢어짐), 핸들러 안에서 다시 로그 금지 | P1 | 중 | Go 의 `os.O_APPEND` 가 Windows 에서 바로 이 권한으로 연다(7.3.4) |
| LOG-020 | 치명 오류는 새 파일에 먼저 쓰고 프로세스 종료 | P1 | 하 | Go: `FATAL` 은 쓰고 `os.Exit(1)`; 잡히지 않은 panic 도 FATAL 로 남긴다(8.9, 원본보다 낫다) |
| LOG-021 | 종료 때 핸들러를 풀고 접힌 반복을 쓰고 파일 닫기. 그 뒤는 stderr | P1 | 하 | `ServiceShutdown` 에서 |
| LOG-022 | 종료 경로 기록: `앱 종료 >>> 마지막 창이 닫혔다`, `앱 종료 >>> 종료 코드 N`. 크래시 처리기 없음 | P1 | 하 | Wails 는 panic 을 잡아 남긴다(8.9) |
| LOG-023 | 수준을 `fmlocal.ini [LOG] LEVEL`(`DEBUG`·`INFO`·`WARN`, 공백·대소문자 무시)에서 시작 때 한 번. 없거나 모르는 값이면 DEBUG. INFO 는 DEBUG 끔, WARN 은 DEBUG·INFO 끔 | P1 | 하 | |
| LOG-024 | 환경변수 `QT_LOGGING_RULES` 가 있으면 그것이 이김, 첫 줄에 적음 | P2 | 중 | Qt 고유. Wails 는 같은 이름·같은 문법의 부분집합(`<분류 글롭>.<수준>=true/false;…`)만 해석(7.3.6). E2E 러너가 이 변수를 건다 |
| LOG-025 | 설정 값이 바뀔 때마다 `snb.settings` INFO `설정 변경 >>> <키>: <이전> → <새 값>`. 시작 때 읽은 값은 안 찍음, 키에 pw·pass·token·secret 이면 `***` | P1 | 하 | E2E LF-06 |
| LOG-026 | 정책 수신마다 보관 정리: 일수 = 정책 `remove_logfile` 이 있으면 그것, 없으면 `fmlocal.ini [LOG] KEEP_DAYS`(15). 0 이하 = 일수로 안 지움. 그 뒤 합이 `MAX_MB`(200, 0 = 없음) 넘으면 오래된 날짜부터. 오늘 파일·이름 앞 8자가 날짜 아닌 파일은 안 지움 | P1 | 중 | 결과 줄 문구는 3.7 |
| LOG-027 | 로그인 전에는 아무것도 안 지움. 로그인 뒤 정한 보관 값으로 날짜 바뀔 때도 정리, 지운 게 있으면 `오래된 로그 삭제 >>> ` | P1 | 하 | |
| LOG-029 | 로그를 못 써도 정상 동작(그 줄만 버림) | P1 | 하 | |
| LOG-030 | 폴더는 있는데 파일을 못 열면 프로세스당 한 번 stderr `log: cannot open <경로>` | P2 | 하 | GUI 앱 stderr 는 사용자에게 안 보인다. 그대로 옮기되 8.10 의 개선을 권한다 |
| LOG-032 | 비밀번호는 길이만(`PW[*** len=N]`): 로그인 시도, 기본 비밀번호 비교, 압축 비밀번호 | P1 | 하 | 각 영역이 남기는 줄이지만 마스킹 함수는 여기서 하나로 |
| LOG-033 | 토큰·세션 키·암호화 키 값은 안 남김 | P1 | 하 | |
| LOG-034 | 사용자 파일·클립보드 **내용**은 안 남김(원본은 클립보드 HTML 전문을 DEBUG 로 남김 ― 상충) | P1 | 하 | Wails 는 처음부터 길이만. 07 영역과 맞춘다 |
| LOG-035 | 개발 PC 소스 경로를 안 남김(파일 칸은 이름만). 계정 ID·사용자 파일 경로·이름·크기·실행 파일 경로는 남김 | P1 | 하 | Go 는 `-trimpath` 로 빌드하고 파일 칸도 base name 만 |
| LOG-036 | 남기면 안 되는 것을 지켰는지 자동 확인(E2E `log-format` LF-01~06) | P1 | 하 | 같은 형식이면 원본 하네스의 파서를 그대로 쓸 수 있다. Go 단위 시험도 둔다 |
| LOG-037 | [로그] 메뉴를 결재자·대리결재자·일반 사용자 메뉴 모두에 둠. WASM 은 뺌 | P1 | 하 | 메뉴 배치는 12 |
| LOG-038 | [로그] 조회 조건: 날짜 하루, 수준 콤보(`INFO`·`DEBUG`·`WARN`·`ERROR`, 기본 INFO, **정확히 같은 수준만**), 분류 콤보(`전체 분류`+14개, 접두사 일치), 단어(메시지 칸만, 대소문자 무시) | P1 | 하 | "정확히 같은 수준만" 은 쓰기 불편 ― 8.11 에서 "이 수준 이상" 선택을 덧붙이는 안 |
| LOG-039 | 조회 시점: 화면 생성, 수준·분류 콤보 변경, 단어 Enter, [조회]. 한 박자 미뤄 한 번으로 모으고 안 보이면 보일 때까지 미룸 | P1 | 하 | 날짜를 바꾸는 것만으로는 조회 안 함(원본 그대로 ― [조회] 를 눌러야 함) |
| LOG-040 | 표 7칸 `#`·`시간`(hh:mm:ss.zzz)·`수준`·`계정`·`분류`·`로그`·`파일`. 처음 시간 내림차순, 머리글 클릭으로 오름·내림 번갈아. 결과 없으면 `조회된 데이터가 없습니다.` 행 클릭 강조, 재조회하면 강조 풀림 | P1 | 중 | 큰 결과는 가상 스크롤 필요(8.11) |
| LOG-041 | 그날 파일을 한 번 읽으며 거르고 결과를 한 번에 교체. `snb.ui` DEBUG `로그 조회 >>> N 줄 중 M 줄, X ms` | P1 | 중 | E2E LO-07 이 이 줄을 읽는다 |
| LOG-042 | 그날 파일 없으면 빈 표 + `<경로> 파일이 없습니다.` DEBUG. 새 형식이 아닌 줄은 건너뜀. 날짜는 `yyyy-MM-dd`·`yyyyMMdd` 둘 다 | P1 | 하 | |
| LOG-043 | WASM 은 메모리 2,000줄 링 버퍼 + 브라우저 콘솔 | 제외 | ― | WASM 전용 |
| LOG-044 | `SNB_FREEZE_WATCH=1` 일 때만 GUI 멈춤 감시: 켜면 `GUI 멈춤 감시를 켰습니다(500ms 넘게 멈추면 로그에 남깁니다).` INFO, 200ms 시계가 500ms 넘게 늦으면 `snb.app` WARN `GUI 멈춤 >>> N ms` | P1 | 중 | Wails 는 화면 스레드가 WebView 다 ― JS 쪽에서 재고 Go 로 보낸다(7.7) |
| LOG-045 | 헬퍼 두 개도 같은 줄 형식(분류 `snb.updater`·`snb.versionmaker`, 파일은 실행 폴더 `log\updater_yyyyMMdd.log`) | 제외 | ― | 헬퍼는 15 영역. 그쪽이 Go 로 만들면 7.3 의 포매터 패키지를 그대로 가져다 쓴다 |
| LOG-046 | 줄 형식을 한 곳에서 만들고 되읽음. 형식 문서의 예시 줄을 파서들이 같은 칸으로 읽는지 시험 | P1 | 하 | Go 패키지 `internal/fmlog` 하나 + `docs/로그_형식.md` 예시 5줄을 쓰는 단위 시험 |

### 2.5 미구현·보류 (한 줄씩)

- 미구현: NET-060~063·065·066(논리적 다중망: 다중망 켜고 끄기 정책, 서버에서 망 목록·허용 조합 받기, 보내는/받는 망 쌍, 망별 확장자 정책, 반대 망 서버 상태 확인, 망 정보 수신 알림) ― 서버에 전문이 없어 원본도 미룸. Wails 도 하지 않는다.
- 미구현: LOG-028(정책 `log_folder_create` 로 폴더 두기·지우기), LOG-031(쓸 수 없으면 다른 위치에 쓰기).
- 보류: LOG-007(옛 기록기 제거 후 새 파일이 `yyyyMMdd.log` 이름을 가져가기) ― Wails 는 처음부터 옛 기록기가 없으나 이름은 `.v2.log` 로 둔다(7.3.1).

---

## 3. 서버 통신

### 3.1 쓰는 RPC

| 서비스.메서드 | 방향 | 언제 | 비고 |
|---|---|---|---|
| `FileMoverComm.NotifyService/NotifyServerStream` (`Empty` → `stream NotifyMessage`) | 서버 → 앱 | 로그인 응답(`Login`) 성공 직후 | proto `NotifyService.proto:39`. 스트림이라 시간 상한 없음 |
| `FileMoverComm.NotifyService/NotifyOnce` (`NotifyMessage` → `Empty`) | 앱 → 서버 | 결재 승인·반려 성공 응답 뒤(06 영역이 부름) | 단일 요청이라 15초 상한. 응답은 버린다(`FileMoverGrpc.cpp:2717-2721`, 실패해도 화면 반응 없음) |
| `NotifyService/NotifyStream` (양방향) | ― | 쓰지 않는다 | 옛 문서(`docs/9003_9004_notification_flow.md`)만 이것을 적는다 |
| `LoginService/Login` 의 `cancel_9003` 칸 | 앱 → 서버 | 로그인 | `"1"` 고정(`FileMoverGrpc.cpp:915`). 9003(서버의 중복 로그인 "kill") 알림을 끈다 |

**인증 메타데이터**: 원본은 PreLogin 응답의 토큰을 채널 옵션 메타데이터 `auth-token` 에 넣어 그 채널의 모든 호출(스트림 포함)에 싣는다(`FileMoverGrpc.cpp:723-739`). Wails 는 `Session.ctx()` 가 이미 `auth-token` 을 붙이므로 스트림도 `s.ctx(0)` 으로 연다.

**전문 번호 표기**: 알림은 `notify_svcno` 값(9001…)이 곧 전문 번호다. 다른 영역의 `[3007]` 같은 요청/응답 번호와 달리 응답 번호가 없다.

### 3.2 `NotifyMessage` 칸 (`proto/NotifyService.proto:17-27`)

| 칸 | 형 | 앱이 읽는가 | 뜻 |
|---|---|---|---|
| `notify_svcno` | int32 | 예 | 9001·9002·9004·9005·9007 (9003 은 안 옴) |
| `kind` | int32 | 9004 에서만 | 0 파일목록 갱신, 1 결재목록 갱신, 2 메일목록, 3 메일결재, 4 URL 전송, 5 결재 종료(승인/반려), 6 다중로그인 종료, 7 다중로그인 알림(`RECON_FM`) (`FileMoverGrpc.h:231-239` 의 `E_NOTIFY_TYPE`). 8 이상(UILESS·메시지 전달 등)은 선언만 |
| `side` | int32 | 9004 kind 6·7 | 0 같은 망, 1 반대 망, 2 양쪽 |
| `alim` | int32 | 아니오(로그만) | 0 알림창 없음, 1 알림창 |
| `userid` | string | 9001·9002·9004 | 받을 사용자. 비면 "모두" 로 본다 |
| `msg` | string | 9002(자가승인 판정), 9007(제목) | 9002 에서는 **요청자 ID** 가 실려 온다(추정: `userid`=결재자, `msg`=요청자 ― 같으면 자가승인으로 본다는 코드에서 역산) |
| `pid` | string | 아니오 | 9003 용 |
| `server_no` | string | 아니오 | 9003 용 |
| `ipaddr` | string | 9004 kind 6·7(로그만) | 상대 PC IP |
| `dat` | string | 9002·9004 kind 4·5·9007 | 9002 `process_replace_approval`(대결 처리 표시), kind 4 URL, kind 5 `Y`/`N`, 9007 본문 |
| `__raw__`(999) | bytes | 쓰지 않음 | 서버 내부용 |

**글자 인코딩**: 원본은 내 ID 를 사용자 정보 칸(EUC-KR 바이트)에서 `codec("euckr")->toUnicode` 로 풀어 비교한다(`FileMoverGrpc.cpp:2593-2594`). proto 문자열은 UTF-8 이다. Wails 는 로그인 응답의 `userid`(UTF-8 문자열)로 비교하면 된다 ― 변환이 필요 없다.

### 3.3 순서

```
Login 응답 성공
  └ loggedIn=true, reconnect=0
  └ NotifyServerStream 열기 (goroutine)
       ├ 메시지마다: "Notify 수신: ..." 로그 → dispatch(전문 번호)
       └ 스트림 끝(err):
            ├ 우리가 cancel 한 것(로그아웃) → 조용히 끝
            └ loggedIn 인데 끝남 → 끊김 처리(3.5)
GetProfile → (정책 수신) → 로그 보관 정리(LOG-026) …   ← 스트림과 병렬
로그아웃: loggedIn=false → 스트림 cancel → conn.Close()
```

원본의 `startNotify`(`FileMoverGrpc.cpp:2523-2577`)는 메시지를 받을 때마다 재연결 횟수를 0 으로 되돌리고(WASM 용), 스트림 종료는 `streamFinished` 신호 → `ViewController` 가 `isLogin_` 일 때만 `E_NOTIFY_DISCONNECTED` 로 넘긴다(`ViewController.cpp:546-552`).

### 3.4 전문별 처리 (원본 `receivedNotify`, `FileMoverGrpc.cpp:2579-2699`)

| 전문 | 판정 | 동작 | 화면 |
|---|---|---|---|
| 9001 | `userid != "" && userid != 내ID` → 무시(`Notify Ignored: Different UserID (` … `)` 로그) | 1초 뒤 서버 파일 목록 다시 조회(04) | 목록 갱신만 |
| 9002 | `userid != "" && userid == msg` → `자가승인은 알림 안띄움 >>> UserID (…==…)` 로그 후 무시 | 1초 뒤 결재 목록 다시 조회(06). `dat == "process_replace_approval"` 이면 여기서 끝 | 그 밖이면 결재 요청 알림(아래) |
| 9004 | `userid != "" && userid != 내ID` → 무시 | kind 별(아래 표) | |
| 9005 | 없음 | 무결성 검사 유형 2 시작(14). 로그 `프로그램의 무결성을 확인합니다.`(INFO) | 14 영역 |
| 9007 | 없음(`userid`·`kind`·`alim` 안 봄) | 확인 창 | 제목 = `msg`, 본문 = `dat`(8.5) |
| 그 밖 | ― | 로그(`Notify 수신 …` 줄)만 | 없음 |

9004 의 kind 별:

| kind | 이름 | 동작 |
|---|---|---|
| 0 | `MSG_KIND_FILE_LIST` | 로그 `파일목록 갱신 요청 Notify 수신` → 1초 뒤 서버 파일 목록 + (이어서) 로그 `결재파일목록 갱신 요청 Notify 수신` → 1초 뒤 결재 목록 |
| 1 | `MSG_KIND_FILE_APRV` | 1초 뒤 결재 목록 |
| 2 | `MSG_KIND_MAIL_LIST` | 로그 `메일목록 갱신 요청 Notify 수신` 만 |
| 3 | `MSG_KIND_MAIL_APRV` | 로그 `메일결재목록 갱신 요청 Notify 수신` 만 |
| 4 | `MSG_KIND_URL_SEND` | 로그 `링크 리다이렉트 Notify 수신 >>> URL: <dat>` → 링크연동(08) |
| 5 | `MSG_KIND_END_APRV` | 로그 `결재요청자에게 결재완료(승인/반려) Notify >>> 수신 메시지: <dat>` → 결재 완료 알림 |
| 6·7 | `MSG_KIND_DUP_FM`·`RECON_FM` | 로그 `동일사용자 다중로그인 감지 후 종료 >>> IP: <ipaddr>`. `side == 0` 일 때만 중복 로그인 종료 창 |
| 그 밖 | | 로그 `알 수 없는 Notify kind: <kind>` |

"1초 뒤" 는 서버가 처리를 끝내기를 기다리는 지연이다(원본 주석 "서버 처리 완료 대기"). Wails 는 `time.AfterFunc(time.Second, …)` 로 두되, 그 사이 로그아웃했으면 부르지 않는다(원본은 닫힌 채널로 그대로 나가 `Unavailable` 로 실패한다 ― 6절).

### 3.5 화면에 내는 문구 (원본 `tr()` 의 한국어 원문 그대로)

| 상황 | 창 종류 | 제목 | 본문 | 닫은 뒤 | 근거 |
|---|---|---|---|---|---|
| 9002 결재 요청(트레이 있음) | 트레이 풍선, Information, 5000ms | `결재 요청` | `파일 이동에 대한 승인 요청 건이 있습니다.` | ― | `ViewControllerNotifySlot.cpp:773-786` |
| 9002 결재 요청(트레이 없음) | 알림 띠, 정보(0) | ― | `파일 이동에 대한 승인 요청 건이 있습니다.` | ― | 같은 곳 |
| 9004 kind 5 결재 완료(트레이 있음) | 트레이 풍선, Information, 5000ms | `결재 완료` | `결재 완료\n승인 요청하신 파일의 결재가 완료되었습니다.\n요청 결과 : %1` (`%1` = `승인` 또는 `반려`) | ― | `ViewControllerNotifySlot.cpp:1193-1208` |
| 9004 kind 5(트레이 없음) | 확인 창 | `결재 완료` | 위와 같음 | ― | 같은 곳 |
| 9004 kind 6·7, side 0 | 확인 창 | `중복 로그인` | `다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다.` | **앱 종료** | `ViewControllerNotifySlot.cpp:750-758` |
| 9007 전체공지 | 확인 창 | `msg` | `dat` | ― | `ViewControllerNotifySlot.cpp:886-890` |
| 알림 스트림 끊김 | 끊김 전용 확인 창 | `연결 끊김` | `서버와의 통신이 끊어졌습니다.\n다시 로그인 해주세요.` | 로그아웃 시 동작(4.4) | `ViewControllerNotifySlot.cpp:457-459`, `HomePage.qml:366-371` |
| 요청 실패(`<번호>\<문구>`) | 확인 창 | `요청 처리 오류` | 첫 `\` 뒤 둘째 칸 | ― | `ViewControllerNotifySlot.cpp:825-842` |
| 요청 실패(그 밖) | 확인 창 | `통신 오류 발생` | `서버와의 통신 중에 오류가 발생하였습니다.` | ― | 같은 곳 |
| 서버 채널 생성 실패 | 확인 창 | `오류` | `서버 접속에 실패했습니다. - %1` (`%1` = 1부터 센 순번) | 다음 안내 | `ViewController.cpp:568` |
| 서버 정보 없음 | 확인 창 | `오류` | `서버정보가 없어 로그인에 실패했습니다.` | 앱 종료 | `ViewController.cpp:574-578` |
| (참고) 로그인 때 다른 PC 사용 중 | 확인 창 | `중복 로그인` | `해당 계정은 이미 다른 PC에서 로그인 중입니다.\n프로그램을 종료합니다.` | 앱 종료 | 01 영역, `ViewControllerNotifySlot.cpp:483-487` |
| (참고) 세션 만료 | 전용 확인 창 | `자동 로그아웃` | `%1분 동안 사용하지 않아 자동으로 로그아웃했습니다.` | 늘 로그인 화면(로그아웃 시 동작 무시) | 01 영역, `ViewControllerNotifySlot.cpp:760-771` |
| 확인 창 제목이 비어 오면 | | `정보` | | | `ViewController.cpp:146` |

`popMessageBox` 의 기본 제목은 `정보` 다(제목 문자열이 null 일 때). 9007 의 `msg` 가 빈 문자열이면 원본은 빈 제목을 그린다(null 이 아니라서). **추정:** Wails 는 빈 제목이면 `정보` 로 채우는 편이 낫다.

### 3.6 로그아웃 뒤 버리는 알림 (NET-027)

원본 `processNotify` 맨 앞(`ViewControllerNotifySlot.cpp:339-352`): 아래 12종이 왔을 때 그 서버의 통신 객체가 로그인 상태가 아니면 `로그아웃 뒤 도착한 알림을 버립니다 >>> 종류: <번호> 내용: <앞 80자>`(DEBUG, `snb.auth`) 를 남기고 버린다.

`TRANSFER_RESULT`, `CHECK_DOWNLOAD_ZIP`, `APRV_FILE_EXIST`, `MESSAGE_BY_SERVER`, `LINKURL_VIRUS`, `PROXY_APPROVER_CHANGED`, `PROXY_APPROVER_DELETED`, `COMPLETE_APRV`, `GET_DETAIL_DATA`, `ERROR`, `SUCCESS`, `LINK_REDIRECT`.

로그인 절차의 알림(로그인 실패·완료·정책 수신·패치·무결성·비밀번호 변경)과 클립보드 받기는 넣지 않는다(클립보드 받기는 버리면 복호화된 임시 파일이 남는다). **Wails 구현 규칙**: 화면으로 이벤트를 보내는 함수 하나(`emitSession(kind, payload)`)를 두고 그 안에서 세션의 `loggedIn` 을 본다. 목록 다시 조회(9001·9002 의 1초 지연)도 같은 검사를 하게 해 원본의 구멍을 막는다.

### 3.7 로그에 남기는 줄 (이 영역, 글자 그대로)

| 줄 | 수준·분류 | 근거 |
|---|---|---|
| `Notify Stream 시작!.` | DEBUG snb.grpc | `FileMoverGrpc.cpp:2525` |
| `Notify 수신: 전문번호[%1] kind[%2] side[%3] alim[%4] msg[%5] userid[%6] ipaddr[%7] dat[%8]` | DEBUG snb.grpc | `FileMoverGrpc.cpp:2584-2592` |
| `Notify Stream finished signal. status = <코드> <메시지>` | DEBUG snb.grpc | `FileMoverGrpc.cpp:2547` |
| `gRPC notify 스트림 비정상 종료 감지 (서버: <i> )` | DEBUG snb.auth | `ViewController.cpp:549` |
| `DISCONNECTED SOCKET >>> 로그인 상태에서 연결 끊김. 서버: <i>` | DEBUG snb.auth | `ViewControllerNotifySlot.cpp:455` |
| `Notify 종료` / `ERROR: Notify stream has gone!` | (상태 문자열) / DEBUG | `FileMoverGrpc.cpp:2726-2740` |
| `서버로 Notify 메시지 전송` | DEBUG snb.grpc | `FileMoverGrpc.cpp:2718` |
| `doLogin: 접속 서버 인덱스 = N <이름>` | DEBUG snb.auth | `ViewController.cpp:529` |
| `N 번째 서버는 현재 선택된 서버가 아니므로 로그인하지 않음.` / `N 번째 서버에 접속 시도... <id>` / `서버정보가 없어 로그인에 실패했습니다.`(INFO) | snb.auth | `ViewController.cpp:555-572` |
| `로그 보관 >>> %1일(%2), 상한 %3MB, 지운 파일 %4개%5` (`%2` = `서버 정책` 또는 `fmlocal.ini·기본값`, `%5` = `: 이름, 이름` 또는 빈 값) | INFO snb.app | `ViewController.cpp:373-378` |
| `설정 변경 >>> %1: %2 → %3` | INFO snb.settings | `ViewController.cpp:3112-3115` |
| `로그 조회 >>> %1 줄 중 %2 줄, %3 ms` | DEBUG snb.ui | `LogModel.cpp:158` |
| `<경로> 파일이 없습니다.` | DEBUG snb.ui | `LogModel.cpp:147` |
| `GUI 멈춤 감시를 켰습니다(500ms 넘게 멈추면 로그에 남깁니다).` / `GUI 멈춤 >>> %1 ms` | INFO / WARN snb.app | `main.cpp:902-910` |
| `앱 종료 >>> 마지막 창이 닫혔다` / `앱 종료 >>> 종료 코드 N` / `앱 종료 >>> programExit` / `프로그램이 실행되었습니다.` | INFO snb.app | `main.cpp:926-934`, `ViewController.cpp:609` |
| `로그 폴더를 지우지 못했다(다른 프로세스가 쓰는 중일 수 있다) ― 로그가 계속 남는다: <경로>` | WARN snb.app | `ViewController.cpp:3073` |

---

## 4. 정책 키·설정 키

### 4.1 `fmenv.ini` (실행 파일 폴더, 무결성 검사 대상, 읽기 전용)

| 구획·키 | 값 | 기본 | 비고 |
|---|---|---|---|
| `[SERVERnn] IP` | IPv4 하나, 또는 `_` 로 여럿(`a_b`). 앞뒤 공백 무시 | (빈 값이면 목록 끝) | `nn` 은 01 부터 두 자리. 01 이 비면 서버 0개 |
| `[SERVERnn] PORT` | 정수 | `29001` | 실제 배포 파일은 `29002` 를 적는다(시험 서버 CA 확인도 29002). 기본값은 원본대로 29001 |
| `[SERVERnn] NAME` | 화면에 보이는 망 이름(예: `업무망`) | `NONAME` | 콤보·배지·로그 계정 칸 `ID@NAME` |
| `[SERVERnn] LOC` | `0` 인터넷망(EXTERNAL), `1` 업무망(INTERNAL) | `1` | 비었거나 0·1 이 아니면 경고 + 1 |
| `[SERVERnn] NETWORK` | 전송 방향 문구(예: `업무망->인터넷망`) | `NONAME` | NET-014 |
| `[GRPC] TLS` | `0`/`1` | 1 | 0 이어도 TLS 유지 + 경고 `gRPC TLS 를 끄려는 설정을 무시한다(fmenv.ini [GRPC] TLS=0) >>> 암호화 접속을 유지한다` |
| `[WASM] INNER_/OUTER_ENVOY_ADDRESS` | | | WASM 전용, 제외 |

INI 읽기 주의: 원본은 `QSettings`(IniFormat)로 읽는다 ― 키는 대소문자를 가린다(원본 주석 "SECTION, KEY 는 모두 대문자"). 값의 앞뒤 공백을 지운다. 파일은 UTF-8(BOM 없음)이며 BOM 이 있어도 견딘다(Wails 의 `readServers` 처럼 BOM 을 벗긴다).

### 4.2 `fmlocal.ini` (`fmenv.ini` 와 같은 폴더, PC 마다 다름, 무결성 목록 밖)

| 구획·키 | 값 | 기본 | 쓰임 |
|---|---|---|---|
| `[NETWORK] LOC` | `0`/`1` | 없음(-1) | 이 PC 의 망 → 서버 목록 맨 앞 |
| `[LOG] LEVEL` | `DEBUG`·`INFO`·`WARN`(공백·대소문자 무시) | `DEBUG` | 시작 때 한 번 |
| `[LOG] KEEP_DAYS` | 정수 | `15` | 정책 `remove_logfile` 이 오면 그것이 이김. 0 이하 = 일수로 안 지움 |
| `[LOG] MAX_MB` | 정수 | `200` | 0 = 상한 없음 |

파일·키가 없으면 기본값. 앱 시작 때 읽으므로 고치면 재시작.

### 4.3 서버 정책 (권한 프로파일 `GetProfile` 의 `k=v,k=v`)

| 키 | 뜻 | 없을 때 |
|---|---|---|
| `remove_logfile` | 로그 보관 일수. 0 이하면 일수로 안 지움. 숫자가 아니면 0 으로 읽혀 일수로 안 지움(용량 상한은 그대로) | `KEEP_DAYS` |
| `log_folder_create` | 읽지 않는다(LOG-028 미구현) | ― |

원본은 `policy_.value("remove_logfile")` 이 **있느냐**(QVariant 유효)로 가른다(`ViewController.cpp:365-367`). Wails 의 `Session.Profile` 맵에서 `v, ok := prof["remove_logfile"]` 의 `ok` 로 가르고, `strconv.Atoi` 실패는 0.

알림 처리 자체가 읽는 서버 정책 키(`fm_*`)는 없다. 9002 의 대상이 누구인지는 서버가 정한다.

### 4.4 로그인 응답·세션

| 값 | 쓰임 |
|---|---|
| `LoginResponse.userid` | 내 ID ― 알림의 `userid` 와 비교, 로그 계정 칸 `ID@NAME` |
| 로그인 응답 `MultiLogin` | 01 영역(로그인 때 중복) ― 이 영역의 9004 kind 6 과 별개 |

### 4.5 사용자 설정 (원본은 레지스트리 `HKCU\Software\SQIsoft\snB App`, Wails 는 13 영역의 설정 저장소)

| 키 | 값 | 기본 | 쓰임 |
|---|---|---|---|
| `serverIndex` | 정수(목록 순번) | 0 | 로그인 콤보 기본값(NET-006). 원본은 `fmusr.ini [USER] serverIndex` 에도 같은 값을 둔다(NET-007) ― Wails 는 하나면 된다 |
| `logoutType` | 0 로그아웃(로그인 화면) · 1 프로그램 종료 · 2 트레이로 숨김 | 0 | 끊김 창을 닫은 뒤 동작 |
| `useLogFile` | bool | false | [로그 저장] 체크 표시값. **실제 동작은 로그 폴더 유무로 갈린다** |
| (원본의 `logLevel: 5`, `logStorageDays: 15`) | 아무도 읽지 않는 옛 선언 | ― | 옮기지 않는다 |

### 4.6 환경변수

| 변수 | 뜻 |
|---|---|
| `SNB_FREEZE_WATCH=1` | GUI 멈춤 감시 켜기 |
| `SNB_INSTANCE=<이름>` | 개발 빌드의 인스턴스 이름 → 데이터 폴더 `snB App <이름>`, 프로세스 칸 `이름:PID`(P2) |
| `QT_LOGGING_RULES` | 수준·분류 규칙, `fmlocal.ini` 보다 이김(P2, 부분 해석) |
| `SNB_TEST_HOOKS=1` | 시험 진입점(알림 주입 등). 개발 빌드에서만 |
| `SNB_APP_DIR` | Wails 판이 `fmenv.ini`·`root_ca.pem` 을 읽을 폴더(이미 있음). `fmlocal.ini` 도 같은 폴더에서 읽는다 |

---

## 5. 화면 동작

### 5.1 로그인 화면의 망 콤보 (`content/LoginView.qml:226-234`)

- 콤보 항목 = 서버 목록의 `NAME`(IP 안 보임), 순서 = 4.2 의 정렬 결과. 폭 315, 높이 40.
- `currentIndex` = 저장된 `serverIndex`. 바꾸면 즉시 저장(로그인을 누르지 않아도).
- 로그인 단추 → 고른 순번으로 로그인(NET-007 의 범위 보정).
- 순번은 "망" 이 아니라 "목록 자리" 다: `fmlocal.ini` 를 다른 망 것으로 바꿔 깔면 저장된 순번이 반대 망을 가리킨다. 그래도 콤보 이름과 실제 대상은 같은 순번이라 어긋나지 않는다.
- Wails 개선(선택): 저장값을 순번 대신 `NAME` 으로도 함께 저장하고, 다음 실행에 같은 이름이 있으면 그것을 고른다. 없으면 순번.

### 5.2 세션 구조 (다중 망)

원본: `m_commList[i]` 가 서버마다 하나, 로그인할 때마다 전부 새로 만들고, 고른 하나에만 로그인한다. 대부분의 화면 호출이 `serverIndex` 를 첫 인자로 받는다. [로그] 화면도 망마다 페이지가 생기지만 같은 파일을 보인다(`content/LogView.qml`).

Wails 권장 구조:

```go
type AppService struct {
    mu       sync.Mutex
    sessions []*Session   // 서버 목록과 같은 순번. 로그인하지 않은 자리는 nil
    current  int          // 화면이 보고 있는 망
}
```

- **P1**: 한 번에 하나만 로그인(원본과 같음). 로그인 전에 다른 자리의 세션을 모두 닫는다.
- **P2(두 망 동시 세션)**: 자리마다 독립된 `Session`(채널·토큰·알림 스트림·프로파일)을 두고, 화면은 망 탭으로 전환한다. 이때 정해야 할 것:
  - 로그 계정 칸: 원본은 "그 시점에 로그인된 계정" 하나다. 동시 세션이면 마지막으로 로그인 완료한 세션의 `ID@NAME` 을 두고, 줄마다 서버를 가르려면 메시지에 `서버[N]` 을 붙인다(원본 관례, 11 경계 절). 추정: 계정 칸을 `ID@업무망+인터넷망` 처럼 바꾸면 기존 파서는 읽지만 E2E 판정 문자열과 어긋난다 ― 바꾸지 않는다.
  - 로그 보관 정리: 정책을 받을 때마다 돌고 마지막에 받은 값이 날짜 전환 정리에 쓰인다(원본 그대로).
  - 끊김 창: 세션별로 뜬다. 창 문구에 망 이름을 덧붙인다(예: 제목 `연결 끊김 - 인터넷망`). 한쪽 끊김이 다른 쪽을 로그아웃시키지 않는다.
  - **실 환경 제약(추정)**: 망분리 현장에서는 업무망 PC 가 인터넷망 서버에 닿지 않는다. 시험 환경(같은 PC 에서 두 서버에 닿음)에서만 의미가 있을 수 있다. 서버가 한 PC·한 계정의 두 망 동시 접속을 거부하는지(`CONNECTION_SIDE_ERROR`·중복 로그인 판정)는 확인하지 않았다 ― 9절의 확인 절차로 먼저 본다.

### 5.3 툴바 접속 망 배지 (`content/ShellUserArea.qml:26-48`)

- 로그인 상태에서 늘 지금 접속한 서버의 `NAME`(`ServerModel.currentServer.name`)을 배지로. 사용자 이름 옆.
- 자동화 손잡이가 없다(원본). Wails 는 `data-testid="networkBadge"`·`aria-label` 을 붙인다.

### 5.4 확인 창 (`controls/CustomMessageDialog.qml`, `HomePage.qml:361-374`)

- 원본은 확인 창 셋을 둔다: 일반 `msgBox`(서버 메시지·공지·결재 결과·오류), 끊김 전용 `grpcDisconnectDlg`(확인·닫기 둘 다 로그아웃 처리 호출), 세션 만료 전용 `sessionTimeoutDlg`(알리기만).
- 일반 창은 하나뿐이고 새 요청이 오면 제목·본문을 덮어쓴다. 닫힘 콜백도 하나뿐(`m_messageBoxCallback`), 닫힐 때 한 번 부르고 지운다(`ViewController.cpp:150-170`).
- 단추는 [확인] 하나. 본문에 `objectName: messageDialogText`(E2E 손잡이). 본문·제목은 `Text.PlainText`.
- **"닫으면 종료" 안내의 보호**: 로그인 상태에서 로그인 화면으로 돌아갈 때 `closeSessionPopups()` 가 열린 팝업을 모두 닫고 콜백을 버린다. 단 중복 로그인 종료 안내(9004 kind 6·7)가 떠 있는 동안 연결이 끊겨 이 정리가 돌면, 콜백 대신 `m_exitWhenNoticeClosed` 를 보고 곧바로 종료한다(`ViewController.cpp:2858-2866`).
- Wails 구현: 화면(Svelte) 쪽에 창 큐 하나. Go 는 `Emit(EvDialog, Dialog{Title, Text, OnClose})` 로 보내고 `OnClose` 는 `"exit"`·`"logoutProcess"`·`""` 같은 동작 이름(콜백 함수 대신). 창이 닫히면 화면(`App.svelte` 의 `goDialog`)이 그 동작을 직접 실행한다. 큐에 "exit" 가 있으면 큐를 비우는 정리에서도 지킨다.

### 5.5 알림 띠 (`controls/CustomInfoBar.qml`, `settings/Notify.qml`, `Main.qml:134-141`)

- 앱 전체에 하나. 화면 아래 가운데, 아래 여백 `Theme.space6`, 최대 글 폭 420px(넘으면 줄바꿈).
- `modal:false`, `dim:false`, 바깥 클릭으로 안 닫힘, 모달 대화상자보다 위(`zDialog+1`).
- 4000ms 뒤 스스로 닫힘(`autoHideMs`, 0 이면 닫기 단추로만). 떠 있을 때 새 안내 → 내용·심각도 바꾸고 시간 다시 잼(줄 세우지 않음).
- 심각도: 0 info(accent 색, `info.svg`)·1 success(`success.svg`)·2 warning(`warning.svg`)·3 error(`danger` 색, `error.svg`). 색은 아이콘과 1px 테두리에만, 배경은 늘 `surfaceRaised`.
- 오른쪽에 닫기 단추(아이콘만, 28×28, 툴팁 `닫기`, `objectName: infoBarClose`).
- 나타날 때 opacity 0→1(`durationBase`), 사라질 때 1→0(`durationFast`). 위치 애니메이션 없음.
- 접근성: 그려지는 쪽에 `objectName: infoBar`, `Accessible.role: AlertMessage`, 이름 = 문구. Wails: `role="alert"`·`aria-live="assertive"`·`data-testid="infoBar"`.
- Go → 화면: `Event.Emit("notify", {severity, text})`. 화면 코드끼리는 스토어 함수 `notify.warning(text)` 등.
- 판단 기준(원본 머리말): **사용자가 답할 것이 없으면 알림 띠, 있거나 놓치면 안 되는 실패면 확인 창.**

### 5.6 트레이 풍선

- 원본은 `QSystemTrayIcon::showMessage(제목, 본문, Information, 5000)`. 트레이가 없거나 지원 안 하면(`isSystemTrayAvailable()` 거짓) 대체(9002 → 알림 띠, kind 5 → 확인 창). 원본 주석: Windows 의 방해 금지가 켜져 있으면 안 보인다.
- 트레이 아이콘 생성은 로그인 완료 때(`setTrayIcon`, 12 영역).
- Wails: `github.com/wailsapp/wails/v3/pkg/services/notifications` 의 `NotificationService.SendNotification(NotificationOptions{ID, Title, Body})`(이 PC 의 모듈 캐시 v3.0.0 에 있음을 확인). Windows 는 토스트 알림이라 원본의 "5초" 는 OS 가 정한다. 권한 확인(`CheckNotificationAuthorization`)이 거짓이거나 보내기가 오류면 원본처럼 대체 경로.

### 5.7 연결 끊김 상태 전이

```
[로그인됨] --스트림 종료(우리가 끊지 않음)--> 계정 칸 비움, loggedIn=false
   → "연결 끊김" 창 → (확인 또는 닫기) → logoutType:
        0: 로그인 화면 + 서버 로그아웃 호출(세션 정리)
        1: 프로그램 종료
        2: 창 숨김(트레이로)
[로그인됨] --로그아웃 단추--> loggedIn=false → 스트림 cancel(끊김 창 없음)
```

원본 결함(6절): 끊김 처리에서 `isLogin_` 을 먼저 내려, 창을 닫은 뒤의 로그아웃이 통신 객체 정리(타이머 정지·소켓 닫기)를 건너뛴다. Wails 는 끊김 처리에서 세션 정리(`Session.Close()`·타이머 정지)를 곧바로 한다(창과 무관).

### 5.8 [로그] 화면 (`content/LogScrollView.qml`)

구성(위에서 아래):

1. **조건 줄**(가로, 간격 `space2`): 날짜 선택(달력 아이콘, 기본 오늘) · 수준 콤보(폭 100, `INFO`·`DEBUG`·`WARN`·`ERROR`, 기본 INFO, `objectName: logTypeCombo`) · 분류 콤보(선호 190, 최소 130, 최대 190, `objectName: logCategoryCombo`) · 단어 입력(남는 폭, 최소 100, 자리표시 `로그에서 조회할 단어`) · [조회] 단추(돋보기 아이콘).
2. **표 머리글**: ` # `, `시간`, `수준`, `계정`, `분류`, `로그`, `파일`. 결과가 0행이면 머리글 숨김. 머리글 오른쪽 정렬 아이콘(정렬 중인 칸만, `sort_asc`/`sort_dsc`, 20×20). `#` 칸은 정렬 안 됨. 칸 경계를 끌어 폭 조절.
3. **표**: 칸 폭 `[50, 110, 70, 130, 170, 600, 220]`, 행 높이 30, 줄무늬. `objectName: logTable`, 접근성 이름 `로그 목록`. 행 번호 칸만 접근성에 노출(`rowNumberCell`, 강조 행은 이름 `N 선택됨`).
4. **빈 상태**: 행 0 이면 표 자리에 `조회된 데이터가 없습니다.`

분류 콤보 항목(순서 그대로, 표시 이름 = 값, 첫 항목만 값이 빈 문자열):
`전체 분류`, `snb.app`, `snb.auth`, `snb.transfer`, `snb.transfer.upload`, `snb.transfer.download`, `snb.transfer.clipboard`, `snb.transfer.auto`, `snb.approval`, `snb.check`, `snb.settings`, `snb.grpc`, `snb.ui`, `default`, `qml`.

동작:

- 조회 = 수준(정확히 같음) AND 분류(접두사) AND 단어(메시지에 포함, 대소문자 무시). `snb.transfer` 는 그 아래 넷을 담는다.
- 조회 시점: 화면이 만들어질 때, 수준·분류 바꿀 때, 단어 칸 Enter, [조회]. 날짜 바꾸기만으로는 조회 안 함.
- 여러 호출을 한 박자 뒤 한 번으로 모은다(`Qt.callLater`). 화면이 안 보이면 보일 때까지 미룬다.
- 처음 정렬: 시간 내림차순. 머리글 클릭: 그 칸으로, 오름·내림을 번갈아(칸을 바꿔도 직전 방향의 반대로 시작한다 ― 원본 그대로 `sortOrder` 하나를 뒤집는다).
- 행 클릭 → 강조(`tableHighColor`). 재조회하면 강조 풀림.
- 시간 칸은 `hh:mm:ss.zzz` 만(날짜 없음 ― 하루치라서).
- 단축키: 단어 칸 Enter 뿐(원본에 그 밖의 단축키 없음 ― `LogScrollView.qml` 전체에서 `Shortcut`·`Keys.` 검색 0건).
- Wails 구현: Go 메서드 `SearchLog(date string, level, category, word string) (LogResult, error)` 가 파일을 읽어 거른 행 배열과 `total`·`ms` 를 돌려주고, 정렬은 화면에서(행 수가 많으면 Go 에서 정렬해 페이지로, 8.11).

### 5.9 [설정] 의 [로그 저장] 체크 (`content/SettingsScrollView.qml:255-266`)

- 체크 표시 = 저장값 `useLogFile`. 바꾸면 **먼저** `setLogFileUse(checked)`(켜면 폴더 생성, 끄면 파일 닫고 폴더 삭제) → **그다음** 값 저장. 이 순서여야 켤 때의 `설정 변경 >>> useLogFile: false → true` 줄이 새 파일에 남는다(끌 때의 줄은 폴더가 이미 없어 안 남는다 ― 원본 그대로).
- 체크 이름 `로그 저장`. WASM 에서 숨김.

---

## 6. 경계·오류 상황

원본이 처리하는(또는 처리하지 못하는) 모서리 경우. **[Wails]** 는 이식 때 할 일.

### 다중 망·설정

1. `[SERVER01] IP` 가 비면 서버 0개 → 로그인 때 "서버정보가 없어 로그인에 실패했습니다." 후 종료.
2. `[SERVER02]` 가 비고 `[SERVER03]` 이 있어도 03 은 안 읽힌다(빈 첫 구획에서 멈춤). **[Wails]** 지금 코드는 03 도 읽는다 ― 원본대로 고친다.
3. `IP=_` 처럼 갈라서 남는 것이 없으면 그 구획만 건너뛰고 다음 번호를 계속 읽는다(멈춤 조건은 원문이 빈 것뿐).
4. `LOC` 가 `abc`·`2`·빈 값 → 경고 후 업무망(1). **[Wails]** 빈 값을 인터넷망으로 보는 지금 동작을 고친다.
5. `PORT` 가 숫자가 아니면 원본은 `toInt()` 가 0 → 채널 생성에서 실패(NET-012 경로). **[Wails]** `strconv.Atoi` 실패를 0 으로 보고 같은 실패 문구.
6. `fmlocal.ini` 의 `LOC` 가 없거나 이상하면 `fmenv.ini` 순서(경고는 이상한 값일 때만).
7. 사용자 설정이 접속 키를 덮으려 하면 무시하고 경고 `사용자 설정으로 덮어쓸 수 없는 항목이다(무시한다) >>> <구획/키> - <파일>`.
8. 저장된 순번이 목록 밖(서버 수가 줄었음) → 0 번. 로그아웃 때도 범위 밖이면 접속 망으로 대신한다(원본 E-030 수정, `ViewController.cpp:2833-2836`).
9. IP·포트가 틀렸거나 서버가 죽은 것은 채널 생성에서 드러나지 않고 첫 전문(PreLogin)에서 실패해 로그인 화면에 남는다(01). 닿지 않는 IP 는 Windows TCP 재시도로 최대 21초.
10. NET-012 의 두 안내는 같은 창에 덮어써지고 곧바로 종료라 사용자가 읽을 틈이 없다(원본 확인 필요 항목). **[Wails]** 둘을 한 창에 합쳐 띄우고 닫은 뒤 종료.

### 알림 스트림

11. 스트림에 생존 확인(ping)이 없다 ― 원본은 서버가 조용하면 살아 있다고 보고, Qt 가 스트림을 끝내야 끊김을 안다. **[Wails]** 이미 채널에 keepalive 30초(`client.go:121-123`, 서버 MinTime 10초라 허용)가 있어 반쯤 끊긴 연결도 감지된다.
12. 로그아웃은 로그인 상태를 먼저 내린 뒤 스트림을 끊는다 → 그 종료를 끊김으로 안 본다. 순서를 바꾸면 로그아웃마다 "연결 끊김" 창이 뜬다.
13. 끊김 처리는 로그인 상태가 참일 때만. 이미 거짓이면(로그아웃 중·WASM 인증 만료) 창 없이 지나간다.
14. 끊김 뒤 원본은 통신 객체의 자동 로그아웃 시계·검사 재조회 타이머가 다음 로그인까지 돌 수 있다(확인 필요 1). **[Wails]** 끊김 즉시 세션 정리.
15. [설정] 의 로그아웃 시 동작이 끊김에도 걸린다 ― "프로그램 종료" 면 끊김 창을 닫는 순간 꺼지고 "트레이로" 면 창이 숨는다. 로그인 실패에는 이 적용을 걷어 냈으나(066) 끊김에는 남아 있다. 원본과 같게 둔다.
16. 트레이로 숨어 있을 때 온 공지·끊김 창은 숨은 창 안에 뜬다(언제 보게 되는지 미확인). **[Wails]** 창이 숨어 있으면 트레이 풍선을 함께 띄운다(8.5).
17. 9001·9002·9004 kind 0·1 의 `userid` 가 빈 문자열이면 "모두 대상" 으로 처리.
18. 9002 의 자가승인 판정은 `userid` 가 비어 있지 않을 때만(`userid=="" && msg==""` 는 자가승인 아님).
19. 9002 `dat` 가 `process_replace_approval` 이면 목록은 갱신하되 알림은 안 띄운다(원 결재자 앱에 대결 처리 알림이 뜨지 않게, 027).
20. 9004 kind 6·7 인데 `side != 0` 이면 아무것도 안 한다(로그만).
21. 9004 kind 0 은 결재 목록까지 조회한다(`break` 누락). 의도인지 불명(확인 필요 4) ― 결과가 해롭지 않아 같게 둔다.
22. 9004 kind 7 을 종료로 다루는 것은 MOSFM 과 다르다(상충, 확인 필요 3). snbApp 과 같게 둔다.
23. 9007 제목에 `_` 가 있으면 원본은 제목이 첫 `_` 에서 잘리고 나머지가 본문 앞에 붙는다. 본문에 `_` 가 없어도 안 죽는다(예전엔 죽었다, C-027). **[Wails]** 제목·본문을 따로 넘겨 이 문제 자체를 없앤다.
24. 9007 은 서버 도구 `fm_noti` 가 9000~9006 만 받아 실서버로는 오지 않는다(SR034 미발송). 처리 코드는 시험 진입점으로만 확인됨.
25. 로그아웃 뒤 도착한 세션 알림 12종은 버린다(3.6). 9001·9002 의 1초 뒤 목록 조회는 버리는 목록 밖이라 닫힌 채널로 나가 `Unavailable` 로 실패한다. **[Wails]** 지연 호출에도 `loggedIn` 검사.
26. 확인 창이 열린 채 다른 창 요청이 오면 덮어쓴다 ― 앞 안내(예: 결재 완료)가 사라진다. **[Wails]** 큐.
27. 중복 로그인 종료 안내가 떠 있는 동안 연결이 끊기면 정리가 콜백을 버리지 않고 종료한다(`m_exitWhenNoticeClosed`).
28. 9002·kind 5 알림은 시간대 제한이 없다(밤에도 뜬다).
29. 트레이가 있어도 Windows 방해 금지가 켜져 있으면 풍선이 안 보인다(원본 주석, 대체 경로 없음).
30. 요청 실패 문구는 `<4자리 번호>\<문구>` 꼴에서 둘째 칸만 쓴다 → 칸이 셋인 응답(`3127\1\결재요청이 실패하였습니다.`)은 `1` 만 보인다(06 결함). 번호가 0 이거나 첫 4자가 숫자가 아니면 "통신 오류 발생".
31. `NotifyOnce` 실패(시간 초과 포함)는 화면에 아무 반응이 없다 ― 결재 완료 알림이 요청자에게 안 갈 뿐. **[Wails]** WARN 로그 한 줄.

### 로그

32. 설치 직후에는 로그가 없다 ― `useLogFile` 기본 꺼짐, 설치본이 `log` 폴더를 만들지 않음. 사람이 폴더를 지우면 체크가 켜진 채 로그가 안 남는다.
33. 로그 폴더를 지우면 같은 데이터 폴더를 쓰는 모든 프로세스(업로드 주입 프로세스 포함)의 그날 로그가 함께 사라진다.
34. 끄기(폴더 삭제)는 열린 파일을 먼저 닫아야 한다 ― Windows 는 열린 파일이 든 폴더를 못 지운다. 닫기와 지우기 사이에 다른 스레드가 다시 열지 못하게 같은 잠금 안에서.
35. 반복 억제: 1초 판정은 "원래 줄을 쓴 시각" 기준 ― 같은 줄이 1초 넘게 계속 오면 1초마다 원래 줄이 한 번 더 쓰이고 다시 접힌다. 반복 직후 앱이 죽으면 마지막 요약 줄이 빠진다.
36. 첫 줄의 시각은 파일을 열게 한 줄의 시각(뒤 줄보다 늦게 찍히지 않게).
37. 보관 정리는 정책을 받을 때마다(망마다) 돈다. 정책 값이 숫자가 아니면 0 → 일수로 안 지움, 상한은 적용.
38. 용량 상한이 오늘 파일 하나로 넘쳐도 오늘 파일은 안 지운다. 원본은 옛·새 두 파일을 합산해 실제 상한의 절반쯤에서 지운다(Wails 는 새 파일만이라 해당 없음).
39. 로그인 전에는 아무것도 안 지운다 ― 설정값으로 지우면 정책(예: 30일)이 오기 전에 남아야 할 파일이 사라진다. 날짜 전환 정리도 로그인 뒤 보관 값이 정해진 뒤에만(-1 이면 안 지움).
40. 계정 칸은 "그 시점에 로그인된 계정" 이지 그 줄이 다룬 서버가 아니다.
41. [로그] 화면은 하루치·새 형식 파일만. 망마다 페이지가 생겨도 같은 파일. 화면이 안 보일 때 조회를 미루지 않으면 [로그] 를 누를 때마다 수만 줄을 여러 번 읽어 0.5초 넘게 멈췄다(원본 실측).
42. 수준 필터는 정확히 같은 값만 ― INFO 를 고르면 WARN·ERROR 도 안 보인다. `FATAL` 선택지 없음.
43. 새 형식이 아닌 줄(찢어진 조각·옛 형식)은 건너뛴다. 날짜 인자는 `-` 를 지워 `yyyyMMdd` 로.
44. `QT_LOGGING_RULES` 가 걸린 셸에서 띄우면 `fmlocal.ini` 수준이 무시된다(첫 줄로만 드러남).
45. 핸들러 안에서 로그 함수를 다시 부르면 같은 잠금으로 멈춘다(정리 함수 `fmLogCleanup` 은 그래서 로그를 안 남기고 결과만 돌려준다).
46. 메시지 큰따옴표를 모두 지운다 ― E2E 하네스가 따옴표 없는 값을 바로 잡는다(`대상: <파일명>`).
47. 파일 칸 정보가 없는 줄(Qt 내부 메시지)은 `-:0` 으로 쓰고 버리지 않는다. **[Wails]** Wails 런타임·라이브러리 로그를 받으면 `default` 분류, `-:0`.

---

## 7. 로컬 처리

### 7.1 `fmenv.ini`·`fmlocal.ini` 읽기 (Go)

원본 알고리즘(`FilemoverSettings.cpp:58-123`, `main.cpp:665-674`):

```
servers = []
for n = 1..:
    sec = "SERVER%02d" % n
    ip = trim(ini[sec]["IP"])
    if ip == "": break
    port = trim(ini[sec]["PORT"] or "29001")
    name = trim(ini[sec]["NAME"] or "NONAME")
    network = trim(ini[sec]["NETWORK"] or "NONAME")
    loc = ini[sec]["LOC"]; loc = (trim(loc) in {"0","1"}) ? int : (경고 if 비지 않음; 1)
    ips = [trim(p) for p in ip.split("_") if trim(p) != ""]
    if ips == []: 경고; continue
    servers.append(...)
home = fmlocal[NETWORK][LOC] in {0,1} ? int : -1
ordered = [s for s in servers if s.loc == home] + [s for s in servers if s.loc != home]
```

Go 메모:
- 표준 라이브러리로 충분하다(지금 `readServers` 처럼 직접 파싱). 구획을 `map[string]map[string]string` 으로 먼저 읽은 뒤 위 반복을 돌린다. 외부 패키지 후보 `gopkg.in/ini.v1` 은 필요 없다.
- `QSettings` 와의 차이 주의: `QSettings` 는 값의 `;` 뒤를 주석으로 보지 않고, 쉼표가 있으면 목록으로 읽는다(원본이 IP 구분자로 `_` 를 쓴 이유). Go 파서는 줄 전체에서 첫 `=` 만 가르고 값은 그대로 둔다. 줄 앞 `;`·`#` 는 주석.
- `Server` 구조체에 `IPs []string`, `Port int`, `Loc int`, `Network string` 을 더한다. 기존 `IP string`·`Loc string` 을 쓰는 곳(`client.go:119,157,239,358`)을 함께 고친다.
- 시험: `client_test.go` 에 위 6절 1~6 경우를 표 시험으로.

### 7.2 알림 스트림 (Go)

```go
func (s *Session) startNotify(onMsg func(*notify.NotifyMessage), onEnd func(error)) {
    ctx, cancel := s.ctx(0)
    s.notifyCancel = cancel
    stream, err := notify.NewNotifyServiceClient(s.conn).NotifyServerStream(ctx, &common.Empty{})
    if err != nil { onEnd(err); return }
    go func() {
        for {
            m, err := stream.Recv()
            if err != nil { onEnd(err); return }   // io.EOF 포함
            onMsg(m)
        }
    }()
}
```

- `onEnd` 는 `s.loggedIn.Load()` 가 참일 때만 끊김 처리. 로그아웃은 `loggedIn.Store(false)` → `notifyCancel()` → `conn.Close()` 순서.
- `onMsg` 는 수신 goroutine 에서 불린다 ― 디스패치에서 화면 이벤트(`Event.Emit`)는 goroutine 안전하다(추정: Wails v3 이벤트는 스레드 안전). 목록 다시 조회는 `time.AfterFunc` 로 별 goroutine.
- 같은 kind 의 알림이 짧은 사이 여러 번 오면 1초 지연 조회가 여러 번 나간다(원본 그대로). 8.7 의 묶기를 권한다.
- `pb/` 에 `notify` 패키지가 생성돼 있는지 확인 ― `gen.go` 가 `proto/*.proto` 를 다 만들면 이미 있다. `Empty` 는 `Common.proto`(`commonservice` 패키지가 아니라 공통 메시지 패키지일 수 있음 ― 생성물 이름을 확인).

### 7.3 로그 패키지 `internal/fmlog` (Go)

#### 7.3.1 파일

- 폴더: `<UserConfig 계열>/SQIsoft/<앱 이름>/log`. Windows 는 `os.UserCacheDir()` 가 `%LOCALAPPDATA%` 다(Qt `AppLocalDataLocation` 과 같은 자리). macOS 는 `~/Library/Application Support`(`os.UserConfigDir()`), Linux 는 `$XDG_DATA_HOME` 또는 `~/.local/share`(표준 라이브러리에 없음 ― 직접 또는 이미 간접 의존인 `github.com/adrg/xdg` 의 `xdg.DataHome`).
- 앱 이름: `snB App`(개발 빌드에서 `SNB_INSTANCE` 가 `^[A-Za-z0-9_-]+$` 면 `snB App <이름>`). **원본과 같은 폴더를 쓴다** ― snb-gui [로그] 탭·E2E 하네스·원본 [로그] 화면이 그대로 읽는다. 두 앱을 한 PC 에서 같이 띄우면 한 파일에 섞이고 프로세스 칸으로 갈린다(원본의 업로드 주입 프로세스와 같은 상황).
- 이름: `yyyyMMdd.v2.log`(로컬 날짜). 원본 037 3단계에서 `.log` 로 바뀌면 경로 함수 한 줄만 고친다.
- 폴더가 없으면 쓰지 않는다(만들지 않는다). 줄마다 "열려 있지 않거나 날짜가 다르면 연다" 를 본다 → 폴더가 생기면 다음 줄부터 쓴다.

#### 7.3.2 한 줄 만들기 (`FmLog.h:130-150` 를 그대로)

```
line = time.Format("2006-01-02 15:04:05.000") + " "
     + level5 + " "               // "DEBUG" "INFO " "WARN " "ERROR" "FATAL"
     + token(process) + " "
     + token(account) + " "
     + token(category) + " "      // 빈 값이면 "default"
     + where + " "                // base(file)+":"+line, 파일이 없으면 "-:0" (token 처리)
     + message(msg) + "\n"
token(s): trim 이 비면 "-", 아니면 모든 공백 문자(unicode.IsSpace)를 '_' 로
message(m): '"' 전부 삭제 → '\r' 삭제 → '\n' 을 `\n` 두 글자로 → 4000 글자(코드 포인트가 아니라
            QString 길이 = UTF-16 코드 단위) 넘으면 앞 4000 + "…(잘림, 원래 N자)"
base(f): '/' 와 '\' 중 마지막 뒤
```

주의:
- **4000 의 단위**: 원본은 `QString::size()`(UTF-16 코드 단위). 한글은 1단위라 Go 에서 `[]rune` 길이로 세도 BMP 범위에선 같다. 이모지(서로게이트 쌍)만 다르다 ― 차이를 무시해도 형식 시험은 통과한다(추정). 바이트 길이로 세면 한글이 1/3 로 잘리니 쓰지 않는다.
- 시각은 로컬 시간대.
- 파일:줄은 `runtime.Caller(skip)` 의 `filepath.Base`. Go 빌드는 `-trimpath` 로(경로가 바이너리에 남지 않게, LOG-035).
- 되읽기 정규식(원본 그대로): `^(\d{4}-\d\d-\d\d) (\d\d:\d\d:\d\d\.\d{3}) (\S+)\s+(\S+) (\S+) (\S+) (\S+) ?(.*)$`.
- 형식 시험: 원본 `docs/로그_형식.md` 의 예시 5줄을 Go 시험에 넣어 되읽기 칸이 같은지 본다.

#### 7.3.3 수준·분류 API

- `log/slog` 의 사용자 정의 `Handler` 로 만든다(표준 라이브러리). 분류는 속성 `slog.String("cat","snb.grpc")` 또는 분류별 `*slog.Logger` 변수(`fmlog.Grpc.Debug(...)`). 원본 변수와 1:1: `App, Auth, Upload, Download, Clipboard, Auto, Approval, Check, Settings, Grpc, UI`.
- slog 수준 매핑: Debug→`DEBUG`, Info→`INFO `, Warn→`WARN `, Error→`ERROR`, 사용자 정의 12→`FATAL`.
- 지금 Wails 코드의 `log.Printf` 는 `log.SetOutput` 으로 이 핸들러에 넘겨 `default` 분류·INFO 로 받는다(추정: 원본에서 분류 없는 `qDebug` 는 DEBUG 지만 Go `log` 에는 수준이 없다 ― DEBUG 로 두는 것이 원본에 가깝다).
- 화면(Svelte)의 로그는 바인딩 메서드 `Log(level, message string)` 로 받아 분류 `qml`, 파일:줄 `-:0`(또는 화면 파일명이 오면 그것).
- 메시지는 이미 만든 문자열을 받는다(속성 키=값을 덧붙이지 않는다 ― 형식이 달라진다).

#### 7.3.4 쓰기·동시성

- 전역 `sync.Mutex` 하나로 쓰기·반복 억제·열기·닫기·폴더 삭제를 직렬화(LOG-019, 34).
- 열기: `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)`. **Go 표준 라이브러리는 Windows 에서 `O_APPEND` 일 때 `GENERIC_WRITE` 를 빼고 `FILE_APPEND_DATA` 만 준다**(이 PC 의 Go 1.27.3 `src/syscall/syscall_windows.go:385-393` 확인) ― 원본이 `CreateFileW(FILE_APPEND_DATA)` 로 한 것과 같아 여러 프로세스가 같은 파일에 써도 줄이 겹치지 않는다. 쓸 때 `Seek` 하지 않는다.
- 다만 Go 의 공유 모드는 `FILE_SHARE_READ|FILE_SHARE_WRITE` 뿐(`FILE_SHARE_DELETE` 없음). 원본은 `FILE_SHARE_DELETE` 를 준다. 영향: 다른 프로세스가 이 파일을 연 채로 있으면 폴더 삭제가 실패한다(원본도 WARN 으로 처리). 같게 하려면 `golang.org/x/sys/windows.CreateFile` 로 열고 `os.NewFile` 로 감싼다(이미 간접 의존).
- 줄마다 `f.Write` 한 번(줄 전체를 한 번에) + flush. Go `*os.File` 은 버퍼가 없어 `Write` 가 곧 시스템 호출이다(flush 불필요). **`bufio.Writer` 로 감싸지 않는다** ― 죽기 직전 줄이 사라지고 다른 프로세스 줄과 섞인다.
- 핸들러 안에서 slog 를 다시 부르지 않는다(같은 뮤텍스로 교착). 정리 함수는 지운 이름만 돌려주고, 줄은 잠금 안에서 직접 포매터로 쓴다(원본 `openV2` 와 같게).

#### 7.3.5 반복 억제 (`main.cpp:95-222`)

```
key = level + "|" + category + "|" + msg(원문)
if key == lastKey && now - lastWritten < 1s:
    repeat++; repeatUntil = now; return
flushRepeat()
write(line); lastKey = key; lastRest = line[23:]; lastMsgLen = len(message(msg)); lastWritten = now
flushRepeat():
    if repeat == 1: write(repeatUntil 시각 + lastRest)
    if repeat >= 2: write(repeatUntil 시각 + lastRest 의 메시지 앞까지 + "(같은 줄 N 회 더, hh:mm:ss.zzz 까지)\n")
    repeat = 0
```

- `lastRest` 는 줄에서 앞 23자(`yyyy-MM-dd hh:mm:ss.zzz`)를 뗀 나머지(앞 공백·`\n` 포함). 요약 줄의 머리(수준·프로세스·계정·분류·파일:줄)는 **원래 줄 그대로**다.
- 날짜가 바뀌어 새 파일을 열기 전, 파일을 닫기 전, 폴더를 지우기 전에도 `flushRepeat`.
- **[Wails] 개선**: 접힌 것이 있으면 1초 타이머로 내보낸다(원본의 "다음 줄이 밀어낸다" 구멍, 6절 35). 타이머 콜백도 같은 뮤텍스.

#### 7.3.6 수준 규칙

- 시작 때 `fmlocal.ini [LOG] LEVEL` 을 `strings.ToUpper(strings.TrimSpace())`. `INFO` → DEBUG 끔, `WARN` → DEBUG·INFO 끔, 그 밖 → `DEBUG`(전부).
- `QT_LOGGING_RULES`(P2): `;` 또는 줄바꿈으로 나눈 `<분류패턴>.<수준>=true|false`. 분류패턴은 `*`·`snb.*`·`default`·`qml` 처럼 끝 `*` 글롭만 해석(Qt 는 앞 `*` 도 받는다). 수준은 `debug`·`info`·`warning`·`critical`. 뒤 규칙이 앞 규칙을 이긴다. 이 변수가 있으면 `fmlocal.ini` 규칙보다 나중에 적용(늘 이김). E2E 러너가 거는 값: `snb.*.debug=true;snb.*.info=true;default.debug=true;default.info=true;qml.debug=true;qml.info=true`.
- 첫 줄 수준 칸: 변수가 있으면 `<수준>(QT_LOGGING_RULES=<값> 이 덮음)`.

#### 7.3.7 첫 줄 (`main.cpp:440-450`)

```
"앱 시작 >>> " 또는 "로그 파일을 새로 열었다 >>> "
+ "버전 %1, 빌드 %2, 수준 %3, 저장 %4, 보관 %5일·상한 %6MB(서버 정책이 오면 그것을 따른다)"
  %1 앱 버전  %2 실행 파일 수정 시각 yyyy-MM-dd hh:mm:ss  %3 수준(위)
  %4 로그 폴더(OS 구분자, 끝에 구분자)  %5 KEEP_DAYS  %6 MAX_MB
```
분류 `snb.app`, 수준 INFO, 파일:줄은 Go 의 그 자리(원본은 `main.cpp:<줄>`). 날짜가 바뀌어 다시 열 때 보관 값이 정해져 있으면(로그인 뒤) 곧바로 정리하고, 지운 게 있으면 `오래된 로그 삭제 >>> <이름들, 쉼표 구분>` 한 줄.

#### 7.3.8 보관 정리 (`FmLog.h:209-235`)

```
files = log 폴더의 *.log (파일만, 심볼릭 링크 제외) → (이름, 크기)
total = 모든 파일 크기 합
dated = [f for f in files if parseDate(name[:8], "yyyyMMdd") 유효 && 날짜 < 오늘]
dated 를 (날짜, 이름) 오름차순
for f in dated:
    tooOld = keepDays > 0 && 날짜 + keepDays < 오늘
    tooBig = maxBytes > 0 && total > maxBytes
    if tooOld || tooBig: 지울 목록에 넣고 total -= 크기
```

- 날짜 비교는 날짜 단위(시각 무시). 이름이 날짜가 아닌 파일·오늘 파일은 목록에 넣지 않지만 `total` 에는 센다.
- `*.log` 글롭은 `.v2.log` 도 잡는다(이름 앞 8자 날짜).
- 지우기 실패는 조용히 넘기고 지운 것만 돌려준다.
- 부르는 때: 정책 수신(프로파일 응답) 직후, 그리고 날짜가 바뀌어 새 파일을 열 때(로그인 뒤 값이 있을 때만).
- 순수 함수라 단위 시험(`tst_LogFormat::toDelete` 와 같은 경우)을 Go 표 시험으로.

#### 7.3.9 설정 변경 기록 (LOG-025)

- 원본은 QML 설정 객체의 모든 속성 변경 시그널을 하나에 이어 이전 값과 비교해 찍는다(`ViewController.cpp:3080-3119`).
- Wails: 설정 저장 메서드(13 영역) 한 곳에서 `old != new` 일 때 `설정 변경 >>> <키>: <이전> → <새 값>`(INFO `snb.settings`). 시작 때 읽어 들인 값은 안 찍는다. 키 이름이 정규식 `(?i)pw|pass|token|secret` 에 걸리면 두 값을 `***`.
- 값 문자열: bool 은 `true`/`false`, 숫자는 십진(원본 `QVariant::toString` 과 같음).

#### 7.3.10 비밀값 마스킹 도우미

`fmlog.Pw(s string) string` → `"*** len=" + strconv.Itoa(utf8.RuneCountInString(s))`. 쓰는 곳(각 영역): `LOGIN TRY >>> ID[%s] PW[%s]`, `기본PW[set/empty] PW[*** len=N] 같음[Y/N]`, 압축 비밀번호. 원본은 `QString::length()`(UTF-16)이지만 비밀번호는 ASCII 라 같다(추정).

### 7.4 시험 진입점(알림 주입)

원본: IPC `mode=notify` 로 `{"svcno":9007,"kind":0,"msg":"…","dat":"…","server":N}` 을 받아 `receivedNotify` 에 넣는다. 개발 빌드이면서 `SNB_TEST_HOOKS=1` 일 때만(`ViewController.cpp` `handleIpcMessage`). 아니면 `[시험진입점] notify >>> 개발 빌드가 아니라 무시합니다.`

Wails: 바인딩 `LogService.InjectNotify(svcno, kind, side int32, userid, msg, dat string)`. 원본과 같이 개발 빌드(`config.IsDev`)이면서 `SNB_TEST_HOOKS=1` 일 때만 받고, 아니면 `[시험진입점] notify >>> 개발 빌드의 SNB_TEST_HOOKS 가 없어 무시합니다.` 를 남긴다. 실서버가 9007 을 못 보내므로 이것 없이는 공지 처리를 시험할 길이 없다(9절).

### 7.5 화면 글자 서식

- Svelte 의 `{value}` 는 HTML 이스케이프한다 ― 원본 `Text.PlainText` 와 같은 효과. `{@html}` 를 쓰지 않는 규칙을 린트로 막는다(`svelte/no-at-html-tags`, eslint-plugin-svelte).
- 줄바꿈 `\n` 은 CSS `white-space: pre-line` 으로 보인다(원본 확인 창 본문이 `\n` 을 줄바꿈으로 그린다).

### 7.6 트레이 풍선

7절이 아니라 5.6 에 정리. 외부 의존 없음(Wails v3 내장 서비스). macOS 는 알림 권한 요청이 필요(`RequestNotificationAuthorization`), Linux 는 D-Bus 알림 데몬에 따름(추정).

### 7.7 GUI 멈춤 감시

원본: GUI 스레드(= 렌더 스레드, `QSG_RENDER_LOOP=basic`)의 200ms 타이머가 직전 틱부터 500ms 넘게 지나 불리면 경과 시간(직전 틱부터)을 WARN 으로.

Wails 에서 "화면이 멈춘다" 는 두 곳이다:
1. **WebView 의 JS 메인 스레드**(Svelte 렌더·이벤트 처리). 감시: 화면 시작 때 Go 메서드 `FreezeWatchEnabled()` 가 `SNB_FREEZE_WATCH=="1"` 을 돌려주면, JS 에서 `setInterval(200)` 으로 `performance.now()` 간격을 재어 500 초과면 `Log("WARN", "GUI 멈춤 >>> N ms")` 를 부른다(분류는 원본과 같게 `snb.app` 으로 Go 가 붙인다 ― 판정 케이스가 메시지만 세는지 분류도 보는지 확인). 브라우저 `PerformanceObserver({type:"longtask"})` 를 함께 쓰면 50ms 넘는 작업의 원인 위치까지 남길 수 있다(추정: WebView2 는 지원, macOS WKWebView 는 미지원 가능).
2. **Go 쪽 메인 스레드**(Wails 앱 루프). 바인딩 메서드는 goroutine 에서 돌아 원칙적으로 막지 않는다. 그래도 감시하려면 `application.InvokeSync` 로 메인 스레드에 200ms 마다 빈 작업을 넣어 지연을 잰다(추정, P2).
- 켜면 `GUI 멈춤 감시를 켰습니다(500ms 넘게 멈추면 로그에 남깁니다).` INFO 한 줄(Go 쪽에서). 기본 꺼짐. 빌드 옵션이 아니라 환경변수인 이유(원본): 설치본은 검증한 바이너리를 그대로 재포장하므로.

### 7.8 외부 Go 패키지 후보

| 용도 | 후보 | 판단 |
|---|---|---|
| INI 읽기 | 표준 라이브러리 직접 파싱 | 지금 방식 유지. `gopkg.in/ini.v1` 불필요 |
| 로그 핸들러 | `log/slog`(표준) | 사용자 정의 Handler 하나 |
| 로그 파일 회전 | `lumberjack` 등 | **쓰지 않는다** ― 날짜별 이름·보관 규칙이 원본 고유라 직접 30줄 |
| Linux 데이터 폴더 | `github.com/adrg/xdg`(이미 간접 의존) | Linux 만 |
| Windows 공유 삭제 모드 | `golang.org/x/sys/windows`(이미 간접 의존) | 선택 |
| 트레이 풍선 | `github.com/wailsapp/wails/v3/pkg/services/notifications` | Wails 내장 |
| 트레이 아이콘 | Wails v3 `application.SystemTray` | 12 영역 |
| gRPC 스트림 | `google.golang.org/grpc`(이미) | |
| 화면 큰 표 | 가상 스크롤: `@tanstack/svelte-virtual` 또는 직접 구현 | 8.11 |

---

## 8. 원본보다 낫게 할 수 있는 곳

### 8.1 두 망 동시 세션(P2)
세션 배열 구조를 처음부터 두면 원본이 "로그아웃 → 다른 망 로그인" 으로만 하던 반입·반출 왕복을 앱 하나에서 볼 수 있다. 서버·현장 제약은 5.2 ― 먼저 시험 서버에서 동시 접속이 거부되는지 본다.

### 8.2 데스크톱 알림 스트림 자동 재연결(P2)
원본 데스크톱은 스트림이 한 번 끊기면 곧바로 "다시 로그인" 창이다(서버 재시작·망 순단에도). 제안: 끊김을 바로 알리지 말고 WASM 규칙(최대 5번, 2·4·8·16·32초, 메시지를 받으면 횟수 0)으로 같은 채널에 스트림만 다시 연다. 상태 코드가 `Unauthenticated` 이거나 메시지에 `token`·`expired` 가 있으면 즉시 중단 → 끊김 창. 재연결 중에는 알림 띠(경고)로 `서버와의 연결을 다시 시도하고 있습니다.`(새 문구 ― 원본에 없음). 5번 실패 뒤 원본 끊김 창. **주의**: 서버는 TCP 연결이 끊길 때만 세션을 회수한다(`imports/FileMoverGrpc/README.md`) ― 채널이 살아 있으면 세션도 살아 있어 재구독만으로 된다(추정). 끊긴 사이의 알림은 잃는다 ― 재연결 직후 서버 파일 목록·결재 목록을 한 번 다시 조회해 메운다.

### 8.3 IP 여러 개 대체 접속
원본은 채널이 지연 연결이라 둘째 IP 로 넘어가지 못한다(NET-010 부분). Wails 는 PreLogin 이 `Unavailable` 이면 다음 IP 로 채널을 새로 만들어 재시도. 첫 IP 의 TCP 연결 상한은 짧게(예: `grpc.WithConnectParams{MinConnectTimeout: 5s}` ― 원본은 Windows 재시도로 21초까지 갔다).

### 8.4 종료 전 안내를 읽게 하기
NET-012 의 두 안내(원본은 즉시 종료로 못 읽음)를 한 창으로 합치고 닫은 뒤 종료.

### 8.5 확인 창 큐·공지 제목
- 큐로 바꿔 앞 안내가 덮여 사라지지 않게(6절 26). 같은 제목·본문이 큐에 이미 있으면 넣지 않는다.
- 9007 은 제목·본문을 따로 넘긴다(6절 23).
- 창이 숨어(트레이) 있을 때 공지·끊김·결재 완료가 오면 트레이 풍선을 함께(6절 16).

### 8.6 공통 오류 문구
원본은 서버 실패를 `<번호>\<문구>` 문자열로 만들어 넘기고 둘째 칸만 읽어 결재요청 실패가 `1` 로 보인다(NET-051). Wails 는 Go 에서 `{code int, reason string}` 구조체로 넘겨 문구를 그대로 보인다.

### 8.7 알림 폭주 묶기
같은 사이에 9001·9004 kind 0 이 여러 번 오면 원본은 1초 뒤 조회를 그만큼 보낸다. Wails 는 목록별 디바운스(1초 안의 요청은 한 번)로 서버 부하·화면 깜빡임을 줄인다. 결과는 원본과 같다(마지막 알림 뒤 1초에 한 번 조회).

### 8.8 로그 반복 억제의 마지막 접힘
1초 타이머로 접힌 줄을 내보낸다(원본의 `ponytail:` 구멍).

### 8.9 크래시 흔적
원본은 크래시 처리기가 없어 로그가 그냥 끊긴다(LOG-022). Wails 는 `main` 과 모든 goroutine 진입점에 `defer fmlog.RecoverFatal()` 을 두어 `FATAL` 줄(패닉 값 + `runtime/debug.Stack()` 을 `\n` 이어 붙인 한 줄, 4000자 상한)을 남기고 종료 코드 2 로 끝낸다. Go 는 다른 goroutine 의 panic 을 main 에서 잡을 수 없으니 진입점마다 필요하다.

### 8.10 로그를 못 쓸 때
원본은 GUI 에서 안 보이는 stderr 에 한 번만 쓴다. Wails 는 [설정] [로그 저장] 이 켜져 있는데 파일을 못 열면 알림 띠(경고) `로그 파일을 열 수 없습니다: <경로>`(새 문구)를 한 번 띄운다.

### 8.11 [로그] 화면 성능·사용성
- 원본 실측: 1.2만~1.4만 줄에서 14~21ms(C++). "16만 줄 1초" 는 미측정. Go 는 `bufio.Scanner`(버퍼를 1MB 로 키운다 ― 4000자 한글 줄이 12KB 라 기본 64KB 로 충분하나 여유) + 미리 컴파일한 정규식 대신 `strings.Fields` 7번 자르기로 빠르게 할 수 있다(되읽기 규칙은 "공백 한 개 이상으로 7번 나누고 나머지가 메시지").
- 화면에 수만 행을 DOM 으로 다 그리면 멈춘다 ― 가상 스크롤 필수. 또는 Go 가 정렬된 결과를 들고 페이지(예: 500행)만 넘긴다.
- 거르기는 Go 에서(파일을 읽으며), 정렬은 Go 에서(결과가 클 때).
- 추가 제안(원본에 없음, P2): 수준 콤보에 "이 수준 이상" 선택, 날짜를 바꾸면 바로 조회, 행 더블클릭으로 메시지 전체 보기(긴 메시지가 칸에서 잘림), 메시지 복사.

### 8.12 명세 '결함 의심'·'상충' 중 바로 피할 것
| 원본 항목 | Wails 에서 |
|---|---|
| NET-024 끊김 뒤 통신 객체 정리 누락(확인 필요 1) | 끊김 즉시 세션 정리 |
| NET-032 중복 로그인 안내를 못 읽음(확인 필요 2, 지금 코드는 고쳐짐) | 닫힘 뒤 종료 + 정리 경로에서도 종료 |
| NET-039 제목 밑줄 잘림 | 제목·본문 분리 |
| NET-041 확인 창 덮어쓰기 | 큐 |
| NET-051 오류 본문 `0`·`1` | 구조체 전달 |
| 6절 25 로그아웃 뒤 1초 지연 조회 | 지연 호출에도 로그인 검사 |
| LOG-006 옛 형식 두 벌 쓰기(디스크 두 배) | 쓰지 않음 |
| LOG-015 마지막 접힘 유실 | 타이머 |
| LOG-022 크래시 무흔적 | panic → FATAL |
| LOG-034 클립보드 HTML 전문 로그 | 길이만 |
| LOG-030 GUI 에서 안 보이는 실패 알림 | 알림 띠 |
| 11 확인 필요 7 `logLevel`·`logStorageDays` 죽은 설정 | 만들지 않음 |
| 11 확인 필요 8 형식 문서 예시 둘째 줄의 분류·파일 불일치 | Go 시험은 예시 줄을 그대로 쓰되(형식 시험 입력일 뿐) 로그인 시도 줄의 실제 분류는 `snb.grpc` |
| NET-002 와 다른 Wails 의 빈 `LOC` 처리 | 고침(7.1) |

---

## 9. 실 서버 검증 방법

시험 서버·계정은 원본 `docs/agents/서버_참조.md` 와 `tools/e2e/README.md` 를 따른다. 비밀번호는 `E:\git\snBv6FM\tools\.secrets\` 의 파일에만 있다(여기 적지 않는다).

- 업무망 서버·인터넷망 서버 IP 는 대상 `fmenv.ini`(`SNB_APP_DIR=E:\git\snBv6FM\build\release` 의 것)에서 읽는다. 인터넷망 서버(예: `.158`)에는 mysql 이 없다 ― DB 확인은 업무망 서버에서.
- 계정 역할: A `user`(요청자, `test<N>_1`), B `approver`(결재자, `test<N>_2`, A 의 결재선에 있어야 함), C `proxyApprover`(대결자, `test<N>_3`). 자리 1 은 `tools\.secrets\login.json`.
- 서버 정책은 E2E 전용 권한 프로파일에만 건다(`tools\srv_settings.py`, SBM 요청). 이 계정으로 다른 시험(autoqa 의 E2E-QA 계정 포함)을 같은 시각에 돌리지 않는다.
- 실행 전 공용 자원: 원본 E2E·autoqa 가 같은 계정·서버 정책을 쓰는 중이 아닌지 `tools\snb.cmd who` 로 본다.

### 9.1 망 목록·콤보·배지 (NET-001~015, 064)
1. Wails 실행 폴더에 시험용 `fmenv.ini` 를 두고 경우를 바꿔 가며 로그 `서버 목록` 줄과 콤보 항목을 본다: (a) `[SERVER02]` 비우고 `[SERVER03]` 두기 → 콤보 1개, (b) `LOC` 비우기 → 업무망으로, (c) `IP=a_b`, (d) `PORT` 없음 → 29001.
2. `fmlocal.ini [NETWORK] LOC=0` 을 두면 콤보 첫 항목이 인터넷망인지.
3. 콤보로 인터넷망을 골라 로그인 → 툴바 배지 = 그 `NAME`, 앱 로그의 접속 IP = 그 구획 IP. 업로드 뒤 서버 DB `fm_approve.file_course=0`(인터넷망에서 올림) ― 원본 RT-03 판정과 같다(`tools\e2e\server_probe.py` 로 업무망 DB 조회).
4. 앱을 다시 띄우면 콤보가 마지막 선택을 가리키는지.

### 9.2 알림 스트림 (NET-020~038)
- **9002 결재 요청 알림**: 앱 둘을 띄운다 ― Wails 판에 B(결재자)로 로그인, 원본 앱(또는 Wails 두 번째 인스턴스, `SNB_INSTANCE`)에서 A 로 사전결재(결재자 B 지정) 업로드. Wails 로그에 `Notify 수신: 전문번호[9002] …` 와 트레이 토스트 `결재 요청`. 서버 정책: A 의 업로드 결재 방식이 사전결재(`fm_influp_aprv`/`fm_otflup_aprv`=1)이고 결재자를 고르는 계정이어야 한다 ― 원본 `73-notify-proxy.ps1` 의 `ServerSettings` 를 그대로 본다.
- **대결 처리 알림 막기**(`dat=process_replace_approval`): B 가 C 를 대리결재자로 지정한 상태에서 A 가 요청 → C 가 처리 → B 의 앱에는 알림 없음(목록만 갱신). 원본 AD-16.
- **9004 kind 5 결재 완료**: Wails 판 A, 원본 앱 B 가 승인 → A 에 `결재 완료 … 요청 결과 : 승인`. 원본 결재 처리가 `NotifyOnce` 로 보내므로 서버가 보내는 것이 아니다(06 영역의 Wails 판이 같은 일을 해야 Wails↔Wails 도 된다).
- **9001**: A 가 반대 망에서 자기에게 반입되는 파일을 올림 → 같은 계정으로 이 망에 로그인한 Wails 판에 9001 → 1초 뒤 목록 갱신. 판정은 로그 줄과 서버 파일 목록 조회 줄.
- **9004 kind 6 중복 로그인 종료**: 서버가 보내는 조건이 확인되지 않았다(원본 확인 필요 2). 시험 진입점 주입으로 화면만 확인(`svcno=9004, kind=6, side=0, userid=<내 ID>`) → "중복 로그인" 창 → 닫으면 종료.
- **9005**: SBM [변조 체크] 로 관리자 요청 → 로그 `프로그램의 무결성을 확인합니다.`(14 영역과 함께). SBM 접속 IP 허용은 `tools\snb.cmd adminip allow`.
- **9007**: 실서버로 불가(SR034). 시험 진입점으로 `msg="공지 제목"`, `dat="<b>본문</b> 둘째_칸"` 주입 → 창 제목·본문이 글자 그대로(태그가 굵게 안 그려짐), 밑줄 없는 본문도 안 죽음.
- **로그아웃 뒤 알림 버림**: 진입점으로 주입하는 순간 로그아웃 → `로그아웃 뒤 도착한 알림을 버립니다` 줄.

### 9.3 연결 끊김 (NET-024)
- 로그인해 둔 채 서버 서비스를 내린다(`tools\snb.cmd srv svc stop` ― **서버에 쓴다, 다른 사람의 시험을 끊는다**. 시험 서버 담당자와 시간을 맞춘다. 내린 뒤 반드시 `svc start`). 또는 Windows 방화벽으로 서버 포트를 잠시 막는다(서버를 건드리지 않아 권장 ― 관리자 권한 필요).
- Wails keepalive(30초) 안에 "연결 끊김" 창. `logoutType` 0·1·2 각각에서 닫은 뒤 동작.
- 원본 E2E 에는 이 항목이 없다 ― Wails 쪽 시험 스크립트로 남긴다.

### 9.4 두 망 동시 세션(P2를 할 때만)
- 같은 계정으로 업무망·인터넷망 서버에 동시에 로그인해 둘 다 성공하는지, 한쪽이 `CONNECTION_SIDE_ERROR` 나 중복 로그인으로 거부되는지 먼저 본다. 서버 로그(`tools\snb.cmd probe log …`)에서 두 세션의 `inner_pid`·`outer_pid` 를 확인.

### 9.5 로그 (LOG-*)
- `%LOCALAPPDATA%\SQIsoft\snB App\log` 를 만들고 Wails 판을 띄워 로그인·업로드·설정 변경 → 그날 `yyyyMMdd.v2.log` 에 대해 원본 하네스의 판정을 그대로 쓸 수 있다: 원본 `tools\e2e\cases\88-log-format.ps1` 의 규칙(LF-01 못 읽는 줄 0, LF-02 파일 칸 경로 구분자 0, LF-03 시험 계정 비밀번호 0, LF-04 소스 경로 0, LF-05 첫 줄, LF-06 설정 변경 줄). 하네스가 Wails 창을 다루지 못하면 줄 규칙만 PowerShell/Go 시험으로 옮긴다.
- 형식 문서 시험: 원본 `uv run tools\tests\test_log_format_doc.py` 는 예시 줄을 세 파서로 읽는다 ― Wails 의 Go 파서도 같은 5줄을 읽는 단위 시험을 둔다.
- 여러 프로세스: Wails 판을 두 번 띄우면서(`SNB_INSTANCE` 없이 같은 폴더) 로그를 동시에 쏟게 해 찢어진 줄이 없는지(LF-01).
- 보관: `log` 폴더에 날짜가 지난 가짜 파일 `20260901.v2.log` 등을 만들고 정책 `remove_logfile` 을 E2E 프로파일에 걸어(`srv_settings.py`, SBM 권한프로필(고급)) 로그인 → `로그 보관 >>> N일(서버 정책), …, 지운 파일 K개` 와 실제 삭제. 정책을 지운 상태에서 `fmlocal.ini [LOG] KEEP_DAYS` 로도.
- 수준: `fmlocal.ini [LOG] LEVEL=INFO` → 첫 줄 `수준 INFO`, DEBUG 0줄.
- [로그] 화면: 원본 E2E `log-view` LO-01~07 과 같은 조작(수준·분류·단어·정렬·강조·조회 시간 줄).
- 멈춤 감시: `SNB_FREEZE_WATCH=1` 로 띄워 켜짐 줄, 화면에서 일부러 긴 작업(개발용)으로 `GUI 멈춤 >>>` 줄.

### 9.6 완료 판정(성공 기준)
- `go test ./...` 통과(INI 읽기 경우들, 알림 디스패치 표 시험 ― 가짜 `NotifyMessage` 로 전문·kind·userid·side 조합, 로그 포매터·되읽기·반복 억제·보관 대상 계산·마스킹).
- 9.2 의 9002·kind 5 왕복이 실서버에서 Wails 로그 `Notify 수신` 줄과 토스트로 확인.
- 9.5 의 LF-01~06 규칙 위반 0.

---

부록: 원본 소스 지도(이 문서가 읽은 곳)

| 원본 | 내용 |
|---|---|
| `imports/FileMoverInfo/FilemoverSettings.cpp:58-123, 255-322` | `fmenv.ini` 읽기, 사용자 설정 덮기 금지, `fmlocal.ini` |
| `src/main.cpp:70-350, 380-470, 660-674, 884-934` | 로그 핸들러·반복 억제·첫 줄·서버 정렬·멈춤 감시·종료 줄 |
| `imports/FileMoverInfo/FmLog.h`, `FmLog.cpp` | 형식·분류·되읽기·보관 계산 |
| `imports/FileMoverGrpc/FileMoverGrpc.cpp:89-125, 388-396, 723-739, 915, 996, 2140-2157, 2509-2740` | 로그아웃 순서, TLS, 토큰 메타데이터, cancel_9003, 스트림 시작, NotifyOnce, 전문 분기 |
| `imports/FileMoverGrpc/FileMoverGrpc.h:180-240` | `E_NOTIFY`, `E_NOTIFY_TYPE` |
| `src/ViewController.cpp:143-170, 352-380, 395-450, 516-618, 2828-2890, 3055-3119` | 확인 창·알림 띠 신호, 보관 정리, 서버 객체, 로그인 분기, 정리, 로그 저장, 설정 변경 기록 |
| `src/ViewControllerNotifySlot.cpp:330-490, 750-860, 886-910, 1140-1210` | 알림 처리 |
| `content/HomePage.qml:222-250, 361-440, 748-757` | 로그아웃 시 동작, 확인 창 셋, 신호 처리 |
| `controls/CustomInfoBar.qml`, `settings/Notify.qml`, `Main.qml:125-141` | 알림 띠 |
| `content/LogScrollView.qml`, `src/model/LogModel.cpp:100-159`, `LogProxyModel.cpp` | [로그] 화면 |
| `content/SettingsScrollView.qml:255-266`, `settings/AppSettings.qml` | [로그 저장], 설정 기본값 |
| `content/LoginView.qml:226-234`, `content/ShellUserArea.qml:41` | 망 콤보, 배지 |
