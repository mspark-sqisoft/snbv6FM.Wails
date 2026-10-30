<!--
  최상위: 로그인 화면 ↔ 메인 틀(Shell) 전환, 전역 알림 띠·확인 창, Go 이벤트(안내·세션) 받기, 자동 로그아웃 시계 되돌리기.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import { get } from 'svelte/store'
  import { Application } from '@wailsio/runtime'
  import { Dialog, Toast } from './lib/ui'
  import { t } from './lib/i18n'
  import {
    AuthService,
    Ev,
    onAll,
    type AutoLogout,
    type GoDialog,
    type LoginInfo,
    type Notice,
    type Settings,
  } from './lib/api'
  import { logoutProcess, setUser, user } from './lib/stores/session'
  import {
    alert,
    applySettings,
    clearDialogs,
    closeDialog,
    closeToast,
    currentToast,
    dialogs,
    goAlert,
    loadSettings,
    navigate,
    toast,
  } from './lib/stores/ui'
  import { HOME } from './routes'
  import Shell from './shell/Shell.svelte'
  import LoginView from './views/auth/LoginView.svelte'

  const front = $derived($dialogs[0])
  let dialogOpen = $state(false)
  let closingOk = false
  $effect(() => {
    dialogOpen = !!front // 줄 맨 앞이 바뀌면 다음 창을 연다
  })

  function login(u: LoginInfo) {
    navigate(HOME)
    setUser(u)
  }

  function goDialog(d: GoDialog) {
    const title = d.title || t('common.dialog.info')
    if (!d.onClose) return goAlert(title, d.text)
    void alert({ title, text: d.text }).then(() => {
      if (d.onClose === 'exit') void Application.Quit()
      else if (d.onClose === 'logoutProcess') void logoutProcess()
    })
  }

  onMount(() => {
    void loadSettings()
    AuthService.CurrentUser()
      .then((u) => u && login(u)) // 화면만 다시 읽힌 경우(세션은 Go 에 살아 있다)
      .catch(() => {})

    const off = onAll({
      [Ev.notice]: (n: Notice) => toast(n.text, n.severity),
      [Ev.dialog]: (d: GoDialog) => goDialog(d),
      [Ev.sessionEnded]: () => {
        clearDialogs() // 열린 창을 모두 닫는다(원본 closeSessionPopups) ― 자동 로그아웃 안내는 이 뒤에 온다
        setUser(null)
      },
      [Ev.autoLogout]: (a: AutoLogout) =>
        void alert({ title: t('common.session.timeoutTitle'), text: t('common.session.timeoutText', { min: a.minutes }) }),
      [Ev.disconnected]: () =>
        void alert({
          title: t('common.session.disconnectedTitle'),
          text: t('common.session.disconnectedText'),
          dismissible: false,
        }).then(() => logoutProcess()),
      [Ev.settingsChanged]: (s: Settings) => applySettings(s),
    })

    // 화면 입력이 있으면 1초에 한 번쯤 자동 로그아웃 시계를 되돌린다
    let last = 0
    const touch = () => {
      if (!get(user) || Date.now() - last < 1000) return
      last = Date.now()
      AuthService.Touch().catch(() => {})
    }
    const evs = ['pointerdown', 'keydown', 'wheel'] as const
    evs.forEach((n) => addEventListener(n, touch, { capture: true, passive: true }))
    return () => {
      off()
      evs.forEach((n) => removeEventListener(n, touch, { capture: true }))
    }
  })
</script>

{#if $user}
  <Shell />
{:else}
  <LoginView onLogin={login} />
{/if}

{#if front}
  {#key front.id}
    <Dialog
      bind:open={dialogOpen}
      title={front.title}
      okLabel={front.okLabel}
      cancelLabel={front.cancelLabel}
      okVariant={front.danger ? 'danger' : 'primary'}
      dismissible={front.dismissible ?? true}
      onOk={() => {
        closingOk = true
      }}
      onClose={() => {
        const ok = closingOk
        closingOk = false
        closeDialog(front.id, ok)
      }}
    >
      <p class="dialog-text">{front.text}</p>
    </Dialog>
  {/key}
{/if}

<Toast notice={$currentToast} onClose={closeToast} />
