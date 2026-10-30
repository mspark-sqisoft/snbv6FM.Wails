<!--
  [압축 비밀번호 입력] 창(원본 HomePage.qml 의 zipPasswordDialog). 암호 압축을 검사할 때 Go 가 policy:zip-password 로 연다.
  두 번째 질문(retry)이면 "비밀번호가 틀렸습니다. 다시 입력하세요." 를 보인다(Go 가 두 번까지 묻는다).
  [확인] 은 ZipPasswordReply(gen, 비밀번호), 어느 길로 닫히든 답하지 않았으면 ZipPasswordReply(gen, '') 로 취소한다 ―
  검사가 이 답을 기다리므로 안 하면 검사가 끝나지 않는다. 비밀번호는 이 창 상태에만 두고 답한 뒤 비운다.
-->
<script lang="ts">
  import { Dialog, PasswordInput } from '../../../lib/ui'
  import { t } from '../../../lib/i18n'
  import { PolicyService } from '../../../lib/api'
  import type { ZipAsk } from '../types'

  let { ask, open = $bindable(false) }: { ask: ZipAsk; open?: boolean } = $props()

  let password = $state('')
  let input = $state<HTMLInputElement>()
  let replied = false

  $effect(() => {
    if (open) input?.focus()
  })

  function answer(pw: string) {
    if (replied) return
    replied = true
    password = ''
    void PolicyService.ZipPasswordReply(ask.gen, pw).catch(() => {})
  }

  function submit() {
    if (!password) return false
    answer(password)
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.isComposing && password) {
      e.preventDefault()
      answer(password)
      open = false
    }
  }
</script>

<Dialog
  bind:open
  title={t('policy.zip.title')}
  width={400}
  okLabel={t('policy.zip.ok')}
  cancelLabel={t('policy.zip.cancel')}
  okDisabled={!password}
  dismissible={false}
  onOk={submit}
  onClose={() => answer('')}
>
  <div class="body" id="zipPasswordDialog" role="group" aria-label={t('policy.zip.title')}>
    <p id="zipPasswordPrompt" class:retry={ask.retry} role={ask.retry ? 'alert' : undefined}>
      {ask.retry ? t('policy.zip.retry') : t('policy.zip.prompt')}
    </p>
    <p id="zipPasswordFileName" class="name" title={ask.name}>{ask.name}</p>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div {onkeydown}>
      <PasswordInput
        id="zipPasswordInput"
        bind:value={password}
        bind:input
        placeholder={t('policy.zip.placeholder')}
        aria-label={t('policy.zip.placeholder')}
        autocomplete="off"
      />
    </div>
  </div>
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    color: var(--color-text);
  }
  p {
    margin: 0;
    font-size: var(--fs-body);
  }
  .retry {
    color: var(--color-danger);
  }
  .name {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--color-text-2);
    font-size: var(--fs-sm);
  }
</style>
