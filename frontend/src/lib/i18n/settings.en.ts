import type ko from './settings.ko'

const en: Record<keyof typeof ko, string> = {
  title: 'Settings',
  subtitle: 'Changes are saved immediately.',

  'card.basic': 'General',
  'card.logout': 'Logout',
  'card.startFolder': 'Start folder',
  'card.autoUpload': 'Auto upload',
  'card.autoDownload': 'Auto download',
  'card.hotkey': 'Clipboard shortcuts',
  'card.about': 'About',

  theme: 'Theme',
  layout: 'Layout',
  'layout.0': 'Left menu + top toolbar',
  'layout.2': 'Top tabs (no menu)',
  language: 'Language',
  useLogFile: 'Save log',
  useLogFileHint: 'When on, logs are written to the log folder in the user data folder; when off, that folder is deleted.',

  logoutType: 'On logout',
  logoutConfirm: 'Ask every time I log out',
  closeToTray: 'Minimize to tray when the window is closed while logged in',

  startFolder: 'Start folder',
  startDesktop: 'Desktop ({path})',
  startHome: 'Home ({path})',
  startRoot: 'Root ({path})',
  pickFolder: 'Choose folder',
  clearFolder: 'Use default',

  network: 'Network',
  autoUploadUse: 'Use auto upload',
  uploadPeriod: 'Upload interval (min)',
  uploadFolder: 'Upload folder',
  autoDownloadUse: 'Use auto download',
  downloadFolder: 'Folder',
  folderUnset: 'Choose a folder.',
  folderNeeded: 'Choose a folder before turning this on.',
  autoHint: 'Auto transfer runs while you are logged in to this network.',

  hotkeyUse: 'Shortcuts',
  'hotkeyUse.on': 'Always on',
  'hotkeyUse.off': 'Off',
  hotkeyCopy: 'Send clipboard',
  hotkeyPaste: 'Receive clipboard',
  hotkeyChar: 'Key',
  hotkeyCharBad: 'Enter a single letter or digit.',
  hotkeyApplyFailed: 'Could not register the shortcut.',

  version: 'Version',
  dataDir: 'User data folder',
  appDir: 'Server config folder',

  logFileFailed: 'Could not change the log setting.',
}

export default en
