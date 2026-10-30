import type ko from './history.ko'

const en: Record<keyof typeof ko, string> = {
  from: 'Start date',
  to: 'End date',
  searchType: 'Search field',
  word: 'Search word',
  search: 'Search',
  excel: 'Save to Excel',
  saveTitle: 'Save {screen}',

  'col.no': ' # ',
  'col.dept': 'Department',
  'col.fileName': 'File name',
  'col.fileSize': 'Size',
  'col.requester': 'Requester',
  'col.requestTime': 'Requested',
  'col.requestCause': 'Request reason',
  'col.course': 'Direction',
  'col.aprvState': 'Approval status',
  'col.aprvType': 'Approval type',
  'col.approver': 'Approver',
  'col.aprvTime': 'Approved',
  'col.aprvCause': 'Approval reason',
  'col.downTime': 'Downloaded',

  actual: '{name} (actual: {act})',
  empty: 'No data found.',
  noMatch: 'No history matches the search word.',
  badRange: 'The start date is later than the end date.',
  exportEmpty: 'There is no history to save to Excel.',
  saved: 'Saved.\n{path}',
  'table.transfer': 'Transfer history list',
  'table.approval': 'Approval history list',
  count: '{n}',
  filtered: '{n} of {total}',
}

export default en
