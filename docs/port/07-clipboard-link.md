# 07 클립보드 전송·Link Redirect·외부 연계 ― Wails 판 이식 분석

작성: 2026-10-30 · 대상: snbApp Qt 판(`E:\git\snBv6FM`, 브랜치 dev/ms.park) → Wails 판(`E:\git\snBv6FM.Wails`)
근거 명세: `specs/current/07-clipboard.md`(CLIP-001~054), `specs/current/08-link-external.md`(LINK-001~047),
`specs/107-clip-image-allowlist/intent.md`(초안). 코드는 읽기만 했다. 줄 번호는 2026-10-30 작업트리 기준이라 조금 움직였을 수 있다.

표기 규칙
- `CLIP-nnn`·`LINK-nnn` 은 현행 명세의 요건 번호다. `CLIP-` 은 클립보드 전송, `LINK-` 는 Link Redirect·명령줄·오른쪽 메뉴·파이프 연계다.
- `DOM-nn`·`IMG-nn` 은 이 문서가 새로 붙인 번호다. 도메인 정책(hosts 파일 반영)과 받은 그림 형식 허용 목록(작업 107)을 가리킨다.
- `[3204]` 처럼 대괄호 안 네 자리 수는 서버 전문 번호다. 로그와 서버 문서가 이 번호로 요청을 가리킨다.
- `추정:` 으로 시작하는 문장은 실행이나 소스로 확인하지 않은 것이다.
- "업무망" 은 location 1, "인터넷망" 은 location 2 다(`fmenv.ini [SERVERxx] LOC`).

---

## 1. 범위

이 문서가 다루는 것

| 묶음 | 원본 위치 | 한 줄 설명 |
|---|---|---|
| 클립보드 업로드 | `src/ViewController.cpp` `clipboardUpload()`(~1316-1415), `checkUploadFiles`(2033), `checkUpFileMode`(2281), `addTransFile`(2592) | 사용자 PC 클립보드(그림·서식 HTML·글)를 임시 파일로 써서 상대망으로 올린다 |
| 서식(HTML) 묶음 | `src/clipboard/ClipHtml.{h,cpp}`, `ClipRefs.h`(591줄), `ClipFmh.h` | HTML 과 그 안 그림을 `__CLIP__.fmh` 한 파일로 묶고, 받는 쪽에서 검사해 푼다 |
| 클립보드 다운로드 | `ViewController::clipboardDownload`, `imports/FileMoverGrpc/FileTransfer.cpp` `download()`(1545-1760), `src/ViewControllerNotifySlot.cpp`(550-790) | 서버에 쌓인 클립보드 건을 받아 사용자 클립보드에 넣고 서버에서 지운다 |
| 받은 그림 형식 허용 목록 | `specs/107-clip-image-allowlist/intent.md` | 받은 그림을 PNG·JPEG·GIF·BMP·WEBP·TIFF 로만 연다(초안, 아직 구현 전) |
| 전역 단축키 | `ViewController::initHotKey/changeHotKey`(172-263), `clipboardSendHotKey`(~1216), `imports/QHotkey`, `content/SettingsScrollView.qml`(623-819, 1044-1068) | 복사 단축키(Ctrl+C 흉내 후 업로드), 붙여넣기 단축키(받기 후 Ctrl+V 흉내) |
| Link Redirect | `src/LinkRedirectServer.{h,cpp}`, `src/ViewControllerRedirect.cpp`, `imports/FileMoverGrpc/FileMoverGrpc.cpp`(1300, 1593, 2516-2560), `imports/FileMoverPolicy/filemoverpolicy.cpp`(234) | 업무망 브라우저 확장이 알린 URL 을 정책에 따라 인터넷망 PC 로 보내 연다 |
| 브라우저 확장 | `imports/ssbridgeExtension/`(manifest V3, background.js, Notice/) | 페이지 로드마다 URL 을 `http://localhost:29000` 으로 알린다 |
| 명령줄·단일 인스턴스·파이프 | `src/main.cpp`(349-372, 470-540, 800-880), `ViewController::handleIpcMessage`(3154), `onIpcBatchTimerTimeout`(3299) | `--mode upload|download|noupdate`, 두 번째 실행의 경로를 떠 있는 앱에 넘긴다 |
| 탐색기 오른쪽 메뉴 | `installer/installer_common.nsh`(257-279) | `snB App 파일 업로드`·`snB App 파일 다운로드` |
| 도메인 정책 | `src/domain/DomainPolicy.cpp`, `src/domain/HostsBlock.h`, `tests/tst_DomainPolicy.cpp` | 서버가 준 도메인·IP 를 hosts 파일의 전용 구역에 쓰고, 끝날 때 지운다 |

다루지 않는 것(다른 문서가 주관)
- 파일 업로드의 검사·결재 갈래 본체 → 03 업로드. 여기서는 클립보드가 그 갈래에 들어가는 입구와 다른 점만 적는다.
- 파일 다운로드의 막기 규칙(`downloadBlock`)·검사값 대조 본체 → 04 다운로드.
- 결재자 선택 창·[결재요청] 화면 본체 → 06 결재.
- 알림 스트림(9001·9004·9005) 수신 본체 → 09 알림. LINK-044~047 은 명세가 08 에 두었지만 Wails 판에서는 09·14 와 함께 만든다.
- 설치본 빌드 → 15 설치본. 오른쪽 메뉴 레지스트리 값만 여기서 정한다.
- WASM 판 전용 동작(CLIP-051·052, 확장 없는 WASM 링크 열기) ― Wails 판은 데스크톱만 만든다.

Wails 판의 지금 상태(2026-10-30, `README.md`·`client.go` 467줄 확인)
- 로그인·파일 목록·파일 업로드(한 번에 하나)·파일 다운로드만 된다. 클립보드·단축키·Link Redirect·IPC·도메인 정책은 하나도 없다
  (검색: `E:\git\snBv6FM.Wails` 의 `*.go`·`frontend/src` 에서 `clip`·`hotkey`·`29000`·`hosts`·`SingleInstance` 0건).
- `client.go` 의 `downloadBlock()` 은 이미 `__CLIP__` 행을 파일 다운로드에서 막는다(CLIP-037 과 같다).
- `client.go` `Download()` 는 첫 응답 `code == 3` 을 검사값으로 본다. **클립보드 받기에서는 code 3 이 서식 묶음(.fmh)이다** ―
  같은 함수를 쓰면 안 된다(6절 참고).

---

## 2. 기능 목록

우선순위: **P1** = Qt 판과 동등하게 꼭 있어야 함, **P2** = 있으면 좋음, **제외** = 만들지 않음(이유를 적는다).
난이도: 상(Win32 직접 호출·바이너리 형식·보안 판정), 중(서버 통신 + 화면), 하(분기·문구).
"Qt 상태" 칸은 명세의 상태(구현·부분·상충)를 그대로 옮겼다. 상충 = 이전 명세와 코드가 다르고 코드를 현행으로 본 것.

### 2.1 클립보드 업로드

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| CLIP-001 | [파일전송] 넓은 표·좁은 카드 모두에 [클립보드 업로드] 단추 | P1 | 하 | 구현. 단추 접근성 이름 `클립보드 업로드` 유지(E2E 가 이름으로 누른다) |
| CLIP-002 | 그림 → 서식(HTML) → 글 순서로 판정해 `__CLIP__.bmp`·`.fmh`·`.txt` 를 `<앱 자료>/temp/clip/` 에 만든다 | P1 | 상 | 구현. Win32 형식 직접 읽기(7.1절). macOS·Linux 는 Wails `Clipboard` 가 글만 다뤄 따로 만든다 |
| CLIP-003 | 비었거나 세 형식이 아니면 `저장된 클립보드가 없습니다.`(제목 `정보`) | P1 | 하 | 구현 |
| CLIP-004 | 로그인 전이면 안내 없이 끝낸다 | P1 | 하 | 구현. 개선: 단추를 비활성으로 보인다(8절) |
| CLIP-005 | 업로드 권한 없으면 `클립보드 업로드가 허용되어 있지 않습니다.` | P1 | 하 | 구현. 칸 뒤바뀜에 주의(4.1절) |
| CLIP-006 | 답을 기다리는 업로드가 있으면 받지 않는다(임시 파일 덮어쓰기 전에 검사) | P1 | 중 | 구현. 03 업로드의 업로드 목록 주인 규칙을 같이 쓴다 |
| CLIP-007 | HTML 을 `src.htm` 에 쓰고 태그 안 `src` 그림 참조를 찾아 문자 참조를 풀고 분류 | P1 | 상 | 구현. `ClipRefs.h` 를 Go 로 옮긴다(7.3절) |
| CLIP-008 | 실을 그림을 작업 스레드에서 확인(로컬은 확장자+앞머리, 원격은 5초·막는 주소·리디렉션 재판정·앞머리) | P1 | 상 | 구현. goroutine + `http.Client` 로 오히려 쉽다 |
| CLIP-009 | 클립보드 전용 상한 `fm_clip_size`(KB)로 검사, 상한 이상이면 `전송 가능한 파일 크기 초과` | P1 | 하 | 구현. 상한과 같아도 막는다(`limit > size` 만 통과) |
| CLIP-011 | 크기 검사만 하고 확장자·위장·압축 검사·[파일 검사 결과] 창 없이 결재 갈래로 | P1 | 하 | 구현 |
| CLIP-013 | 결재자 사전 선택(`fm_upfile_mode=0`)이면 결재자 창, 결재 유형 0·6 이면 창 없이 보낸다 | P1 | 중 | 구현. 06 결재의 결재자 창이 먼저 있어야 한다 |
| CLIP-014 | 결재자 사후 선택(`fm_upfile_mode=1`)이면 클립보드 결재 유형을 실어 곧바로(`resv` "2") | P1 | 하 | 구현 |
| CLIP-015 | 전송을 대기열에 넣을 때 사용자 클립보드를 비운다. 앞에서 멈추면 그대로 둔다 | P1 | 중 | 구현(작업 083) |
| CLIP-016 | 전송 상태 패널에 `클립보드`/`업로드`, `클립보드`/`다운로드`/`다운로드 대기중` | P1 | 하 | 구현. 03·04 의 전송 상태 서랍이 먼저 있어야 한다 |
| CLIP-017 | `E_UPLOAD_CLIP` 종류, 요청 `fpath` = `__CLIP__.<확장자>`, 결재 종류·패키지 번호·압축 비밀번호 없음 | P1 | 하 | 구현 |
| CLIP-018 | 업로드 임시 파일을 전송 뒤 지우지 않고 다음 로그인 때 `temp/` 째 지운다 | P1(개선해서) | 하 | 상충. Wails 판은 전송이 끝나면 곧바로 지운다. 로그인 때 정리도 남긴다(8절) |
| CLIP-019 | 사용자가 올리는 파일 이름에 `__CLIP__` 이 있으면 빼고 로그 | P1 | 하 | 구현. 원본의 바로 뒤 파일 누락 결함(CLIP-053)은 따라 하지 않는다 |
| CLIP-050 | 사용자 정보 창에 클립보드 업·다운로드 허용과 결재방식 표시 | P1 | 하 | 구현. 12 메인 창 쪽 화면. 표시 칸 불일치는 명세 확인 필요 1(4.1절) |

### 2.2 클립보드 다운로드·붙여넣기

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| CLIP-020 | [클립보드 다운로드] 단추. 0건이면 `다운로드 가능한 클립보드가 없습니다.` | P1 | 하 | 구현 |
| CLIP-021 | 툴바에 `클립보드 N 건`(상태 `다운로드 가능` 인 `__CLIP__` 행 수) | P1 | 하 | 구현. 서버 파일 목록 응답에서 센다 |
| CLIP-022 | 다운로드 권한 없으면 `클립보드 다운로드가 제한되어 있습니다.` | P1 | 하 | 구현 |
| CLIP-023 | `fpath="__CLIP__"` 로 요청해 서버가 건을 고르고, 첫 응답 code 로 확장자(1 txt, 2 png/bmp, 3 fmh) | P1 | 중 | 구현. 파일 받기 함수와 분리(6절) |
| CLIP-024 | 업로드 목록 잠금이 잡혀 있으면 안내 없이 버린다 | P1(개선해서) | 하 | 구현. Wails 판은 `앞 작업이 끝난 뒤 다시 시도하세요` 안내를 띄운다(8절) |
| CLIP-025 | 받기를 마치면 클립보드를 비우고 글·그림을 넣고 받은 임시 파일을 지운다 | P1 | 상 | 구현. Win32 쓰기(7.2절) |
| CLIP-027 | `.fmh` 인데 앞머리가 그림이면 그림으로 넣는다 | P1 | 하 | 구현. 보내는 쪽과 같은 앞머리 판정 함수 하나를 쓴다 |
| CLIP-028 | 묶음 앞머리를 모두 검사한 뒤에만 파일을 만든다 | P1 | 중 | 구현(보안 C-011). 7.4절 |
| CLIP-029 | 항목 이름은 파일명만, NUL 없음·`.`·`..`·`:`·장치 이름 거부, 클립 폴더 밖 거부 | P1 | 중 | 구현(보안 C-003) |
| CLIP-030 | 그림 항목 하나를 못 쓰면 그것만 건너뛰고, 0번(HTML)을 못 쓰면 묶음 거부 | P1 | 하 | 구현 |
| CLIP-031 | 받은 `dst.htm` 의 `MOSFM_CLIP_FOLDER`·다른 OS 절대 경로를 이 PC 경로로 바꾸고 정리(스크립트·바깥 참조 제거) | P1 | 상 | 구현. `clipCleanReceivedHtml` 이식 |
| CLIP-032 | 받은 `dst.htm`·그림을 지우지 않고 다음 로그인 때 지운다 | P1(개선해서) | 하 | 부분. Wails 판은 클립보드에 올린 뒤 지운다. 단, Windows 의 HTML 그림이 파일 경로를 가리키면 지우면 깨진다 → 7.2절의 data URI 방식으로 바꿔 지울 수 있게 한다 |
| CLIP-033 | 단축키로 받았으면 수정키·V 를 뗄 때까지(최대 5초) 기다렸다가 Ctrl+V(macOS Cmd+V) 흉내 | P1 | 상 | 구현. `SendInput`(7.5절). Linux 는 하지 않는다 |
| CLIP-034 | [파일전송] 받기는 받은 뒤 서버에서 지우고([3036], `fpath` `__CLIP__`), [결재처리] 받기는 지우지 않는다 | P1 | 하 | 구현 |
| CLIP-035 | 받기 완료를 로그아웃 뒤에 도착해도 처리(복호화된 내용이 폴더에 남지 않게) | P1 | 하 | 구현(작업 043) |
| CLIP-036 | 응답 `reason` 이 16진(32~128자)이면 검사값으로 SHA-256 대조, 다르면 `파일이 변조되었거나 손상되었습니다. Original(%1), Download_file(%2)` | P1 | 중 | 부분. 서버가 클립보드에 검사값을 보내는지 미확인(서버요청 SR008) |
| CLIP-037 | [파일 다운로드]로 클립보드 행은 ` 클립보드는 파일형태로 다운로드할 수 없습니다.`, 자동 다운로드는 건너뜀 | P1 | 하 | 구현. Wails `downloadBlock` 에 이미 있다(문구만 맞춘다) |
| CLIP-038 | [결재요청]에서 `__CLIP__` 행은 클립보드 건, 파일과 섞이면 `파일과 클립보드는 동시에 결재 요청 할 수 없습니다.` | P1 | 하 | 구현. 06 결재 |
| CLIP-039 | 결재자 창에서 모두 클립보드면 파일 칸·요약에 `클립보드`, 크기 비움, 결재방법은 클립보드 유형 | P1 | 하 | 구현. 06 결재 |
| CLIP-040 | [결재처리]에서 클립보드 건 [다운로드] 는 폴더를 묻지 않고 결재자 클립보드에 넣는다(붙여넣기 흉내 없음) | P1 | 중 | 구현. 06 결재 화면에서 부른다 |
| CLIP-041 | 목록 파일명 칸에 서버 이름(`__CLIP__.txt`) 그대로 | P1(개선해서) | 하 | 구현. Wails 판은 보이는 글자를 `클립보드(글)`·`클립보드(그림)`·`클립보드(서식)` 로 하고 접근성 id 에 서버 이름을 남긴다(보류 작업 073 = CLIP-054 를 함께 해결) |
| IMG-01 | 받은 그림을 PNG·JPEG·GIF·BMP·WEBP·TIFF 여섯 형식으로만 열고, 허용 밖·디코드 실패면 안내 창 | P1 | 중 | Qt 는 초안(작업 107). Go 는 형식별 디코더를 직접 고르므로 처음부터 이렇게 만든다. 안내 문구는 107 spec 이 정하면 맞춘다 |

### 2.3 단축키

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| CLIP-042 | [설정] `클립보드 단축키 설정`: `항상사용`/`사용안함`, 복사·붙여넣기마다 수정키(ALT·SHIFT·ALT+SHIFT·CTRL+SHIFT)와 한 글자, 바꾸면 곧바로 다시 등록 | P1 | 중 | 구현. 13 설정 화면. Qt 는 Linux 에서 칸을 숨긴다 |
| CLIP-043 | 로그인할 때마다 앱 설정(레지스트리, 없으면 `fmusr.ini [USER]`)을 읽어 다시 등록. 서버 정책으로 켜고 끄지 않는다 | P1 | 하 | 상충(이전 명세는 정책). 지금 코드대로 |
| CLIP-044 | 복사 단축키: 수정키·C 뗄 때까지(최대 5초) 기다린 뒤 Ctrl+C 흉내, 클립보드 바뀜 또는 500ms 뒤 업로드 | P1 | 상 | 구현. 바뀜은 `GetClipboardSequenceNumber` 로 본다(7.5절). Linux 는 흉내 없이 지금 클립보드를 올린다 |
| CLIP-045 | 붙여넣기 단축키: 단축키 표시(결재 유형 칸 100)를 단 받기, 완료 때 단축키 알림으로 갈라 붙여넣기 | P1 | 중 | 구현. Go 에서는 받기 요청 구조체에 `byHotkey bool` 을 두고 결재 유형 칸 100 흉내는 하지 않는다(서버로 가지 않는 앱 안 표시다) |
| CLIP-046 | 등록 실패해도 계속 돈다. QHotkey 경고만 | P1(개선해서) | 하 | 부분. Wails `GlobalShortcutManager.Register` 의 오류를 설정 화면에 `다른 프로그램이 이 단축키를 쓰고 있습니다` 로 보인다 |
| CLIP-047 | 단축키로 보낸 것과 단추로 보낸 것을 화면에서 구분하지 않는다(로그로만) | P2 | 하 | 부분. 전송 상태 행에 `(단축키)` 표시를 더하는 정도 |

### 2.4 Link Redirect(업무망 → 인터넷망 URL 전달)

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| LINK-001 | 포트 29000 하나에만 창구, 생성·상태를 한 객체가 맡는다 | P1 | 하 | 구현. Go `net/http` 서버 하나 |
| LINK-002 | 127.0.0.1 과 ::1 두 소켓, 한쪽만 열리면 `- 한쪽만 열렸습니다.` 경고 후 계속 | P1 | 하 | 구현 |
| LINK-003 | 앱 시작 때 정책·로그인과 무관하게 연다 | P1 | 하 | 상충. 확장이 늘 붙으므로 그대로 둔다. 처리기에서 로그인 여부를 본다(8절) |
| LINK-004 | 둘 다 실패하면 `LinkRedirect httpServer listen failed port[29000]` + `링크연동을 사용할 수 없습니다 - 나머지 기능은 정상 동작합니다.` | P1 | 하 | 구현 |
| LINK-005 | 로그아웃해도 닫지 않고 앱 종료 때 정리 | P1 | 하 | 상충. 그대로 |
| LINK-006 | `/` 요청 본문을 처리기에 넘기고 참이면 `200 LinkRedirect`, 거짓이면 `205` | P1 | 하 | 구현. 개선: 본문 상한(8절) |
| LINK-008 | `/` 외 요청은 404 + `LinkRedirect httpServer 처리되지 않은 요청(404):` | P1 | 하 | 부분. 그대로 |
| LINK-009 | 요청마다 메서드·주소·URL·본문 앞 120자 디버그 로그 | P2 | 하 | 구현. 개선: URL 대신 호스트만 남긴다(사용자 방문 기록 노출 줄임) |
| LINK-010 | 화면에서 고른 서버가 업무망이 아니거나 `fm_hyperlink_mode` 가 없거나 0 이면 200 | P1 | 하 | 구현 |
| LINK-011 | `redirectUrl=` 뒤를 소문자·trim, 문자열 자르기로 호스트 추출(`black` 과 그 밖이 다르게 자른다) | P1 | 중 | 구현. 코드는 `lastIndexOf`(보안 D-007 조치)로 명세의 "처음 나오는" 보다 앞서 있다. Go 는 `net/url` 로 호스트를 뽑되 판정 결과가 같은지 시험표로 맞춘다(7.6절) |
| LINK-012 | 필터 항목이 호스트에 부분 문자열로 들어 있으면 걸림. `black` 이면 안 걸린 것, 그 밖이면 걸린 것이 대상 | P1 | 중 | 구현. Wails 판 white 는 항목과 같거나 그 하위 도메인일 때만 걸린다(8.1절). black 은 원본 그대로 |
| LINK-013 | 로그인 완료·트레이 [정책 받아오기] 때 [3094] `GetUrlFilter`(code 0), 항목 `url` 만 | P1 | 하 | 구현. Wails 판은 `flag`(SBM 적용 범위)도 판정에 쓴다(8.1절) |
| LINK-014 | 대상이고 URL 에 호스트가 있으면 [3204] `SendURL`, 응답을 기다리지 않고 205 | P1 | 하 | 구현 |
| LINK-016 | [3204] 응답 code 4·res≠1 이면 `요청 처리 오류` 창에 `링크 연동에서 악성코드가 발견되었습니다.`, 그 밖 실패는 로그 | P1 | 하 | 구현 |
| LINK-018 | 로그인 완료 때 `LinkRedirect 정책 location[..] fm_hyperlink_mode[..] ...` 와 가지 로그 | P2 | 하 | 구현. 진단용 |
| LINK-020 | 확장: 페이지 로드 완료마다 제목·URL 을 POST(`~~docTitle=<제목> redirectUrl=<URL>`) | P1 | 하 | 구현. 확장은 그대로 쓴다(앱 쪽 형식 호환) |
| LINK-021 | 확장 안내 페이지 `인터넷망 분리 안내` 3초 뒤 닫기, `fm_hyperlink_notice0` 는 로그에만 | P2 | 하 | 부분. 확장 쪽 |
| LINK-022 | 확장의 `ERR_TUNNEL_CONNECTION_FAILED` 처리기 결함(정의 안 된 `tab`) | P2 | 하 | 부분. 확장을 고칠 때 함께 |
| LINK-024 | 알림 9004 `MSG_KIND_URL_SEND`(4) 받으면(userid 가 있고 내 ID 와 다르면 버림) `dat` 를 기본 브라우저로 연다. http/https·호스트만 | P1 | 하 | 구현. Wails `application.Get().Browser.OpenURL`. Wails 판은 사이트마다 로그인당 한 번 사용자에게 묻는다(원본은 묻지 않는다, 8.2절). 09 알림과 함께 |
| LINK-025 | [정책] 화면 유형 `LinkRedirect` 에 URL 필터 목록 한 행씩 | P2 | 하 | 구현. [정책] 화면을 만들 때 |

### 2.5 명령줄·단일 인스턴스·파이프·오른쪽 메뉴

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| LINK-026 | 설치본이 오른쪽 메뉴 셋 등록(파일 업로드, 폴더 다운로드, 폴더 빈 곳 다운로드) | P1 | 중 | 구현. Windows 만. 15 설치본과 함께(7.8절) |
| LINK-027 | 폴더를 오른쪽 메뉴로 올리는 길이 없다(폴더 메뉴는 다운로드) | P2 | 중 | 상충. 폴더 업로드(03)가 생기면 `Directory\shell\SNBUploadDir` 를 더한다 |
| LINK-028 | `--mode <upload|download|noupdate>`(`--mode=값` 포함)와 첫 위치 인자 하나만 경로 | P1 | 하 | 구현. 개선: 경로 여럿을 모두 받는다 |
| LINK-029 | 인자나 원 명령줄 어디에든 `noupdate`(대소문자 무시)면 업데이트를 끈다 | P1 | 하 | 구현. 경로에 `noupdate` 가 든 오판은 따라 하지 않는다(`--mode` 값만 본다). 업데이트(16)와 함께 |
| LINK-030 | 공유 메모리 `MOSFM 6.0` 으로 떠 있는지 판정, 보통 약 0.5초·`noupdate` 면 약 5초 기다림 | P1 | 중 | 구현. Wails `SingleInstance` 는 뮤텍스로 즉시 판정한다. `noupdate` 의 "앞 앱이 끝나길 기다림" 은 따로 만든다(7.7절) |
| LINK-031 | 떠 있으면 `upload`·`download` 는 파이프 `FileMoverActivation` 에 `{"mode","path"}` 보내고 끝 | P1 | 중 | 구현. Wails 는 WM_COPYDATA 로 인자 전체를 넘긴다 |
| LINK-032 | 그 밖 모드면 `snB App 프로그램이 이미 실행중입니다.`(제목 `정보`) 뒤 `Activate`, 기존 앱은 보이는 창만 올린다 | P1(개선해서) | 하 | 부분. Wails 판은 경고 창 없이 기존 창을 트레이에서 꺼내 앞으로 올린다(8절) |
| LINK-033 | 파이프 조각을 JSON 이 될 때까지 모으고 1MB 넘으면 버림 | P1 | 하 | 구현. WM_COPYDATA 는 한 번에 오므로 크기 상한만 둔다 |
| LINK-034 | IPC `upload` 를 모아 400ms 단발 타이머 뒤 한 번에 업로드 입구로 | P1 | 중 | 구현 |
| LINK-035 | 앞 업로드가 사람의 답을 기다리면 400ms×최대 75번(30초) 미루고, 넘으면 `IPC 업로드 대기 한도 초과` 뒤 입구로 | P1 | 중 | 구현(작업 057) |
| LINK-036 | IPC `download` 경로 하나만 보관(덮어씀), 타이머 뒤 체크된 행을 그 폴더로 | P1 | 하 | 구현 |
| LINK-037 | 앱이 꺼져 있으면 그 프로세스가 본체가 되어 보관, 첫 로그인 완료 때 한 번 실행 | P1 | 하 | 구현 |
| LINK-038 | IPC 업로드는 일반 업로드와 같은 입구(파일 수 상한, 권한, 검사, 목록 주인) | P1 | 하 | 구현. 03 업로드의 입구를 그대로 부른다 |
| LINK-039 | 로그아웃 동안 들어온 IPC 업로드는 안내·로그 없이 버림 | P1(개선해서) | 하 | 구현. Wails 판은 보관했다가 다음 로그인 때 실행하거나 최소한 로그를 남긴다(8절) |
| LINK-043 | 오른쪽 메뉴는 Windows 설치본에만. 명령줄·단일 인스턴스는 모든 데스크톱 | P1 | 하 | 구현. macOS 는 Wails `SingleInstance` 가 되고, Finder 메뉴는 없음(그대로) |

### 2.6 알림·NBMT 연계(명세 08 에 있으나 09·14 가 만든다)

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| LINK-044 | 알림 9001(목록 갱신)은 userid 가 비었거나 내 ID 면 1초 뒤 목록 재조회 | P1 | 하 | 구현. 09 알림. 대소문자 비교 결함은 8절 |
| LINK-045 | 검사 중 항목이 남으면 1·2·4초, 그 뒤 8초 간격 재조회 | P1 | 중 | 구현. 09 알림 또는 서버 파일 화면 |
| LINK-046 | 알림 9005 → 관리자 요청 무결성 검사 | P1 | 하 | 구현. 14 무결성 |
| LINK-047 | NBMT 결과는 서버파일 목록[3311]의 `virus_flag`·`priv_state` 로만 읽는다 | P1 | 하 | 구현. 04 다운로드의 막기 규칙이 이미 그렇다 |

### 2.7 도메인 정책(hosts 파일 반영)

명세 07·08 에 요건 번호가 없다. 근거는 `src/domain/DomainPolicy.cpp`·`HostsBlock.h`·`tst_DomainPolicy`, 명세 01 인증의 AUTH-012·AUTH-034, 12 메인 창의 MAIN-072 다.

| ID | 기능 | Wails 판 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| DOM-01 | 로그인 완료 때 업무망이고 `fmenv.ini [CONFIG] DOMAIN_POLICY` 가 참이면 [3230] `GetDomainPolicy` | P1 | 하 | 구현 |
| DOM-02 | 응답 건수 > 0 이면 hosts 의 ssBridge 구역을 지우고 새로 쓴다(바이트 그대로, 원본 줄바꿈 따름, 임시 파일 후 바꿔치기) | P1 | 중 | 구현. 관리자 권한 문제(6·7.9절) |
| DOM-03 | 앱 종료 때 구역을 지운다 | P1 | 하 | 구현. Wails `OnShutdown` |
| DOM-04 | 행 검사: IP 는 16진·`:`·`.` 만이고 유효 주소, 이름은 `[A-Za-z0-9.-]+`, 설명은 제어문자 제거 | P1 | 하 | 구현. hosts 에 임의 줄을 넣지 못하게 막는 보안 경계 |
| DOM-05 | 실패 시 `도메인 등록 - Hosts 파일 열기에 실패했습니다.`·`도메인 삭제 - …`·`hosts 디렉토리를 찾는데 실패했습니다.`(제목 `정보`) | P1 | 하 | 구현 |

### 2.8 만들지 않는 것과 미구현·보류 묶음

- **제외(WASM 전용)**: CLIP-051(브라우저 클립보드 API 로 읽기), CLIP-052(브라우저 클립보드로 쓰기) ― Wails 판은 데스크톱만 만든다.
- **Qt 판 미구현·보류(한 줄로 묶음)**: CLIP-010(크기 초과 안내에 상한·실제 크기 표시), CLIP-012(클립보드 개인정보 차단), CLIP-026(덮어쓰기 전 알림),
  CLIP-048(클립보드 전용 실패 문구), CLIP-049(탐색기에서 복사한 파일을 업로드로), CLIP-053(보류 061, `__CLIP__` 뒤 파일 누락 결함 수정),
  CLIP-054(보류 073, 목록에 `클립보드` 로 표시), LINK-007(확장 출처 확인), LINK-015(보내기 전 로그인 확인), LINK-017(링크 악성 알림 신호),
  LINK-019(PAC 방식), LINK-023(확장 배포), LINK-040(알 수 없는 IPC 모드 거부 로그), LINK-041(외부 업무 시스템 REST 창구), LINK-042(GPKI 인증서 전달).
  이 가운데 Wails 판이 비용 없이 함께 하는 것: CLIP-010(문구에 크기 덧붙이기), CLIP-053(같은 결함을 만들지 않음), CLIP-054(CLIP-041 개선),
  LINK-007(POST 만 받고 Host 는 루프백 이름만, Origin 이 있으면 확장 스킴만 받는다. 확장 ID 는 고정되지 않아 보지 않는다. Origin 이 없는 요청은 막지 않는다: 2026-10-31 실측에서 확장은 `chrome-extension://…`(Edge 156), 웹 페이지는 `no-referrer` 여도 페이지 주소(Edge·Chrome 156)를 보내 브라우저에서는 빈 값이 나오지 않았고, 로컬 프로그램은 Origin 을 꾸밀 수 있어 거부해도 막히는 공격이 없다),
  LINK-015(처리기에서 로그인 확인), LINK-040(모르는 모드 로그). PAC(LINK-019)·REST(LINK-041)·GPKI(LINK-042)는 제외 ― Qt 판도 없고 결정이 없다.
- **시험 진입점**: IPC `mode=drop·aprv-hold·auto-upload·notify·clip-recv·check-pop-delay·extract-limits·probe` 는 Qt 판 E2E 전용(`SNB_TEST_HOOKS`)이라
  제외. Wails 판 E2E 가 필요하면 그때 따로 정한다.

---

## 3. 서버 통신

모든 RPC 는 이미 있는 gRPC 연결(`Session.conn`, TLS, `auth-token` 메타데이터)을 쓴다. proto 는 `E:\git\snBv6FM.Wails\proto` 사본, Go 코드는 `pb/`.

### 3.1 클립보드 업로드 ― `ServerFileService.FileUpload`(클라이언트 스트림)

순서
1. 앱 안 검사: 로그인 → 권한(4.1절) → 업로드 목록 주인 → 임시 파일 쓰기 → 크기(`fm_clip_size`) → 결재 갈래.
2. 첫 메시지 `FileUploadRequest{info: FileTransInfo}`.
3. 512KB 조각 `FileUploadRequest{chunk}` 반복(Wails `uploadChunk` 와 같다).
4. `CloseSend` 뒤 `AnswerResponse` 를 읽는다. `res != 1` 이면 실패. 도중 실패는 스트림을 cancel 한다(Wails `Upload` 와 같다, 서버요청 SR002).

`FileTransInfo` 에 싣는 칸(원본 `FileTransfer.cpp` ~1094-1139)

| 칸 | 값 |
|---|---|
| `fsize` | 임시 파일 크기 |
| `fpath` | `"__CLIP__." + 소문자 확장자` ― `__CLIP__.bmp`·`__CLIP__.fmh`·`__CLIP__.txt`(WASM 판은 `__CLIP__.png`) |
| `resv` | 사후(`isAfter`, `fm_upfile_mode=1`) 면 `"2"`, 아니면 `"1"` |
| `aprv_type` | 클립보드 결재 유형: 업무망 `fm_inclup_aprv`, 인터넷망 `fm_otclup_aprv`. 사전 갈래에서는 원본이 칸을 채우지 않는다(명세 확인 필요 2) |
| `aprv_dept` | 사용자 부서명(로그인 응답 `UserDeptName`) |
| `approver_id` | 결재자 창에서 고른 ID 목록. 자가(6)면 자기 ID(Wails `Upload` 와 같은 규칙) |
| `fcause` | 결재 사유(결재자 창에서 입력) |
| `checksum` | 정책 `fm_checksum` 이 `"0"` 이 아니면 SHA-256 16진 |
| `pkg_no`·`aprv_kind`·`dir_flag`·`zip_pswd` | 싣지 않는다(0/빈 값) |

주의: Wails `Upload()` 의 `resv` 규칙(결재자가 있으면 "1", 없으면 "2")과 원본 클립보드 규칙(사후면 "2")은 표현이 다르다. 클립보드는 원본 규칙을 따른다 ―
클립보드 결재 유형으로 `aprv_type` 을 정하는 것도 파일과 다르므로 `Upload()` 를 `uploadSpec{fpath, aprvType, resv, approvers}` 를 받는 내부 함수로 쪼개 함께 쓴다.

### 3.2 클립보드 다운로드 ― `ServerFileService.FileDownload`(서버 스트림), 명세는 다운로드 요청[3032]

요청
- [파일전송] 받기: `FileDownloadRequest{fpath: "__CLIP__"}` ― 어느 건을 줄지 서버가 고른다(가장 오래된 것, 추정: 서버 코드 미확인).
- [결재처리] 받기: `FileDownloadRequest{spath: <서버 파일명>}`.

응답(oneof `answer`·`chunk`) 처리(원본 `FileTransfer.cpp` 1545-1760)
1. 첫 `answer`: `res != 1` 이면 실패(`reason` 을 사유로 보인다).
2. `reason` 이 16진 32~128자면 검사값으로 보관(`isLikelyFileChecksumHex`). 아니고 아직 받은 바이트가 0 이면 `reason` = 서버 경로로 보관.
3. `code` 로 확장자: 1 → `.txt`, 2 → 서버 경로에 `.png` 가 있으면 `.png` 아니면 `.bmp`, 3 → `.fmh`.
4. `length` 는 서버의 **암호화된** 크기라 받은 바이트보다 크다(실측 1000 ↔ 1103). 클립보드는 크기 대조를 하지 않는다.
5. `chunk` 를 `<클립 폴더>/__CLIP__<확장자>.part` 에 쓴다. 끝나면 검사값 대조(있을 때) → 이름 바꾸기.
6. [파일전송] 받기면 `RemoveFile(FileRemoveRequest{spath: <2단계의 서버 경로>, flag: 1, fpath: "__CLIP__"})`[3036]. [결재처리] 받기는 지우지 않는다.

`AnswerResponse` 칸 뜻(proto 주석): `res`(1 정상), `code`(클립보드 1 txt · 2 bmp · 3 fmh), `reason`, `length`, `txt`.
**파일 받기에서는 code 3 이 "reason 이 검사값" 이라는 표시**(Wails `client.go:422`)라 같은 칸이 다른 뜻이다.

### 3.3 클립보드 건수 ― 서버파일 목록[3311] `GetServerFileList`

Wails `Files()` 응답에서 `NameStr` 에 `__CLIP__` 이 있고 상태 글자가 `다운로드 가능` 인 행을 센다. 상태 글자는 원본 `FileMoverPolicy::getState(aprv_state, priv_state, virus_flag)` 가 만드는
값이라 Go 로 같은 함수를 옮겨야 한다(04 다운로드와 공유). 추정: 결재 승인(`State` 1)·백신 통과(`VirusFlag` 1·4)·개인정보 통과일 때다.

### 3.4 클립보드 [결재요청] ― 업로드 후 결재요청[3127]

06 결재가 만든다. 클립보드 건은 `aprv_type` 에 클립보드 결재 유형을 싣고 `zip_pswd` 는 받은 값을 그대로 돌려보낸다(원본 `approvalRequest` 2651, NotifySlot 1537-1543).
결재자 목록·부서 조회 [3250]·[3252] 도 06 이 만든다.

### 3.5 URL 필터 ― `CommonService.GetUrlFilter` [3094]/[3095]

- 요청 `UrlFilterRequest{code: 0}`(0 = filemover).
- 응답 `UrlFilterResponse{count, list: []UrlFilterInfo{code, flag, name, url}}`. 원본은 `url` 만 모아 정책의 URL 목록을 **통째로 바꾼다**. Wails 판은 `flag` 까지 보관한다. 서버는 `flag` 로 거르지 않고 모든 행을 보낸다.
- 부르는 때: 로그인 완료, 트레이 [정책 받아오기].
- 로그 원문: 성공 `LinkRedirect UrlFilter 목록 조회[3095] 응답 건수:`, 실패 `LinkRedirect UrlFilter 목록 조회[3094] 실패!!!`.

### 3.6 URL 보내기 ― `CommonService.SendURL` [3204]

- 요청 `URLInfo{url: <소문자 URL>}`. 응답 `AnswerResponse`.
- 앱은 응답을 기다리지 않고 확장에 205 로 답한다(LINK-014). 응답은 비동기로 받아:
  - `code == 4 && res != 1` → `E_NOTIFY_ERROR` 문자열 `3204\\링크 연동에서 악성코드가 발견되었습니다.` → 화면에 제목 `요청 처리 오류` 창, 본문 `링크 연동에서 악성코드가 발견되었습니다.`
  - 그 밖의 실패 → 로그 `링크 연동 실패`. 통신 실패는 `LinkRedirect URL 전송[3204] 실패!!!`.
- 성공 로그: `인터넷망으로 URL전송[%1]`.

### 3.7 URL 받기 ― 알림 9004, kind 4(`MSG_KIND_URL_SEND`)

- `NotifyService` 스트림의 `NotifyMessage{notify_svcno: 9004, kind: 4, userid, dat}`.
- `userid` 가 있고 내 ID 와 다르면 버린다(원본은 대소문자를 가린다, `FileMoverGrpc.cpp:2516,2549`).
- `dat` 를 URL 로 읽어 http/https 이고 호스트가 있으면 로그 `업무망에서 전달된 URL 수신:` 을 남긴다. 아니면 경고 `업무망에서 전달된 URL 을 열지 않았다(http/https 주소가 아니다)` 로 끝낸다.
- Wails 판은 그 뒤 OS 질문 창(제목 `링크 열기`, 단추 [열기]·[취소], 기본 단추 [취소])으로 사이트와 주소를 보이고 [열기] 를 누르면 기본 브라우저로 연다. 원본은 묻지 않고 바로 연다(8.2절).
  허락한 호스트는 로그아웃까지 다시 묻지 않는다. 창이 떠 있는 동안 온 새 호스트의 주소는 묻지 않고 버리며 정보 로그 `업무망에서 전달된 URL 을 열지 않았다(거절했거나 다른 확인 창이 떠 있다)` 를 남긴다.
- `CommonService.RecvURL` 은 proto 에만 있고 원본은 부르지 않는다(검색: `RecvURL` 호출 0건). Wails 판도 부르지 않는다.
- 같은 9004 의 kind 0(`MSG_KIND_FILE_LIST`)은 1초 뒤 목록 재조회. 원본은 `break` 가 없어 kind 1(결재) 처리로 이어진다 ― 의도인지 불분명하다(8절).

### 3.8 도메인 정책 ― `CommonService.GetDomainPolicy` [3230]/[3231]

- 요청 `DomainPolicyRequest{userid}`. 응답 `DomainPolicyResponse{count, list: []DomainInfo{profile_name, domain_name, domain_ip}}`.
- 원본은 서버 문자열을 EUC-KR 로 보고 바꾼다(`safeStrCopy`). **Go protobuf 는 proto3 `string` 칸에 UTF-8 이 아닌 바이트가 오면 메시지 전체를 거부한다**(`proto: invalid UTF-8`).
  `profile_name` 에 한글 EUC-KR 이 오면 응답 전체가 실패한다. 대책은 6절·7.9절.

### 3.9 오류 코드·문구 원문 모음

| 상황 | 원문 | 근거 |
|---|---|---|
| 클립보드 비었음 | `저장된 클립보드가 없습니다.`(제목 `정보`) | `ViewController.cpp:1333-1347` |
| 업로드 권한 없음 | `클립보드 업로드가 허용되어 있지 않습니다.` | CLIP-005 |
| 앞 업로드 대기 | `앞 업로드를 마친 뒤 다시 시도하세요.` | `src/README.md`(057) |
| HTML 묶음 폴더 없음 | `폴더가 존재하지 않아서 진행할수 없습니다.\n%1` | `ViewController.cpp` clipboardUpload |
| 크기 초과 | `전송 가능한 파일 크기 초과` | `getPolicyErrText(E_FILE_SIZE_OVER)` |
| 받을 것 없음 | `다운로드 가능한 클립보드가 없습니다.` | CLIP-020 |
| 다운로드 권한 없음 | `클립보드 다운로드가 제한되어 있습니다.` | CLIP-022 |
| 파일로 받기 시도 | ` 클립보드는 파일형태로 다운로드할 수 없습니다.`(앞 공백 포함) | CLIP-037 |
| 검사값 불일치 | `파일이 변조되었거나 손상되었습니다. Original(%1), Download_file(%2)` | `FileMoverGrpc.cpp:256` |
| 결재요청 섞임 | `파일과 클립보드는 동시에 결재 요청 할 수 없습니다.` | CLIP-038 |
| 결재요청 목록 없음 | `결재요청 가능한 목록이 없습니다.` | `approvalRequest` 2651 |
| `__CLIP__` 이름 파일 | `사용할 수 없는 파일명이 포함되어 있어 업로드에서 제외되었습니다.( __CLIP__ )`(로그) | CLIP-019 |
| 링크 악성코드 | `링크 연동에서 악성코드가 발견되었습니다.`(제목 `요청 처리 오류`) | LINK-016 |
| 이미 실행 중 | `snB App 프로그램이 이미 실행중입니다.`(제목 `정보`) | LINK-032 |
| IPC 파일 수 초과 | `파일 최대 전송 개수를 초과해서 전송할 수 없습니다.\n( 파일 최대 전송개수 : %1 )` | `uploadFiles` |
| IPC 업로드 권한 없음 | `현재 망에서는 업로드할 수 없습니다.` | `uploadFiles` |
| IPC 대기 한도 | `IPC 업로드 대기 한도 초과 - 앞선 업로드가 답을 기다리는 중이라 이번 업로드를 받지 않습니다.`(로그) | LINK-035 |
| hosts 실패 | `도메인 등록 - Hosts 파일 열기에 실패했습니다.` / `도메인 삭제 - Hosts 파일 열기에 실패했습니다.` / `hosts 디렉토리를 찾는데 실패했습니다.` | `DomainPolicy.cpp` |

---

## 4. 정책 키

### 4.1 로그인 응답 칸(클립보드)

| 칸(proto `LoginResponse`) | 원본 저장 이름 | 뜻 | 기본값 |
|---|---|---|---|
| `fm_clip_down`(4, 주석 "클립보드 내부->외부") | `fm_clip_up` 으로 **바꿔** 담는다(`FileMoverGrpc.cpp:984`) | 아래 판정표 | 0(막음) |
| `fm_clip_up`(5, 주석 "외부->내부") | `fm_clip_down` 으로 바꿔 담는다 | 아래 판정표 | 0 |
| `fm_clip_size`(14) | `fm_clip_size × 1024` | 클립보드 상한(KB), 0 = 무제한 | 0 |
| `fm_inclup_aprv`(7) | 그대로 | 업무망 클립보드 업로드 결재 유형 | 0 |
| `fm_otclup_aprv`(9) | 그대로 | 인터넷망 클립보드 업로드 결재 유형 | 0 |
| `fm_clip_speed` | 그대로 | 받아 두기만 한다 | ― |

결재 유형 값: 0 자동승인, 1 사전, 3 사후, 4 요청 시 선택, 6 자가, 7 전체(선택).

**판정을 proto 칸 이름으로 다시 쓴 표**(원본 `loginrespolicy.cpp:97-112` 의 `checkAllowTransfer` 와 984 줄의 맞바꿈을 합친 결과)

| 지금 망 | 클립보드 업로드 허용 칸 | 클립보드 다운로드 허용 칸 |
|---|---|---|
| 업무망(loc 1) | proto `fm_clip_down`(내부→외부) | proto `fm_clip_up`(외부→내부) |
| 인터넷망(loc 2) | proto `fm_clip_up`(외부→내부) | proto `fm_clip_down`(내부→외부) |

즉 "업무망에서 올리고 인터넷망에서 받는" 흐름은 둘 다 proto `fm_clip_down` 이 연다. 방향 이름과 맞으므로 Go 에서는 맞바꿈 없이 이 표 하나로 쓴다:
`allowClip(up bool) = (loc=="1") == up ? FmClipDown : FmClipUp`. 시험표(4칸)를 함께 둔다.

사용자 정보 창 표시(`getAuthority`)는 위치와 무관하게 업로드 = 원본 `fm_clip_down`(= proto `fm_clip_up`), 다운로드 = 원본 `fm_clip_up`(= proto `fm_clip_down`)이라
업무망에서 판정과 표시가 반대 칸을 본다(명세 확인 필요 1). Wails 판은 **표시도 판정표로** 한다 ― 화면에 보이는 허용 여부가 실제 동작과 같아야 한다.
MOSFM 대조 결과가 나오면 표 하나만 고친다.

### 4.2 프로파일 정책(`profile2`, `GetProfile` 의 `k=v,k=v`)

| 키 | 값 | 없을 때 | 쓰는 곳 |
|---|---|---|---|
| `fm_upfile_mode` | 0 결재자 사전 선택, 1 사후 선택 | 0 | CLIP-013·014 |
| `self_approve` | 자기 자신을 결재자로 허용 | ― | 결재자 창(06) |
| `fm_checksum` | `"0"` 이면 검사값 끔 | 켠 것 | 업로드 `checksum` 칸 |
| `fm_hyperlink_mode` | 0·없음 = 사용 안 함, 그 밖 = 사용(주석상 2 = 일반) | 0 | LINK-010 |
| `url_check_method` | `white` / `black`. 처리기 판정은 `black` 대소문자 무시, 그 밖은 `white`(`white` 의 도메인 일치는 8.1절) | white | LINK-011·012 |
| `fm_hyperlink_forcepac` | 1 = PAC 강제. 원본은 경고 로그만 | ― | LINK-019(제외) |
| `fm_hyperlink_pacpath0`·`pacpath1`·`fm_hyperlink_notice0` | 로그에만 | ― | LINK-018·021 |
| `fm_dlp_*` | `SAMSUNG_CARD` 빌드에서만, 차단은 주석 | ― | CLIP-012(제외) |

`profile2` 는 원본에서 로그아웃해도 비우지 않는다(`filemoverpolicy.cpp:78,93`). Wails 판 `Session` 은 로그아웃 때 세션째 버리므로(`AppService.Logout`) 로그아웃 뒤 Link Redirect 판정이
자연히 멈춘다 ― 원본의 "로그아웃 뒤 205 를 답하고 전송은 실패" 를 따라 하지 않는다.

### 4.3 서버 목록·설정 파일

| 자리 | 키 | 뜻 | 기본값 |
|---|---|---|---|
| `fmenv.ini [SERVERxx]` | `LOC` | 1 업무망, 2 인터넷망. Link Redirect 는 1 에서만, 도메인 정책도 1 에서만 | ― |
| `fmenv.ini [CONFIG]` | `DOMAIN_POLICY` | 참이면 도메인 정책 조회 | 거짓(추정: 키가 없으면 하지 않는다) |
| 레지스트리 `HKCU\Software\SQIsoft\snB App` | `clipboardHotkeyUse`(bool), `clipboardCopyKey`, `clipboardCopyChar`, `clipboardPasteKey`, `clipboardPasteChar` | 단축키 사용·키 | false, `ALT`, `C`, `ALT`, `V` |
| `<앱 자료>/fmusr.ini [USER]` | 같은 이름 | 레지스트리에 없을 때 읽는다 | 같음 |
| 상수 | 포트 29000, 파이프 `FileMoverActivation`, 공유 메모리 `MOSFM 6.0` | 바꿀 수 없음(확장·설치본·옛 앱과 맞춘다) | ― |

Wails 판 설정 저장: 13 설정 문서가 정하는 저장소를 쓰되, **처음 실행 때 위 레지스트리 값을 한 번 읽어 옮긴다**(Qt 판 사용자가 고른 단축키를 잃지 않게).
레지스트리 읽기는 `golang.org/x/sys/windows/registry`.

---

## 5. 화면 동작

### 5.1 [파일전송] 화면(`content/ServerFileScrollView.qml`)

| 요소 | 원본 | Wails 판 |
|---|---|---|
| [클립보드 업로드] | 285 줄, `FmBusi.clipboardUpload()`. 좁은 카드 목록에도 있다(1415) | `App.svelte` 툴바 단추 → `AppService.ClipboardUpload()` |
| 툴바 건수 | 322 줄, `qsTr("클립보드") + "\n" + count + " 건"`, 아이콘 | 같은 글자. 건수는 목록 새로 고침마다 Go 가 계산해 이벤트로 보낸다 |
| [클립보드 다운로드] | 1284 줄, `FmBusi.clipboardDownload(false)` | `AppService.ClipboardDownload()` |
| 목록 파일명 | 서버 이름 `__CLIP__.txt` 그대로(CLIP-041) | `클립보드(글)` 등으로 보이고 `title`·`data-name` 에 서버 이름(8절) |
| 전송 상태 행 | 이름 `클립보드`, 크기, `업로드`/`다운로드`, `대기중`/`다운로드 대기중` | 같은 글자 |

### 5.2 [설정] 단축키 칸(`content/SettingsScrollView.qml` 623-819, 1044-1068; `settings/AppSettings.qml` 85-89)

- Pane 제목 `클립보드 단축키 설정`. wasm·linux 에서 숨긴다(645 줄).
- 라디오 `항상사용`/`사용안함`.
- 줄 두 개 `복  사`·`붙여넣기`: 수정키 콤보(`ALT`·`SHIFT`·`ALT+SHIFT`·`CTRL+SHIFT`) + 한 글자 `TextField`(`maximumLength: 1`).
- 값이 바뀌면 `changedHotkey()` → `FmBusi.changeHotKey(use, copyKey, pasteKey)` 로 곧바로 다시 등록. 시퀀스 문자열은 `"ALT+C"` 모양.
- Wails 판: 같은 칸. 글자 칸은 영문·숫자 한 자만 받고 대문자로 맞춘다(원본은 아무 글자나 받아 등록 실패가 조용히 묻힌다, 추정).
  등록 결과를 칸 아래에 보인다(`등록됨` / `다른 프로그램이 이미 쓰는 단축키입니다`). 복사·붙여넣기 키가 같으면 저장을 막는다.
  Wails 가속기 문자열은 `"Alt+C"`·`"Alt+Shift+C"`·`"Ctrl+Shift+V"` 형식으로 바꿔 넘긴다.

### 5.3 결재자 선택 창(06 결재)

클립보드 업로드가 사전 선택 갈래로 가면 결재자 창이 뜬다. 파일 칸 행마다 `클립보드`, 요약 `클립보드`, 크기 비움, 결재방법은 클립보드 결재 유형(NotifySlot 1364-1392).
창 없이 보내는 경우: 클립보드 결재 유형이 0(자동) 또는 6(자가).

### 5.4 안내 창

모든 안내는 제목 `정보` 의 확인 창(원본 `popMessageBox`). 링크 악성코드만 제목 `요청 처리 오류`. Wails 판은 공용 확인 창 컴포넌트 하나로 띄우고,
E2E 가 읽을 수 있게 본문에 고정 접근성 이름(원본 `messageDialogText` 와 같은 역할)을 준다.

### 5.5 Link Redirect·IPC 는 화면이 없다

- 확장 안내 페이지(`imports/ssbridgeExtension/Notice/RedirectNotice.html`): 제목 `인터넷 접속 안내`, h2 `인터넷망 분리 안내`, 본문 `접속하신 사이트가 인터넷 PC에서 자동으로 열립니다.`, 그림 `image2.png`, `notice.js` 가 3초 뒤 `window.close()`.
- 두 번째 실행: 원본은 두 번째 프로세스가 경고 창을 띄운다(`QMessageBox::warning`). Wails 판은 첫 앱의 창을 올린다(8절).

---

## 6. 경계·오류 상황

클립보드
1. **다른 프로그램이 클립보드를 쥐고 있음**: `OpenClipboard` 가 실패한다. Qt 는 50ms 간격 3번 재시도(`Retrying to obtain clipboard.`), 마지막 실패만 문제(`Unable to obtain clipboard.`).
   Go 도 같은 재시도(50ms×3, 그 뒤 `클립보드를 다른 프로그램이 쓰고 있습니다. 잠시 뒤 다시 시도하세요.`). 비울 때 주인이 응답하지 않으면 Windows 가 최대 5초 기다린다 ― UI 스레드에서 하지 않는다.
2. **클립보드에 파일 목록(CF_HDROP)만 있음**: 원본은 `저장된 클립보드가 없습니다.`(CLIP-049). 같게 둔다.
3. **서식 묶음 앞머리만 13,264바이트**: 상한이 1KB 면(인증시험 0039) 짧은 서식 글도 늘 크기 초과다. 동작은 같게 두고, 안내 문구에 상한·실제 크기를 덧붙인다(CLIP-010).
4. **그림이 비압축 BMP**: 화면 캡처 한 장이 수 MB(1920×1080×4 ≈ 8MB). 크기 상한에 쉽게 걸린다. 8절 PNG 안 참고.
5. **묶음 만들기 실패(0 바이트)**: 원본 데스크톱은 그래도 보낸다(`ClipHtml.cpp:243-254`). Wails 판은 멈추고 안내한다(WASM 판 동작과 같다).
6. **HTML 안 원격 그림이 닿지 않음**: 그림 한 장마다 최대 5초. 원본은 이벤트 루프로 기다려 단추가 안 눌린다. Go 는 그림을 동시에 받고(최대 50장, 동시 4~8) 전체 시한을 둔다(8절).
7. **Windows 에서 SourceURL 이 사라짐**: Qt 의 `QMimeData::html()` 은 CF_HTML 머리를 떼어 `SourceURL:` 이 없어진다(Qt `qwindowsmimeregistry.cpp` 721-756).
   그래서 원본은 Windows 에서 상대 주소 그림을 해석하지 못한다(추정: 소스로만 확인, 실행 미확인). Go 는 CF_HTML 을 직접 읽어 `SourceURL` 을 얻는다 ― 동작이 원본보다 넓어지므로
   상대 주소 해석 결과도 `clipRemoteRef` 의 막는 주소 판정을 지나게 한다.
8. **받기 code 와 파일 받기 code 의 뜻이 다름**: 3.2절. 클립보드 받기 함수를 따로 둔다.
9. **받은 그림이 허용 밖 형식**(작업 107): 내용 앞머리로 형식을 고르고 그 디코더만 부른다. 확장자는 보지 않는다. 실패하면 안내 창, 클립보드는 비운 그대로, 받은 파일은 지운다.
10. **받은 HTML 의 `data:image/<형식>`**: 원본 `HtmlToClip` 이 HTML 안 data URI 그림을 디코드해 image 형식으로도 클립보드에 넣는다. 107 은 이 경로를 "따로 확인" 으로 남겼다.
    Wails 판은 data URI 의 바이트도 같은 허용 목록 함수로 판정한다(앞머리가 허용 형식이 아니면 넣지 않는다).
11. **로그아웃 뒤 받기 완료**: 처리한다(CLIP-035). 화면 갱신만 건너뛴다.
12. **붙여넣기 흉내 중 사용자가 키를 누르고 있음**: 수정키·글자 키를 뗄 때까지 최대 5초 기다리고, 그래도 눌려 있으면 그대로 보낸다. 보낸 뒤 10ms 에 Ctrl 이 눌린 상태로 남으면 Ctrl up 을 한 번 더 보낸다.
13. **단축키 자동 반복**: 원본은 `static s_running`·`s_pasting` 으로 막는다. Wails 는 `MOD_NOREPEAT` 로 등록하고(`global_shortcut_windows.go:47-49`), 진행 중 플래그(`atomic.Bool`)도 둔다.
14. **같은 이름 문제**: 서버에는 누가 올려도 `__CLIP__.txt` 같은 이름이다. 클립보드 판별은 이름에 `__CLIP__` 이 있는가 하나뿐이다 ― Go 에서 `isClipName(name string) bool` 하나로 모은다
    (원본은 8곳에 흩어져 있다: `ServerFileModel.cpp:169`, `ApprovalModel.cpp:221`, `DownloadBlock.cpp:15`, `ViewController.cpp:1001`·`2526`, `ViewControllerUiSlot.cpp:146`, `ViewControllerAuto.cpp:209`, `FileTransfer.cpp:1115`).
15. **업로드 목록 잠금이 잡혀 있을 때 받기**: 원본은 조용히 버린다(CLIP-024). Wails 판은 안내한다.
16. **클립보드 받기 범위 검사**: 원본 `serverIndex <= m_commList.size()` 는 하나 넘친다(`ViewController.cpp:1439`). Go 는 세션 하나라 해당 없음.

Link Redirect
17. **포트 29000 을 다른 프로그램이 먼저 잡음**: 앱은 계속 뜨고 확장 요청은 그쪽으로 간다(보안 C-040). Go 는 Windows 에서 `SO_EXCLUSIVEADDRUSE` 로 열어 다른 프로세스가 같은 포트에 겹쳐 붙지 못하게 한다(7.6절).
18. **`redirectUrl=` 없는 본문**: 원본은 `indexOf` -1 이면 본문 12번째 글자부터를 URL 로 쓴다(보안 C-021). Go 는 없으면 200 으로 끝낸다.
19. **제목에 `redirectUrl=` 이 든 페이지**: 확장이 제목을 앞에 붙이므로 `lastIndexOf` 로 마지막 것을 쓴다(지금 코드, 보안 D-007 조치). 그래도 URL 자체에 `redirectUrl=` 이 들면 잘린다 ―
    Go 는 `" redirectUrl="`(앞 공백 포함)의 마지막 위치로 자르고, 확장을 고칠 수 있으면 JSON 본문으로 바꾼다(8절).
20. **부분 문자열 판정**: 원본은 필터 `abc.com` 이 `xabc.com.evil.net` 에도 걸린다(보안 D-006). Go 는 white 를 도메인 단위로 판정하고 black 만 부분 문자열로 둔다(8.1절).
21. **첫 로그인 전 요청**: 원본은 빈 서버 목록을 첨자로 읽어 죽을 수 있다(보안 C-010, 실행 미확인). Go 는 세션이 없으면 200.
22. **확장이 하위 프레임 완료마다 보냄**: 같은 주소가 여러 번 온다. Go 처리기는 같은 URL 을 2초 안에 다시 받으면 보내지 않는다(8절).
23. **받은 URL 의 스킴**: 받는 쪽도 http/https 만 연다(보안 C-001 조치가 코드에 있다). Go 는 열기 전에 사용자에게 묻는다(8.2절).
24. **9001·9004 userid 대소문자**: 원본은 가린다. NBMT 는 ID 를 소문자로 바꿔 보낸다(`docs/NBMT_알림_사용자ID_절단.md`). Go 는 `strings.EqualFold` 로 비교한다(8절).

IPC·단일 인스턴스
25. **탐색기 다중 선택 15개 제한**: 탐색기 기본값. 레지스트리 `MultipleSelectModel` 로 늘릴 수 있다(`main.cpp:528-540` 주석). 앱 문제 아님.
26. **앱이 막 뜨는 중에 여러 파일**: 원본은 QML 을 다 읽은 뒤 파이프를 열어 그 사이 500ms 안에 못 붙은 요청이 사라질 수 있다(확인 필요 3). Wails `SingleInstance` 는 뮤텍스 획득 직후
    숨은 창을 만들고 두 번째 프로세스는 `FindWindow` 로 찾아 WM_COPYDATA 를 보낸다. 창을 만들기 전 아주 짧은 틈은 남는다(추정). 두 번째 프로세스가 창을 못 찾으면 200ms 간격 10번 다시 찾게
    감싸는 것을 검토한다(Wails 코드를 고치지 않고는 어렵다 → 7.7절 대안).
27. **트레이로 숨긴 앱**: 원본은 보이는 창만 올린다(확인 필요 4). Wails 판은 `window.Show()` + `Focus()` 로 숨긴 창도 꺼낸다.
28. **IPC 다운로드가 연달아 옴**: 마지막 경로만 쓴다(원본과 같다).
29. **로그아웃 중 IPC 업로드**: 원본은 버린다. Wails 판은 로그인 화면이면 보관했다가 로그인 완료 때 실행하고, 그 사실을 로그인 화면에 `로그인하면 파일 N 개를 올립니다` 로 보인다(8절).
30. **경로에 `noupdate`**: 원본은 경로 문자열에 `noupdate` 가 있어도 모드를 바꾼다. Go 는 `--mode` 값만 본다.

도메인 정책
31. **hosts 쓰기 권한**: `C:\Windows\System32\drivers\etc\hosts` 는 관리자만 쓸 수 있다. 추정: 설치본이 관리자 권한 실행(매니페스트 `requireAdministrator`)이 아니면 원본도 실패해 `도메인 등록 - Hosts 파일 열기에 실패했습니다.` 가 뜬다.
    Wails 판에서 앱 전체를 관리자로 띄우면 탐색기 끌어 놓기(UIPI)가 막힌다 ― 7.9절 대안.
32. **백신이 hosts 변경을 막음**: Microsoft Defender 는 hosts 변경을 위협으로 볼 수 있다(추정). 실패를 안내하고 로그에 OS 오류 문구를 남긴다.
33. **비정상 종료로 구역이 남음**: 다음 로그인 때 먼저 지우고 쓰므로 정상화된다. 앱이 다시 뜨지 않으면 남는다(원본과 같다).
34. **끝 표식이 없는 옛 구역**: 주의 줄과 정책 줄 모양(`#` 로 시작하지 않고 `\t\t#` 포함)까지만 지운다(`HostsBlock.h`). 그대로 옮긴다.
35. **EUC-KR 문자열**: 3.8절. Go protobuf 가 응답 전체를 거부할 수 있다.

---

## 7. 로컬 처리 ― 알고리즘과 Go 패키지

새 Go 파일 제안(패키지는 지금처럼 `main` 하나로 시작하고 커지면 나눈다):
`clip_windows.go`(Win32 클립보드), `clip_other.go`(macOS·Linux), `cliphtml.go`(참조 판정·묶음), `clipfmh.go`(앞머리), `hotkey.go`, `inject_windows.go`(SendInput),
`linkredirect.go`, `ipc.go`, `hosts.go`. 순수 함수 파일마다 `_test.go` 하나.

### 7.1 Windows 클립보드 읽기(업로드)

Wails v3 `Clipboard` 는 글만 다룬다(`pkg/application/clipboard.go` 의 `SetText`·`Text` 둘뿐). `golang.design/x/clipboard` 도 글·PNG 만 다루고 HTML 이 없다. 그래서 Win32 를 직접 부른다.

- DLL: `golang.org/x/sys/windows` 의 `NewLazySystemDLL("user32.dll")`·`kernel32.dll`.
  함수: `OpenClipboard`, `CloseClipboard`, `IsClipboardFormatAvailable`, `GetClipboardData`, `EmptyClipboard`, `SetClipboardData`, `RegisterClipboardFormatW`, `GetClipboardSequenceNumber`,
  `GlobalAlloc`(GMEM_MOVEABLE 0x0002), `GlobalLock`, `GlobalUnlock`, `GlobalSize`.
- 클립보드 작업은 `runtime.LockOSThread()` 한 goroutine 안에서 열고 닫는다(열기와 닫기가 같은 스레드여야 한다). `OpenClipboard(0)` 로 연다.
- 판정 순서(원본과 같다): 그림 → HTML → 글.
  1. 그림: `RegisterClipboardFormatW("PNG")` 가 있으면 그 바이트(투명도 보존), 없으면 `CF_DIBV5`(17) 또는 `CF_DIB`(8).
     DIB → BMP 파일: 14바이트 `BITMAPFILEHEADER`(`"BM"`, 파일 크기, 0, 0, 픽셀 시작 위치)를 앞에 붙인다. 픽셀 시작 위치 = 14 + `biSize` + 색표 크기
     (`biBitCount ≤ 8` 이면 `biClrUsed`(0 이면 2^bitcount)×4, `biCompression == BI_BITFIELDS(3)` 이고 `biSize == 40` 이면 +12). `encoding/binary` 리틀엔디언.
  2. HTML: `RegisterClipboardFormatW("HTML Format")`. 바이트는 UTF-8 이고 머리가 있다:
     `Version:0.9\r\nStartHTML:0000000105\r\nEndHTML:...\r\nStartFragment:...\r\nEndFragment:...\r\nSourceURL:https://...\r\n`.
     오프셋은 바이트 단위다. `StartHTML`~`EndHTML` 을 잘라 HTML 로, `SourceURL` 을 도메인으로 쓴다. 오프셋이 -1 이거나 범위 밖이면 `StartFragment`~`EndFragment` 로, 그것도 없으면 머리 뒤 전부.
  3. 글: `CF_UNICODETEXT`(13) → `windows.UTF16PtrToString` → UTF-8 로 `__CLIP__.txt` 에 쓴다. 원본도 UTF-8 이다.
- macOS·Linux: 추정: macOS 는 `NSPasteboard` 를 cgo 로, Linux 는 `xclip`/`wl-paste` 명령이나 X11 셀렉션이 필요하다. P1 은 Windows 이고 두 OS 는 글만 먼저(Wails `Clipboard.Text`), 그림·HTML 은 P2.

8절 PNG 안을 택하면 1번의 DIB 를 `image` 로 읽어(`golang.org/x/image/bmp`) `image/png` 로 다시 쓴다.

### 7.2 Windows 클립보드 쓰기(받기)

- 글: `EmptyClipboard` → `CF_UNICODETEXT`(UTF-16 + NUL, `GlobalAlloc` 에 복사) → `SetClipboardData`. 성공하면 메모리 소유권이 OS 로 넘어가므로 해제하지 않는다.
- 그림: 허용 목록으로 디코드한 `image.Image` 를 32bpp `CF_DIB`(BITMAPINFOHEADER 40바이트, `biHeight` 양수 = 아래에서 위로, BGRA)로 만들어 넣고, 같은 그림을 `PNG` 등록 형식으로도 넣는다
  (Office·브라우저는 PNG 를 먼저 읽어 투명도를 살린다). 원본 Qt 도 `setImage` 가 DIB·PNG 등을 함께 올린다(추정: Qt 내부 동작).
- 서식: CF_HTML 머리를 만들어 `HTML Format` 으로 넣고, 같은 내용의 평문(`fmhPlainText` 와 같은 규칙: `golang.org/x/net/html` 로 글만 모으고 U+FFFC 제거)을 `CF_UNICODETEXT` 로 함께 넣는다.
  머리 오프셋은 머리 길이가 고정되게 10자리 0 채움으로 쓴 뒤 계산한다. `<!--StartFragment-->`·`<!--EndFragment-->` 표식을 `<body>` 안에 넣는다.
- 받은 HTML 의 그림: 원본 Windows 는 로컬 파일 경로(`file:///C:/…/clip/1xxxx.png`)를 그대로 두고, macOS·Linux 는 base64 data URI 로 바꾼다. 파일 경로 방식은 받은 그림을 지울 수 없게 만든다(CLIP-032).
  Wails 판은 **모든 OS 에서 data URI 로 바꾸고 받은 파일을 곧바로 지운다**. 추정: Word·한글·Outlook 이 CF_HTML 안 data URI 그림을 붙여 넣는지는 프로그램마다 다르다 ―
  실측(9절) 뒤 안 되는 프로그램이 있으면 Windows 만 원본처럼 파일 경로로 두고 로그아웃·종료 때 지운다.

### 7.3 서식(HTML) 보내기 ― `ClipRefs.h` 이식

`ClipRefs.h`(591줄)는 Qt 의존이 거의 없는 순수 함수라 Go 로 한 줄씩 옮기고 원본 시험(`tst_ClipFmh`·ClipRefs 시험)의 표를 그대로 Go 시험표로 옮긴다. HTML 을 DOM 으로 다시 쓰지 않는다 ―
원본도 태그 안 속성만 훑어 나머지 바이트를 보존한다. `golang.org/x/net/html` 의 `Tokenizer` 는 다시 쓸 때 원문을 바꾸므로 판정용으로만 쓰고, 고치기는 원문 바이트 위치로 한다.

| 원본 함수 | 하는 일 | Go |
|---|---|---|
| `clipDecodeEntities` | 숫자 참조와 이름 참조(amp lt gt quot apos colon sol bsol period lpar rpar Tab NewLine nbsp)를 푼다 | `html.UnescapeString` 은 이름 표가 훨씬 넓다 ― 판정이 달라지므로 원본 표만 쓰는 작은 함수로 옮긴다 |
| `clipIsImageSuffix` | jpg jpeg jpe jfif png gif bmp dib webp tif tiff | `strings.ToLower(filepath.Ext)` + map |
| `clipLooksLikeImage` | 12바이트 이상: `\x89PNG\r\n\x1a\n`, `FF D8 FF`, `GIF87a`/`GIF89a`, `BM`(14바이트 이상), `RIFF....WEBP`, `II*\0`/`MM\0*` | `bytes.HasPrefix`. **107 의 허용 목록과 같은 함수**(제약: 같은 판정을 두 곳에 두지 않는다) |
| `clipAddressBlocked` | 127/8, 0/8, 169.254/16, ::1, ::, fe80::/10. v4-mapped·v4 호환은 v4 로. 사설 대역은 허용 | `net/netip`: `Addr.Unmap()`, `IsLoopback`, `IsUnspecified`, `IsLinkLocalUnicast`, 0/8 은 직접 |
| `clipRemoteRef` | http/https 이고 호스트 있음. localhost·`*.localhost`·막는 리터럴은 막음 | `net/url.Parse` |
| `clipIsDataImage` | `^data:image/(png|jpeg|jpg|gif|bmp|webp|tiff)[;,]`(대소문자 무시) | `regexp`(한 번 컴파일) |
| `clipClassifyRef` | Drop / DataImage / Local / Remote / Blocked. UNC·드라이브 경로·다른 스킴은 Drop, 스킴 없으면 클립 폴더 안만 Local, SourceURL 이 http(s) 면 상대 주소 해석 | `url.URL.ResolveReference` |
| `scanTags` | 브라우저 토큰화 흉내. 주석 안도 훑는다. rawText: script/style/textarea/title/xmp | 손으로 쓴 상태 기계(원본 그대로) |
| `clipPrepareSendHtml` | srcset·script src 제거, style 속성·`<style>` 의 url() 비움(data:image 제외), src 판정해 Drop 이면 속성 제거, 같은 키는 한 번, 50개 넘으면 OverLimit, 이름 `<번호><초><확장자>`, `src="MOSFM_CLIP_FOLDER<이름>"` | 같은 알고리즘. 결과 로그 원문 `클립보드 HTML 보내기 >>> 참조 제외 %1건(그림 아님 %2, 막는 주소 %3, 허용 안 된 스킴 %4, 개수 초과 %5, 묶지 않는 속성 %6, 받지 못함 %7)` |

원격 그림 받기(`GetImage`, CLIP-008)
- `http.Client{Timeout: 5s}`, `Transport.DialContext` 에서 **이름 풀이 결과 주소마다** `clipAddressBlocked` 를 본다(원본 `HostResolvesAllowed`). 풀린 주소로 직접 연결해 DNS 바꿔치기(rebinding) 틈을 없앤다.
- `CheckRedirect` 에서 대상마다 `clipRemoteRef` 를 다시 판정(원본 `UserVerifiedRedirectPolicy`), 리디렉션 최대 5번.
- 본문은 `io.LimitReader(fm_clip_size 또는 20MB)` 로 읽고 앞 16바이트가 그림이어야 싣는다.
- 동시 처리: `errgroup` 없이 `sync.WaitGroup` + 세마포어 채널(동시 6). 표준 라이브러리만 쓴다.
- 로컬 그림: 확장자가 그림이고 앞 16바이트가 그림이면 복사(`LocalImageOk`).

### 7.4 묶음(.fmh) 형식 ― `ClipHtml.h`·`ClipFmh.h` 이식

```
오프셋 0       int32  nCnt                 항목 수(1~51)
오프셋 4       int32  nSize[51]            항목별 바이트 수
오프셋 208     byte   szName[51][256]      항목 이름(NUL 로 끝남, UTF-8)
오프셋 13264   항목 바이트를 0번부터 차례로(크기 0 인 항목은 없음)
```

- 상수: `IMG_MAX_CNT` 50, `MAX_NAME` 256, 0번 이름 `dst.htm`, 작업 파일 이름 `src.htm`·`dst.htm`·`__CLIP__.fmh`.
- 엔디언: 원본은 구조체를 메모리 그대로 쓴다. 추정: x86·ARM 리틀엔디언, `int` 4바이트, 패딩 없음(4·4×51·256×51 모두 4의 배수). Go 는 `binary.LittleEndian` 으로 명시해 읽고 쓴다.
  **Qt 판(Windows x64)이 만든 묶음을 Wails 판이 풀고, 그 반대도 되는지** 실측해야 한다(9절).
- 받는 쪽 앞머리 검사(`fmhHeaderProblem`, 모두 통과해야 파일을 만든다): 앞머리보다 짧음 / nCnt 1~51 밖 / nSize[0] ≤ 0 이거나 0번 이름 ≠ `dst.htm` / 크기 있는 항목의 이름이 빔 /
  1번부터 이름이 `__CLIP__.fmh`·`src.htm`·`dst.htm`(대소문자 무시) / 음수 크기 / 크기 합 > 파일 크기 − 13264. 사유 문구 원문(로그용): `헤더보다 짧다(%1 / %2 바이트)`, `항목 수 %1(1~%2)`,
  `0번 항목이 %1 이 아니다`, `%1번 항목 이름에 파일명이 없다`, `%1번 항목 이름 %2 는 작업 파일이다`, `항목 크기 합 %1 > 남은 %2 바이트`. 거부 로그 머리 `클립보드 수신 >>> 묶음 거부:`.
- 항목 이름(`fmhEntryName`): 256 칸에 NUL 이 없으면 거부 → UTF-8 로 읽고 `\` 를 `/` 로 → 마지막 조각만 → trim → 끝의 `.`·공백 제거 → `.`·`..`·`:` 포함·장치 이름(`^(con|prn|aux|nul|com\d|lpt\d)(\..*)?$`, 대소문자 무시) 거부.
  Go 에서 이름에 잘못된 UTF-8 이 있으면 `strings.ToValidUTF8` 로 바꾸지 말고 거부한다(경로가 달라지는 것을 막는다, 원본보다 엄격 ― 추정: 원본은 Qt 가 U+FFFD 로 바꾼다).
- 경로(`fmhEntryPath`): `filepath.Clean(root + "/" + name)` 이 `root + 분리자` 로 시작해야 한다. Go 1.24 이상이면 `os.Root`(`os.OpenRoot(clipDir)` → `root.Create(name)`)로 폴더 밖 쓰기를 OS 수준에서 막는다 ― 이 모듈은 Go 1.26.
- 받는 쪽 풀기(`ReturnFiles`): 0번 HTML 의 `MOSFM_CLIP_FOLDER` 를 클립 폴더 경로로, `rewriteFmhHtmlImageSrcs` 로 다른 OS 의 절대 경로·`file://` 를 이 PC 경로로, `clipCleanReceivedHtml` 로 정리
  (script 요소·`base`·`meta refresh`·on* 속성 제거, a/area href 는 `javascript:`·`vbscript:`·그림 아닌 `data:` 만 제거, srcset·srcdoc 제거, 참조 속성 src background poster data href xlink:href o:href lowsrc dynsrc 는
  data:image 이거나 묶음 항목을 가리킬 때만 남김, CSS url()·@import 정리). 지운 것이 있으면 로그 `클립보드 HTML 받기 >>> 바깥 참조 %1건, 스크립트 %2건 지움`.
  그림 항목은 이름이 그림 확장자가 아니고 내용에 `html`·`script`·`body`·`src=`·`head` 가 있으면 버린다(`IsNotImage`).
- 보내는 쪽 묶기(`CombineFiles`): 0번 `dst.htm`, 1번부터 그림. 결과 크기를 돌려주고 0 이면 실패(Wails 판은 멈춘다).

### 7.5 단축키와 키 흉내

- 등록: Wails v3 `app.GlobalShortcut.Register("Alt+C", func(){...})`(추정: `App` 의 필드 이름은 `GlobalShortcut`, 타입 `*GlobalShortcutManager` ― `global_shortcut_manager.go:100` `Register(accelerator string, callback func()) error`).
  Windows 구현은 `RegisterHotKey` + `MOD_NOREPEAT` 이고 숨은 창의 스레드에서 받는다. 실패하면 오류를 돌려준다(`ERROR_HOTKEY_ALREADY_REGISTERED`). macOS 는 Carbon, Linux 는 X11 또는 포털.
  다시 등록: `Unregister(옛 값)` → `Register(새 값)`. 로그인 완료 때마다, 설정을 바꿀 때마다.
- 수정키 매핑: `ALT` → `Alt`, `SHIFT` → `Shift`, `ALT+SHIFT` → `Alt+Shift`, `CTRL+SHIFT` → `Ctrl+Shift`. 글자는 `A`~`Z`·`0`~`9`.
- 키 뗄 때까지 기다리기: `GetAsyncKeyState(VK_SHIFT 0x10 / VK_CONTROL 0x11 / VK_MENU 0x12 / 글자 VK)` 의 최상위 비트를 30ms 간격으로 최대 5초. goroutine 에서 하므로 화면을 막지 않는다(원본은 `processEvents` 로 돌았다).
- 흉내(`SendInput`): `INPUT{type: INPUT_KEYBOARD(1), ki: KEYBDINPUT{wVk, wScan, dwFlags, time, dwExtraInfo}}`. amd64 에서 `INPUT` 은 40바이트(union 정렬 8) ― `unsafe.Sizeof` 로 넘긴다.
  순서: Ctrl down(VK_CONTROL), C 또는 V down, 같은 키 up(`KEYEVENTF_KEYUP` 2), Ctrl up. 50ms 앞당겨 기다린 뒤 보내고, 10ms 뒤 `GetAsyncKeyState(VK_CONTROL)` 가 눌림이면 Ctrl up 을 한 번 더.
- 복사 뒤 클립보드 바뀜 기다리기: 흉내 직전 `GetClipboardSequenceNumber()` 를 적어 두고 20ms 간격으로 최대 500ms 동안 값이 바뀌기를 본다. 바뀌면 곧바로, 아니면 500ms 뒤 업로드
  (원본은 `QClipboard::changed` 또는 500ms 타이머). 창이 필요 없는 방법이다.
- macOS: `CGEventCreateKeyboardEvent` + `CGEventPost`(keycode 8 = C, 9 = V, 플래그 Cmd), 150ms 지연. cgo 필요, 손쉬운 사용 권한 필요 여부 미확인(명세 확인 필요 8). P2.
- Linux: 흉내 없음(원본과 같다).
- SendInput 은 UIPI 때문에 관리자 권한 창에는 들어가지 않는다(Windows 동작). 원본과 같다.

### 7.6 Link Redirect 서버

```go
// 두 소켓, 루프백만
for _, addr := range []string{"127.0.0.1:29000", "[::1]:29000"} {
    ln, err := lc.Listen(ctx, "tcp", addr) // lc.Control 에서 SO_EXCLUSIVEADDRUSE(-5) 설정, Windows 만
    ...
    go srv.Serve(ln)
}
```

- `net/http` `ServeMux`: `/` 만 정확히(`"/{$}"` 패턴, Go 1.22+) 처리하고 나머지는 404 + 경고 로그. 본문은 `http.MaxBytesReader(64KB)`.
  응답: 참 → `200`, 본문 `LinkRedirect`. 거짓 → `205`. `Server.ReadHeaderTimeout` 5초.
- 처리기(원본 `linkRedirectProcess` 순서 그대로):
  1. 세션 없음(로그인 전·로그아웃 뒤) → 200. (원본 LINK-015 미구현을 메운다.)
  2. 세션 서버 `Loc != "1"` 또는 `fm_hyperlink_mode` 가 없거나 `"0"` → 200.
  3. `method := profile["url_check_method"]`.
  4. 본문에서 `redirectUrl=` 의 마지막 위치 뒤를 `strings.ToLower(strings.TrimSpace(...))` → `webUrl`. 없으면 200.
  5. 호스트 뽑기(`getHostName`): 원본은 method 가 정확히 `"black"`(대소문자 구분)일 때와 아닐 때 다르게 자른다.
     - black: 스킴 뒤 첫 `:` 앞까지, 맨 앞 `www.` 제거, 스킴 뒤 첫 `/` 앞까지 ― 결과에 스킴이 남는다(`https://abc.com`).
     - 그 밖: `http://`·`https://`·`www.` 를 모두 지우고 첫 `:`·`/` 앞까지(`abc.com`).
     이 차이 때문에 black 의 부분 문자열 판정이 스킴까지 포함한 문자열을 대상으로 돈다. Go 는 black 은 원본 글자 자르기를 그대로 쓰고, 그 밖은 `url.Parse` 의 `Hostname()` 을 쓴다.
     `www.` 는 지우지 않는다(`www.abc.com` 은 항목 `abc.com` 의 하위 도메인이다). 파서를 못 지나는 주소는 호스트가 없다. 까닭은 8.1절.
  6. 호스트가 비면 200.
  7. 대상 판정(`isLinkRedirectTarget`): 원본은 필터 항목마다(빈 항목 제외) `host.indexOf(item) >= 0` 이면 걸림이고, 대소문자 무시 `black` 이면 `!걸림` 이 대상, 그 밖이면 `걸림` 이 대상이다.
     항목은 소문자로 바꾸지 않아 대문자 항목은 호스트에 걸리지 않는다(`filemoverpolicy.cpp:234-258`).
     Go 는 항목을 소문자로 바꾸고 앞뒤 공백을 지운다. black 은 원본과 같은 부분 문자열이고, 그 밖은 `host == item || strings.HasSuffix(host, "."+item)` 이다.
     원본은 행의 `flag` 를 보지 않지만 Go 는 미적용 행을 건너뛰고 차단 행은 어느 모드든 뺀다(8.1절).
  8. 대상 아님 → 200.
  9. `url.Parse(webUrl)` 이 http/https 이고 호스트가 있어야 한다. 아니면 경고 `링크연동 주소가 http/https 가 아니라 보내지 않았다:` 뒤 200.
  10. `strings.Contains(webUrl, host)` 이면 [3204] `SendURL` 을 goroutine 으로 보내고 로그 `인터넷망으로 URL전송[%1]`, 205. 아니면 200.
- 로그 원문: 시작 성공 `LinkRedirect httpServer Listening port[29000] IPv4[127.0.0.1] IPv6[::1]`, 실패 `LinkRedirect httpServer listen failed port[29000] - 다른 프로그램이 port를 사용중인지 확인하세요.`,
  한쪽 `- 한쪽만 열렸습니다.`, 앱 쪽 `링크연동을 사용할 수 없습니다 - 나머지 기능은 정상 동작합니다.`, 요청 `LinkRedirect httpServer 요청 수신:`, 404 `LinkRedirect httpServer 처리되지 않은 요청(404):`.
- `SO_EXCLUSIVEADDRUSE`: `golang.org/x/sys/windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_EXCLUSIVEADDRUSE, 1)`(상수 값 `^4` = -5). 추정: x/sys/windows 에 상수가 있다 ― 없으면 직접 정의.
  Go 의 Windows `net.Listen` 은 `SO_REUSEADDR` 를 걸지 않는다(추정: `internal/poll`·`net/sockopt_windows.go` 확인 필요).

### 7.7 단일 인스턴스와 두 번째 실행

선택지 A ― Wails v3 `SingleInstance`(권장)
- `application.Options{SingleInstance: &application.SingleInstanceOptions{UniqueID: "com.sqisoft.snbapp", OnSecondInstanceLaunch: func(d application.SecondInstanceData){...}, ExitCode: 0}}`.
- Windows 구현(`single_instance_windows.go`): 뮤텍스 `wails-app-<UniqueID>-sim`, 숨은 창 클래스 `...-sic`, 두 번째 프로세스가 `FindWindowW` 로 찾아 `WM_COPYDATA` 로 JSON(Args·WorkingDir·AdditionalData) 전달.
  `EncryptionKey` 를 주면 내용을 암호화한다(같은 사용자의 다른 프로세스가 업로드를 주입하는 것을 줄인다 ― 키는 실행 파일에 박히므로 완전한 방어는 아니다).
- `OnSecondInstanceLaunch` 에서 `d.Args` 를 원본과 같은 규칙(`--mode`, 위치 인자)으로 읽어 upload 면 대기 목록에 더하고 400ms 타이머, download 면 경로 덮어쓰기, 그 밖이면 창을 꺼내 앞으로.
  이 콜백은 앱 이벤트 스레드가 아닐 수 있으므로(추정) 상태는 `sync.Mutex` 로 지킨다.
- 장점: 경로 여럿을 한 메시지로 받는다(원본은 첫 위치 인자 하나). 첫 프로세스가 창을 만들기 전에 붙는 문제가 줄어든다.
- 단점: Qt 판과 이름이 달라 **Qt 판과 Wails 판이 동시에 뜰 수 있다**. 설치본이 Qt 판을 지우고 설치하면 문제없다. 포트 29000 은 먼저 뜬 쪽이 잡는다.
- `noupdate`(LINK-029·030): 업데이트 도우미가 앱을 다시 띄울 때 앞 앱이 아직 끝나지 않았을 수 있어 원본은 최대 5초 기다린다. Wails `SingleInstance` 는 기다리지 않고 두 번째로 판정해 끝난다 ―
  `--mode=noupdate` 면 `application.New` 전에 뮤텍스를 `OpenMutex` 로 100ms 간격 50번 확인해 사라질 때까지 기다린 뒤 진행한다.

선택지 B ― 원본 호환 명명 파이프 `\\.\pipe\FileMoverActivation`
- `github.com/Microsoft/go-winio` 의 `ListenPipe`/`DialPipe`. Qt `QLocalSocket` 은 Windows 에서 명명 파이프다. 메시지는 compact JSON 한 줄 `{"mode":"upload","path":"..."}` 또는 `Activate`.
- 옛 설치본의 오른쪽 메뉴·E2E 하네스(`Invoke-SnbUploadByIpc`)를 그대로 쓸 수 있다. 의존성이 하나 는다.
- 결론: A 를 쓰고, E2E 가 파이프에 직접 쓰는 곳이 생기면 그때 B 를 더한다(지금 Qt 판 하네스는 명령줄로 프로세스를 띄우므로 A 로도 된다).

### 7.8 탐색기 오른쪽 메뉴(설치본)

원본 `installer/installer_common.nsh` 257-279 를 Wails 설치본(NSIS, `build/windows/nsis/`)에 옮긴다. HKCU 기준(관리자 불필요):

| 키 | 값 |
|---|---|
| `HKCU\Software\Classes\*\shell\SNBUpload` | 기본값 `snB App 파일 업로드`, `Icon` = `"<exe>",0` |
| `...\*\shell\SNBUpload\command` | `"<exe>" "--mode=upload" "%1"` |
| `HKCU\Software\Classes\Directory\shell\SNBDownload` | `snB App 파일 다운로드` |
| `...\Directory\shell\SNBDownload\command` | `"<exe>" "--mode=download" "%1"` |
| `HKCU\Software\Classes\Directory\Background\shell\SNBDownload` | `snB App 파일 다운로드` |
| `...\Directory\Background\shell\SNBDownload\command` | `"<exe>" "--mode=download" "%V"` |

- 옛 HKCR 키(`*\shell\SNBUpload` 등)를 지운다(원본도 한다).
- 원본은 `HKCU\Software\Classes\CLSID\{86ca1aa0-34aa-4e8b-a509-50c905bae2a2}\InprocServer32` 기본값을 빈 값으로 써서 **Windows 11 의 새 오른쪽 메뉴를 사용자 전체에서 옛 메뉴로 되돌린다**.
  이것은 다른 프로그램의 메뉴 모양까지 바꾸는 사용자 설정 변경이다. Wails 판은 이 값을 쓰지 않고 "추가 옵션 표시"(Shift+F10) 아래에 두는 것을 기본으로 하고, 새 메뉴 최상단에 두려면
  `IExplorerCommand` + 희소 패키지(sparse package) 등록이 필요하다(난이도 상, P2). 고객 요구가 있으면 원본처럼 되돌리는 것을 설치 옵션으로 둔다.
- 실행 파일 이름: 원본은 `FileMoverApp.exe`, Wails 판은 `snbApp.exe`. 설치본이 메뉴 명령을 새 경로로 다시 쓰면 된다. 원본 HKLM `Run` `snB App` 시작 프로그램 등록도 새 경로로.
- 앱 코드에서 레지스트리를 쓸 일은 없다(설치본 몫). 앱이 확인만 하려면 `golang.org/x/sys/windows/registry.OpenKey(registry.CURRENT_USER, ...)`.

### 7.9 hosts 파일(도메인 정책)

- 경로: `windows.GetSystemDirectory()` + `\drivers\etc\hosts`(원본은 `GetWindowsDirectory + \system32\...`), 그 밖 `/etc/hosts`.
- 순수 함수(`HostsBlock.h` 이식, `[]byte` 로만 다룬다):
  - 표식(UTF-8 바이트): 시작 `# 본영역은 ssBridge의 도메인 정책이 등록됩니다.`, 주의 `# 삭제될 수 있는 영역이므로 다른 내용을 넣지 마세요.`, 끝 `# ssBridge 도메인 정책 등록 영역 끝`.
  - `removeBlocks(b []byte) []byte`: 시작~끝 표식 사이를 지운다. 끝이 없으면 주의 줄과 정책 줄 모양까지만. 구역 앞 빈 줄 하나도 지운다.
  - `appendBlock(b []byte, rows []row) []byte`: 줄 끝은 원본의 첫 줄바꿈을 따르고, 줄바꿈이 없으면 Windows 는 CRLF. 구역 앞에 빈 줄. 행 `IP\t이름\t\t#설명`.
  - 행 검사: IP `^[0-9A-Fa-f:.]+$` + `netip.ParseAddr` 성공, 이름 `^[A-Za-z0-9.-]+$`, 설명은 `unicode.IsControl` 제거.
  - 원본 `tst_DomainPolicy` 의 시험 입력을 그대로 Go 시험표로 옮긴다.
- 쓰기: 같은 폴더에 임시 파일 → `windows.MoveFileEx(tmp, hosts, MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH)`. 실패하면 원본처럼 직접 쓰기로 대체.
  주의: 바꿔치기는 hosts 의 ACL 을 임시 파일의 것으로 바꾼다(추정: 폴더 상속 ACL 이라 실무상 같다). 확인 전까지는 원본과 같은 순서로 두되 ACL 을 실측한다.
- 권한: 관리자 권한이 필요하다(6절 31). 대안
  1. 원본처럼 앱이 직접 쓰고 실패하면 안내(동등, P1 기본값).
  2. 설치본이 hosts 쓰기만 하는 작은 서비스나 예약 작업을 관리자 권한으로 등록하고 앱이 요청만 보낸다(P2, 난이도 상).
  3. `ShellExecute "runas"` 로 쓰기 도우미를 매번 띄운다(UAC 창이 로그인마다 떠 사용자 경험이 나쁘다).
- EUC-KR: `DomainInfo` 의 `string` 칸이 UTF-8 이 아니면 Go protobuf 가 응답 전체를 거부한다. 로컬 proto 사본에서 그 칸을 `bytes` 로 바꾸면 전송 형식(wire type 2)은 같아 서버와 호환된다.
  바꾼 칸은 `golang.org/x/text/encoding/korean.EUCKR.NewDecoder()` 로 풀고, UTF-8 로 이미 맞으면(`utf8.Valid`) 그대로 쓴다. 이 처리는 **다른 응답의 한글 칸에도 같은 위험이 있으므로**
  01 인증·06 결재와 함께 정한다(로그인·목록은 지금 실 서버에서 되므로 그 칸들은 UTF-8 로 오는 것으로 보인다).

### 7.10 Go 패키지 후보 요약

| 쓰임 | 패키지 | 비고 |
|---|---|---|
| 클립보드·SendInput·GetAsyncKeyState·hosts 경로·MoveFileEx | `golang.org/x/sys/windows`(이미 간접 의존) | DLL 직접 호출 |
| 레지스트리(옛 설정 옮기기) | `golang.org/x/sys/windows/registry` | |
| 전역 단축키 | `github.com/wailsapp/wails/v3/pkg/application` `GlobalShortcutManager` | 이미 의존. `golang.design/x/hotkey` 불필요 |
| 단일 인스턴스·두 번째 실행 인자 | Wails v3 `SingleInstanceOptions` | 선택지 B 면 `github.com/Microsoft/go-winio` |
| Link Redirect | `net/http`, `net`(ListenConfig) | |
| 원격 그림·주소 판정 | `net/http`, `net/url`, `net/netip` | |
| 그림 판정·디코드 | `image`, `image/png`, `image/jpeg`, `image/gif`, `golang.org/x/image/bmp`, `golang.org/x/image/webp`, `golang.org/x/image/tiff` | `image.Decode` 의 자동 판별은 쓰지 않는다(등록된 형식 전부가 입구가 된다 ― 107 과 같은 이유). 앞머리로 고른 디코더만 부른다 |
| HTML 평문·토큰 판정 | `golang.org/x/net/html`(이미 간접 의존) | 고치기는 원문 바이트로 |
| 묶음 앞머리 | `encoding/binary`, `os.Root` | |
| 검사값 | `crypto/sha256`, `encoding/hex` | Wails `sha256File` 재사용 |
| EUC-KR | `golang.org/x/text/encoding/korean`(이미 간접 의존) | |
| 브라우저 열기 | Wails `app.Browser.OpenURL`(추정 이름) 또는 `windows.ShellExecute(0, "open", url, ...)` | http/https 만 넘긴다 |

---

## 8. 원본보다 낫게 할 곳

동작 호환을 깨지 않는 것부터, 결정이 필요한 것은 따로 표시했다.

성능
1. **원격 그림 동시 받기**: 원본은 그림마다 차례로 최대 5초(50장이면 최악 250초 동안 단추가 안 눌린다). Go 는 동시 6개 + 전체 시한 15초, 화면은 계속 쓸 수 있고 진행 중 표시. 넘은 그림은 "받지 못함" 으로 센다.
2. **그림을 PNG 로 보내기**(결정 필요): 비압축 BMP 대신 PNG 면 화면 캡처가 수 MB → 수백 KB 로 줄어 `fm_clip_size` 에 덜 걸리고 전송도 빠르다. Qt 판 받는 쪽은 code 2 + 서버 경로 `.png` 를 이미 처리한다(WASM 판이 PNG 를 보낸다).
   **MOSFM(이전 세대) 받는 쪽이 PNG 를 여는지 미확인** ― 확인 전에는 BMP 를 기본으로 두고 설정 키로 바꿀 수 있게 한다.
3. **단축키 대기를 goroutine 으로**: 원본은 `processEvents(ExcludeUserInput)` 로 화면 스레드에서 최대 5초 돈다. Go 는 별도 goroutine 이라 화면이 멈추지 않는다.
4. **클립보드 바뀜 감지를 시퀀스 번호로**: 복사가 빨리 끝나면 500ms 를 다 기다리지 않는다.

정확성·보안
5. **SourceURL 보존**: Windows 에서도 상대 주소 그림을 싣는다(6절 7). 상대 주소도 막는 주소 판정을 지난다.
6. **임시 파일 바로 지우기**: 업로드가 끝나면(성공·실패 모두) `__CLIP__.*`·`src.htm`·`dst.htm`·그림을 지운다. 받은 파일도 클립보드에 올린 뒤 지운다(7.2절 data URI 방식). 로그인 때 `temp/` 정리는 남긴다(비정상 종료 대비).
   원본 CLIP-018·032 의 "다음 로그인까지 업무망 내용이 디스크에 남는" 문제를 없앤다.
7. **묶음 만들기 실패 시 멈춤**: 0 바이트 묶음을 보내지 않는다.
8. **받은 그림 허용 목록(107)을 처음부터**: 형식별 디코더만, 실패하면 안내 창.
9. **Link Redirect 본문 상한·로그인 확인·같은 URL 2초 중복 억제**: 64KB 상한, 세션 없으면 200, 하위 프레임 완료로 같은 주소가 여러 번 가는 것을 줄인다.
10. **포트 독점 열기**(`SO_EXCLUSIVEADDRUSE`): 다른 프로세스가 29000 에 겹쳐 붙어 URL 을 가로채는 것을 막는다(보안 C-040).
11. **필터 대소문자 통일**: 호스트·필터 항목 모두 소문자로 비교. `url_check_method` 도 대소문자 무시로 통일(원본은 호스트 뽑기에서만 구분한다).
12. **white 도메인 일치**(결정: 적용): `abc.com` 필터가 `xabc.com.evil.net` 에 걸리지 않게 `host == item || strings.HasSuffix(host, "."+item)` 로 판정한다. black 은 부분 문자열을 유지한다(8.1절, 보안 D-006).
13. **userid 비교 대소문자 무시**: 9001·9004 에서 `strings.EqualFold`. NBMT 가 소문자로 보내는 ID 와 맞는다.
14. **9004 kind 0 의 `break` 누락**을 따라 하지 않는다. 결재 목록 갱신이 필요하면 명시적으로 부른다(09 알림과 함께 MOSFM 대조).
15. **로그에 URL 전체 대신 호스트**: 사용자 방문 기록이 로그에 평문으로 쌓이지 않게(LINK-009). 디버그 수준에서만 전체.
16. **IPC 경로 여럿·로그아웃 중 보관**: 두 번째 실행 인자 전부를 받고, 로그인 화면일 때 온 업로드는 보관해 로그인 뒤 실행(원본 LINK-039 는 조용히 버린다).
17. **두 번째 실행은 경고 창 대신 창 올리기**: 트레이로 숨긴 창도 꺼낸다. 원본 경고 창(`snB App 프로그램이 이미 실행중입니다.`)은 인증시험이 문구를 보는지 확인 후 결정 ―
    추정: 인증시험 0009 는 중복 로그인이라 무관. 필요하면 창을 올린 뒤 같은 문구를 알림으로 보인다.
18. **`noupdate` 오판 제거**: 경로에 든 `noupdate` 로 모드가 바뀌지 않게.
19. **클립보드 받기 잠금 실패 안내**: 원본은 단추가 먹지 않은 것처럼 보인다.
20. **허용 표시와 판정을 한 표로**: 사용자 정보 창 표시가 실제 판정과 같은 칸을 본다(4.1절).
21. **한 이름 판별 함수**: `isClipName` 하나로 8곳을 대신한다. CLIP-053(뒤 파일 누락)도 생기지 않는다.

디자인
22. **목록에 `클립보드(글/그림/서식)`**: 서버 이름 `__CLIP__.txt` 대신 사람이 읽는 이름(CLIP-054). E2E 는 접근성 id 의 서버 이름으로 행을 고른다.
23. **단추 상태**: 로그인 전·권한 없음이면 [클립보드 업로드]/[클립보드 다운로드] 를 비활성으로 보이고 툴팁에 까닭을 적는다(원본은 눌러야 안내가 뜨거나 아무 일도 없다).
24. **크기 초과 안내에 숫자**: `전송 가능한 파일 크기 초과` 뒤에 `(상한 1 KB, 현재 13 KB)`(CLIP-010).
25. **단축키 등록 결과 표시**: 설정 칸 아래에 `등록됨`/`다른 프로그램이 쓰는 단축키입니다`.
26. **받기 완료 알림**: 클립보드를 덮어쓴 뒤 화면 구석에 `클립보드에 글을 넣었습니다(12 자)` 같은 짧은 알림(CLIP-026 "알리기" 는 결정 기록이 없어 묻는 창은 만들지 않는다).

### 8.1 결정: white 필터를 도메인 단위로 판정한다

부분 문자열 판정에서는 허용 항목 `daum.net` 이 `daum.net.evil.example` 에도 걸려, 공격자가 만든 도메인의 주소가 인터넷망 PC 로 건너간다.
white 목록은 망을 건너도 되는 곳을 정하는 허용 목록으로 보아 2026-10-18 에 도메인 단위 판정을 정했다(`E:\git\snBv6FM\docs\security\fixes\D-plugins.md` 의 D-006). Wails 판은 이 결정을 따른다.

- white: `url.Parse` 의 호스트가 항목과 같거나 `.항목` 으로 끝날 때만 걸린다.
- black: 항목이 호스트 문자열 어디에든 들어 있으면 뺀다. 좁히면 차단할 주소가 새어 나가므로 그대로 둔다. 넓게 걸려서 틀리면 주소를 보내지 않는 쪽으로 틀린다.
- 호스트를 URL 파서로 뽑는다. 글자 자르기에 끝 일치만 더하면 `https://evil.example?.daum.net` 의 호스트가 `evil.example?.daum.net` 이 되어 통과하고, `https://daum.net@evil.example/` 은 `daum.net` 으로 읽힌다.
- 서버 `make_proxypac.c` 는 `shExpMatch(url, "*값*")` 로 URL 전체를 부분 문자열로 비교한다. 앱 판정은 이 규칙과 일부러 달라진다. 항목 하나로 PAC 와 앱의 판정이 다를 수 있다.

기존 필터가 깨지는지 SBM 등록 규칙(`redirectMng.js:232-257`, `valid.js:99`)과 시험 서버 값으로 확인한 결과는 다음과 같다. 항목은 SBM URL 필터에 등록한 값이고, "원본 white" 는 Qt 판 판정이다.

| 항목 | 원본 white | Wails 판 white | 비고 |
|---|---|---|---|
| `daum.net` | `daum.net`, `www.daum.net`, `mail.daum.net` 대상 | 같다 | 칸 제목 "도메인" 대로 등록한 값은 그대로다 |
| `10.1.2.3` | `10.1.2.3`, `10.1.2.33` 대상 | `10.1.2.3` 만 대상 | `10.1.2.33` 이 빠지는 것이 맞다 |
| `google.co` | `google.com`, `google.co.kr` 대상 | `google.co` 와 그 하위만 대상 | **깨진다**. 도메인 경계가 아닌 일부만 적은 항목은 정확한 도메인으로 다시 등록해야 한다 |
| `daum` | `daum.net` 대상 | 걸리지 않는다 | **깨진다**. 다만 SBM 은 점 없는 영문 값을 받지 않아 DB 직접 입력일 때만 생긴다 |
| `www.daum.net` | 호스트에서 `www.` 를 지워 걸리지 않는다 | `www.daum.net` 과 그 하위 대상 | 넓어진다. 관리자가 적은 뜻대로다 |
| `https://daum.net`, `daum.net/news`, `daum.net:8080` | 걸리지 않는다(SBM 은 받는다) | 걸리지 않는다 | 같다. 넓히지 않으려고 정규화하지 않는다 |

행의 적용 범위도 판정에 쓴다. SBM 링크리다이렉트 설정 화면(`redirectMng.jsp`)에서 미적용·프록시 허용·차단으로 고르는 칸은 `fm_url_filter.allow_flag` 에 저장되고, 서버 `GetUrlFilter` 는 이 값으로 거르지 않은 채 모든 행을 준다. `make_proxypac.c` 는 `allow_flag = 1` 행만 PAC 에 넣는다.
원본처럼 값을 무시하면 관리자가 미적용·차단으로 둔 주소도 white 에서 망을 건넌다.

| 적용 범위(`allow_flag`) | white | black | PAC 와의 관계 |
|---|---|---|---|
| 미적용(0) | 판정에 쓰지 않는다 | 판정에 쓰지 않는다 | 같다. 등록 때 값을 비우면 0 이다 |
| 프록시 허용(1) | 도메인이 일치하면 보낸다 | 부분 문자열이 일치하면 뺀다 | 같다(일치 방식은 위 white 설명대로 다르다) |
| 차단(2) | 부분 문자열이 일치하면 뺀다 | 부분 문자열이 일치하면 뺀다 | 다르다. `make_proxypac.c` 는 2 행을 넣지 않는다 |

- 차단 행은 허용 행보다 앞선다. `daum.net` 이 허용이고 `bad.daum.net` 이 차단이면 `bad.daum.net` 은 보내지 않는다. 행 순서와 무관하다.
- 모르는 값은 차단처럼 다룬다. 어느 모드에서도 주소를 보내지 않는 쪽이다.
- 동작이 넓어지는 경우가 하나 있다. black 에서 미적용 행의 주소는 원본에서는 빠졌지만 이제 전송 대상이다. PAC 도 그 주소를 앱으로 보내므로, 관리자가 규칙을 끈 뜻과 PAC 에 맞췄다.
- SBM 안내 문구의 "차단: pac포함(명시적 차단 0.0.0.0:0)" 은 저장소의 `make_proxypac.c` 에 구현돼 있지 않다. 앱은 이 문구의 뜻대로 보내지 않는다.
- [정책] 화면의 LinkRedirect 행은 적용 범위와 상관없이 서버가 준 행을 모두 보인다.

실례 표본은 작다. 시험 서버 .157 의 `fm_url_filter` 는 0행(`url_check_method=white`, `fm_hyperlink_mode=0`)이고, DB 초기 자료에 `sqisoft.com` 1건이 있다(`ssdb/mariadb_init/smex_int.sql:1290`). 192.168.107.121·192.168.101.233 은 접속 시간 초과로 보지 못했다.
고객사 필터 값은 본 적이 없다. 표의 "깨진다" 는 SBM 규칙에서 유도한 것이며, 그런 값이 실제로 등록돼 있는지는 모른다.

### 8.2 결정: 받는 쪽에서 사용자에게 묻고 연다

MOSFM·Qt 판·포팅 전 Wails 판 셋 다 받은 주소를 묻지 않고 연다(`E:\git\MOSFM\source\FileMover\ViewControllerNotifySlot.cpp:1284-1290`, `E:\git\snBv6FM\src\ViewControllerNotifySlot.cpp:1061-1069`). 원본 대조로는 확인 창이 없는 것이 맞다.
그래도 Wails 판은 확인 창을 넣는다.

- 보내는 쪽 판정을 넘는 경로가 있다. black 모드에서는 공격 페이지가 `location` 으로 이동해도 확장이 대신 보낸다. 앱 창구는 Origin 없는 로컬 프로그램의 POST 를 받는다. 서버 `SendURL` 은 스킴·필터를 검사하지 않는다(`svc_sendurl.go`).
  서버 알림 UDP 수신기는 인증이 없어 개발 PC 패킷이 수신기까지 닿았다(`E:\git\snBv6FM\docs\security\20_서버요청문-SR024-027-검증.md` 2-5~2-8절. 위조 전문이 앱까지 닿는지는 확인하지 못했다).
- 업무망이 인터넷에 못 나가도, 주소가 망을 건너 인터넷 PC 에서 GET 되면 URL 에 담긴 자료가 공격자 서버로 나간다. 이 경로에서 업무망 공격자가 제어할 수 없는 판정은 받는 PC 의 사람뿐이다.
- 보안 조사 C-001 의 대응에 확인 창이 있고 남은 일로 기록돼 있다(`E:\git\snBv6FM\docs\security\fixes\C-app-core.md`).

주소가 자동으로 열리는 것이 이 기능의 목적이다(레거시 스펙 016 SC-001). 매번 물으면 쓸 수 없으므로 범위를 좁혔다.

- 사이트(호스트)당 로그인 동안 한 번 묻는다. 기본 단추는 [취소]다.
- 창은 한 번에 하나만 띄운다. 창이 떠 있는 동안 온 새 호스트의 주소는 버리고 로그만 남겨, 주소를 밀어 넣어 창을 쌓는 일을 막는다.
- 답을 기다리는 사이 로그아웃하면 [열기] 를 눌러도 열지 않고 기억하지도 않는다. 로그아웃하면 허락을 잊는다.
- OS 질문 창을 쓴다. 창이 하나도 없는 앱에서도 맨 앞에 뜨고 프런트엔드 디자인을 건드리지 않는다.
  Windows 에서 [열기]·[취소] 단추는 `build/windows/wails.exe.manifest`(공용 컨트롤 6)가 실행 파일에 들어 있어야 나온다. Taskfile 빌드는 넣는다. 매니페스트 없이 `go build` 만 한 실행 파일은 예(Y)·아니요(N) 창이 되고 기본 단추가 예(Y)로 바뀌며 Esc 도 먹지 않는다. 예 = 열기, 아니요 = 취소로 뜻은 같다.

한계: 허락한 호스트로 가는 주소는 묻지 않으므로 그 호스트로의 유출은 막지 못한다. 받는 쪽에서 필터를 다시 판정하는 안은 인터넷망 쪽 필터·정책 값이 유효한지 확인하지 못해 채택하지 않았다.
Qt 판에는 이 두 결정이 들어가지 않았다. 같은 필터에서 Qt 판과 Wails 판의 전송 대상이 다를 수 있고, 받는 쪽 확인 창은 Wails 판에만 뜬다.

---

## 9. 실 서버 검증 방법

계정·서버(비밀번호는 적지 않는다 ― 자격증명은 `E:\git\snBv6FM\tools\.secrets\login.json`)
- 업무망 서버 192.168.101.157(DB 있음), 인터넷망 서버 192.168.101.158(mysql 없음 ― 계정·설정 조회는 .157 에서).
- 역할: 요청자 `test<N>_1`, 결재자 `test<N>_2`, 대리결재자 `test<N>_3`. 잠기면 `uv run tools\srv_settings.py unlock` 으로 푼다.
- 서버 권한은 프로파일 단위(`fm_profile`·`fm_profile2`). SBM 화면으로 바꾸는 것이 원칙이고, E2E 가 쓰는 값은 `tools\srv_settings.py` 로 건다. autoqa 계정(E2E-QA 프로파일)과 같은 시각에 돌리지 않는다.
- Wails 판 실행: `$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'; .\bin\snbApp.exe`(원본 개발 빌드의 `fmenv.ini`·`root_ca.pem` 사용).
- 대조 기준: 같은 서버에 Qt 판 `tools\snb.cmd run` 을 다른 망 계정으로 띄워 Wails ↔ Qt 양방향을 본다.

서버 설정(시험별)

| 시험 | 설정 |
|---|---|
| 기본 | 요청자 프로파일 클립보드 업·다운로드 허용(proto `fm_clip_down`=1, `fm_clip_up`=1), `fm_clip_size`=0, 결재 유형 0, `fm_upfile_mode`=1 |
| 크기 | `fm_clip_size`=1(KB) ― 인증시험 0039 와 같다 |
| 권한 없음 | 업무망 업로드 칸 0 |
| 사전 결재 | 클립보드 사전승인 `fm_aprv_clup`=1(SBM DB 이름, code 행 필요), `fm_upfile_mode`=0 |
| Link Redirect | `fm_hyperlink_mode`=2, `url_check_method`=white, SBM URL 필터에 시험 호스트 1건(적용 범위 프록시 허용) |
| 도메인 정책 | 업무망 `fmenv.ini [CONFIG] DOMAIN_POLICY=1`, SBM 도메인 정책 1건(시험용 이름·사설 IP). 시험 PC 에서 관리자 권한으로 실행 |

검증 항목과 통과 기준

1. **단위 시험**: `go test .` ― 다음이 모두 통과한다.
   - 그림 앞머리 판정·허용 목록(`svg_as_png.png`·`icns_as_jpg.jpg` 같은 위장 입력 거부, 잘린 PNG 실패) ― Qt `build\fuzz\data\img_abnormal` 입력을 빌려 쓴다.
   - 묶음 앞머리·이름 검사(원본 `tst_ClipFmh` 40건의 입력표).
   - `clipPrepareSendHtml`·`clipCleanReceivedHtml` 입력표(원본 ClipRefs 시험).
   - 권한 판정표 4칸(4.1절), 크기 경계(상한과 같으면 막힘).
   - Link Redirect 호스트 뽑기·대상 판정(white/black × 걸림/안 걸림 × 대소문자), 적용 범위(white/black × 미적용·프록시 허용·차단), 유사 도메인·사용자 정보·쿼리·조각에 숨긴 도메인, `redirectUrl=` 없음·두 번.
   - 받은 주소 확인(`TestGate`): 거절·허락 기억·창이 떠 있을 때 새 호스트 버림·로그아웃 뒤 다시 질문.
   - hosts `removeBlocks`·`appendBlock`(원본 `tst_DomainPolicy` 입력, CRLF·LF·줄바꿈 없음·끝 표식 없음).
2. **글 왕복**: 업무망 Wails 판에서 메모장 글 복사 → [클립보드 업로드] → 인터넷망 Qt 판에서 `클립보드 N 건` 이 1 늘고 [클립보드 다운로드] → 메모장에 같은 글. 반대 방향(Qt 업로드 → Wails 받기)도.
   서버 목록에서 그 건이 사라진다([3036]). 한글·이모지·CRLF 포함 글로.
3. **그림 왕복**: 그림판 그림 복사 → 업로드 → 상대 판에서 받아 그림판에 붙여 넣어 픽셀이 같다(인증시험 0035-1 과 같은 판정). 투명 PNG(브라우저 그림 복사)도.
4. **서식 왕복**: 브라우저에서 글+그림이 든 영역 복사 → 업로드 → 상대에서 받아 워드패드·한글(한워드)·Outlook 이 없으면 한워드에 붙여 서식·그림이 보인다.
   Wails 판이 만든 `.fmh` 를 Qt 판이 풀고, Qt 판이 만든 것을 Wails 판이 푸는지(앞머리 엔디언·패킹 호환). data URI 그림이 붙는 프로그램 목록을 적는다(7.2절 결정 근거).
5. **크기 상한**: `fm_clip_size`=1 에서 1KB 넘는 글 → `전송 가능한 파일 크기 초과`, 닫은 뒤 메모장에 원래 글이 붙는다(클립보드가 비지 않았다).
6. **권한 없음**: `클립보드 업로드가 허용되어 있지 않습니다.` / `클립보드 다운로드가 제한되어 있습니다.`.
7. **사전 결재**: 결재자 창에 `클립보드` 표시 → 결재자 `test<N>_2` 로 [결재처리] 에서 승인 → 요청자 상대망에서 받기.
8. **단축키**: 설정에서 `항상사용`, 복사 `ALT+SHIFT`+`C`. 메모장 글을 선택하고 단축키 → 업로드 성공. 상대망에서 붙여넣기 단축키 → 메모장 커서 자리에 글이 들어간다. 다른 프로그램이 같은 키를 쓰면 설정 칸에 실패가 보인다.
9. **Link Redirect**: Chrome 에 `imports\ssbridgeExtension` 을 압축 해제 확장으로 올리고(개발자 모드), 업무망 로그인 상태에서 필터 호스트 페이지를 연다 →
   인터넷망 PC(같은 사용자 계정으로 로그인한 Wails 또는 Qt 판)의 기본 브라우저에 같은 주소가 열린다. 앱 로그 `인터넷망으로 URL전송[` 과 상대 로그 `업무망에서 전달된 URL 수신:`.
   받는 쪽이 Wails 판이면 먼저 `링크 열기` 창이 뜬다. 트레이로 숨긴 상태에서도 맨 앞에 뜨고, [취소]·Esc 면 열리지 않으며, [열기] 뒤 같은 사이트의 다른 주소는 묻지 않고 열리고, 로그아웃 뒤 다시 로그인하면 또 묻는다.
   확장 탭이 안내 페이지로 바뀌는지도 본다 ― **바뀌지 않으면 no-cors 불투명 응답(status 0) 추정이 맞는 것**이고 Qt 판도 같은지 대조한다.
   `curl -s -o NUL -w "%{http_code}" -d "~~docTitle=t redirectUrl=https://<필터호스트>/" http://127.0.0.1:29000/` → 205, 필터 밖 → 200, `/x` → 404.
10. **포트 충돌**: 29000 을 먼저 다른 프로세스(`python -m http.server 29000 --bind 127.0.0.1`)로 잡고 Wails 판을 띄운다 → 앱은 뜨고 로그에 실패 문구.
11. **오른쪽 메뉴·두 번째 실행**: 설치본 설치 후 탐색기에서 파일 3개 선택 → `snB App 파일 업로드` → 한 번에 3개가 올라간다(로그 `IPC Batch Upload 실행 >>> 파일개수:3` 에 해당하는 Wails 로그). 앱이 꺼진 상태에서 같은 조작 → 로그인 뒤 실행.
    폴더 빈 곳 `snB App 파일 다운로드` → 체크한 행이 그 폴더로. 트레이로 숨긴 상태에서 바로가기 다시 실행 → 창이 앞으로.
12. **도메인 정책**: 관리자 권한으로 업무망 로그인 → hosts 끝에 표식 구역과 행이 생기고 원래 줄바꿈이 유지된다(바이트 비교). 앱 종료 → 구역만 사라지고 나머지 바이트가 로그인 전과 같다.
    관리자가 아니면 `도메인 등록 - Hosts 파일 열기에 실패했습니다.`.
13. **E2E 재사용**: Qt 판 E2E(`clipboard`, `clip-freeze`, `clipboard-aprv`, `cliphotkey-freeze`, 인증시험 `c09-clipboard` 0032~0040, `upload-multi` UP-10)는 UIA 이름·로그 문구로 판정한다.
    Wails(WebView2) 화면의 접근성 이름을 같은 글자로 맞추면 일부를 옮겨 쓸 수 있다 ― 로그 문구(`파일업로드 성공 >>> transIndex[` 등)도 같은 원문으로 남길지 함께 정한다.
