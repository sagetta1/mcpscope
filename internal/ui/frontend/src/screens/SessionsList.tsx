import { useEffect, useState } from 'react'
import { listSessions, type Session } from '../api'

function fmtTs(ms: number): string {
  return new Date(ms).toISOString().replace('T', ' ').slice(0, 19)
}

function fmtDur(start: number, end?: number): string {
  if (!end) return '—'
  const s = Math.round((end - start) / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${m}m${r.toString().padStart(2, '0')}s`
}

function navTo(hash: string) {
  window.location.hash = hash
}

export function SessionsList() {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listSessions(50).then(setSessions).catch((e) => setError(String(e)))
  }, [])

  if (error) {
    return (
      <div className="rounded border border-red-700 bg-red-900/30 p-3 text-sm text-red-300">
        Failed to load sessions: {error}
      </div>
    )
  }

  if (!sessions) return <p className="text-sm text-gray-500">Loading…</p>

  if (sessions.length === 0) {
    return (
      <p className="text-sm text-gray-500">
        No sessions yet — run{' '}
        <code className="font-mono text-gray-300">mcpscope wrap -- &lt;command&gt;</code> first.
      </p>
    )
  }

  return (
    <table className="w-full border-collapse text-sm">
      <thead className="text-left text-gray-400">
        <tr className="border-b border-gray-800">
          <th className="py-2 pr-4 font-medium">Session</th>
          <th className="py-2 pr-4 font-medium">Started</th>
          <th className="py-2 pr-4 font-medium">Duration</th>
          <th className="py-2 pr-4 font-medium text-right">Msgs</th>
          <th className="py-2 pr-4 font-medium">Command</th>
        </tr>
      </thead>
      <tbody>
        {sessions.map((s) => (
          <tr
            key={s.id}
            className="cursor-pointer border-b border-gray-900 hover:bg-gray-900/50"
            onClick={() => navTo(`#/session/${s.id}`)}
          >
            <td className="py-1.5 pr-4 font-mono text-xs text-blue-400">{s.id}</td>
            <td className="py-1.5 pr-4 font-mono text-xs text-gray-400">{fmtTs(s.started_at)}</td>
            <td className="py-1.5 pr-4 font-mono text-xs text-gray-500">
              {s.ended_at ? fmtDur(s.started_at, s.ended_at) : <span className="text-amber-400">running</span>}
            </td>
            <td className="py-1.5 pr-4 text-right tabular-nums text-gray-300">{s.msg_count}</td>
            <td className="py-1.5 pr-4 truncate font-mono text-xs text-gray-400" style={{ maxWidth: '60ch' }}>
              {s.target_cmd}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
