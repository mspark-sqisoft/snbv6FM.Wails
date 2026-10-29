# snbApp (Wails 판)

snBv6FM(Qt Quick + C++)의 망연계 파일전송 클라이언트를 Go + [Wails v3](https://v3.wails.io) + Svelte(TS) 로 옮기는 저장소다.
서버(NBS)와는 원본과 같은 gRPC 규격(`proto/`)으로 통신한다. 원본 저장소는 `E:\git\snBv6FM` 이다.

## 지금 되는 것과 안 되는 것

| 기능 | 상태 | 원본에서 대응하는 곳 |
|---|---|---|
| fmenv.ini 의 서버(망) 고르기 | 됨 | `fmenv.ini` `[SERVERxx]` |
| 로그인(PreLogin → Login → 권한 프로파일) | 됨, 실 서버 확인 | `FileMoverGrpc::login()` |
| 서버 파일 목록 | 됨, 실 서버 확인 | `GetServerFileList` |
| 업로드(파일, 한 번에 하나씩) | 됨, 실 서버 확인 | `FileTransfer.cpp` |
| 다운로드(받은 뒤 서버 목록에서 지움) | 됨, 실 서버 확인 | `FileTransfer.cpp`, `src/DownloadBlock.cpp` |
| 서버 암호 모드 1~6(메시지 암호화) | 안 됨 ― 0(TLS 만)만 | `imports/FMEncryption` |
| 업로드 때 결재자 고르기(`fm_upfile_mode=0` 계정) | 안 됨 | 결재자 선택창 |
| 결재 요청·결재·대리결재·이력 | 안 됨 | `ApprovalService`·`ApproverService` |
| 폴더 업로드(zip)·받기, 압축 검사, 확장자 정책 | 안 됨 | `ZipExtractor`, `FileExtensionService` |
| 동시 전송·전송 상태 서랍·취소 | 안 됨 | `TransferQueue` |
| 알림 스트림, 클립보드, 자동 업로드·다운로드, 트레이, 무결성 검사, 업데이트 | 안 됨 | `NotifyService`, `src/clipboard` 등 |

## 필요한 것

- Go(1.26 이상), Node.js(npm)
- Wails CLI ― `go.mod` 의 판과 맞춘다: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`
- proto 를 다시 만들 때만: `protoc`(이 PC 는 `D:\git\protoc-35.1-win64\bin`), `protoc-gen-go`, `protoc-gen-go-grpc`

## 빌드·실행

```powershell
wails3 build                     # bin\snbApp.exe (Taskfile.yml 의 build 작업)
wails3 dev                       # 고치면 다시 불러오는 개발 실행
```

`app.go` 의 메서드나 반환 구조체를 바꾸면 화면 쪽 바인딩(`frontend/bindings/`)을 다시 만든다. 빌드도 다시 만들지만,
`svelte-check` 를 먼저 돌릴 때는 직접 만든다:

```powershell
wails3 generate bindings -clean=true -ts -i
cd frontend; npm run check
```

앱은 `fmenv.ini`·`root_ca.pem` 을 **실행 파일 폴더**에서 읽는다. 환경변수 `SNB_APP_DIR` 가 있으면 그 폴더에서 읽는다.
원본 개발 빌드의 설정을 그대로 쓰려면:

```powershell
$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'; .\bin\snbApp.exe
```

로그는 아직 파일로 남기지 않고 표준 오류로만 낸다.

## 시험

```powershell
go test .                        # 단위 시험(fmenv.ini 읽기, 다운로드 막기 규칙)
```

`main.go` 가 `frontend/dist` 를 실행 파일에 넣으므로 그 폴더가 없으면 `go test`·`go vet` 이 컴파일에서 실패한다.
`wails3 build` 를 한 번 하거나 빈 `frontend/dist/index.html` 을 만든다.

실 서버 시험(`live_test.go`)은 `SNB_LIVE_ID` 가 있을 때만 돈다. 변수는 그 파일 머리 주석에 있다. 업로드 시험은
서버에 `WAILS_<시각>.txt`(1.5MB)를 남긴다.

```powershell
$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'; $env:SNB_LIVE_ID = '…'; $env:SNB_LIVE_PW = '…'
go test -run Live -v -count=1 .
```

## proto 를 바꿨을 때

`proto/` 는 원본 `imports/FileMoverGrpc/proto` 의 사본이다. 원본이 바뀌면 다시 복사하고 `go generate .` 로 `pb/` 를 다시 만든다
(`gen.go`). proto 의 `go_package`(`servicespb/…`)는 이 모듈 경로와 달라 `gen.go` 의 `M` 옵션으로 덮는다.

## 파일

| 파일 | 하는 일 |
|---|---|
| `main.go` | Wails 앱과 창을 만들고 `AppService` 를 서비스로 등록한다 |
| `app.go` | `AppService`: 화면이 부르는 메서드, 파일·폴더 선택 창, 진행률 이벤트(`progress`) |
| `client.go` | gRPC 통신(로그인·목록·업로드·다운로드), fmenv.ini 읽기 |
| `frontend/src/App.svelte` | 로그인 화면과 서버 파일 화면 |
| `frontend/bindings/` | `app.go` 에서 만든 TS 바인딩(생성물, 손으로 고치지 않는다) |
| `pb/` | `proto/` 에서 만든 Go 코드(생성물, 손으로 고치지 않는다) |
| `Taskfile.yml`, `build/` | Wails v3 빌드 작업과 아이콘·설치 관리자·판 정보(`build/config.yml`) |
