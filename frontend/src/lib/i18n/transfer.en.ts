import type ko from './transfer.ko'

const en: Record<keyof typeof ko, string> = {
  title: 'Transfers',
  empty: 'No data is being transferred.',
  auto: '(auto)',

  'col.name': 'File',
  'col.state': 'Status',
  'col.size': 'Size',
  'col.dir': 'Type',
  'col.speed': 'Speed / time left',
  'col.action': 'Cancel',
  'dir.upload': 'Upload',
  'dir.download': 'Download',

  'state.wait': 'Waiting',
  'state.waitHash': 'Waiting (computing checksum)',
  'state.waitSerial': 'Waiting (large files one at a time)',
  'state.waitDownload': 'Waiting to download',
  'state.prepare': 'Preparing transfer...',
  'state.commit': 'Encrypting file...',
  'state.zip': 'Compressing {p}%',
  'state.extract': 'Extracting {p}%',
  'state.done': 'Done',
  'state.failed': 'Transfer failed',
  'state.cancelled': 'Cancelled',

  overall: 'Overall {p}%',
  summary: 'Active {a} · Waiting {p} · Done {d} · Failed {f}',
  clear: 'Clear list',
  clearHint: 'Clears the list only. Running transfers continue.',
  cancelOne: 'Cancel this transfer. A partly received file is deleted.',
  cancelAll: 'Cancel all',
  cancelAllConfirm: 'Cancel every waiting or running transfer.',

  'conflict.title': 'Duplicate file',
  'conflict.text': 'A file with the same name already exists. What should be done?',
  'conflict.folderHint': 'Overwriting a folder merges the files inside it.',
  'conflict.count': '{i} / {n}',
  'conflict.all': 'Apply to all',
  'conflict.overwrite': 'Overwrite',
  'conflict.rename': 'Rename',
  'conflict.cancel': 'Cancel',
}

export default en
