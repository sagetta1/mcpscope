import { useState } from 'react'

// Recursive JSON viewer with collapsible objects/arrays. Hand-rolled to
// avoid pulling in 30-100 KB of react-json-view; we only need the
// DevTools-style indented tree with a toggle per branch.
//
// Color palette: VSCode dark+ tokens.

type Props = {
  value: unknown
  initiallyExpandedDepth?: number
}

export function JsonViewer({ value, initiallyExpandedDepth = 2 }: Props) {
  return (
    <pre className="overflow-x-auto font-mono text-xs leading-relaxed">
      <Node value={value} depth={0} maxOpenDepth={initiallyExpandedDepth} />
    </pre>
  )
}

function Node({
  value,
  depth,
  maxOpenDepth,
  keyName,
  isLast,
}: {
  value: unknown
  depth: number
  maxOpenDepth: number
  keyName?: string
  isLast?: boolean
}) {
  const [open, setOpen] = useState(depth < maxOpenDepth)

  const renderKey = keyName !== undefined && (
    <span className="text-purple-300">"{keyName}"</span>
  )

  const t = typeOf(value)

  if (t === 'null') return <Line indent={depth} keyEl={renderKey} valueEl={<span className="text-gray-500">null</span>} isLast={isLast} />
  if (t === 'boolean') return <Line indent={depth} keyEl={renderKey} valueEl={<span className="text-amber-300">{String(value)}</span>} isLast={isLast} />
  if (t === 'number') return <Line indent={depth} keyEl={renderKey} valueEl={<span className="text-cyan-300">{String(value)}</span>} isLast={isLast} />
  if (t === 'string') {
    return (
      <Line
        indent={depth}
        keyEl={renderKey}
        valueEl={<span className="break-all text-emerald-300">"{escapeStr(value as string)}"</span>}
        isLast={isLast}
      />
    )
  }

  // object or array
  const isArr = Array.isArray(value)
  const entries = isArr
    ? (value as unknown[]).map((v, i) => [String(i), v] as [string, unknown])
    : Object.entries(value as Record<string, unknown>)
  const open_b = isArr ? '[' : '{'
  const close_b = isArr ? ']' : '}'

  if (entries.length === 0) {
    return (
      <Line
        indent={depth}
        keyEl={renderKey}
        valueEl={<span className="text-gray-500">{open_b}{close_b}</span>}
        isLast={isLast}
      />
    )
  }

  return (
    <>
      <div style={{ paddingLeft: depth * 12 }} className="select-text">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="mr-1 inline-block w-3 cursor-pointer text-gray-500 hover:text-gray-300"
          aria-label={open ? 'collapse' : 'expand'}
        >
          {open ? '▾' : '▸'}
        </button>
        {renderKey}
        {renderKey && <span className="text-gray-500">: </span>}
        <span className="text-gray-500">{open_b}</span>
        {!open && (
          <span className="text-gray-600">
            {' '}{entries.length} {isArr ? 'items' : 'keys'} {close_b}
          </span>
        )}
      </div>
      {open && (
        <>
          {entries.map(([k, v], i) => (
            <Node
              key={k}
              keyName={isArr ? undefined : k}
              value={v}
              depth={depth + 1}
              maxOpenDepth={maxOpenDepth}
              isLast={i === entries.length - 1}
            />
          ))}
          <div style={{ paddingLeft: depth * 12 }} className="text-gray-500">
            {close_b}
            {!isLast && <span>,</span>}
          </div>
        </>
      )}
    </>
  )
}

function Line({
  indent,
  keyEl,
  valueEl,
  isLast,
}: {
  indent: number
  keyEl: React.ReactNode
  valueEl: React.ReactNode
  isLast?: boolean
}) {
  return (
    <div style={{ paddingLeft: indent * 12 + 16 }} className="select-text">
      {keyEl}
      {keyEl && <span className="text-gray-500">: </span>}
      {valueEl}
      {!isLast && <span className="text-gray-500">,</span>}
    </div>
  )
}

function typeOf(v: unknown): string {
  if (v === null) return 'null'
  if (Array.isArray(v)) return 'array'
  return typeof v
}

function escapeStr(s: string): string {
  // Show newlines and tabs visibly in the JSON viewer.
  return s.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n').replace(/\t/g, '\\t')
}
