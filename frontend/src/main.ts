import { mount } from 'svelte'
// 글꼴은 번들에 넣는다(망분리 PC 라 CDN 을 쓰지 않는다). 가변 woff2 하나가 모든 굵기를 맡는다.
import 'pretendard/dist/web/variable/pretendardvariable.css'
import './lib/ui/tokens.css'
import './designs/index.css'
import App from './App.svelte'

mount(App, { target: document.getElementById('app')! })
