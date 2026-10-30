import type ko from './local.ko'

const en: Record<keyof typeof ko, string> = {
  title: 'Explorer',
  pathPlaceholder: 'Type a folder path and press Enter',
  path: 'Folder path',
  up: 'Parent folder',
  upload: 'Upload',
  uploadTip: 'Upload {files} selected file(s) / {dirs} folder(s).',
  uploadNone: 'Select files or folders to upload.',
  download: 'Download',
  downloadTip: 'Download the server files selected above into the current folder.',
  downloadNone: 'Select files in the server file list above to download.',
  newFolder: 'New folder',
  newFolderTip: 'Create a new folder in the current folder.',
  newFolderAt: 'Location: {path}',
  folderName: 'Folder name',
  showHidden: 'Show hidden files',

  'col.name': 'Name',
  'col.size': 'Size',
  'col.type': 'Type',
  'col.modified': 'Modified',

  'type.link': 'Shortcut',
  'type.folder': 'Folder',
  'type.ext': '{ext} file',
  'type.file': 'File',

  'tree.label': 'Explorer folder tree',
  'tree.desktop': 'Desktop',
  'tree.documents': 'Documents',
  'tree.computer': 'This PC',
  'tree.drive': 'Local disk',
  'tree.removable': 'Removable disk',
  'tree.network': 'Network drive',
  'tree.cdrom': 'CD drive',

  'list.label': 'Explorer file list',
  emptyNoFolder: 'Select a folder.',
  emptyFolder: 'This folder is empty.',
  reading: 'Reading folder...',
  status: '{n} items',
  statusSelected: '{n} items · {files} file(s) / {dirs} folder(s) selected',
  dragLabel: '{name} and {n} more',
  selectedOne: '{name} selected',

  'err.notFound': 'Folder not found.',
  'err.permission': 'You do not have permission to read this folder.',
  'err.noParent': 'Select a folder first.',
  'err.emptyName': 'Enter a folder name.',
  'err.badChars': 'A folder name cannot contain:\n\\ / : * ? " < > |',
  'err.badName': 'This folder name cannot be used.',
  'err.exists': 'A folder or file with the same name already exists.',
  'err.cannotWrite': 'Could not create the folder. Check write permission.',

  uploadStarted: 'Uploading {n} item(s).',
  downloadStarted: 'Downloading {n} file(s) to {path}.',
}

export default en
