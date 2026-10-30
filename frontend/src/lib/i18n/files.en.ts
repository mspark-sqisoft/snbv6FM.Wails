import type ko from './files.ko'

const en: Record<keyof typeof ko, string> = {
  upload: 'Upload file',
  uploadFolder: 'Upload folder',
  download: 'Download file',
  request: 'Request approval',
  dropHint: 'Drop files or folders here to upload them.',
  splitHint: 'Resize explorer',
  delete: 'Delete',
  explorer: 'Explorer',
  explorerOpen: 'Open file explorer',
  explorerClose: 'Close file explorer',
  filter: 'Filter by file name',

  'col.no': ' # ',
  'col.name': 'File name',
  'col.state': 'Status',
  'col.size': 'Size',
  'col.uploadTime': 'Uploaded',
  'col.course': 'Direction',
  'col.reqCause': 'Request reason',
  'col.approver': 'Approver',
  'col.aprvCause': 'Approval reason',
  'col.isDir': 'Folder',
  'col.serverName': 'Server file name',
  folder: 'Folder',

  empty: 'No data found.',
  emptyHint: 'Use the Upload file button to send files from this PC.',
  noMatch: 'No files match the filter.',
  table: 'Server file list',
  menu: 'Server file menu',

  'menu.download': 'Download',
  'menu.delete': 'Delete',
  'menu.priv': 'View personal data detection',
  'menu.virus': 'View virus detection',
  'menu.apt': 'View APT detection',

  'detect.0': 'Personal data detection result',
  'detect.1': 'APT detection result',
  'detect.2': 'Virus detection result',
  'detect.one': 'You can view only one file at a time.',

  'delete.none': 'Select files to delete.',
  'delete.title': 'Delete server files',
  'delete.confirm': 'Delete the {n} selected files from the server.',
  'delete.done': 'Deleted {n} files.',
  'delete.failed': 'Could not delete {n} files.',

  'download.none': 'Select files that can be downloaded.\nCheck whether they are awaiting approval or are clipboard items.',
  'block.1': '{name}: clipboard items cannot be downloaded as files.',
  'block.2': '{name}: cannot be downloaded on the same network.',
  'block.3': '{name}: virus scan has not finished, so it cannot be downloaded.',
  'block.4': '{name}: not in a downloadable state.',
  'blocked.head': 'The following files were excluded because they cannot be downloaded.',
  'blocked.1': 'Clipboard items ({n})',
  'blocked.2': 'Same network, not downloadable ({n})',
  'blocked.3': 'Virus scan not finished ({n})',
  'blocked.4': 'Not downloadable ({n})',

}

export default en
