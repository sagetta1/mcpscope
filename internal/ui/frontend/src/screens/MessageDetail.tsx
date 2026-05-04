import { useMemo } from 'react'
import type { Message } from '../api'
import { JsonViewer } from '../components/JsonViewer'

// MessageDetail shows the selected message and (when applicable) its
// correlated counterpart from the same session. Correlation key is
// (direction, jsonrpc_id) — Bug 2 from week 1 dogfood: in real Claude
// Code traffic the server sends roots/list as a server→client request
// with id=0 that overlaps the client→initialize id=0; matching on
// jsonrpc_id alone would mis-pair them.

function findPartner(msg: Message, all: Message[]): Message | null {
  if (!msg.jsonrpc_id) return null // notifications don't have a partner
  const oppositeDir = msg.direction === 'in' ? 'out' : 'in'
  // request → response, response → request: same id, opposite direction
  return all.find((m) => m.direction === oppositeDir && m.jsonrpc_id === msg.jsonrpc_id) ?? null
}

function safeParse(raw: string): unknown {
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

export function MessageDetail({ message, allMessages }: { message: Message; allMessages: Message[] }) {
  const partner = useMemo(() => findPartner(message, allMessages), [message, allMessages])

  // For requests, show request on left + response on right.
  // For responses, show request on left + response (this msg) on right.
  // For notifications, show only one panel.
  let leftMsg: Message | null = null
  let rightMsg: Message | null = null
  if (message.kind === 'request') {
    leftMsg = message
    rightMsg = partner
  } else if (message.kind === 'response') {
    leftMsg = partner
    rightMsg = message
  } else {
    leftMsg = message
  }

  const layout = rightMsg ? 'grid-cols-2' : 'grid-cols-1'

  return (
    <div className={`grid h-full gap-3 ${layout}`}>
      {leftMsg && <Pane title="Request" msg={leftMsg} />}
      {rightMsg && <Pane title="Response" msg={rightMsg} />}
    </div>
  )
}

function Pane({ title, msg }: { title: string; msg: Message | null }) {
  if (!msg) {
    return (
      <div className="rounded border border-dashed border-gray-800 p-3 text-xs text-gray-600">
        {title} — not captured
      </div>
    )
  }
  return (
    <div className="flex h-full flex-col overflow-hidden rounded border border-gray-800">
      <div className="flex items-baseline justify-between border-b border-gray-800 px-2 py-1.5 text-[11px]">
        <span className="font-mono uppercase text-gray-400">{title}</span>
        <span className="font-mono text-gray-500">
          {msg.direction} · {msg.kind}
          {msg.jsonrpc_id ? ` · id=${msg.jsonrpc_id}` : ''}
          {msg.method ? ` · ${msg.method}` : ''}
        </span>
      </div>
      <div className="flex-1 overflow-auto bg-gray-950 p-2">
        <JsonViewer value={safeParse(msg.raw)} />
      </div>
    </div>
  )
}
