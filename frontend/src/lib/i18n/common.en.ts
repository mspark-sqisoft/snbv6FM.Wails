import type ko from './common.ko'

// 한국어 파일의 키를 모두 가져야 컴파일된다(빠진 번역을 svelte-check 가 잡는다).
const en: Record<keyof typeof ko, string> = {
  ok: 'OK',
  cancel: 'Cancel',
  close: 'Close',
  retry: 'Retry',
  refresh: 'Refresh',
  loading: 'Loading…',
  empty: 'Nothing to show.',
  selectAll: 'Select all',
  resize: 'Resize',
  collapse: 'Collapse',
  expand: 'Expand',
  showPassword: 'Show password',
  hidePassword: 'Hide password',
  capsLock: 'Caps Lock is on.',
  selected: '{n} selected',
  total: '{n} total',

  'app.name': 'snB App',

  'menu.files': 'File Transfer',
  'menu.request': 'Request Approval',
  'menu.approve': 'Approvals',
  'menu.proxy': 'Proxy Approver',
  'menu.transHistory': 'Transfer History',
  'menu.approvalHistory': 'Approval History',
  'menu.policy': 'Policy',
  'menu.log': 'Log',
  'menu.settings': 'Settings',

  'shell.menu': 'Menu',
  'shell.collapseMenu': 'Collapse menu',
  'shell.expandMenu': 'Expand menu',
  'shell.logout': 'Log out',
  'shell.theme': 'Theme',
  'shell.language': 'Language',
  'shell.userInfo': 'User information',
  'shell.direction': 'Direction',
  'shell.changePassword': 'Change password',
  'shell.logoutTitle': 'Choose how to log out',
  'shell.logoutGroup': 'Action',
  'shell.logoutDontAsk': 'Do not show again (can be changed in Settings)',
  'shell.policyReloaded': 'Policy reloaded.',
  'shell.policyReloadFailed': 'Could not reload the policy.',

  'logoutType.0': 'Log out',
  'logoutType.1': 'Exit program',
  'logoutType.2': 'Minimize to tray',

  'theme.system': 'Use system setting',
  'theme.light': 'Light theme',
  'theme.dark': 'Dark theme',

  'lang.ko': '한국어',
  'lang.en': 'English',

  'net.business': 'Business',
  'net.internet': 'Internet',

  'role.approver': 'Approver',
  'role.proxy': 'Proxy approver',
  'role.user': 'User',

  'placeholder.title': 'Coming soon',
  'placeholder.desc': 'This screen has not been ported yet.',

  'dialog.info': 'Information',
  'session.timeoutTitle': 'Auto logout',
  'session.timeoutText': 'You were logged out after {min} minutes of inactivity.',
  'session.disconnectedTitle': 'Disconnected',
  'session.disconnectedText': 'The connection to the server was lost.\nPlease log in again.',

  'error.notImplemented': 'This feature is not supported yet.',
  'error.notLoggedIn': 'You are not logged in. Please log in again.',
  'error.connect': 'Cannot connect to the server.',
  'error.timeout': 'The server took too long to respond.',
  'error.unknown': 'An unknown error occurred.',
  'error.settingsSave': 'Could not save settings.',
}

export default en
