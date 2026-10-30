<!--
  라디오 묶음(운영체제 기본 input[type=radio] ― 화살표로 옮기고 Tab 은 묶음 하나로 지난다).
  - value(bind): 고른 값. options: Option[]. label: 묶음 이름(fieldset legend, 화면 읽기 프로그램에 읽힌다).
  - onchange(value): 사용자가 고를 때만 부른다(처음 그릴 때는 부르지 않는다).
  - direction: 'column'(기본) · 'row'.
-->
<script lang="ts" generics="V extends string | number">
  import type { Option } from './types'

  let {
    value = $bindable(),
    options,
    label,
    name,
    disabled = false,
    direction = 'column',
    onchange,
  }: {
    value?: V
    options: Option<V>[]
    label: string
    name?: string
    disabled?: boolean
    direction?: 'column' | 'row'
    onchange?: (value: V) => void
  } = $props()

  const auto = $props.id()
  const groupName = $derived(name ?? auto)
</script>

<fieldset class="group" class:row={direction === 'row'} {disabled}>
  <legend class="label">{label}</legend>
  {#each options as o (o.value)}
    <label class="opt" class:off={disabled || o.disabled}>
      <input
        type="radio"
        name={groupName}
        value={o.value}
        checked={value === o.value}
        disabled={disabled || o.disabled}
        onchange={() => {
          value = o.value
          onchange?.(o.value)
        }}
      />
      <span>{o.label}</span>
    </label>
  {/each}
</fieldset>

<style>
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  .group.row {
    flex-direction: row;
    flex-wrap: wrap;
    gap: var(--sp-4);
  }
  .label {
    padding: 0;
    margin-bottom: var(--sp-1);
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
    color: var(--color-text-2);
  }
  .opt {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    color: var(--color-text);
    font-size: var(--fs-body);
    cursor: pointer;
  }
  .opt.off {
    color: var(--color-text-disabled);
    cursor: default;
  }
  input {
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--color-accent-fill);
    cursor: inherit;
  }
  input:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
</style>
