// 안내 창 합치기를 검사한다: 같은 제목의 덮어쓸 창은 글만 바뀌고, 다른 창은 그대로다.

import assert from 'node:assert/strict'
import { mergeAlert } from '../src/lib/stores/dialogQueue.ts'

const fail = { id: 1, title: '파일 송수신 실패', text: 'a', merge: true }
const other = { id: 2, title: '정보', text: 'b', merge: true }
const plain = { id: 3, title: '파일 송수신 실패', text: 'c' } // 화면이 띄운 창은 덮어쓰지 않는다

assert.equal(mergeAlert([], '파일 송수신 실패', 'x'), null)
assert.equal(mergeAlert([plain], '파일 송수신 실패', 'x'), null)
assert.deepEqual(mergeAlert([other, fail, plain], '파일 송수신 실패', 'x'), [other, { ...fail, text: 'x' }, plain])

console.log('check-dialogs: 통과')
