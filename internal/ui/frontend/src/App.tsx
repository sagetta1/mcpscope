import { useEffect, useState } from 'react'

type Session = {
  id: string
  started_at: number
  ended_at?: number
  target_cmd: string
  msg_count: number
}

function fmtTs(ms: number): string {
  return new Date(ms).toISOString().replace('T', ' ').slice(0, 19)
}

function App() {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/sessions?limit=50')
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then(setSessions)
      .catch((e) => setError(String(e)))
  }, [])

  return (
    <div className="min-h-screen p-6 font-sans">
      <header className="mb-6 flex items-baseline gap-3">
        <h1 className="text-xl font-semibold text-white">mcpscope</h1>
        <span className="text-sm text-gray-400">Chrome DevTools for the Model Context Protocol</span>
      </header>

      {error && (
        <div className="rounded border border-red-700 bg-red-900/30 p-3 text-sm text-red-300">
          Failed to load sessions: {error}
        </div>
      )}

      {!sessions && !error && <p className="text-sm text-gray-500">Loading…</p>}

      {sessions && sessions.length === 0 && (
        <p className="text-sm text-gray-500">
          No sessions yet — run{' '}
          <code className="font-mono text-gray-300">mcpscope wrap -- &lt;command&gt;</code> first.
        </p>
      )}

      {sessions && sessions.length > 0 && (
        <table className="w-full border-collapse text-sm">
          <thead className="text-left text-gray-400">
            <tr className="border-b border-gray-800">
              <th className="py-2 pr-4 font-medium">Session</th>
              <th className="py-2 pr-4 font-medium">Started</th>
              <th className="py-2 pr-4 font-medium">Ended</th>
              <th className="py-2 pr-4 font-medium text-right">Msgs</th>
              <th className="py-2 pr-4 font-medium">Command</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map((s) => (
              <tr key={s.id} className="border-b border-gray-900 hover:bg-gray-900/50">
                <td className="py-1.5 pr-4 font-mono text-xs text-gray-300">{s.id}</td>
                <td className="py-1.5 pr-4 font-mono text-xs text-gray-400">{fmtTs(s.started_at)}</td>
                <td className="py-1.5 pr-4 font-mono text-xs text-gray-500">
                  {s.ended_at ? fmtTs(s.ended_at) : <span className="text-amber-400">(running)</span>}
                </td>
                <td className="py-1.5 pr-4 text-right tabular-nums text-gray-300">{s.msg_count}</td>
                <td className="py-1.5 pr-4 truncate font-mono text-xs text-gray-400" style={{ maxWidth: '40ch' }}>
                  {s.target_cmd}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

export default App
