<!--
  사용자 정보(전송 권한) 팝업 ― 원본 UserProfileInfo.qml. 툴바의 사용자 이름을 누르면 연다(메인 담당: <AuthorityDialog bind:open />).
  열 때마다 AuthService.Authority() 로 다시 받는다. 방향 이름은 fmenv.ini 의 NETWORK(예: 인터넷망->업무망)다.
  파일: 들어오는·나가는 방향 전송(허용/차단)·결재 방식, 최대전송크기(0 = 무제한), 최대전송개수. 클립보드: 같은 넷 + 최대전송크기.
  [비밀번호 변경] 은 이 창을 닫고 비밀번호 변경 창(Host)을 연다. 로그아웃하면 Shell 과 함께 사라진다(AUTH-038).
-->
<script lang="ts">
  import KeyRound from '@lucide/svelte/icons/key-round'
  import { Badge, Button, Dialog, EmptyState, Spinner } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { AuthService, errorText, type Authority } from '../../lib/api'
  import { session } from '../../lib/stores/session'
  import { openPasswordChange } from './state'

  let { open = $bindable(false) }: { open?: boolean } = $props()

  let auth = $state<Authority | null>(null)
  let loading = $state(false)
  let error = $state('')

  $effect(() => {
    if (open) void load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      auth = await AuthService.Authority()
    } catch (e) {
      auth = null
      error = errorText(e)
    } finally {
      loading = false
    }
  }

  /** 원본 FileMoverUtil::getFileSizeToText: GB·MB 소수 둘, KB 소수 하나, 0 = 무제한(권한 팝업 규칙). */
  function sizeText(b: number): string {
    if (b <= 0) return t('auth.auth.unlimited')
    const G = 1024 ** 3
    const M = 1024 ** 2
    if (b > G) return `${(b / G).toFixed(2)} GB`
    if (b > M) return `${(b / M).toFixed(2)} MB`
    if (b > 1024) return `${(b / 1024).toFixed(1)} KB`
    return `${b} byte`
  }
  const aprvText = (n: number) => (n >= 0 && n <= 7 ? t(`auth.aprv.${n}`) : t('auth.aprv.unknown', { n }))

  type Row = { label: string; value: string; tone?: 'success' | 'danger' }
  const allow = (v: boolean): Row['tone'] => (v ? 'success' : 'danger')
  const permText = (v: boolean) => (v ? t('auth.auth.allow') : t('auth.auth.deny'))

  const fileRows = $derived<Row[]>(
    auth
      ? [
          { label: t('auth.auth.send', { dir: auth.inName }), value: permText(auth.fileIn), tone: allow(auth.fileIn) },
          { label: t('auth.auth.send', { dir: auth.outName }), value: permText(auth.fileOut), tone: allow(auth.fileOut) },
          { label: t('auth.auth.aprv', { dir: auth.inName }), value: aprvText(auth.fileInAprv) },
          { label: t('auth.auth.aprv', { dir: auth.outName }), value: aprvText(auth.fileOutAprv) },
          { label: t('auth.auth.maxSize'), value: sizeText(auth.maxFileSize) },
          {
            label: t('auth.auth.maxCount'),
            value: auth.maxFileCount > 0 ? String(auth.maxFileCount) : t('auth.auth.unlimited'),
          },
        ]
      : [],
  )
  const clipRows = $derived<Row[]>(
    auth
      ? [
          { label: t('auth.auth.send', { dir: auth.inName }), value: permText(auth.clipIn), tone: allow(auth.clipIn) },
          { label: t('auth.auth.send', { dir: auth.outName }), value: permText(auth.clipOut), tone: allow(auth.clipOut) },
          { label: t('auth.auth.aprv', { dir: auth.inName }), value: aprvText(auth.clipInAprv) },
          { label: t('auth.auth.aprv', { dir: auth.outName }), value: aprvText(auth.clipOutAprv) },
          { label: t('auth.auth.maxSize'), value: sizeText(auth.maxClipSize) },
        ]
      : [],
  )

  function changePassword() {
    open = false
    openPasswordChange()
  }
</script>

<Dialog bind:open title={t('auth.auth.title')} width={400} cancelLabel={null} okLabel={t('common.close')}>
  <div class="profile" id="userProfileInfo" aria-busy={loading}>
    {#if $session.user}
      <div class="who">
        <span class="avatar" aria-hidden="true">{$session.user.userName.slice(0, 1)}</span>
        <div class="whotext">
          <div class="name">{$session.user.userName}</div>
          <div class="meta">{$session.user.userId}{$session.user.dept ? ` · ${$session.user.dept}` : ''}</div>
        </div>
        <Badge size="sm" tone={$session.isBusiness ? 'business' : 'internet'} label={$session.networkName} />
      </div>
    {/if}

    {#if loading && !auth}
      <div class="state"><Spinner label={t('common.loading')} /></div>
    {:else if error}
      <EmptyState compact title={t('auth.auth.loadFail')} description={error}>
        <Button size="sm" onclick={load}>{t('common.retry')}</Button>
      </EmptyState>
    {:else if auth}
      {#each [{ title: t('auth.auth.file'), rows: fileRows, id: 'authFile' }, { title: t('auth.auth.clip'), rows: clipRows, id: 'authClip' }] as sec (sec.id)}
        <section aria-labelledby={sec.id}>
          <h3 id={sec.id}>{sec.title}</h3>
          <dl>
            {#each sec.rows as r (r.label)}
              <dt>{r.label}</dt>
              <dd>
                {#if r.tone}<span class="dot {r.tone}" aria-hidden="true"></span>{/if}{r.value}
              </dd>
            {/each}
          </dl>
        </section>
      {/each}
    {/if}

    <Button icon={KeyRound} full onclick={changePassword} id="changePasswordButton">{t('auth.auth.changePassword')}</Button>
  </div>
</Dialog>

<style>
  .profile {
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }
  .who {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
  }
  .whotext {
    flex: 1;
    min-width: 0;
  }
  .avatar {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: var(--r-full);
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    font-size: var(--fs-lg);
    font-weight: var(--fw-bold);
  }
  .name {
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
  }
  .meta {
    color: var(--color-text-3);
    font-size: var(--fs-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .state {
    display: flex;
    justify-content: center;
    padding: var(--sp-6) 0;
  }
  h3 {
    margin: 0 0 var(--sp-2);
    padding-bottom: var(--sp-1);
    border-bottom: 1px solid var(--color-border-subtle);
    font-size: var(--fs-body);
    font-weight: var(--fw-semibold);
  }
  dl {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: var(--sp-1) var(--sp-3);
    margin: 0;
    font-size: var(--fs-body);
  }
  dt {
    color: var(--color-text-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  dd {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-1);
    justify-content: flex-end;
    margin: 0;
    font-weight: var(--fw-medium);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: var(--r-full);
  }
  .dot.success {
    background: var(--color-success);
  }
  .dot.danger {
    background: var(--color-danger);
  }
</style>
