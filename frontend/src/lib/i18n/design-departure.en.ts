import type ko from './design-departure.ko'

const en: Record<keyof typeof ko, string> = {
  strip: 'Departure status',
  from: 'From',
  to: 'To',
  up: 'Uploading',
  down: 'Downloading',
  idle: 'No transfers',
  more: '+{n} more',
  active: 'Active',
  pending: 'Queued',
  done: 'Done',
  failed: 'Failed',
  role: 'Role',
  'art.title': 'Departures',
  'art.no': 'No',
  'art.net': 'Network',
  'art.route': 'Route',
  'art.state': 'State',
  'art.ready': 'Ready',
  'art.busy': 'Busy',
}

export default en
