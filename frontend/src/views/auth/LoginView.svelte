<!--
  로그인 화면(원본 LoginView.qml, docs/port/01 5.1): 망 고르기 · ID · 비밀번호 · ID 저장 · 로그인 중 표시 · 경고 문구.
  - 서버 주소는 화면이 보내지 않고 fmenv.ini 목록의 순번만 보낸다(AuthService.Login(serverIndex, id, pw)). 고른 망은 곧바로 저장한다.
  - 빈 칸이면 그 칸을 빨갛게 하고 「계정정보를 정확히 입력해 주세요.」, 서버에 보내지 않는다. Enter 와 [로그인] 이 같다.
  - 응답까지 칸·단추를 잠그고 「접속 중..」 의 마지막 점을 0.4초마다 깜빡인다. 경고 자리는 늘 비워 두어 폼이 흔들리지 않는다.
  - 실패: 「오류」 창과 경고 문구에 같은 글. 비밀번호 칸은 그대로 둔다. 미등록 ID·중복 로그인·무결성 실패는 창을 닫으면 종료.
  - 성공: 비밀번호 칸을 비운다. 비밀번호 기한이 지났으면 강제 변경 창을 이 화면에서 띄우고, 변경해야 메인 화면으로 간다.
  자동화 이름(원본 objectName): loginView·loginIdInput·loginPasswordInput·loginButton·loginWarningText 를 id 로 둔다.
-->
<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { Application } from '@wailsio/runtime'
  import Sun from '@lucide/svelte/icons/sun'
  import Moon from '@lucide/svelte/icons/moon'
  import Monitor from '@lucide/svelte/icons/monitor'
  import Languages from '@lucide/svelte/icons/languages'
  import UserIcon from '@lucide/svelte/icons/user'
  import CircleAlert from '@lucide/svelte/icons/circle-alert'
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import { Badge, Button, Checkbox, IconButton, Input, PasswordInput } from '../../lib/ui'
  import { t, locale } from '../../lib/i18n'
  import { AuthService, SettingsService, errorText, type LoginInfo, type Server } from '../../lib/api'
  import { LOC_BUSINESS } from '../../lib/stores/session'
  import { alert, design, theme, cycleTheme, setLanguage } from '../../lib/stores/ui'
  import { artOf, designOf } from '../../designs/registry'
  import DesignMenu from '../../shell/DesignMenu.svelte'
  import Logo from '../../shell/Logo.svelte'
  import PasswordChangeDialog from './PasswordChangeDialog.svelte'
  import { cancelPendingLogin, passwordPrompt } from './state'

  let { onLogin }: { onLogin: (u: LoginInfo) => void } = $props()

  let servers = $state<Server[]>([])
  let serverIdx = $state(0)
  let id = $state('')
  let pw = $state('')
  let saveId = $state(true)
  let busy = $state(false)
  let error = $state('')
  let idErr = $state('')
  let pwErr = $state('')
  let version = $state('')
  let idInput = $state<HTMLInputElement>()
  let pwInput = $state<HTMLInputElement>()
  let forceOpen = $state(false)
  let dotOn = $state(true)

  const themeIcon = $derived($theme === 'light' ? Sun : $theme === 'dark' ? Moon : Monitor)
  const canSubmit = $derived(servers.length > 0 && !busy)
  const Art = $derived(artOf($design))

  // 「접속 중..」 의 마지막 점 깜빡임(0.4초)
  $effect(() => {
    if (!busy) return
    dotOn = true
    const timer = setInterval(() => (dotOn = !dotOn), 400)
    return () => clearInterval(timer)
  })

  onMount(async () => {
    SettingsService.AppInfo()
      .then((a) => (version = a.version))
      .catch(() => {})
    try {
      servers = (await AuthService.Servers()) ?? []
      if (!servers.length) error = t('auth.noServer')
      const saved = await AuthService.SavedLogin()
      serverIdx = saved.serverIndex
      saveId = saved.saveId
      id = saved.userId ?? ''
    } catch (e) {
      error = `${t('auth.serverLoadFail')} ${errorText(e)}`
    }
    await tick()
    ;(id ? pwInput : idInput)?.focus()
  })

  function pickServer(i: number) {
    serverIdx = i
    AuthService.SetServerIndex(i).catch(() => {}) // 고르는 즉시 저장(원본 09 5.1)
  }

  function onSaveId(v: boolean) {
    AuthService.SetSaveID(v).catch(() => {})
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    if (!canSubmit) return
    idErr = id.trim() ? '' : t('auth.required')
    pwErr = pw || idErr ? '' : t('auth.required')
    if (idErr || pwErr) {
      error = ''
      ;(idErr ? idInput : pwInput)?.focus()
      return
    }
    busy = true
    error = ''
    try {
      const r = await AuthService.Login(serverIdx, id.trim(), pw)
      if (!r) throw new Error(t('common.error.unknown'))
      if (r.error) {
        error = r.error // 경고 문구와 오류 창에 같은 글(AUTH-020), 비밀번호 칸은 그대로
        await alert({ title: r.title || t('auth.errorTitle'), text: r.error, dismissible: !r.exit })
        if (r.exit) {
          await Application.Quit() // 창이 닫힌 뒤 종료(미등록 ID·중복 로그인·무결성 실패)
          return
        }
        await tick()
        pwInput?.focus()
        return
      }
      pw = ''
      if (r.prompt === 'force') {
        forceOpen = true // 변경에 성공해야 메인 화면으로(onForceDone)
        return
      }
      if (r.prompt === 'default') passwordPrompt.set({ kind: 'default' })
      else if (r.prompt === 'expire') passwordPrompt.set({ kind: 'expire', days: r.remainDays })
      if (r.user) onLogin(r.user)
    } catch (err) {
      error = errorText(err)
    } finally {
      busy = false
    }
  }

  async function onForceDone() {
    try {
      const u = await AuthService.CompleteLogin()
      if (u) onLogin(u)
    } catch (err) {
      error = errorText(err)
    }
  }

  async function onForceCancel() {
    await cancelPendingLogin()
    await Application.Quit()
  }
</script>

<PasswordChangeDialog bind:open={forceOpen} force notice={t('auth.pw.forceNotice')} onDone={onForceDone} onCancel={onForceCancel} />

<div class="login" id="loginView">
  <div class="corner">
    <DesignMenu />
    {#if !designOf($design).scheme}
      <IconButton icon={themeIcon} label="{t('common.shell.theme')}: {t(`common.theme.${$theme}`)}" onclick={cycleTheme} />
    {/if}
    <IconButton
      icon={Languages}
      label="{t('common.shell.language')}: {$locale === 'ko' ? t('common.lang.ko') : t('common.lang.en')}"
      onclick={() => setLanguage($locale === 'ko' ? 'en' : 'ko')}
    />
  </div>

  <div class="card">
    <aside class="brand">
      <Logo size={30} />
      <p class="tagline">{t('auth.tagline')}</p>
      <div class="art" aria-hidden="true">
        {#if Art}
          <Art {busy} network={servers[serverIdx]?.name ?? ''} direction={servers[serverIdx]?.network ?? ''} />
        {:else}
          <span class="node b"></span><span class="wire"></span><span class="node i"></span>
        {/if}
      </div>
      {#if version}<p class="ver">{t('auth.version', { v: version })}</p>{/if}
    </aside>

    <form class="form" onsubmit={submit} aria-labelledby="login-title" novalidate>
      <h1 id="login-title">{t('auth.title')}</h1>

      <fieldset class="nets" disabled={busy}>
        <legend>{t('auth.network')}</legend>
        <div class="netlist">
          {#each servers as s, i (s.index)}
            {@const biz = s.loc === LOC_BUSINESS}
            <label class="net" class:on={serverIdx === i}>
              <input type="radio" name="server" value={i} checked={serverIdx === i} onchange={() => pickServer(i)} />
              <span class="dot" class:biz aria-hidden="true"></span>
              <span class="nname">{s.name}</span>
              {#if s.name !== t('common.net.business') && s.name !== t('common.net.internet')}
                <Badge size="sm" tone={biz ? 'business' : 'internet'} label={biz ? t('common.net.business') : t('common.net.internet')} />
              {/if}
            </label>
          {/each}
        </div>
        {#if servers[serverIdx]?.network}
          <p class="direction">
            {#each servers[serverIdx].network.split('->') as part, j (j)}
              {#if j > 0}<ArrowRight size={12} aria-hidden="true" />{/if}{part.trim()}
            {/each}
          </p>
        {/if}
      </fieldset>

      <Input
        label={t('auth.id')}
        bind:value={id}
        bind:input={idInput}
        icon={UserIcon}
        size="lg"
        id="loginIdInput"
        placeholder={t('auth.idPlaceholder')}
        error={idErr}
        oninput={() => (idErr = '')}
        autocomplete="username"
        spellcheck={false}
        disabled={busy}
      />
      <PasswordInput
        label={t('auth.password')}
        bind:value={pw}
        bind:input={pwInput}
        size="lg"
        id="loginPasswordInput"
        placeholder={t('auth.pwPlaceholder')}
        error={pwErr}
        oninput={() => (pwErr = '')}
        maxlength={30}
        autocomplete="current-password"
        disabled={busy}
      />

      <Checkbox label={t('auth.saveId')} bind:checked={saveId} onchange={onSaveId} disabled={busy} />

      <div class="error" class:shown={!!error} role="alert" id="loginWarningText" aria-hidden={!error}>
        {#if error}<CircleAlert size={16} aria-hidden="true" /><span>{error}</span>{/if}
      </div>

      <Button type="submit" variant="primary" size="lg" full disabled={!servers.length || busy} id="loginButton" aria-busy={busy}>
        {#if busy}{t('auth.submitting')}<span class="tail" class:off={!dotOn} aria-hidden="true">.</span>{:else}{t('auth.submit')}{/if}
      </Button>
    </form>
  </div>
</div>

<style>
  .login {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--sp-6);
    background:
      radial-gradient(1200px 600px at 15% 0%, var(--color-accent-subtle), transparent 60%),
      var(--color-bg-chrome);
    overflow: auto;
  }
  .corner {
    position: absolute;
    top: var(--sp-3);
    right: var(--sp-3);
    display: flex;
    gap: var(--sp-1);
  }
  .card {
    display: grid;
    grid-template-columns: 300px 400px;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-xl);
    background: var(--color-surface-raised);
    box-shadow: var(--shadow-3);
    overflow: hidden;
  }
  .brand {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-8) var(--sp-6);
    background: var(--color-surface);
    border-right: 1px solid var(--color-border-subtle);
  }
  .tagline {
    margin: 0;
    color: var(--color-text-2);
    font-size: var(--fs-md);
  }
  .art {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    min-height: 120px;
  }
  .node {
    width: 44px;
    height: 44px;
    border-radius: var(--r-lg);
    border: 2px solid;
  }
  .node.b {
    border-color: var(--color-net-business-border);
    background: var(--color-net-business-subtle);
  }
  .node.i {
    border-color: var(--color-net-internet-border);
    background: var(--color-net-internet-subtle);
  }
  .wire {
    width: 72px;
    height: 2px;
    background: repeating-linear-gradient(90deg, var(--color-border-strong) 0 6px, transparent 6px 10px);
  }
  .ver {
    margin: 0;
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
    padding: var(--sp-8) var(--sp-8) var(--sp-6);
  }
  h1 {
    margin: 0 0 var(--sp-1);
    font-size: var(--fs-2xl);
    font-weight: var(--fw-bold);
  }
  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    margin-bottom: var(--sp-1);
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--color-text-2);
  }
  .netlist {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: var(--sp-2);
  }
  .net {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    height: var(--h-lg);
    padding: 0 var(--sp-3);
    border: 1px solid var(--color-border);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    cursor: pointer;
    transition:
      border-color var(--dur-fast) var(--ease),
      background-color var(--dur-fast) var(--ease);
  }
  .net:hover {
    border-color: var(--color-border-strong);
  }
  .net.on {
    border-color: var(--color-accent-fill);
    box-shadow: 0 0 0 1px var(--color-accent-fill);
    background: var(--color-accent-subtle);
  }
  .net:has(input:focus-visible) {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  .net input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-net-internet);
  }
  .dot.biz {
    background: var(--color-net-business);
  }
  .nname {
    flex: 1;
    min-width: 0;
    font-weight: var(--fw-semibold);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .direction {
    display: flex;
    align-items: center;
    gap: var(--sp-1);
    margin: var(--sp-2) 0 0;
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
  .error {
    display: flex;
    align-items: flex-start;
    gap: var(--sp-2);
    min-height: 2.75em; /* 문구가 없어도 자리를 비워 둔다(AUTH-005) */
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid transparent;
    border-radius: var(--r-md);
    color: var(--color-danger);
    font-size: var(--fs-body);
    white-space: pre-line;
    opacity: 0;
    transition: opacity var(--dur-fast) var(--ease);
  }
  .error.shown {
    opacity: 1;
    border-color: var(--color-danger-border);
    background: var(--color-danger-subtle);
  }
  .tail.off {
    visibility: hidden;
  }
  .error :global(svg) {
    flex: none;
    margin-top: 2px;
  }
  @media (max-width: 760px) {
    .card {
      grid-template-columns: minmax(0, 400px);
    }
    .brand {
      display: none;
    }
  }
</style>
