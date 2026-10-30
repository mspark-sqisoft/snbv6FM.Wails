<!-- 디자인 고르기 카드 묶음. 키보드는 라디오 묶음 그대로 화살표로 옮긴다. -->
<script lang="ts">
  import { t } from '../../lib/i18n'
  import { design, setDesign } from '../../lib/stores/ui'
  import { designs } from '../../designs/registry'
</script>

<div class="picker" role="radiogroup" aria-label={t('design.card')}>
  {#each designs as d (d.id)}
    <label class="item" class:on={$design === d.id}>
      <input type="radio" name="design" value={d.id} checked={$design === d.id} onchange={() => setDesign(d.id)} />
      <span class="swatch" aria-hidden="true">
        {#each d.swatch as c, i (i)}<i style:background={c}></i>{/each}
      </span>
      <b>{t(`design.${d.id}.name`)}</b>
      <span class="desc">{t(`design.${d.id}.desc`)}</span>
    </label>
  {/each}
</div>

<style>
  .picker {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(165px, 1fr));
    gap: var(--sp-3);
  }
  .item {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    padding: var(--sp-3);
    border: 1px solid var(--color-border);
    border-radius: var(--r-md);
    background: var(--color-surface-raised);
    cursor: pointer;
  }
  .item:hover {
    border-color: var(--color-border-strong);
  }
  .item.on {
    border-color: var(--color-accent-fill);
    box-shadow: 0 0 0 1px var(--color-accent-fill);
  }
  .item:has(input:focus-visible) {
    outline: 2px solid var(--color-focus);
    outline-offset: 2px;
  }
  input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }
  .swatch {
    display: flex;
    height: 28px;
    margin-bottom: var(--sp-1);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-sm);
    overflow: hidden;
  }
  .swatch i {
    flex: 1;
  }
  b {
    font-weight: var(--fw-semibold);
  }
  .desc {
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
</style>
