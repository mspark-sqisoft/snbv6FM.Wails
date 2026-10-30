import type ko from './log.ko'

const en: Record<keyof typeof ko, string> = {
  table: 'Log list',
  date: 'Date',
  level: 'Level',
  'level.all': 'All levels',
  atLeast: 'This level and above',
  category: 'Category',
  'category.all': 'All categories',
  word: 'Word',
  wordPlaceholder: 'Word to find in the log',
  search: 'Search',
  'col.no': '#',
  'col.time': 'Time',
  'col.level': 'Level',
  'col.account': 'Account',
  'col.category': 'Category',
  'col.message': 'Log',
  'col.file': 'File',
  summary: '{matched} of {total} lines · {ms}ms',
  truncated: 'Too many matches: showing only the latest {n} lines.',
  empty: 'No data found.',
  emptyHint: 'Change the conditions and search again.',
  missing: 'There is no log file for that day.',
  missingOff: '[Save log] in [Settings] is off, so no log is written.',
  failed: 'Could not read the log.',
  detail: 'Log {time}',
}

export default en
