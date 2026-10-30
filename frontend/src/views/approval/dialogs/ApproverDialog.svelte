<!--
  [결재자 선택] 창(원본 content/ApproverSelectDlg.qml, docs/port/06 5.1).
  Go 가 approval:select 로 연 창이다. [결재요청] 은 ApproverReply(gen, ids, 사유, false), 어느 길로 닫히든(취소·X·Esc·바깥) 답하지
  않았으면 ApproverReply(gen, [], '', true) 로 취소한다. Go 가 판정 오류(자기 지정 등)를 돌려주면 창을 닫지 않고 창 안에 보인다.
-->
<script lang="ts">
  import { untrack } from 'svelte'
  import { Button, Checkbox, DataTable, Dialog, Input, Select, formatBytes, type Column } from '../../../lib/ui'
  import { t } from '../../../lib/i18n'
  import { ApprovalService, errorText, type ApproverUser } from '../../../lib/api'
  import type { ApproverDept, SelectAsk } from '../types'

  let { ask, open = $bindable(false) }: { ask: SelectAsk; open?: boolean } = $props()

  const CAUSES = ['cause.direct', 'cause.request', 'cause.business', 'cause.customer']
  let picked = $state(new Set<string>())
  let causeIdx = $state(0)
  let causeText = $state('')
  let error = $state('')
  let sending = $state(false)
  let replied = false

  // 새 창마다 처음 상태로(원본은 앞 값이 남았다)
  $effect(() => {
    void ask.gen
    untrack(() => {
      picked = new Set()
      causeIdx = 0
      causeText = ''
      error = ''
      sending = false
      replied = false
    })
  })

  const causeOptions = $derived(CAUSES.map((k, i) => ({ value: i, label: t(`approval.${k}`) })))
  const typeOptions = $derived((ask.typeOptions ?? []).map((label, i) => ({ value: i, label })))
  const files = $derived(
    (ask.files ?? []).map((f, i) => ({ no: i + 1, name: ask.clip ? t('approval.approve.clip') : f.name, size: f.size })),
  )
  type FileLine = (typeof files)[number]
  const fileColumns: Column<FileLine>[] = $derived([
    { key: 'no', title: t('approval.col.no'), width: 44, align: 'right', sortable: false },
    { key: 'name', title: t('approval.col.fileName'), width: 280, grow: true, sortable: false },
    { key: 'size', title: t('approval.col.fileSize'), width: 96, align: 'right', sortable: false, value: (r) => formatBytes(r.size) },
  ])

  function toggle(id: string, on: boolean) {
    const next = new Set(picked)
    if (on) next.add(id)
    else next.delete(id)
    picked = next
    error = ''
  }

  function onCause() {
    if (causeIdx !== 0) causeText = '' // 직접입력이 아니면 입력란을 잠그고 비운다
    error = ''
  }

  async function submit() {
    if (sending) return
    if (!picked.size) return void (error = t('approval.select.noApprover'))
    const cause = (causeIdx === 0 ? causeText : t(`approval.${CAUSES[causeIdx]}`)).trim()
    if (!cause) return void (error = t('approval.select.noCause'))
    if (!ask.selfApprove && picked.has(ask.myId)) return void (error = t('approval.select.self'))
    sending = true
    try {
      await ApprovalService.ApproverReply(ask.gen, [...picked], cause, false)
      replied = true
      open = false
    } catch (e) {
      error = errorText(e) // Go 판정(창 안에서 다시 고르게)
    } finally {
      sending = false
    }
  }

  function onClose() {
    if (replied) return
    replied = true
    void ApprovalService.ApproverReply(ask.gen, [], '', true).catch(() => {})
  }
</script>

{#snippet dept(d: ApproverDept)}
  <details open class="dept">
    <summary>{d.name}</summary>
    <ul>
      {#each d.users ?? [] as u (u.id)}
        {@render user(u)}
      {/each}
    </ul>
    {#each d.children ?? [] as c (c.id)}
      <div class="child">{@render dept(c)}</div>
    {/each}
  </details>
{/snippet}

{#snippet user(u: ApproverUser)}
  <li>
    <Checkbox
      checked={picked.has(u.id)}
      aria-label="{u.label} ({u.id})"
      title="{u.dept} · {u.id}"
      onchange={(on) => toggle(u.id, on)}
    >
      <span class="uname">{u.label}</span>
      {#if u.id === ask.myId}<span class="me">{t('approval.select.selfMark')}</span>{/if}
    </Checkbox>
  </li>
{/snippet}

<Dialog bind:open title={t('approval.select.title')} width={560} closeOnBackdrop={false} {onClose}>
  <div class="form">
    <dl class="grid">
      <dt>{t('approval.select.requester')}</dt>
      <dd>{ask.reqDept} {ask.reqName}</dd>
      <dt>{t('approval.select.method')}</dt>
      <dd>
        <Select size="sm" value={ask.aprvType} options={typeOptions} disabled aria-label={t('approval.select.method')} />
      </dd>
      <dt>{t('approval.select.fileInfo')}</dt>
      <dd class="ellipsis" title={ask.fileDesc}>{ask.fileDesc}</dd>
      <dt>{t('approval.select.direction')}</dt>
      <dd>{ask.direction}</dd>
      <dt>{t('approval.select.fileSize')}</dt>
      <dd class="num">{formatBytes(ask.totalSize)}</dd>
    </dl>

    <section aria-label={t('approval.select.approvers')}>
      <div class="head">
        <h3>{t('approval.select.approvers')}</h3>
        <span class="count">{t('approval.select.picked', { n: picked.size })}</span>
      </div>
      <div class="tree" role="group" aria-label={t('approval.select.tree')}>
        {#each ask.tree ?? [] as d (d.id)}
          {@render dept(d)}
        {/each}
      </div>
    </section>

    <div class="cause">
      <Select size="sm" bind:value={causeIdx} options={causeOptions} label={t('approval.select.cause')} onchange={onCause} />
      <Input
        size="sm"
        bind:value={causeText}
        disabled={causeIdx !== 0}
        maxlength={100}
        placeholder={t('approval.select.causePlaceholder')}
        aria-label={t('approval.select.cause')}
        oninput={() => (error = '')}
      />
    </div>

    <div class="files">
      <DataTable label={t('approval.select.filesTable')} rows={files} columns={fileColumns} rowKey={(r) => String(r.no)} selectable={false} rowHeight={26} />
    </div>

    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </div>

  {#snippet footer({ close })}
    <Button onclick={close} disabled={sending}>{t('common.cancel')}</Button>
    <Button id="approverRequestButton" variant="primary" loading={sending} onclick={submit}>{t('approval.select.request')}</Button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    color: var(--color-text);
  }
  .grid {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--sp-1) var(--sp-3);
    align-items: center;
    margin: 0;
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--color-text-3);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-bottom: var(--sp-1);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-sm);
    font-weight: var(--fw-medium);
  }
  .count {
    font-size: var(--fs-sm);
    color: var(--color-text-3);
  }
  .tree {
    max-height: 220px;
    overflow: auto;
    padding: var(--sp-2);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    background: var(--color-surface-sunken);
  }
  .dept summary {
    cursor: pointer;
    font-weight: var(--fw-medium);
    font-size: var(--fs-sm);
    padding: 2px 0;
  }
  .dept ul {
    list-style: none;
    margin: 0;
    padding: 0 0 0 var(--sp-4);
  }
  .dept li {
    padding: 2px 0;
  }
  .child {
    padding-left: var(--sp-4);
  }
  .uname {
    white-space: pre; /* 서버 글자를 그대로(서식 해석 없음) */
  }
  .me {
    margin-left: var(--sp-1);
    font-size: var(--fs-sm);
    color: var(--color-text-3);
  }
  .cause {
    display: grid;
    grid-template-columns: 180px 1fr;
    gap: var(--sp-2);
    align-items: end;
  }
  .files {
    height: 120px;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .error {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--color-danger-border);
    border-radius: var(--r-md);
    background: var(--color-danger-subtle);
    color: var(--color-danger);
    font-size: var(--fs-sm);
    white-space: pre-line;
  }
  .ellipsis {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .num {
    font-variant-numeric: tabular-nums;
  }
</style>
