// Typed fetch helpers for the mcpscope HTTP API. Mirrors the DTOs in
// internal/ui/server.go — keep these in sync when the API changes.

export type Session = {
  id: string
  started_at: number
  ended_at?: number
  target_cmd: string
  msg_count: number
}

export type Direction = 'in' | 'out'
export type Kind = 'request' | 'response' | 'notification' | 'invalid'

export type Message = {
  id: number
  ts: number
  direction: Direction
  kind: Kind
  jsonrpc_id?: string
  method?: string
  raw: string
}

export async function listSessions(limit = 50): Promise<Session[]> {
  const r = await fetch(`/api/sessions?limit=${limit}`)
  if (!r.ok) throw new Error(`listSessions: HTTP ${r.status}`)
  return r.json()
}

export async function listMessages(sessionID: string, fromID = 0): Promise<Message[]> {
  const r = await fetch(`/api/sessions/${encodeURIComponent(sessionID)}/messages?from=${fromID}`)
  if (!r.ok) throw new Error(`listMessages: HTTP ${r.status}`)
  return r.json()
}

// openLive returns an EventSource that streams new Message rows as the
// wrap subprocess writes them. Caller is responsible for .close().
export function openLive(sessionID: string, fromID = 0): EventSource {
  return new EventSource(`/api/sessions/${encodeURIComponent(sessionID)}/live?from=${fromID}`)
}
