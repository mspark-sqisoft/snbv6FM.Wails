<!--
  머리줄(원본 toolBar): 표지 · 망 배지(업무망 보라/인터넷망 청록) · 전송 방향 · ─ · 사용자(누르면 권한 팝업) · 테마 · 언어 · 로그아웃.
  사용자 이름을 누르면 views/auth/AuthorityDialog.svelte({open}) 를 연다(인증 담당 파일 ― 없으면 단추만 그린다).
  narrow(좁은 창)면 전송 방향과 사용자 이름 글자를 숨긴다(배지·머리글자는 남는다).
-->
<script lang="ts">
  import type { Component } from 'svelte'
  import Sun from '@lucide/svelte/icons/sun'
  import Moon from '@lucide/svelte/icons/moon'
  import Monitor from '@lucide/svelte/icons/monitor'
  import Languages from '@lucide/svelte/icons/languages'
  import LogOut from '@lucide/svelte/icons/log-out'
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import ChevronDown from '@lucide/svelte/icons/chevron-down'
  import { Badge, IconButton } from '../lib/ui'
  import { t, locale } from '../lib/i18n'
  import { session, requestLogout } from '../lib/stores/session'
  import { design, theme, cycleTheme, setLanguage } from '../lib/stores/ui'
  import { designOf } from '../designs/registry'
  import DesignMenu from './DesignMenu.svelte'
  import Logo from './Logo.svelte'

  let { narrow = false }: { narrow?: boolean } = $props()

  type AuthorityDialogType = Component<{ open?: boolean }, {}, 'open'>
  const AuthorityDialog = Object.values(
    import.meta.glob<{ default: AuthorityDialogType }>('../views/auth/AuthorityDialog.svelte', { eager: true }),
  )[0]?.default

  let authOpen = $state(false)

  const themeIcon = $derived($theme === 'light' ? Sun : $theme === 'dark' ? Moon : Monitor)
  const themeName = $derived(t(`common.theme.${$theme}`))
  const netTone = $derived($session.isBusiness ? 'business' : 'internet')
  // fmenv.ini NETWORK 는 "업무망->인터넷망" 꼴 ― 양쪽 이름으로 갈라 화살표 아이콘으로 잇는다
  const dir = $derived($session.direction.split('->').map((s) => s.trim()))
</script>

<header class="topbar" id="toolBar">
  <Logo />
  {#if $session.user}
    <span class="sep" aria-hidden="true"></span>
    <Badge tone={netTone} label={$session.networkName} title={t('common.net.' + ($session.isBusiness ? 'business' : 'internet'))} />
    {#if !narrow}
      {#if dir.length === 2}
        <span class="dir" aria-label="{t('common.shell.direction')}: {$session.direction}">
          {dir[0]}<ArrowRight size={12} aria-hidden="true" />{dir[1]}
        </span>
      {:else if $session.direction}
        <span class="dir">{$session.direction}</span>
      {/if}
    {/if}
  {/if}

  <span class="spacer"></span>

  {#if $session.user}
    <button
      class="user"
      type="button"
      id="userName"
      aria-label="{t('common.shell.userInfo')}: {$session.user.userName}"
      aria-haspopup="dialog"
      disabled={!AuthorityDialog}
      onclick={() => (authOpen = true)}
    >
      <span class="avatar" aria-hidden="true">{$session.user.userName.slice(0, 1)}</span>
      {#if !narrow}<span class="name">{$session.user.userName}</span>{/if}
      <ChevronDown size={14} aria-hidden="true" />
    </button>
  {/if}

  <DesignMenu />
  {#if !designOf($design).scheme}
    <IconButton icon={themeIcon} label="{t('common.shell.theme')}: {themeName}" onclick={cycleTheme} id="headerThemeToggle" />
  {/if}
  <IconButton
    icon={Languages}
    label="{t('common.shell.language')}: {$locale === 'ko' ? t('common.lang.ko') : t('common.lang.en')}"
    onclick={() => setLanguage($locale === 'ko' ? 'en' : 'ko')}
  />
  {#if $session.user}
    <IconButton icon={LogOut} label={t('common.shell.logout')} onclick={requestLogout} id="logoutButton" />
  {/if}
</header>

{#if AuthorityDialog && $session.user}
  <AuthorityDialog bind:open={authOpen} />
{/if}

<style>
  .topbar {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    height: var(--topbar-h);
    padding: 0 var(--sp-3) 0 var(--sp-4);
    flex: none;
    min-width: 0;
    background: var(--color-bg-chrome);
    border-bottom: 1px solid var(--color-border-subtle);
  }
  .sep {
    width: 1px;
    height: 20px;
    margin: 0 var(--sp-2);
    background: var(--color-border);
  }
  .dir {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-1);
    color: var(--color-text-3);
    font-size: var(--fs-sm);
    white-space: nowrap;
  }
  .spacer {
    flex: 1;
  }
  .user {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    height: var(--h-md);
    padding: 0 var(--sp-2) 0 var(--sp-1);
    margin-right: var(--sp-1);
    border: 0;
    border-radius: var(--r-md);
    background: none;
    color: var(--color-text);
    cursor: pointer;
  }
  .user:hover:not(:disabled) {
    background: var(--color-hover);
  }
  .user:disabled {
    cursor: default;
  }
  .user:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: -2px;
  }
  .avatar {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    font-size: var(--fs-sm);
    font-weight: var(--fw-bold);
  }
  .name {
    font-weight: var(--fw-medium);
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
