import type ko from './design.ko'

const en: Record<keyof typeof ko, string> = {
  menu: 'Design',
  card: 'Design',
  fixedHint: 'This design has a fixed brightness and may fix the layout.',
  'standard.name': 'Standard',
  'standard.desc': 'Follows the light or dark theme setting.',
  'airlock.name': 'Airlock',
  'airlock.desc': 'Files pass through a double door between networks.',
  'gongmun.name': 'Official Document',
  'gongmun.desc': 'A drafting form with sign-off boxes and seals.',
  'departure.name': 'Departures',
  'departure.desc': 'Files leave like flights on the board.',
  'waybill.name': 'Waybill',
  'waybill.desc': 'Label the cargo and ship it across.',
  'metro.name': 'Metro Map',
  'metro.desc': 'Ride the stations to the other network.',
}

export default en
