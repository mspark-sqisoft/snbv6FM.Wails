# 앱 틀·설정·빌드 배포 ― Wails 이식 이해 문서

- 작성: 2026-10-30, 읽기 전용 조사(원본 코드는 고치지 않았다)
- 원본: `E:\git\snBv6FM`(Qt Quick + C++), 이식 대상: `E:\git\snBv6FM.Wails`(Go + Wails v3 + Svelte 5)
- 근거 명세: `specs/current/12-main-window.md`(MAIN-001~162), `specs/current/13-settings.md`(SET-001~106),
  `specs/current/15-build-deploy.md`(BLD-001~047)
- 표기: "추정:" 은 코드·실행으로 확인하지 못한 판단이다. "없다" 를 적을 때는 무엇을 찾아봤는지 함께 적었다.

---

## 1. 범위

이 문서가 다루는 것은 "로그인 뒤 앱의 틀과 그 주변" 이다. 개별 업무 화면(파일전송 표·결재·이력·로그)의 내용은
다른 영역 문서가 다루고, 여기서는 그 화면들을 담는 그릇과 공통 부품만 본다.

| 묶음 | 원본 파일 | 이 문서에서 보는 것 |
|---|---|---|
| 앱 틀 | `Main.qml`, `content/HomePage.qml`, `content/SideBar.qml`, `content/BottomBar.qml`, `content/ShellUserArea.qml`, `content/UserProfileInfo.qml` | 창 크기·배치(넓은/좁은/모바일), 메뉴 모델, 툴바, 사용자 권한 팝업, 화면 명령(`qmlSignal`) 분배, 공통 대화상자 |
| 알림 띠 | `controls/CustomInfoBar.qml`, `settings/Notify.qml` | 답이 필요 없는 안내의 표시 규칙 |
| 로컬 탐색기 | `content/LocalFileExplorerPanel.qml`, `src/model/LocalDirTreeModel.cpp`, `src/model/LocalFileListModel.cpp` | 폴더 트리·목록·새 폴더·업로드/다운로드 연결·끌어 놓기 |
| 끌어 내기 다운로드 | `src/DriveWatcherThread.cpp`, `ServerFileModel::createDropTempFile` | 서버 파일을 탐색기로 끌어 내면 받는 장치 |
| 트레이·단축키·단일 실행 | `src/ViewController.cpp`(`setTrayIcon`·`initHotKey`·`changeHotKey`·`programExit`·`programTray`), `src/main.cpp` | 트레이 메뉴, 전역 단축키, 두 번째 실행 처리, `--mode upload|download` |
| 설정 화면 | `content/SettingsView.qml`, `content/SettingsScrollView.qml`, `settings/AppSettings.qml` | 테마·배치·언어·로그 저장·로그아웃 방식·시작 폴더·자동 전송 폴더·단축키·프로그램 정보 |
| 설정 저장소 | `imports/FileMoverInfo/FilemoverSettings.cpp`, `settings/AppSettings.qml` | 레지스트리·`fmusr.ini`·`fmenv.ini`·`fmlocal.ini` 의 키와 우선순위 |
| 다국어 | `src/LanguageManager.cpp`, `lang/lang_en.ts` | 한국어 원문 + 영어 번역 전환 |
| 정책 화면 | `content/PolicyScrollView.qml`, `ViewController::getPolicyList` | 메뉴에는 없지만 남아 있는 정책 조회 표 |
| 로그아웃 확인 | `content/LogoutActionDlg.qml` | 로그아웃 처리 방식 고르기 |
| 빌드·배포 | `CMakeLists.txt`, `tools/build-release.cmd`, `installer/*.nsi`, `imports/FileMoverUpdater` | 버전, 설치본 두 개(업무망·인터넷망), 설치 동작, 업데이트 미제공 |
| 디자인 값 | `settings/Theme.qml`, `settings/Constants.qml`, `content/README.md` "색·글자 크기" | Wails 디자인 토큰의 출발점 |

범위 밖(다른 영역): 로그인 화면 자체, 서버 파일 표·전송 대기열·전송 상태 서랍, 결재·대리결재, 전송내역, 로그 화면,
클립보드 전송 본체, 자동 업로드·다운로드의 전송 동작(여기서는 설정값과 저장만), 무결성 검사, 링크연동(29000 포트).
WASM 전용 항목은 이식 대상에서 뺐다(과제 지시).

현재 Wails 판의 상태(`README.md`, `client.go` 확인): 서버 고르기·로그인·서버 파일 목록·단일 업로드·다운로드만 된다.
트레이·설정·다국어·탐색기·단축키·단일 실행·설치본은 없다(`app.go`·`main.go`·`frontend/src/App.svelte` 에서
`systray`·`SingleInstance`·`i18n`·`hotkey` 를 찾아 없음을 확인).

---

## 2. 기능 표

우선순위: **P1** = 첫 판에 반드시(원본 사용자가 매일 쓰는 것), **P2** = 첫 판 뒤, **제외** = 이유를 적음.
난이도: 상(네이티브 API·운영체제별 차이·서버 확인 필요) / 중 / 하.
명세 상태가 '구현'·'부분' 인 요건을 모두 담았다(범위를 묶어 적은 줄은 그 범위의 요건 전부가 같은 판단이다).

### 2.1 앱 틀·창·배치(12-main-window)

| ID | 기능 | Wails 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| MAIN-001 | 창 880×660 시작, 로그인 전후 크기 고정·사용자 늘림 허용 | P1 | 하 | Wails `WebviewWindowOptions{Width:880,Height:660}` |
| MAIN-002 | 최소 폭 360, 최소 높이 272(모바일 배치 720) | P1 | 하 | 원본 식은 480/360 을 고르려다 늘 360(상수 비교 오류). Wails 는 최소 폭을 그대로 360 으로 두되 모바일 배치 최소 높이 720 은 창 최소값이 아니라 레이아웃 스크롤로 처리하는 편이 낫다(창이 720 아래로 못 줄어드는 불편 제거) |
| MAIN-003 | 창 제목 `snB App v6.0`, 배경 `Theme.bg` | P1 | 하 | Wails 창 `BackgroundColour` 를 테마 bg 로 맞춰 첫 그림 전 흰 번쩍임 방지 |
| MAIN-004 | 배치 판정: 넓은(폭≥880·폭≥높이)/좁은(647≤폭<880)/모바일 | P1 | 하 | CSS 미디어쿼리 `(min-width:880px) and (min-aspect-ratio:1/1)` 등으로 대체, JS 계산 불필요 |
| MAIN-005 | 배치 값(`layoutIndex`) 2=상단 탭, 그 밖=왼쪽 메뉴+툴바, 저장된 1 은 0 으로, 로그인 전은 0 모양 | P1 | 중 | 기본 2 |
| MAIN-006 | 모바일 배치: 왼쪽 메뉴 숨김·아래 탭바 56·툴바 39(데스크톱 44) | P2 | 하 | 데스크톱 창을 좁힐 때만 생기는 모양. 첫 판은 좁은 배치(아이콘 메뉴)까지만 해도 사용 가능 |
| MAIN-007 | (상충) 배치 2 에서 화면 제목 줄 숨김 | P1 | 하 | 상충: 016 FR-003 은 "제목으로 지금 화면을 안다". Wails 는 탭 강조 + 창 제목 부제(예: `snB App v6.0 — 전송내역`)로 둘 다 만족시키는 안을 권함 |
| MAIN-008, MAIN-010 | 넓은/좁은 배치용 뷰를 둘 다 만들고, 좁은 배치 스와이프로 망 변경 금지 | 제외 | - | Qt 레이아웃 제약 때문의 구현 방식. Svelte 는 한 컴포넌트를 반응형으로 그린다. 스와이프 자체를 만들지 않으므로 금지할 것도 없다 |
| MAIN-009 | 안 보이는 뷰 만들지 않기(보류 024) | (미구현·보류 묶음, 아래) | | |
| MAIN-011 | Windows 렌더 루프 basic+OpenGL | 제외 | - | WebView2 가 렌더링. 대신 "GUI 스레드를 막지 않는다" 원칙은 Go 고루틴으로 자연 충족 |
| MAIN-020 | 망·역할별 메뉴: 업무망 결재자 8개(정책 숨김 → 7), 대리결재자 5, 일반 4 | P1 | 중 | 역할은 로그인 응답 `aprv_yn`·`replace_yn`. 인터넷망(LOC 0)은 늘 일반 메뉴 |
| MAIN-021 | [정책] 화면을 메뉴에 내지 않음 | P1 | 하 | 화면을 만들지 않으면 자동 충족(2.4 참조) |
| MAIN-022, MAIN-023 | 메뉴 누름: 현재 화면 표시 먼저 바꾸고 화면 교체 / 탭은 즉시 교체 | P1 | 하 | Svelte 상태 하나(`currentView`)로 통일 |
| MAIN-024 | 메뉴 이동 때마다 화면 새로 만듦 | P1 | 하 | Wails 에서는 화면 상태(스크롤·필터)를 유지하는 쪽이 더 낫다. 다만 목록은 진입 시 새로 조회해야 원본과 같다(8장) |
| MAIN-025 | 로그인 성공 시 [파일전송] 으로 | P1 | 하 | |
| MAIN-026~031 | 메뉴 강조(3×18 막대·raised 면·DemiBold·강조색 아이콘), 넘치면 스크롤, 넓은 배치에서만 [메뉴 접기], 아이콘 전용 때 0.5초 툴팁, 폭 189/250(영어), 탭 균등 폭 | P1 | 하 | 접기 상태 `navCollapsed` 저장 |
| MAIN-032 | 목록 화면은 보일 때만 결과를 처리 | P1 | 중 | Wails 이벤트 구독을 화면 마운트 동안만 유지하면 됨 |
| MAIN-040 | (상충) 툴바 로고 그림 높이 22 | P1 | 하 | 상충: 041 FR-003 은 워드마크를 글자 부품으로. Wails 는 SVG 인라인(테마 색 상속)로 둘 다 충족 |
| MAIN-041~043 | 망 이름 배지, 사용자 이름(누르면 권한 팝업), 로그인 전 언어 콤보 / 로그인 뒤 [로그아웃]·테마 단추 | P1 | 하 | 배지 글자는 `fmenv.ini` `NAME` |
| MAIN-044 | [로그아웃] → 확인 창(설정 켬) 또는 즉시 처리 | P1 | 하 | |
| MAIN-050~053 | 화면 명령 한 곳 분배, 확인 창 하나(닫힘 콜백), 연결 끊김 창, 로그아웃 시 열린 팝업 전부 닫기 | P1 | 중 | Go → 화면 이벤트 하나(`ui:cmd`)와 Svelte 스토어 하나로 같은 구조 |
| MAIN-054~058 | 알림 띠: 하단 가운데·4초 자동 숨김·새 안내가 바꿔치기·심각도 4종·쌓임 순서 서랍0<대화상자1<띠2 | P1 | 하 | |
| MAIN-059, MAIN-060 | 확인 창 디자인(반경 10·굵은 제목·오른쪽 아래 단추 최소 80), 중복 확인·압축 비밀번호·새 폴더 창도 같은 틀 | P1 | 하 | 공통 `Dialog.svelte` 하나 |
| MAIN-061 | 나머지 창(비밀번호 변경·로그아웃 확인 등) 통일(보류 076) | (미구현·보류 묶음) | | Wails 는 처음부터 공통 틀 하나라 자연 해소 |
| MAIN-070~072 | 로그인 때 트레이 아이콘 생성, 두 번 누름/[열기]=창 보이기, [종료]=앱 끝 | P1 | 중 | Windows 우선. macOS 는 메뉴 막대 아이콘, Linux 는 데스크톱 환경 따라 안 보일 수 있음(StatusNotifier) |
| MAIN-073 | 트레이 [정책 받아오기] → 정책 4종 재조회 | P2 | 중 | 3장 RPC 4개 |
| MAIN-074, MAIN-075 | 로그아웃 방식 "트레이로 내리기"=창 숨김(로그인 유지), "프로그램 종료"=정리 후 종료 | P1 | 중 | |
| MAIN-076 | 닫기(X)=앱 종료 | P1 | 하 | 원본 그대로 둔다(8장의 개선안은 선택 사항) |
| MAIN-077 | 최소화는 운영체제 기본 | P1 | 하 | |
| MAIN-078 | 결재 요청·완료 알림을 트레이 풍선(5초)으로도 | P2 | 중 | Wails v3 알림 서비스(`notifications`) 또는 트레이. 결재 영역과 함께 |
| MAIN-079 | 두 번째 실행 시 「snB App 프로그램이 이미 실행중입니다.」 | P1 | 중 | Wails v3 `SingleInstance` 옵션. 원본 결함(숨긴 창 안 올림)은 고친다(8장) |
| MAIN-080 | 앱 아이콘, 데스크톱 파일 이름 `com.sqisoft.mosfm` | P1 | 하 | Linux `.desktop` 이름은 비고: 원본 값 유지(사용자 설정 연속성) |
| MAIN-090 | 로그인 때마다 클립보드 전역 단축키 두 개 등록 | P2 | 상 | 클립보드 영역과 함께. Linux 는 원본도 숨김 |
| MAIN-091 | 앱 안 단축키는 탐색기 끌기 중 Escape 하나 | P1 | 하 | Wails 는 표준 단축키(F5 새로고침, Ctrl+A, Delete 아님)를 더해도 됨(8장) |
| MAIN-100, MAIN-101 | [탐색기] 토글로 패널 열기, 열 때 「바탕 화면」·「내 컴퓨터」 펼침·시작 폴더 표시 | P1 | 중 | |
| MAIN-102, MAIN-103 | 트리: 바탕 화면 > 내 문서 / 내 컴퓨터 > 드라이브, 드라이브 목록은 시작 때 한 번 | P1 | 중 | macOS·Linux 는 볼륨(`/Volumes`, 마운트 지점). Wails 는 드라이브 변경을 반영하는 쪽이 낫다(8장) |
| MAIN-104, MAIN-105 | 목록 폴더가 바뀌면 트리를 그 노드까지 펼쳐 강조, 트리 한 번 누름=열기·두 번=펼침 | P1 | 중 | |
| MAIN-106, MAIN-107 | 목록 4컬럼(이름·크기·유형·수정일), 폴더 먼저, 크기 1024 단위, `yyyy-MM-dd hh:mm` | P1 | 하 | |
| MAIN-108, MAIN-109 | 작업 스레드에서 읽기·늦은 옛 응답 버림, 「폴더를 찾을 수 없습니다.」·「폴더를 읽을 권한이 없습니다.」 | P1 | 하 | Go 고루틴 + 요청 번호 |
| MAIN-110, MAIN-111 | 경로 입력 + Enter 이동, 누름/Ctrl/Shift 다중 선택 | P1 | 하 | |
| MAIN-112, MAIN-113 | [업로드] 활성 조건과 실행, 목록 행을 서버 표로 끌어 놓아 업로드 | P1 | 중 | 앱 안 끌기라 Wails 에서도 HTML5 DnD 로 됨 |
| MAIN-114 | 패널 [다운로드] → 고른 서버 파일을 패널 현재 폴더로 | P1 | 하 | 다운로드 본체는 기존 `client.go` |
| MAIN-115 | [새 폴더] 이름 검사와 만들기 | P1 | 하 | 오류 문구 6개(3장) |
| MAIN-116, MAIN-117 | [새로고침], 트리/목록 경계 끌기(기본 190·최소 120) | P1 | 하 | |
| MAIN-118 | 패널 높이·컬럼 폭·정렬 저장 안 함 | P1 | 하 | Wails 는 저장하도록 개선(8장) ― 저장 안 함은 "명세상 구현" 이지만 개선 대상 |
| MAIN-120, MAIN-121 | 첫 로그인 때 드라이브 감시 시작, `__DragFilesDownloding__` 파일이 사용자 데이터 폴더 밖에 생기면 그 폴더로 다운로드(끌어 내기 다운로드) | P2 | 상 | Windows 전용 장치(ReadDirectoryChangesW). 원본 버퍼 해석 결함 있음. Wails v3 의 OS 로 끌어 내기 지원 확인 필요(9장 위험) |
| MAIN-130 | (부분) 목록 표 공통(화면마다 복제) | P1 | 중 | Wails 는 처음부터 공통 표 컴포넌트 하나 |
| MAIN-131~134 | 표 가상화, 컬럼 폭 끌기(최소 50), 점선 행 경계, 헤더 체크박스 3상태 | P1 | 중 | 가상화는 `svelte-virtual` 계열 또는 직접(행 30px 고정이라 간단) |
| MAIN-135 | (부분) 잘린 글자 `…`, 전체 이름 툴팁 없음 | P1 | 하 | Wails 는 `title` 또는 사용자 정의 툴팁으로 전체 이름 제공(개선) |
| MAIN-140~146 | 디자인 토큰 한 곳, 라이트/다크(기본 다크), 기본 컨트롤 색, 강조색·주 단추·선택 행 색, 브랜드 금색 제한, Pretendard, 컨트롤 높이 32/44 | P1 | 하 | 7장 토큰표 |
| MAIN-150~152 | 한국어 원문 + English 번역, 한국어일 때 표준 단추도 한국어, 언어 둘 | P1 | 중 | Qt 표준 단추 번역 문제는 Wails 에 없음(단추를 직접 그림) |
| MAIN-160~162 | 접근성 이름: `toolBar`·`sideMenu`·`topTabBar`, 메뉴 `ListItem`, 알림 띠 `AlertMessage`·확인 창 본문 | P1 | 중 | WebView2 는 ARIA 를 UIA 로 노출. E2E(autoqa) 가 이 이름에 의존하므로 `aria-label`·`role` 을 같은 이름으로 |
| 미구현·보류 | MAIN-009(보이지 않는 뷰 만들지 않기, 보류 024), MAIN-061(나머지 창 디자인 통일, 보류 076) | 둘 다 P1 에 자연 포함 | 하 | Svelte 조건부 렌더링·공통 대화상자로 별도 작업 없이 충족 |

### 2.2 설정(13-settings)

| ID | 기능 | Wails 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| SET-001~005 | 카드 목록(1140 미만 1열·1800 미만 2열·그 밖 3열), 카드 순서, 배치 2 넓은 창에서 제목 접기, 모바일 1열, 기본 카드만 높이 자동 | P1 | 하 | CSS grid `repeat(auto-fill, minmax(530px,1fr))` |
| SET-010 | 테마 아이콘·머리줄 테마 단추로 `isDarkTheme` 뒤집기 | P1 | 하 | |
| SET-011 | (상충) 스킨 고르기 없음 | P1 | 하 | 상충 내용: 이전 세대 MOSFM 에는 스킨 선택이 있었다. Wails 는 라이트/다크 + "시스템 따름" 3택을 권함(8장) |
| SET-012~015 | [배치] 콤보(0/2), 목록 밖 값은 0 으로 보임, 즉시 적용, 기본 2 | P1 | 하 | 원본은 콤보가 저장된 1 을 0 으로 다시 써 버림(열린 질문). Wails 는 화면이 저장값을 몰래 바꾸지 않게 한다 |
| SET-016~021 | [언어] 콤보, 로그인 전 머리줄 콤보와 같은 값, 다음 실행 로그인 화면부터 적용, 영어 번역 파일, 실패해도 값은 1, 번역 없는 글자는 한국어 | P1 | 중 | Go 쪽 메시지(오류 문구)도 번역 필요 ― 원본은 C++ `tr()` 로 번역 |
| SET-024 | 영어일 때 메뉴 폭 250 | P1 | 하 | |
| SET-025 | [로그 저장] 켜면 `log` 폴더를 만들고 켬, 끄면 폴더 지움 | P1 | 중 | Wails 판은 지금 로그를 파일로 남기지 않음 ― 로그 영역과 함께 |
| SET-026 | 로그 수준·보관·상한은 `fmlocal.ini` `[LOG]` | P1 | 하 | |
| SET-030~033 | 로그아웃 방식 라디오 3개, 확인 창 켜기, 확인 창에서도 같은 값 변경, 로그인 실패에는 방식 적용 안 함 | P1 | 하 | SET-033 은 반드시 지킨다(적용하면 로그인 실패마다 앱이 꺼짐) |
| SET-040~045 | 시작 폴더 칸 읽기 전용·폴더 선택, 비었거나 못 읽으면 바탕 화면→홈→루트, 안내 글자 `바탕 화면 (%1)`·`홈 (%1)`, 탐색기 첫 폴더·파일 선택 창 시작 위치, `fmusr.ini` 의 옛 `initFolder` 무시 | P1 | 하 | |
| SET-050~056 | 자동 업로드·다운로드 카드: 망별 값(`_<n>`), 체크·폴더 선택 독립, 첫 실행 빈 폴더, 실제 동작 조건, 주기 콤보 9개 | P1 | 중 | 저장·화면은 P1, 실제 자동 전송 동작은 자동 전송 영역 |
| SET-057 | 로그인 때 주기 미저장·0 이면 30분으로 저장 | P1 | 하 | 화면 첫 생성 때 콤보가 1 로 다시 쓰는 원본 동작과 엇갈림(8장) |
| SET-058 | 자동 업로드 값 하나라도 쓰면 모든 망의 타이머 재시작 | P1 | 중 | 원본은 [설정] 을 열기만 해도 재시작(열린 질문). Wails 는 값이 실제로 바뀐 망만 |
| SET-059 | 자동 다운로드를 켜 저장하면 30초 뒤 한 번 | P1 | 하 | |
| SET-070, SET-071 | 단축키 [항상사용]/[사용안함], 조합키 콤보 4종 + 글자 하나, 로그인마다 재등록 | P2 | 상 | 원본은 [설정] 을 열 때마다 글자 칸 `onTextChanged` 로 재등록(열린 질문) ― 따라 하지 않는다 |
| SET-072 | Linux 에서 단축키 카드 숨김 | P2 | 하 | Wails 도 Linux(Wayland) 전역 단축키는 불안정 → 같은 판단 |
| SET-076 | [프로그램 정보]: 아이콘·`snB App v6.0 (<버전>)`·`Copyrightⓒ SQIsoft All rights reserved.` | P1 | 하 | 버전은 빌드 때 `-ldflags -X` 로 주입 |
| SET-080, SET-081 | 사용자 설정 저장소(Windows 레지스트리 `HKCU\Software\SQIsoft\snB App`), 0.5초 뒤 저장·종료 때 저장 | P1 | 중 | 저장소 선택은 7.6 |
| SET-082 | 개발 빌드 `SNB_INSTANCE` 로 저장 자리 분리 | P2 | 하 | 여러 개 동시 실행 시험용. Wails 도 같은 환경변수 지원 권함 |
| SET-083~087 | `fmusr.ini` 위치(`%LOCALAPPDATA%\SQIsoft\snB App`), `fmenv.ini` 실행 파일 옆, 없어도 시작(부분), `[USER]` 은 fmusr→fmenv→기본, `fmlocal.ini` `LOC` 망을 먼저 | P1 | 중 | Wails 는 지금 `SNB_APP_DIR` 만 봄 |
| SET-088 | (상충) `fmenv.ini` 옛 자동 키를 `fmusr.ini` 로 옮김, 번호가 1부터라 앱(0부터)과 어긋남 | P2 | 하 | Wails 는 이 이관을 하지 않거나, 하더라도 0부터로 바로잡는다(8장) |
| SET-089 | 설정이 바뀌면 `설정 변경 >>> 키: 이전 → 새` 로그, 비밀 단어 가림 | P1 | 하 | |
| SET-090 | 제거·재설치 때 사용자 설정 보존 | P1 | 하 | NSIS 제거 절차가 `%LOCALAPPDATA%`·HKCU 를 건드리지 않으면 됨 |
| SET-091 | [설정] 은 서버를 부르지 않음 | P1 | 하 | |
| SET-092~094 | 로그 보관에 정책 `remove_logfile` 우선, 정책 `autotrans_step` 이 2 가 아니면 자동 다운로드 안 함, 서버 경로형 자동 다운로드는 서버 경로로 | P1 | 중 | 정책 키는 4장 |
| SET-085, SET-105, SET-106 | (부분) `fmenv.ini` 없음·설정 깨짐에도 기동, WASM 저장 위치 | P1(085·105), 제외(106) | 중 | 106 은 WASM 전용이라 제외. 085·105 는 Wails 에서 "서버 목록이 비었습니다" 안내까지 해 완성 |
| 미구현·보류 | SET-022(번역 빌드 미연결), SET-023(서버 이름 번역 안 함), SET-075(결재 알림 시간, 카드 늘 숨김), SET-100(비활성 창 색), SET-101(창 테두리 없애기), SET-102(배포 설정의 시작 크기), SET-103(트레이에서 시작), SET-104(설정 저장 실패 알림) | 022·104 는 P1 에 포함, 나머지 P2/제외 | 하~중 | 022: Wails 는 번역 카탈로그를 빌드에 넣어 자연 해소. 104: 저장 실패를 알림 띠로(쉬움). 103: Wails 는 창을 숨긴 채 시작(`Hidden:true`)이 쉬워 P2. 100·101·102: 배포 설정 키가 정해지지 않아 제외(요건 출처 확인 필요). 075·023: 제외(원본도 숨김/미정) |

### 2.3 빌드·배포(15-build-deploy)

| ID | 기능 | Wails 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| BLD-001, BLD-002, BLD-004 | 버전 한 곳에서 정하고 세 곳에 표시, 로그인 요청 앱 버전 칸에 빌드 버전 | P1 | 하 | 원본 `project(... VERSION 6.0.0.1)`. Wails 는 `build/config.yml` 의 `info.version` 한 곳 → `-ldflags "-X main.version=..."`·Windows 판 정보 리소스·NSIS 에 함께 |
| BLD-003 | (상충) 설치본 버전 문자열은 따로 `6.0.0.1` | P1 | 하 | Wails 는 한 곳에서 내려받게 해 상충 해소 |
| BLD-005 | 무결성 검사 요청 버전은 `6.0.0.1` 고정 | P1 | 중 | 무결성 영역. 앱 버전과 섞지 말 것 |
| BLD-006~013, BLD-015, BLD-017 | Qt 6.9.3·빌드 구성·빌드 전 앱 종료 대기·헬퍼 ExternalProject·7-Zip 엔진 판 검사·qt.conf 리소스·windeployqt 옵션 | 제외 | - | Qt/CMake 고유. 대응: Go 판 고정(`go.mod`), `wails3 build`, 7-Zip 대신 Go 압축 처리는 압축 영역 판단 |
| BLD-009 | (부분) 빌드 끝 요약(`change :`) | P2 | 하 | Taskfile 에 산출물 해시·크기 출력 정도 |
| BLD-016 | 설치 폴더 `bin\` 구성 | P1 | 중 | Wails 는 exe 하나 + `fmenv.ini`·`fmlocal.ini`·`root_ca.pem`(+ 필요 시 `ssbr_cert.pem` 여부는 서버 영역 확인). DLL·Qt 런타임 불필요. WebView2 런타임 필요(Windows 10 일부) |
| BLD-018, BLD-019 | `fmenv.ini` 를 실행 파일 리소스에도 내장, 개발 폴더 복사 | P2 | 하 | Wails 는 `embed` 로 기본값 내장 가능. 단 설치본 `fmenv.ini` 가 무결성 대상이라 디스크 파일이 우선 |
| BLD-020~025 | 설치본 2단계, 두 설치본은 `fmlocal.ini` `LOC` 만 다름, `-Fmenv` 옵션, 덮어 설치 시 `fmlocal.ini.bak`, 파일 이름 `snB Client App V6.0.0.1-B<yymm.ddHH>.<망>.exe`, makensis 없음 사전 검사 | P1 | 중 | Wails NSIS 템플릿(`build/windows/nsis/`)을 고쳐 망마다 두 번 `makensis -DNETWORK_KEY=...` |
| BLD-026~030 | 설치 전 실행 중 프로세스 종료, `bin` 일반 사용자 쓰기 권한 제거(icacls), `$PROGRAMFILES64\SQISoft\snB App\bin`, 바로가기·제거 항목·HKLM Run 시작 프로그램·탐색기 메뉴(「snB App 파일 업로드」·「snB App 파일 다운로드」)·Win11 옛 메뉴 복원, 제거 시 정리 | P1 | 중 | 탐색기 메뉴는 `--mode upload|download <경로>` 와 짝(단일 실행 IPC 필수). 프로세스 이름이 `FileMoverApp.exe` 가 아니면 원본·Wails 동시 설치 시 서로 못 끈다 → 이름 결정 필요(9장) |
| BLD-031 | 자동 업데이트 동작 안 함(`USE_UPDATE` 꺼짐, `FileMoverUpdater` 는 빌드되지만 실행 안 됨) | 제외 | - | `docs/자동업데이트_미제공_결정.md` 결정을 따른다. 서버 `Update` 스트림 RPC 는 구현하지 않는다 |
| BLD-032 | `FileMoverVersionMaker.exe` 를 설치본에 포함 | 제외 | - | 관리자 도구. 이식 대상 아님(추정: 운영 쪽이 원본 도구를 계속 쓴다) |
| BLD-033 | 앱은 설치 폴더에 쓰지 않음(로그·`fmusr.ini`·임시는 사용자 데이터 자리) | P1 | 하 | 지금 Wails 판은 실행 파일 폴더에서 읽기만 함 ― 쓰기 금지 유지 |
| BLD-034 | 단일 실행(공유 메모리 `MOSFM 6.0`) + `--mode` IPC | P1 | 중 | Wails v3 `SingleInstance{UniqueID, OnSecondInstanceLaunch}` 가 두 번째 실행의 인자를 첫 실행에 넘김. 원본과 동시 실행 금지 여부는 9장 |
| BLD-035~037 | `CMakeCache.txt` 로 개발 빌드 판정, `SNB_INSTANCE`, 개발 빌드는 무결성 실패에도 계속 | P2 | 하 | Wails 는 빌드 태그 `dev`·`production` 으로 판정(`config.IsDev`, 파일 존재 판정보다 안전) |
| BLD-038 | 빌드 끝에 시험 서버 무결성 목록 동기화 | P2 | 중 | 원본 `tools/ca_lib.py` 계열을 그대로 써서 Wails exe 해시 등록. 공유 서버라 e2e 와 충돌 주의(AGENTS.md) |
| BLD-039 | (부분) 운영 서버 해시 등록 절차 없음 | 제외 | - | 운영 절차. 원본과 같음 |
| BLD-040 | SBOM(CycloneDX) | P2 | 하 | `cyclonedx-gomod`·`npm sbom` |
| BLD-045, BLD-046 | (부분) macOS dmg 두 개, Linux 패키지 | P2 | 중 | Windows 먼저. Wails v3 는 macOS `.app`·dmg, Linux AppImage/deb 템플릿 있음 |
| BLD-047 | (부분) WASM 빌드 | 제외 | - | WASM 전용 |
| 미구현·보류 | BLD-042(코드 서명 없음), BLD-043(번역 누락 검사 없음), BLD-014(Linux·macOS 7-Zip 옛 갈래, 보류 068), BLD-041(Qt 동봉 부품 취약점, 보류 080), BLD-044(설치 용량, 보류 046) | 042 P2, 043 P1 에 포함, 014·041·044 는 Wails 에서 소멸 | 중 | 043: 번역 키 누락을 Go 시험으로 막는다. 041: Qt 를 안 써서 해당 없음(대신 WebView2·Go 모듈 취약점 관리). 044: 추정 설치본 15~25MB(Go exe + 프런트 자산) |

---

## 3. 서버 통신

이 영역은 업무 RPC 가 적다. 대부분 로그인 응답과 정책 응답을 "보여 주는" 쪽이다.

### 3.1 쓰는 RPC

| 순서·때 | 원본 함수 | RPC(proto) | 메시지 번호(로그 표기) | 요청 | 응답에서 쓰는 것 |
|---|---|---|---|---|---|
| 로그인 직후(로그인 영역이 이미 함) | `FileMoverGrpc::setProfile2` | `CommonService.GetProfile(ProfileRequest)` | 조회 [3007], 수신 [3008] | `userid` | `profile` 문자열 `k=v,k=v`(EUC-KR) → 4장 키 |
| 로그인 직후, 트레이 [정책 받아오기] | `setExtInfo` | `FileExtensionService.GetExtInfo(ExtInfoRequest)` | [3005]/[3006] | `type = 2`(0 out→in, 1 in→out, 2 all) | 확장자별 허용·결재 프로필 |
| 같음 | `setExtPatt` | `FileExtensionService.GetExtPattern(ExtPatternRequest)` | [3015]/[3016] | `reqMsg = ""` | 확장자 헤더 패턴(16진) |
| 같음 | `setUrlFilter` | `CommonService.GetUrlFilter(UrlFilterRequest)` | [3094]/[3095] | `code = 0`(늘 0) | 링크연동 URL 목록 |
| [비밀번호 변경](권한 팝업 단추) | `changePassword` | `CommonService.ChangePassword` | [3002] | | 로그인·계정 영역 |

- 트레이 [정책 받아오기] 는 위 넷(`setProfile2`·`setExtInfo`·`setExtPatt`·`setUrlFilter`)을 **현재 망**에 차례로 부른다.
  응답은 망마다 정책 한 벌(`PolicySet`)을 통째로 갈아 끼운다(앞 목록 뒤에 덧붙이지 않는다 ― 원본 보안 감사 D-001 의 교훈).
- **로그아웃 RPC 는 없다.** `FileMoverGrpc::logout()` 은 로그인 상태를 내리고, 정책을 비우고, 알림 스트림을 끊고,
  전송을 전부 취소한 뒤 **TCP 연결을 실제로 닫는다**. 서버(NBS)는 gRPC 연결이 끊길 때만 세션을 회수하므로
  (`handleConnEnd → RemoveSessionByToken → LogoutUser`, 원본 주석) 연결을 닫지 않으면 다음 로그인이 중복 로그인이 된다.
  Wails: 로그아웃에서 `grpc.ClientConn.Close()` 를 반드시 부르고, 진행 중 업로드에 `CloseSend()` 를 보내지 않는다
  (원본 주석: 서버가 EOF 를 "다 받았다" 로 보고 잘린 파일을 등록한다 ― 스트림 컨텍스트 취소로 끊는다).
- 세션 암호화 키·비밀번호를 메모리에서 지운다(`fmSecureZero`, `clearPassword`). Go 는 `[]byte` 를 0 으로 덮는다.

### 3.2 로그인 응답 → 권한 팝업([사용자 이름] 누름)

`LoginResponse`(proto `LoginService.proto`)의 필드를 `loginResPolicy::setAuthority` 로 담고 `getAuthority` 로 꺼낸다.
`UserProfileInfo.qml` 의 행과 값의 대응(원본 그대로 옮긴다):

| 화면 행 | 값 출처 | 표시 |
|---|---|---|
| 파일 · `%1 전송`(첫째 = 반입 방향 글자) | `fm_file_down` | 「허용」/「차단」 |
| 파일 · `%1 전송`(둘째 = 반출 방향 글자) | `fm_file_up` | 「허용」/「차단」 |
| 파일 · `%1 결재`(반입) | `fm_otflup_aprv` | 결재 방식 글자 |
| 파일 · `%1 결재`(반출) | `fm_influp_aprv` | 결재 방식 글자 |
| 클립보드 · `%1 전송`(반입) | 응답 `fm_clip_up` | 「허용」/「차단」 |
| 클립보드 · `%1 전송`(반출) | 응답 `fm_clip_down` | 「허용」/「차단」 |
| 클립보드 · `%1 결재`(반입/반출) | `fm_otclup_aprv` / `fm_inclup_aprv` | 결재 방식 글자 |
| 최대전송크기 | `fm_file_size`(KB) × 1024 | 0 → 「무제한」, 그 밖 크기 글자 |
| 최대전송개수 | `fm_file_count` | 0 → 「무제한」 |

- 방향 글자 `%1` 은 `fmenv.ini` 의 `NETWORK` 값이다(반입 = LOC 0 서버의 NETWORK, 반출 = LOC 1 서버의 NETWORK).
  시험 PC 의 `build\release\fmenv.ini` 는 `NETWORK=업무망->인터넷망`·`NETWORK=인터넷망->업무망` 이라 화면에
  「업무망->인터넷망 전송」 처럼 나온다.
- **클립보드 필드는 뒤바뀌어 있다**: `getAuthority` 가 `clipboardUpload = fm_clip_down`, `clipboardDownload = fm_clip_up` 으로 꺼낸다
  (`loginrespolicy.cpp:174-175`). proto 주석은 `fm_clip_down` = "내부->외부", `fm_clip_up` = "외부->내부". 화면 결과는 맞게 보이도록 짜여 있으나
  이식할 때 필드 이름만 보고 "고치면" 화면이 뒤집힌다. **원본 화면 결과를 기준으로 시험을 먼저 만든다.**
- 결재 방식 글자: 0 「자동 결재」, 1 「사전 결재」, 2 「확장자결재」, 3 「사후 결재」, 4 「요청시 사전사후결정」, 5 「요청시 결재생략결정」,
  6 「자동자가결재」, 7 「자동,사전,사후」, 그 밖 「Unknown(n)」.
- 크기 글자(`getFileSizeToText`): 0 → `0 Byte`, 1GB 초과 `%.2f GB`, 1MB 초과 `%.2f MB`, 1KB 초과 `%.1f KB`, 그 밖 `%d byte`.
  (탐색기 크기 글자와 규칙이 다르다 ― 7.3)
- 사용자 이름(`user_name`)은 EUC-KR 바이트로 오므로 디코딩한다(Go `golang.org/x/text/encoding/korean`).
  추정: Wails `client.go` 가 이미 로그인에서 이름을 디코딩하는지 확인 필요(지금 화면은 이름을 크게 쓰지 않음).
- 역할: 결재자 = `aprv_yn`, 대리결재자 = `replace_yn`. 메뉴 고르기(MAIN-020)에 쓴다. 원본은 이 둘을 `AppSettings.isApprover`·`isProxyApprover`
  (레지스트리)에도 적는다 ― Wails 는 세션 상태로만 두고 저장하지 않는 편이 맞다(서버 값을 디스크에 남길 이유 없음).

### 3.3 이 영역이 받는 화면 명령(원본 `qmlSignal` → Wails 이벤트)

| 명령 | 언제 | 원본 문구(그대로) | 화면 동작 |
|---|---|---|---|
| `SESSION_TIMEOUT` | 자동 로그아웃(알림) | 제목 「자동 로그아웃」, 본문 「%1분 동안 사용하지 않아 자동으로 로그아웃했습니다.」 | 먼저 로그아웃(로그아웃 처리 방식 적용 안 함), 그다음 안내 창 |
| `GRPC_DISCONNECTED` | 로그인 상태에서 스트림 비정상 종료 | 제목 「연결 끊김」, 본문 「서버와의 통신이 끊어졌습니다.\n다시 로그인 해주세요.」 | [확인]·닫기 모두 `logoutProcess`(처리 방식 적용) |
| `LOGIN_FAILED` | 로그인 실패·로그인 중 끊김 | (로그인 영역) | 로그아웃 화면으로, 처리 방식 적용 안 함 |
| `DISCONNECTED` | | | `logoutProcess` |
| `INTEGRITY_FAILED` | 무결성 실패 | (무결성 영역) | 화면을 로그아웃 상태로 |
| 중복 로그인 | 다른 PC 로그인 | 제목 「중복 로그인」, 본문 「다른 PC에서 로그인하여 현재 PC의 연결을 종료합니다.」 | 창 닫힘 뒤 앱 종료 |
| `MESSAGE_BOX` | 일반 안내 | 기본 제목 「정보」 | 확인 창 하나, 떠 있으면 본문만 바꿈 |
| `NOTIFY` | 가벼운 안내 | | 알림 띠(심각도 0~3) |
| `DOWNLOAD_DUPLICATE` | 받을 폴더에 같은 이름 | 제목 「파일 중복 확인」, 「중복된 파일이 존재합니다. 덮어쓰시겠습니까?」, 체크 「모두 적용」, 단추 「덮어쓰기」(0)·「이름 변경」(1)·「취소」(2) | 답 없이 닫히지 않음, 파일 이름 3줄 제한 |
| `ZIP_PASSWORD_ASK` | 암호 압축 | 제목 「압축 비밀번호 입력」, 「압축 파일의 비밀번호를 입력하세요.」, 재시도 「비밀번호가 틀렸습니다. 다시 입력하세요.」, 자리표시 「비밀번호」, 「확인」·「취소」 | |
| 서버 접속 실패 | 망 바꾸기 | 제목 「오류」, 「서버 접속에 실패했습니다. - %1」 / 제목 「서버 접속 실패」, 「서버에 접속하는데 실패했습니다.」 | |

그 밖에 `LINKURL_VIRUS`·`PASSWORD_CHANGE_POP`·`FILE_CHECK_DLG_POP`·`APROVER_POP` 은 각 업무 영역이 주인이다(여기서는 분배만).

### 3.4 로컬 처리 문구(서버 아님, 그대로 옮길 것)

- 파일·폴더 선택 창 제목: 「파일을 선택해 주세요.」, 「디렉토리를 선택해 주세요.」
- 탐색기: 「폴더 경로를 입력하고 Enter」(자리표시), [업로드] 툴팁 「선택한 파일 %1개 / 폴더 %2개를 업로드합니다.」/「업로드할 파일이나 폴더를 선택해 주세요.」,
  [다운로드] 툴팁 「위 목록에서 선택한 서버 파일을 현재 폴더로 다운로드합니다.」, [새 폴더] 툴팁 「현재 폴더 아래에 새 폴더를 만듭니다.」, 「새로고침」,
  컬럼 「이름」·「크기」·「유형」·「수정일」, 빈 상태 「폴더를 선택해 주세요.」/「이 폴더에는 항목이 없습니다.」, 「다시 시도」,
  읽는 중(300ms 뒤) 「폴더를 읽고 있습니다...」, 상태 줄 「%1개 항목 · 파일 %2개 / 폴더 %3개 선택」/「%1개 항목」,
  끌기 그림 「%1 외 %2개」, 상위 행 「상위 폴더」, 접근성 이름 「탐색기 폴더 트리」·「탐색기 파일 목록」·「%1 선택됨」.
- 읽기 오류: 「폴더를 찾을 수 없습니다.」, 「폴더를 읽을 권한이 없습니다.」
- 새 폴더 창: 제목 「새 폴더」, 기본 이름 「새 폴더」, 「만들 위치: %1」, 자리표시 「폴더 이름」, 「확인」·「취소」.
  오류: 「먼저 폴더를 선택해 주세요.」, 「폴더 이름을 입력해 주세요.」, 「폴더 이름에 다음 문자는 쓸 수 없습니다.\n\\ / : * ? " < > |」,
  「사용할 수 없는 폴더 이름입니다.」, 「같은 이름의 폴더나 파일이 이미 있습니다.」, 「폴더를 만들 수 없습니다. 쓰기 권한을 확인해 주세요.」
- 유형 글자: 「바로 가기」, 「폴더」, 확장자의 MIME 설명, 「%1 파일」(확장자 대문자), 「파일」
- 트리: 「바탕 화면」, 「내 문서」, 「내 컴퓨터」
- 단일 실행: 제목 「정보」, 「snB App 프로그램이 이미 실행중입니다.」
- 트레이 메뉴: 「열기」, 「정책 받아오기」, 「종료」. 툴팁 원본은 "FileMover"(결함 의심, 8장)
- 로그아웃 확인 창: 제목 「로그아웃 처리 방식 확인」, 묶음 「처리 방식」, 「로그아웃」·「프로그램 종료」·「트레이로 내리기」,
  체크 「다음부터 보이지 않기(설정에서 변경 가능)」
- 머리줄: 「로그아웃」, 「라이트 테마로」/「다크 테마로」, 메뉴 「메뉴 접기」/「메뉴 펴기」, 「메뉴」
- 설정: 카드 [기본 설정]·[로그아웃 처리]·[시작 폴더 지정]·[자동 업로드 설정]·[자동 다운로드 설정]·[클립보드 단축키 설정]·[프로그램 정보],
  「왼쪽 메뉴 + 상단 툴바」·「상단 탭 (메뉴 없음)」, 「로그 저장」, 「로그아웃 시에 확인창 볼 수 있게」, 「바탕 화면 (%1)」·「홈 (%1)」, 「폴더 선택」,
  「자동 업로드 사용」·「업로드 주기(분)」·「업로드 폴더」, 「자동 다운로드 사용」·「폴더」, 「항상사용」·「사용안함」,
  「Copyrightⓒ SQIsoft All rights reserved.」
- 권한 팝업: 「파일」·「클립보드」·「%1 전송」·「%1 결재」·「최대전송크기」·「최대전송개수」·「허용」·「차단」·「무제한」·「비밀번호 변경」
- 정책 화면(만들 경우): 「조회할 단어」, 「조회」, 컬럼 「#」·「유형」·「명칭」·「값」·「설명」, 「조회된 데이터가 없습니다.」

---

## 4. 정책 키와 설정 키

### 4.1 서버 정책(GetProfile `profile` 의 k=v) ― 이 영역이 읽는 것

| 키 | 뜻 | 없을 때 | 쓰는 곳 |
|---|---|---|---|
| `remove_logfile` | 로그 보관 일수(0 = 지우지 않음) | `fmlocal.ini [LOG] KEEP_DAYS`(없으면 15) | `deleteOldLogs`(로그인 뒤) |
| `autotrans_step` | 자동 전송 단계: 0 미사용, 1 업로드만, 2 전체 | 추정: 자동 다운로드 안 함(2 가 아니면 안 함, SET-093) | 자동 다운로드 |
| `autodown_path` | 서버 경로형 자동 다운로드의 서버 지정 경로 | 사용자 폴더 | SET-094 |
| `print_relay` | 출력 중계 관련(13-settings 의 서버 정책 키 표에 나옴) | | 추정: 이 영역에서는 표시 안 함 |
| `ext_check_method` | `white`/`black`/`gray`, 그 밖 black | black | 정책 화면 "허용/차단" 표시, 업로드 검사 |
| `url_check_method` | `black`(대소문자 무시)/그 밖 white | white | 링크연동 |
| `fm_checksum` | 업로드 검사값 사용, 키 없으면 켜짐 | 켜짐 | 업로드(이미 Wails 구현) |

읽기 규칙(원본 README): 없는 키를 만들지 않는다(`value()` 로만 읽는다). "0" 과 "키 없음" 이 다른 뜻인 키는 존재 여부를 먼저 본다.
Go 는 `map[string]string` + `v, ok := m[k]` 로 자연스럽게 구분된다.

### 4.2 로그인 응답 필드 ― 이 영역이 읽는 것

`fm_file_down`·`fm_file_up`·`fm_clip_down`·`fm_clip_up`(0 차단/1 허용), `fm_influp_aprv`·`fm_inclup_aprv`·`fm_otflup_aprv`·`fm_otclup_aprv`(결재 방식 0~7),
`fm_file_size`(KB, 0 무제한)·`fm_file_count`(0 무제한)·`fm_clip_size`(KB), `fm_logout_auto`(자동 로그아웃 분 ― 로그인 영역),
`aprv_yn`(결재자)·`replace_yn`(대리결재자), `user_name`(EUC-KR), `pswd_remain_day`(비밀번호 변경 잔여일 ― 계정 영역).

### 4.3 사용자 설정(원본 `AppSettings`, Windows 레지스트리 `HKCU\Software\SQIsoft\snB App`)

| 키 | 기본값 | 뜻 | Wails 처리 |
|---|---|---|---|
| `serverIndex` | 0 | 마지막으로 고른 망 | 유지 |
| `isUserIdSave` | true | 아이디 저장 | 로그인 영역 |
| `userId` | "" | 저장된 아이디 | 로그인 영역 |
| `isADLogon`, `domainPolicy` | false, 0 | AD 로그인·도메인 정책 | 로그인 영역 |
| `logLevel` | 5 | 쓰지 않음 | 옮기지 않음 |
| `isApprover`, `isProxyApprover` | false | 역할(서버 값 사본) | 옮기지 않음(3.2) |
| `platform` | "Windows" | | 옮기지 않음(`runtime.GOOS`) |
| `isDarkTheme` | true | 다크 테마 | 유지. "시스템 따름" 추가 권함 |
| `navCollapsed` | false | 왼쪽 메뉴 접힘 | 유지 |
| `layoutIndex` | 2 | 0 왼쪽 메뉴, 2 상단 탭(1 은 0 으로 그림) | 유지 |
| `languageIndex` | 0 | 0 한국어, 1 English | 유지 |
| `useLogFile` | false | 로그 저장 | 유지(로그 폴더 존재가 실제 스위치) |
| `logStorageDays` | 15 | (화면 없음) | 옮기지 않음, `fmlocal.ini` 사용 |
| `logoutType` | 0 | 0 로그아웃, 1 종료, 2 트레이 | 유지 |
| `logoutConfirmPopup` | true | 로그아웃 확인 창 | 유지 |
| `initFolder` | "" | 시작 폴더 | 유지 |
| `lastExportFolder` | "" | 엑셀 내보내기 마지막 폴더 | 전송내역 영역 |
| `transferDrawerHeight` | 0 | 전송 상태 서랍 높이 | 전송 영역 |
| `clipboardHotkeyUse` | false | 단축키 사용 | 유지 |
| `clipboardCopyKey`/`clipboardCopyChar` | "ALT"/"C" | 올리기 단축키 | 유지 |
| `clipboardPasteKey`/`clipboardPasteChar` | "ALT"/"V" | 붙이기 단축키 | 유지 |
| `approvalAlarmUseType`/`StartTime`/`EndTime` | 0/8/20 | 결재 알림 시간(화면 숨김) | 옮기지 않음 |

### 4.4 `fmusr.ini`(`%LOCALAPPDATA%\SQIsoft\snB App\fmusr.ini`) `[USER]`

| 키 | 뜻 | 기본 |
|---|---|---|
| `autoUploadUse_<n>` | 망 n(0부터, `serverIndex`) 자동 업로드 사용 | false |
| `autoUploadPeriod_<n>` | 주기(분) | 미저장·0 이면 로그인 때 30 |
| `autoUploadPath_<n>` | 업로드 폴더 | "" |
| `autoDownloadUse_<n>` / `autoDownloadPath_<n>` | 자동 다운로드 사용·폴더 | false / "" |
| 단축키 키(`initHotKey` 의 두 번째 출처) | 레지스트리에 없을 때 읽음 | ALT+C / ALT+V |

추정: 정확한 키 이름은 `ViewControllerAuto.cpp` 의 `getAutoFileLoadBool/String` 호출 인자에서 한 번 더 확인할 것(13-settings SET-051 에 적힌 이름을 따랐다).
옛 `initFolder` 키는 읽지도 쓰지도 않는다(SET-044).

### 4.5 `fmenv.ini`(실행 파일 옆, 무결성 검사 대상)

| 구획·키 | 뜻 | 원본 기본 | 비고 |
|---|---|---|---|
| `[SERVER01]`… `IP` | 서버 주소, `_` 로 여러 개 | 비면 목록 끝 | |
| `PORT` | 포트 | **29001**(`FilemoverSettings.cpp:75`) | Wails `client.go:64` 는 **29002**. 실제 배포 파일은 `PORT=29002` 를 적어 두어 지금은 차이가 드러나지 않는다. 키가 빠진 파일에서 갈린다 → 원본 29001 로 맞출지 결정 필요(9장) |
| `NAME` | 망 이름(배지) | "NONAME" | |
| `NETWORK` | 방향 글자(권한 팝업) | "NONAME" | 예 `업무망->인터넷망` |
| `LOC` | 1 업무망, 0 인터넷망 | 잘못된 값 → 1 + 경고 | |
| `[GRPC] TLS` | TLS 사용 | | 로그인 영역 |
| `[USER]` | 사용자 설정 기본값(`fmusr.ini` 가 이김) | | `[USER]` 밖 구획을 `fmusr.ini` 로 덮으려 하면 무시하고 `사용자 설정으로 덮어쓸 수 없는 항목이다(무시한다) >>>` 경고 |

### 4.6 `fmlocal.ini`(`fmenv.ini` 옆, 무결성 목록 밖, 고객 PC 에서 고칠 수 있음)

| 키 | 뜻 | 기본 |
|---|---|---|
| `[NETWORK] LOC` | 이 PC 가 속한 망. 이 망의 서버를 목록 앞에 둔다. 두 설치본의 유일한 차이 | 없으면 -1(모름) |
| `[LOG] LEVEL` | 로그 수준(시작 때 한 번) | DEBUG |
| `[LOG] KEEP_DAYS` | 보관 일수(정책 `remove_logfile` 이 이김) | 15 |
| `[LOG] MAX_MB` | 로그 폴더 상한 | 200 |

### 4.7 환경변수(개발·시험용)

`SNB_INSTANCE`(개발 빌드만, 앱 이름·저장 자리·단일 실행 이름 분리), `SNB_APP_DIR`(Wails 판 전용, 설정 파일 폴더), `SNB_FREEZE_WATCH`(GUI 멈춤 기록),
`SNB_TEST_HOOKS`(IPC 시험 진입점), `SNB_INTEGRITY_ENFORCE`. `QT_LOGGING_RULES` 는 Qt 전용이라 옮기지 않는다.

---

## 5. 화면 동작

### 5.1 창과 배치(`Main.qml`, `HomePage.qml`, `settings/Constants.qml`)

- 창 880×660, 제목 `snB App v6.0`, 배경 `Theme.bg`. 최소 폭 360, 최소 높이 272(모바일 배치에서 720).
- 배치 판정(창 크기 바뀔 때마다):
  - 넓은 데스크톱: 폭 ≥ 880 이고 폭 ≥ 높이
  - 좁은 데스크톱: 647 ≤ 폭 < 880 이고 폭 ≥ 높이 → 왼쪽 메뉴는 아이콘만(폭 48), [메뉴 접기] 숨김
  - 그 밖(세로로 길거나 647 미만): 모바일 배치 → 아래 탭바(56, 좁으면 40), 툴바 39
- 데스크톱 툴바 44. `layoutIndex` 2 면 툴바 아래 상단 탭(높이 40, 아이콘 옆 글자), 그 밖이면 왼쪽 메뉴(위 여백 8).
- 로그인 전에는 늘 0 모양(메뉴 없음, 툴바만).
- 화면 전환: 메뉴 누름은 `replace`(즉시, 애니메이션 없음), 로그인 직후 스택 교체는 투명도 200ms.
- `Constants.pageTitleVisible = !(layoutIndex === 2 && isDesktopLayout)` ― 상단 탭 배치의 데스크톱에서는 각 화면 제목 줄을 숨긴다.

### 5.2 메뉴(`HomePage.qml`, `SideBar.qml`, `BottomBar.qml`)

- 메뉴 모델 셋(이름 / 화면 / 아이콘):
  - 결재자(업무망): 파일전송 `ServerFileView` `menu_serverFile.svg`, 전송내역 `TransHistoryView` `menu_transHistory.svg`,
    결재처리 `ApprovalView` `menu_approve.svg`, 결재내역 `ApprovalHistoryView` `menu_approvalHistory.svg`,
    대리결재자 `ProxyApproverView` `menu_subAprver.svg`, 로그 `LogView` `menu_log.svg`, (정책 `PolicyView` `menu_policy.svg` ― 숨김),
    설정 `SettingsView` `menu_settings.svg`
  - 대리결재자(업무망): 파일전송·전송내역·결재처리·로그·설정
  - 일반 사용자, 그리고 인터넷망(LOC ≠ 1)의 모든 사용자: 파일전송·전송내역·로그·설정
- 고르기: `location === 1 ? (isApprover ? 결재자 : isProxy ? 대리 : 일반) : 일반`
- 왼쪽 메뉴 항목: 높이 44, 반경 6, 폭 189(한국어)/250(그 밖), 글자 14. 현재 항목은 왼쪽 3×18 막대 + `surfaceRaised` 면 + DemiBold + 강조색 아이콘.
  마우스 올림 `surfaceHover`(120ms 색 전환), 키보드 초점 2px 강조색 테두리. 아이콘 전용일 때 0.5초 뒤 툴팁.
- 누름 처리: `currentView` 를 먼저 바꾸고 화면 교체는 `Qt.callLater` 로 미룬다(빠르게 여러 번 눌러도 한 번만 교체). 탭은 즉시.
- 아래 탭바: 균등 폭, 높이 40(아이콘 옆 글자) 또는 56(아이콘 아래 글자), caption 글자, 현재 탭은 강조색 + 위쪽 28×3 막대, 배경 `surface` + 위 1px 테두리.

### 5.3 툴바 오른쪽(`ShellUserArea.qml`)

- 로그인 뒤: 망 배지(높이 28, 알약 모양, `accentSubtle` 채움, `accent` 테두리, DemiBold 13, 라이트 테마 글자색 `accentFillHover`) →
  사용자 이름(Medium 13, 누르면 권한 팝업) → [로그아웃](테두리 없는 단추, `login.svg`) → 테마 단추(「라이트 테마로」/「다크 테마로」, `sun.svg`/`moon.svg`, `objectName: headerThemeToggle`).
- 로그인 전: 언어 콤보(폭 110) → 바꾸면 `LanguageManager.language = AppSettings.languageIndex = 고른 값`. 테마 단추.
- 왼쪽: 로고(높이 22, 왼쪽 여백 18, `images/logo_dark`·`logo_light`).

### 5.4 권한 팝업(`UserProfileInfo.qml`)

- 250×400 팝업, 사용자 이름 아래. Esc·바깥 누름으로 닫힘, 로그아웃하면 자동 닫힘. 넓은 배치에서만 화살표 꼬리.
- 열릴 때마다 `getAuthority` 로 값을 다시 읽는다(3.2 표). 아래에 [비밀번호 변경].

### 5.5 로그아웃 흐름(`HomePage.qml`, `LogoutActionDlg.qml`)

1. [로그아웃] → `logoutConfirmPopup` 이 참이면 「로그아웃 처리 방식 확인」 창(400×270), 아니면 바로 2.
   - 창 안 라디오는 누르는 즉시 `logoutType` 에 저장, 체크 「다음부터 보이지 않기…」 = `!logoutConfirmPopup`. 표준 [Ok] → 2.
2. `logoutProcess`: `logoutType` 0 → 화면 로그아웃 + `FmBusi.logout(serverIndex)`, 1 → `programExit`(통신 정리·모든 창 닫기·종료), 2 → `programTray`(창 숨김만, 로그인 유지).
3. 로그아웃으로 로그인 화면에 돌아갈 때(`onLogedOut`, 로그인 상태였을 때만) 열린 팝업·대화상자·서랍을 전부 닫는다(`closeSessionPopups`).
- 자동 로그아웃·로그인 실패에는 1~2 의 처리 방식을 적용하지 않는다.

### 5.6 알림 띠(`CustomInfoBar.qml`, `Notify.qml`)

- 화면 아래 가운데, 아래에서 32 위. 모달 아님. 4000ms 뒤 사라짐. 떠 있는 동안 새 안내는 바꿔치기 + 시계 재시작(줄 세우지 않음).
- 심각도: 0 정보(`accent`, `info.svg`), 1 성공(`success`), 2 경고(`warning`), 3 오류(`danger`, `error.svg`).
- 배경 `surfaceRaised`, 1px 심각도 색 테두리, 반경 6, 본문 최대 폭 420 줄바꿈, 28px 닫기 단추 「닫기」, 나타남 160ms·사라짐 120ms, 쌓임 순서 대화상자 + 1.

### 5.7 로컬 탐색기(`LocalFileExplorerPanel.qml`)

- [파일전송] 화면 단추 줄 오른쪽 [탐색기] 토글로 화면 아래에 높이 260(고정) 패널을 연다.
- 위 줄: 경로 입력란(「폴더 경로를 입력하고 Enter」) · [업로드] · [다운로드] · [새 폴더] · [새로고침].
- 왼쪽 트리(기본 폭 190, 최소 120, 경계 끌기): 「바탕 화면」(없으면 홈) > 「내 문서」 / 「내 컴퓨터」 > 볼륨. 노드는 펼칠 때 자식을 읽고(GUI 스레드),
  자식 유무는 첫 항목 하나만 들여다본다. 한 번 누름 = 오른쪽에 열기, 두 번 누름·[+] = 펼치기. 오른쪽 폴더가 바뀌면 트리를 그 노드까지 펼쳐 강조.
- 오른쪽 목록: 「상위 폴더」(`..`) 행이 늘 첫 줄, 폴더가 늘 앞, 컬럼 이름·크기·유형·수정일, 헤더 누름으로 정렬(같은 컬럼 다시 누르면 방향 반전).
  누름/Ctrl+누름/Shift+누름 선택. 두 번 누름 = 폴더 열기. 300ms 넘게 걸리면 「폴더를 읽고 있습니다...」, 오류면 문구 + 「다시 시도」.
- 상태 줄: 선택이 있으면 「N개 항목 · 파일 a개 / 폴더 b개 선택」, 없으면 「N개 항목」.
- [업로드]: 고른 것이 있을 때만 켜짐, 파일·폴더를 한 번에 업로드 함수로. 목록 행을 위 서버 표로 끌어 놓아도 같다(끌기 그림 「이름 외 N개」, Esc 로 취소).
- [다운로드]: 위 서버 표에서 고른 파일을 패널의 현재 폴더로.
- [새 폴더]: 창(「만들 위치: %1」, 기본 이름 「새 폴더」) → 검사 → 만들기 → 목록 새로고침.
- 패널 높이·컬럼 폭·정렬은 저장하지 않는다(MAIN-118).

### 5.8 설정 화면(`SettingsScrollView.qml`)

- 카드 530×276(기본 카드만 내용 높이), 폭 1140 미만 1열, 1800 미만 2열, 그 밖 3열.
- [기본 설정]: 테마 아이콘(`themeToggle`), [배치] `layoutCombo`(0/2, 목록 밖 값은 0 자리), [언어] `languageCombo`, [로그 저장](먼저 `setLogFileUse(checked)` 로 폴더를 만들거나 지운 뒤 `useLogFile` 저장).
- [로그아웃 처리]: 라디오 셋 + 「로그아웃 시에 확인창 볼 수 있게」.
- [시작 폴더 지정]: 읽기 전용 `initFolderField`, 비었으면 안내 글자로 실제 열릴 폴더, 돋보기 「폴더 선택」 → 폴더 선택 창(저장값에서 시작) → `initFolder`.
- [자동 업로드 설정]: 「자동 업로드 사용」(값이 바뀔 때만 저장), 「업로드 주기(분)」 1/5/10/15/20/30/60/90/120(체크 꺼지면 비활성, 처음 만들 때 지금 값 다시 저장·목록 밖이면 1),
  읽기 전용 「업로드 폴더」 + `autoUploadFolderButton`. 값은 현재 `serverIndex` 망 것.
- [자동 다운로드 설정]: 「자동 다운로드 사용」 + 「폴더」 + `autoDownloadFolderButton`.
- [클립보드 단축키 설정](Linux 숨김): 「항상사용」/「사용안함」, 올리기·붙이기 조합키 콤보(ALT, SHIFT, ALT+SHIFT, CTRL+SHIFT) + 글자 한 칸. 바꾸면 저장 후 `changeHotKey(use, "MOD+C", "MOD+V")`.
- [결재 알림 시간 설정]: 늘 숨김.
- [프로그램 정보]: 아이콘 40, `snB App v6.0 (6.0.0.1)`, 저작권 줄.

### 5.9 정책 화면(`PolicyScrollView.qml`, 메뉴에서 닿지 않음)

- 유형 콤보 Profile/LinkRedirect/Extension, 「조회할 단어」, [조회]. 컬럼 #·유형·명칭·값·설명(폭 50/180/200/200/800), 명칭·값 정렬(기본 명칭 오름차순).
- 행 만들기(`getPolicyList`): 프로필 키마다 (Profile, 키, 값) / URL 마다 (LinkRedirect, url) / 확장자·패턴마다 (Extension, 확장자, 16진 패턴을 2글자마다 띄움, 「허용」·「차단」).

### 5.10 트레이(`ViewController::setTrayIcon`)

- 로그인 완료 때 아이콘이 없거나 안 보이면 만든다. 아이콘 `FileMover.ico`, 툴팁 "FileMover".
- 두 번 누름 → 창 보이기·올리기·활성화. 한 번 누름은 무시. 메뉴 「열기」·「정책 받아오기」·「종료」.

### 5.11 두 번째 실행(`main.cpp`)

- 공유 메모리 `MOSFM 6.0`(+ `SNB_INSTANCE` 접미사)로 판정, 100ms 간격 5번 재시도(`noupdate` 모드 50번).
- 인자가 `--mode upload|download <경로>` 면 경고 없이 JSON `{"mode":..., "path":...}` 를 로컬 소켓 `FileMoverActivation` 으로 보내고 끝.
  탐색기에서 여러 파일을 고르면 프로세스가 여럿 뜨므로 첫 실행은 400ms 동안 모아 한 번에 올린다(최대 30초 미룸).
- 그 밖이면 「snB App 프로그램이 이미 실행중입니다.」 경고 + "Activate" 전송. 받는 쪽은 **보이는 창만** 올린다(숨긴 창은 그대로 ― 결함 의심).

---

## 6. 경계·오류 상황

- `fmenv.ini` 가 없거나 `[SERVER01] IP` 가 비면 서버 목록이 0개다. 원본은 시작은 하고 로그에만 남긴다(SET-085 부분). Wails 는 로그인 화면에 "서버 설정이 없습니다" 를 보여야 한다.
- `LOC` 가 0/1 이 아니면 1 로 보고 경고. `fmlocal.ini` 가 없으면 집 망 -1 → 서버 순서는 파일 순서.
- `PORT` 키가 빠진 `fmenv.ini`: 원본 29001, Wails 29002 로 서로 다른 포트에 붙는다.
- 같은 IP 칸에 `_` 로 여러 주소: 원본은 나눠 담는다. 추정: Wails `client.go` 는 첫 주소만 쓰는지 확인 필요.
- 시작 폴더가 지워졌거나 권한이 없어졌거나 네트워크 드라이브가 끊김 → 바탕 화면 → 홈 → 루트로 내려간다. 저장값은 지우지 않는다.
- 탐색기에서 빠르게 폴더를 옮겨 다님 → 늦게 온 옛 결과는 버린다(요청 번호 비교).
- 큰 폴더(수만 항목): 원본은 작업 스레드에서 읽지만 모델 전체를 한 번에 넘긴다. Wails 는 IPC 직렬화 비용이 커서 나눠 보내거나 가상 스크롤 + 정렬을 Go 에서 해야 한다.
- `.lnk` 바로 가기는 대상이 아니라 링크 파일 자체 크기, 유형 「바로 가기」. 폴더를 가리키는 `.lnk` 를 두 번 누를 때의 동작은 원본도 특별 처리 없음(추정: 파일로 취급).
- 숨김 파일은 기본 제외, 시스템 파일은 포함(원본 필터 `System`, `showHidden` 일 때만 `Hidden`).
- 새 폴더 이름: 빈 이름, 금지 문자 `\ / : * ? " < > |`, `.`·`..`·예약 이름(추정: `CON`·`NUL`·`COM1` 등은 「사용할 수 없는 폴더 이름입니다.」로 걸린다 ― `tst_LocalFileListModel` 의 `makeDir_*` 5건으로 확인 필요), 같은 이름 존재, 쓰기 권한 없음.
- 드라이브: 원본은 시작 때 한 번만 목록을 만든다 → 실행 중 꽂은 USB 가 트리에 안 나온다(경로 입력으로는 열림).
- 드라이브 감시: 고정·네트워크 드라이브마다 스레드. 감시 중 드라이브가 사라지면 스레드가 끝난다(추정). 원본 버퍼 해석 결함(`while(!Buffer[i].NextEntryOffset)` 를 배열 첨자로 씀)으로 한 번에 여러 변경이 오면 둘째 이후 항목을 잘못 읽을 수 있다.
- 끌어 내기 임시 파일 `__DragFilesDownloding__` 이 사용자 데이터 폴더 밖에 생긴 순간 그 폴더로 받는다 → 다른 프로그램이 같은 이름을 만들면 오작동(이름이 고정이라).
- 로그아웃 방식이 "트레이로 내리기" 인데 트레이 아이콘이 안 보이는 환경(Linux 일부): 창을 되찾을 방법이 두 번째 실행뿐인데 원본은 숨긴 창을 올리지 않는다 → 사실상 앱을 잃는다.
- 닫기(X)는 로그인 중이어도 확인 없이 종료(진행 중 전송도 끊김). 원본 동작이며 바꾸려면 결정 필요.
- 로그인 실패·자동 로그아웃에 로그아웃 방식(종료·트레이)을 적용하면 안 된다(원본 066 의 교훈).
- 중복 로그인 안내가 떠 있는 동안 연결이 끊겨도 안내를 닫은 뒤 종료해야 한다(원본 `m_exitWhenNoticeClosed`).
- 확인 창이 떠 있는데 새 `MESSAGE_BOX` 가 오면 원본은 본문만 바꾼다 → 앞 안내를 못 읽고 지나갈 수 있다(원본 동작). Wails 는 줄 세우기를 고려(8장).
- 언어 바꾸기: 로그인 뒤에는 [설정] 에서만. 원본은 Qt `retranslate()` 로 즉시 다시 그린다. Go 쪽이 이미 만든 문구(알림 띠 등)는 바뀌지 않는다.
- `lang_en.qm` 을 못 읽어도 언어 값은 1 로 저장된다(SET-020) → 영어를 골랐는데 한국어로 보임. Wails 는 카탈로그를 실행 파일에 넣어 이 경우가 없다.
- 레지스트리 쓰기 실패·`fmusr.ini` 쓰기 실패를 알리지 않는다(SET-104 미구현).
- 자동 업로드 주기: 로그인 때 0/미저장 → 30, [설정] 카드가 처음 만들어질 때 목록 밖 값 → 1. 두 규칙의 기본값이 다르다.
- `SNB_INSTANCE` 는 영숫자·`-`·`_` 만, 설치본은 무시.
- 설치: 같은 이름의 프로세스를 전부 끈 뒤 설치한다(다른 사용자 세션 것도 끌 수 있음 ― 추정, `taskkill /F /IM`).
- 단일 실행 판정 공유 메모리가 유령 프로세스(창 없음)로 남으면 새 실행이 경고만 띄우고 끝난다.

---

## 7. 로컬 처리 ― 알고리즘과 Go 구현 후보

### 7.1 설정 파일 읽기(`fmenv.ini`·`fmlocal.ini`·`fmusr.ini`)

- 원본: Qt `QSettings`(INI). 구획 `[SERVER01]` 부터 IP 가 빌 때까지. `[USER]` 은 `fmusr.ini` → `fmenv.ini` → 기본값.
- Go: 지금 `client.go` 의 손 파서를 유지(의존성 추가 없음). 쓰기가 필요한 `fmusr.ini` 만 `gopkg.in/ini.v1` 을 검토하거나, 손 파서에 쓰기를 더한다(키 몇 개라 손 파서로 충분).
- 주의: Qt `QSettings` 는 한글 값을 UTF-8 로 저장하고, 특수 문자(`,` 등)가 든 값을 따옴표로 감싸며, 키의 공백·`%` 를 `%XX` 로 바꾼다. `fmusr.ini` 의 폴더 경로에 `,` 가 있으면 Qt 는
  `"C:\a,b"` 처럼 따옴표로 쓴다 → Go 파서가 따옴표를 벗겨야 원본 파일을 읽는다. 원본이 쓴 파일과 Wails 가 쓴 파일을 서로 읽을 수 있는지 시험한다.
- 주의: BOM 이 있는 `fmenv.ini` 를 견딘다(손으로 고친 파일).

### 7.2 사용자 설정 저장소

| 안 | 장점 | 단점 |
|---|---|---|
| A. 원본과 같은 레지스트리 `HKCU\Software\SQIsoft\snB App`(`golang.org/x/sys/windows/registry`) | 원본에서 옮겨 올 때 설정이 이어짐, E2E 가 레지스트리로 시험 준비하는 방식 유지 | Windows 전용, macOS/Linux 는 다른 코드 |
| B. `%LOCALAPPDATA%\SQIsoft\snB App\settings.json`(`os.UserConfigDir`/`os.UserCacheDir` 대신 `LOCALAPPDATA` 직접) | 한 코드로 세 운영체제, 원자적 쓰기(임시 파일 + 이름 바꾸기) 쉬움 | 원본 설정을 한 번 옮겨 와야 함 |

권장: B 로 저장하되 첫 실행에 A 에서 읽어 옮긴다(Windows 만). Qt 레지스트리 값 형식: bool 은 문자열 `"true"`/`"false"`, 정수는 `REG_SZ` 또는 `REG_DWORD`(추정 ― 실제 값 형식은 `reg query` 로 확인 필요).
쓰기는 원본처럼 0.5초 모아 쓰기(디바운스) + 종료 때 쓰기. 쓰기 실패는 알림 띠로(SET-104 해소).
`설정 변경 >>> 키: 이전 → 새` 로그, 키 이름이 `pw|pass|token|secret` 에 맞으면 값 가림.

### 7.3 탐색기 목록

- 읽기: `os.ReadDir` + `entry.Info()`. 숨김 판정은 Windows `FILE_ATTRIBUTE_HIDDEN`(`syscall.Win32FileAttributeData`), macOS/Linux 는 이름이 `.` 로 시작.
- 정렬: 폴더 먼저, 그다음 자연 정렬(숫자를 수로, 대소문자 무시), 같으면 이름. 원본은 `QCollator`(numericMode, CaseInsensitive, 시스템 로캘).
  Go 후보: `golang.org/x/text/collate`(`collate.Numeric`, `collate.IgnoreCase`, 한국어 로캘) ― 한글 가나다 순까지 맞춘다. 손으로 쓴 자연 정렬은 한글 순서가 코드값 순이라 대부분 같지만 로캘 규칙과 다를 수 있다.
- 크기 글자(탐색기): 1024 미만 「%1 byte」, KB 는 정수, MB/GB/TB 는 소수 한 자리. (권한 팝업의 `getFileSizeToText` 와 규칙이 달라 함수 둘을 따로 둔다 ― 하나로 합치면 화면이 바뀐다.)
- 유형 글자: `.lnk` → 「바로 가기」, 폴더 → 「폴더」, 확장자의 MIME 설명 → 없으면 「%1 파일」(대문자) → 확장자 없으면 「파일」.
  MIME 설명: Qt 는 freedesktop shared-mime-info 의 한국어 설명을 쓴다(예: "PDF 문서"). Go `mime.TypeByExtension` 은 형식 이름(`application/pdf`)만 준다.
  후보: Windows 는 `SHGetFileInfoW(SHGFI_TYPENAME)` 로 탐색기와 같은 글자(가장 자연스러움, `golang.org/x/sys/windows` + `syscall`), 그 밖 운영체제는 「%1 파일」로 단순화.
  추정: 원본 화면과 글자가 달라지므로 화면 비교 시험에서 "유형" 컬럼은 예외로 둔다.
- 날짜: `yyyy-MM-dd hh:mm` → Go `t.Format("2006-01-02 15:04")`(지역 시각).
- 비동기: 요청마다 번호를 올리고 결과에 번호를 실어 화면이 최신 번호만 받는다(원본 토큰 방식 그대로). 300ms 넘으면 읽는 중 표시.
- 큰 폴더: 결과를 Go 에서 정렬해 한 번에 보내되 1만 행 이상이면 페이지로. 아이콘은 화면에서 종류(폴더/파일/바로가기) 세 가지만 그린다.

### 7.4 탐색기 트리·드라이브

- 바탕 화면: Windows `SHGetKnownFolderPath(FOLDERID_Desktop)`(OneDrive 이동 반영 ― `%USERPROFILE%\Desktop` 고정은 틀린다), 문서 `FOLDERID_Documents`.
  Go: `golang.org/x/sys/windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)`. macOS/Linux 는 `~/Desktop`(Linux 는 `xdg-user-dir DESKTOP` 추정).
- 드라이브: `windows.GetLogicalDrives` + `GetDriveType` + `GetVolumeInformation`(이름). macOS `/Volumes/*`, Linux `/proc/mounts` 의 사용자 마운트(추정).
- 드라이브 변경 반영(개선): Windows `WM_DEVICECHANGE` 는 창 메시지가 필요해 Wails 에서 번거롭다 → 트리의 「내 컴퓨터」 를 펼칠 때마다 다시 읽는 것이 가장 단순하다.
- 자식 유무: 디렉터리를 열어 첫 항목 하나만 읽는다(`f.ReadDir(1)`). 네트워크 드라이브에서 느릴 수 있으므로 고루틴 + 시간 제한.

### 7.5 새 폴더

- 검사 순서(원본): 현재 폴더 없음 → 이름 비었음(앞뒤 공백 제거 후) → 금지 문자 → 사용할 수 없는 이름 → 이미 있음 → 만들기 실패.
- Go: `strings.ContainsAny(name, `\/:*?"<>|`)`, 예약 이름·끝의 점/공백 검사(Windows), `os.Mkdir`(이미 있으면 `os.ErrExist`), 권한 오류는 `errors.Is(err, fs.ErrPermission)`.

### 7.6 시작 폴더 결정

`initFolder` 가 있고, 폴더이고, 읽을 수 있으면 그것 → 바탕 화면 → 홈 → 루트(Windows 는 시스템 드라이브). 쓰는 자리(탐색기 첫 폴더, 파일·폴더 선택 창 시작 위치)마다 같은 함수 하나.
Wails v3 파일 대화상자는 `SetDirectory`(시작 위치)를 지원한다(추정: v3.0.0 API 이름 확인 필요).

### 7.7 끌어 내기 다운로드(Windows)

- 원본 장치: 서버 파일을 끌기 시작하면 사용자 데이터 폴더에 빈 파일 `__DragFilesDownloding__` 을 만들고 그 파일을 운영체제 끌기 데이터로 넘긴다 →
  사용자가 탐색기 폴더에 놓으면 탐색기가 그 파일을 복사 → 감시 스레드가 "사용자 데이터 폴더 밖에 그 이름이 생김" 을 보고 그 폴더 경로를 알아낸다 → 복사된 임시 파일을 지우고 그 폴더로 다운로드.
- 감시: 고정·네트워크 드라이브마다 `ReadDirectoryChangesW`(하위 포함, `FILE_NOTIFY_CHANGE_FILE_NAME`). Go 는 `golang.org/x/sys/windows.ReadDirectoryChanges` 로 같은 일을 할 수 있다.
  **버퍼 해석은 `NextEntryOffset` 을 따라 바이트 단위로 걸어가야 한다**(원본 결함을 따라 하지 않는다). `github.com/fsnotify/fsnotify` 는 드라이브 루트 전체 재귀 감시를 지원하지 않아(Windows 백엔드는 재귀 옵션이 실험적) 직접 구현이 낫다.
- 더 나은 안(권장 검토): 운영체제가 끌기의 "놓은 위치" 를 알려 주는 방식(`IDataObject` 에 `CFSTR_FILEDESCRIPTOR`+지연 렌더링, 즉 가상 파일 끌기)이 정석이지만 Wails/WebView2 에서 끌기 시작을
  네이티브로 바꿔 쳐야 해 난이도가 높다. 첫 판은 이 기능을 빼고(P2) [탐색기] 패널의 [다운로드] 와 앱 안 끌어 놓기로 대신한다.
  추정: Wails v3.0.0 은 창 **안으로** 들어오는 파일 끌기(`EnableFileDrop`)만 지원하고 창 **밖으로** 끌어 내기 API 는 없다.

### 7.8 트레이·단일 실행·단축키

- 트레이: Wails v3 `app.SystemTray.New()` + `SetIcon`·`SetTooltip`·`SetMenu`·`OnDoubleClick`(Windows). macOS 는 템플릿 아이콘, Linux 는 StatusNotifier(GNOME 은 확장 필요).
- 단일 실행: Wails v3 `application.Options{SingleInstance: &application.SingleInstanceOptions{UniqueID: ..., OnSecondInstanceLaunch: func(d application.SecondInstanceData){...}}}`.
  두 번째 실행의 인자(`d.Args`)로 `--mode upload|download <경로>` 를 처리하고, 인자가 없으면 창을 `Show()`+`Focus()`(숨김 상태여도). 400ms 묶음 처리는 Go 타이머로.
  UniqueID 를 원본 이름(`MOSFM 6.0`)과 같게 해도 Qt 공유 메모리와는 장치가 달라 서로를 막지 못한다 → 원본·Wails 동시 실행 정책을 정해야 한다(9장).
- 전역 단축키: `golang.design/x/hotkey`(Windows `RegisterHotKey`, macOS Carbon, Linux X11 만). Wails v3 자체에는 전역 단축키가 없다(추정, 앱 안 키 바인딩만).
  주의: `hotkey` 패키지는 macOS 에서 메인 스레드 요구(`mainthread`)가 있어 Wails 이벤트 루프와 충돌 가능 ― Windows 먼저.

### 7.9 다국어

- 원본: 한국어가 원문, `lang/lang_en.ts`(Qt Linguist XML) → `lang_en.qm`. 언어 0 이면 번역기 제거 + Qt 한국어 표준 단추 번역, 1 이면 `lang_en.qm`.
- Wails: `lang_en.ts` 를 한 번 변환해 `frontend/src/lib/i18n/en.json`(원문 한국어 → 영어)으로 만들고, 화면은 `t("파일전송")` 처럼 한국어 원문을 키로 쓴다(원본과 같은 방식이라 문구 대조가 쉽다).
  Go 쪽 오류 문구도 같은 카탈로그(`embed`)를 읽는다. 변환은 `encoding/xml` 몇십 줄. 외부 i18n 라이브러리 불필요.
- 번역 누락 검사(BLD-043 해소): Svelte 소스의 `t("…")` 를 모아 카탈로그에 없는 키를 실패로 하는 Go 시험 하나.

### 7.10 버전·빌드

- 버전 한 곳: `build/config.yml` `info.version` → Taskfile 이 `-ldflags "-X main.appVersion=6.0.0.1"` 와 Windows 판 정보(`wails3 generate syso`)·NSIS `!define INFO_PRODUCTVERSION` 에 함께 넣는다.
- 로그인 `AppVersion` 에 이 값을 보낸다(이미 `client.go` 가 보냄 ― 값의 출처만 바꾼다). 무결성 검사 버전 `6.0.0.1` 은 별도 상수.
- 설치본 두 개: NSIS 스크립트에 `!ifdef NETWORK_INT` 로 `fmlocal.ini`(LOC 1/0)만 바꿔 `makensis` 두 번. 파일 이름 `snB Client App V<버전>-B<yymm.ddHH>.<업무망|인터넷망>.exe`.
- 설치 동작 이식 목록: 프로세스 종료, `icacls` 로 `bin` 의 Users 쓰기 권한 제거, `$PROGRAMFILES64\SQISoft\snB App\bin`, HKLM `InstallDir`·`App Paths`, 바로가기, HKLM `Run`,
  탐색기 메뉴(파일 「snB App 파일 업로드」 → `"…exe" --mode upload "%1"`, 폴더 「snB App 파일 다운로드」 → `--mode download "%1"`), Win11 옛 메뉴 복원 키, 덮어 설치 시 `fmlocal.ini.bak`, 제거 시 사용자 데이터 보존.
- WebView2: Windows 11 은 기본 포함, Windows 10 은 부트스트래퍼를 설치본에 넣거나 설치 중 내려받기. 망분리 PC(인터넷 없음)가 대상이라 **고정 판 또는 오프라인 설치 파일 포함**이 필요하다(9장 위험).

### 7.11 Go 패키지 후보 정리

| 용도 | 후보 | 비고 |
|---|---|---|
| 트레이·단일 실행·대화상자·창·이벤트·알림 | Wails v3 내장(`github.com/wailsapp/wails/v3/pkg/application`) | 이미 의존 |
| Windows API(레지스트리·KnownFolder·드라이브·ReadDirectoryChanges·SHGetFileInfo) | `golang.org/x/sys/windows`, `golang.org/x/sys/windows/registry` | 추정: 이미 간접 의존 |
| 자연 정렬·한국어 콜레이션 | `golang.org/x/text/collate`, `golang.org/x/text/language` | |
| EUC-KR 디코딩 | `golang.org/x/text/encoding/korean` | 로그인·정책 응답 |
| 전역 단축키 | `golang.design/x/hotkey` | P2 |
| INI 쓰기(선택) | `gopkg.in/ini.v1` | 손 파서로 충분하면 생략 |
| 파일 감시(선택) | `github.com/fsnotify/fsnotify` | 자동 업로드 폴더 감시에는 적합, 드라이브 전체 감시에는 부적합 |
| SBOM | `github.com/CycloneDX/cyclonedx-gomod` | 빌드 도구 |
| Windows 판 정보·아이콘 | `wails3 generate syso`(내부 `go-winres` 계열) | |

---

## 8. Wails 에서 더 잘할 수 있는 것

### 8.1 성능

- **화면 전환**: 원본은 메뉴를 누를 때마다 QML 화면을 새로 만든다(Debug 에서 60배 느린 원인). Svelte 는 컴포넌트를 유지(`{#key}` 없이 `display` 전환)하고 데이터만 다시 조회 → 전환이 즉시.
  단 "들어올 때 목록 새로 조회" 는 지켜야 원본과 같은 최신성.
- **GUI 멈춤 없음**: 원본은 렌더 스레드가 GUI 스레드와 같아(`QSG_RENDER_LOOP=basic`) 동기 호출 하나가 화면 전체를 멈춘다. Wails 는 Go 호출이 비동기라 구조적으로 막힌다.
  트리 자식 읽기(원본은 GUI 스레드)도 고루틴으로.
- **탐색기 큰 폴더**: 정렬을 Go 에서, 가상 스크롤로 보이는 행만. 원본 `QCollator` 비교보다 `collate.Key` 를 한 번 계산해 두고 비교하면 수만 항목도 빠르다.
- **설치본 크기**: Qt 런타임 129.7MB 설치 → 추정 20MB 안팎. 업데이트·배포 부담이 줄어든다.
- **시작 시간**: Qt QML 엔진 + 폰트 등록보다 Go exe + WebView2 가 대체로 빠르다(추정, 같은 PC 에서 측정해 비교할 것).

### 8.2 사용성

- 긴 이름 툴팁(MAIN-130·135 부분 해소): 잘린 칸에만 전체 이름 툴팁.
- 탐색기 패널 높이·컬럼 폭·정렬·트리 폭 저장(MAIN-118). 패널 높이 고정 260 대신 끌어서 조절.
- 드라이브 목록을 펼칠 때마다 갱신(MAIN-103).
- 두 번째 실행·트레이 숨김 상태에서도 창을 올린다(원본 결함 ― 숨긴 창은 `activateWindows` 가 건너뜀).
- 트레이 툴팁을 "FileMover" 가 아니라 `snB App`(+ 로그인한 망 이름)으로(원본 열린 질문, 이전 세대 이름이 남은 것).
- 테마 "시스템 따름"(Windows 앱 모드 따라감) 추가. 기본은 원본대로 다크.
- 키보드: 탐색기에서 F5 새로고침, Enter 열기, Backspace 상위, Ctrl+A 전체 선택, F2 는 이름 바꾸기 기능이 없으므로 넣지 않음.
- 확인 창 바꿔치기 대신 줄 세우기(앞 안내를 못 읽고 지나가는 문제). 단 원본 E2E 가 "같은 창 본문 바뀜" 을 전제할 수 있어 autoqa 시나리오 확인 후 결정.
- 닫기(X)를 로그인 중에 누르면 진행 중 전송이 있을 때만 확인(원본은 바로 종료). 결정 필요 ― 기본은 원본 유지.
- 설정 저장 실패를 알림 띠로(SET-104).
- 모바일 배치 최소 높이 720 을 창 최소값으로 강제하지 않는다(창을 낮게 줄일 수 없는 불편).

### 8.3 피해야 할 원본 결함·상충

| 원본 항목 | 내용 | Wails 에서 |
|---|---|---|
| MAIN-002 | 최소 폭 식이 늘 360(480 을 의도) | 의도를 확인하지 못했으므로 360 유지, 식은 옮기지 않음 |
| MAIN-007(상충) | 상단 탭 배치에서 제목 줄 숨김 vs "제목으로 지금 화면을 안다" | 탭 강조 + 창 제목 부제 |
| MAIN-040(상충) | 로고 그림 vs 글자 워드마크 | 테마 색을 따르는 SVG |
| 드라이브 감시 버퍼 해석 | `while(!Buffer[i].NextEntryOffset)` 를 배열 첨자로 씀 → 한 번에 여러 변경 시 오판 | `NextEntryOffset` 을 바이트 오프셋으로 따라가기 |
| 숨긴 창 재실행 | 두 번째 실행이 숨긴 창을 안 올림 | 올린다 |
| 트레이 툴팁 "FileMover" | 이전 세대 이름 | `snB App` |
| SET-011(상충) | 이전 세대에는 스킨 선택이 있었음 | 라이트/다크/시스템 3택으로 정리 |
| SET-012~013 | [배치] 콤보가 저장된 1 을 0 으로 다시 써 버림 | 화면이 저장값을 몰래 바꾸지 않음(사용자가 고를 때만 저장) |
| SET-056~057 | 주기 기본값이 로그인 때 30, 카드 생성 때 1 로 엇갈림. 카드가 만들어질 때 값을 다시 저장 | 기본값 30 하나, 화면이 열릴 때 저장하지 않음 |
| SET-058 | [설정] 을 열기만 해도 모든 망의 자동 업로드 타이머 재시작 | 값이 바뀐 망만 재시작 |
| SET-070~071 | 단축키 글자 칸 `onTextChanged` 가 [설정] 을 열 때마다 단축키 재등록 | 값이 바뀔 때만 재등록 |
| SET-088(상충) | `fmenv.ini` → `fmusr.ini` 이관이 1부터 번호, 앱은 0부터 | 이관하지 않거나 0부터. 추정: 이관 대상 고객이 없다(원본 메모리 "고객사 없음") → 이관 생략 권함 |
| BLD-003(상충) | 설치본 버전 문자열을 따로 적음 | 한 곳에서 내려받음 |
| 클립보드 권한 필드 뒤바뀜 | `getAuthority` 가 `fm_clip_down`/`fm_clip_up` 을 바꿔 꺼냄 | 원본 화면 결과를 시험으로 고정한 뒤 깔끔한 이름으로. 서버 의미가 proto 주석과 같은지 서버 담당 확인 전에는 화면 결과만 맞춘다 |
| PORT 기본값 | 원본 29001, Wails 29002 | 원본에 맞추거나 차이를 문서화(9장) |
| `AppSettings.isApprover` 등 저장 | 서버 역할을 레지스트리에 남김 | 세션 상태로만 |
| 자동 로그인 비밀번호 파일 `fmx.dat` | 원본 104 에서 폐지 | 만들지도 읽지도 않음 |

---

## 9. 실 서버 검증 방법

비밀번호는 이 문서에 적지 않는다. 계정은 원본 저장소의 `tools/.secrets/login*.json`(개발 PC 로컬, git 밖)에 역할별로 있고, 원본 E2E 는 `Get-SnbCredential -Role <역할>` 로 읽는다.
Wails 시험(`live_test.go`)은 환경변수 `SNB_LIVE_ID`·`SNB_LIVE_PW` 로 받는다.

### 9.1 계정과 역할

| 역할 | 원본 표기 | 이 영역에서 보는 것 |
|---|---|---|
| 일반 사용자 | `user`(자리 N 의 `test<N>_1` 계열 ― 원본 `tools/e2e/README.md` "역할" 표) | 메뉴 4개(파일전송·전송내역·로그·설정), 권한 팝업 값 |
| 결재자 | `approver`(`test<N>_2`, 결재자 지정) | 업무망 메뉴 7개(정책 숨김) |
| 대리결재자 | `proxyApprover`(`test<N>_3`) | 메뉴 5개. 결재자가 대리로 지정했을 때 `replace_yn` 이 참인지 |
| autoqa 계정 | valid·requester·approver·proxy·sec_user(권한 프로파일 'E2E-QA') | autoqa 시험이 도는 시각에는 쓰지 않는다(AGENTS.md "공용 자원") |

- 시험 계정은 잠겨도 원본 도구(`srv_settings.py unlock`)로 푼다. 업무망 DB 확인은 `.157` 쪽(인터넷망 `.158` 에는 mysql 없음).
- 서버 정책을 바꾸는 시험은 시험 계정의 전용 권한 프로파일에만 건다(공유 서버).

### 9.2 서버 설정

- `fmenv.ini`: `SNB_APP_DIR=E:\git\snBv6FM\build\release` 로 원본 개발 빌드의 파일을 그대로 쓴다(업무망 LOC 1·인터넷망 LOC 0 두 서버, `PORT=29002`).
- `fmlocal.ini`: 같은 폴더. 없으면 집 망 판정이 -1 이므로 업무망 설치본 흉내를 내려면 `[NETWORK] LOC=1` 파일을 둔다.
- 무결성: Wails exe 는 서버 `file_hash` 에 없으므로 로그인 직후 무결성 검사(유형 0)가 실패한다. 원본 개발 빌드처럼 "개발 빌드는 종료하지 않음" 을 Wails 에도 두거나,
  시험 서버에 Wails exe 해시를 등록한다(원본 `build-release.cmd` 의 동기화 절차 참고). autoqa 실행 직전(before_run)에 서버 해시가 QA 설치본으로 바뀌므로 그 시간대는 피한다.
- 중복 로그인: 같은 계정으로 원본 앱이 떠 있으면 Wails 로그인이 중복 로그인이 된다 → 원본 앱 자리(`tools\snb.cmd who`)를 먼저 본다.

### 9.3 시험 항목(실행 가능한 판정)

| 항목 | 절차 | 통과 기준 |
|---|---|---|
| 역할별 메뉴 | 일반·결재자·대리결재자로 업무망 로그인, 인터넷망에도 결재자로 로그인 | 메뉴 이름·순서가 5.2 와 같다. 인터넷망은 늘 4개 |
| 권한 팝업 | 같은 계정으로 원본 앱과 Wails 에서 팝업을 열어 비교 | 9개 행의 글자가 원본과 한 글자도 다르지 않다(특히 클립보드 두 행) |
| 망 배지·방향 글자 | 로그인 후 툴바 | 배지 = `NAME`, 팝업 방향 = `NETWORK` 값 |
| 로그아웃 세 방식 | 방식 0/1/2 각각 | 0: 로그인 화면 + 서버에 세션이 남지 않아 곧바로 다시 로그인해도 중복 로그인이 아니다. 1: 프로세스 종료. 2: 창만 숨김, 트레이 두 번 누름으로 복귀, 서버 파일 목록이 계속 조회됨 |
| 연결 끊김 | 로그인 상태에서 서버 쪽 연결을 끊기(네트워크 어댑터 끄기 등, 공유 서버 프로세스는 건드리지 않는다) | 「연결 끊김」 창, [확인] 뒤 설정한 로그아웃 방식 |
| 자동 로그아웃 | `fm_logout_auto` 를 시험 계정 프로파일에 1(분)로 걸고 대기 | 「1분 동안 사용하지 않아 자동으로 로그아웃했습니다.」, 방식이 종료여도 앱이 꺼지지 않는다. 시험 뒤 원복 |
| 트레이 정책 받아오기 | 트레이 [정책 받아오기] | 앱 로그에 [3007]·[3005]·[3015]·[3094] 네 요청과 응답 건수, 확장자 목록이 덧붙지 않고 같은 건수 |
| 두 번째 실행 | 앱이 떠 있는 상태(보임/트레이 숨김 둘 다)에서 exe 를 다시 실행 | 경고 후 기존 창이 앞으로 온다(숨김 상태 포함) |
| 탐색기 메뉴 업로드 | 설치본 설치 후 탐색기에서 파일 3개를 골라 「snB App 파일 업로드」 | 업로드 한 번에 3개가 올라가고 서버 목록에 3개 |
| 설정 유지 | 테마·배치·언어·로그아웃 방식·시작 폴더·자동 전송 값을 바꾸고 재시작 | 모두 유지. 원본 앱이 쓴 레지스트리 값을 첫 실행에 가져온다(옮기기를 택했다면) |
| 버전 | [프로그램 정보], 설치본 파일 속성, 로그인 로그의 앱 버전 | 세 곳이 같은 값 |
| 화면 디자인 | 같은 PC·같은 날, 원본 `build\release` 와 Wails 를 같은 창 크기(880×660)로 캡처 | 토큰 색(7장 토큰표)이 픽셀 값으로 일치. 유형 컬럼 글자는 예외로 기록 |

### 9.4 디자인 토큰(Wails CSS 변수의 출발점, `settings/Theme.qml`)

팔레트(IBM Carbon Cool Gray):

| 이름 | 값 | 이름 | 값 |
|---|---|---|---|
| White | `#ffffff` | Gray10 | `#f2f4f8` |
| Gray10Hover | `#e4e9f1` | Gray20 | `#dde1e6` |
| Gray30 | `#c1c7cd` | Gray40 | `#a2a9b0` |
| Gray50 | `#878d96` | Gray60 | `#697077` |
| Gray70 | `#4d5358` | Gray80 | `#343a3f` |
| Gray80Hover | `#434a51` | Gray90 | `#21272a` |
| Gray90Hover | `#2b3236` | Gray100 | `#121619` |
| Blue20 | `#d0e2ff` | Blue40 | `#a6c8ff` |
| Blue60 | `#0f62fe` | Blue60Hover | `#0353e9` |
| Blue70 | `#0043ce` | Blue90 | `#001d6c` |
| Green70 | `#0e6027` | Green40 | `#6fdc8c` |
| Orange70 | `#8a3800` | Yellow30 | `#f1c21b` |
| Yellow60 | `#8e6a00` | Red70 | `#a2191f` |
| Red40 | `#ffb3b8` | BrandGold | `#d9a526` |
| CardGlow | `#7b7b7c` | | |

의미 토큰(다크 : 라이트):

| 토큰 | 다크 | 라이트 |
|---|---|---|
| bg | Gray90 | White |
| surface | Gray80 | Gray10 |
| surfaceAlt | Gray90Hover | Gray20 |
| surfaceRaised | Gray70 | White |
| surfaceSunken | Gray80 | Gray10 |
| surfaceHover | Gray80Hover | Gray10Hover |
| surfaceSelected | Blue90 | Blue20 |
| text | Gray10 | Gray100 |
| textSecondary | Gray30 | Gray70 |
| textDisabled | Gray50 | Gray50 |
| textOnAccent | White | White |
| borderSubtle | Gray60 | Gray30 |
| borderControl | Gray50 | Gray50 |
| borderButton | Gray60 | Gray30 |
| borderStrong | Gray30 | Gray50 |
| accent | Blue40 | Blue70 |
| accentFill / accentFillHover | Blue60 / Blue60Hover | 같음 |
| accentSubtle | Gray90Hover | Gray20 |
| brand | BrandGold | Yellow60 |
| loginCardStart / loginCardEnd | Gray70 / CardGlow | Gray20 / White |
| tableRowDivider | Gray40 | Gray50 |
| success / warning / danger | Green40 / Yellow30 / Red40 | Green70 / Orange70 / Red70 |
| scrim | `#B3000000` | `#660A0B10` |
| shadow | `#73000000` | `#201E1F26` |

주의: Qt 색 `#AARRGGBB` 는 알파가 앞이다. CSS 로 옮길 때 `#B3000000` → `rgba(0,0,0,0.70)`, `#660A0B10` → `rgba(10,11,16,0.40)`, `#73000000` → `rgba(0,0,0,0.45)`, `#201E1F26` → `rgba(30,31,38,0.125)`.

치수·글자·모션:

- 글꼴 Pretendard(Regular·Medium·Bold, 앱 기본 10pt). 크기 caption 11, small 12, body 13, large 15, heading 18(좁은 배치 16), title 22(좁은 배치 18, DemiBold).
  Qt 의 pt 와 CSS px 차이: Qt Quick 글자 크기(`font.pixelSize`)를 썼는지 `pointSize` 를 썼는지에 따라 다르다. 추정: Theme 의 값은 픽셀 크기다 ― 같은 PC 캡처로 확인.
- 간격 4/8/12/16/24/32, 반경 4/6/10/14/999.
- 컨트롤 높이 32(좁은 배치 44), 표 행 30, 아이콘 18(20), 메뉴 아이콘 20(24), 메뉴 항목 44·반경 6·폭 189/250·글자 14.
- 모션 120/160ms. 상태 겹침 투명도: 올림 0.08, 누름 0.14, 비활성 0.38.
- 대비 규칙: 글자 4.5:1 이상, 면과 면 사이 ΔE2000 ≥ 6.
- `content/README.md` 규칙: 단색 SVG 아이콘을 색으로 물들임, 토큰 파일 밖에 16진 색 금지, 화면당 주 단추 하나, 외부 문자열은 평문(HTML 해석 금지 ― 보안 E-004, Svelte 는 `{@html}` 금지로 지킨다),
  쌓임 서랍 0 < 대화상자 1 < 띠 2, 대화상자 반경 10·굵은 제목·scrim·오른쪽 아래 단추 최소 80, 표 행 30·줄무늬·점선 경계(5px 선 2px 틈)·넘칠 때 늘 보이는 스크롤바·컬럼 최소 50, 헤더 체크박스 3상태는 행 선택에서 계산.

### 9.5 원본과 대조할 때

- 원본 실행: `tools\snb.cmd run`(자리 확인 `tools\snb.cmd who`), 원본 로그 `%LOCALAPPDATA%\SQIsoft\snB App\log\yyyyMMdd.v2.log`.
- "원본에 없다" 고 적기 전에 원본 `docs/agents/MOSFM_대조.md` 의 절차를 따른다(이 문서에서 "없다" 로 적은 것: Wails 판의 트레이·단일 실행·다국어·단축키 ― `E:\git\snBv6FM.Wails` 의 Go·Svelte 소스에서 `SystemTray`·`SingleInstance`·`i18n`·`hotkey` 를 찾아 0건).
