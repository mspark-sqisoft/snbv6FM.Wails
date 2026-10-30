<!--
  모달 대화상자(운영체제 기본 <dialog>.showModal ― 뒤 화면은 눌리지 않고 초점은 창 안에 갇힌다).
  - open 을 참으로 두면 열리고, 닫힐 때 거짓으로 돌아온다(bind:open).
  - 단추: 기본은 오른쪽 아래 [취소][확인]. okLabel·cancelLabel 로 글자, okVariant 로 확인 단추 색(primary·danger),
    cancelLabel 을 null 로 주면 [확인] 하나(안내 창). footer snippet 을 주면 단추 줄을 통째로 바꾼다.
  - onOk 가 Promise 를 돌려주면 끝날 때까지 확인 단추가 도는 표시로 바뀐다. onOk 가 false 를 돌려주면 닫지 않는다.
  - dismissible 이 거짓이면 Esc·바깥 누름으로 닫히지 않는다(답이 꼭 필요한 창 ― 파일 중복 확인 등).
  - 닫히면 열기 전 초점 자리로 돌아간다.
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import X from '@lucide/svelte/icons/x'
  import Button from './Button.svelte'
  import IconButton from './IconButton.svelte'
  import { t } from '../i18n'

  type OkResult = void | boolean | Promise<void | boolean>

  let {
    open = $bindable(false),
    title,
    width = 420,
    okLabel,
    cancelLabel,
    okVariant = 'primary',
    okDisabled = false,
    dismissible = true,
    closeOnBackdrop = true,
    onOk,
    onCancel,
    onClose,
    children,
    footer,
  }: {
    open?: boolean
    title: string
    width?: number
    okLabel?: string
    /** null 이면 취소 단추를 그리지 않는다 */
    cancelLabel?: string | null
    okVariant?: 'primary' | 'danger'
    okDisabled?: boolean
    dismissible?: boolean
    /** 거짓이면 바깥 누름으로만 안 닫힌다(Esc 는 dismissible 을 따른다) ― 입력이 많은 창이 실수로 닫히지 않게 */
    closeOnBackdrop?: boolean
    onOk?: () => OkResult
    onCancel?: () => void
    /** 어떤 길로든 닫힌 뒤 한 번 */
    onClose?: () => void
    children?: Snippet
    footer?: Snippet<[{ close: () => void }]>
  } = $props()

  const titleId = $props.id()
  let el: HTMLDialogElement
  let busy = $state(false)
  let returnFocus: HTMLElement | null = null

  $effect(() => {
    if (open && !el.open) {
      returnFocus = document.activeElement as HTMLElement | null
      el.showModal()
      // 첫 초점이 머리글 [닫기] 면 툴팁이 뜨고 Enter 가 창을 닫는다 ― 창 자신으로 옮긴다
      if ((document.activeElement as HTMLElement | null)?.closest('header')) el.focus()
    } else if (!open && el.open) {
      el.close()
    }
  })

  function close() {
    open = false
  }

  function onclose() {
    open = false
    busy = false
    onClose?.()
    returnFocus?.focus?.()
    returnFocus = null
  }

  async function ok() {
    if (!onOk) return close()
    busy = true
    try {
      const r = await onOk()
      if (r !== false) close()
    } finally {
      busy = false
    }
  }

  function cancel() {
    onCancel?.()
    close()
  }

  function oncancel(e: Event) {
    e.preventDefault() // Esc: 우리가 정한 길로만 닫는다
    if (dismissible && !busy) cancel()
  }

  function onclick(e: MouseEvent) {
    if (e.target === el && dismissible && closeOnBackdrop && !busy) cancel() // 창 바깥(배경) 누름
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog
  bind:this={el}
  class="dlg"
  tabindex="-1"
  style:width="min({width}px, calc(100vw - 32px))"
  aria-labelledby={titleId}
  {onclose}
  {oncancel}
  {onclick}
>
  {#if open}
    <header>
      <h2 id={titleId}>{title}</h2>
      {#if dismissible}
        <IconButton size="sm" icon={X} label={t('common.close')} onclick={cancel} disabled={busy} />
      {/if}
    </header>
    <div class="body">{@render children?.()}</div>
    <footer>
      {#if footer}
        {@render footer({ close })}
      {:else}
        {#if cancelLabel !== null}
          <Button onclick={cancel} disabled={busy}>{cancelLabel ?? t('common.cancel')}</Button>
        {/if}
        <Button variant={okVariant} onclick={ok} loading={busy} disabled={okDisabled}>{okLabel ?? t('common.ok')}</Button>
      {/if}
    </footer>
  {/if}
</dialog>

<style>
  .dlg {
    max-height: calc(100vh - 48px);
    padding: 0;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-lg);
    background: var(--color-surface-raised);
    color: var(--color-text);
    box-shadow: var(--shadow-3);
    overflow: hidden;
    outline: none;
  }
  .dlg[open] {
    display: flex;
    flex-direction: column;
    animation: pop var(--dur) var(--ease);
  }
  .dlg::backdrop {
    background: var(--color-scrim);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    padding: var(--sp-4) var(--sp-4) var(--sp-2) var(--sp-5);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-lg);
    font-weight: var(--fw-bold);
    line-height: var(--lh-tight);
  }
  .body {
    padding: var(--sp-1) var(--sp-5) var(--sp-4);
    overflow: auto;
    color: var(--color-text-2);
  }
  /* 서버·Go 문구의 \n 을 줄바꿈으로 보일 때 <p class="dialog-text"> 로 감싼다 */
  .body :global(.dialog-text) {
    margin: 0;
    white-space: pre-line;
    overflow-wrap: anywhere;
  }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--sp-2);
    padding: var(--sp-3) var(--sp-4);
    border-top: 1px solid var(--color-border-subtle);
    background: var(--color-surface-sunken);
  }
  footer :global(.btn) {
    min-width: 80px;
  }
  @keyframes pop {
    from {
      opacity: 0;
      transform: translateY(4px) scale(0.98);
    }
  }
</style>
