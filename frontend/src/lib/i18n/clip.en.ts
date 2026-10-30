import type ko from './clip.ko'

const en: Record<keyof typeof ko, string> = {
  upload: 'Send clipboard',
  download: 'Receive clipboard',
  'download.count': '{count} clipboard item(s) to receive',
  received: 'Received clipboard content was copied.',
}

export default en
