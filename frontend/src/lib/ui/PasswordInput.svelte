<!--
  비밀번호 칸. 눈 단추로 보이기·숨기기, Caps Lock 이 켜져 있으면 아래에 알린다.
  값은 화면 상태에만 두고 저장·로그 하지 않는다(쓰는 쪽이 성공 뒤 비운다).
-->
<script lang="ts">
  import type { ComponentProps } from 'svelte'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import Input from './Input.svelte'
  import IconButton from './IconButton.svelte'
  import { t } from '../i18n'

  type Props = Omit<ComponentProps<typeof Input>, 'type' | 'end'>

  let { value = $bindable(''), input = $bindable(), hint, ...rest }: Props = $props()
  let shown = $state(false)
  let caps = $state(false)

  function onkey(e: KeyboardEvent) {
    caps = e.getModifierState?.('CapsLock') ?? false
  }
</script>

<Input
  {...rest}
  bind:value
  bind:input
  type={shown ? 'text' : 'password'}
  autocomplete={rest.autocomplete ?? 'current-password'}
  spellcheck={false}
  hint={caps ? t('common.capsLock') : hint}
  onkeydown={onkey}
  onkeyup={onkey}
>
  {#snippet end()}
    <IconButton
      size="sm"
      icon={shown ? EyeOff : Eye}
      label={shown ? t('common.hidePassword') : t('common.showPassword')}
      pressed={shown}
      onclick={() => (shown = !shown)}
      tabindex={-1}
    />
  {/snippet}
</Input>
