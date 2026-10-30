# 01 로그인·세션·무결성·보안 ― Wails 판 이식 이해 문서

> 원본: `E:\git\snBv6FM`(Qt 6.9.3 + C++, 앱 이름 snbApp). 이식 대상: `E:\git\snBv6FM.Wails`(Go + Wails v3 + Svelte 5/TS).
> 근거 명세: 원본 `specs/current/01-auth-session.md`(요건 AUTH-001~061), `specs/current/14-integrity-security.md`(요건 SEC-001~034).
> 이 문서는 읽기만 해서 썼다. 원본·서버 저장소는 하나도 고치지 않았다. 암호 알고리즘의 시험 벡터는 서버 `crypt2` 를 스크래치 폴더에
> 복사해 실제로 돌려 맞춘 값이다(7.3 절). 추측은 '추정:' 으로 적었다.

---

## 1. 범위

이 문서가 다루는 것:

- 로그인 화면과 로그인 전문(사전로그인 PreLogin → 로그인 Login → 정책 조회 GetProfile[3007])
- 로그인 실패 문구·종료 규칙, 중복 로그인, 사용 중 다른 PC 로그인 알림(9004)
- 단말 식별(IP·MAC), TLS·서버 인증서 검증
- 세션 유지: 자동 로그아웃, 연결 끊김, 로그아웃, 프로그램 종료
- 비밀번호: 비저장, 로그 가림, 기본 비밀번호·만료 안내·강제 변경, 비밀번호 변경[3002]
- 사용자 정보 팝업(전송 권한 표시)
- 단일 인스턴스(한 PC 에 하나만 실행)
- 프로그램 무결성 검사[3510/3511](로그인 직후·3시간 주기·관리자 요청)
- 메시지 암호화(암호 모드 1~6)의 정확한 알고리즘과 Go 로 옮기는 방법
- 사용자 설정 덮어쓰기 범위(`fmusr.ini`), 사용자 데이터 위치

다루지 않는 것(다른 문서):

- 알림 스트림의 9001·9002·9007 처리, 망 전환 화면 → 09(다중 망·알림)
- 트레이 아이콘·메뉴·툴바 배치 → 12(메인 창)
- [설정] 화면의 로그아웃 방식 저장 → 13(설정)
- 배포본 구성·서버 무결성 등록 절차·자동 업데이트 → 15(빌드·배포)
- 업로드 IPC(`--mode upload`)의 내용 처리 → 03(업로드)
- 결재자 목록 조회 → 06(결재)
- 로그 파일 형식·보관 → 11(로그)

Wails 판의 출발점(지금 `client.go` 에 있는 것):

| 이미 되는 것 | 원본과 다른 점 |
|---|---|
| `fmenv.ini [SERVERxx]` 읽기 | ― |
| TLS 접속, `root_ca.pem` 의 인증서로 만든 신뢰 목록 | 핸드셰이크 실패와 연결 실패를 같은 문구로 낸다 |
| PreLogin → Login → GetProfile 순서, `auth-token` 메타데이터 | 암호 모드가 0 이 아니면 거절한다 |
| 계정 실패 한 문구, 라이선스·망 구분 문구 | 미등록 ID 종료, 31바이트 길이 검사가 없다 |
| 단말 식별 1순위(UDP 로 서버 경로를 구함) | 어댑터 거르기(Up·Running·가상 제외)와 2순위가 없다 |
| 요청 상한 15초, keepalive 30초 | ― |

알림 스트림, 자동 로그아웃, 로그아웃(창 정리), 비밀번호 판단·변경, 무결성 검사, 단일 인스턴스는 아직 없다.

---

## 2. 기능 목록

우선순위: **P1** = 첫 배포 전에 원본과 같게, **P2** = 첫 배포 뒤, **제외** = 옮기지 않음(이유를 적는다).
난이도: 상(새 설계·서버 협조 필요) / 중(여러 곳을 잇는다) / 하(한 곳).
ID 는 원본 명세의 요건 번호다. 원본 상태가 '구현'·'부분'·'상충'인 것을 빠짐없이 넣었다.

### 2.1 로그인 화면과 전문(AUTH)

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| AUTH-001 | 앱을 켜면 로그인 화면부터, 로그인 전에는 메인 화면을 안 보인다 | P1 | 하 | Svelte 의 상태 하나로 화면을 가른다 |
| AUTH-002 | ID·비밀번호·접속 망 콤보·[ID 저장]·[로그인], 고른 망을 `fmusr.ini [USER] serverIndex` 에 저장 | P1 | 하 | 범위 밖 번호 → `fmusr.ini` 값 → 0번 순서 |
| AUTH-003 | 빈 칸이면 오류 표시, `계정정보를 정확히 입력해 주세요.` 로 덮어쓰고 요청 안 보냄. Enter 와 [로그인] 이 같다 | P1 | 하 | |
| AUTH-004 | 비밀번호 즉시 가림, 최대 30자, 형식 검사 없음. ID·비밀번호가 UTF-8 31바이트를 넘으면 `아이디 또는 비밀번호가 너무 깁니다.` | P1 | 하 | 바이트 검사는 Go 에서 한다(`len(s) > 31`) |
| AUTH-005 | 응답까지 칸·단추 잠금, `접속 중..` 의 마지막 점 0.4초 깜빡임, 경고 문구는 투명도로만 숨김 | P1 | 하 | 폼이 흔들리지 않게 자리를 비워 둔다 |
| AUTH-006 | 로그인 요청마다 15초 상한, 넘으면 취소·실패 | P1 | 하 | 이미 있음(`rpcTimeout`) |
| AUTH-007 | PreLogin → Login, 응답 토큰을 `auth-token` 에 싣는다. TLS 를 끈 경우 PreLogin 을 고정 AES 키로 암호화 | P1(TLS 경로) / 고정 키 경로 P2 | 중 | 원본도 TLS 를 끌 수 없어(SEC-015) 고정 키 경로는 도달하지 않는다. 7.2 절 |
| AUTH-008 | Login 에 ID·비밀번호·망 구분·IP·MAC·실제 앱 버전, 고정값 `otp` 8888·`first_login` 337·`cancel_9003` "1"·`accepted_ip` "" | P1 | 하 | 이미 있음 |
| AUTH-009 | IP·MAC 을 같은 어댑터에서: 1순위 서버 경로(0.2초), 2순위 첫 실물 어댑터. 못 고르면 비우고 `단말 식별 실패 >>>` | P1 | 중 | 거르기 조건과 2순위를 더한다(7.5 절). macOS·Linux 도 같은 Go 코드 |
| AUTH-010 | `root_ca.pem` 전부로 검증, 없으면 접속 안 함, 인증서 실패와 연결 실패를 다른 문구로 | P1 | 중 | Go 에서 핸드셰이크 실패를 가려내는 법은 7.6 절 |
| AUTH-011 | 로그인 응답의 사용자 정보·전송 권한 보관, 알림 스트림 열기 | P1 | 중 | 스트림 처리 본체는 09 |
| AUTH-012 | 로그인 완료에서 무결성 검사·트레이·권한 계산·정책·확장자·URL 필터·결재자·부서·도메인 정책을 차례로 건다. 0번 서버 + 업데이트 켜짐이면 업데이트 확인 먼저 | P1(업데이트 확인 제외) | 중 | 원본도 업데이트는 꺼져 있다(SEC-031). 각 조회는 해당 영역 문서 |
| AUTH-013 | 정책[3007] 응답 뒤에 화면 전환: 비밀번호 판단 → `LOGIN_COMPLETE` → 서버 파일 목록 → 로그인 전 들어온 IPC 전송 | P1 | 중 | 정책 실패 때 멈추지 않게 고친다(8장) |
| AUTH-014 | `LOGIN_COMPLETE` 에 결재자·대결자 여부 저장, 로그인 화면 숨김, 툴바에 이름, [파일전송] 화면 | P1 | 하 | |
| AUTH-015 | 정책 키가 없으면 코드 기본값(`fm_upfile_mode` 없음 → 0), 오류로 보지 않음 | P1 | 하 | |
| AUTH-016 | 첫 로그인 ID·비밀번호를 프로세스 안에 보관해 망 전환 때 다시 쓴다 | P2 | 중 | 망 전환은 09. 비밀번호를 쥐는 기간이 늘어난다(AUTH-061 과 부딪힘, 6장) |
| AUTH-017 | 로그인마다 준비: 앱 자료 `temp` 비우고 다시 만들기, 홈은 건드리지 않음, 단축키 재등록, 목록 비우기, 통신 객체 새로, 타이머 재시작 | P1(temp·목록·통신 객체) / 단축키는 P2 | 하 | 단축키는 클립보드 영역 |
| AUTH-019 | 계정 실패는 사유 무관 `로그인에 실패하였습니다.`, 사유는 로그에만. 그 밖은 연결·라이선스·망 구분 문구 | P1 | 하 | 이미 있음. 로그 문구를 원본과 맞춘다 |
| AUTH-020 | 실패 문구를 `오류` 창과 로그인 화면 경고에 같이, 서식 태그를 해석하지 않음 | P1 | 하 | Svelte 는 `{text}` 로 넣으면 HTML 해석이 없다 |
| AUTH-021 | 미등록 ID(`OTHER_ERROR`·`USER_NOT_FOUND`)로 실패하면 창을 닫은 뒤 종료 | P1 | 하 | 종료는 창 닫힘 뒤 |
| AUTH-022 | 등록된 ID 실패·계정 무관 실패는 로그인 화면에 남고 로그아웃 방식(종료·트레이)을 적용 안 함 | P1 | 하 | |
| AUTH-023 | (상충) 서버 IP·포트 없음·CA 못 읽음이면 안내 뒤 곧바로 종료 | P1(바꿔서) | 하 | Wails 판은 로그인 화면에 남기고 원인을 보인다(8장) |
| AUTH-024 | (상충) `fm_login_multi` 꺼짐 + `multi_login_flag` 면 `중복 로그인` 창 → 닫으면 종료, 다른 PC 세션은 둔다 | P1 | 하 | 원본 명세 상충은 이전 명세와의 차이일 뿐, 지금 동작이 기준 |
| AUTH-025 | 중복 로그인을 시도 종류로 가르지 않음(자동 로그인 없음) | P1 | 하 | |
| AUTH-026 | (부분) 사용 중 9004(kind 6·7, side 0)면 `중복 로그인` 창 → 종료 | P1 | 하 | 원본은 2026-10-26 에 창 닫힘 뒤 종료로 고쳤다(`ViewControllerNotifySlot.cpp:750-757`). 명세의 '부분'은 낡았다 |
| AUTH-027 | `fm_logout_auto`(분) 동안 입력이 없으면 자동 로그아웃, 0 이면 끄지 않음 | P1 | 중 | 원본은 10초 단위. Wails 판은 정확한 타이머로(8장) |
| AUTH-028 | 마우스 누름·키·휠·터치와 전송 진행 보고가 시계를 되돌림, 1초에 한 번으로 줄임, 이동·호버는 안 셈 | P1 | 중 | 화면 쪽 이벤트 → Go 메서드 |
| AUTH-029 | 자동 로그아웃은 로그아웃 방식과 무관하게 로그인 화면으로, `자동 로그아웃` 창 `N분 동안 사용하지 않아 자동으로 로그아웃했습니다.` | P1 | 하 | |
| AUTH-031 | 로그인 상태에서 알림 스트림이 끝나면 `연결 끊김` 창, [확인] 은 로그아웃 방식을 따른다 | P1 | 중 | 스트림 오류 판정은 09 와 함께 |
| AUTH-032 | [로그아웃] → `로그아웃 처리 방식 확인` 창(설정 `logoutConfirmPopup`), 고른 방식 실행·저장 | P1 | 하 | 설정 저장 위치는 13 |
| AUTH-033 | 로그아웃 순서: 로그인 상태 내림 → 스트림 종료 → 타이머 정지 → 토큰·키 삭제 → 전송 전부 취소 → TCP 닫기 | P1 | 중 | 서버에 로그아웃 전문이 없다. `conn.Close()` 가 곧 로그아웃 |
| AUTH-034 | 프로그램 종료 때 모든 서버 로그아웃, hosts 도메인 정책 지움 | P1(로그아웃) / hosts 는 P2 | 중 | hosts 는 도메인 정책 영역 |
| AUTH-035 | "트레이로 내리기"면 창만 숨기고 로그인 유지 | P2 | 하 | 트레이(12)가 먼저 있어야 한다. Windows 먼저 |
| AUTH-036 | 로그인 화면으로 돌아갈 때 팝업·선택 창 닫고 앞 세션의 창 닫힘 콜백 버림. 로그인 상태가 아니었으면 안 닫음 | P1 | 중 | Svelte 쪽 "세션 번호"로 늦은 콜백을 버린다 |
| AUTH-037 | 로그아웃 뒤 도착한 세션 알림 11종을 버리고 로그만 남김(클립보드 받기는 버리지 않음) | P1 | 하 | Go 에서 세션 번호로 거른다 |
| AUTH-038 | 사용자 정보 팝업은 로그인 상태가 풀리면 닫음 | P1 | 하 | AUTH-036 에 포함 |
| AUTH-040 | [ID 저장](기본 켬)이면 ID 를 저장해 다음 실행 때 채움, 끄면 빈 값 저장 | P1 | 하 | |
| AUTH-041 | 비밀번호 저장·자동 로그인 없음, 옛 `fmx.dat`·`isAutoLogin` 은 읽지도 지우지도 않음 | P1 | 하 | 하지 않는 것을 지킨다 |
| AUTH-042 | 비밀번호 파일이 없으므로 저장 형태를 따지지 않음 | P1 | 하 | AUTH-041 과 같음 |
| AUTH-043 | 로그에 비밀번호·토큰 값을 남기지 않음(`PW[*** len=N]`, `기본PW[set] ... 같음[N]`) | P1 | 하 | |
| AUTH-044 | 정책 `default_pw` 와 로그인 비밀번호가 같으면 변경 창. 비교는 로그인마다 한 번, 비교하며 비밀번호를 지움 | P1 | 하 | |
| AUTH-045 | `pswd_remain_day` > 0 이면 `패스워드 사용기간이 %1일 이후 만료예정입니다.\n지금 패스워드를 변경하시겠습니까?`(값−1) | P1 | 하 | 원본은 이 물음이 닫힐 때까지 로그인 완료를 멈춘다 |
| AUTH-046 | (부분) 잔여일 < 0 이면 강제 변경 창, [취소] 는 종료, Esc·창 밖으로 안 닫힘 | P1 | 중 | Wails 판은 변경이 끝나기 전 메인 화면을 열지 않는다(8장) |
| AUTH-047 | (상충) 변경 창 검사 순서·문구, 규칙 `영문·숫자·@$!%*#?& 포함 10~16자`, 공백 뺀 값으로 [3002] | P1 | 하 | 규칙이 서버 정책이 아니라 코드 고정인 점은 원본 그대로 둔다 |
| AUTH-048 | (부분) 변경 요청의 계정 칸에 `AppSettings.userId`([ID 저장] 끄면 빈 값) | P1(바꿔서) | 하 | 세션의 실제 ID 를 싣는다(8장) |
| AUTH-049 | (부분) 성공 `비밀번호가 정상적으로 변경되었습니다.`, 거절은 서버 사유 그대로, gRPC 오류는 로그만 | P1(바꿔서) | 하 | gRPC 오류도 화면에 보인다(8장) |
| AUTH-051 | 로그인 상태에서 툴바에 망 배지·이름·[로그아웃], 로그인 전엔 언어 콤보 | P1 | 하 | 배치는 12 |
| AUTH-052 | 이름을 누르면 전송 권한 팝업(방향별 허용·결재 방식, 최대 크기 0=`무제한`, 최대 개수) | P1 | 중 | 원본의 클립보드 방향 뒤바뀜 의심(원본 명세 확인 필요 10)을 같이 검증 |
| AUTH-053 | 사용자 정보 팝업의 [비밀번호 변경] | P1 | 하 | |
| AUTH-058 | 한 PC 에 하나만 실행, 있으면 `snB App 프로그램이 이미 실행중입니다.` 뒤 기존 앱 활성화. `--mode upload/download` 는 경로만 넘김 | P1 | 중 | Wails v3 의 `SingleInstanceOptions` 로(7.8 절) |
| AUTH-059 | 활성화 신호에 보이는 창만 앞으로(트레이 창은 그대로) | P1(바꿔서) | 하 | Wails 판은 숨은 창도 다시 보인다(8장) |
| AUTH-060 | 개발 빌드만 `SNB_INSTANCE` 로 별도 인스턴스 | P2 | 하 | E2E 를 Wails 판에 붙일 때 |
| AUTH-061 | 비밀번호는 AUTH-044 판단까지만 쥐고 지운다. 실패·끊김·로그아웃에도 지운다. 성공하면 화면 칸 비움 | P1 | 중 | Go 문자열은 지울 수 없다 ― `[]byte` 로 쥔다(6장) |

### 2.2 무결성·보안(SEC)

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| SEC-001 | 로그인 완료 알림에서 검사유형 0(시작)으로 검사, 중복 로그인 판정보다 먼저 | P1 | 중 | |
| SEC-002 | 로그인 시도마다 3시간 주기 시계를 다시 켜고 검사유형 1, 앱 끝날 때만 멈춤 | P1 | 하 | `time.Ticker` |
| SEC-003 | 개발 빌드만 `SNB_INTEGRITY_PERIOD_SEC`(1~86400초) | P2 | 하 | 시험할 때 |
| SEC-004 | 로그인 상태가 아니면 보내지 않고 `무결성검사 건너뜀 ― 로그인 상태가 아니다. 검사유형: N` | P1 | 하 | |
| SEC-005 | 알림 9005 면 검사유형 2(수동), 통과하면 아무것도 안 띄움 | P1 | 하 | 알림 수신은 09 |
| SEC-006 | 9003 을 무결성으로 처리 안 함, `cancel_9003="1"` | P1 | 하 | 이미 있음 |
| SEC-007 | 검사 대상 10개를 SHA-256 16진 문자열로 [3510], `type_div` "FM", `version` "6.0.0.1", 이름·크기·`location` | P1 | 상 | 대상 목록을 Wails 배포본으로 바꿔야 하고 서버 등록이 필요하다(6·7.7 절) |
| SEC-008 | 해시 못 낸 파일도 크기 0·`"not found file"` 로 보냄 | P1 | 하 | 원본 코드는 이 항목에도 `location` 을 싣는다(명세의 "넣지 않는다"는 낡았다) |
| SEC-009 | 3511 을 요청 기준으로 판정(`result != 1`, 응답 누락(대소문자 무시), 해시 불가 → 실패) | P1 | 하 | 순수 함수 + 시험 하나 |
| SEC-010 | 응답 없음: 유형 0 은 실패, 유형 1·2 는 `Unavailable`·`Cancelled`·`DeadlineExceeded` 면 판정 생략 | P1 | 하 | gRPC `codes` 로 그대로 |
| SEC-011 | 닫힌 채널로 보낸 요청도 실패 콜백이 반드시 불림 | P1 | 하 | Go gRPC 는 닫힌 `ClientConn` 에 `Canceled`(`grpc: the client connection is closing`)를 돌려준다. 추정: 원본의 `Unavailable` 대신 `Canceled` 로 와도 SEC-010 목록에 있어 같게 판정된다 |
| SEC-012 | 실패면 화면을 로그인 상태로 돌리고 `무결성 검증 실패` 창 → 닫으면 종료, 파일 이름은 안 씀 | P1 | 하 | |
| SEC-013 | 개발 빌드는 `SNB_INTEGRITY_ENFORCE=1` 이 아니면 종료 안 함 | P1 | 하 | 개발 빌드 판정 기준을 Wails 에 맞게 바꾼다(7.7 절) |
| SEC-014 | Windows 에서만 검사, 그 밖은 `무결성검사 무시 : 서버에 Windows 배포본 해시만 등록돼 있음` | P1 | 하 | `runtime.GOOS` |
| SEC-015 | `fmenv.ini [GRPC] TLS` 값과 무관하게 TLS, 0 이면 경고만 | P1 | 하 | 이미 TLS 만 쓴다. 경고 문구만 더한다 |
| SEC-016 | `fmusr.ini` 는 `[USER]` 구획만 덮어씀, 다른 구획은 경고 뒤 무시 | P1 | 하 | |
| SEC-017 | (상충) `fmenv.ini` 도 검사 대상, 요청에는 `fmenv.ini.int`(LOC 1)·`fmenv.ini.ext`(그 밖) | P1 | 하 | 상충은 인증 문서와의 차이(앱이 뜬 뒤 걸림). 동작은 그대로 옮긴다 |
| SEC-018 | TLS 1.2 이상·ALPN h2, `root_ca.pem` 전부를 신뢰 목록으로(운영체제 목록 안 씀), 검증 끄는 설정 없음 | P1 | 하 | `tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}`. grpc-go 는 h2 를 스스로 건다 |
| SEC-019 | 인증서를 하나도 못 읽으면 채널 안 만들고 `서버 인증서를 검증할 수 없습니다.` | P1 | 하 | 지금 Wails 문구를 원본 문구로 |
| SEC-020 | 검증 오류마다 `[gRPC] 서버 인증서 검증 실패: <사유> \| host:<주소>`, 무시하지 않음 | P1 | 하 | `InsecureSkipVerify` 를 쓰지 않는다 |
| SEC-021 | PreLogin 핸드셰이크 실패는 `서버를 신뢰할 수 없어 접속하지 않았습니다. 관리자에게 문의해 주세요.`, 그 밖 `서버에 연결할 수 없습니다. 접속 서버 설정을 확인해 주세요.` | P1 | 중 | 7.6 절 |
| SEC-022 | WASM 은 프록시로 gRPC-Web | 제외 | ― | WASM 전용 |
| SEC-023 | 모든 메시지를 암호 모드에 따라 암·복호화, 실패면 그 메시지를 버림 | P2(모드 1~5) | 상 | TLS 서버는 늘 모드 0 이라 실제로 쓰이지 않는다(7.1 절) |
| SEC-024 | PreLogin 은 초기 모드로 보내고 응답의 `enc_mode`·키 재료 6개로 키를 만든다 | P1(모드 0 확인) / P2(키 생성) | 중 | 모드 0 이 아니면 지금처럼 거절하되 문구를 정리 |
| SEC-025 | (부분) 모드 6(NP-ARIA)이면 응답 `ctx` 를 복사 | 제외 | ― | 벤더 DLL·CGO 필요, Go 서버는 `ctx` 를 보내지 않는다(7.4 절) |
| SEC-026 | 비밀번호를 파일·레지스트리·설정에 쓰지 않음 | P1 | 하 | |
| SEC-027 | 사용자 데이터를 `%LOCALAPPDATA%\SQIsoft\snB App` 에, 옛 설치 폴더 `fmusr.ini` 는 한 번 복사 | P1 | 하 | macOS `~/Library/Application Support/...`, Linux `~/.local/share/...` ― Go `os.UserConfigDir` 가 아니라 원본과 같은 자리로 맞춘다(추정: Qt `AppLocalDataLocation` 과 같은 자리) |
| SEC-028 | 공유 메모리 `MOSFM 6.0` 으로 하나만 실행, 파이프 `FileMoverActivation` | P1 | 중 | AUTH-058 과 같음 |
| SEC-029 | `SNB_INSTANCE` 는 개발 빌드만 | P2 | 하 | AUTH-060 과 같음 |
| SEC-030 | 시험 진입점은 개발 빌드만, `notify` 흉내는 개발 빌드가 아니면 버림 | P2 | 중 | Wails 판 시험 진입점을 만들 때 같은 규칙 |
| SEC-031 | 자동 업데이트 안 함 | P1(안 하는 것을 지킴) | 하 | 업데이트 헬퍼도 만들지 않는다 |

**표에 넣지 않은 원본 요건(미구현·보류, 한 줄로 묶음):** AUTH-018(한 번만 할 준비 정리, 보류)·AUTH-030(접속 확인 주기, 미구현)·AUTH-039(WASM 로그아웃, 보류·WASM)·AUTH-050(서버 정책으로 비밀번호 규칙, 미구현)·AUTH-054~057(AD 로그인·AD 전용·OTP 입력·ID 대소문자, 미구현)·SEC-032(GPKI 인증서, 미구현)·SEC-033(Qt 동봉 라이브러리 취약점, Qt 전용이라 해당 없음)·SEC-034(APT 상태 글자, 보류) ― 모두 Wails 판 첫 배포에서 **제외**한다. AD 로그인은 원본에서 `loginType 1` 을 넘기는 곳이 없어 도달하지 않고(`content/LoginView.qml:54`, `src/ViewController.cpp:2778`), LDAP 주소가 `ldap://192.168.101.64` 로 하드코딩돼 있다. GPKI 는 루트 `CMakeLists.txt:585-589` 에서 주석 처리돼 빌드되지 않는다.

표의 행 수: AUTH 53행 + SEC 31행 = **84행**.

---

## 3. 서버 통신

### 3.1 전문 순서

```
[로그인 단추]
  └─ (Go) ID·PW 길이 검사(31바이트)
  └─ LoginService.PreLogin {userid, msg:"[START]"}                       15초
        응답: enc_mode, token, id, en_* 6개, ctx, frm_size, fm_ver
  └─ (모드 ≠ 0 이면 키 생성 ― P2)  auth-token 메타데이터에 token
  └─ LoginService.Login {…}                                             15초
        응답: 사용자 정보·권한·fm_login_multi·multi_login_flag·pswd_remain_day
  └─ NotifyService.NotifyServerStream(Empty)  ← 로그인 응답 직후 연다(끝까지 유지)
  └─ fm_login_multi==0 && multi_login_flag!=0 → "MultiLogin"
  └─ [로그인 완료]
        ├─ CommonService.CheckIntegrity [3510] (chk_div 0)  ← MultiLogin 판정보다 먼저
        ├─ MultiLogin 이면 `중복 로그인` 창 → 종료 (여기서 멈춤)
        ├─ CommonService.GetProfile [3007]  → 응답이 화면 전환의 계기
        ├─ 확장자 정보·패턴·URL 필터·결재자 목록·부서 계층·(업무망+DOMAIN_POLICY) 도메인 정책
  └─ [정책 응답] 비밀번호 판단 → LOGIN_COMPLETE → 서버 파일 목록 → 미뤄 둔 IPC 전송
```

근거: `imports/FileMoverGrpc/FileMoverGrpc.cpp:517-686`(PreLogin·Login), `:919-1005`(로그인 응답), `src/ViewControllerNotifySlot.cpp:464-531`(로그인 완료), `:915-1057`(정책 응답).

Wails 판은 지금 GetProfile 을 Login 안에서 이어 부른다. 무결성 검사는 그보다 먼저(로그인 응답 직후) 보내도 원본 순서와 어긋나지 않는다 ― 원본도 정책 응답을 기다리지 않고 검사를 보낸다.

### 3.2 PreLogin

요청 `PreLoginRequest`:

| 필드 | 값 |
|---|---|
| `userid` | 입력한 ID |
| `msg` | `"[START]"` |

응답 `PreLoginResponse` 에서 읽는 것:

| 필드 | 쓰는 곳 |
|---|---|
| `enc_mode` | 0 이면 그대로, 1~5 면 키 생성(P2), 6 이면 제외 |
| `token` | 이후 모든 요청의 메타데이터 `auth-token` |
| `id` | 비어 있지 않으면 이후 ID 를 이 값으로 바꾼다(서버가 공백을 뺀 실제 ID) |
| `en_svc_id`·`en_svc_no`·`en_svc_port`·`en_site_abb`·`en_my_ip`·`en_svc_ip` | 키 재료(7.2 절) |
| `ctx` | 모드 6 전용, 744바이트일 때만 복사(제외) |

읽지 않는 것: `frm_size`·`fm_ver`·`ip`(Go 서버는 `"NULL"`)·`session_key`.

서버 쪽 값(읽기만 한 서버 `go/service/login/svc_prelogin.go`): SvcID 22, SvcNo 1, MyIP `"127.0.0.1"`, SvcIP 무작위 40자, SvcPort 22, SiteAbb 환경변수 `SERVER_ABB`. 서버를 TLS 로 띄우면 `FM_ENC_MODE` 와 무관하게 `enc_mode` 를 0 으로 덮는다(`go/cmd/services/fm/fm.go:549`).

실패 문구(원본 `FileMoverGrpc.cpp:582-601`):

| 경우 | 화면 문구 |
|---|---|
| TLS 핸드셰이크 실패(`SslHandshakeFailed`) | `서버를 신뢰할 수 없어 접속하지 않았습니다. 관리자에게 문의해 주세요.` |
| 그 밖의 실패 | `서버에 연결할 수 없습니다. 접속 서버 설정을 확인해 주세요.` |
| `root_ca.pem` 에 인증서가 없음 | `서버 인증서를 검증할 수 없습니다.` |
| ID·PW 가 UTF-8 32바이트 이상 | `아이디 또는 비밀번호가 너무 깁니다.` |

### 3.3 Login

요청 `LoginRequest`(원본 `FileMoverGrpc.cpp:830-868`):

| 필드 | 값 | 비고 |
|---|---|---|
| `user_id` | PreLogin 응답 `id`(있으면) 또는 입력 ID | |
| `passwd` | 입력 비밀번호 | |
| `in_out` | 고른 서버 `LOC` 가 1 이면 `"1"`, 아니면 `"2"` | |
| `user_ip_in`·`user_ip_out` | 단말 IP(둘 다 같은 값) | 원본은 EUC-KR 로 바꾸지만 ASCII 라 같다 |
| `user_mac_in`·`user_mac_out` | 구분자를 뺀 MAC(대문자 12자) | |
| `otp` | 8888 | |
| `first_login` | 337 | |
| `sso` | `""`(AD 면 `"g"` ― 도달 안 함) | |
| `cancel_9003` | `"1"` | 9003 알림을 받지 않는다 |
| `app_version` | 앱 버전 | Go 서버는 읽지도 저장하지도 않는다 |
| `accepted_ip` | `""` | |

나머지 필드(`new_passwd`·`init_time`·`con_date` 등)는 비운다.

응답 `LoginResponse` 에서 읽는 것:

| 필드 | 쓰는 곳 |
|---|---|
| `userid` | 세션 ID(비면 보낸 ID) |
| `user_name`·`user_dept`·`user_dept2`·`user_dept_name`·`user_duty_name`·`user_dept_rank` | 사용자 정보. 부서 코드는 `user_dept2` 가 있으면 그것 |
| `aprv_yn`·`replace_yn` | 결재자·대결자 여부 → 메뉴(12) |
| `fm_logout_auto` | 자동 로그아웃 분 |
| `pswd_remain_day`(문자열) | 만료 안내. 0 = 확인 안 함, 음수 = 기한 지남 |
| `fm_login_multi`·`multi_login_flag` | 중복 로그인 |
| `fm_file_up`·`fm_file_down`·`fm_clip_down`·`fm_clip_up`·`fm_influp_aprv`·`fm_inclup_aprv`·`fm_otflup_aprv`·`fm_otclup_aprv`·`fm_file_speed`·`fm_file_size`·`fm_file_count`·`fm_clip_speed`·`fm_clip_size`·`fm_sync_term`·`fm_virus_check`·`fm_ver` | 전송 권한(원본 `setAuthority`) → 사용자 정보 팝업·전송 영역 |

읽지 않는 것: `pswd_min_size`(서버가 채우지 않는다)·`pswd_upper`(오지만 원본이 안 읽는다)·`otp_key`·`multi_login_ip` 등.

실패 판정(원본 `FileMoverGrpc.cpp:867-914`). gRPC 상태와 메시지 문자열로 가른다:

| 조건 | 화면 문구 | 앱 동작 |
|---|---|---|
| 상태가 `Unauthenticated` 가 아님 | `서버에 연결할 수 없습니다.` | 로그인 화면 |
| 메시지에 `LICENSE_ERROR` | `라이선스가 만료되었거나 사용할 수 없습니다.` | 로그인 화면 |
| 메시지에 `CONNECTION_SIDE_ERROR` | `접속한 서버의 망 구분이 설정과 다릅니다.` | 로그인 화면 |
| 메시지에 `OTHER_ERROR` 또는 `USER_NOT_FOUND` | `로그인에 실패하였습니다.` | **창을 닫으면 종료**(미등록 ID) |
| 그 밖(`PASSWORD_ERROR`·`PASSWORD_ERROR_COUNT_OVER`·`OTP_ERROR`·`IP_ERROR`·`MAC_ERROR`) | `로그인에 실패하였습니다.` | 로그인 화면 |

판정 순서 주의: 원본은 문구를 고른 뒤 미등록 여부를 따로 본다. 서버가 사용자 조회에서 DB 오류를 만나도 `OTHER_ERROR` 를 보내므로 그때는 등록된 ID 도 종료된다(원본 `src/README.md`). 같은 동작을 옮긴다.

로그 문구: `LOGIN TRY >>> ID[..] PW[*** len=N]`, `로그인 실패 상세(화면에는 내지 않는다) >>> <상태> <메시지>`.

### 3.4 GetProfile [3007]/[3008]

- 요청 `ProfileRequest{userid}`, 응답 `ProfileResponse{profile}` ― `'k=v,k=v'` 문자열.
- 쉼표로 나누고 첫 `=` 로 키·값을 가른다(Wails `client.go:193-198` 가 이미 같다).
  추정: 값에 쉼표가 들어가면 원본·Wails 판 모두 잘린다(`default_pw` 에 쉼표가 있으면 비교가 틀린다). 서버 SBM 이 쉼표를 막는지는 확인하지 않았다.
- 실패: 원본은 비밀번호를 지우고 로그 `fm_profile2 목록 조회[3007] 실패!!!` 만 남겨 화면이 `접속 중..` 에 멈춘다. Wails 판은 8장대로 바꾼다.

### 3.5 ChangePassword [3002]

- 요청 `ChangePasswordRequest{userId, curPw, newPw}` → 응답 `AnswerResponse{res, reason}`.
- `res > 0` → `비밀번호가 정상적으로 변경되었습니다.`
- `res <= 0` → `reason` 문자열을 그대로 보인다. 서버 거절 코드: -1~-3 DB 오류, -4 정책 없음, -5 쓴 적 있는 비밀번호.
- gRPC 오류 → 원본은 로그만. 계정 칸이 비면 서버가 `Unauthenticated`(`사용자 정보가 일치하지 않습니다.`)로 거절한다(서버 `go/service/common/svc_changepassword.go:36-41`, 토큰 검사가 켜진 경우).

### 3.6 CheckIntegrity [3510]/[3511]

요청 `IntegrityRequest`:

| 필드 | 값 |
|---|---|
| `list` | 아래 항목 10개 |
| `tot_cnt` | 10(목록 길이) |
| `chk_div` | 0 시작 / 1 주기 / 2 관리자 요청(9005) |

항목 `FileChecksumInfo`:

| 필드 | 값 | 주의 |
|---|---|---|
| `type_div` | `"FM"` | |
| `version` | `"6.0.0.1"` | 앱 버전이 아니다. 서버 `ssbr_version.ini` 배포 폴더 이름(`6.0.0.1/WIN`)과 같아야 한다 |
| `fname` | 경로 없는 파일 이름. `fmenv.ini` 만 `fmenv.ini.int`(LOC 1)·`fmenv.ini.ext`(그 밖) | |
| `fsize` | 바이트 크기(uint64). 해시 실패면 0 | |
| `hash` | **SHA-256 의 소문자 16진 문자열(64자)을 그대로 bytes 로**. 해시 실패면 `"not found file"` | 원시 32바이트가 아니다 |
| `location` | 고른 서버의 `LOC` 값 | proto 주석은 `1: int-fm, 0: ext-fm` |

응답 `IntegrityResponse{list[]{fname, detail, result}}`. `result`: 0 모름, 1 성공, 2 크기 다름, 3 해시 다름, 4 둘 다.

판정(원본 `IntegrityVerdict.cpp:5-37`):

1. `result != 1` 인 응답 항목 → 실패(사유 `detail`)
2. 보냈는데 응답에 없는 이름(대소문자 무시) → 실패(`서버 응답에 없어 대조하지 못했습니다`)
3. 해시를 못 낸 항목 → 실패(서버가 통과시켜도)
4. 응답 자체가 없을 때: 유형 0 은 2번으로 전부 실패, 유형 1·2 는 상태가 `Unavailable`·`Cancelled`·`DeadlineExceeded` 면 판정하지 않고 경고 `무결성검사[3510] 응답 없음 ― 서버에 닿지 못해 이번 검사는 판정하지 않는다(검사유형:N)`

로그: 실패 사유마다 `[3511] 무결성 오류 - <파일> : <사유>`, 통과 `무결성검사 응답[3511] => <무결성 오류없음> 검증건수: N`.

서버 대조 방식(원본 `docs/agents/서버_참조.md`): 서버는 `file_name LIKE '%<이름>'` 으로 **첫 행만** 본다(version·경로 조건 없음). 서버 `[FILEMOVER]` 에 없는 이름은 `file not found in database` 로 실패한다.

### 3.7 알림 스트림 중 이 영역이 다루는 것

`NotifyService.NotifyServerStream(Empty)` 의 `NotifyMessage{notify_svcno, kind, side, userid, msg, ipaddr, …}`:

| 번호 | 조건 | 동작 |
|---|---|---|
| 9004 | `kind` 6(중복 FM) 또는 7(재접속 FM) 이고 `side` 0(같은 망) | `중복 로그인` 창 `다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다.` → 닫으면 종료 |
| 9004 | `kind` 0 | 목록 갱신(09) |
| 9004 | `kind` 1 | 결재 목록 갱신(06·09) |
| 9005 | ― | 무결성 검사유형 2 |
| 9003 | ― | 받지 않는다(`cancel_9003`) |
| 스트림 종료(로그인 상태) | ― | `연결 끊김` 창 `서버와의 통신이 끊어졌습니다.\n다시 로그인 해주세요.` |

`kind` 열거: FILE_LIST 0, FILE_APRV 1, MAIL_LIST 2, MAIL_APRV 3, URL_SEND 4, END_APRV 5, DUP_FM 6, RECON_FM 7. `side`: SAME 0, OTHER 1, BOTH 2.
근거: `imports/FileMoverGrpc/FileMoverGrpc.cpp:2543-2561`.

### 3.8 로그아웃 전문

없다. 연결(TCP)을 닫는 것이 로그아웃이다. 서버는 연결이 끊기면 1분 안에, 정상 로그아웃이면 곧바로 세션을 거둔다(원본 `docs/agents/서버_참조.md` "로그인·세션"). Go 에서는 `grpc.ClientConn.Close()` 가 TCP 를 닫는다.

---

## 4. 정책 키

| 이름 | 어디서 오나 | 쓰는 곳 | 없을 때 |
|---|---|---|---|
| `fm_logout_auto` | 로그인 응답(분) | 자동 로그아웃 | 0 = 끄지 않음 |
| `fm_login_multi` | 로그인 응답 | 중복 로그인 허용 여부 | 0 |
| `multi_login_flag` | 로그인 응답 | 다른 세션 있음 | 0 |
| `pswd_remain_day` | 로그인 응답(문자열) | 만료 안내·강제 변경 | 0 = 확인 안 함 |
| `aprv_yn`·`replace_yn` | 로그인 응답 | 결재자·대결자 메뉴 | false |
| `default_pw` | 정책[3007] | 기본 비밀번호 비교 | 비교 안 함(빈 값끼리 같다고 보지 않는다) |
| `fm_upfile_mode` | 정책[3007] | 업로드 방식(03) | 0 |
| `remove_logfile` | 정책[3007] | 로그 보관(11) | 원본 `fmlocal.ini [LOG] KEEP_DAYS` |
| `autotrans_step`·`print_relay`·`print_relay_aprv_type`·`autodown_path` | 정책[3007] | 원본은 로그에만 남긴다 | ― |
| `fmenv.ini [GRPC] TLS` | 이 PC | 0 이어도 무시하고 경고 | TLS |
| `fmenv.ini [SERVERxx] IP·PORT·LOC·이름` | 이 PC | 접속 대상·망 구분 | 비면 로그인 불가 |
| `fmenv.ini [CONFIG] DOMAIN_POLICY` | 이 PC | 로그인 뒤 도메인 정책 조회 | 끔 |
| `fmusr.ini [USER] serverIndex` | 사용자 자료 폴더 | 마지막에 고른 망 | 0 |
| `fmusr.ini [USER] *` | 사용자 자료 폴더 | `fmenv.ini [USER]` 를 덮어씀 | `fmenv.ini` 값 |
| 앱 설정 `userId`·`saveId`·`logoutConfirmPopup`·`logoutType` | 원본 레지스트리 `HKCU\Software\SQIsoft\snB App` | ID 저장·로그아웃 방식 | `saveId` 켬, `logoutConfirmPopup` 켬, `logoutType` 로그아웃 |
| 서버 `login_ip_check`·`login_mac_check` | 서버 권한 프로필(고급) | 서버가 IP·MAC 쌍을 대조 | 앱은 보내기만 |
| 서버 `pw_error_cnt` 등 잠금 정책 | 서버 | 서버가 잠근다. 앱은 계정 실패 한 문구 | ― |
| 환경변수 `SNB_INTEGRITY_ENFORCE`·`SNB_INTEGRITY_PERIOD_SEC`·`SNB_INSTANCE`·`SNB_TEST_HOOKS` | 이 PC | 개발 빌드 시험 | 설치본은 무시 |

Wails 판 앱 설정 저장 자리는 13(설정)에서 정한다. 추정: 레지스트리 대신 사용자 자료 폴더의 파일 하나가 더 단순하다(macOS·Linux 와 같은 코드).

---

## 5. 화면 동작

### 5.1 로그인 화면 ― `content/LoginView.qml`

- 칸: ID(자리표시 `ID를 입력하세요.`), 비밀번호(`비밀번호를 입력하세요.`, 최대 30자, 마지막 글자도 곧바로 가림 `passwordMaskDelay: 0`), 접속 망 콤보, [ID 저장], [로그인].
- [로그인]·Enter: 빈 칸이면 해당 칸 오류 표시 + 경고 `계정정보를 정확히 입력해 주세요.`(`:36-66`).
- 대기 중: 칸·단추 잠금, 단추 글자 `접속 중..` + 0.4초마다 점 하나 깜빡임(`:259`, `:262-270`).
- 실패: `오류` 창과 경고 문구에 같은 글(`:102-106`). 비밀번호 칸은 그대로 둔다.
- 성공: 비밀번호 칸을 비운다. 다시 보일 때도 비운다(`:18-22`).
- 시작: [ID 저장] 이 켜져 있으면 저장된 ID 를 채운다.
- 열리지 않는 창: `계정정보 입력없이 로그인 하시겠습니까?`(`:71-80`)는 `open()` 이 주석이라 뜨지 않는다 ― 옮기지 않는다.
- 자동화 이름(원본): `loginView`·`loginIdInput`·`loginPasswordInput`·`loginButton`·`loginWarningText`. Wails 판은 같은 이름을 `data-testid` 와 접근성 이름(`aria-label`)으로 둔다(E2E 를 다시 쓸 때 대응표가 된다).

### 5.2 메인 셸 ― `content/HomePage.qml`, `content/ShellUserArea.qml`

- `LOGIN_COMPLETE` 를 받으면 로그인 화면 숨김 → 툴바에 망 배지·이름·[로그아웃] → [파일전송] 화면(`HomePage.qml:837-860`).
- `LOGIN_FAILED` 에는 로그아웃 방식(종료·트레이)을 적용하지 않는다(`:407-411`).
- `GRPC_DISCONNECTED`: `연결 끊김` 창, [확인]·닫기 → 로그아웃 방식대로(`:360-365`, `:402-406`).
- `SESSION_TIMEOUT`: 먼저 로그아웃해 로그인 화면으로 돌린 뒤 `자동 로그아웃` 창(`:367-396`).
- `INTEGRITY_FAILED`: 화면을 로그인 상태로 돌리고(`rootPage.logedOut()`), 그 뒤 `무결성 검증 실패` 창(`:397-401`).
- 로그인 상태가 풀리면 열린 팝업(Dialog·Drawer·Menu)과 파일 선택 창을 닫는다(`:746-755`). 앱 로그 `로그아웃으로 닫은 팝업: N 개`.

### 5.3 로그아웃 처리 방식 창 ― `content/LogoutActionDlg.qml`

- 제목 `로그아웃 처리 방식 확인`, 본문 `로그아웃` / `내용`, 처리 방식 셋: `로그아웃`·`프로그램 종료`·`트레이로 내리기`, 체크 `다음부터 보이지 않기(설정에서 변경 가능)`, [OK].
- 설정 `logoutConfirmPopup` 가 꺼져 있으면 창 없이 저장된 `logoutType` 을 실행한다.

### 5.4 비밀번호 변경 창 ― `content/PasswordChangeDlg.qml`

- 제목 `비밀번호 변경`. 칸: `현재 비밀번호`(최대 30자, 형식 검사 없음), `새 비밀번호`(최대 16자), `비밀번호 확인`(최대 16자). 새·확인 칸 자리표시 `영문, 숫자, 특수문자 포함 10~16글자`.
- 새·확인 칸 형식 `^(?=.*[A-Za-z])(?=.*\d)(?=.*[@$!%*#?&])[A-Za-z\d@$!%*#?&]{10,16}$`(`:126`, `:145`). 칸을 떠날 때 맞지 않으면 `영문, 숫자, 특수문자 포함 10~16글자를 입력해 주세요.`
- [확인] 검사 순서와 빨간 문구(`:178-206`):
  1. 현재 칸 빔 → `현재 비밀번호를 입력해 주세요.`
  2. 새 칸 빔 → `새 비밀번호를 입력해 주세요.`
  3. 새 = 현재 → `새 비밀번호가 현재 비밀번호와 같습니다.`
  4. 확인 칸 빔 → `확인 비밀번호를 입력해 주세요.`
  5. 새 ≠ 확인 → `새 비밀번호와 확인 비밀번호는 같아야 합니다.`
  6. 확인 칸 형식 위반 → `영문, 숫자, 특수문자 포함 10~16글자를 입력해 주세요.`
  7. 통과 → 공백을 뺀 값으로 [3002]
- 결과 창 제목 `정보`, 성공 문구 뒤 [확인] 에 변경 창을 닫는다.
- 강제 모드(`MUST_CHANGE`): 닫힘 정책 `Popup.NoAutoClose`(Esc·창 밖 누름 무시), [취소](`:214`) = 앱 종료.
- 주의: 이 정규식은 앞보기(`(?=`)를 쓴다. Go `regexp`(RE2)는 앞보기를 지원하지 않는다 ― 검사는 Svelte(JS 정규식)에서 하고, Go 에서도 검사하려면 조건 셋(영문·숫자·특수문자 포함)과 허용 글자·길이를 따로 본다.
- 원본에는 같은 창이 두 곳에 있다(`HomePage.qml:338`, `UserProfileInfo.qml:210`). Wails 판은 하나만 둔다.

### 5.5 만료 안내

- 잔여일 > 0: 창 제목 `정보`, 본문 `패스워드 사용기간이 %1일 이후 만료예정입니다.\n지금 패스워드를 변경하시겠습니까?`(값 − 1), [Yes]/[No]. [Yes] → 변경 창(`src/ViewControllerNotifySlot.cpp:938-961`). 원본 데스크톱은 이 창이 `QMessageBox::exec()` 라 닫힐 때까지 로그인 완료 처리가 멈춘다.
- 잔여일 < 0: 강제 모드 변경 창.
- 잔여일 = 0: 아무것도 안 띄운다. 로그 `비밀번호 변경 잔여일 >>> 0 (0:체크안함, 음수:기한초과)`.
- 기본 비밀번호가 같으면(AUTH-044) 잔여일을 보지 않고 변경 창(일반 모드).

### 5.6 사용자 정보 팝업 ― `content/UserProfileInfo.qml`

- 툴바 이름을 누르면 그 아래에 뜬다. 열 때마다 권한을 다시 계산한다.
- `파일`: `<망> 전송`(허용/차단) ×2, `<망> 결재`(결재 방식) ×2, `최대전송크기`(0 = `무제한`), `최대전송개수`.
- `클립보드`: 같은 넷 + `최대전송크기`.
- 방향 이름은 `fmenv.ini` 의 망 이름을 쓴다.
- [비밀번호 변경](`:210-225`).
- 원본 의심점: 권한 계산이 클립보드 업·다운로드를 서로 바꿔 넣고, 팝업이 들어오는 방향 줄에 `clipboardUpload` 를 붙여 두 번 바뀐다(원본 명세 확인 필요 10). Wails 판은 로그인 응답 필드 주석(`fm_clip_down` = 내부→외부, `fm_clip_up` = 외부→내부)을 기준으로 다시 짜고, 원본 화면과 같은 계정으로 대조한다.

### 5.7 그 밖의 창 문구(원본 `src/ViewControllerNotifySlot.cpp`)

| 경우 | 창 제목 | 본문 | 닫으면 |
|---|---|---|---|
| 로그인 실패 | `오류` | 3.3 절 문구 | 미등록 ID 만 종료 |
| 로그인 때 중복 | `중복 로그인` | `해당 계정은 이미 다른 PC에서 로그인 중입니다.\n프로그램을 종료합니다.` | 종료 |
| 사용 중 9004 | `중복 로그인` | `다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다.` | 종료 |
| 자동 로그아웃 | `자동 로그아웃` | `%1분 동안 사용하지 않아 자동으로 로그아웃했습니다.` | 로그인 화면 그대로 |
| 연결 끊김 | `연결 끊김` | `서버와의 통신이 끊어졌습니다.\n다시 로그인 해주세요.` | 로그아웃 방식대로 |
| 무결성 실패 | `무결성 검증 실패` | `프로그램 파일이 손상되었거나 서버에 등록된 것과 다릅니다.\n안전을 위해 프로그램을 종료합니다.` | 종료 |
| 두 번째 실행 | (원본 메시지 상자) | `snB App 프로그램이 이미 실행중입니다.` | 두 번째 프로세스 종료 |
| 비밀번호 변경 성공 | `정보` | `비밀번호가 정상적으로 변경되었습니다.` | 변경 창 닫힘 |

**종료는 언제나 창이 닫힌 뒤에 한다.** 원본은 `popMessageBox` 가 신호만 보내고 돌아오므로 그 자리에서 종료하면 안내가 보이기 전에 꺼졌다(원본에서 두 번 겪었다). Wails 판도 Go 가 화면에 이벤트를 보낸 뒤 화면이 "닫힘"을 Go 메서드로 알려 올 때 종료한다. 창이 떠 있는 동안 연결이 끊겨 로그아웃 정리가 돌아도 종료가 사라지지 않게 "닫으면 종료" 표시를 세션 정리와 따로 둔다(원본 `m_exitWhenNoticeClosed`).

---

## 6. 경계·오류 상황

| 상황 | 원본 동작 | Wails 판에서 할 것 |
|---|---|---|
| 정책[3007] 이 실패하거나 안 옴 | `접속 중..` 에 멈춤, 서버에는 세션이 생김 | 15초 상한 뒤 연결을 닫고 `서버에 연결할 수 없습니다.` 로 로그인 화면 |
| 서버 IP·포트 없음, CA 못 읽음 | 안내 뒤 곧바로 종료(안내가 안 보일 수 있음) | 로그인 화면에 남고 원인 문구 |
| 닿지 않는 IP | 채널은 지연 생성이라 PreLogin 실패 → 연결 실패 문구 | 같음(`grpc.NewClient` 도 지연 연결) |
| 로그인 실패가 연달아 옴 | `오류` 창이 겹침 | 한 번에 한 창, 새 문구로 바꿔 끼움 |
| 로그인 도중 연결 끊김 | 로그인 실패와 같은 갈래 | 같음. 실패 안내 하나만 |
| 로그인 응답 사용자 ID 가 빔 | 데스크톱은 검사 안 함 | 보낸 ID 를 쓴다(지금 Wails 와 같음) |
| 서버가 준 이름·부서가 길다 | 원본은 고정 칸 크기로 잘라 담고 `칸보다 길어 잘랐다` | Go 문자열이라 자르지 않는다. 보낼 때만 31바이트 검사 |
| 서버 문자열 인코딩 | 원본은 EUC-KR 로 잘라 담는다 | Go protobuf 는 `string` 이 UTF-8 이 아니면 해석에서 실패한다. 추정: Go 서버는 UTF-8 로 보낸다(Wails 판 실 서버 로그인이 이미 성공). 한글 이름 계정으로 확인한다 |
| 자동 로그아웃 중 전송 | 진행 보고가 시계를 되돌려 만료 안 됨. 멈춘 전송은 시계가 돌아 만료 → 전송 취소 | 같음 |
| 자동 로그아웃 시계 시작 | 첫 서버 파일 목록 조회 때(사실상 로그인 직후) | 로그인 완료 때 시작 |
| 강제 변경 창 뒤 메인 화면 | 이미 열림, 모달 하나가 막음. 트레이·단축키·IPC 로 닿는지 미확인 | 변경 성공 전에는 `LOGIN_COMPLETE` 를 내지 않는다 |
| 변경 요청 계정 칸 | [ID 저장] 을 끄면 빈 값 → 서버 거절 → 화면 반응 없음 | 세션 ID 를 싣는다 |
| 같은 로그인에서 정책을 다시 받음 | 비밀번호가 지워져 판단 생략, 로그 `비밀번호 판단 생략 >>> 이번 로그인에서 이미 했다` | 같음 |
| 망 전환(AUTH-016) | 앞 망 ID·비밀번호로 새 망 로그인 | 비밀번호를 프로세스에 계속 쥐어야 한다 → AUTH-061 과 부딪힌다. P2 에서 "망 전환 때 비밀번호를 다시 묻기"와 비교해 정한다 |
| 개발 빌드 무결성 | 다른 작업트리의 빌드가 서버 해시를 덮어 내 빌드가 실패 | 같은 이유로 개발 빌드는 종료하지 않는다 |
| 무결성 검사 대상이 없음(파일 이름 다름) | `not found file` 로 실패 | Wails 배포본 이름으로 목록을 바꾸고 서버에 등록해야 한다(아래) |
| 무결성: 서버 `[FILEMOVER]` 에 Wails 파일 이름이 없음 | ― | `file not found in database` 로 **로그인 직후 종료**. 서버 등록 전에는 개발 빌드로만 시험한다 |
| Go 문자열의 비밀번호 | 원본은 0 으로 덮어쓴다(Qt·gRPC 힙 사본은 한계로 받아들임) | Go `string` 은 바꿀 수 없다. 화면 → Go 는 `string` 으로 올 수밖에 없으니(Wails 바인딩), 받자마자 `[]byte` 로 옮겨 쥐고 판단 뒤 `clear()`. protobuf 요청에 담긴 사본·바인딩 JSON 사본은 지울 수 없다 ― 원본과 같은 한계로 적어 둔다 |
| 9004 를 서버가 실제로 보내는가 | 확인 안 함(앱 쪽 `closeOtherClient()` 는 부르는 곳이 없다) | 처리만 옮기고 시험은 `notify` 흉내로 |
| 원본 단일 인스턴스와 공존 | Qt 판과 Wails 판이 같은 PC 에 함께 뜰 수 있다 | 고유 ID 를 다르게 둔다. 같은 계정으로 둘을 로그인하면 서버가 중복 로그인으로 막는다 |

---

## 7. 로컬 처리 알고리즘과 Go 패키지 후보

### 7.1 암호 모드가 실제로 쓰이는가

- 서버는 TLS 로 기동하면 `enc_mode` 를 0 으로 덮는다(`go/cmd/services/fm/fm.go:549`). 1~6 은 서버를 `-tls=false` 로 띄웠을 때만 나온다.
- 원본 앱은 TLS 를 끌 수 없다(SEC-015, `grpcTlsEnabled()` 가 늘 참). 이 PC 의 원본 앱 로그에서 로그인 5,801회가 모두 `암호모드: 0` 이었다(원본 `docs/cert/문서정정요청_Client_App_암호통신_20261028.md`).
- 그래서 Wails 판 첫 배포는 **모드 0 만** 받고(P1), 모드 1~5 코덱은 P2 로 둔다. 다만 인증 문서가 "메시지 암호화"를 내세우므로 코덱을 옮길 수 있게 아래를 정확히 적는다.

### 7.2 메시지 암호화의 틀(모드 1~5 공통)

**어디를 암호화하나.** protobuf 로 직렬화한 메시지 바이트 전체다. gRPC 프레임 머리 5바이트(압축 1 + 길이 4)는 암호문을 감싼다. 빈 메시지(0바이트)는 암호화하지 않고 빈 채로 보낸다. 받은 빈 데이터는 성공으로 본다(원본 `EncryptSerializer.cpp`).

**서버 쪽.** 이름이 `"proto"` 인 우회 코덱을 등록하고(`go/codec/codec.go`), 인터셉터가 원문 바이트(필드 999 `__raw__`)를 세션 키로 복호화한 뒤 protobuf 를 해석한다. 응답은 직렬화 뒤 암호화한다(`go/service/server_interceptor.go`). PreLogin 은 고정 세션 `"prelogin"` 을 쓴다.

**Go 클라이언트 구조(제안).**

```go
// 연결마다 하나. 서버가 여럿이면 서버마다 따로 ― 전역 RegisterCodec 을 쓰지 않는다.
type cryptCodec struct{ c atomic.Pointer[crypt] } // nil = 모드 0
func (cryptCodec) Name() string { return "proto" } // content-type application/grpc+proto 유지
func (k *cryptCodec) Marshal(v any) ([]byte, error) {
    b, err := proto.Marshal(v.(proto.Message))
    if c := k.c.Load(); err == nil && c != nil && len(b) > 0 { return c.Encrypt(b) }
    return b, err
}
func (k *cryptCodec) Unmarshal(d []byte, v any) error {
    if c := k.c.Load(); c != nil && len(d) > 0 { var err error; if d, err = c.Decrypt(d); err != nil { return err } }
    return proto.Unmarshal(d, v.(proto.Message))
}
// grpc.NewClient(addr, ..., grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)))
```

- TLS 를 끈 경우: PreLogin 요청을 고정 키(모드 3, `SvcParams{22, 1, "127.0.0.1", "127.0.0.1", 22, "SSBR"}`)로 암호화해 보내고, 응답도 같은 키로 푼다. 그 뒤 응답 값으로 만든 세션 키를 `atomic` 으로 갈아 끼운다. 서버의 Go 참고 클라이언트(`go/cmd/services/fm_client/fm_client_main.go:250-367`)가 같은 순서다.
- TLS 를 켠 경우(실제 운영): 처음부터 모드 0. 응답의 `enc_mode` 가 0 이 아니면 그 값으로 키를 만들어 갈아 끼운다(P2). 원본도 이 경로다(SEC-024).
- 원본은 출력 버퍼를 입력의 4배로 잡고 `INT_MAX/4` 를 넘는 크기를 거절한다. Go 는 길이가 늘어나는 모드(2·5)만 최대 16바이트 늘어 크기 걱정이 없다.

**키 유도.**

```
site   = SiteAbb 의 앞 4바이트까지
keyStr = fmt.Sprintf("SQI%.4s%d,%d,%d,%s,%s;KEY",    site, SvcID, SvcNo, SvcPort, MyIP, SvcIP)
vecStr = fmt.Sprintf("SQI%.4s%d,%d,%d,%s,%s;VECTOR", site, SvcID, SvcNo, SvcPort, MyIP, SvcIP)
         (SEED-128 만 ";IVECTOR")
key32  = reverseEachUint32(pbkdf2.Key(sha1.New, keyStr, []byte("sqi"), 8, 32))   // Go 1.24+ crypto/pbkdf2
iv32   = reverseEachUint32(pbkdf2.Key(sha1.New, vecStr, []byte("sqi"), 8, 32))
reverseEachUint32: 4바이트마다 순서를 뒤집는다(b0 b1 b2 b3 → b3 b2 b1 b0)
```

- 순서는 SvcID, SvcNo, **SvcPort**, MyIP, SvcIP 다(구조체 순서와 다르다).
- 뒤집기는 C 원본이 PBKDF2 결과를 uint32 낱말 배열로 만들고 리틀엔디언 메모리로 `memcpy` 하기 때문이다.
- C 원본 `sub_pkdf2.cpp` 는 자체 SHA1·HMAC·PBKDF2 로 40바이트를 만들고 끝 낱말을 0 으로 둔다. 앞 32바이트는 표준 PBKDF2-HMAC-SHA1 과 같다(원본 KAT 로 확인).
- `crypto/pbkdf2.Key` 의 시그니처는 `Key[Hash hash.Hash](h func() Hash, password string, salt []byte, iter, keyLength int) ([]byte, error)` 다(`go.mod` 가 Go 1.26 이라 쓸 수 있다). 외부 `golang.org/x/crypto/pbkdf2` 가 필요 없다.

### 7.3 모드별 알고리즘

| 모드 | 이름 | 쓰는 키·IV | 방식 | 길이 |
|---|---|---|---|---|
| 1 | SEED-256 | key32 전부(라운드 키 48개, 24라운드), iv32 전부 | 32바이트 단위 체인: 평문 32바이트를 직전 32바이트(처음은 IV)와 XOR → **앞 16바이트만** SEED-256 블록 암호화 → 그 32바이트가 다음 체인 값. 패딩 없음. 끝에 남는 조각(32 미만)은 직전 32바이트와 XOR 만 | 그대로 |
| 2 | ARIA-256 | key32, iv32[:16] | CBC + PKCS#7 패딩(16) | 16의 배수로 늘어남(딱 맞으면 16 더) |
| 3 | AES-256 | key32, iv32[:16] | CBC 패딩 없음. 끝 조각은 직전 암호문 블록(없으면 IV)과 XOR 만(CTS 아님) | 그대로 |
| 4 | SEED-128 | key32[:16], iv32[:16] | OFB 계열 스트림: IV 낱말을 빅엔디언으로 읽어 리틀엔디언으로 저장한 `ti` 를 16바이트마다 SEED-128 로 다시 암호화하고, 결과 낱말을 빅엔디언으로 풀어 키스트림으로 XOR. 암·복호가 같다 | 그대로 |
| 5 | HIGHT | key32[:16], iv32[:8] | KISA 구현 CBC + PKCS 패딩(8) | 8의 배수로 늘어남(딱 맞으면 8 더) |
| 6 | NP-ARIA-256 | key32, iv32[:16] | 벤더 DLL `npcl_win_64u.dll` 의 `NpEncSymk(OP_MODE_CBC)` + 패딩 | 늘어남. **제외** |

모드 1 복호: 32바이트 블록마다 앞 16바이트를 SEED-256 복호화하고 32바이트 전체를 직전 **암호문** 32바이트(처음은 IV)와 XOR. 끝 조각은 직전 암호문 32바이트와 XOR.

모드 3 복호: 표준 CBC 복호, 끝 조각은 직전 암호문 블록과 XOR.

**상태를 남기지 않는다.** 서버 `crypt2` 는 메시지마다 같은 키·IV 로 새로 시작한다. C 원본 SEED-128 은 호출 뒤 IV 를 마지막 `ti` 로 덮어써, 같은 객체로 두 번째 메시지를 처리하면 서버와 어긋난다(원본 시험 `tst_FMEncryption::seed128_encryptCorruptsKeyContext` 가 이 동작을 못박아 둠). Wails 판은 서버처럼 상태 없이 구현한다.

**HIGHT 복호 패딩.** 서버 Go `kisaHightCbcClose` 는 패딩 값이 8(평문 길이가 8의 배수)이면 `padlen <= 0` 으로 거절한다(`go/crypt2/hight_kisa.go:385-389`). 실측: 평문 8·16바이트 → `hight: invalid ciphertext or padding`, 7·33바이트 → 정상. Wails 판 복호는 C 원본처럼 패딩 값 1~8 을 받는다. 서버 결함은 서버 요청문으로 남길 거리다(이 문서는 서버를 고치지 않는다).

**구현 출처.** 서버 `E:\git\snBv6Srv\go\crypt2` 의 순수 Go 구현을 그대로 옮긴다(Wails 저장소 안 `internal/crypt` 같은 패키지):
`aes256_crypt.go`(표준 `crypto/aes` + 위 CBC 변형), `aria256_crypt.go` + `internalaria/`, `seed256.go`·`seed256_crypt.go`·`seed256_tables.go`(SS 표를 16진 문자열로 내장), `seed128_crypt.go`·`seed128_v4.go`, `hight256_crypt.go`·`hight_kisa.go`, `core.go`(키 유도).
바꿀 것: `golang.org/x/crypto/pbkdf2` → 표준 `crypto/pbkdf2`, HIGHT 패딩 검사, `SEED256_DUMP_ROUND_KEYS` 환경변수 출력 삭제, NP-ARIA 파일 빼기. 스크래치 사본에서 이렇게 고쳐 아래 KAT 15건이 모두 맞았다.

**원본 KAT**(원본 `tests/tst_FMEncryption.cpp:513-567`). 평문은 ASCII `"087 known answer 0123456789abcdef!"`(34바이트). 키 재료 셋:

- key0 = `SvcParams{SvcID 22, SvcNo 1, MyIP "127.0.0.1", SvcIP "127.0.0.1", SvcPort 22, SiteAbb "SSBR"}`(PreLogin 고정 키와 같다)
- key1 = `SvcParams{SvcID 99, SvcNo 7, MyIP "10.0.0.1", SvcIP "10.0.0.2", SvcPort 1234, SiteAbb "OTHER"}`
- key2 = `SvcParams{SvcID 31, SvcNo 2, MyIP "192.168.0.10", SvcIP "192.168.0.20", SvcPort 5000, SiteAbb "SQI"}`

| 모드 | key0 암호문(16진) |
|---|---|
| 1 SEED256 | `b7cc90b33093bd74173a382e368177dc184288309291f81042230abda21c1948d1ed` |
| 2 ARIA256 | `0e0fe5528baf3dd52ea223e1e61f5ea1559e0126b285946dcf6380a9b7cc7a7d370924e010dc99733b462c0a87b115f1` |
| 3 AES256 | `64ad3cee9eee0d600905cff798e47fcffcd35d0a70641b837a50859cc5100c409af2` |
| 4 SEED128 | `a5985e51430337580af087f2a687fd12207b569af532dd6e4a4345cc63c64db73912` |
| 5 HIGHT | `9a023cf33cbac1d5925e6352eed73a143b2197514975fae3a8417228123c351c72aff2628ec07c6c` |

| 모드 | key1 암호문(16진) |
|---|---|
| 1 | `ff789fb547a701008a9b25c728ddb0176183120523301874f10c291c650224789959` |
| 2 | `c7c3d93bcd796d54eb71b123f86a8e4317524a2c517eba79e2b909c5a12d570a724a945aea1f59ed07cbbe9ecc9bb48b` |
| 3 | `5cbbe3fd6f0f5777d58ff391c84e1e70ed1f24053fedb7da640582cce4409d088b3e` |
| 4 | `d3f40d881e56f81eff4a9b47adbee568ebd6fbbd7149b7fcc560a5dfd3f58c6e7eb3` |
| 5 | `6a6260b51c522708c2ff4b803d249799e2db621a7beed4ec10dbd600bf2ecd04b40c0dc31bb9e3e9` |

| 모드 | key2 암호문(16진) |
|---|---|
| 1 | `35d15b2fbcf14e0d247c1b4dfbc251dd834fd681068ea82172708425c965b82753f0` |
| 2 | `2a43a119280ee414827189fc367c2ad619db1f655219bcce5796af746038536ae5ffafac2505ff7948687e32ae066154` |
| 3 | `94bf72ca1eccba0cad490cb326f6362e6c2aeb6deb395a258be68c73285831c80a0b` |
| 4 | `944654337b0576e1d909c1bb287efc05990c7b42c3ac53926abf87da0e03c13201dc` |
| 5 | `81fd81a350a1c236245bb79917764361e1a6873772b8904aa89c4c202d6d6662dfaad7c0c43d8284` |

길이 확인: 모드 1·3·4 는 34바이트 그대로, 모드 2 는 48바이트, 모드 5 는 40바이트.

**공개 시험 벡터(블록 암호 단독).** 키 유도·체인을 빼고 블록 함수만 확인한다. 스크래치 사본에서 모두 맞았다.

| 알고리즘 | 출처 | 키 | 평문 | 암호문 |
|---|---|---|---|---|
| ARIA-256 | RFC 5794 A.3 | `000102…1e1f` | `00112233445566778899aabbccddeeff` | `f92bd7c79fb72e2f2b8f80c1972d24fc` |
| SEED-128 | RFC 4269 B.1 | 0(16바이트) | `000102030405060708090a0b0c0d0e0f` | `5ebac6e0054e166819aff1cc6d346cdb` |
| SEED-128 | RFC 4269 B.2 | `000102030405060708090a0b0c0d0e0f` | 0(16바이트) | `c11f22f20140505084483597e4370f43` |
| HIGHT | 설계 논문 표기 | `00112233445566778899aabbccddeeff` | `0000000000000000` | `00f418aed94f03f2` |
| HIGHT | 위를 KISA 바이트 순서로 | `ffeeddccbbaa99887766554433221100` | `0000000000000000` | `f2034fd9ae18f400` |
| HIGHT | 설계 논문 표기 | `ffeeddccbbaa99887766554433221100` | `0011223344556677` | `23ce9f72e543e6d8` |
| HIGHT | 위를 KISA 바이트 순서로 | `00112233445566778899aabbccddeeff` | `7766554433221100` | `d8e643e5729fce23` |
| AES-256 | 표준 라이브러리 | ― | ― | `crypto/aes` 를 그대로 써서 따로 시험하지 않는다 |
| SEED-256 | 없음 | ― | ― | 공개 벡터가 없다(검색: RFC·KISA 배포 자료에서 SEED 256비트 키 벡터를 찾지 못했다). 원본 KAT 만 쓴다 |

KISA HIGHT 구현은 키·평문·암호문의 바이트 순서를 모두 뒤집어 다룬다 ― 논문 벡터를 그대로 넣으면 틀린다.

### 7.4 모드 6(NP-ARIA)을 빼는 근거

- 원본 `sub_nparia_256.cpp` 는 키·IV 를 다른 모드와 같은 PBKDF2 로 만든 뒤, 실제 암호화는 벤더 DLL(`npcl_win_64u.dll`, Linux `libnpcl_linux_64u.so`)의 `NpEncSymk`/`NpDecSymk` 에 맡긴다. 컨텍스트 `n_CTX` 는 iv 16 + key 32 + 더미 520 = 568바이트로 패킹돼 있다.
- PreLogin 응답 `ctx` 는 원본이 744바이트일 때만 복사한다. Go 서버는 `ctx` 를 채우지 않고, 서버 키는 서버 설치본의 고정 파일(`/var/ssbr/.ssbr.master.uuid`)이다. 서버 `crypt2` 의 Windows·비CGO 빌드는 빈 껍데기(stub)다.
- 원본 보안 점검(B-038)은 `ctx` 를 복사한 뒤 `MakeKey` 가 0 으로 채운 컨텍스트로 덮는다고 읽었다 ― 원본에서도 동작이 확인되지 않았다.
- Go 에서 쓰려면 CGO 또는 `syscall.LoadDLL` 로 벤더 DLL 을 불러야 하고 배포본에 DLL 이 늘어난다. 쓰는 서버가 없으므로 **제외**한다. 서버가 모드 6 을 보내면 `서버 암호 모드 6 은 지원하지 않습니다.` 로 로그인을 멈춘다.

### 7.5 단말 식별

```
usable(it) = !loopback && Up && Running && len(MAC)>0 && !hasPrefixFold(name,"vm") && !hasPrefixFold(name,"virtual")

1순위: net.DialTimeout("udp", 서버주소, 200ms) → LocalAddr IP
       루프백이 아니고, 그 IP 를 가진 usable 어댑터가 있으면 (IP, MAC)
2순위: usable 어댑터를 차례로 보며 첫 IPv4 중 루프백·링크로컬(169.254/16)이 아닌 것 → (IP, MAC)
실패:  ("", "") + 경고 `단말 식별 실패 >>>`
MAC 형식: 구분자를 뺀 대문자 16진 12자
```

- 근거: 원본 `FileMoverGrpc.cpp:697-804`. 서버의 `login_ip_check`·`login_mac_check` 가 계정에 등록된 IP·MAC 쌍을 대조하므로 둘을 다른 어댑터에서 고르면 거절된다.
- Go `net.Interface.Flags` 에 `FlagUp`·`FlagRunning`(Go 1.20+)·`FlagLoopback` 이 있다. 이름은 Windows 에서 "이더넷"·"Wi-Fi" 같은 연결 이름이다. 추정: Qt `QNetworkInterface::name()` 은 Windows 에서 내부 이름(`ethernet_32769` 형식)을 돌려주고 `humanReadableName()` 이 연결 이름이다 ― 원본이 어느 쪽에 `vm` 을 대조하는지 확인하고 맞춘다. VMware·Hyper-V 어댑터가 있는 PC 로 원본과 같은 IP·MAC 이 나오는지 비교한다.
- 지금 Wails `deviceIdentity` 는 UDP 연결 시간 상한이 없고(`net.Dial`), 어댑터 거르기와 2순위가 없다.

### 7.6 TLS 와 핸드셰이크 실패 가려내기

- `tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}`. `ServerName` 은 grpc 가 주소에서 채운다. 서버 인증서가 IP 를 SAN 으로 가지는지는 원본과 같은 `root_ca.pem`·주소로 이미 붙고 있으므로 그대로다.
- 인증서 여러 개: `AppendCertsFromPEM` 이 PEM 블록을 전부 읽는다(업무망·인터넷망 CA 둘 다 CN 이 `SmartBridge_CA`). 하나도 못 읽으면 `서버 인증서를 검증할 수 없습니다.`
- 핸드셰이크 실패 구분: gRPC 오류는 `Unavailable` 하나로 뭉쳐 온다. 방법 둘:
  1. 오류 메시지에 `x509:`·`tls:`·`certificate` 가 들어 있는지 본다(grpc-go 는 `connection error: desc = "transport: authentication handshake failed: tls: ..."` 를 싣는다). 가장 짧다.
  2. `credentials.TransportCredentials` 를 감싸 `ClientHandshake` 오류를 원자 변수에 남긴다. 문자열에 기대지 않는다.
  추정: 1 로 충분하다. 위조 서버 시험(원본 `tools/security/mitm-tls-relay.py`)으로 확인한다.
- 실패마다 로그 `[gRPC] 서버 인증서 검증 실패: <사유> | host:<주소>`. `InsecureSkipVerify` 를 쓰지 않는다.

### 7.7 무결성 검사

**대상 목록.** 원본 10개는 Qt 빌드의 파일이다(`FileMoverApp.exe`·`FileMoverUpdater.exe`·`lang_en.qm`·DLL 6개·`fmenv.ini`). Wails 배포본에는 대부분 없다. 추정: Wails v3 Windows 배포본은 `snbApp.exe` 하나에 화면(`frontend/dist`)이 들어가고 WebView2 로더도 실행 파일에 들어간다 ― 그러면 대상은 `snbApp.exe`·`fmenv.ini` 둘이다. 배포본 구성이 정해지면(15) 목록을 그 파일로 정한다.

**서버에 등록해야 한다.** 서버는 이름으로 첫 행만 대조하고, 없는 이름은 `file not found in database` 로 실패한다. 원본은 `tools/integrity-sync.py` 가 `add_hash` 로 등록하고 옛 행을 `use_flag=0` 으로 내린다. Wails 판도 같은 도구로 Wails 배포 폴더를 등록할 수 있는지 확인하고, 운영 서버 `ssbr_version.ini [FILEMOVER]` 목록 변경은 서버 요청문으로 낸다. 원본 Qt 판과 같은 서버를 함께 쓰는 동안에는 두 판의 이름이 겹치지 않아야 한다(`fmenv.ini.int/.ext` 는 겹친다 ― 같은 `fmenv.ini` 를 쓰면 문제없다).

**계산.**

```go
func sha256Hex(path string) (hex string, size int64, ok bool) // 64KB 단위로 읽는다(io.Copy 기본 32KB 도 무방)
```

- Wails 에 `sha256File` 이 이미 있다(`client.go:456`). 그것을 쓰고, 열기 실패면 원본처럼 공유 모드를 넓혀 다시 연다: Windows 에서 `windows.CreateFile(..., FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE, ...)`. 추정: Go `os.Open` 은 `FILE_SHARE_READ|FILE_SHARE_WRITE` 로 열어(Go 1.23 부터 DELETE 도 포함한다는 기억이 있으나 확인하지 않았다) 대부분 다시 열 필요가 없다. 실행 중인 자기 exe 는 읽기 공유로 열린다.
- 파일 10개 → 2개라 병렬화는 의미가 없다. 원본보다 낫게 할 곳은 "UI 스레드를 막지 않는다"(Go 고루틴이면 저절로 된다).

**판정.** 3.6 절 그대로. 순수 함수 `integrityFailures(sent []string, answers []*FileChecksumResult, notHashed []string) []string` 와 `integritySkipUnanswered(checkType int, code codes.Code) bool` 로 두고 `_test.go` 하나로 원본 `tst_IntegrityVerdict` 8건을 옮긴다.

**개발 빌드 판정.** 원본은 실행 파일 옆 `CMakeCache.txt` 로 가린다. Wails 판은 빌드 태그로 가린다: `-tags dev` 는 개발 빌드, `-tags production`(`wails3 build`·Docker)은 `SNB_DEV=1` 도 듣지 않는 배포 빌드, 그 밖에는 `SNB_DEV=1` 일 때만 개발 빌드다(`config.IsDev`). 개발 빌드면 무결성 실패로 종료하지 않고 `무결성 검증 실패( <사유> ) ― 개발 빌드라 종료하지 않는다`.

**시점.** 로그인 완료(0), 3시간 `time.Ticker`(1, 로그인 시도 때 다시 켜고 앱 끝날 때만 멈춤, 로그인 상태가 아니면 건너뜀), 알림 9005(2). Windows 가 아니면 보내지 않는다.

### 7.8 단일 인스턴스

- Wails v3(`go.mod` 의 v3.0.0)에 `application.Options.SingleInstance *SingleInstanceOptions{UniqueID, OnSecondInstanceLaunch, AdditionalData, ExitCode, EncryptionKey}` 가 있다(`pkg/application/single_instance.go:28`). Windows 는 명명 뮤텍스 `<UniqueID>-sim` + 숨은 창에 `WM_COPYDATA` 로 두 번째 실행의 인자를 넘긴다(`single_instance_windows.go`). 새 패키지가 필요 없다.
- 두 번째 실행의 인자(`--mode upload <경로>`)는 `SecondInstanceData.Args` 로 첫 앱에 온다 → 업로드 IPC(03)로 넘긴다. 원본의 400ms 묶음 처리도 03 에서 같이 옮긴다.
- 차이: 원본은 **두 번째 프로세스**가 `snB App 프로그램이 이미 실행중입니다.` 를 띄운다. Wails 는 두 번째 프로세스가 곧바로 끝난다. 그래서 첫 앱이 창을 앞으로 올리고(숨어 있으면 다시 보이고) 같은 문구를 알림으로 보인다. `--mode upload/download` 는 문구 없이 경로만 처리한다.
- `UniqueID` 는 Qt 판(`MOSFM 6.0`)과 다르게 둔다(예: `com.sqisoft.snbapp`). 개발 빌드의 `SNB_INSTANCE` 가 있으면 뒤에 붙인다(P2).
- 원본 `noupdate` 모드(앞 앱이 끝나기를 5초 기다림)는 업데이트 헬퍼 전용이라 옮기지 않는다.

### 7.9 자동 로그아웃

```
Go: type idle struct{ mu; t *time.Timer; limit time.Duration; gen uint64 }
    start(min): min==0 이면 끔. t = time.AfterFunc(limit, fire)
    touch():    t.Reset(limit)   ← 화면 활동·전송 진행 보고가 부른다
    fire():     세션이 같으면 로그아웃 → 이벤트 "SESSION_TIMEOUT"(min)
화면: pointerdown·keydown·wheel·touchstart 를 window 에서 capture 로 듣고 1초에 한 번만 Touch() 호출
     (pointermove·hover 는 세지 않는다, preventDefault 하지 않는다)
```

- 원본은 10초 간격 시계 + 6번을 1분으로 세어 최대 10초 늦는다. `time.AfterFunc` 는 정확하다.
- 원본은 시계를 첫 서버 파일 목록 조회 때 켠다(`FileTransfer.cpp:292-295`). Wails 판은 로그인 완료 때 켠다(실질 같음).

### 7.10 로그아웃·세션 정리

```
logout():
  1. loggedIn = false; sessionGen++        ← 늦게 온 응답·알림을 세대 번호로 버린다
  2. 알림 스트림 context cancel
  3. 자동 로그아웃 timer Stop, 무결성 ticker 는 그대로(로그인 상태가 아니면 건너뜀)
  4. token = ""; crypt = nil; 비밀번호 []byte clear
  5. 대기·진행 전송 전부 취소(전송 영역의 cancel)
  6. conn.Close()                          ← TCP 닫기 = 서버 세션 회수
  7. 화면 이벤트 "LOGGED_OUT"               ← 화면은 팝업·선택 창 닫고, 앞 세션 창 콜백 버림
```

- 원본 순서(`FileMoverGrpc.cpp:80-144`)와 같다. 세대 번호는 원본의 "로그아웃 뒤 도착한 알림을 버린다"(AUTH-037)와 "앞 세션 창 콜백을 버린다"(AUTH-036)를 한 장치로 한다.
- 버리는 로그 `로그아웃 뒤 도착한 알림을 버립니다 >>>`. 클립보드 받기 알림은 버리지 않는다(원본이 그렇다 ― 09·클립보드와 함께 본다).

### 7.11 Go 패키지 후보

| 용도 | 패키지 | 비고 |
|---|---|---|
| gRPC·protobuf | `google.golang.org/grpc`, `google.golang.org/protobuf` | 이미 있다 |
| 메시지 암호(모드 1~5) | 서버 `crypt2` 사본 + 표준 `crypto/pbkdf2`·`crypto/sha1`·`crypto/aes`·`crypto/cipher`·`encoding/binary` | 외부 패키지 없음 |
| 무결성 해시 | `crypto/sha256`, `encoding/hex` | 이미 있다 |
| 단일 인스턴스 | Wails v3 `application.SingleInstanceOptions` | 새 의존 없음 |
| 넓은 공유 모드로 다시 열기 | `golang.org/x/sys/windows` | 이미 간접 의존(`go.mod`) |
| 단말 식별 | 표준 `net` | |
| 타이머 | 표준 `time` | |
| ini 읽기 | 이미 있는 `readServers`(손으로 짠 해석기) | `fmusr.ini [USER]` 덮어쓰기도 같은 해석기로. 외부 ini 패키지를 더하지 않는다 |
| EUC-KR | 필요 없음 | 보내는 값이 ASCII 이고 서버 응답이 UTF-8. 한글 이름이 깨지면 그때 `golang.org/x/text/encoding/korean`(이미 간접 의존인 `x/text` 안) |
| 명명 파이프 | 필요 없음 | Wails 단일 인스턴스가 대신한다(`github.com/Microsoft/go-winio` 를 더하지 않는다) |

---

## 8. 원본보다 낫게 할 곳

| 곳 | 원본 | Wails 판 | 근거 |
|---|---|---|---|
| 정책 조회 실패 | `접속 중..` 에 영원히 멈춤, 서버에는 세션이 남음 | 15초 뒤 연결을 닫고 `서버에 연결할 수 없습니다.` | 원본 명세 경계 첫 항 |
| 서버 정보·CA 없음 | 안내가 보이기 전에 종료될 수 있음 | 로그인 화면에 남아 원인을 보임 | AUTH-023 상충, 원본 확인 필요 3 |
| 강제 비밀번호 변경 | 메인 화면이 뒤에서 이미 열림 | 변경 성공 전에는 `LOGIN_COMPLETE` 를 내지 않음. [취소] = 종료는 유지 | AUTH-046 |
| 비밀번호 변경 계정 칸 | [ID 저장] 을 끄면 빈 값 → 조용히 실패 | 세션의 실제 ID | AUTH-048 |
| 변경 전문 실패 | 로그만, 화면 반응 없음 | `비밀번호를 변경하지 못했습니다.` + 사유(추정 문구 ― 문구는 화면 설계 때 정한다) | AUTH-049 |
| 만료 안내 | `QMessageBox::exec()` 가 로그인 처리를 멈춤 | 비동기 창, 답을 기다리는 동안에도 Go 쪽 처리는 계속 | AUTH-045 |
| 비밀번호 변경 창 두 벌 | `HomePage.qml`·`UserProfileInfo.qml` 각각 | 하나 | 원본 확인 필요 9 |
| 연속 실패 창 | `오류` 창이 겹침 | 한 창, 문구 교체 | 원본 명세 경계 |
| 자동 로그아웃 정밀도 | 10초 단위 | `time.AfterFunc` 로 정확 | AUTH-027 |
| 두 번째 실행 | 트레이로 숨은 창은 다시 안 보임 | 숨은 창도 다시 보임 | AUTH-059 |
| SEED-128 상태 | 두 번째 메시지에서 서버와 어긋남 | 상태 없음 | 7.3 절 |
| HIGHT 복호 | (서버 결함) 8의 배수 길이 거절 | 패딩 1~8 수용, 서버 요청문 | 7.3 절 |
| 늦은 응답·알림 | 알림 종류마다 따로 거름(11종) | 세션 세대 번호 하나로 모든 응답·알림·창 콜백을 거름 | AUTH-036·037 |
| 단말 식별 시간 | 원본 0.2초, Wails 지금 상한 없음 | 0.2초 상한 | AUTH-009 |
| 로그인 응답 문자열 | 고정 칸에서 잘라 담음 | 자르지 않음 | AUTH-004 |
| 무결성 계산 | GUI 와 같은 스레드에서 해시(추정: 원본은 요청 함수 안에서 계산) | 고루틴 | 7.7 절 |
| 사용자 정보 팝업 클립보드 방향 | 두 번 뒤바뀜 의심 | proto 주석 기준으로 한 번에 계산, 원본 화면과 대조 | 원본 확인 필요 10 |

하지 않을 것(원본을 그대로 둠):

- 로그인 칸 형식 검사(서버 판정에 맡긴다), 계정 실패 한 문구(없는 ID 를 알아내지 못하게), 무결성 창에 파일 이름을 쓰지 않음, 비밀번호 비저장, TLS 강제 ― 모두 보안 결정이라 바꾸지 않는다.
- 비밀번호 규칙을 서버 정책에서 받기(AUTH-050)는 원본에서도 결정되지 않았다(원본 확인 필요 1). 코드 고정 규칙을 그대로 옮긴다.

---

## 9. 실 서버 검증 방법

비밀번호는 여기에 적지 않는다. 시험 계정 자격은 원본 `tools\.secrets\` 와 Wails `live_test.go` 머리 주석의 환경변수(`SNB_LIVE_ID`·`SNB_LIVE_PW`)로만 넘긴다.

### 9.1 준비

- 설정: `$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'` ― 원본 개발 빌드의 `fmenv.ini`·`root_ca.pem` 을 그대로 쓴다. 화면의 망 이름과 실제 접속 IP 가 같은지 로그 `로그인 성공: … @ <IP>` 로 확인한다.
- 계정 역할: 일반 사용자(valid), 결재 요청자(requester), 결재자(approver), 대결자(proxy), 보안 담당(sec_user). 원본 E2E 계정은 서버 권한 프로필 `E2E-QA` 에 있다. autoqa 실행과 같은 시각에 이 계정을 쓰지 않는다(공용 자원 ― 원본 `AGENTS.md` "e2e QA").
- 서버 정책은 SBM 화면으로 바꾸고, SBM 에 화면이 없는 값만 원본 `tools\srv_settings.py` 로 바꾼다. 바꾼 값은 시험 뒤 되돌린다. 잠긴 시험 계정은 `srv_settings.py unlock` 으로 푼다.
- 무결성 시험 전 주의: 원본 `build-release.cmd` 와 autoqa 실행 직전 단계가 서버 `file_hash` 를 각자 덮어쓴다. Wails 판 무결성 시험은 다른 세션이 빌드·E2E 중이 아닐 때 한다.

### 9.2 시나리오

| # | 확인할 것 | 서버 설정·계정 | 성공 판정(화면·로그) |
|---|---|---|---|
| 1 | 정상 로그인 | valid | 메인 화면, 툴바에 이름, 로그 `LOGIN TRY >>> ID[..] PW[*** len=N]`·`로그인 완료 >>> msg[LOGIN SUCCESS]`. 로그 파일에 비밀번호 문자열 0건 |
| 2 | 틀린 비밀번호 | valid | `로그인에 실패하였습니다.` 창, 닫아도 로그인 화면 유지, 비밀번호 칸 그대로 |
| 3 | 없는 ID | 아무 ID | 같은 문구, 창을 닫으면 앱 종료 |
| 4 | 31바이트 초과 | 한글 11자 ID | `아이디 또는 비밀번호가 너무 깁니다.`, 서버 요청 0건 |
| 5 | 서버 꺼짐·주소 틀림 | `fmenv.ini` IP 를 닿지 않는 주소로(사본에서) | 15초 안에 `서버에 연결할 수 없습니다. 접속 서버 설정을 확인해 주세요.` |
| 6 | 위조 서버 | 원본 `tools/security/mitm-tls-relay.py` 를 손으로 띄움 | `서버를 신뢰할 수 없어 접속하지 않았습니다. 관리자에게 문의해 주세요.`, 로그 `[gRPC] 서버 인증서 검증 실패:` |
| 7 | CA 없음 | 빈 `root_ca.pem`(사본 폴더) | `서버 인증서를 검증할 수 없습니다.`, 앱은 로그인 화면에 남음 |
| 8 | IP·MAC 대조 | 권한 프로필 `login_mac_check` 켜고 계정에 다른 MAC 등록 | `로그인에 실패하였습니다.`, 로그 상세에 `MAC_ERROR`. 원본과 같은 IP·MAC 이 실리는지 원본 앱 로그와 비교 |
| 9 | 중복 로그인 | 다중 로그인 불허, 원본 앱으로 같은 계정 먼저 로그인 | `중복 로그인` 창 `해당 계정은 이미 다른 PC에서 로그인 중입니다.\n프로그램을 종료합니다.`, 닫으면 종료, 원본 앱 세션 유지 |
| 10 | 자동 로그아웃 | `fm_logout_auto=1` | 1분 무입력 뒤 로그인 화면 + `1분 동안 사용하지 않아 자동으로 로그아웃했습니다.`. 마우스를 누르고 있으면 끊기지 않음 |
| 11 | 로그아웃 세션 회수 | valid | [로그아웃] 직후 원본 앱으로 같은 계정 로그인 → 중복 로그인 없이 성공 |
| 12 | 연결 끊김 | 서버 재시작은 공용이라 피하고, 방화벽으로 서버 포트를 잠시 막는다(로컬 PC 규칙) | `연결 끊김` 창, 닫으면 로그아웃 방식대로 |
| 13 | 기본 비밀번호 | 계정 비밀번호를 정책 `default_pw` 와 같게 | 로그인 직후 변경 창, 로그 `기본PW[set] PW[*** len=N] 같음[Y]` |
| 14 | 만료 예정·지남 | 서버 비밀번호 변경 주기를 조정해 `pswd_remain_day` 를 양수·음수로 | 양수: 만료 예정 물음. 음수: 강제 변경 창, 변경 전 메인 화면 안 보임, [취소] = 종료 |
| 15 | 비밀번호 변경 | valid, [ID 저장] 끔 | 성공 문구. 같은 비밀번호 다시 → 서버 사유(쓴 적 있는 비밀번호) 그대로 표시. 변경 뒤 원래 값으로 되돌린다 |
| 16 | 사용자 정보 팝업 | requester·approver | 방향별 허용·결재 방식·크기·개수가 원본 앱 팝업과 같음 |
| 17 | 무결성 통과 | Wails 배포본을 서버에 등록한 뒤 설치본으로 | 로그 `무결성검사 응답[3511] => <무결성 오류없음> 검증건수: N` |
| 18 | 무결성 실패 | 등록 뒤 `fmenv.ini` 한 글자 바꿈(사본) | `무결성 검증 실패` 창 → 닫으면 종료. 개발 빌드는 경고만, `SNB_INTEGRITY_ENFORCE=1` 이면 종료 |
| 19 | 관리자 요청 검사 | SBM 사용자 정보 [변조 체크] | 로그에 검사유형 2 요청, 통과면 화면 변화 없음 |
| 20 | 주기 검사 | 개발 빌드 `SNB_INTEGRITY_PERIOD_SEC=40` | 40초마다 검사유형 1, 로그아웃 상태에서는 `무결성검사 건너뜀 ― 로그인 상태가 아니다.` |
| 21 | 두 번째 실행 | ― | 첫 창이 앞으로(트레이에 숨어 있어도), 두 번째 프로세스 종료. `--mode upload <파일>` 은 첫 앱에 경로 전달 |
| 22 | 9004 | 실 서버로 낼 길이 확인되지 않음 | 개발 빌드 시험 진입점으로 알림을 흉내 내 `다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다.` → 닫으면 종료 |
| 23 | 암호 모드 1~5(P2) | TLS 를 켠 서버는 늘 0 이라 실 서버로 시험할 수 없다 | 단위 시험: 7.3 절 KAT 15건 + 공개 벡터 + 왕복. 서버 `crypt2` 원본과 교차(같은 입력 → 같은 암호문) |

### 9.3 단위 시험(Wails 저장소, 첫 배포 전)

- `go test ./...` 에 다음을 둔다: 무결성 판정 8건(원본 `tst_IntegrityVerdict` 옮김), 망별 `fmenv.ini` 이름, 로그인 실패 문구 판정표(상태·메시지 → 문구·종료 여부), 31바이트 검사, 단말 식별의 어댑터 거르기(가짜 어댑터 목록), `fmusr.ini [USER]` 덮어쓰기.
- P2 에서: 암호 KAT 15건, 공개 벡터 7건, 모드별 길이(34 → 34/48/34/34/40), HIGHT 8의 배수 길이 왕복.

---

## 부록: 이 문서가 기대는 원본 위치

| 내용 | 위치 |
|---|---|
| 로그인 전문·실패 판정 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:517-914` |
| 로그인 응답·중복 로그인 플래그 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:919-1005` |
| 로그아웃·자동 로그아웃 시계 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:65-144` |
| 단말 식별 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:697-804` |
| TLS·CA | `imports/FileMoverGrpc/FileMoverGrpc.cpp:346-413`, `CustomHttp2Channel.cpp:443-451` |
| 무결성 요청·판정 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:1386-1511`, `IntegrityVerdict.cpp` |
| 비밀번호 변경 전문 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:1514-1531`, `:1716-1742` |
| 알림 9004·9005 | `imports/FileMoverGrpc/FileMoverGrpc.cpp:2543-2561` |
| 화면 쪽 분기 | `src/ViewControllerNotifySlot.cpp:410-531`, `:750-772`, `:915-1057`, `:1140-1179` |
| 단일 인스턴스 | `src/main.cpp:348-372`, `:522-565`, `:806-861` |
| 암호 구현 | `imports/FMEncryption/*.cpp`, 서버 `go/crypt2/*.go` |
| KAT | `tests/tst_FMEncryption.cpp:513-567` |
| 서버 암호 모드 결정 | 원본 `docs/security/13_서버-encmode-조사.md`, 서버 `go/cmd/services/fm/fm.go:450-467`·`:549` |
