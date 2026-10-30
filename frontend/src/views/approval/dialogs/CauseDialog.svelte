<!--
  [승인 사유]/[반려 사유] 창(원본 content/ApprovalDlg.qml, docs/port/06 5.3).
  콤보 마지막 `직접입력` 일 때만 입력란이 열린다(최대 100자). 열 때마다 첫 항목으로 되돌린다(원본은 앞 값이 남았다).
  onSubmit(사유) 가 끝나면 닫힌다. 빈 사유는 「결재 사유를 입력해 주세요.」
-->
<script lang="ts">
  import { untrack } from 'svelte'
  import { Dialog, Input, Select } from '../../../lib/ui'
  import { t } from '../../../lib/i18n'

  let {
    open = $bindable(false),
    approve,
    count,
    onSubmit,
  }: { open?: boolean; approve: boolean; count: number; onSubmit: (cause: string) => Promise<void> } = $props()

  const keys = $derived(approve ? ['reason.a1', 'reason.a2', 'reason.a3'] : ['reason.r1', 'reason.r2', 'reason.r3'])
  const DIRECT = 3
  let idx = $state(0)
  let text = $state('')
  let error = $state('')

  $effect(() => {
    if (open)
      untrack(() => {
        idx = 0
        text = ''
        error = ''
      })
  })

  const options = $derived([...keys.map((k, i) => ({ value: i, label: t(`approval.${k}`) })), { value: DIRECT, label: t('approval.cause.direct') }])

  async function ok() {
    const cause = (idx === DIRECT ? text : t(`approval.${keys[idx]}`)).trim()
    if (!cause) {
      error = t('approval.reason.empty')
      return false
    }
    await onSubmit(cause)
  }
</script>

<Dialog
  bind:open
  title={approve ? t('approval.reason.approveTitle') : t('approval.reason.rejectTitle')}
  width={400}
  okLabel={approve ? t('approval.approve.approve') : t('approval.approve.reject')}
  okVariant={approve ? 'primary' : 'danger'}
  onOk={ok}
>
  <div class="form">
    <p class="count">{t('approval.reason.count', { n: count })}</p>
    <Select size="sm" bind:value={idx} options={options} label={t('approval.reason.label')} onchange={() => (error = '')} />
    <Input
      size="sm"
      bind:value={text}
      disabled={idx !== DIRECT}
      maxlength={100}
      placeholder={t('approval.reason.placeholder')}
      aria-label={t('approval.reason.label')}
      error={error || undefined}
      oninput={() => (error = '')}
    />
  </div>
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .count {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--color-text-3);
  }
</style>
