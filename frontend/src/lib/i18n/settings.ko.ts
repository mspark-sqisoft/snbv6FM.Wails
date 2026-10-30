// 설정 화면 문구(원문). 원본 SettingsScrollView.qml 문구 그대로(docs/port/12 3.4·5.8). Wails 판에서 더한 칸은 비고에 적었다.
const ko = {
  title: '설정',
  subtitle: '바꾼 값은 바로 저장됩니다.',

  'card.basic': '기본 설정',
  'card.logout': '로그아웃 처리',
  'card.startFolder': '시작 폴더 지정',
  'card.autoUpload': '자동 업로드 설정',
  'card.autoDownload': '자동 다운로드 설정',
  'card.hotkey': '클립보드 단축키 설정',
  'card.about': '프로그램 정보',

  theme: '테마',
  layout: '배치',
  'layout.0': '왼쪽 메뉴 + 상단 툴바',
  'layout.2': '상단 탭 (메뉴 없음)',
  language: '언어',
  useLogFile: '로그 저장',
  useLogFileHint: '켜면 사용자 데이터 폴더의 log 폴더에 기록하고, 끄면 그 폴더를 지웁니다.',

  logoutType: '로그아웃 처리 방식',
  logoutConfirm: '로그아웃 시에 확인창 볼 수 있게',
  closeToTray: '로그인 중 창을 닫으면 트레이로 내리기', // Wails 판에서 더함

  startFolder: '시작 폴더',
  startDesktop: '바탕 화면 ({path})',
  startHome: '홈 ({path})',
  startRoot: '루트 ({path})',
  pickFolder: '폴더 선택',
  clearFolder: '기본값으로',

  network: '망',
  autoUploadUse: '자동 업로드 사용',
  uploadPeriod: '업로드 주기(분)',
  uploadFolder: '업로드 폴더',
  autoDownloadUse: '자동 다운로드 사용',
  downloadFolder: '폴더',
  folderUnset: '폴더를 선택해 주세요.',
  folderNeeded: '폴더를 먼저 선택해야 켤 수 있습니다.',
  autoHint: '자동 전송은 이 망에 로그인해 있는 동안 동작합니다.',

  hotkeyUse: '단축키',
  'hotkeyUse.on': '항상사용',
  'hotkeyUse.off': '사용안함',
  hotkeyCopy: '클립보드 올리기',
  hotkeyPaste: '클립보드 붙이기',
  hotkeyChar: '글자',
  hotkeyCharBad: '영문자나 숫자 하나를 입력해 주세요.',
  hotkeyApplyFailed: '단축키를 등록하지 못했습니다.',

  version: '버전',
  dataDir: '사용자 데이터 폴더',
  appDir: '서버 설정 폴더',

  logFileFailed: '로그 저장 설정을 바꾸지 못했습니다.',
}

export default ko
