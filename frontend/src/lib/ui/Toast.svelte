<!--
  알림 띠(원본 CustomInfoBar). 화면 아래 가운데, 모달 아님. 4초 뒤 사라지고 새 안내는 바꿔치기한다 ―
  그 시계와 바꿔치기는 lib/stores/ui.ts 의 toast() 가 하고, 이 컴포넌트는 notice 하나를 그리기만 한다.
  top layer(popover)에 그려 대화상자 위에도 보인다(원본 쌓임: 서랍 < 대화상자 < 띠). id 는 원본 이름 AlertMessage.
-->
<script lang="ts">
  import Info from '@lucide/svelte/icons/info'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import CircleAlert from '@lucide/svelte/icons/circle-alert'
  import X from '@lucide/svelte/icons/x'
  import type { Severity } from './types'
  import { severityTone } from './types'
  import { t } from '../i18n'

  let {
    notice,
    onClose,
  }: {
    notice: { id: number; severity: Severity; text: string } | null
    onClose: () => void
  } = $props()

  const icons = { 0: Info, 1: CircleCheck, 2: TriangleAlert, 3: CircleAlert }
  let el: HTMLDivElement

  $effect(() => {
    const n = notice
    if (!el) return
    if (el.matches(':popover-open')) el.hidePopover()
    if (n) el.showPopover() // 다시 열어 가장 위로 올린다(그 사이 열린 대화상자보다 위)
  })
</script>

<div
  bind:this={el}
  id="AlertMessage"
  class="toast {notice ? severityTone[notice.severity] : ''}"
  popover="manual"
  role={notice?.severity === 3 ? 'alert' : 'status'}
>
  {#if notice}
    {@const Icon = icons[notice.severity]}
    {#key notice.id}
      <span class="icon"><Icon size={18} aria-hidden="true" /></span>
      <span class="text">{notice.text}</span>
    {/key}
    <button class="close" type="button" aria-label={t('common.close')} onclick={onClose}><X size={14} aria-hidden="true" /></button>
  {/if}
</div>

<style>
  .toast {
    position: fixed;
    inset: auto auto 32px 50%;
    transform: translateX(-50%);
    margin: 0;
    align-items: flex-start;
    gap: var(--sp-2);
    max-width: min(480px, calc(100vw - 32px));
    padding: var(--sp-2) var(--sp-2) var(--sp-2) var(--sp-3);
    border: 1px solid var(--tone-border);
    border-left: 4px solid var(--tone-fg);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    color: var(--color-text);
    box-shadow: var(--shadow-3);
    font-size: var(--fs-body);
  }
  /* 닫힌 popover 는 UA 가 숨긴다 ― display 는 열렸을 때만 준다(늘 주면 빈 띠가 보인다) */
  .toast:popover-open {
    display: flex;
    animation: rise var(--dur) var(--ease);
  }
  .info {
    --tone-fg: var(--color-info);
    --tone-border: var(--color-info-border);
  }
  .success {
    --tone-fg: var(--color-success);
    --tone-border: var(--color-success-border);
  }
  .warning {
    --tone-fg: var(--color-warning);
    --tone-border: var(--color-warning-border);
  }
  .danger {
    --tone-fg: var(--color-danger);
    --tone-border: var(--color-danger-border);
  }
  .icon {
    display: flex;
    padding-top: 1px;
    color: var(--tone-fg);
  }
  .text {
    flex: 1;
    min-width: 0;
    padding: 1px 0;
    white-space: pre-line;
    overflow-wrap: anywhere;
  }
  .close {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    color: var(--color-text-3);
    cursor: pointer;
  }
  .close:hover {
    background: var(--color-hover);
    color: var(--color-text);
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translate(-50%, 8px);
    }
  }
</style>
