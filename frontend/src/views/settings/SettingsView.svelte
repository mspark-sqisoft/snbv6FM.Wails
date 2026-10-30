<!--
  [설정] 화면(원본 SettingsScrollView.qml, docs/port/12 5.8). 카드: 기본 설정 · 로그아웃 처리 · 시작 폴더 지정 · 자동 업로드 설정 ·
  자동 다운로드 설정 · 클립보드 단축키 설정(Linux 숨김) · 프로그램 정보. 값은 고르는 즉시 저장한다(서버를 부르지 않는다 ― SET-091).
  원본과 다르게 고친 것: 화면을 열기만 해서는 아무 값도 다시 저장하지 않는다(배치 1 → 0, 주기 → 1 로 몰래 바꾸던 것, 단축키 재등록).
  자동 전송 값은 망마다(설정 auto[망 자리]) ― 기본은 로그인한 망, 위 콤보로 다른 망도 고친다.
-->
<script lang="ts">
  import { onMount } from 'svelte'
  import FolderSearch from '@lucide/svelte/icons/folder-search'
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw'
  import { Button, IconButton, Input, Page, RadioGroup, Select, Switch, type Option } from '../../lib/ui'
  import { t } from '../../lib/i18n'
  import {
    AuthService,
    ClipService,
    LocalService,
    LogService,
    SettingsService,
    errorText,
    type AppInfo,
    type AutoTransfer,
    type Hotkey,
    type LocalStart,
    type Server,
    type Settings,
  } from '../../lib/api'
  import { session } from '../../lib/stores/session'
  import { design, setLanguage, setTheme, settings, toast, updateSettings, type Theme } from '../../lib/stores/ui'
  import { designOf } from '../../designs/registry'
  import Logo from '../../shell/Logo.svelte'
  import DesignPicker from './DesignPicker.svelte'

  const PERIODS = [1, 5, 10, 15, 20, 30, 60, 90, 120]
  const DEFAULT_PERIOD = 30 // Go config.DefaultUploadPeriod
  const MODS = ['ALT', 'SHIFT', 'ALT+SHIFT', 'CTRL+SHIFT']

  let info = $state<AppInfo | null>(null)
  let servers = $state<Server[]>([])
  let defaultStart = $state<LocalStart | null>(null)
  let logOn = $state<boolean | null>(null) // 실제 로그 저장 상태(로그 폴더 유무). null = 모름 → 설정값
  let net = $state(0) // 자동 전송을 고칠 망 자리
  // Switch 는 누르면 제 값을 먼저 바꾼다. 거절할 때 값이 그대로면 되돌아가지 않으므로 이 번호를 올려 스위치를 다시 그린다.
  let rev = $state(0)

  const s = $derived($settings)
  const look = $derived(designOf($design))
  const auto = $derived<AutoTransfer>({
    uploadUse: false,
    uploadPeriod: 0,
    uploadFolder: '',
    downloadUse: false,
    downloadFolder: '',
    ...(s?.auto?.[`${net}`] ?? {}),
  })
  const hk = $derived<Hotkey>(s?.hotkey ?? { use: false, copyKey: 'ALT', copyChar: 'C', pasteKey: 'ALT', pasteChar: 'V' })

  // ── 선택지 ──
  const themeOptions = $derived<Option<string>[]>(
    (['system', 'light', 'dark'] as const).map((v) => ({ value: v, label: t(`common.theme.${v}`) })),
  )
  const layoutOptions = $derived<Option<number>[]>([0, 2].map((v) => ({ value: v, label: t(`settings.layout.${v}`) })))
  const langOptions = $derived<Option<number>[]>([
    { value: 0, label: t('common.lang.ko') },
    { value: 1, label: t('common.lang.en') },
  ])
  const logoutOptions = $derived<Option<number>[]>([0, 1, 2].map((v) => ({ value: v, label: t(`common.logoutType.${v}`) })))
  const periodOptions: Option<number>[] = PERIODS.map((v) => ({ value: v, label: String(v) }))
  const modOptions: Option<string>[] = MODS.map((v) => ({ value: v, label: v }))
  const hotkeyUseOptions = $derived<Option<string>[]>([
    { value: 'on', label: t('settings.hotkeyUse.on') },
    { value: 'off', label: t('settings.hotkeyUse.off') },
  ])
  const netOptions = $derived<Option<number>[]>(servers.map((v) => ({ value: v.index, label: v.name })))

  /** select 의 change 에서 고른 선택지 값(숫자 그대로). */
  function picked<V>(options: Option<V>[], e: Event): V {
    return options[(e.currentTarget as HTMLSelectElement).selectedIndex].value
  }

  const startHint = $derived.by(() => {
    if (!defaultStart) return ''
    const key = defaultStart.kind === 'desktop' ? 'startDesktop' : defaultStart.kind === 'home' ? 'startHome' : 'startRoot'
    return t(`settings.${key}`, { path: defaultStart.path })
  })

  const save = (fn: (x: Settings) => void) => updateSettings(fn)

  function saveAuto(fn: (a: AutoTransfer) => void) {
    const idx = net
    return save((x) => {
      const cur: AutoTransfer = {
        uploadUse: false,
        uploadPeriod: 0,
        uploadFolder: '',
        downloadUse: false,
        downloadFolder: '',
        ...(x.auto?.[`${idx}`] ?? {}),
      }
      fn(cur)
      x.auto = { ...(x.auto ?? {}), [`${idx}`]: cur }
    })
  }

  // ── 동작 ──
  async function setLogFile(use: boolean) {
    try {
      await LogService.SetLogFile(use) // 먼저 폴더를 만들거나 지운다(원본 setLogFileUse) ― 실패하면 값을 바꾸지 않는다
      logOn = use
      await save((x) => (x.useLogFile = use))
    } catch (e) {
      logOn = !use
      rev++
      toast(`${t('settings.logFileFailed')} ${errorText(e)}`, 3)
    }
  }

  async function pickFolder(current: string): Promise<string | null> {
    try {
      const p = await SettingsService.PickFolder(current)
      return p || null
    } catch (e) {
      toast(errorText(e), 3)
      return null
    }
  }

  async function checkFolder(p: string): Promise<boolean> {
    try {
      await SettingsService.CheckFolder(p)
      return true
    } catch (e) {
      toast(errorText(e), 3)
      return false
    }
  }

  async function chooseStart() {
    const p = await pickFolder(s?.initFolder ?? '')
    if (p) await save((x) => (x.initFolder = p))
  }

  async function chooseAuto(kind: 'upload' | 'download') {
    const cur = kind === 'upload' ? auto.uploadFolder : auto.downloadFolder
    const p = await pickFolder(cur)
    if (!p || !(await checkFolder(p))) return
    await saveAuto((a) => (kind === 'upload' ? (a.uploadFolder = p) : (a.downloadFolder = p)))
  }

  async function setAutoUse(kind: 'upload' | 'download', on: boolean) {
    const folder = kind === 'upload' ? auto.uploadFolder : auto.downloadFolder
    if (on && (!folder || !(await checkFolder(folder)))) {
      if (!folder) toast(t('settings.folderNeeded'), 2)
      rev++ // 스위치를 되돌린다(저장값은 그대로)
      return
    }
    await saveAuto((a) => {
      if (kind === 'upload') {
        a.uploadUse = on
        if (!a.uploadPeriod) a.uploadPeriod = DEFAULT_PERIOD
      } else a.downloadUse = on
    })
  }

  async function saveHotkey(fn: (h: Hotkey) => void) {
    let next: Hotkey | null = null
    await save((x) => {
      fn(x.hotkey)
      next = { ...x.hotkey }
    })
    if (!next) return
    try {
      await ClipService.SetHotkey(next) // 값이 바뀔 때만 다시 등록한다(원본은 화면을 열 때마다)
    } catch (e) {
      const msg = errorText(e)
      if (msg !== t('common.error.notImplemented')) toast(`${t('settings.hotkeyApplyFailed')} ${msg}`, 3)
    }
  }

  function hotkeyChar(e: Event, which: 'copy' | 'paste') {
    const el = e.currentTarget as HTMLInputElement
    const v = el.value.trim().toUpperCase()
    if (!/^[A-Z0-9]$/.test(v)) {
      toast(t('settings.hotkeyCharBad'), 2)
      el.value = which === 'copy' ? hk.copyChar : hk.pasteChar
      return
    }
    el.value = v
    void saveHotkey((h) => (which === 'copy' ? (h.copyChar = v) : (h.pasteChar = v)))
  }

  onMount(() => {
    SettingsService.AppInfo()
      .then((v) => (info = v))
      .catch(() => {})
    LocalService.DefaultFolder()
      .then((v) => (defaultStart = v))
      .catch(() => {})
    LogService.LogFileEnabled()
      .then((v) => (logOn = v))
      .catch(() => {})
    AuthService.Servers()
      .then((v) => {
        servers = v ?? []
        net = $session.user?.server.index ?? s?.serverIndex ?? servers[0]?.index ?? 0
      })
      .catch(() => {})
  })
</script>

<Page title={t('settings.title')} subtitle={t('settings.subtitle')}>
  <div class="scroll">
    {#if s}
      <div class="grid">
        <!-- 디자인 -->
        <section class="card wide" id="designCard" aria-labelledby="designCardTitle">
          <h2 id="designCardTitle">{t('design.card')}</h2>
          <DesignPicker />
          {#if look.scheme}<p class="hint">{t('design.fixedHint')}</p>{/if}
        </section>

        <!-- 기본 설정 -->
        <section class="card" id="basicCard" aria-labelledby="basicCardTitle">
          <h2 id="basicCardTitle">{t('settings.card.basic')}</h2>
          <div class="fields two">
            <Select
              id="themeCombo"
              label={t('settings.theme')}
              options={themeOptions}
              value={look.scheme ?? (s.theme === 'light' || s.theme === 'dark' ? s.theme : 'system')}
              disabled={!!look.scheme}
              onchange={(e: Event) => setTheme(picked(themeOptions, e) as Theme)}
            />
            <Select
              id="layoutCombo"
              label={t('settings.layout')}
              options={layoutOptions}
              value={look.layout ? (look.layout === 'tabs' ? 2 : 0) : s.layout === 2 ? 2 : 0}
              disabled={!!look.layout}
              onchange={(e: Event) => {
                const v = picked(layoutOptions, e)
                void save((x) => (x.layout = v))
              }}
            />
            <Select
              id="languageCombo"
              label={t('settings.language')}
              options={langOptions}
              value={s.language === 1 ? 1 : 0}
              onchange={(e: Event) => setLanguage(picked(langOptions, e) === 1 ? 'en' : 'ko')}
            />
          </div>
          {#key rev}
            <div class="row">
              <Switch label={t('settings.useLogFile')} checked={logOn ?? s.useLogFile} onchange={(c) => void setLogFile(c)} />
            </div>
          {/key}
          <p class="hint">{t('settings.useLogFileHint')}</p>
        </section>

        <!-- 로그아웃 처리 -->
        <section class="card" id="logoutCard" aria-labelledby="logoutCardTitle">
          <h2 id="logoutCardTitle">{t('settings.card.logout')}</h2>
          <RadioGroup
            label={t('settings.logoutType')}
            name="logoutType"
            options={logoutOptions}
            value={s.logoutType}
            onchange={(v) => void save((x) => (x.logoutType = v))}
          />
          <div class="switches">
            <Switch
              label={t('settings.logoutConfirm')}
              checked={s.logoutConfirm}
              onchange={(c) => void save((x) => (x.logoutConfirm = c))}
            />
            <Switch label={t('settings.closeToTray')} checked={s.closeToTray} onchange={(c) => void save((x) => (x.closeToTray = c))} />
          </div>
        </section>

        <!-- 시작 폴더 지정 -->
        <section class="card" id="startFolderCard" aria-labelledby="startFolderCardTitle">
          <h2 id="startFolderCardTitle">{t('settings.card.startFolder')}</h2>
          <div class="folder">
            <Input
              id="initFolderField"
              label={t('settings.startFolder')}
              value={s.initFolder}
              placeholder={startHint}
              readonly
            />
            <IconButton icon={FolderSearch} label={t('settings.pickFolder')} variant="secondary" onclick={chooseStart} />
            <IconButton
              icon={RotateCcw}
              label={t('settings.clearFolder')}
              disabled={!s.initFolder}
              onclick={() => void save((x) => (x.initFolder = ''))}
            />
          </div>
        </section>

        <!-- 자동 업로드 설정 -->
        <section class="card" id="autoUploadCard" aria-labelledby="autoUploadCardTitle">
          <h2 id="autoUploadCardTitle">{t('settings.card.autoUpload')}</h2>
          {#if servers.length > 1}
            <Select
              label={t('settings.network')}
              options={netOptions}
              value={net}
              onchange={(e: Event) => (net = picked(netOptions, e))}
            />
          {/if}
          {#key rev}
            <div class="row">
              <Switch label={t('settings.autoUploadUse')} checked={auto.uploadUse} onchange={(c) => void setAutoUse('upload', c)} />
            </div>
          {/key}
          <Select
            id="autoUploadPeriodCombo"
            label={t('settings.uploadPeriod')}
            options={periodOptions}
            value={PERIODS.includes(auto.uploadPeriod) ? auto.uploadPeriod : DEFAULT_PERIOD}
            disabled={!auto.uploadUse}
            onchange={(e: Event) => {
              const v = picked(periodOptions, e)
              void saveAuto((a) => (a.uploadPeriod = v))
            }}
          />
          <div class="folder">
            <Input label={t('settings.uploadFolder')} value={auto.uploadFolder} placeholder={t('settings.folderUnset')} readonly />
            <span id="autoUploadFolderButton">
              <IconButton icon={FolderSearch} label={t('settings.pickFolder')} variant="secondary" onclick={() => chooseAuto('upload')} />
            </span>
          </div>
          <p class="hint">{t('settings.autoHint')}</p>
        </section>

        <!-- 자동 다운로드 설정 -->
        <section class="card" id="autoDownloadCard" aria-labelledby="autoDownloadCardTitle">
          <h2 id="autoDownloadCardTitle">{t('settings.card.autoDownload')}</h2>
          {#if servers.length > 1}
            <Select
              label={t('settings.network')}
              options={netOptions}
              value={net}
              onchange={(e: Event) => (net = picked(netOptions, e))}
            />
          {/if}
          {#key rev}
            <div class="row">
              <Switch label={t('settings.autoDownloadUse')} checked={auto.downloadUse} onchange={(c) => void setAutoUse('download', c)} />
            </div>
          {/key}
          <div class="folder">
            <Input label={t('settings.downloadFolder')} value={auto.downloadFolder} placeholder={t('settings.folderUnset')} readonly />
            <span id="autoDownloadFolderButton">
              <IconButton icon={FolderSearch} label={t('settings.pickFolder')} variant="secondary" onclick={() => chooseAuto('download')} />
            </span>
          </div>
          <p class="hint">{t('settings.autoHint')}</p>
        </section>

        <!-- 클립보드 단축키 설정(Linux 숨김 ― SET-072) -->
        {#if info?.os !== 'linux'}
          <section class="card" id="hotkeyCard" aria-labelledby="hotkeyCardTitle">
            <h2 id="hotkeyCardTitle">{t('settings.card.hotkey')}</h2>
            <RadioGroup
              label={t('settings.hotkeyUse')}
              direction="row"
              name="hotkeyUse"
              options={hotkeyUseOptions}
              value={hk.use ? 'on' : 'off'}
              onchange={(v) => void saveHotkey((h) => (h.use = v === 'on'))}
            />
            {#each [{ which: 'copy', label: t('settings.hotkeyCopy'), mod: hk.copyKey, ch: hk.copyChar }, { which: 'paste', label: t('settings.hotkeyPaste'), mod: hk.pasteKey, ch: hk.pasteChar }] as k (k.which)}
              <div class="hotkey">
                <Select
                  label={k.label}
                  options={modOptions}
                  value={MODS.includes(k.mod) ? k.mod : 'ALT'}
                  disabled={!hk.use}
                  onchange={(e: Event) => {
                    const v = picked(modOptions, e)
                    void saveHotkey((h) => (k.which === 'copy' ? (h.copyKey = v) : (h.pasteKey = v)))
                  }}
                />
                <span class="plus" aria-hidden="true">+</span>
                <Input
                  label={t('settings.hotkeyChar')}
                  value={k.ch}
                  maxlength={1}
                  disabled={!hk.use}
                  onchange={(e: Event) => hotkeyChar(e, k.which as 'copy' | 'paste')}
                />
              </div>
            {/each}
          </section>
        {/if}

        <!-- 프로그램 정보 -->
        <section class="card" id="aboutCard" aria-labelledby="aboutCardTitle">
          <h2 id="aboutCardTitle">{t('settings.card.about')}</h2>
          <div class="about">
            <Logo size={40} word={false} />
            <div>
              <div class="product">{info?.title ?? 'snB App v6.0'} ({info?.version ?? ''})</div>
              <div class="copy">{info?.copyright ?? ''}</div>
            </div>
          </div>
          {#if info}
            <dl class="paths">
              <dt>{t('settings.dataDir')}</dt>
              <dd>{info.dataDir}</dd>
              <dt>{t('settings.appDir')}</dt>
              <dd>{info.appDir}</dd>
            </dl>
          {/if}
        </section>
      </div>
    {:else}
      <p class="hint">{t('common.loading')}</p>
    {/if}
  </div>
</Page>

<style>
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0 calc(-1 * var(--sp-5));
    padding: 0 var(--sp-5) var(--sp-2);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: var(--sp-4);
    align-items: start;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--r-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--shadow-1);
    min-width: 0;
  }
  .card.wide {
    grid-column: 1 / -1;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-md);
    font-weight: var(--fw-semibold);
    color: var(--color-text);
  }
  .fields.two {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); /* 「왼쪽 메뉴 + 상단 툴바」 가 잘리지 않는 폭 */
    gap: var(--sp-3);
  }
  .row,
  .switches {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .folder {
    display: flex;
    align-items: flex-end;
    gap: var(--sp-1);
  }
  .folder :global(.field) {
    flex: 1;
    min-width: 0;
  }
  .hotkey {
    display: grid;
    grid-template-columns: 1fr auto 72px;
    align-items: end;
    gap: var(--sp-2);
  }
  .plus {
    padding-bottom: 8px;
    color: var(--color-text-3);
  }
  .hint {
    margin: 0;
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
  .about {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
  }
  .product {
    font-weight: var(--fw-semibold);
  }
  .copy {
    color: var(--color-text-3);
    font-size: var(--fs-sm);
  }
  .paths {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--sp-1) var(--sp-3);
    margin: 0;
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--color-text-3);
  }
  dd {
    margin: 0;
    color: var(--color-text-2);
    word-break: break-all;
    user-select: text;
  }
</style>
