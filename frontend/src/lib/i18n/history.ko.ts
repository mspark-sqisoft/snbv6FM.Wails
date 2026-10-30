// 전송내역·결재내역 화면 문구(docs/port/04 5.7). 원본 문구는 그대로 옮겼다.
const ko = {
  from: '시작일 선택',
  to: '종료일 선택',
  searchType: '조회 종류',
  word: '조회할 단어',
  search: '조회',
  excel: '엑셀저장',
  saveTitle: '{screen} 저장',

  'col.no': ' # ',
  'col.dept': '부서',
  'col.fileName': '파일명',
  'col.fileSize': '파일크기',
  'col.requester': '요청자',
  'col.requestTime': '요청일시',
  'col.requestCause': '요청사유',
  'col.course': '전송방향',
  'col.aprvState': '결재상태',
  'col.aprvType': '결재방식',
  'col.approver': '결재자',
  'col.aprvTime': '결재일시',
  'col.aprvCause': '결재사유',
  'col.downTime': '다운로드일시',

  actual: '{name} (실제: {act})',
  empty: '조회된 데이터가 없습니다.',
  noMatch: '조회할 단어에 맞는 내역이 없습니다.',
  badRange: '시작일이 종료일보다 늦습니다.',
  exportEmpty: '엑셀로 저장할 내역이 없습니다.',
  saved: '저장했습니다.\n{path}',
  'table.transfer': '전송내역 목록',
  'table.approval': '결재내역 목록',
  count: '{n}건',
  filtered: '{n}건 / 전체 {total}건',
}

export default ko
