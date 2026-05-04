import { useEffect, useMemo, useState } from 'react'
import { listMessages, type Message } from '../api'
import { MessageDetail } from './MessageDetail'

function fmtTime(ms: number): string {
  const d = new Date(ms)
  return d.toTimeString().slice(0, 8) + '.' + (ms % 1000).toString().padStart(3, '0')
}

const dirColors: Record<string, string> = {
  in: 'text-blue-400',
  out: 'text-emerald-400',
}

const kindBadges: Record<string, string> = {
  request: 'bg-purple-900/50 text-purple-300',
  response: 'bg-emerald-900/50 text-emerald-300',
  notification: 'bg-amber-900/50 text-amber-300',
  invalid: 'bg-red-900/50 text-red-300',
}

export function Timeline({ sessionID }: { sessionID: string }) {
  const [messages, setMessages] = useState<Message[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedID, setSelectedID] = useState<number | null>(null)

  useEffect(() => {
    setMessages(null)
    setSelectedID(null)
    listMessages(sessionID).then((m) => {
      setMessages(m)
      if (m.length > 0) setSelectedID(m[0].id)
    }).catch((e) => setError(String(e)))
  }, [sessionID])

  const selected = useMemo(
    () => messages?.find((m) => m.id === selectedID) ?? null,
    [messages, selectedID]
  )

  if (error) {
    return <div className="text-sm text-red-400">Failed to load messages: {error}</div>
  }
  if (!messages) return <p className="text-sm text-gray-500">Loading…</p>
  if (messages.length === 0) {
    return <p className="text-sm text-gray-500">No messages in this session.</p>
  }

  return (
    <div className="grid h-[calc(100vh-7rem)] grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)] gap-4">
      {/* Timeline */}
      <div className="overflow-y-auto rounded border border-gray-800 bg-gray-950/50">
        <table className="w-full border-collapse text-xs">
          <tbody>
            {messages.map((m) => {
              const isSel = m.id === selectedID
              return (
                <tr
                  key={m.id}
                  className={
                    'cursor-pointer border-b border-gray-900 ' +
                    (isSel ? 'bg-blue-900/30' : 'hover:bg-gray-900/50')
                  }
                  onClick={() => setSelectedID(m.id)}
                >
                  <td className="px-2 py-1 font-mono text-gray-500">{fmtTime(m.ts)}</td>
                  <td className={'px-2 py-1 font-mono font-bold uppercase ' + (dirColors[m.direction] ?? '')}>
                    {m.direction}
                  </td>
                  <td className="px-2 py-1">
                    <span className={'rounded px-1.5 py-0.5 font-mono text-[10px] ' + (kindBadges[m.kind] ?? '')}>
                      {m.kind}
                    </span>
                  </td>
                  <td className="truncate px-2 py-1 font-mono text-gray-300" style={{ maxWidth: '24ch' }}>
                    {m.method ?? <span className="text-gray-600">—</span>}
                  </td>
                  <td className="px-2 py-1 text-right font-mono text-gray-500">
                    {m.jsonrpc_id ? `id=${m.jsonrpc_id}` : ''}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Detail pane */}
      <div className="overflow-y-auto rounded border border-gray-800 bg-gray-950/50 p-3">
        {selected ? (
          <MessageDetail message={selected} allMessages={messages} />
        ) : (
          <p className="text-sm text-gray-500">Select a message on the left.</p>
        )}
      </div>
    </div>
  )
}
