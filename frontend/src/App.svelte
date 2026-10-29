<script lang="ts">
  import { onMount } from 'svelte'
  import { Events } from '@wailsio/runtime'
  import { AppService, type FileRow, type LoginInfo, type Server } from '../bindings/snbapp'

  const { Servers, Login, Logout, Files, Upload, Download } = AppService
  const STATE: Record<number, string> = { 0: '결재요청', 1: '승인', 2: '반려', 4: '결재요청 가능' }

  let servers: Server[] = []
  let serverIdx = 0
  let id = ''
  let pw = ''
  let user: LoginInfo | null = null
  let rows: FileRow[] = []
  let picked = new Set<string>()
  let busy = false
  let message = ''
  let progress: { name: string; done: number; total: number } | null = null

  onMount(async () => {
    Events.On('progress', (e) => (progress = e.data))
    try {
      servers = (await Servers()) ?? []
      if (!servers.length) message = 'fmenv.ini 에 서버가 없습니다'
    } catch (e) {
      message = `fmenv.ini 를 읽지 못했습니다: ${e}`
    }
  })

  // 바인딩 호출 하나를 감싼다: 도는 동안 단추를 잠그고, 실패 문구는 화면 아래에 낸다
  async function run(fn: () => Promise<void>) {
    busy = true
    message = ''
    try {
      await fn()
    } catch (e) {
      message = String(e)
    } finally {
      busy = false
      progress = null
    }
  }

  const login = () =>
    run(async () => {
      user = await Login(servers[serverIdx], id, pw)
      pw = ''
      await refresh()
    })

  async function refresh() {
    rows = (await Files()) ?? [] // Go 의 빈 목록(nil)은 null 로 온다
    picked = new Set([...picked].filter((n) => rows.some((r) => r.serverName === n)))
  }

  const upload = () =>
    run(async () => {
      const done = (await Upload()) ?? []
      if (done.length) message = `올렸습니다: ${done.join(', ')}`
      await refresh()
    })

  const download = () =>
    run(async () => {
      const saved = (await Download([...picked])) ?? []
      if (saved.length) message = `받았습니다: ${saved.join(', ')}`
      picked = new Set()
      await refresh()
    })

  async function logout() {
    await Logout()
    user = null
    rows = []
    picked = new Set()
  }

  function toggle(n: string) {
    picked.has(n) ? picked.delete(n) : picked.add(n)
    picked = picked
  }

  const size = (b: number) =>
    b < 1024 ? `${b} B` : b < 1048576 ? `${(b / 1024).toFixed(1)} KB` : `${(b / 1048576).toFixed(1)} MB`
</script>

{#if !user}
  <form class="login" on:submit|preventDefault={login}>
    <h1>snbApp</h1>
    <label>망
      <select bind:value={serverIdx}>
        {#each servers as s, i}<option value={i}>{s.name || s.ip}</option>{/each}
      </select>
    </label>
    <label>아이디 <input bind:value={id} autocomplete="username" required /></label>
    <label>비밀번호 <input type="password" bind:value={pw} autocomplete="current-password" required /></label>
    <button type="submit" disabled={busy || !servers.length}>{busy ? '로그인 중…' : '로그인'}</button>
  </form>
{:else}
  <header>
    <span><b>{user.userName}</b> ({user.userId}) · {user.dept} · {user.server.name}</span>
    <button on:click={logout} disabled={busy}>로그아웃</button>
  </header>
  <div class="bar">
    <button on:click={upload} disabled={busy}>업로드</button>
    <button on:click={download} disabled={busy || !picked.size}>다운로드 ({picked.size})</button>
    <button on:click={() => run(refresh)} disabled={busy}>새로고침</button>
  </div>
  <div class="table">
    <table>
      <thead><tr><th></th><th>파일 이름</th><th>크기</th><th>상태</th><th>올린 망</th><th>승인 시각</th><th>만료</th></tr></thead>
      <tbody>
        {#each rows as r (r.serverName)}
          <tr class:off={r.blocked} title={r.blocked}>
            <td><input type="checkbox" disabled={!!r.blocked} checked={picked.has(r.serverName)} on:change={() => toggle(r.serverName)} aria-label="{r.name} 선택" /></td>
            <td>{r.name}</td>
            <td class="num">{size(r.size)}</td>
            <td>{STATE[r.state] ?? r.state}</td>
            <td>{r.course === 1 ? '업무망' : '인터넷망'}</td>
            <td>{r.aprvTime}</td>
            <td>{r.expireTime}</td>
          </tr>
        {:else}
          <tr><td colspan="7" class="empty">서버에 파일이 없습니다</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<footer>
  {#if progress}
    <!-- 받기 진행률은 서버 파일 이름으로 오므로 PC 파일 이름으로 바꿔 보인다 -->
    <span>{rows.find((r) => r.serverName === progress?.name)?.name ?? progress.name}</span>
    <progress max={progress.total} value={progress.done}></progress>
    <span>{size(progress.done)} / {size(progress.total)}</span>
  {:else}
    <span role="status">{message}</span>
  {/if}
</footer>

<style>
  :global(:root) {
    --bg: #ffffff; --fg: #161616; --muted: #6f6f6f; --line: #e0e0e0; --accent: #0f62fe; --row: #f4f4f4;
    font-family: 'Malgun Gothic', system-ui, sans-serif; font-size: 14px;
    color-scheme: light dark;
  }
  @media (prefers-color-scheme: dark) {
    :global(:root) { --bg: #161616; --fg: #f4f4f4; --muted: #a8a8a8; --line: #393939; --accent: #4589ff; --row: #262626; }
  }
  :global(body) { margin: 0; background: var(--bg); color: var(--fg); }
  :global(#app) { display: flex; flex-direction: column; height: 100vh; }
  .login { margin: auto; display: flex; flex-direction: column; gap: 12px; width: 280px; }
  .login label { display: flex; flex-direction: column; gap: 4px; color: var(--muted); }
  input, select, button { font: inherit; padding: 6px 10px; }
  button { background: var(--accent); color: #fff; border: 0; cursor: pointer; }
  button:disabled { opacity: .4; cursor: default; }
  header, .bar, footer { display: flex; gap: 8px; align-items: center; padding: 8px 16px; }
  header { justify-content: space-between; border-bottom: 1px solid var(--line); }
  .table { flex: 1; overflow: auto; padding: 0 16px; }
  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--line); white-space: nowrap; }
  th { position: sticky; top: 0; background: var(--bg); color: var(--muted); font-weight: normal; }
  tbody tr:hover { background: var(--row); }
  .num { text-align: right; }
  .off { color: var(--muted); }
  .empty { text-align: center; color: var(--muted); padding: 24px; }
  footer { border-top: 1px solid var(--line); min-height: 20px; }
  footer progress { flex: 1; }
</style>
