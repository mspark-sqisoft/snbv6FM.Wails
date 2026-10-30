import type ko from './design-waybill.ko'

const en: Record<keyof typeof ko, string> = {
  from: 'From',
  to: 'To',
  owner: 'Handler',
  cargo: 'Cargo',
  clock: 'Now',
  accepting: 'Accepting',
}

export default en
