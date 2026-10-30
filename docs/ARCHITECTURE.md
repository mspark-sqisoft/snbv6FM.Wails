# Wails 판 구조 설계

원본 snbApp(Qt) 기능을 옮기는 동안 여러 사람(에이전트)이 동시에 고쳐도 서로 파일이 겹치지 않게 경계를 정한 문서다.
기능의 세부 동작은 `docs/port/*.md`(영역별 이식 지침서)가 기준이다. 이 문서는 "어디에 무엇을 두는가" 만 정한다.

## 결정한 것

| 주제 | 결정 | 이유 |
|---|---|---|
| 세션 | 한 번에 망 하나만 로그인한다(원본과 같다). 두 망 동시 연결은 하지 않는다 | 원본도 고른 망 하나만 로그인. 망분리 현장에서 반대 망에 닿지 않는다 |
| 암호 모드 | 0(TLS) 과 1~5 를 Go 로 구현한다. 6(NP-ARIA)은 벤더 DLL 이 필요해 뺀다 | 서버 Go 구현(crypt2)을 읽어 같은 알고리즘으로, 시험 벡터로 검증 |
| 압축 검사 | zip(암호 포함)·7z·tar·gz·tgz·bz2·xz 를 순수 Go 로. LZH·ZIPX PPMd 는 "검사 불가" 로 판정해 정책(암호 압축과 같은 묻기/막기)을 따른다 | 순수 Go 구현이 없다 |
| 이어올리기·이어받기 | 하지 않는다. 끊기면 처음부터 다시 | proto 에 offset 칸이 없다(서버 선행) |
| 무결성 검사 | 구현하되 개발 빌드(`-tags dev` 또는 `SNB_DEV=1`, 설치본 빌드는 `SNB_DEV` 를 무시)는 실패해도 막지 않는다 | 시험 서버 file_hash 에 Wails exe 가 없다 |
| 설정 저장 | `%LOCALAPPDATA%\SQIsoft\snB App\wails-settings.json`(JSON). 원본 레지스트리 값은 처음 한 번 읽어 옮긴다 | 원본과 같은 폴더, 파일 이름만 달리해 서로 덮지 않는다 |
| 로그 | 원본과 같은 폴더·같은 줄 형식(`yyyyMMdd.v2.log`) | 원본 E2E 판정·로그 화면 파서를 그대로 쓴다 |
| 화면 | Svelte 5(runes 문법 허용), 자체 디자인 토큰, 라이트·다크, 한국어·영어 | 원본 Theme.qml 값에서 출발해 다듬는다 |
| 서버 문자열 | 화면에 `{@html}` 를 쓰지 않는다 | 서버 글자 서식 해석 금지 |

## Go 패키지(module `snbapp`)

```
main.go                      앱·창·트레이·단일 인스턴스·서비스 등록(기반)
internal/config/             경로, fmenv.ini·fmlocal.ini·fmusr.ini, 설정 저장소(기반)
internal/nbs/                gRPC 연결·세션·로그인·프로파일·오류 문구(기반) + 서비스별 RPC 감싸기(영역별 파일)
internal/nbs/crypt/          암호 모드 1~5 코덱(인증)
internal/applog/             로그 파일 쓰기·읽기(알림·로그)
internal/policy/             파일·확장자·압축 검사(정책)
internal/transfer/           전송 대기열·업로드·다운로드·폴더 압축·자동 전송(전송)
internal/files/              서버파일 상태 판정·삭제·검출 조회·이름 겹침 규칙(파일·내역)
internal/history/            전송내역·결재내역·엑셀 저장(파일·내역)
internal/approval/           결재 요청·처리·대리결재자·결재자 트리(결재)
internal/notify/             알림 스트림 수신·분배(알림·로그)
internal/clip/               클립보드 읽기·쓰기·서식 묶음(클립보드)
internal/link/               Link Redirect 서버·IPC 인자 처리·도메인 정책(클립보드)
internal/localfs/            로컬 탐색기 목록·드라이브·새 폴더(메인)
internal/integrity/          무결성 검사(인증)
internal/svc/                Wails 에 묶는 서비스(화면이 부르는 메서드). 서비스마다 파일 하나
pb/                          proto 생성물(손대지 않는다)
```

### 서비스(`internal/svc`)

모든 서비스는 `*Core` 하나를 공유한다. `Core` 는 `core.go`(기반) 에 있고 아래를 가진다.

- `Session() (*nbs.Session, error)` ― 로그인한 세션(없으면 `ErrNotLoggedIn`)
- `Emit(name string, data any)` ― 화면으로 이벤트
- `Settings() *config.Store`, `Log() *slog.Logger`
- `OnLogin(func(*nbs.Session))`, `OnLogout(func())` ― 로그인·로그아웃 때 할 일을 영역별로 등록(정책 받기, 알림 스트림, 자동 전송 시작·정지, 단축키 등록 등)
- `Touch()` ― 자동 로그아웃 시계를 되돌린다(입력·전송 진행)

| 파일 | 서비스 | 담당 영역 |
|---|---|---|
| `core.go`, `events.go` | (공유) | 기반 |
| `auth.go` | `AuthService` | 인증: 서버 목록, 로그인, 로그아웃, 비밀번호 변경, 사용자 정보, ID 저장, 자동 로그아웃 |
| `files.go` | `FilesService` | 파일·내역: 서버파일 목록, 상태, 삭제, 검출 조회 |
| `history.go` | `HistoryService` | 파일·내역: 전송내역·결재내역, 엑셀 |
| `transfer.go` | `TransferService` | 전송: 업로드·다운로드 요청, 대기열 조회·취소, 자동 전송 |
| `policy.go` | `PolicyService` | 정책: 검사 결과 창 응답, 정책 조회 화면, 정책 받아오기 |
| `approval.go` | `ApprovalService` | 결재: 결재자 트리, 결재 요청·처리, 대리결재자 |
| `clip.go` | `ClipService` | 클립보드: 보내기·받기, 단축키 |
| `log.go` | `LogService` | 알림·로그: 로그 조회 |
| `local.go` | `LocalService` | 메인: 로컬 탐색기 |
| `settings.go` | `SettingsService` | 메인: 설정 읽기·쓰기, 앱 정보 |

화면에 내는 사건 이름은 `events.go` 에 상수로 모은다(기반이 미리 만들어 둔다). 영역이 새 사건이 필요하면
자기 서비스 파일에 상수를 두고 `events.go` 는 고치지 않는다.

## 화면(`frontend/src`)

```
main.ts, App.svelte          로그인 ↔ 메인 전환(기반)
lib/ui/                      디자인 시스템 컴포넌트(기반): Button, IconButton, Input, Select, Checkbox, Dialog, Drawer,
                             DataTable(가상 스크롤·컬럼 폭 끌기·정렬·선택), Toast(알림 띠), Tabs, Menu, Badge, Progress, Tooltip, EmptyState
lib/ui/tokens.css            디자인 토큰(색·글자·간격·그림자), 라이트·다크
lib/i18n/                    번역. 영역마다 lib/i18n/<영역>.ko.ts·<영역>.en.ts 를 두고 index.ts 가 모은다
lib/stores/                  session.ts·ui.ts(기반), 영역별 store 는 영역 파일(transfers.ts 등)
lib/api.ts                   바인딩 다시 내보내기·오류 문구 다듬기(기반)
shell/                       메인 틀(기반): Shell.svelte, SideBar, TopBar(망 배지·사용자·테마·언어·로그아웃), StatusBar
views/<영역>/                영역 화면. 기반이 메뉴마다 빈 화면 파일을 만들어 두고 영역 담당이 채운다
routes.ts                    메뉴 모델(망·역할별 항목과 화면 연결)(기반)
```

메뉴와 화면 파일:

| 메뉴 | 화면 파일 | 담당 |
|---|---|---|
| 파일전송(서버파일 + 로컬 탐색기) | `views/files/FilesView.svelte`, `views/local/LocalPanel.svelte` | 파일·내역, 메인 |
| 결재요청 | `views/approval/RequestView.svelte` | 결재 |
| 결재처리 | `views/approval/ApproveView.svelte` | 결재 |
| 대리결재자 | `views/approval/ProxyView.svelte` | 결재 |
| 전송내역 | `views/history/TransHistoryView.svelte` | 파일·내역 |
| 결재내역 | `views/history/ApprovalHistoryView.svelte` | 파일·내역 |
| 정책 | `views/policy/PolicyView.svelte` | 정책 |
| 로그 | `views/log/LogView.svelte` | 알림·로그 |
| 설정 | `views/settings/SettingsView.svelte` | 메인 |
| 전송 상태 서랍 | `views/transfer/TransferDrawer.svelte` | 전송 |
| 대화상자 | `views/<영역>/dialogs/*.svelte` | 각 영역 |

## 함께 쓰는 규칙

- 남의 파일을 고치지 않는다. 필요한 것이 남의 파일에 없으면 자기 파일에 두거나, 기반 파일이면 마지막 통합 때 고친다(보고에 적는다).
- `go.mod` 의존은 기반이 미리 넣는다. 새 의존이 꼭 필요하면 `go get` 하되 보고에 적는다.
- 자기 패키지는 늘 컴파일되는 상태로 둔다(같은 폴더의 다른 사람이 깨지지 않게). 확인은 `go build ./...`, `go vet ./internal/<자기 패키지>/...`.
- 바인딩은 `wails3 generate bindings -clean=true -ts -i` 로 만든다. 다른 패키지가 컴파일되지 않아 실패하면 잠시 뒤 다시 한다.
- 화면 확인은 `cd frontend; npm run check`(svelte-check). 
- 시험: 순수 판정 함수는 표 시험(`*_test.go`)을 남긴다. 원본 단위 시험 표가 있으면 그대로 옮긴다.
- 문구: 원본 한국어 문구를 그대로 쓰고 영어 번역을 붙인다.
- 실 서버 시험은 통합 뒤 한 곳에서 차례로 한다(시험 계정이 겹치면 중복 로그인으로 서로 끊는다).
