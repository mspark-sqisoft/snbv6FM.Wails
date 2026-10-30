<!--
  글자 입력칸. label 을 주면 칸 위에 보이는 이름, 없으면 aria-label 을 꼭 넘긴다.
  error 가 있으면 빨간 테두리와 아래 문구(aria-invalid·aria-describedby), 없으면 hint 를 보인다.
  icon 은 왼쪽 안쪽 아이콘, end 는 오른쪽 안쪽에 넣을 것(PasswordInput 의 눈 단추 등).
-->
<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLInputAttributes } from 'svelte/elements'
  import type { Icon } from './types'

  type Props = Omit<HTMLInputAttributes, 'size' | 'value'> & {
    value?: string
    label?: string
    hint?: string
    error?: string
    size?: 'sm' | 'md' | 'lg'
    icon?: Icon
    end?: Snippet
    input?: HTMLInputElement
  }

  let {
    value = $bindable(''),
    input = $bindable(),
    label,
    hint,
    error,
    size = 'md',
    icon: IconCmp,
    end,
    id,
    type = 'text',
    ...rest
  }: Props = $props()

  const autoId = $props.id()
  const inputId = $derived(id ?? autoId)
  const noteId = $derived(`${inputId}-note`)
</script>

<div class="field">
  {#if label}<label class="label" for={inputId}>{label}</label>{/if}
  <div class="box {size}" class:invalid={!!error} class:disabled={rest.disabled}>
    {#if IconCmp}<span class="lead"><IconCmp size={16} aria-hidden="true" /></span>{/if}
    <input
      {...rest}
      {type}
      id={inputId}
      bind:value
      bind:this={input}
      aria-invalid={error ? true : undefined}
      aria-describedby={error || hint ? noteId : undefined}
    />
    {#if end}<span class="end">{@render end()}</span>{/if}
  </div>
  {#if error}
    <div class="note error" id={noteId}>{error}</div>
  {:else if hint}
    <div class="note" id={noteId}>{hint}</div>
  {/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    min-width: 0;
  }
  .label {
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--color-text-2);
  }
  .box {
    display: flex;
    align-items: center;
    height: var(--h-md);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    transition: border-color var(--dur-fast) var(--ease);
  }
  .box.sm {
    height: var(--h-sm);
  }
  .box.lg {
    height: var(--h-lg);
  }
  .box:hover:not(.disabled) {
    border-color: var(--color-text-3);
  }
  .box:focus-within {
    border-color: var(--color-focus);
    box-shadow: 0 0 0 1px var(--color-focus);
  }
  .box.invalid {
    border-color: var(--color-danger);
    box-shadow: 0 0 0 1px var(--color-danger);
  }
  .box.disabled {
    opacity: 0.5;
  }
  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0 var(--sp-3);
    border: 0;
    background: transparent;
    outline: none;
    font-size: var(--fs-body);
  }
  .lg input {
    font-size: var(--fs-md);
  }
  .lead + input {
    padding-left: var(--sp-2);
  }
  input::placeholder {
    color: var(--color-text-3);
  }
  input:read-only {
    color: var(--color-text-2);
  }
  .lead {
    display: flex;
    padding-left: var(--sp-3);
    color: var(--color-text-3);
  }
  .end {
    display: flex;
    align-items: center;
    padding-right: var(--sp-1);
  }
  .note {
    font-size: var(--fs-sm);
    color: var(--color-text-3);
  }
  .note.error {
    color: var(--color-danger);
  }
</style>
