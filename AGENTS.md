# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

snbApp 은 서로 끊어진 두 망 사이로 파일을 건네는 클라이언트다. 이 저장소는 원본 `E:\git\snBv6FM`(Qt Quick + C++)을 Go + Wails v3 + Svelte 5(TS)로 옮긴 판이고, 서버 NBS 와 원본과 같은 gRPC 규격(`proto/`)으로 통신한다.

응답과 문서는 한국어로 쓴다. 기능별 확인 상태와 지원하지 않는 것은 `README.md`, 패키지·화면 경계는 `docs/ARCHITECTURE.md`, 디자인 규칙은 `docs/DESIGNS.md` 가 기준이다. 기능의 세부 동작은 영역별 이식 지침서 `docs/port/*.md` 가 기준이고, `AUTH-024` 같은 요건 ID 는 원본 명세의 번호다.

## 명령

```powershell
wails3 build DEV=true                                   # 개발 빌드, bin\snbApp.exe
wails3 dev                                              # 고치면 다시 불러오는 개발 실행
wails3 build                                            # 배포 빌드(-tags production)
go build -tags server -o bin\snbapp-server.exe .        # 창 없이 브라우저로 보는 서버 모드

go test ./... -skip TestLive -count=1                   # Go 시험 전체(실 서버 시험 제외)
go test ./internal/clip -run TestReceiveCleansDisguisedCSS -count=1   # 시험 하나
go vet ./...
gofmt -l internal

cd frontend; npm run check; npm run check:i18n; npm run check:designs; npm run check:dialogs

wails3 generate bindings -clean=true -ts -i             # Go 서비스 → frontend/bindings
go generate .                                           # proto/ → pb/ (protoc 필요, 이 PC 는 D:\git\protoc-35.1-win64\bin)
```

- `main.go` 가 `frontend/dist` 를 실행 파일에 넣으므로, 그 폴더가 없으면 `go test`·`go vet` 이 컴파일에서 실패한다. `wails3 build` 를 한 번 하거나 빈 `frontend/dist/index.html` 을 만든다.
- 서비스 메서드나 반환 구조체를 바꾸면 바인딩을 다시 만든 뒤 `npm run check` 를 돌린다.
- 앱은 `fmenv.ini`·`root_ca.pem` 을 실행 파일 폴더에서 읽는다. 저장소에는 이 파일들이 없고 `bin\` 은 추적하지 않으므로, 파일을 찾지 못하면 `$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'` 를 준다.
- `wails3 build` 는 `go mod tidy` 를 돌린다. 코드에서 쓰지 않는 의존을 유지하려면 `internal/deps/deps.go` 에 둔다.
- 이 PC 에는 gcc 가 없어 `-race` 를 돌리지 못한다. `_other.go` 를 고쳤으면 `$env:GOOS='linux'; go vet ./internal/<패키지>` 로 확인한다.
- 로그는 `<데이터 폴더>\log\yyyyMMdd.v2.log` 에 남고, `log` 폴더가 없으면 오류 없이 버린다. 그래서 로그가 없다고 그 동작이 없었다고 판정하지 않는다. `SNB_INSTANCE` 의 새 데이터 폴더에는 `log` 를 먼저 만든다. 인스턴스 없이 띄우면 Qt 원본과 같은 파일을 쓰고, 설정의 [로그 저장]을 끄면 폴더째 지워진다.

## 빌드 태그

| 태그 | 효과 |
|---|---|
| `dev` | 개발 빌드. `config.IsDev()` 가 참이고 무결성 실패는 경고로만 남는다(`SNB_INTEGRITY_ENFORCE=1` 이면 어떤 빌드든 강제한다) |
| `production` | 배포 빌드. `SNB_DEV` 를 무시하므로 무결성 실패면 앱이 끝난다 |
| `production` + `snb_noenforce` | 등록 전 시험용으로 `SNB_DEV=1` 을 다시 듣게 한다. 배포에 쓰지 않는다(`wails3 build EXTRA_TAGS=snb_noenforce`) |
| `server` | 창 없는 HTTP 서버 모드. Wails 만 보는 태그라 `go build -tags server` 는 맨 빌드와 같고, 로그인하려면 `SNB_DEV=1` 이 필요하다. `wails3 task build:server` 는 `server,production` 이라 `SNB_DEV` 를 듣지 않는다. 파일 선택 창은 오류를 돌려주고 메시지 창·트레이는 아무 일도 하지 않는다 |

태그가 없는 맨 `go build` 도 `SNB_DEV=1` 을 들으며, `go test` 가 이 상태다. Wails exe 는 시험 서버의 클라이언트 무결성 목록(`file_hash`)에 없어서, 등록 전에는 배포 빌드로 로그인하면 무결성 창이 뜨고 앱이 끝난다. `SNB_INSTANCE`·`SNB_CDP_PORT`·`SNB_TEST_HOOKS`·`SNB_INTEGRITY_PERIOD_SEC` 는 개발 빌드에서만 듣고, `SNB_CDP_PORT`(화면 자동 조작용 원격 디버깅)는 `SNB_INSTANCE` 도 있어야 켜진다.

## 구조

호출은 `frontend/src` 화면 → `lib/api.ts` → `frontend/bindings`(생성물) → `internal/svc` → 영역 패키지 → `nbs.Session` → gRPC(`pb/`) 순서로 내려간다. 화면은 `bindings/` 를 직접 import 하지 않고 `lib/api.ts` 에서 가져온다.

**이벤트.** Go 는 `Core.Emit` 으로 화면에 사건을 보내고, 이름 상수는 `internal/svc/events.go` 나 서비스 파일에 둔다. Wails 에 등록하지 않아 타입이 생성되지 않으므로 화면 쪽은 손으로 맞춘다. 대개 `Ev`(`lib/api.ts`)에 있지만 `views/transfer/store.ts` 의 `EV_*` 와 `views/clip/Host.svelte` 의 `clip:received` 처럼 따로 있는 것도 있으니, 이름을 바꿀 때는 `frontend/src` 전체에서 그 글자를 찾는다.

본문은 지금 화면 하나만 그리므로 화면 컴포넌트 안의 구독은 메뉴를 옮기면 사라진다. 상시 구독과 창은 `views/<영역>/Host.svelte` 에 둔다(Shell 이 로그인 뒤에 저절로 붙인다). `OnLogin` 훅은 `Login` 이 돌아오기 전에 돌아 들을 화면이 없으므로, Host 는 붙을 때 현재 값을 바인딩으로 받아 온다. Go 가 창의 답을 기다릴 때는 `Transfers.askConflict` 처럼 질문 번호(`Core.gen` 과 별개)를 붙이고, 로그아웃 때 풀리게 하고, 창 닫기 사건을 낸다.

**서비스 등록.** 서비스마다 파일 하나이고 모두 `*svc.Core` 를 공유한다. `Core.Services()` 가 서비스를 만들고, 생성자가 `OnLogin`·`OnLogout` 으로 할 일을 등록하고 영역 사이 연결도 직접 건다(`serverFiles.onChange`, `c.Notify().Register` 등). 그래서 `Services()` 의 생성 순서에 기대는 곳이 있다. `wire()`(`wiring.go`)는 어느 영역에도 속하지 않는 무결성 알림만 잇는다. 서버 알림의 기본 분배는 `log.go` 의 `registerDefaults` 에 있다.

**세션과 늦은 결과.** 한 번에 망 하나만 로그인한다. 세대 번호 `Core.gen` 은 로그인·로그아웃마다 오른다. 오래 걸리는 일은 시작할 때 `Gen()` 을 받아 두고 끝날 때 `IsCurrent(gen)`·`LogoutIf(gen, …)` 로 낡았는지 판정해야 한다. 그러지 않으면 앞 로그인의 늦은 응답이 다음 로그인의 상태를 건드린다.

`OnLogout` 훅과 알림 처리기는 아무것도 기다리지 않는다. 처리기는 스트림 고루틴에서 돌고 그 안에서 `Logout` 이 불릴 수 있어, 전송이나 스트림의 종료를 기다리면 교착된다. 화면은 `session:ended` 에서 확인 창을 모두 닫으므로, 로그아웃과 함께 띄울 안내는 `LogoutNotice` 로 보낸다. 서버에 Logout RPC 가 없어 `Session.Close()` 로 TCP 를 끊어야 서버가 세션을 회수한다.

단일 요청은 `s.Ctx(nbs.RPCTimeout)`(15초), 스트림은 `s.Ctx(0)` 을 쓴다. `Job.Run` 이 받는 ctx 에는 인증 토큰이 없고 `s.Ctx` 는 그 취소를 잇지 않으므로, 대기열 작업 안의 RPC 는 `transfer.SessionCtx(ctx, s)` 를 쓴다. 이것을 빼면 전송 행의 [취소]가 스트림에 닿지 않는데도 오류가 나지 않는다.

**전송.** `internal/transfer` 의 대기열에 `Job{Prepare, Run, Done}` 을 넣는다. `Prepare`(압축·검사값)는 차례 전에 미리 돌고, `Key` 로 같은 파일의 중복을 막는다. 대기열 앞의 업로드 입구(검사·결재자 창 단계)는 한 번에 한 주인만 쓰고, 겹칠 때의 처리는 `gate.go` 의 `AdmitUpload` 표가 정한다. 주인 상태는 `Transfers.claim` 이 쥐고, `EnqueueUpload` 는 이 입구를 거치지 않는다. 이어올리기는 proto 에 offset 칸이 없어 하지 않는다.

**순수 판정과 OS 입출력.** 패키지마다 판정 로직은 OS 와 무관한 파일에 두고 표 시험을 붙인다. OS 호출은 `*_windows.go`·`*_other.go` 로 나눈다.

**신뢰 경계.** 클립보드 HTML(`internal/clip/refs.go`)과 받은 주소(`internal/link`)는 반대 망에서 온 데이터로 다룬다. 화면에 `{@html}` 을 쓰지 않는다. `main.go` 의 `warnOnly` 는 Wails 가 DEBUG 로그에 바인딩 호출 인자(로그인 비밀번호 포함)를 찍는 것을 막으므로 낮추지 않는다.

**시험.** 서버가 필요한 시험은 `internal/transfer/fake_test.go` 처럼 `bufconn` 가짜 서버와 `nbs.NewTestSession` 으로 짠다. 실 서버 시험은 이름이 모두 `TestLive` 로 시작하고(대부분 `live_test.go`·`*_live_test.go`, `internal/svc/approval_test.go` 의 `TestLivePick` 도 있다), 주석에 적힌 `SNB_LIVE_*` 환경변수가 없으면 건너뛴다. `LogService.InjectNotify` 는 서버가 보내 주지 못하는 알림을 시험하는 진입점이다. 개발 빌드이면서 `SNB_TEST_HOOKS=1` 이고 로그인 상태여야 하며, 빌드나 변수 조건이 맞지 않으면 `ErrNotImplemented` 를 돌려주므로 미구현으로 오해하지 않는다.

**화면.** `routes.ts` 가 망·역할별 메뉴를 정한다. 번역은 `lib/i18n/<영역>.ko.ts`·`<영역>.en.ts` 이고 원본 한국어 문구를 그대로 쓴다. 디자인을 더할 때는 `designs/registry.ts` 의 `DesignId` 와 `designs` 배열, `designs/<id>/design.css`, `designs/index.css` 의 `@import`, `scripts/check-designs.mjs` 의 `IDS`, 번역 키를 함께 고친다. `Art.svelte`·`Strip.svelte` 만 있으면 저절로 붙는다.

## 작업 규칙

- `pb/`·`frontend/bindings/` 는 생성물이라 손으로 고치지 않는다. `proto/` 는 원본 `imports/FileMoverGrpc/proto` 의 사본이다.
- 원본의 동작이 의도인지는 `E:\git\snBv6FM` 과 대조해 판정한다. 원본과 서버 저장소는 읽기만 하고, 서버 변경은 담당자에게 요청문으로 넘긴다.
- commit 과 push 는 각각 허락을 받은 뒤 한다. 이전 승인은 이후 수정에 적용하지 않고, main push 는 별도로 명시적인 허락이 필요하다.
- 커밋 메시지는 한국어 conventional commits 로 쓰고 현상·원인·조치·검증·검토 절을 둔다. attribution 줄을 넣지 않는다.
- 시험 서버는 메모리가 3.7GB 다. 큰 파일을 반복해 보내지 않고, 목록 폴링은 2초 이상으로 둔다. 시험 계정이 겹치면 중복 로그인으로 서로 끊기므로 실 서버 시험은 차례로 돌린다.
