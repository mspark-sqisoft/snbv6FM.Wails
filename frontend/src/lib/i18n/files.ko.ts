// 서버파일 화면 문구(docs/port/04 5.1·5.4·5.8). 원본 문구는 그대로 옮겼다.
const ko = {
  upload: '파일 업로드',
  uploadFolder: '폴더 업로드',
  download: '파일 다운로드',
  request: '결재요청',
  dropHint: '여기에 파일·폴더를 끌어 놓으면 업로드합니다.',
  splitHint: '탐색기 높이 조절',
  delete: '삭제',
  explorer: '탐색기',
  explorerOpen: '파일탐색기 열기',
  explorerClose: '파일탐색기 닫기',
  filter: '파일명 거르기',

  'col.no': ' # ',
  'col.name': '파일명',
  'col.state': '상태',
  'col.size': '파일크기',
  'col.uploadTime': '업로드일시',
  'col.course': '전송방향',
  'col.reqCause': '요청사유',
  'col.approver': '결재자',
  'col.aprvCause': '결재사유',
  'col.isDir': '폴더전송',
  'col.serverName': '서버파일명',
  folder: '폴더',

  empty: '조회된 데이터가 없습니다.',
  emptyHint: '파일 업로드 단추로 PC 의 파일을 올립니다.',
  noMatch: '거르기에 맞는 파일이 없습니다.',
  table: '서버파일 목록',
  menu: '서버 파일 메뉴',

  'menu.download': '다운로드',
  'menu.delete': '삭제',
  'menu.priv': '개인정보 검출 조회',
  'menu.virus': '바이러스 검출 조회',
  'menu.apt': 'APT 검출 조회',

  'detect.0': '개인정보 검출 결과',
  'detect.1': 'APT 검출 결과',
  'detect.2': '바이러스 검출 결과',
  'detect.one': '한 번에 하나의 파일만 조회 가능합니다.',

  'delete.none': '삭제할 파일을 선택해 주세요.',
  'delete.title': '서버 파일 삭제',
  'delete.confirm': '선택한 {n}개 파일을 서버에서 삭제합니다.',
  'delete.done': '{n}개 파일을 삭제했습니다.',
  'delete.failed': '{n}개 파일을 삭제하지 못했습니다.',

  'download.none': '다운로드 가능한 파일을 선택해 주세요.\n결재 대기중이거나 클립보드인지 확인해 주세요.',
  'block.1': '{name} 클립보드는 파일형태로 다운로드할 수 없습니다.',
  'block.2': '{name} 동일망에선 다운로드할 수 없습니다.',
  'block.3': '{name} 파일은 바이러스 검사가 끝나지 않아 다운로드할 수 없습니다.',
  'block.4': '{name} 파일은 다운로드 가능한 상태가 아닙니다.',
  'blocked.head': '다음 파일은 받을 수 없어 제외했습니다.',
  'blocked.1': '클립보드 형태 ({n}개)',
  'blocked.2': '동일망 다운로드 불가 ({n}개)',
  'blocked.3': '바이러스 검사가 끝나지 않음 ({n}개)',
  'blocked.4': '다운로드 가능한 상태 아님 ({n}개)',

}

export default ko
