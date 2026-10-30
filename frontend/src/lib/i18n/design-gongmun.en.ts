import type ko from './design-gongmun.ko'

const en: Record<keyof typeof ko, string> = {
  from: 'From',
  to: 'To',
  date: 'Date',
  flow: 'Transfers',
  box: 'Sign',
  staff: 'Draft',
  review: 'Send',
  approve: 'Final',
  count: '{n}',
  failed: 'Failed {n}',
  done: 'Done',
  approver: 'Can\nsign',
  proxy: 'Proxy\nsign',
  form: 'Drafting Form',
  network: 'Network',
  sealSlot: 'Seal',
  seal: 'Signed in',
  staffTip: 'Draft: signed-in user',
  reviewTip: 'Send: transfer queue status',
  approveTip: "Final: this user's approval authority (struck through if none)",
}

export default en
