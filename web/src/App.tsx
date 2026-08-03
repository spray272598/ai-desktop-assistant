import { useEffect, useState } from 'react'
import { api, ChatResult, SessionInfo, ToolInfo } from './api'

type Msg = { role: 'user' | 'bot' | 'sys' | 'perm'; text: string }

const USER = 'desktop-user'

export default function App() {
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [sessionId, setSessionId] = useState(localStorage.getItem('ada_session') || '')
  const [tools, setTools] = useState<ToolInfo[]>([])
  const [market, setMarket] = useState<any[]>([])
  const [mcp, setMcp] = useState<any[]>([])
  const [perms, setPerms] = useState<any[]>([])
  const [models, setModels] = useState<any>({})
  const [messages, setMessages] = useState<Msg[]>([])
  const [input, setInput] = useState('')
  const [autoApprove, setAutoApprove] = useState(false)
  const [health, setHealth] = useState('…')
  const [tab, setTab] = useState<'tools' | 'mcp' | 'market' | 'skills' | 'models'>('tools')
  const [skills, setSkills] = useState<any[]>([])
  const [busy, setBusy] = useState(false)

  const push = (m: Msg) => setMessages((prev) => [...prev, m])

  async function ensureSession() {
    if (sessionId) {
      try {
        await api(`/api/v1/session/info?sessionId=${encodeURIComponent(sessionId)}`)
        return sessionId
      } catch { /* recreate */ }
    }
    const s = await api<{ sessionId: string }>('/api/v1/session/create', {
      method: 'POST',
      body: JSON.stringify({ userId: USER, title: 'React 控制台' }),
    })
    setSessionId(s.sessionId)
    localStorage.setItem('ada_session', s.sessionId)
    return s.sessionId
  }

  async function refreshAll() {
    try {
      const h = await api<any>('/health')
      setHealth('online ' + (h.time || ''))
    } catch {
      setHealth('offline')
    }
    const sid = await ensureSession()
    const [ss, ts, mk, mp, pr, md, sk] = await Promise.all([
      api<SessionInfo[]>(`/api/v1/session/list?userId=${USER}`),
      api<ToolInfo[]>('/api/v1/tools'),
      api<any[]>('/api/v1/mcp/market'),
      api<any[]>('/api/v1/mcp/servers'),
      api<any[]>(`/api/v1/permission/pending?sessionId=${encodeURIComponent(sid)}`),
      api<any>('/api/v1/models'),
      api<any[]>('/api/v1/skills'),
    ])
    setSessions(ss || [])
    setTools(ts || [])
    setMarket(mk || [])
    setMcp(mp || [])
    setPerms(pr || [])
    setModels(md || {})
    setSkills(sk || [])
  }

  async function loadHistory(sid: string) {
    try {
      const msgs = await api<any[]>(`/api/v1/session/messages?sessionId=${encodeURIComponent(sid)}`)
      const mapped: Msg[] = (msgs || []).map((m) => {
        if (m.role === 'user') return { role: 'user', text: m.content }
        if (m.role === 'tool') return { role: 'sys', text: `[tool:${m.toolName || ''}] ${m.content}` }
        return { role: 'bot', text: m.content }
      })
      setMessages(mapped)
    } catch (e: any) {
      push({ role: 'sys', text: '历史加载失败: ' + e.message })
    }
  }

  useEffect(() => {
    refreshAll().then(async () => {
      const sid = localStorage.getItem('ada_session')
      if (sid) await loadHistory(sid)
      push({ role: 'sys', text: 'React 控制台就绪。可试 list files / run_code / open_url' })
    })
  }, [])

  async function send(text: string) {
    if (!text.trim() || busy) return
    setBusy(true)
    const sid = await ensureSession()
    push({ role: 'user', text })
    setInput('')
    try {
      const data = await api<ChatResult>('/api/v1/chat', {
        method: 'POST',
        body: JSON.stringify({
          sessionId: sid,
          userId: USER,
          message: text,
          autoApprove,
        }),
      })
      push({ role: 'bot', text: data.response || '(empty)' })
      if (data.taskPlan?.subTasks) {
        const plan =
          '📋 ' +
          (data.taskPlan.summary || '计划') +
          '\n' +
          data.taskPlan.subTasks.map((t: any) => `  ${t.index}. [${t.status}] ${t.title}`).join('\n')
        push({ role: 'sys', text: plan })
      }
      if (data.skillId) {
        push({ role: 'sys', text: `🧩 Skill: ${data.skillId}` })
      }
      if (data.errorClass) {
        push({ role: 'sys', text: `⚠ errorClass: ${data.errorClass}` })
      }
      if (data.needPermission && data.pendingPermission) {
        push({
          role: 'perm',
          text: `需要确认: ${data.pendingPermission.tool} — ${data.pendingPermission.reason}\nID: ${data.pendingPermission.id}`,
        })
      }
      await refreshAll()
    } catch (e: any) {
      push({ role: 'sys', text: '错误: ' + e.message })
    } finally {
      setBusy(false)
    }
  }

  async function selectSession(s: SessionInfo) {
    setSessionId(s.sessionId)
    localStorage.setItem('ada_session', s.sessionId)
    await loadHistory(s.sessionId)
    await refreshAll()
  }

  async function exportSession(format: string) {
    if (!sessionId) return
    try {
      const r = await api<{ path: string }>(
        `/api/v1/session/export?sessionId=${encodeURIComponent(sessionId)}&format=${format}`,
      )
      push({ role: 'sys', text: `已导出: ${r.path}` })
    } catch (e: any) {
      push({ role: 'sys', text: '导出失败: ' + e.message })
    }
  }

  return (
    <div className="app">
      <header>
        <h1>AI Desktop Assistant</h1>
        <div className="meta">
          user=<b>{USER}</b> · session=<code>{sessionId.slice(0, 18)}…</code> · {health}
        </div>
      </header>
      <div className="layout">
        <aside>
          <div className="pt">
            <span>会话</span>
            <button
              className="btn sm"
              onClick={async () => {
                localStorage.removeItem('ada_session')
                setSessionId('')
                setMessages([])
                await ensureSession()
                await refreshAll()
              }}
            >
              新建
            </button>
          </div>
          <div className="list">
            {sessions.map((s) => (
              <div
                key={s.sessionId}
                className={'item' + (s.sessionId === sessionId ? ' active' : '')}
                onClick={() => selectSession(s)}
              >
                <div className="t">{s.title || s.sessionId}</div>
                <div className="s">
                  {s.messageCount} msgs · {s.status}
                </div>
              </div>
            ))}
          </div>
          <div className="pt">导出</div>
          <div className="pad">
            <button className="btn sm ghost" onClick={() => exportSession('json')}>
              JSON
            </button>{' '}
            <button className="btn sm ghost" onClick={() => exportSession('md')}>
              Markdown
            </button>
          </div>
        </aside>

        <main>
          <div className="pt">
            <span>对话</span>
            <label className="chk">
              <input type="checkbox" checked={autoApprove} onChange={(e) => setAutoApprove(e.target.checked)} />
              自动批准
            </label>
          </div>
          <div className="chat">
            {messages.map((m, i) => (
              <div key={i} className={'bubble ' + m.role}>
                {m.text}
              </div>
            ))}
          </div>
          <div className="composer">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  send(input)
                }
              }}
              placeholder="输入指令…"
            />
            <div className="actions">
              <button className="btn" disabled={busy} onClick={() => send(input)}>
                {busy ? '…' : '发送'}
              </button>
              <button className="btn ghost" onClick={() => send('继续')}>
                继续
              </button>
            </div>
          </div>
        </main>

        <div className="right">
          <div className="tabs">
            {(['tools', 'mcp', 'market', 'skills', 'models'] as const).map((t) => (
              <button key={t} className={'tab' + (tab === t ? ' on' : '')} onClick={() => setTab(t)}>
                {t}
              </button>
            ))}
          </div>
          <div className="list">
            {tab === 'tools' &&
              tools.map((t) => (
                <div key={t.name} className="chip">
                  <b>{t.name}</b>
                  <span>{t.description}</span>
                </div>
              ))}
            {tab === 'mcp' &&
              mcp.map((s) => (
                <div key={s.name} className="chip">
                  <b>
                    {s.name} {s.online ? <em className="on">online</em> : <em className="off">off</em>}
                  </b>
                  <span>
                    {s.transport} · tools={s.toolCount ?? 0} · {s.command || s.url}
                  </span>
                </div>
              ))}
            {tab === 'market' &&
              market.map((p) => (
                <div key={p.id} className="chip">
                  <b>
                    {p.name} <small>{p.id}</small>
                    {p.installed ? <em className="on"> installed</em> : null}
                  </b>
                  <span>{p.description}</span>
                  <div className="row">
                    <button
                      className="btn sm ok"
                      onClick={async () => {
                        try {
                          const r = await api<any>('/api/v1/mcp/market/install', {
                            method: 'POST',
                            body: JSON.stringify({ id: p.id }),
                          })
                          push({
                            role: 'sys',
                            text: `已安装 MCP ${p.id} tools=${r?.toolCount ?? '?'}`,
                          })
                        } catch (e: any) {
                          push({ role: 'sys', text: '安装失败: ' + e.message })
                        }
                        await refreshAll()
                      }}
                    >
                      安装
                    </button>
                    <button
                      className="btn sm danger"
                      onClick={async () => {
                        await api('/api/v1/mcp/market/uninstall', {
                          method: 'POST',
                          body: JSON.stringify({ id: p.id }),
                        })
                        await refreshAll()
                      }}
                    >
                      卸载
                    </button>
                  </div>
                </div>
              ))}
            {tab === 'skills' &&
              skills.map((sk) => (
                <div key={sk.id} className="chip">
                  <b>
                    {sk.name} <small>{sk.id}</small>
                  </b>
                  <span>{sk.description}</span>
                  <span className="s">tools: {(sk.tools || []).join(', ') || 'any'}</span>
                </div>
              ))}
            {tab === 'models' && (
              <pre className="pre">{JSON.stringify(models, null, 2)}</pre>
            )}
          </div>
          <div className="pt">待确认权限</div>
          <div className="list perm">
            {perms.map((p) => (
              <div key={p.id} className="chip">
                <b>{p.tool}</b>
                <span>{p.reason}</span>
                <div className="row">
                  <button
                    className="btn sm ok"
                    onClick={async () => {
                      try {
                        const r = await api<any>('/api/v1/permission/approve', {
                          method: 'POST',
                          body: JSON.stringify({
                            id: p.id,
                            scope: 'once',
                            continue: true,
                            sessionId,
                            userId: USER,
                          }),
                        })
                        push({ role: 'sys', text: '已批准并继续: ' + p.tool })
                        if (r?.chat?.response) {
                          push({ role: 'bot', text: r.chat.response })
                        }
                      } catch (e: any) {
                        push({ role: 'sys', text: '批准失败: ' + e.message })
                      }
                      await refreshAll()
                    }}
                  >
                    批准并继续
                  </button>
                  <button
                    className="btn sm danger"
                    onClick={async () => {
                      await api('/api/v1/permission/reject', {
                        method: 'POST',
                        body: JSON.stringify({ id: p.id }),
                      })
                      await refreshAll()
                    }}
                  >
                    拒绝
                  </button>
                </div>
              </div>
            ))}
            {!perms.length && <div className="s pad">无待确认项</div>}
          </div>
        </div>
      </div>
    </div>
  )
}
