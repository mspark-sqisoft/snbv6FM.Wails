<!--
  인증 영역의 로그인 뒤 대화상자 모음(메인 담당의 Shell 이 로그인 뒤 붙인다).
  - 비밀번호 변경 창: 기본 비밀번호(곧바로)·만료 예정(먼저 「정보」 물음)·사용자 정보 팝업의 [비밀번호 변경]
  로그아웃 처리 방식 확인 창은 메인 담당(shell/LogoutDialog.svelte)이 그린다.
  로그아웃하면 Shell 과 함께 사라져 열린 창도 닫힌다(AUTH-036).
-->
<script lang="ts">
  import { onDestroy } from 'svelte'
  import { t } from '../../lib/i18n'
  import { confirm } from '../../lib/stores/ui'
  import PasswordChangeDialog from './PasswordChangeDialog.svelte'
  import { passwordPrompt, type PasswordPrompt } from './state'

  let pwOpen = $state(false)
  let pwNotice = $state('')

  async function show(p: PasswordPrompt) {
    if (p.kind === 'expire') {
      const yes = await confirm({
        title: t('auth.info'),
        text: t('auth.expireAsk', { n: p.days }),
        okLabel: t('auth.yes'),
        cancelLabel: t('auth.no'),
      })
      if (!yes) return
    }
    pwNotice = p.kind === 'default' ? t('auth.pw.defaultNotice') : ''
    pwOpen = true
  }

  // 로그인 화면이 남긴 요청은 붙자마자, 그 뒤 요청은 들어올 때마다 꺼내 쓰고 비운다
  const off = passwordPrompt.subscribe((p) => {
    if (!p) return
    passwordPrompt.set(null)
    void show(p)
  })
  onDestroy(off)

  // 로그아웃으로 사라질 때 남은 요청도 버린다(다음 로그인에 옛 창이 뜨지 않게)
  onDestroy(() => passwordPrompt.set(null))
</script>

<PasswordChangeDialog bind:open={pwOpen} notice={pwNotice} />
