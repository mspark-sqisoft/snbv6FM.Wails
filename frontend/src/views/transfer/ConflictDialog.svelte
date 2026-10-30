<!--
  파일 중복 확인 창(원본 DOWNLOAD_DUPLICATE, docs/port/04 5.x). 받을 폴더에 같은 이름이 있으면 Go 가 transfer:conflict 로 묻는다.
  답할 때까지 닫히지 않는다(Esc·바깥 누름 무시). [덮어쓰기](폴더는 병합)·[이름 변경](_00000)·[취소](그 파일만 뺀다), [모두 적용].
  답을 기다리는 동안 온 다른 받기 요청은 Go 가 이 질문 뒤에 차례로 묻는다. 로그아웃이면 transfer:conflict-close 로 닫힌다.
-->
<script lang="ts">
  import { Button, Checkbox, Dialog } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import { TransferService, errorText } from '../../lib/api'
  import { toast } from '../../lib/stores/ui'
  import type { ConflictAsk } from './store'

  let { ask = $bindable(null) }: { ask: ConflictAsk | null } = $props()

  let open = $state(false)
  let all = $state(false)
  $effect(() => {
    open = !!ask
    if (ask) all = false // 질문마다 꺼짐
  })

  async function reply(choice: 0 | 1 | 2) {
    const a = ask
    if (!a) return
    ask = null
    try {
      await TransferService.ConflictReply(a.gen, choice, all)
    } catch (e) {
      toast(errorText(e), 3)
    }
  }
</script>

<Dialog bind:open title={t('transfer.conflict.title')} width={460} dismissible={false}>
  <p class="text">{t('transfer.conflict.text')}</p>
  {#if ask}
    <p class="name" title={ask.path} data-testid="conflictName">{ask.name}</p>
    {#if ask.folder}<p class="hint">{t('transfer.conflict.folderHint')}</p>{/if}
    {#if ask.total > 1}<p class="hint">{t('transfer.conflict.count', { i: ask.index, n: ask.total })}</p>{/if}
  {/if}
  {#snippet footer()}
    <div class="foot">
      <Checkbox bind:checked={all} label={t('transfer.conflict.all')} />
      <div class="btns">
        <Button variant="primary" onclick={() => reply(0)}>{t('transfer.conflict.overwrite')}</Button>
        <Button onclick={() => reply(1)}>{t('transfer.conflict.rename')}</Button>
        <Button variant="ghost" onclick={() => reply(2)}>{t('transfer.conflict.cancel')}</Button>
      </div>
    </div>
  {/snippet}
</Dialog>

<style>
  .text {
    margin: 0 0 var(--sp-2);
  }
  .name {
    margin: 0;
    font-weight: var(--fw-semibold);
    word-break: break-all;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .hint {
    margin: var(--sp-2) 0 0;
    font-size: var(--fs-sm);
    color: var(--color-text-2);
  }
  .foot {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    width: 100%;
  }
  .btns {
    margin-left: auto;
    display: flex;
    gap: var(--sp-2);
  }
</style>
