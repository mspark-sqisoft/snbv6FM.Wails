# snbApp (Wails 판)

snBv6FM(Qt Quick + C++)의 망연계 파일전송 클라이언트를 Go + [Wails v3](https://v3.wails.io) + Svelte(TS) 로 옮긴 저장소다.
서버(NBS)와는 원본과 같은 gRPC 규격(`proto/`)으로 통신한다. 원본 저장소는 `E:\git\snBv6FM` 이다.

구조는 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), 영역별 이식 기준은 `docs/port/*.md`, 디자인 규칙은 [docs/DESIGNS.md](docs/DESIGNS.md) 에 있다.

## 기능과 확인 상태

"실 서버 확인"은 시험 서버에서 시험이 통과했다는 뜻이다. "단위 시험"은 로컬 가짜 서버나 함수 시험까지 통과했고 실 서버 왕복은 아직 확인하지 못했다는 뜻이다.

| 기능 | 상태 | 패키지 |
|---|---|---|
| 서버(망) 고르기, 로그인, 권한 프로파일, 사용자 정보 | 실 서버 확인 | `internal/nbs`, `internal/svc/auth.go` |
| 서버 파일 목록, 업로드, 다운로드, 동시 전송, 취소 | 실 서버 확인 | `internal/transfer` |
| 결재 요청, 승인·반려, 결재 받기(검사값·같은 이름·덮어쓰기·폴더 풀기·받는 중 취소) | 실 서버 확인 | `internal/approval` |
| 전송내역·결재내역, 엑셀 저장 | 단위 시험 | `internal/history` |
| 파일·확장자·압축 검사 | 단위 시험 | `internal/policy` |
| 클립보드 보내기·받기 | 단위 시험, 실 서버 왕복 실패 후 미확인 | `internal/clip` |
| 도메인 정책, 링크 왕복 | 단위 시험, 실 서버 미확인 | `internal/link` |
| 알림 스트림, 로그 파일, 로그 화면 | 단위 시험 | `internal/notify`, `internal/applog` |
| 자동 업로드·다운로드, 트레이, 단일 실행, 단축키 | 구현, 실제 창에서 미확인 | `internal/transfer`, `main.go` |
| 디자인 6종(기본, 에어록, 공문, 출발 안내판, 운송장, 노선도), 한국어·영어 | 캡처로 확인 | `frontend/src/designs` |
| 서버 암호 모드 0~5 | 모드 0 은 실 서버 확인, 1~5 는 시험 벡터 | `internal/nbs/crypt` |

하지 않는 것과 알려진 제약은 다음과 같다.

- 암호 모드 6(NP-ARIA)은 벤더 DLL 이 필요해 지원하지 않는다.
- 이어올리기·이어받기는 proto 에 offset 칸이 없어 지원하지 않는다. 끊기면 처음부터 다시 보낸다.
- LZH·ZIPX PPMd 압축은 순수 Go 구현이 없어 "검사 불가"로 판정하고 정책을 따른다.
- 앱 밖으로 끌어서 받기는 Wails API 가 없어 지원하지 않는다.
- Wails exe 는 시험 서버의 클라이언트 무결성 목록(`file_hash`)에 없다. 그래서 등록 전에는 개발 빌드로만 로그인할 수 있다(아래 "빌드·실행").
- macOS·Linux 클립보드는 글만 지원한다.

## 필요한 것

- Go(1.26 이상), Node.js(npm)
- Wails CLI: `go.mod` 의 판과 맞춘다. `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`
- proto 를 다시 만들 때만: `protoc`(이 PC 는 `D:\git\protoc-35.1-win64\bin`), `protoc-gen-go`, `protoc-gen-go-grpc`

## 빌드·실행

```powershell
wails3 build DEV=true            # 개발 빌드, bin\snbApp.exe
wails3 dev                       # 고치면 다시 불러오는 개발 실행(개발 빌드)
wails3 build                     # 배포 빌드, bin\snbApp.exe (Taskfile.yml 의 build 작업)
```

`wails3 build` 는 `-tags production` 배포 빌드다. 무결성 실패면 앱이 끝나므로, 서버 `file_hash` 에 그 exe 가 등록되기 전에는 로그인하자마자 무결성 창이 뜨고 앱이 끝난다. `SNB_DEV`·`SNB_INSTANCE`·`SNB_CDP_PORT` 도 듣지 않는다.

개발 빌드는 무결성 실패를 경고로만 남긴다. 개발 빌드는 `-tags dev` 이거나 `SNB_DEV=1` 을 준 빌드다. Linux·macOS 의 `DEV=true` 는 `dev` 태그가 없어 `SNB_DEV=1` 도 함께 준다.

등록 전에 배포 빌드를 시험할 때는 `wails3 build EXTRA_TAGS=snb_noenforce` 로 만들어 `SNB_DEV=1` 로 실행한다. 이 태그는 배포에 쓰지 않는다.

서비스 메서드나 반환 구조체를 바꾸면 화면 쪽 바인딩(`frontend/bindings/`)을 다시 만든다. 빌드도 다시 만들지만, `svelte-check` 를 먼저 돌릴 때는 직접 만든다.

```powershell
wails3 generate bindings -clean=true -ts -i
cd frontend; npm run check
```

앱은 `fmenv.ini`·`root_ca.pem` 을 **실행 파일 폴더**에서 읽는다. 환경변수 `SNB_APP_DIR` 가 있으면 그 폴더에서 읽는다. 원본 개발 빌드의 설정을 그대로 쓰려면 개발 빌드를 다음과 같이 실행한다.

```powershell
wails3 build DEV=true
$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'; .\bin\snbApp.exe
```

설정은 `%LOCALAPPDATA%\SQIsoft\snB App\wails-settings.json` 에 저장한다. 로그는 원본과 같은 폴더에 `yyyyMMdd.v2.log` 로 남긴다.

창 없이 브라우저로 화면을 보려면 서버 모드로 빌드한다. `WAILS_SERVER_HOST`·`WAILS_SERVER_PORT` 로 주소를 정한다.

```powershell
go build -tags server -o bin\snbapp-server.exe .
```

## 시험

```powershell
go test ./... -count=1           # Go 단위 시험
cd frontend; npm run check; npm run check:i18n; npm run check:designs; npm run check:dialogs
```

`check:i18n` 은 번역 키 누락을, `check:designs` 는 디자인 토큰과 글자 대비를, `check:dialogs` 는 안내 창 합치기를 검사한다.

`main.go` 가 `frontend/dist` 를 실행 파일에 넣으므로 그 폴더가 없으면 `go test`·`go vet` 이 컴파일에서 실패한다. `wails3 build` 를 한 번 하거나 빈 `frontend/dist/index.html` 을 만든다.

실 서버 시험(패키지마다 `live_test.go`, `internal/svc/*_live_test.go`)은 `SNB_LIVE_ID` 등 환경변수가 있을 때만 돈다. 변수는 각 파일 머리 주석에 있다. 시험 서버는 메모리가 3.7GB 라 큰 파일을 반복해 보내는 시험을 돌리지 않는다.

## proto 를 바꿨을 때

`proto/` 는 원본 `imports/FileMoverGrpc/proto` 의 사본이다. 원본이 바뀌면 다시 복사하고 `go generate .` 로 `pb/` 를 다시 만든다(`gen.go`). proto 의 `go_package`(`servicespb/…`)는 이 모듈 경로와 달라 `gen.go` 의 `M` 옵션으로 덮는다.

## 주요 폴더

| 폴더 | 하는 일 |
|---|---|
| `main.go` | Wails 앱·창·트레이·단일 실행·서비스 등록 |
| `internal/` | 기능 패키지. `internal/svc` 가 화면이 부르는 서비스다 |
| `frontend/src/views`, `shell`, `lib` | 화면, 앱 틀, 공용 부품·번역·상태 |
| `frontend/src/designs` | 디자인 5종의 CSS 와 전송 띠 |
| `frontend/bindings/` | Go 서비스에서 만든 TS 바인딩(생성물, 손으로 고치지 않는다) |
| `pb/` | `proto/` 에서 만든 Go 코드(생성물, 손으로 고치지 않는다) |
| `Taskfile.yml`, `build/` | Wails v3 빌드 작업과 아이콘·설치 관리자·판 정보(`build/config.yml`) |
