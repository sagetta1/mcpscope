import { useEffect, useRef, useState } from 'react'
import { listMessages, openLive, type Message } from '../api'

function fmtTime(ms: number): string {
  return new Date(ms).toTimeString().slice(0, 8) + '.' + (ms % 1000).toString().padStart(3, '0')
}

const dirColors: Record<string, string> = {
  in: 'text-blue-400',
  out: 'text-emerald-400',
}

export function Live({ sessionID }: { sessionID: string }) {
  const [messages, setMessages] = useState<Message[]>([])
  const [error, setError] = useState<string | null>(null)
  const [connected, setConnected] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)

  // 1) Load existing messages, then 2) open SSE for everything after the
  //    last seen id. Two-step pattern avoids the race where a brand-new
  //    message lands between the initial fetch and the EventSource open.
  useEffect(() => {
    let es: EventSource | null = null
    let cancelled = false

    listMessages(sessionID).then((initial) => {
      if (cancelled) return
      setMessages(initial)
      const lastID = initial.length > 0 ? initial[initial.length - 1].id : 0
      es = openLive(sessionID, lastID)
      es.onopen = () => setConnected(true)
      es.onerror = () => setConnected(false)
      es.onmessage = (ev) => {
        try {
          const m = JSON.parse(ev.data) as Message
          setMessages((prev) => [...prev, m])
        } catch {
          // ignore malformed events
        }
      }
    }).catch((e) => setError(String(e)))

    return () => {
      cancelled = true
      es?.close()
    }
  }, [sessionID])

  // Auto-scroll on new message.
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [messages.length])

  if (error) return <div className="text-sm text-red-400">{error}</div>

  return (
    <div className="flex h-[calc(100vh-7rem)] flex-col gap-2">
      <div className="flex items-center gap-2 text-xs">
        <span
          className={'inline-block h-2 w-2 rounded-full ' + (connected ? 'bg-emerald-500' : 'bg-red-500')}
        />
        <span className="text-gray-400">{connected ? 'Live — streaming new messages' : 'Disconnected'}</span>
        <span className="ml-auto text-gray-500">{messages.length} messages</span>
      </div>

      <div ref={scrollRef} className="flex-1 overflow-y-auto rounded border border-gray-800 bg-gray-950/50">
        <table className="w-full border-collapse text-xs">
          <tbody>
            {messages.map((m) => (
              <tr key={m.id} className="border-b border-gray-900">
                <td className="px-2 py-1 font-mono text-gray-500">{fmtTime(m.ts)}</td>
                <td className={'px-2 py-1 font-mono font-bold uppercase ' + (dirColors[m.direction] ?? '')}>
                  {m.direction}
                </td>
                <td className="px-2 py-1 font-mono text-gray-400">{m.kind}</td>
                <td className="px-2 py-1 font-mono text-gray-300">
                  {m.method ?? <span className="text-gray-600">—</span>}
                </td>
                <td className="px-2 py-1 text-right font-mono text-gray-500">
                  {m.jsonrpc_id ? `id=${m.jsonrpc_id}` : ''}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {messages.length === 0 && (
          <p className="p-3 text-sm text-gray-500">Waiting for new messages…</p>
        )}
      </div>
    </div>
  )
}
