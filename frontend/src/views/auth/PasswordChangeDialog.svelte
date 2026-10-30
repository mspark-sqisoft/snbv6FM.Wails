<!--
  비밀번호 변경 창(원본 PasswordChangeDlg.qml 하나로 합침). 기본 비밀번호·만료 안내·사용자 정보 팝업·강제 변경이 모두 이 창을 쓴다.
  - force: 비밀번호 기한 지남. Esc·창 밖으로 닫히지 않고 [취소] 는 onCancel(앱 종료). 성공하면 onDone(로그인 마저 하기).
  - 검사 순서·문구는 원본 그대로(현재 빔 → 새 빔 → 새=현재 → 확인 빔 → 새≠확인 → 규칙). Go(AuthService.ChangePassword)도 같은 검사를 한 번 더 한다.
  - 서버 거절은 서버 사유 그대로 창 안 빨간 글로 보인다. 성공은 「정보」 창 뒤 닫는다.
-->
<script lang="ts">
  import { tick } from 'svelte'
  import KeyRound from '@lucide/svelte/icons/key-round'
  import CircleAlert from '@lucide/svelte/icons/circle-alert'
  import { Button, Dialog, PasswordInput } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { AuthService, errorText } from '../../lib/api'
  import { alert } from '../../lib/stores/ui'

  let {
    open = $bindable(false),
    force = false,
    notice = '',
    onDone,
    onCancel,
  }: {
    open?: boolean
    force?: boolean
    /** 창 위쪽 안내 한 줄(기본 비밀번호·강제 변경) */
    notice?: string
    onDone?: () => void
    onCancel?: () => void
  } = $props()

  // 원본 정규식(앞보기) ― JS 정규식은 지원한다
  const RULE = /^(?=.*[A-Za-z])(?=.*\d)(?=.*[@$!%*#?&])[A-Za-z\d@$!%*#?&]{10,16}$/

  let cur = $state('')
  let next = $state('')
  let confirmPw = $state('')
  let err = $state('')
  let busy = $state(false)
  let curInput = $state<HTMLInputElement>()

  $effect(() => {
    if (open) {
      cur = next = confirmPw = err = ''
      void tick().then(() => curInput?.focus())
    }
  })

  function validate(): string {
    if (!cur) return t('auth.pw.curEmpty')
    if (!next) return t('auth.pw.newEmpty')
    if (next === cur) return t('auth.pw.same')
    if (!confirmPw) return t('auth.pw.confirmEmpty')
    if (next !== confirmPw) return t('auth.pw.mismatch')
    if (!RULE.test(confirmPw)) return t('auth.pw.rule')
    return ''
  }

  /** 칸을 떠날 때 규칙 검사(원본 onEditingFinished). */
  function checkRule(v: string) {
    if (v && !RULE.test(v)) err = t('auth.pw.rule')
    else if (err === t('auth.pw.rule')) err = ''
  }

  async function submit() {
    if (busy) return
    err = validate()
    if (err) return
    busy = true
    try {
      await AuthService.ChangePassword(cur, next, confirmPw)
    } catch (e) {
      err = errorText(e)
      busy = false
      return
    }
    busy = false
    cur = next = confirmPw = ''
    open = false
    await alert({ title: t('auth.info'), text: t('auth.pw.changed') })
    onDone?.()
  }

  function cancel() {
    open = false
    onCancel?.()
  }
</script>

<Dialog bind:open title={t('auth.pw.title')} width={420} dismissible={!force && !busy} onCancel={() => onCancel?.()}>
  <form
    class="pwform"
    id="passwordChangeDlg"
    novalidate
    onsubmit={(e) => {
      e.preventDefault()
      void submit()
    }}
  >
    {#if notice}
      <p class="notice"><KeyRound size={16} aria-hidden="true" /><span>{notice}</span></p>
    {/if}
    <PasswordInput
      label={t('auth.pw.current')}
      bind:value={cur}
      bind:input={curInput}
      maxlength={30}
      autocomplete="current-password"
      disabled={busy}
      id="pwCurrentInput"
    />
    <PasswordInput
      label={t('auth.pw.next')}
      bind:value={next}
      placeholder={t('auth.pw.placeholder')}
      maxlength={16}
      autocomplete="new-password"
      disabled={busy}
      onblur={() => checkRule(next)}
      id="pwNewInput"
    />
    <PasswordInput
      label={t('auth.pw.confirm')}
      bind:value={confirmPw}
      placeholder={t('auth.pw.placeholder')}
      maxlength={16}
      autocomplete="new-password"
      disabled={busy}
      onblur={() => checkRule(confirmPw)}
      id="pwConfirmInput"
    />
    <div class="err" class:shown={!!err} role="alert" id="pwErrorText">
      {#if err}<CircleAlert size={14} aria-hidden="true" /><span>{err}</span>{/if}
    </div>
    <!-- Enter 로 제출되게 숨은 단추 -->
    <button type="submit" hidden aria-hidden="true" tabindex="-1"></button>
  </form>
  {#snippet footer()}
    <Button onclick={cancel} disabled={busy}>{t('auth.pw.cancel')}</Button>
    <Button variant="primary" onclick={submit} loading={busy}>{t('auth.pw.ok')}</Button>
  {/snippet}
</Dialog>

<style>
  .pwform {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .notice {
    display: flex;
    align-items: flex-start;
    gap: var(--sp-2);
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--color-warning-border);
    border-radius: var(--r-md);
    background: var(--color-warning-subtle);
    color: var(--color-text);
    font-size: var(--fs-sm);
  }
  .notice :global(svg) {
    flex: none;
    margin-top: 2px;
    color: var(--color-warning);
  }
  .err {
    display: flex;
    align-items: flex-start;
    gap: var(--sp-1);
    min-height: 1.4em;
    color: var(--color-danger);
    font-size: var(--fs-sm);
    white-space: pre-line;
  }
  .err :global(svg) {
    flex: none;
    margin-top: 2px;
  }
</style>
