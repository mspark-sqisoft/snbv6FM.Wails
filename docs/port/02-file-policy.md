# 02. 파일·정책 검사 이식 조사

원본: `E:\git\snBv6FM`(Qt Quick + C++, 이하 "원본") · 대상: 이 저장소(Go + Wails v3 + Svelte 5, 이하 "Wails 판")
기준 명세: 원본 `specs/current/02-file-policy-check.md`(기준 커밋 63bbed4, 2026-10-04). 이 문서의 `POL-0xx` 는 그 명세의 요건 번호다.
조사일: 2026-10-30. 이 문서는 읽기만 해서 쓴 것이다 ― 원본·Wails 판 코드는 고치지 않았다.

표기 규칙

- 소스 위치는 원본 저장소 기준 상대 경로와 지금 소스의 줄 번호다. **명세에 적힌 줄 번호는 지금 소스와 어긋난다**(예: 명세는
  `checkExtPolicy` 를 `FileMoverGrpc.cpp:2692` 로 적지만 지금은 `:2780`, `checkZipFile` 은 명세 `:1785` → 지금 `:1812`). 이 문서는 지금 줄을 쓴다.
- 코드에서 직접 확인하지 못하고 짐작한 것은 앞에 **추정:** 을 붙였다.
- "없다" 고 적은 곳에는 무엇을 어디서 검색했는지 붙였다.
- **명세의 번호 중복**: `POL-034` 가 두 번 쓰인다(압축 해제 상한, 암호 압축 정책). 이 문서에서는 `POL-034(상한)`·`POL-034(암호)` 로 나눠 적는다.
  원본 명세를 고칠 때 하나를 새 번호로 바꿔야 한다.

---

## 1. 범위

### 포함

| 묶음 | 내용 | 원본의 주된 자리 |
|---|---|---|
| 검사 진입 | 업로드 목록의 개수 상한, `__CLIP__` 이름 제외, 작업 스레드 검사, 세대 번호로 지난 검사 버리기 | `src/ViewController.cpp` `uploadFiles`(:636)·`checkUploadFiles`(:2033) |
| 파일 하나의 판정 | 취소 → 존재 → 0바이트 → 열기·앞 256바이트 → 확장자 없음 → 크기 상한 → 헤더 패턴(black/white/gray) → 확장자 허용 목록 | `imports/FileMoverGrpc/FileMoverGrpc.cpp` `checkExtPolicy`(:2780-2885), `imports/FileMoverPolicy/extpolicy.cpp`·`filemoverpolicy.cpp`·`loginrespolicy.cpp` |
| 실패 문구 | 결과 코드 → 화면 문구와 [상세] 칸 | `src/ViewControllerNotifySlot.cpp` `getPolicyErrText`·`getPolicyErrDetail`·`getZipErrText` |
| 불일치 기록 | 실패 내역을 서버에 등록(전문 3316) | `FileMoverGrpc.cpp` `sendMismatchLog`(:2888) |
| 압축 내부 검사 | 압축 판정(확장자 7종), 중첩 깊이, 내부 파일 검사, 경로 탈출·링크 차단, 해제 상한(크기·항목 수·디스크) | `imports/ZipExtractor/zipextractor.cpp`, `FileMoverGrpc.cpp` `checkZipFile`(:1812) |
| 암호 압축 | 두 정책에 따른 통과·묻기·막기, 비밀번호 창, 2회 시도, 맞힌 비밀번호를 업로드 요청에 싣기 | `src/ZipPasswordGate.h`, `ViewController.cpp` `checkLockedZip`(:1828), `content/HomePage.qml`(:604-747) |
| 폴더 안 검사 | 폴더 아래 모든 파일 검사, 일부만 걸리면 걸린 것만 빼고 올리기 | `ViewController.cpp` `verifyUploadDir`(:1894)·`checkUploadFiles` |
| 검사 결과 창 | [파일 검사 결과] 창의 두 표, 문구, 단추, 2초 자동 진행 | `content/FileCheckResultDlg.qml`, `src/model/FileCheckResultModel.cpp` |
| 서버 검사 상태 | 서버파일 [상태] 열 문구(백신·개인정보·결재), 검출 내역 조회 | `imports/FileMoverPolicy/filemoverpolicy.cpp` `getState`, `ViewController.cpp` `getFileDetailInfo`(:1665) |
| 정책 수신 | 로그인 뒤 정책 조회 3종, 트레이 [정책 받아오기], 망마다 한 벌 | `ViewControllerNotifySlot.cpp:503-504`, `ViewController.cpp:318-326`, `filemoverpolicy.cpp` |

### 범위 밖(다른 영역 문서가 맡는다)

- 업로드 입구(파일 선택·끌어 놓기·IPC), 전송 대기열, 폴더 압축 자체, 검사값(SHA-256) 미리 계산, 업로드 권한 판정 → 03-upload.
- 클립보드 업로드 흐름 → 07-clipboard(여기서는 크기 상한이 따로라는 것만).
- 결재방식 결정, 결재자가 받은 압축의 내부 목록 창(`verifyDownloadZip`, 검사 창의 `DOWNLOAD` 모드) → 06-approval.
- 받을 수 있는지 판정(`DownloadBlock`) → 04-download. 이 문서의 `getState` 문구는 그쪽 [상태] 열이 쓴다.
- 7-Zip 엔진 파일 배포 → 15-build.
- WASM 전용 요건(POL-033, POL-047)은 Wails 판에 WASM 이 없으므로 다루지 않는다.

### Wails 판의 지금 상태

`client.go` 와 `README.md` 를 읽고 확인했다.

- 됨: `fmenv.ini` 읽기, PreLogin → Login → GetProfile(암호 모드 0), 프로파일 `'k=v,…'` 를 `Session.Profile`(map)로 파싱, 서버 파일 목록,
  파일 하나 업로드(결재 0/6 만, `Fpath` 에 파일 이름만, `extskip` 미설정 ― 원본도 설정하지 않는다), 다운로드.
- 없음: 확장자 정책·헤더 패턴·압축 검사·폴더 업로드·검사 결과 창. `client.go` 에서 `GetExtPattern`·`GetExtInfo`·`SetMismatch`·`zip` 을
  검색해 호출이 0건이었다(생성물 `pb/` 의 정의만 있다). README 표에도 "안 됨" 으로 적혀 있다.
- **프로파일 파싱이 원본과 다르다**: 원본 `setFmProfile2` 는 `split("=")` 결과가 정확히 2조각인 항목만 받아 `a=b=c` 를 버린다
  (`tst_FileMoverPolicy::setFmProfile2_ignoresMalformedEntries`). Wails 판은 `strings.Cut` 이라 `a` → `b=c` 로 받는다. 이 영역이 읽는
  키(`ext_check_method` 등)는 값에 `=` 가 없어 지금은 영향이 없지만, 파서를 원본과 맞춰 두는 편이 안전하다.

---

## 2. 기능 목록

우선순위: **P1** 은 원본과 같은 수준의 첫 출시에 꼭 필요한 것, **P2** 는 첫 출시 뒤 바로 해도 되는 것, **제외** 는 옮기지 않는 것(이유 함께).
난이도는 Go 로 옮길 때의 기준이다.

| ID | 기능 | Wails 우선순위 | 난이도 | 비고 |
|---|---|---|---|---|
| POL-001 | 업로드 목록을 작업 스레드에서 하나씩 검사, 새 검사가 오면 앞 검사 결과를 버린다(세대 번호) | P1 | 중 | Go 는 goroutine + `context.Context` 취소 + 세대 번호(`atomic.Uint64`). 화면 이벤트에 세대를 실어 Svelte 가 지난 결과를 버린다 |
| POL-002 | 고른 항목 수가 `fm_file_count` 를 넘으면 검사 전에 거절, 창 `파일 최대 전송 개수를 초과해서 전송할 수 없습니다.\n( 파일 최대 전송개수 : N )`. 자동 업로드는 로그만. 폴더는 1개 | P1 | 하 | 0 은 제한 없음. 로그인 응답 `fm_file_count`(필드 12) |
| POL-003 | 판정 순서 ① 취소 ② 존재 ③ 크기 0 ④ 열기·256바이트 ⑤ 확장자 없음 ⑥ 크기 상한 ⑦ 헤더 패턴 ⑧ 확장자 허용 목록, 첫 실패 하나만 | P1 | 중 | 명세 상태 '상충'(MIME 단계 없음). 순서를 그대로 지켜야 같은 파일에 같은 문구가 나온다. 7절 알고리즘 |
| POL-004 | 없는 파일 `파일 열기 실패`(-2), 0바이트 `파일 사이즈 0`(-1). 존재가 먼저 | P1 | 하 | 독점으로 잠긴 파일도 -2(Windows). macOS·Linux 는 권한 없음이 -2 |
| POL-005 | `no_ext_no_permit`≠0 이면 확장자 없는 파일 `확장자 없음`(-3) | P1 | 하 | 확장자 규칙: 마지막 `.` 뒤, 점이 0번 자리이거나 맨 끝이면 확장자 없음(`.bashrc`·`a.` 는 없음) |
| POL-006 | 크기 ≥ `fm_file_size`×1024 이면 `전송 가능한 파일 크기 초과`(-11), [상세] `<크기> / 상한 <상한>` | P1 | 하 | **같으면 막힌다**(`상한 > 크기` 일 때만 통과). 0 은 제한 없음 |
| POL-007 | 클립보드는 `fm_clip_size` 로 따로, 넘으면 창 `전송 가능한 파일 크기 초과` | P1 | 하 | 흐름은 07-clipboard. 확장자·헤더 검사 없음 |
| POL-008 | 앞 256바이트 읽어 앞 64바이트(대문자 16진 128자)를 헤더 패턴과 대조, `X` 는 반 바이트 와일드카드, 패턴 길이만큼, 16진 밖 글자 행은 버림 | P1 | 중 | 경고 `헤더패턴 행을 버린다 >>> 확장자: %1, 패턴: %2, 글자: '%3'`. 7.2절 |
| POL-009 | `ext_check_method` 가 `white`→white, `gray`→gray, 그 밖·없음→black | P1 | 하 | 대소문자 그대로 비교(추정: 원본이 `==` 문자열 비교. `filemoverpolicy.cpp:119-134`) |
| POL-010 | black 규칙(-6 미등록, -9 허용 없음, -7 변경, -8 패턴 없음) | P1 | 중 | 7.3절. 원본 단위 시험 `blackMode_*` 6개를 Go 표 시험으로 옮긴다 |
| POL-011 | white 규칙 | P1 | 중 | 7.3절. `whiteMode_*` 3개 |
| POL-012 | gray 규칙 | P1 | 중 | 7.3절. `grayMode_*` 3개 |
| POL-013 | `fm_chk_header` 를 읽지 않고 헤더 단계를 늘 수행 | P2 | 하 | 명세 '상충'. MOSFM 은 0 이면 건너뛴다. Wails 판은 MOSFM 대조 뒤 키를 읽도록 바꾸는 것을 권한다(8절). 첫 출시는 원본과 같게 늘 수행 |
| POL-014 | 확장자 허용 목록(전문 3005)에 `allow_upload=0` 이 하나라도 있으면 `허용되지 않은 확장자`(-4), 대소문자 무시 | P1 | 하 | 목록에 없으면 통과. 키는 대문자로 담고 대문자로 찾는다 |
| POL-016 | 실패 코드별 문구(-11~0) | P1 | 하 | 문구 표는 6.1절. -5 를 내는 경로는 없다(-5 는 -7 로 바뀐다) |
| POL-017 | [상세] 칸: 정상=크기, -11=크기와 상한, -4·-6=`.확장자`(소문자). 헤더 16진·시스템 오류는 넣지 않음 | P1 | 하 | 크기 표기 함수는 7.6절 |
| POL-018 | 취소가 아닌 실패를 쌓아 두었다가 검사 창의 답(진행·취소) 때 `SetMismatch`(3316)로 등록, 한 번에 5건 | P1 | 중 | 명세 '부분'. 원본의 5건 한도·유실·버퍼 넘침은 따라 하지 않는다(8절). 서버 반영 확인 E2E 가 없다 |
| POL-019 | 사용자 취소(-10)는 불일치 기록에 넣지 않음 | P1 | 하 | |
| POL-023 | 검사 창 [취소]·바깥 누름·로그아웃이면 도는 검사를 다음 파일(압축은 다음 항목)에서 멈춤, 기다리지 않음 | P1 | 중 | `context` 취소로 자연스럽다. 압축 풀기 루프 안에서도 `ctx.Err()` 를 본다 |
| POL-024 | 검사 100건마다 50ms 쉼 | 제외 | 하 | 원본은 GUI·네트워크 스레드 양보용(추정). Go 는 화면이 별도 프로세스(WebView)라 필요 없다. 디스크 부하가 문제면 동시 개수 제한으로 푼다 |
| POL-025 | 확장자 `ZIP`·`ZIPX`·`TAR`·`GZ`·`LZH`·`7Z`·`XZ` 만 압축으로 본다, 압축 자신이 POL-003 을 먼저 통과해야 내부 검사 | P1 | 하 | **RAR·BZ2·CAB·ISO 는 압축으로 보지 않는다** ― 일반 파일로 헤더·확장자만 검사. 원본과 같게 둔다(넓히면 정책 변경) |
| POL-026 | `fm_chk_insidezip` 이 없거나 1 이면 내부 검사, 0 이면 통과 | P1 | 하 | 키 없음 = 검사(키 유무를 구별해야 한다 ― Go map 의 `v, ok`) |
| POL-027 | 중첩 깊이 `fm_extract_zipdepth`(없으면 2, 1~5 로 자름), 넘으면 `압축 횟수 초과`(-991) | P1 | 중 | 잘랐으면 경고 `압축 중첩 깊이 정책 값을 범위로 자른다 >>> fm_extract_zipdepth[%1] → %2`. `tar.gz` 는 깊이 2개를 쓴다(7.4절) |
| POL-028 | 내부 파일마다 POL-003 검사, 아래 표 (압축 이름, 결과, 내부 파일, 상세), 하나라도 걸리면 압축 전체 막음, 위 표 `압축 해제 성공`·`내부 파일 N개, 오류 M건` | P1 | 중 | 안쪽 압축 파일 자신도 목록에 들어가 검사받는다. 내부 0바이트 파일도 `파일 사이즈 0` 으로 압축 전체를 막는다 |
| POL-029 | 풀지 못하면 사유 문구(-999~-986)로 막음, 푼 항목 수 ≠ 목록 파일 수면 `알 수 없는 오류`(-989) | P1 | 중 | 오류 코드를 Go 라이브러리 오류에서 다시 만들어야 한다(7.5절, 위험 항목) |
| POL-030 | 항목 이름이 `/`·`\`·드라이브 문자로 시작, `..`(뒤 공백 포함) 단계, 심볼릭·하드 링크면 아무것도 풀지 않고 `허용되지 않는 경로 포함`(-987) | P1 | 중 | 로그 `압축파일 경로 검사 거절(허용되지 않는 경로 또는 링크) >>> 압축파일: …, 항목: …`. 폴더 받기(04)도 같은 함수를 쓴다 |
| POL-031 | `<사용자 데이터>/temp/` 아래 임시 폴더에 풀고 끝나면 지움 | P1 | 하 | Wails 판은 잎 파일은 디스크에 쓰지 않고 앞 256바이트만 읽는 방식을 권한다(8절). 안쪽 압축만 임시 파일 |
| POL-032 | 큰 항목(1MiB 초과)은 1%·256바이트 넘게 쓰이면 멈춤, 내부 파일 크기 판정은 잘린 파일로 | P1 | 중 | 원본 결함 성격(크기 상한을 넘는 내부 파일이 통과). Wails 판은 목록에 적힌 크기로 판정을 권한다(8절, 결정 필요) |
| POL-034(상한) | 풀기 한 번마다 10GiB·10만 항목·디스크 여유 1GiB, 넘으면 -985·-984·-983 | P1 | 중 | 목록 크기와 실제로 쓴 양 둘 다로 센다. 로그 `압축 풀기 중단 >>> 사유[…], 경로[업로드 검사|…], …` |
| POL-034(암호) | 바깥이 잠긴 압축(-990): `fm_disable_zip_pass=1` 통과 / `fm_use_Zip_pwd=1` 이면 사람에게 묻고 자동은 막음 / 둘 다 아니면 `압축 패스워드 존재` | P1 | 하 | 키 이름 대소문자 그대로(`fm_use_Zip_pwd`). 판정표는 7.7절 |
| POL-035 | 「압축 비밀번호 입력」 창: 검사 창 위, 압축 이름, 가려진 입력, 비면 [확인] 비활성, Enter 답, Esc·[취소]·닫기는 취소 | P1 | 하 | Svelte 모달. 5.3절 |
| POL-036 | 비밀번호로 압축 전체를 시험(디스크에 안 씀) 뒤 풀어 검사, 2회까지, 두 번째 문구 `비밀번호가 틀렸습니다. 다시 입력하세요.`, 틀리면 -1001, 취소 -1002 | P1 | 상 | 압축 전체 시험 = 모든 항목을 끝까지 풀어 CRC·MAC 확인. ZIP(ZipCrypto·AES)·7z(AES) 둘 다 필요 |
| POL-037 | ZIP 과 7z(헤더 암호 포함)를 같은 방식으로 판정 | P1 | 상 | 7z 헤더 암호는 목록조차 못 읽는다 → 그때도 -990 으로 묻기 |
| POL-038 | 바깥은 풀렸는데 안쪽 압축이 잠겼으면 묻지 않고 `압축 패스워드 존재`(-986) | P1 | 하 | |
| POL-039 | 암호 압축을 파일마다 따로 묻고 앞 비밀번호를 재사용하지 않음 | P1 | 하 | |
| POL-040 | 비밀번호 창이 떠 있는 동안 취소·로그아웃·새 검사가 오면 질문을 취소로 깨우고 창을 닫음, 그만둔 검사는 창을 다시 띄우지 않음 | P1 | 중 | Go 는 `select { case pw := <-answer: case <-ctx.Done(): }`. 질문 이벤트에 세대 번호를 실어 화면이 지난 질문을 무시 |
| POL-041 | 사람이 올린 낱개 파일에서 맞힌 비밀번호만 업로드 요청 `zip_pswd` 에 싣는다. 폴더 안 압축은 안 실음, 128자 초과는 안 실음 | P1 | 하 | `FileTransInfo.zip_pswd`(필드 12). 전송 쪽은 03-upload |
| POL-042 | 비밀번호를 로그(길이만 `PW[*** len=N]`)·설정·레지스트리에 남기지 않고, 막히거나 취소되면 버림 | P1 | 하 | Svelte 쪽 콘솔 로그·Wails 바인딩 로그에도 남지 않게 한다(Wails 의 메서드 호출 디버그 로그 주의) |
| POL-043 | 폴더 아래 모든 파일(하위·숨김 포함)을 검사, 아래 표에 (폴더, 결과, 파일, 상세), 걸린 것이 없으면 `정상 폴더` | P1 | 중 | 폴더 업로드(03) 자체가 P1 일 때 함께. `filepath.WalkDir`. 심볼릭 링크 폴더는 원본이 따라가지 않는다(추정: `QDirIterator` 기본값) |
| POL-044 | 사람이 올린 폴더에서 일부만 걸리고 통과 1개 이상이면 `차단 N건 제외`·`(제외됨)`·[차단파일 제외 업로드] | P2 | 중 | 압축 직전 폴더가 바뀌었는지 대조는 03-upload. 로그 `<폴더> 차단 파일 N건 제외` |
| POL-045 | 전부 걸린 폴더·낱개 파일·자동 업로드에는 빼고 올리는 길이 없음(`오류 파일 존재`) | P1 | 하 | |
| POL-046 | 폴더의 개별 파일 크기만 비교하고 압축본 크기는 다시 비교하지 않음 | P2 | 하 | 원본 그대로면 상한을 넘는 압축본이 올라간다. 서버가 막는지 확인 필요(8절) |
| POL-048 | 검사 시작마다 [파일 검사 결과] 모달, `검사가 진행중입니다.`, [확인] 숨김, 열 때 앞 상태 초기화 | P1 | 하 | 5.1절 |
| POL-049 | 오류 0 이면 `검사결과 모두 정상 파일입니다.` 뒤 2초 후 자동 진행, 오류가 있으면 `오류 파일 개수 : N` 과 [취소] 만 | P1 | 하 | 자동 업로드도 같은 창 |
| POL-050 | 답 없이 닫히면 취소로 답함, Esc 로 안 닫힘, 답은 한 번만 | P1 | 하 | |
| POL-051 | 위 표 `#`·`파일명`·`결과`·`상세`, 아래 표 `#`·`파일명`·`내부 파일`·`결과`·`상세`, 정상 계열은 초록, 그 밖 빨강 | P1 | 하 | 원본은 내부 파일이 걸린 압축의 위 표 행도 초록(`압축 해제 성공`) ― 8절에서 고치기를 권한다 |
| POL-053 | 서버파일 [상태] 문구: 백신 → 개인정보 → 결재 순 | P1 | 하 | **명세 표에 빠진 값**: 백신 6 `OLE 검출 파일 차단`(`filemoverpolicy.cpp getState`). 6.4절 |
| POL-055 | 서버파일 우클릭 [개인정보·바이러스·APT 검출 조회] 창 | P2 | 중 | 전문 3096. 우클릭 메뉴는 12-main-window 와 얽힌다 |
| POL-057 | 로그인 뒤 정책 3종(3007·3005·3015)과 링크 URL(3094) 조회, 트레이 [정책 받아오기] 로 다시 | P1 | 하 | Wails 판은 GetProfile 만 한다. 3005·3015 를 더한다. 3094 는 링크연동 영역 |
| POL-058 | 정책을 망마다 한 벌, 응답마다 통째로 교체, 새 로그인·로그아웃 때 비움, 검사 하나는 시작 때 잡은 한 벌로 끝까지 | P1 | 중 | Go 는 불변 구조체 포인터를 `atomic.Pointer[PolicySet]` 로 교체. 09 영역(다중 망)과 얽힌다 |
| POL-059 | 확장자 정보를 방향 2(전체)로 받아 지금 망의 보내는 방향(업무망 1·인터넷망 0)과 양방향(2) 행만 씀 | P1 | 하 | `course==location || course==2` |
| POL-060 | 정책을 아직 못 받았으면 black(빈 표)이 되어 전부 `등록되지 않은 확장자`, 내부 검사 켜짐, 암호 압축 막힘 | P1 | 하 | Wails 판은 정책 3종이 다 올 때까지 검사를 잠깐 기다리게 하는 것을 권한다(8절). 그래도 못 받으면 원본과 같이 막는다 |
| POL-061 | [정책] 화면(유형·단어 거르기) | 제외 | 중 | 원본에서 2026-08-20 부터 메뉴에서 감춰져 사용자가 열 수 없다(명세 '부분'). 되살릴지 정해진 바 없다 |
| 미구현·보류 묶음 | 미구현: MIME 검사(POL-015), 불일치 누적 한도 `fm_max_mismatch`(POL-020), 확장자별 결재 `aFlag`(POL-022). 보류: 하루 전송 한도(POL-021), 같은 이름 파일 행(POL-052), APT 걸린 파일이 `다운로드 불가` 로 보임(POL-054), `개인정보 검색 중` 결재요청 회귀(POL-056) | 제외 | ― | 원본에도 없거나 결정이 안 났다. 단 POL-052 는 Wails 판이 행 ID 로 찾으면 저절로 풀린다(8절). POL-054 는 문구 하나라 결정이 나면 바로 바꾼다 |

---

## 3. 서버 통신

모든 RPC 는 Wails 판이 이미 쓰는 TLS gRPC 채널(`client.go` 의 연결, 시간 제한 `rpcTimeout` 15초)로 보낸다. 전문 번호는 원본 코드·로그가
쓰는 옛 번호이고 gRPC 메서드 이름과 짝이다. proto 는 이 저장소 `proto/` 의 사본이다.

### 3.1 정책 수신 순서

```
Login (LoginResponse: fm_file_size 11, fm_file_count 12, fm_clip_size 14)
  └─ 로그인 완료 뒤 (원본 ViewControllerNotifySlot.cpp:503-504, 비동기로 연달아 보낸다)
       ├─ [3007] CommonService.GetProfile        → 프로파일 'k=v,…'
       ├─ [3005] FileExtensionService.GetExtInfo → 확장자 허용 목록
       ├─ [3015] FileExtensionService.GetExtPattern → 헤더 패턴
       └─ [3094] 링크연동 URL 목록(이 영역 밖)
트레이 [정책 받아오기] (ViewController.cpp:318-326): 위 넷을 다시 보낸다
```

- 원본은 응답 순서를 맞추지 않는다. 응답 하나가 오면 그 부분만 바꾼 새 한 벌로 통째로 교체한다(POL-058).
- 새 로그인 시작과 로그아웃 때 그 망의 한 벌을 비운다(`FileMoverPolicy::reset`).
- 프로파일은 로그인 때 읽으므로 서버 설정을 바꾼 뒤에는 다시 로그인해야 반영된다(원본 `docs/agents/서버_참조.md`).

### 3.2 [3007] `CommonService.GetProfile`

| 방향 | 필드 | 값 |
|---|---|---|
| 요청 | `ProfileRequest.userid` | 로그인 ID |
| 응답 | `profile` | `'키=값,키=값,…'` 문자열. Wails 판 `client.go` 가 이미 받아 `Session.Profile` 에 넣는다 |

이 영역이 읽는 키는 4절.

### 3.3 [3005] `FileExtensionService.GetExtInfo`

| 방향 | 필드 | 값·쓰임 |
|---|---|---|
| 요청 | `ExtInfoRequest.type` | **2**(전송 방향 전체) |
| 응답 | `count` | 건수(쓰지 않음) |
| 응답 | `list[].extName` | 확장자. 담을 때 대문자로 |
| 응답 | `list[].allow_upload` | 0 이면 차단(POL-014) |
| 응답 | `list[].course` | 1 업무망→, 0 인터넷망→, 2 양방향. 지금 망 `location` 과 같거나 2 인 행만 씀(POL-059) |
| 응답 | `list[].aFlag`·`mode`·`aprv_user_flag`·`aprv_type` | 원본은 저장만 하고 판정에 안 씀(POL-022). Wails 판도 담기만 해도 된다 |

원본 처리: `FileMoverGrpc.cpp` `setExtInfo`(:1341) → `ExtPolicy::setExtProfile(vec, serverInfo_.location)`.

### 3.4 [3015] `FileExtensionService.GetExtPattern`

| 방향 | 필드 | 값·쓰임 |
|---|---|---|
| 요청 | `ExtPatternRequest.reqMsg` | 빈 문자열 |
| 응답 | `list[].extName` | 확장자 |
| 응답 | `list[].hFlag`(bool) | 헤더 패턴 사용 여부 |
| 응답 | `list[].extPatt` | 16진 패턴(`X` 와일드카드) |
| 응답 | `list[].aFlag`(bool) | 허용 여부 |

- 원본 처리: `setExtPatt`(:1253) 가 응답을 C 구조체 `EXT_PATT_INFO`(`extName[8]`, `extPatt[128]`)로 옮긴 뒤 `setPattData` 에 넘긴다.
  **그래서 원본은 7바이트를 넘는 확장자 이름을 자르고 127자를 넘는 패턴을 자른다.** Wails 판은 자르지 않는다(위험 항목 참고 ― 같은
  응답에서 판정이 달라질 수 있는 유일한 자리).
- 로그: 원본은 `응답 건수 … 저장된 행 …` 형식으로 응답 건수와 버리지 않고 담은 행 수를 남긴다. Wails 판도 같은 두 숫자를 남긴다.
- **서버 결함 SR032**: 시험 서버는 같은 확장자의 헤더패턴 행 중 정렬상 첫 행이 아닌 행의 차단을 허용으로 보낸다. SBM 에서 차단으로
  바꾼 확장자가 앱에는 허용으로 온다. 앱 쪽에서는 고칠 수 없다(원본 명세 "확인 필요" 1).

### 3.5 [3316] `FileExtensionService.SetMismatch`

| 방향 | 필드 | 원본이 채우는 값 |
|---|---|---|
| 요청 | `MismatchRegRequest.user_id` | 로그인 ID |
| 요청 | `m_count` | 이번에 싣는 건수(≤5) |
| 요청 | `list[].fname` | 파일 **전체 경로**(압축 안 파일은 임시 폴더 경로). 원본은 EUC-KR 로 바꿔 담는다 |
| 요청 | `list[].extName` | 확장자(대문자) |
| 요청 | `list[].extPatt` | 앞 64바이트의 16진 128자 |
| 요청 | `list[].desc1` | 영문 사유(아래 표) |
| 응답 | `AnswerResponse` | 원본은 실패해도 로그만 남기고 화면 알림 없음 |

`desc1` 영문 사유(원본 `checkExtPolicy` catch 절, 서버 DB 에 그대로 쌓이므로 글자 그대로 맞춘다):

| 결과 코드 | `desc1` |
|---|---|
| -6 | `No Extension in DB` |
| -9 | `Not Allowed Extension` |
| -7 | `Renamed pattern` |
| -8 | `Not exist matched pattern in DB` |
| -1 | `File Size Zero` |
| -4 | `EXT_PROFILE NOT ALLOWED` |
| -11 | `File Size Over Than Profile` |
| -3 | `File has no extension` |
| -2 | `File Open Error` |
| 그 밖 | `Unknown` |
| -10(취소) | 쌓지 않는다 |

- 보내는 때: [파일 검사 결과] 창의 답이 올 때(취소·진행 두 경로 모두, `fileCheckReply`).
- 원본은 쌓인 것 중 앞 5건만 보내고 지운다. 나머지는 다음 답까지 메모리에 남고, 앱을 끄면 사라진다.
- 압축 안 파일·폴더 안 파일의 실패도 쌓인다(압축 해제 오류 -99x 는 `checkExtPolicy` 를 거치지 않으므로 쌓이지 않는다).
- **원본 결함(추정, 코드 읽기로만 확인)**: `FILE_MISMATCH_PATTERN` 은 `extName[16]`, `extPatt[128]`, `fname[512]`, `desc1[256]` 순서다.
  파일이 64바이트 이상이면 `extPatt` 128바이트가 NUL 없이 꽉 차서 `setExtPatt(char*)` 가 뒤의 `fname` 까지 이어 읽는다. 또 `fname` 은
  EUC-KR 바이트인데 protobuf `string` 으로 넘길 때 UTF-8 로 읽으므로 한글 경로가 깨질 수 있다. Wails 판은 Go 문자열로 바로 채워
  둘 다 생기지 않는다. 서버 DB 의 기존 기록과 모양이 달라질 수 있다는 점만 기억한다.
- 추정: 서버가 `fname` 칸 길이(512바이트)를 넘는 경로를 어떻게 처리하는지는 확인하지 않았다. Wails 판은 UTF-8 기준 500바이트 안으로
  자르는 것을 권한다.

### 3.6 [3311] `ServerFileService.GetServerFileList` 의 검사 상태 필드

목록 조회 자체는 04-download 가 맡는다. 이 영역은 행마다 아래 필드로 [상태] 문구를 만든다(6.4절).

| 필드(번호) | 쓰임 |
|---|---|
| `virus_flag`(11) | 백신 상태 |
| `priv_state`(13) | 개인정보 상태 |
| `priv_cnt`(19) | 개인정보 건수 |
| `priv_detail`(22) | 개인정보 검출 내역(검출 조회 창) |
| `state`(9) | 결재 상태 |
| `zip_pswd`(21) | 결재요청 때 되돌려 보내는 압축 비밀번호(06-approval) |

### 3.7 [3096] `ServerFileService.GetDetectionDetail`

| 방향 | 필드 | 값 |
|---|---|---|
| 요청 | `DetectionDetailRequest.spath` | 서버 파일 경로 |
| 요청 | `type` | 1 APT, 2 바이러스(3 DRM 은 메뉴 없음, 0 개인정보는 이 RPC 를 쓰지 않음) |
| 응답 | `detail` | 검출 내역 문자열. `-` 는 내용 없음 |

- 원본 `reqDetectionDetail`(:1229): 실패해도 `1_<type>_` (빈 내용)으로 화면에 알려 창을 띄운다 → `검출된 내용이 없습니다.`.
- 원본 화면 통지 형식은 `<팝업>_<유형>_<내용>` 이고 앞 두 칸만 잘라 나머지를 내용으로 본다(내용에 `_` 가 있어도 된다).

### 3.8 오류 처리

- 정책 조회 3종이 실패하면 원본은 로그만 남기고 화면 알림을 띄우지 않는다. 그 결과 그 부분은 빈 채로 남아 POL-060 동작(모두 막힘)이 된다.
  추정: 원본 실패 로그의 정확한 문구는 `FileMoverGrpc.cpp` 의 각 응답 처리(:1253, :1341 부근)의 실패 분기를 봐야 한다 ― 이 조사에서는 성공
  경로만 읽었다.
- gRPC 상태 코드별 화면 문구는 이 영역에 따로 없다. Wails 판은 `client.go` 의 공통 오류 처리(이미 있는 것)를 쓰고, 정책 조회 실패는
  "정책을 받지 못해 업로드를 검사할 수 없습니다" 같은 안내를 업로드 시도 때 보이는 것을 권한다(원본보다 나은 점, 8절).

---

## 4. 정책 키

| 키 | 출처 | 형식 | 없을 때 | 쓰는 곳 |
|---|---|---|---|---|
| `fm_file_count` | 로그인 응답 필드 12 | 정수 | 0 = 제한 없음 | POL-002 |
| `fm_file_size` | 로그인 응답 필드 11 | KB 정수(×1024 해서 바이트, `uint64`) | 0 = 제한 없음 | POL-006 |
| `fm_clip_size` | 로그인 응답 필드 14 | KB 정수 | 0 = 제한 없음 | POL-007 |
| `fm_file_up`·`fm_file_down` | 로그인 응답 | 0/1 | 0 = 막음 | 업로드 권한(03). 업무망(loc 1)은 `fm_file_up`, 인터넷망은 `fm_file_down` 을 본다 |
| `ext_check_method` | [3007] | `white`·`gray`·그 밖 | black | POL-009 |
| `no_ext_no_permit` | [3007] | 정수 | 0(통과) | POL-005 |
| `fm_chk_insidezip` | [3007] | 0/1 | **검사함**(키 없음과 0 을 구별) | POL-026 |
| `fm_extract_zipdepth` | [3007] | 정수 | 2, 범위 1~5 로 자름(숫자가 아니면 자름 경고) | POL-027 |
| `fm_disable_zip_pass` | [3007] | 0/1 | 0 | POL-034(암호) |
| `fm_use_Zip_pwd` | [3007] | 0/1, **키 이름 대소문자 그대로** | 0 | POL-034(암호) |
| 읽지 않는 키 | `fm_chk_header`(POL-013), `check_mime`·`mime_max`·`download_mime_types`(POL-015), `fm_max_mismatch`(POL-020) | ― | ― | 원본 `src`·`imports`·`content` 에서 이 이름들을 검색해 0건(명세 근거 칸) |

- 원본은 값을 `QVariant::toInt()` 로 읽는다 ― 숫자가 아니면 0. Go 는 `strconv.Atoi` 실패를 0 으로 보되, `fm_chk_insidezip`·`fm_extract_zipdepth`
  처럼 "키 없음" 과 "0" 이 다른 키는 키 유무를 먼저 본다.
- 업로드 상한 확인은 원본 앱 로그 `권한 조회 >>> fileMaxSize:` 줄. Wails 판도 같은 형식의 줄을 남기면 원본과 나란히 비교할 수 있다.
- 시험 서버에서 키를 바꾸는 법은 9절.

---

## 5. 화면 동작

### 5.1 [파일 검사 결과] 창 (`content/FileCheckResultDlg.qml`)

| 항목 | 원본 동작 |
|---|---|
| 종류 | 모달 대화상자, 제목 `파일 검사 결과` |
| 열기 | `show(gen)`: 자동 닫기 타이머 정지, 결재자 받기 모드 끔, [취소] 보임, 문구 `검사가 진행중입니다.`(초록), [확인] 숨김, 받은 세대 번호 저장 |
| 크기 | 폭 = min(아래 표에 행이 있으면 1000, 없으면 700, 부모 폭 - 40). 높이 = min(500, 245 + max(행 수, 3) × (행 높이 + 1)) |
| 위 표 | 머리 `#`·`파일명`·`결과`·`상세`, 폭 40·180·200·나머지(최소 60). 결과가 `정상 파일`·`정상 폴더`·`압축 해제 성공` 이면 연녹색, 아니면 연빨강 |
| 아래 표 | 제목 `내부 파일 검사 결과`, 행이 있을 때만 보임. 머리 `#`·`파일명`·`내부 파일`·`결과`·`상세`, 폭 40·150·150·200·150. `정상 파일` 만 녹색. 빈 상태 문구 `조회된 데이터가 없습니다.` |
| 결과 통지 `FILE_CHECK_RESULT(fails, value)` | `value` 가 `SKIP:n` 이면 skip=n. [확인] 은 fails==0 이거나 skip>0 일 때 보임. 글자는 skip 이면 `차단파일 제외 업로드`, 아니면 `확인` |
| fails==0 | 문구 `검사결과 모두 정상 파일입니다.`. `value` 가 `DOWNLOAD` 가 아니면 2초 타이머 → 진행으로 답하고 닫힘 |
| skip>0 | 문구 `오류 파일 개수 : N` + ` (차단 파일 M건을 빼고 올립니다)`(빨강). 자동으로 닫지 않음 |
| 그 밖 | 문구 `오류 파일 개수 : N`(빨강), [취소] 만 |
| `value=="DOWNLOAD"` | 결재자 받기 모드(06-approval). [확인] 만 보이고 [취소] 숨김, 답을 서버로 보내지 않음 |
| `value=="AUTO"` | 자동 업로드. 창 동작은 같다 |
| 답 | `reply(reject)` 는 한 번만(`replied`). 결재자 받기 모드가 아닐 때만 `fileCheckReply(gen, reject)` |
| 닫힘 | 답 없이 닫히면(바깥 누름·로그아웃 정리) `reply(true)` = 취소. Esc 로는 닫히지 않음 |
| 창 닫기 통지 | `FILE_CHECK_DLG_CLOSE`: 사람이 자동 업로드 검사를 밀어냈을 때 화면이 창을 닫음(`HomePage.qml onQmlFileCheck`) |
| 자동화 이름 | `fileCheckTable`(접근성 이름 `파일 검사 결과 목록`)·칸 `resultCell`, `zipCheckTable`(`내부 파일 검사 결과 목록`)·칸 `innerFileCell`·`resultCell` |

Svelte 이식 메모

- 표 두 개는 같은 행 타입 `{id, parent, result, inner, detail, ok bool}` 을 쓰고, 색은 문구 비교가 아니라 `ok` 로 정한다(원본은 문구 문자열로 색을 정한다).
- Go 쪽이 행을 하나씩 이벤트(`filecheck:row`)로 보내고 Svelte 가 붙인다. 행 갱신은 **행 ID** 로 한다(원본은 파일 이름으로 찾아 POL-052 결함이 있다).
- Esc 무시, 바깥 누름은 취소. 원본은 Qt `Popup` 기본 동작에 기댄 것이라 Svelte 에서는 명시해야 한다.
- 접근성 이름(`aria-label`)을 원본 접근성 이름과 같게 주면 E2E 하네스(UIA)가 재사용할 여지가 있다. 추정: WebView2 의 UIA 노출은 표 칸
  이름을 다르게 낼 수 있어 하네스 수정이 필요할 수 있다.

### 5.2 결과 행이 채워지는 방식 (`ViewController.cpp`, `FileCheckResultModel.cpp`)

| 상황 | 위 표 | 아래 표 |
|---|---|---|
| 낱개 파일 시작 | `정상 파일` + 크기 행을 먼저 넣는다 | ― |
| 낱개 파일 정책 실패 | 그 행을 실패 문구 + 상세로 고침 | ― |
| 압축, 내부 검사 성공 | `압축 해제 성공`, 상세 `내부 파일 N개, 오류 M건`(오류 0 이면 `내부 파일 N개`) | 내부 파일마다 (압축 이름, 문구, 내부 파일 이름, 상세) |
| 압축, 풀기 실패(<0) | `getZipErrText` 문구 | 폴더 안 압축이면 아래 표 행으로 |
| 폴더, 걸린 것 없음 | `정상 폴더` | 파일마다 행 |
| 폴더, 일부 걸림 + 통과 ≥1 + 사람이 올림 | `차단 N건 제외`, 상세 `내부 오류 N건` | 걸린 행 결과 앞에 `(제외됨) ` |
| 폴더, 그 밖 실패 | `오류 파일 존재`, 상세 `내부 오류 N건` | |

### 5.3 「압축 비밀번호 입력」 창 (`content/HomePage.qml:604-747`)

| 항목 | 원본 동작 |
|---|---|
| 띄우는 통지 | `ZIP_PASSWORD_ASK(name, retry)` ― retry 가 `RETRY` 면 두 번째. `name` 이 비면 "닫으라" 는 뜻(답한 것으로 두고 닫음) |
| 위치 | 검사 창보다 위(z 값 +1), 폭 min(부모×0.8, 420), 모달 |
| 문구 | 처음 `압축 파일의 비밀번호를 입력하세요.`, 두 번째 `비밀번호가 틀렸습니다. 다시 입력하세요.`(경고 색) |
| 파일 이름 | 일반 글자(서식 해석 안 함), 최대 3줄 |
| 입력란 | 가려진 입력, 자리표시 `비밀번호`. Enter 는 내용이 있을 때만 답 |
| 단추 | [확인](주 단추, 비어 있으면 비활성), [취소] → 빈 답 |
| 닫힘 | Esc 로 닫힘(= 취소). 답 없이 닫히면 빈 답 |
| 자동화 이름 | `zipPasswordDialog`·`zipPasswordPrompt`·`zipPasswordFileName`·`zipPasswordInput` |

Svelte 이식 메모: `<input type="password" autocomplete="off">`. 답은 Wails 바인딩 메서드 `ZipPasswordReply(gen, pw)` 로 보내고, 세대가 다르면
Go 가 버린다. 창을 닫을 때 입력값을 지운다. 브라우저 비밀번호 저장 제안이 뜨지 않게 `autocomplete="off"`·폼 태그 없이 둔다.

### 5.4 검출 조회 (`content/ServerFileScrollView.qml:50-78`, `ServerFileCardView.qml`)

| 항목 | 원본 동작 |
|---|---|
| 메뉴 | 서버파일 행 우클릭 `개인정보 검출 조회`(유형 0), `바이러스 검출 조회`(2), `APT 검출 조회`(1). DRM 메뉴 없음 |
| 여럿 선택 | `한 번에 하나의 파일만 조회 가능합니다.` |
| 개인정보 | 목록에 실려 온 `priv_detail` 을 가공: `[SI]`·`case detected` 제거, 줄마다 `()` 또는 `(0)` 인 줄 제외. 비었거나 서버 파일 이름과 같으면 `검출된 내용이 없습니다.` |
| 바이러스·APT | 전문 3096 조회. `-` 나 빈 값이면 `검출된 내용이 없습니다.` |
| 창 제목 | `개인정보 검출 결과`, `APT 검출 결과`, `바이러스 검출 결과`(`DRM 검출 결과` 도 문구는 있다) |

### 5.5 그 밖의 알림

| 상황 | 원본 문구 | 형태 |
|---|---|---|
| 개수 초과 | `파일 최대 전송 개수를 초과해서 전송할 수 없습니다.\n( 파일 최대 전송개수 : %1 )` | 창, 제목 `정보` |
| 클립보드 크기 초과 | `전송 가능한 파일 크기 초과` | 창, 제목 `정보` |
| 업로드 권한 없음 | `현재 망에서는 업로드할 수 없습니다.` | 알림 띠(03) |
| 목록 경합 | `앞 업로드를 마친 뒤 다시 시도하세요.` | 알림(03) |
| `__CLIP__` 이름 | `사용할 수 없는 파일명이 포함되어 있어 업로드에서 제외되었습니다.( __CLIP__ )` | 로그 |
| 취소 | `사용자에 의해 파일 업로드가 취소되었습니다.` | 로그 |
| 지난 검사의 답 | `파일검사 회신 >>> 지난 검사의 답이라 버린다 …` | 로그 |

---

## 6. 경계·오류 상황

### 6.1 파일 하나의 결과 코드와 문구

| 코드 | 화면 문구(`getPolicyErrText`) | [상세](`getPolicyErrDetail`) | 생기는 곳 |
|---|---|---|---|
| 0 | `정상 파일` | 크기 | |
| -1 | `파일 사이즈 0` | | 크기 0 |
| -2 | `파일 열기 실패` | | 없음·열기 실패·앞 256바이트 읽기 0 이하 |
| -3 | `확장자 없음` | | `no_ext_no_permit` |
| -4 | `허용되지 않은 확장자` | `.확장자`(소문자) | 확장자 허용 목록(POL-014)만(헤더 패턴의 -4 는 -9 로 바뀐다, 아래 주의) |
| -5 | `변경된 확장자` | | 내는 경로 없음 |
| -6 | `등록되지 않은 확장자` | `.확장자` | 헤더 패턴 -3 |
| -7 | `변경된 확장자` | | 헤더 패턴 -5 |
| -8 | `존재하지 않는 헤더패턴` | | 헤더 패턴 -6, 그리고 알 수 없는 값(경고 뒤) |
| -9 | `허용되지 않은 확장자 헤더패턴` | | 헤더 패턴 -4 |
| -10 | `사용자 취소` | | 실행 플래그 꺼짐 |
| -11 | `전송 가능한 파일 크기 초과` | `<크기> / 상한 <상한>`(상한 0 이면 크기만) | 크기 상한 |
| 그 밖 | `알 수 없는 오류` | | |

주의: 헤더 패턴 함수(`checkExtPattern`)의 반환값과 `checkExtPolicy` 의 반환값은 숫자가 다르다. 변환은 패턴 -3→-6, -4→-9, -5→-7, -6→-8 이다.
확장자 허용 목록(`checkExtProfile`) 실패만 -4 다. Go 로 옮길 때 두 계층의 코드를 **다른 타입**으로 두면 섞일 일이 없다.

### 6.2 압축 결과 코드와 문구(`getZipErrText`)

| 코드 | 문구 | 원인 |
|---|---|---|
| 1 | `압축 해제 성공` | |
| -999 | `사용자 취소` | 실행 플래그 꺼짐 |
| -998·-997 | `압축 실패` | 재압축·빈 압축(풀기에서는 안 나옴) |
| -996 | `압축 모듈 없음` | 7z.dll 없음(bit7z 126). Go 순수 구현이면 생기지 않는다 |
| -995 | `파일 없음` | bit7z 2 |
| -994 | `압축파일 아님` | bit7z 14(확장자는 압축인데 내용이 아님) |
| -993 | `디렉토리 없음` | |
| -992 | `압축 대상 중복` | |
| -991 | `압축 횟수 초과` | 중첩 깊이 |
| -990 | `압축 패스워드 존재` | 바깥이 잠김(bit7z 9·18·19·20·21) |
| -989 | `알 수 없는 오류` | 그 밖, 임시 폴더 못 만듦, 푼 수 ≠ 목록 수 |
| -988 | `압축 모듈 파라미터 오류` | bit7z 87 |
| -987 | `허용되지 않는 경로 포함` | 경로 탈출·링크 |
| -986 | `압축 패스워드 존재` | 안쪽이 잠김 |
| -985·-984 | `압축 해제 상한 초과` | 크기·항목 수 |
| -983 | `압축 해제 공간 부족` | 디스크 여유 |
| -1001 | `압축 비밀번호 오류` | 두 번 틀림(앱 코드) |
| -1002 | `압축 비밀번호 입력 취소` | 취소(앱 코드) |
| 그 밖 | `Unknown` | 원본 그대로 영문. Wails 판은 `알 수 없는 오류` 로 맞추는 것을 권한다 |

### 6.3 경계 상황 목록

| 상황 | 원본 결과 | Wails 판 |
|---|---|---|
| 크기가 상한과 같다 | 막힘 | 같게 |
| 상한 0 | 제한 없음 | 같게 |
| 파일 63바이트(패턴은 64바이트 덮음) | 헥스가 126자뿐 → 패턴 길이보다 짧으면 "다름" | 같게. 단 패턴 길이가 짧으면(예 8자) 맞을 수 있다 |
| 파일 1바이트 | 헥스 2자. 헤더 없는 행만 맞음 | 같게 |
| 확장자 `.TXT` 와 정책 `txt` | 같다고 봄 | 같게(대문자 맞춤) |
| `a.tar.gz` | 확장자 `GZ` | 같게 |
| `.zip`(점으로 시작) | 확장자 없음 → 압축 아님 | 같게 |
| `a.`(점으로 끝) | 확장자 없음 | 같게 |
| 다른 프로그램이 독점 잠금 | `파일 열기 실패` | Windows 는 `os.Open` 이 공유 위반으로 실패 → 같게 |
| 정책 못 받음 | 전부 `등록되지 않은 확장자` | 같게, 다만 수신 대기와 안내 추가(8절) |
| 내부 0바이트 파일 | 압축 전체 막힘(`파일 사이즈 0`) | 같게(사용자에게 놀라운 동작이지만 원본·MOSFM 대조 전에는 바꾸지 않는다) |
| 내부 디렉터리 항목 | 세지 않음 | 같게 |
| 안쪽 압축 파일 자체 | 목록에 들어가 정책 검사(예: `.zip` 이 확장자 정책에 없으면 막힘) | 같게 |
| 큰 내부 파일(크기 상한 초과) | 잘린 파일 크기로 판정 → 대개 통과 | 목록 크기로 판정 권장(결정 필요) |
| macOS 압축의 `__MACOSX` 항목 | macOS 빌드만 파일 수에서 뺀다 | 플랫폼 상관없이 같은 규칙을 권한다(추정: 원본이 macOS 만 뺀 것은 macOS 에서 풀린 항목 수가 달라졌기 때문). 순수 Go 구현은 플랫폼 차이가 없다 |
| 비밀번호 창이 떠 있는데 로그아웃 | 창 닫힘, 결과 안 씀, 다음 로그인에 창 안 뜸 | 같게 |
| 같은 이름 파일 둘 | 결과가 앞 행에 붙을 수 있음(POL-052) | 행 ID 로 해결 |
| 128자 넘는 비밀번호 | 맞혀도 업로드 요청에 안 실음 | 같게 |
| 자동 업로드의 암호 압축 | 묻지 않고 막음, 로그 `[Auto]암호 압축 >>> 자동 업로드라 비밀번호를 묻지 않고 막는다:` | 같게 |
| `__CLIP__` 이 든 이름 | 업로드에서 뺌(로그만) | 같게 |
| 검사 중 파일이 바뀜 | 크기·수정 시각을 남겨 전송 때 대조(03) | 03 에서 |

### 6.4 서버파일 [상태] 문구(`filemoverpolicy.cpp getState(virus, priv, aprv)`)

판정 순서는 백신 → 개인정보 → 결재. 앞 단계가 "통과" 일 때만 다음을 본다. 통과 아닌 문구의 상태 코드는 -1, 모두 통과면 `다운로드 가능`(코드 1).

| 단계 | 값 → 문구 |
|---|---|
| 백신 `virus_flag` | -1 `백신체크 실패`, 0 `바이러스 체크 중`, 2 `바이러스 발견`, 3 `APT 탐지중`, 5 `다운로드 불가`, **6 `OLE 검출 파일 차단`**, 21 `CDR 있음`, 22 `암호설정문서파일 차단`, 1·4 통과, 그 밖 `다운로드 불가` |
| 개인정보 `priv_state` | -1 `개인정보 검색 실패`, 0 `개인정보 검색 중`, 1·3 통과, 2 `개인정보 검색됨`(삼성카드 빌드만 통과), 그 밖 `다운로드 불가` |
| 결재 `aprv_state` | 0 `결재 대기중`, 1·3 통과, 2 `반려됨`, 4 `결재요청 가능`, 그 밖 `다운로드 불가` |

- 값 6 은 원본 명세 표에 빠져 있다. 코드가 기준이다.
- 결재 4 는 서버 스키마 주석에 없지만 실제로 가장 많은 값이다(원본 `src/README.md`).
- 삼성카드 빌드 분기는 고객사 전용 빌드 옵션이다. Wails 판에서는 제외(추정: 지금 고객사가 없다).

---

## 7. 로컬 처리 알고리즘과 Go 패키지 후보

### 7.1 파일 하나의 판정(`checkExtPolicy` 를 옮긴 모양)

```go
// 반환: 결과 코드(6.1절), 불일치 기록용 헥스
func CheckFile(ctx context.Context, p *PolicySet, path string, sizeOverride int64) (code int, hexHead string) {
    if ctx.Err() != nil { return -10, "" }
    st, err := os.Stat(path); if err != nil { return -2, "" }
    size := st.Size(); if sizeOverride > 0 { size = sizeOverride }
    if size == 0 { return -1, "" }
    f, err := os.Open(path); if err != nil { return -2, "" }
    buf := make([]byte, 256); n, _ := io.ReadFull(f, buf); f.Close()
    if n <= 0 { return -2, "" }
    hexAll := strings.ToUpper(hex.EncodeToString(buf[:n]))  // %02X 와 같다
    head := hexAll[:min(len(hexAll), 128)]                    // 앞 64바이트
    ext := FileExt(filepath.Base(path))
    if p.IntOr("no_ext_no_permit", 0) != 0 && ext == "" { return -3, head }
    if !p.CheckSize(size) { return -11, head }
    switch p.CheckExtPattern(ext, head) { case 0: case -3: return -6, head; case -4: return -9, head
        case -5: return -7, head; default: return -8, head }   // -6 과 알 수 없는 값
    if !p.CheckExtProfile(ext) { return -4, head }
    return 0, head
}
```

- 읽는 양은 256바이트지만 대조는 앞 128자만 쓴다. 원본이 256바이트를 읽는 이유는 찾지 못했다(추정: 예전 MIME 단계 흔적). Go 는 64바이트만
  읽어도 결과가 같다 ― 다만 `n<=0` 판정은 그대로 둔다.
- 실패 시 불일치 기록에 `{fname, ext, head, desc1}` 를 쌓는다(-10 제외).

### 7.2 헤더 패턴 표 담기(`setPattData`)

```
행마다:
  ext := ToUpper(extName)
  if hFlag && extPatt != "":
     v := extPatt 의 a-f, x 를 대문자로
     if v 에 [0-9A-FX] 밖 글자가 있으면: 경고 "헤더패턴 행을 버린다 …" 후 이 행 전체를 버린다(byName 에도 넣지 않는다)
     byPatt = append(byPatt, {patt: v, ext, allow: aFlag})
     byName[ext] = append(byName[ext], {patt: v, allow: aFlag})
  else:
     byName[ext] = append(byName[ext], {patt: "", allow: aFlag})   // 헤더 없는 행
```

패턴 대조(`comparePatt`): `len(head) < len(patt)` 이면 다름, 아니면 앞에서부터 `patt[i]=='X' || patt[i]==head[i]`.
`isAllX(patt)`: 비어 있지 않고 전부 `X`.

### 7.3 세 판정 방식(`extpolicy.cpp` 를 그대로 옮김)

반환은 패턴 계층 코드(0, -3, -4, -5, -6). `ext` 는 대문자.

**black**(기본)

```
rows, ok := byName[ext]; if !ok → -3
anyAllow := false
for r in rows where r.allow:
    anyAllow = true
    if r.patt != "":  if match(r.patt, head) → 0
    else:             for q in byPatt: if match(q.patt, head) && !isAllX(q.patt) → -5
                      → 0
if !anyAllow → -4
→ -6
```

추정: 헤더 없는 허용 행을 만나면 그 자리에서 결론(-5 또는 0)을 낸다 ― 원본 루프가 그 분기에서 바로 반환한다고 읽었다. Go 로 옮긴 뒤
원본 `tst_FileMoverPolicy` 의 `blackMode_*`·`headerPattern_compare` 표를 그대로 돌려 확인한다.

**white**

```
hasSame, renamed := false, false
for q in byPatt where match(q.patt, head):
    if q.ext == ext && !q.allow → -4
    if q.ext == ext → hasSame = true
    else if !isAllX(q.patt) → renamed = true
for r in byName[ext]: if r.patt == "" && r.allow → hasSame = true
if hasSame → 0; if renamed → -5; → 0
```

**gray**

```
notAllow, otherExt := false, false
for q in byPatt where match(q.patt, head):
    if q.ext == ext { if q.allow → 0; notAllow = true }
    else if !isAllX(q.patt) → otherExt = true
if notAllow → -4; if otherExt → -5; → 0
```

확장자 허용 목록(`checkExtProfile`): `profile[ext]` 의 행 중 `allow_upload==0` 이 하나라도 있으면 거짓, 목록에 없거나 전부 허용이면 참.

크기(`checkSize`): `limit==0 || limit > size`. 클립보드는 `fm_clip_size` 로 같은 식.

### 7.4 압축 내부 검사(`checkZipFile` + `ZipExtractor::ExtractFile`)

```
CheckArchive(ctx, p, path, password, isHumanSingle):
  if !isArchiveExt(ext) → 성공(내부 검사 없음)
  if p.Has("fm_chk_insidezip") && p.Int("fm_chk_insidezip") == 0 → 성공
  maxDepth := depthFromPolicy(p)            // 없으면 2, 1..5 로 자르고 경고
  limits := 새 상한 셈(10GiB, 100000, 여유 1GiB)   // 검사 하나에 하나
  if password != "": TestPassword(전체를 끝까지 풀어 무결성 확인, 디스크에 안 씀) 실패면 그 코드
  files, code := extract(ctx, path, password, depth=0)
  return code, files

extract(depth):
  if depth >= maxDepth → -991
  열기 실패 → 오류 코드(7.5절). depth>0 에서 비밀번호 오류면 -986
  항목 목록을 먼저 훑어 위험 항목(POL-030)이 하나라도 있으면 → -987, 아무것도 안 풂
  상한 셈에 항목 수·목록 크기 합 반영 → 넘으면 -984/-985, 디스크 여유 부족 -983
  항목마다(ctx 취소면 -999):
     디렉터리면 건너뜀(수에 안 셈), macOS 메타 `__MACOSX/` 처리 규칙 적용
     if isArchiveExt(항목 이름): 임시 파일로 끝까지 풀어(쓴 양 상한 셈) → extract(depth+1) → 실패면 그 코드
                                  그리고 그 항목 자신도 결과 목록에 넣음
     else: 앞 256바이트만 읽음 → 결과 목록에 {이름, 목록 크기, 헥스}
  depth==0 에서: 결과 수 ≠ 목록 파일 수 → -989
```

- 원본은 내부 파일을 임시 폴더에 앞부분만 풀고 그 파일로 `checkExtPolicy` 를 다시 한다. Go 판은 같은 판정 함수를 "경로" 대신
  "이름 + 크기 + 앞 바이트" 를 받게 나누면 디스크를 거치지 않는다(`CheckFile` 의 앞부분을 `CheckEntry(name, size, head)` 로 분리).
- **GZ·XZ 는 단일 스트림**이다. 원본은 7-Zip 이 `a.tar.gz` 를 풀어 `a.tar` 하나를 만들고, 그것이 `TAR` 확장자라 다시 깊이 +1 로 푼다.
  그래서 `tar.gz` 하나가 깊이 2를 쓰고, 기본값 2 에서는 zip 안의 `tar.gz` 가 `압축 횟수 초과` 가 된다(추정: 코드 흐름으로 읽은 것, 실측 안 함).
  Go 판은 gz/xz 를 풀어 나온 이름(헤더의 원래 이름, 없으면 확장자를 뗀 이름)을 다음 단계로 넘기는 방식으로 같은 깊이 셈을 재현한다.
- 원본은 TAR 를 확장자가 아니라 내용으로도 판정한다(`IsTar`: Tar 로 열어 보고 성공이거나 오류 9·20·21 이면 Tar). Go 판은 확장자가
  `TAR` 이면 `archive/tar`, 그 밖은 내용 앞 바이트(매직)로 형식을 고른다.
- 원본 `progress` 콜백의 "1% 넘으면 멈춤" 은 앞부분만 쓰려는 장치다. 순수 Go 는 스트림에서 256바이트만 읽고 닫으면 되므로 필요 없다.
  단 **ZIP 은 항목마다 따로 읽을 수 있지만, 7z 의 solid 블록과 tar.gz 는 앞 항목을 지나야 뒤 항목에 닿는다** ― 건너뛰는 동안 푼 양도
  상한 셈(바이트)에 넣어야 압축 폭탄을 막는다.

### 7.5 형식별 Go 구현 후보

| 형식 | 원본 | Go 후보 | 암호 | 비고 |
|---|---|---|---|---|
| ZIP | 7z.dll | 표준 `archive/zip` | 없음 → `github.com/yeka/zip`(ZipCrypto·WinZip AES 읽기) 또는 `github.com/alexmullins/zip`(AES 중심, 추정: ZipCrypto 지원 여부 확인 필요) | 표준은 Store·Deflate 만. `zip.RegisterDecompressor` 로 방식 추가 가능 |
| ZIPX | 7z.dll | `archive/zip` + 방식 등록: 12 bzip2(표준 `compress/bzip2`), 14 LZMA(`github.com/ulikunitz/xz/lzma`), 93 zstd(`github.com/klauspost/compress/zstd`), 95 xz(`ulikunitz/xz`) | 위와 같음 | **98 PPMd·97 WavPack·96 JPEG 은 순수 Go 후보를 확인하지 못했다**(이 작업에서 웹 검색 안 함). 못 푸는 방식은 -989 대신 따로 코드를 두는 것을 권한다 |
| TAR | 7z.dll | 표준 `archive/tar` | 없음 | 심볼릭·하드 링크는 `Typeflag` 로 판정(POL-030) |
| GZ | 7z.dll | 표준 `compress/gzip` | 없음 | 단일 스트림, 다중 멤버 gz 는 `Multistream(true)`(기본) |
| XZ | 7z.dll | `github.com/ulikunitz/xz` | 없음 | 단일 스트림 |
| 7Z | 7z.dll | `github.com/bodgit/sevenzip` | AES-256(데이터·헤더 암호 모두, `OpenReaderWithPassword`) | LZMA·LZMA2·PPMd·BZip2·Deflate 등. ReaderAt 필요 → 안쪽 7z 는 임시 파일 |
| LZH | 7z.dll | **순수 Go 라이브러리를 확인하지 못했다** | 없음 | 선택지: ① lh5/lh6/lh7(정적 허프만 + LZSS) 해독기를 직접 구현(추정: 400~600줄) ② 7z.dll/7z.exe 호출 ③ LZH 를 "압축 아님" 으로 바꾸기(정책 변경이라 비권장) |
| 통합 | ― | `github.com/mholt/archives`(선택) | 추정: 암호 ZIP 미지원 | zip·tar·gz·bz2·xz·zstd·7z(bodgit)·rar 를 한 인터페이스로. 형식 판별(Identify)이 쓸 만하다. 직접 조합해도 코드량이 크지 않아 필수는 아니다 |

그 밖에 필요한 것

- 디스크 여유 공간: `golang.org/x/sys/windows.GetDiskFreeSpaceEx`, macOS·Linux 는 `golang.org/x/sys/unix.Statfs`.
- 임시 폴더: 표준 `os.MkdirTemp(<사용자 데이터>/temp, "chk-*")` + `defer os.RemoveAll`.
- 폴더 훑기: 표준 `filepath.WalkDir`(숨김 포함, 심볼릭 링크는 따라가지 않음).
- 헥스: 표준 `encoding/hex` + `strings.ToUpper`.

오류 코드 대응(원본 `fromBitError` → Go)

| 원본 | Go 에서 판정할 조건(권장) |
|---|---|
| -995 파일 없음 | `os.Open` 의 `fs.ErrNotExist` |
| -994 압축파일 아님 | 형식별 열기 실패(`zip.ErrFormat`, tar 헤더 오류, gzip 헤더 오류, 7z 서명 불일치) |
| -990 비밀번호 | 암호 플래그가 있는데 비밀번호 없음, 또는 비밀번호로 풀었을 때 CRC·MAC·체크섬 불일치, 7z 헤더 암호 |
| -989 알 수 없음 | 그 밖(지원하지 않는 압축 방식 포함 ― 별도 코드 권장) |
| -996·-988 | 순수 Go 면 생기지 않는다 |

### 7.6 크기 표기(`getFileSizeToText`)와 확장자(`getFileExtension`)

```
size < 1        → "0 Byte"
size > 1 GiB    → "%.2f GB"
size > 1 MiB    → "%.2f MB"
size > 1 KiB    → "%.1f KB"
그 밖           → "%d byte"
(1024 진법, 경계는 '>' ― 정확히 1024 는 "1024 byte")

FileExt(name): i := LastIndex(name, "."); if i <= 0 || i == len(name)-1 → ""; return ToUpper(name[i+1:])
```

### 7.7 암호 압축 판정과 대기(`ZipPasswordGate.h`)

```
action(disable, use, isAuto):
  disable == 1 → Pass        // 로그 "암호 압축 >>> 안을 검사하지 않고 통과(fm_disable_zip_pass=1):"
  use != 1     → Block       // 로그 "…묻지 않고 막는다(fm_use_Zip_pwd != 1):"  → -990
  isAuto       → BlockAuto   // 로그 "[Auto]암호 압축 >>> 자동 업로드라 비밀번호를 묻지 않고 막는다:" → -990
  else         → Ask

Ask (최대 2회):
  화면에 ZipPasswordAsk{gen, name, retry: i>0}
  select { pw := <-answer; <-ctx.Done() → -1002 }
  pw == "" → -1002
  앞 임시 폴더 지움, CheckArchive(password=pw)
  결과 != -990 → 그 결과(성공이고 사람이 올린 낱개 파일이면 passwords[path]=pw)
두 번 다 -990 → -1001
로그에는 PW[*** len=N] 만
```

Pass 는 바깥이 잠긴 압축을 내부 검사 없이 통과시킨다는 뜻이다(결과 코드 0 취급, 위 표 행 문구는 원본에서 정상 계열).
추정: Pass 일 때 위 표 행 문구가 `정상 파일` 인지 `압축 해제 성공` 인지는 `verifyUploadFile` 의 분기(압축 결과 >0 이고 내부 파일 있음)로 보아
`정상 파일` 로 남는다.

### 7.8 검사 흐름 전체(`checkUploadFiles` → `fileCheckReply`)

```
uploadFiles(paths, isAuto):
  len(paths) > fm_file_count(>0) → 창 또는 [Auto] 로그, 끝
  업로드 권한 없음 → 알림, 끝 (03)
  목록 주인 판정(03), "__CLIP__" 이름 제외(로그)
  checkUploadFiles

checkUploadFiles:
  fails, blocks, folderExcludes, passwords 비움; gen++ ; isRun=true
  화면: FileCheckOpen{gen}
  go func():
    policy := policies.Load()                    // 한 벌 고정
    for item in items:
       folder → verifyDir → (errCount, passed, excluded)
           errCount>0 && passed>0 && 사람 → folderExcludes[item]=excluded; 행 "차단 N건 제외"/"내부 오류 N건"; "(제외됨)" 표시
           errCount>0 그 밖 → blocks += errCount; 행 "오류 파일 존재"/"내부 오류 N건"
           0 → "정상 폴더"
       file → 행 "정상 파일"/크기; n := verifyFile; fails+=n; blocks+=n
    화면: FileCheckResult{gen, fails, value}     // value: 자동 "AUTO", blocks==0 && folderExcludes 있음 "SKIP:<합>", 그 밖 ""
    fails==0 → 검사값 미리 계산 시작(03)

fileCheckReply(gen, reject):
  지난 세대·결과 전 답 → 버림(로그)
  reject || blocks>0 → isRun=false, 미리 계산 버림, gen++, sendMismatch, 목록 놓기, 로그 "사용자에 의해 파일 업로드가 취소되었습니다."
  진행 → sendMismatch, passwords 를 각 파일 전송 정보 zip_pswd 에, 크기·mtime 기준값 옮김, 업로드 시작(03)
```

---

## 8. 원본보다 나아질 수 있는 곳

동작(무엇이 막히고 통과하는가)을 바꾸는 항목은 **[동작 변경]** 으로 표시했다. 원본 지침상 동작 변경은 MOSFM(이전 세대) 대조 뒤에 정한다.
나머지는 같은 결과를 더 빠르거나 안전하게 내는 것이다.

| 항목 | 원본 | Wails 판 제안 |
|---|---|---|
| 잎 파일 디스크 쓰기 | 내부 파일마다 임시 폴더에 앞부분을 풀어 쓴 뒤 다시 열어 읽는다 | 스트림에서 앞 64~256바이트만 읽고 판정. 안쪽 압축만 임시 파일. 디스크 쓰기와 백신 실시간 검사 부하가 크게 준다 |
| 병렬 검사 | 파일을 하나씩, 100건마다 50ms 쉼 | 낱개 파일 헤더 판정은 `errgroup` 으로 동시 4~8개(디스크 종류 따라). 화면 행 순서는 입력 순서로 고정. 압축 내부 검사는 파일 단위로 병렬, 상한 셈은 검사(압축 하나)마다 |
| 행 찾기 | 파일 이름으로 찾아 같은 이름이면 앞 행에 붙음(POL-052) | 행 ID 로 갱신 → 보류 항목이 저절로 풀린다 |
| 압축 행 색 | 내부가 걸려도 위 표 압축 행이 초록 `압축 해제 성공` | 내부 오류가 있으면 빨강(문구는 그대로 두되 색은 `ok` 로). **[화면 변경]** 사용자 혼동을 줄인다 |
| 불일치 기록 | 5건만 보내고 나머지는 메모리에 남다가 앱 종료 때 유실, 버퍼 넘침·EUC-KR 깨짐(추정) | 답 때 쌓인 것을 5건씩 나눠 **모두** 보낸다(서버가 한 요청 5건 제한을 둔다고 보고 유지). 실패하면 다음 답 때 다시. `fname` 은 압축 안이면 `<압축 경로>/<내부 경로>` 로(임시 폴더 경로 대신). **[서버 기록 변경]** |
| 내부 파일 크기 판정 | 잘린 파일 크기로 → 상한 넘는 내부 파일이 통과(POL-032) | 목록(헤더)에 적힌 크기로 판정. **[동작 변경]** MOSFM 대조 필요. 거짓 크기는 상한 셈이 막는다 |
| 폴더 압축본 크기 | 다시 비교하지 않음(POL-046) | 압축 뒤 크기를 `fm_file_size` 와 비교. **[동작 변경]** 서버가 막는지 먼저 확인 |
| `fm_chk_header` | 읽지 않고 늘 헤더 검사(POL-013) | MOSFM 처럼 0 이면 헤더 단계를 건너뛴다. **[동작 변경]** |
| 정책 미수신 | 응답 전에 업로드하면 전부 `등록되지 않은 확장자` | 정책 3종이 다 올 때까지 최대 수 초 기다린 뒤 검사. 실패면 "정책을 받지 못했다" 안내와 [정책 받아오기] 단추. 막는 결과는 같다 |
| 결과 코드 계층 | 패턴 계층 코드와 정책 계층 코드가 둘 다 정수라 섞이기 쉽다 | Go 에서 타입을 나눈다(`patternResult`, `PolicyCode`) |
| 정책 한 벌 교체 | 복사 후 교체 + 잠금 | `atomic.Pointer[PolicySet]`, 불변 구조체 |
| 비밀번호 대기 | 200ms 폴링 | 채널 + `context` 로 즉시 깨어남 |
| 7-Zip 의존 | 7z.dll·7z-mac.so·7z-linux.so(맥·리눅스는 옛 p7zip 갈래) | 순수 Go 로 플랫폼 차이와 엔진 배포·판 관리가 사라진다(LZH·PPMd 만 예외) |
| 헤더 패턴 이름 잘림 | 확장자 7바이트·패턴 127자로 잘림 | 자르지 않는다. 잘린 이름 행은 원본에서 어떤 파일과도 안 맞는 죽은 행이었다(추정) |
| 문구 `Unknown` | 압축 문구 표의 기본값이 영문 | `알 수 없는 오류` 로 통일 |
| 진행 표시 | `검사가 진행중입니다.` 문구만 | 파일 수 기준 진행 막대(`n / 전체`)와 지금 검사 중인 이름. 큰 압축의 비밀번호 시험처럼 오래 걸리는 단계에 특히 |
| 검출 조회 | 우클릭 메뉴만 | 같은 메뉴 + 상태 칸을 누르면 해당 조회(디자인 개선, 12 영역과 협의) |

---

## 9. 실 서버 검증 방법

비밀번호·키는 이 문서에 적지 않는다. 계정과 접속 정보는 원본 `docs/agents/서버_참조.md`·`tools/e2e/README.md` 와 `tools\.secrets\` 의 절차를 따른다.

### 9.1 준비

- 시험 서버: 업무망 .157(DB 조회 가능), 인터넷망 .158(mysql 없음 ― DB 확인은 업무망에서).
- 계정: 자리 N 의 `test<N>_1`(일반·요청자), `test<N>_2`(결재자), `test<N>_3`(대리결재자). 잠기면 원본 `srv_settings.py unlock` 으로 푼다.
- autoqa 의 시험 계정 5개와 권한 프로파일 `E2E-QA` 는 autoqa 실행이 쓰는 동안 같이 쓰지 않는다(원본 `AGENTS.md`).
- Wails 판 실행: `$env:SNB_APP_DIR = 'E:\git\snBv6FM\build\release'` 로 원본 개발 빌드의 `fmenv.ini`·`root_ca.pem` 을 쓴다.
  단 원본 `build-release.cmd` 와 autoqa 실행 전 단계가 서버의 클라이언트 무결성 등록을 서로 덮으므로, 실 서버 시험 시각을 겹치지 않는다.

### 9.2 서버 정책 바꾸기

- 원본 `srv_settings.py`(SBM 요청)로 '기본설정' 권한 프로파일에 키를 걸고 끝나면 되돌린다. 확장자 패턴 행은 `fm_ext:<확장자>`=`allow`|`deny`.
- 바꾼 뒤 **다시 로그인**해야 앱이 읽는다(트레이 [정책 받아오기] 로 3005·3015 는 다시 받지만 크기·개수는 로그인 응답이다).
- 헤더 패턴이 실제로 어떻게 내려왔는지는 서버 로그 `GetExtPattern: count=…`(원본 `snb probe log`)와 Wails 판의 `응답 건수 … 저장된 행 …` 로그를
  나란히 본다. SR032 로 차단이 허용으로 내려오는 행이 있으니, 차단 시험은 그 확장자의 첫 행으로 만든다.

### 9.3 원본과 나란히 비교(같은 서버·같은 계정·같은 파일)

원본 E2E 케이스를 기준선으로 쓴다: `policy`, `policy-size`, `policy-zip`, `zip-path-escape`, `zip-password`, `folder`, `policy-refresh`.

```powershell
.\tools\run-e2e.ps1 -Launch -Desktop -Auto -Only policy,policy-size,policy-zip,zip-path-escape,zip-password,folder
```

원본 결과 JSON 의 파일별 결과 문구와, Wails 판이 같은 입력에 낸 결과 문구를 표로 대조한다. 원본 E2E 대부분(`PL-01`~`PL-05`)은 실패 수만 보므로
**문구 대조는 Wails 판 쪽 시험이 따로 해야 한다.**

### 9.4 Wails 판 시험 계획

| 시험 | 방법 | 통과 기준 |
|---|---|---|
| 판정 단위 시험 | 원본 `tests/tst_FileMoverPolicy.cpp` 의 표(`blackMode_*`·`whiteMode_*`·`grayMode_*`·`headerPattern_compare`·`checkSize_*`·`checkExtProfile_*`·`getState_*`)를 Go 표 시험으로 옮김 | `go test .` 통과, 원본 기대값과 같음 |
| 압축 단위 시험 | 원본 `tests/tst_ZipExtractor.cpp` 의 고정 자료(정상 항목·`..`·절대 경로·링크·잠긴 zip/7z·헤더 암호 7z·중첩 깊이·상한)를 재현 | 코드가 원본과 같음(-987, -990, -986, -991, -985/-984) |
| 정책 수신 | 실 서버(`live_test.go`, `SNB_LIVE_ID` 있을 때만)에서 3005·3015 를 받아 건수 로그 | 서버 로그 `GetExtPattern: count=` 와 응답 건수가 같다 |
| 차단 확장자 업로드 | 프로파일에 `fm_ext:txt=deny` → `.txt` 업로드 | 결과 `허용되지 않은 확장자 헤더패턴` 또는 `허용되지 않은 확장자`(정책 행 종류에 따라), 서버 목록에 파일이 생기지 않음 |
| 크기 상한 | `fm_file_size` 를 작게 → 상한과 같은 크기·1바이트 큰 파일 | 둘 다 `전송 가능한 파일 크기 초과` |
| 압축 내부 | 차단 확장자를 넣은 zip | 위 표 `압축 해제 성공`·`내부 파일 N개, 오류 M건`, 아래 표에 걸린 파일 |
| 경로 탈출 | `../x.txt` 항목 zip | `허용되지 않는 경로 포함`, 임시 폴더 밖에 파일 없음 |
| 암호 압축 | `fm_use_Zip_pwd=1`·`fm_disable_zip_pass=0` | 창이 뜨고, 맞으면 진행·서버 `fm_approve.zip_pswd` 에 값(업무망 DB), 두 번 틀리면 `압축 비밀번호 오류` |
| 불일치 기록 | 차단 파일 6개 이상 올리고 취소 | 서버 불일치 기록 표에 모두 쌓인다(추정: 표 이름은 서버 저장소 `snBv6Srv` 에서 읽기로 확인 필요). 원본에는 이 판정 E2E 가 없다 |
| 상태 문구 | 서버 검사 결과가 다양한 파일 목록 | 6.4절 문구와 같음(값 6 포함) |
| 화면 | 같은 입력을 원본·Wails 판에서 캡처 | 단추 글자·문구·색이 5절과 같음. E2E 통과만으로 화면 정확성을 단정하지 않는다 |

### 9.5 로그로 판정할 줄(원본과 같은 글자로 남길 것)

- `FILE_CHECK_RESULT received >>> ret: N` (원본 QML 로그, E2E 가 읽는다)
- `압축파일 경로 검사 거절(허용되지 않는 경로 또는 링크) >>> 압축파일: …, 항목: …`
- `암호 압축 >>> …`, `[Auto]암호 압축 >>> 자동 업로드라 비밀번호를 묻지 않고 막는다:`
- `압축 풀기 중단 >>> 사유[크기 상한|항목 수 상한|디스크 공간], 경로[업로드 검사], …`
- `압축 중첩 깊이 정책 값을 범위로 자른다 >>> fm_extract_zipdepth[%1] → %2`
- `헤더패턴 행을 버린다 >>> 확장자: %1, 패턴: %2, 글자: '%3'`
- `<폴더> 차단 파일 N건 제외`
- `파일검사 회신 >>> 지난 검사의 답이라 버린다 …`
- `사용자에 의해 파일 업로드가 취소되었습니다.`
- `권한 조회 >>> fileMaxSize: …`

Wails 판의 로그는 지금 표준 오류로만 나간다(README). 위 줄을 E2E 가 읽으려면 원본처럼 파일 로그가 먼저 있어야 한다(로그 영역과 얽힘).
