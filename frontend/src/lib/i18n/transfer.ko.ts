// 전송 상태 서랍·파일 중복 확인 창 문구(docs/port/03 5.5, 04 5.x). 원본 문구는 그대로 옮겼다.
const ko = {
  title: '전송 상태',
  empty: '전송 중인 데이터가 없습니다.',
  auto: '(자동)',

  'col.name': '파일',
  'col.state': '상태',
  'col.size': '크기',
  'col.dir': '유형',
  'col.speed': '속도·남은 시간',
  'col.action': '취소',
  'dir.upload': '업로드',
  'dir.download': '다운로드',

  'state.wait': '대기중',
  'state.waitHash': '대기중(검사값 계산)',
  'state.waitSerial': '대기중(대용량 순차 전송)',
  'state.waitDownload': '다운로드 대기중',
  'state.prepare': '전송 준비중...',
  'state.commit': '파일 암호화 중...',
  'state.zip': '압축 중 {p}%',
  'state.extract': '압축 해제 중 {p}%',
  'state.done': '완료',
  'state.failed': '전송 실패',
  'state.cancelled': '취소됨',

  overall: '전체 {p}%',
  summary: '진행 {a} · 대기 {p} · 완료 {d} · 실패 {f}',
  clear: '목록 지우기',
  clearHint: '목록만 지웁니다. 진행 중인 전송은 계속됩니다.',
  cancelOne: '이 전송을 취소합니다. 진행 중이면 받던 파일은 지웁니다.',
  cancelAll: '모두 취소',
  cancelAllConfirm: '대기 중이거나 진행 중인 전송을 모두 취소합니다.',

  'conflict.title': '파일 중복',
  'conflict.text': '같은 이름의 파일이 있습니다. 어떻게 할까요?',
  'conflict.folderHint': '폴더는 덮어쓰면 안의 파일을 합칩니다.',
  'conflict.count': '{i} / {n}',
  'conflict.all': '모두 적용',
  'conflict.overwrite': '덮어쓰기',
  'conflict.rename': '이름 변경',
  'conflict.cancel': '취소',
}

export default ko
