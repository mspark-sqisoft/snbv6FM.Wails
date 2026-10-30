<!--
  로그인 뒤의 틀: 머리줄(TopBar) · [상단 탭(설정 배치 2) 또는 왼쪽 메뉴(그 밖)] · 본문 · 아래 전송 상태 서랍.
  - 창 폭이 880 보다 좁으면(원본 좁은 데스크톱 배치) 왼쪽 메뉴는 아이콘만, 상단 탭은 아이콘만 남긴다.
  - 본문은 지금 화면 하나만 그린다(들어올 때마다 새로 조회 ― 원본과 같다).
  - 영역 Host(views/<영역>/Host.svelte ― 영역의 대화상자·전역 구독)는 있는 것을 모두 저절로 붙인다(로그인 뒤에만).
  - 창 제목은 「snB App v6.0 — 지금 화면」(상단 탭 배치에서 화면 제목 줄이 없어도 지금 화면을 안다 ― 원본 MAIN-007 상충 해소).
  - 트레이 [정책 받아오기] 는 Go 가 사건만 보내고 여기서 PolicyService.Reload 를 부른다.
-->
<script lang="ts">
  import { onMount, type Component } from 'svelte'
  import { Window } from '@wailsio/runtime'
  import { Progress, Tabs } from '../lib/ui'
  import { t } from '../lib/i18n'
  import { Ev, PolicyService, errorText, on } from '../lib/api'
  import { session } from '../lib/stores/session'
  import { busy, design, navigate, route, settings, toast, updateSettings } from '../lib/stores/ui'
  import { menuFor, routes, type RouteId } from '../routes'
  import { designOf, stripOf } from '../designs/registry'
  import TopBar from './TopBar.svelte'
  import SideBar from './SideBar.svelte'
  import LogoutDialog from './LogoutDialog.svelte'
  import TransferDrawer from '../views/transfer/TransferDrawer.svelte'

  const NARROW = 880
  const TITLE = 'snB App v6.0'

  const hosts = Object.values(
    import.meta.glob<{ default: Component }>('../views/*/Host.svelte', { eager: true }),
  ).map((m) => m.default)

  let width = $state(window.innerWidth)
  const narrow = $derived(width < NARROW)
  // 디자인이 배치를 정하면 그것을 따른다. 저장값 1 등은 0(왼쪽 메뉴)으로 그린다
  const layout = $derived(designOf($design).layout ?? (($settings?.layout ?? 2) === 2 ? 'tabs' : 'side'))
  const Strip = $derived(stripOf($design))
  const menu = $derived(menuFor($session.role))
  const collapsed = $derived(narrow || ($settings?.navCollapsed ?? false))
  const Current = $derived(routes[$route].component)
  const tabItems = $derived(menu.map((r) => ({ id: r.id, label: t(r.labelKey), icon: r.icon })))

  // 메뉴에 없는 화면(역할이 바뀐 뒤 남은 결재처리 등)에 머물지 않게 ― 결재요청·정책은 id 로 여는 화면이라 둔다
  $effect(() => {
    if (!['request', 'policy'].includes($route) && !menu.some((r) => r.id === $route)) navigate(menu[0].id)
  })

  $effect(() => {
    const name = t(routes[$route].labelKey)
    Window.SetTitle(`${TITLE} — ${name}`).catch(() => {})
  })

  function toggleNav() {
    const next = !($settings?.navCollapsed ?? false)
    void updateSettings((s) => (s.navCollapsed = next))
  }

  onMount(() => {
    const off = on(Ev.trayPolicyReload, async () => {
      try {
        await PolicyService.Reload()
        toast(t('common.shell.policyReloaded'), 1)
      } catch (e) {
        const msg = errorText(e)
        if (msg !== t('common.error.notImplemented')) toast(`${t('common.shell.policyReloadFailed')} ${msg}`, 3)
      }
    })
    return () => {
      off()
      Window.SetTitle(TITLE).catch(() => {})
    }
  })
</script>

<svelte:window bind:innerWidth={width} />

<div class="shell" class:narrow data-route={$route}>
  <TopBar {narrow} />
  {#if Strip}<Strip />{/if}
  <div class="busy" aria-hidden={$busy > 0 ? undefined : 'true'}>
    {#if $busy > 0}<Progress indeterminate size="sm" label={t('common.loading')} />{/if}
  </div>
  {#if layout === 'tabs'}
    <div class="tabs">
      <Tabs
        id="topTabBar"
        variant="nav"
        idPrefix="nav"
        items={tabItems}
        value={$route}
        label={t('common.shell.menu')}
        onchange={(id) => navigate(id as RouteId)}
      />
    </div>
  {/if}
  <div class="body">
    {#if layout === 'side'}
      <SideBar items={menu} current={$route} {collapsed} canCollapse={!narrow} onNavigate={navigate} onToggle={toggleNav} />
    {/if}
    <main
      class="content"
      id={layout === 'tabs' ? `nav-panel-${$route}` : 'content'}
      role={layout === 'tabs' ? 'tabpanel' : undefined}
      aria-labelledby={layout === 'tabs' ? `nav-tab-${$route}` : undefined}
    >
      <Current />
    </main>
  </div>
  <TransferDrawer />
</div>

<LogoutDialog />
{#each hosts as Host, i (i)}
  <Host />
{/each}

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .busy {
    position: relative;
    height: 0;
    z-index: var(--z-overlay);
  }
  .busy :global(.wrap) {
    position: absolute;
    left: 0;
    right: 0;
    top: -2px;
  }
  .tabs {
    flex: none;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  /* 좁은 창: 상단 탭은 아이콘만(이름은 aria-label·툴팁 대신 탭 글자를 화면 읽기 프로그램용으로 남긴다) */
  .narrow .tabs :global([role='tab'] > span:not(.count)) {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .body {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .content {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--color-bg);
  }
</style>
