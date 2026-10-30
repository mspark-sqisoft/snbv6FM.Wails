import type ko from './design-airlock.ko'

const en: Record<keyof typeof ko, string> = {
  here: 'Here',
  sealed: 'Both doors sealed',
  passing: 'Passing {n}',
  queued: 'Queued {n}',
  failed: 'Failed {n}',
}

export default en
