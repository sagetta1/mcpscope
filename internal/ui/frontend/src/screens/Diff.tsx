import { useEffect, useMemo, useState } from 'react'
import * as jsondiffpatch from 'jsondiffpatch'
import { listMessages, type Message } from '../api'

const dp = jsondiffpatch.create({
  // Diff arrays elementwise so reordering doesn't pretend everything changed.
  arrays: { detectMove: true },
  // Treat objects as the same identity even if "id" differs — we want value diff.
  objectHash: (_obj: unknown, idx?: number) => idx?.toString() ?? '',
})

function safeParse(raw: string): unknown {
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

function summarize(m: Message): string {
  const parts = [
    m.direction.toUpperCase(),
    m.kind,
    m.jsonrpc_id ? `id=${m.jsonrpc_id}` : '',
    m.method ?? '',
  ]
  return parts.filter(Boolean).join(' · ') + ` (#${m.id})`
}

// Plain-text rendering of jsondiffpatch deltas — colored, monospace, no
// HTML formatter dep needed. Each line is one change with a +/-/Δ marker.
function renderDelta(delta: unknown, prefix = ''): React.ReactNode {
  if (delta === undefined || delta === null) return null
  if (Array.isArray(delta)) {
    // Leaf change: [oldValue, newValue] | [newValue] (added) | [oldValue, 0, 0] (deleted)
    if (delta.length === 1) {
      return (
        <div className="text-emerald-300">
          <span className="text-emerald-500">+ </span>
          {prefix} = {JSON.stringify(delta[0])}
        </div>
      )
    }
    if (delta.length === 3 && delta[1] === 0 && delta[2] === 0) {
      return (
        <div className="text-red-300">
          <span className="text-red-500">- </span>
          {prefix} (was {JSON.stringify(delta[0])})
        </div>
      )
    }
    if (delta.length === 2) {
      return (
        <div>
          <div className="text-red-300">
            <span className="text-red-500">- </span>
            {prefix} = {JSON.stringify(delta[0])}
          </div>
          <div className="text-emerald-300">
            <span className="text-emerald-500">+ </span>
            {prefix} = {JSON.stringify(delta[1])}
          </div>
        </div>
      )
    }
  }
  if (typeof delta === 'object') {
    const d = delta as Record<string, unknown>
    return (
      <>
        {Object.entries(d).map(([key, child]) => {
          if (key === '_t') return null // jsondiffpatch internal marker
          const childPath = prefix ? `${prefix}.${key}` : key
          return <div key={key}>{renderDelta(child, childPath)}</div>
        })}
      </>
    )
  }
  return null
}

export function Diff({ sessionID }: { sessionID: string }) {
  const [messages, setMessages] = useState<Message[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [aID, setAID] = useState<number | null>(null)
  const [bID, setBID] = useState<number | null>(null)

  useEffect(() => {
    listMessages(sessionID).then(setMessages).catch((e) => setError(String(e)))
  }, [sessionID])

  // Read ?a= and ?b= from hash query params, if present.
  useEffect(() => {
    const q = window.location.hash.split('?')[1] ?? ''
    const params = new URLSearchParams(q)
    const a = params.get('a')
    const b = params.get('b')
    if (a) setAID(parseInt(a, 10))
    if (b) setBID(parseInt(b, 10))
  }, [])

  const a = useMemo(() => messages?.find((m) => m.id === aID) ?? null, [messages, aID])
  const b = useMemo(() => messages?.find((m) => m.id === bID) ?? null, [messages, bID])

  const delta = useMemo(() => {
    if (!a || !b) return null
    return dp.diff(safeParse(a.raw), safeParse(b.raw))
  }, [a, b])

  if (error) return <div className="text-sm text-red-400">{error}</div>
  if (!messages) return <p className="text-sm text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <Picker label="A (left)" messages={messages} value={aID} onChange={setAID} />
        <Picker label="B (right)" messages={messages} value={bID} onChange={setBID} />
      </div>

      {!a || !b ? (
        <p className="text-sm text-gray-500">Pick two messages above to compare.</p>
      ) : delta === undefined ? (
        <p className="text-sm text-emerald-400">Identical — no differences.</p>
      ) : (
        <div className="rounded border border-gray-800 bg-gray-950 p-3 font-mono text-xs leading-relaxed">
          {renderDelta(delta)}
        </div>
      )}
    </div>
  )
}

function Picker({
  label,
  messages,
  value,
  onChange,
}: {
  label: string
  messages: Message[]
  value: number | null
  onChange: (id: number) => void
}) {
  return (
    <label className="block text-xs">
      <span className="mb-1 block text-gray-400">{label}</span>
      <select
        className="w-full rounded border border-gray-700 bg-gray-900 px-2 py-1 font-mono text-xs text-gray-200"
        value={value ?? ''}
        onChange={(e) => onChange(parseInt(e.target.value, 10))}
      >
        <option value="">— select —</option>
        {messages.map((m) => (
          <option key={m.id} value={m.id}>
            {summarize(m)}
          </option>
        ))}
      </select>
    </label>
  )
}
