<!--
  클립보드 보내기·받기 단추와 받을 건수 배지. 어디에 끼워도 혼자 돈다(props 없음, size 만 고를 수 있다).
  건수는 ClipService.Count 와 clip:count 사건으로 받는다. 안내 창은 Go 가 사건으로 내므로 여기서는 호출 오류만 띄운다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy'
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import { ClipService, Ev, errorText, on } from '../../lib/api'
  import { t } from '../../lib/i18n'
  import { toast } from '../../lib/stores/ui'
  import { Badge, Button } from '../../lib/ui'

  let { size = 'md' }: { size?: 'sm' | 'md' | 'lg' } = $props()

  let count = $state(0)
  let sending = $state(false)
  let receiving = $state(false)

  onMount(() => {
    ClipService.Count()
      .then((n) => (count = n))
      .catch(() => {})
    return on<{ count: number }>(Ev.clipCount, (d) => (count = d.count))
  })

  async function send() {
    sending = true
    try {
      await ClipService.Upload()
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      sending = false
    }
  }

  async function receive() {
    receiving = true
    try {
      await ClipService.Download()
    } catch (e) {
      toast(errorText(e), 3)
    } finally {
      receiving = false
    }
  }
</script>

<Button {size} icon={ClipboardCopy} loading={sending} onclick={send}>{t('clip.upload')}</Button>
<Button {size} icon={ClipboardPaste} loading={receiving} onclick={receive} aria-label={count > 0 ? t('clip.download.count', { count }) : t('clip.download')}>
  {t('clip.download')}
  {#if count > 0}<Badge tone="accent" size="sm" label={String(count)} title={t('clip.download.count', { count })} />{/if}
</Button>
