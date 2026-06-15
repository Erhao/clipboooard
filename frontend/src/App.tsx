import { useState, useEffect, useRef, useCallback } from 'react'

interface Clip {
  id: string
  type: 'text' | 'file'
  content?: string
  filename?: string
  fileSize?: number
  createdAt: string
}

const API = '/api'

function fmtSize(b: number): string {
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  return (b / (1024 * 1024)).toFixed(1) + ' MB'
}

function timeAgo(d: string): string {
  const s = Math.floor((Date.now() - new Date(d).getTime()) / 1000)
  if (s < 60) return '刚刚'
  const m = Math.floor(s / 60)
  if (m < 60) return m + ' 分钟前'
  const h = Math.floor(m / 60)
  if (h < 24) return h + ' 小时前'
  return Math.floor(h / 24) + ' 天前'
}

function getToken(): string | null {
  return localStorage.getItem('clipboooard_token')
}

function authHeaders(): Record<string, string> {
  const t = getToken()
  return t ? { Authorization: 'Bearer ' + t, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' }
}

// ---- Login screen ----
function LoginScreen({ onLogin }: { onLogin: (token: string) => void }) {
  const [pwd, setPwd] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const ref = useRef<HTMLInputElement>(null)

  useEffect(() => { ref.current?.focus() }, [])

  const submit = async () => {
    if (!pwd || busy) return
    setBusy(true)
    setErr('')
    try {
      const r = await fetch(API + '/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: pwd }),
      })
      const data = await r.json()
      if (r.ok) {
        localStorage.setItem('clipboooard_token', data.token)
        onLogin(data.token)
      } else {
        setErr(data.remaining !== undefined
          ? `密码错误，还剩 ${data.remaining} 次机会`
          : data.error || '登录失败')
      }
    } catch {
      setErr('网络错误')
    }
    setBusy(false)
  }

  return (
    <div className="login-wrap">
      <form className="login-card" onSubmit={e => { e.preventDefault(); submit() }}>
        <h1>Clipboooard</h1>
        <input
          ref={ref}
          type="password"
          value={pwd}
          onChange={e => setPwd(e.target.value)}
          placeholder="请输入密码"
          className="login-input"
        />
        {err && <p className="login-err">{err}</p>}
        <button className="pri login-btn" type="submit" disabled={!pwd || busy}>
          {busy ? '验证中...' : '登录'}
        </button>
      </form>
    </div>
  )
}

// ---- Main app ----
export default function App() {
  const [token, setToken] = useState(getToken)
  const [clips, setClips] = useState<Clip[]>([])
  const [text, setText] = useState('')
  const [connected, setConnected] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadPct, setUploadPct] = useState(0)
  const [uploadErr, setUploadErr] = useState('')
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const taRef = useRef<HTMLTextAreaElement>(null)

  const loadClips = useCallback(() => {
    fetch(API + '/clips?limit=50', { headers: authHeaders() })
      .then(r => r.json())
      .then(setClips)
      .catch(() => {})
  }, [])

  useEffect(() => { if (token) loadClips() }, [loadClips, token])

  useEffect(() => {
    if (!token) return
    const t = getToken()
    if (!t) return
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(proto + '//' + location.host + '/ws?token=' + t)
    ws.onopen = () => setConnected(true)
    ws.onclose = () => setConnected(false)
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'new_clip') setClips(prev => [msg.clip, ...prev])
      } catch {}
    }
    return () => { ws.onclose = null; ws.close() }
  }, [token])

  const submitText = async () => {
    if (!text.trim()) return
    try {
      const res = await fetch(API + '/clip', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ type: 'text', content: text }),
      })
      if (res.ok) setText('')
      else if (res.status === 401) { setToken(null); localStorage.removeItem('clipboooard_token') }
    } catch {}
  }

  const uploadFile = (file: File) => {
    setUploading(true)
    setUploadPct(0)
    setUploadErr('')
    const xhr = new XMLHttpRequest()
    xhr.open('POST', API + '/upload')
    const t = getToken()
    if (t) xhr.setRequestHeader('Authorization', 'Bearer ' + t)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) setUploadPct(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      setUploading(false)
      if (xhr.status === 401) { setToken(null); localStorage.removeItem('clipboooard_token'); setUploadErr('登录已过期，请重新登录') }
      else if (xhr.status >= 500) { setUploadErr('服务器错误，请重试') }
      else if (xhr.status >= 400) { setUploadErr('上传失败 (' + xhr.status + ')') }
    }
    xhr.onerror = () => { setUploading(false); setUploadErr('网络错误，上传中断') }
    xhr.ontimeout = () => { setUploading(false); setUploadErr('上传超时') }
    xhr.timeout = 300000

    const fd = new FormData()
    fd.append('file', file)
    xhr.send(fd)
  }

  const copyText = async (content: string, id: string) => {
    try {
      await navigator.clipboard.writeText(content)
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch {}
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault()
        submitText()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [text])

  if (!token) return <LoginScreen onLogin={setToken} />

  return (
    <div className="app">
      <header>
        <h1>Clipboooard</h1>
        <span className={'dot ' + (connected ? 'on' : 'off')}>
          {connected ? '已连接' : '未连接'}
        </span>
      </header>

      <main>
        <div className="card">
          <div className="panel">
            <textarea
              ref={taRef}
              value={text}
              onChange={e => setText(e.target.value)}
              placeholder="在此粘贴或输入文本..."
              rows={5}
            />
            <div className="row">
              <button className="pri" onClick={submitText} disabled={!text.trim()}>
                发送 ⌘⏎
              </button>
            </div>
            <div className="divider" />
            <div
              className={'drop drop-sm' + (uploading ? ' uploading' : '')}
              onClick={() => !uploading && fileRef.current?.click()}
              onDragOver={e => { e.preventDefault(); e.currentTarget.classList.add('over') }}
              onDragLeave={e => e.currentTarget.classList.remove('over')}
              onDrop={e => {
                e.preventDefault()
                e.currentTarget.classList.remove('over')
                if (uploading) return
                const f = e.dataTransfer.files[0]
                if (f) uploadFile(f)
              }}
            >
              {uploading ? (
                <div className="progress-wrap">
                  <div className="progress-bar">
                    <div className="progress-fill" style={{ width: uploadPct + '%' }} />
                  </div>
                  <p>{uploadPct}%</p>
                </div>
              ) : (
                <p>拖拽文件到此处或点击上传</p>
              )}
            </div>
            {uploadErr && <p className="login-err" style={{ marginTop: 8 }}>{uploadErr}</p>}
            <input ref={fileRef} type="file" hidden
              onChange={e => { const f = e.target.files?.[0]; if (f) uploadFile(f); e.target.value = '' }}
            />
          </div>
        </div>

        <section className="history">
          <h2>历史记录</h2>
          {clips.length === 0 && <p className="empty">暂无记录，粘贴文本或上传文件开始使用</p>}
          <ul>
            {clips.map(c => (
              <li key={c.id} className="item">
                {c.type === 'text' ? (
                  <>
                    <pre className="content">{c.content}</pre>
                    <div className="meta">
                      <span>{timeAgo(c.createdAt)}</span>
                      <button onClick={() => copyText(c.content!, c.id)}>
                        {copiedId === c.id ? '已复制' : '复制'}
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="file-info">
                      <span className="ficon">📄</span>
                      <span className="fname">{c.filename}</span>
                      <span className="fsize">{fmtSize(c.fileSize || 0)}</span>
                    </div>
                    <div className="meta">
                      <span>{timeAgo(c.createdAt)}</span>
                      <a href={API + '/file/' + c.id + '?token=' + getToken()} download>下载</a>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        </section>
      </main>
    </div>
  )
}
