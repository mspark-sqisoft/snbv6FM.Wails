<!--
  왼쪽 메뉴(원본 sideMenu). 지금 화면은 왼쪽 막대 + 면 + 굵은 글자 + 강조색 아이콘, aria-current="page".
  collapsed 면 아이콘만 그리고 이름은 툴팁으로. canCollapse 가 거짓(좁은 창)이면 [메뉴 접기] 를 숨긴다.
-->
<script lang="ts">
  import PanelLeftClose from '@lucide/svelte/icons/panel-left-close'
  import PanelLeftOpen from '@lucide/svelte/icons/panel-left-open'
  import { Tooltip } from '../lib/ui'
  import { t, locale } from '../lib/i18n'
  import type { Route, RouteId } from '../routes'

  let {
    items,
    current,
    collapsed,
    canCollapse,
    onNavigate,
    onToggle,
  }: {
    items: Route[]
    current: RouteId
    collapsed: boolean
    canCollapse: boolean
    onNavigate: (id: RouteId) => void
    onToggle: () => void
  } = $props()
</script>

<nav
  class="side"
  class:collapsed
  class:en={$locale === 'en'}
  id="sideMenu"
  aria-label={t('common.shell.menu')}
>
  <ul>
    {#each items as r (r.id)}
      {@const label = t(r.labelKey)}
      <li>
        <Tooltip text={label} placement="right" disabled={!collapsed}>
          <button
            type="button"
            class="item"
            aria-current={r.id === current ? 'page' : undefined}
            aria-label={collapsed ? label : undefined}
            onclick={() => onNavigate(r.id)}
          >
            <r.icon size={18} aria-hidden="true" />
            {#if !collapsed}<span class="label">{label}</span>{/if}
          </button>
        </Tooltip>
      </li>
    {/each}
  </ul>
  {#if canCollapse}
    {@const label = collapsed ? t('common.shell.expandMenu') : t('common.shell.collapseMenu')}
    <Tooltip text={label} placement="right" disabled={!collapsed}>
      <button type="button" class="item toggle" aria-label={label} aria-expanded={!collapsed} onclick={onToggle}>
        {#if collapsed}<PanelLeftOpen size={18} aria-hidden="true" />{:else}<PanelLeftClose size={18} aria-hidden="true" />{/if}
        {#if !collapsed}<span class="label">{label}</span>{/if}
      </button>
    </Tooltip>
  {/if}
</nav>

<style>
  .side {
    display: flex;
    flex-direction: column;
    flex: none;
    width: var(--sidebar-w);
    padding: var(--sp-2);
    background: var(--color-bg-chrome);
    border-right: 1px solid var(--color-border-subtle);
    overflow-y: auto;
    overflow-x: hidden;
    transition: width var(--dur) var(--ease);
  }
  .side.en {
    width: var(--sidebar-w-en);
  }
  .side.collapsed {
    width: var(--sidebar-w-collapsed);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
  }
  li,
  .side :global(.anchor) {
    display: flex;
    width: 100%;
  }
  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    width: 100%;
    height: 38px;
    padding: 0 var(--sp-3) 0 10px;
    border: 0;
    border-radius: var(--r-md);
    background: none;
    color: var(--color-text-2);
    font-size: var(--fs-md);
    text-align: left;
    cursor: pointer;
    transition:
      background-color var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
  }
  .item:hover {
    background: var(--color-hover);
    color: var(--color-text);
  }
  .item[aria-current='page'] {
    background: var(--color-surface-raised);
    color: var(--color-text);
    font-weight: var(--fw-semibold);
    box-shadow: var(--shadow-1);
  }
  .item[aria-current='page']::before {
    content: '';
    position: absolute;
    left: 0;
    top: 10px;
    bottom: 10px;
    width: 3px;
    border-radius: 2px;
    background: var(--color-accent-fill);
  }
  .item[aria-current='page'] :global(svg) {
    color: var(--color-accent);
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .collapsed .item {
    justify-content: center;
    padding: 0;
  }
  .toggle {
    margin-top: var(--sp-2);
    color: var(--color-text-3);
    font-size: var(--fs-body);
  }
</style>
