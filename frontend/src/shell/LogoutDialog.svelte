<!--
  「로그아웃 처리 방식 확인」 창(원본 LogoutActionDlg.qml). 라디오는 누르는 즉시 설정 logoutType 에 저장하고,
  「다음부터 보이지 않기」 는 logoutConfirm 을 끈다. [확인] 이면 logoutProcess(고른 방식대로), [취소]·Esc 면 아무것도 안 한다.
-->
<script lang="ts">
  import { Checkbox, Dialog, RadioGroup } from '../lib/ui'
  import { t } from '../lib/i18n'
  import { logoutAsk, logoutProcess } from '../lib/stores/session'
  import { settings, updateSettings } from '../lib/stores/ui'

  let type = $state(0)
  let dontAsk = $state(false)

  $effect(() => {
    if ($logoutAsk) {
      type = $settings?.logoutType ?? 0
      dontAsk = !($settings?.logoutConfirm ?? true)
    }
  })

  const options = $derived([0, 1, 2].map((v) => ({ value: v, label: t(`common.logoutType.${v}`) })))
</script>

<Dialog
  bind:open={$logoutAsk}
  title={t('common.shell.logoutTitle')}
  width={400}
  okLabel={t('common.ok')}
  cancelLabel={t('common.cancel')}
  onOk={() => {
    // 닫히고 나서 처리한다(종료·숨김 전에 창이 닫혀 있어야 다음에 다시 열린다)
    const chosen = type
    setTimeout(() => void logoutProcess(chosen), 0)
  }}
>
  <div class="body" id="logoutActionDlg">
    <RadioGroup
      label={t('common.shell.logoutGroup')}
      {options}
      bind:value={type}
      onchange={(v) => void updateSettings((s) => (s.logoutType = v))}
    />
    <Checkbox
      label={t('common.shell.logoutDontAsk')}
      bind:checked={dontAsk}
      onchange={(c) => void updateSettings((s) => (s.logoutConfirm = !c))}
    />
  </div>
</Dialog>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--sp-4);
  }
</style>
