<!-- 켜기·끄기 스위치(role="switch"). label 이 없으면 aria-label 을 꼭 넘긴다. 바뀌면 onchange(checked). -->
<script lang="ts">
  let {
    checked = $bindable(false),
    disabled = false,
    label,
    onchange,
    'aria-label': ariaLabel,
  }: {
    checked?: boolean
    disabled?: boolean
    label?: string
    onchange?: (checked: boolean) => void
    'aria-label'?: string
  } = $props()

  function toggle() {
    checked = !checked
    onchange?.(checked)
  }
</script>

<button
  type="button"
  class="sw"
  role="switch"
  aria-checked={checked}
  aria-label={label ? undefined : ariaLabel}
  {disabled}
  onclick={toggle}
>
  <span class="track" aria-hidden="true"><span class="thumb"></span></span>
  {#if label}<span class="text">{label}</span>{/if}
</button>

<style>
  .sw {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
    font-size: var(--fs-body);
  }
  .sw:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .track {
    position: relative;
    width: 32px;
    height: 18px;
    border-radius: var(--r-full);
    background: var(--color-border-strong);
    transition: background-color var(--dur) var(--ease);
  }
  .thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--color-text-on-accent); /* 두 테마 모두 흰색 */
    box-shadow: var(--shadow-1);
    transition: transform var(--dur) var(--ease);
  }
  [aria-checked='true'] .track {
    background: var(--color-accent-fill);
  }
  [aria-checked='true'] .thumb {
    transform: translateX(14px);
  }
</style>
