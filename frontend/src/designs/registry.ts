// 디자인 목록. 규격은 docs/DESIGNS.md 다. 디자인을 더하려면 여기에 한 줄, designs/<id>/design.css, 번역 키 두 개를 더한다.
// Art·Strip 은 있는 것만 저절로 붙는다(Host.svelte 와 같은 방식).

import type { Component } from 'svelte'

export type DesignId = 'standard' | 'airlock' | 'gongmun' | 'departure' | 'waybill' | 'metro'

export interface Design {
  id: DesignId
  /** 고정 밝기. 없으면(standard) 사용자의 테마 설정을 따른다 */
  scheme?: 'light' | 'dark'
  /** 강제 배치. 없으면 사용자의 배치 설정을 따른다 */
  layout?: 'side' | 'tabs'
  /** 고르기 카드의 색 미리보기(바탕·면·글자·강조) */
  swatch: [string, string, string, string]
}

export const designs: Design[] = [
  { id: 'standard', swatch: ['#f2f4f8', '#ffffff', '#121619', '#0f62fe'] },
  { id: 'airlock', scheme: 'dark', swatch: ['#0d1117', '#161b22', '#a78bfa', '#2dd4bf'] },
  { id: 'gongmun', scheme: 'light', layout: 'side', swatch: ['#f6f1e4', '#fffdf6', '#1f1b16', '#b3261e'] },
  { id: 'departure', scheme: 'dark', layout: 'tabs', swatch: ['#0b0b0c', '#17171a', '#ffb000', '#ffd400'] },
  { id: 'waybill', scheme: 'light', swatch: ['#c9ae85', '#fbf8f1', '#1a1a1a', '#f05a00'] },
  { id: 'metro', scheme: 'light', layout: 'side', swatch: ['#ffffff', '#f4f5f7', '#6a2fc9', '#00a5a0'] },
]

export function designOf(id: string | undefined): Design {
  return designs.find((d) => d.id === id) ?? designs[0]
}

/** 로그인 화면 왼쪽 그림이 받는 값. 실제 상태만 넘긴다. */
export interface ArtProps {
  /** 접속 중인가 */
  busy: boolean
  /** 고른 망의 이름 */
  network: string
  /** 전송 방향 글자(예: 업무망->인터넷망) */
  direction: string
}

const arts = import.meta.glob<{ default: Component<ArtProps> }>('./*/Art.svelte', { eager: true })
const strips = import.meta.glob<{ default: Component }>('./*/Strip.svelte', { eager: true })

export const artOf = (id: DesignId) => arts[`./${id}/Art.svelte`]?.default
export const stripOf = (id: DesignId) => strips[`./${id}/Strip.svelte`]?.default
