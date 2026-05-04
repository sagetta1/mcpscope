import { useEffect, useState } from 'react'
import { SessionsList } from './screens/SessionsList'
import { Timeline } from './screens/Timeline'
import { Diff } from './screens/Diff'
import { Live } from './screens/Live'

// Hash-based router: avoids react-router-dom (-50 KB) and works as a static
// asset embedded in the Go binary. Routes:
//   #/                              → SessionsList
//   #/session/<id>                  → Timeline + MessageDetail split-pane
//   #/session/<id>/diff[?a=&b=]     → Diff selection / view
//   #/session/<id>/live             → Live SSE feed
function parseHash(hash: string): { route: string; sessionID?: string; sub?: string } {
  // strip leading "#" and any "?query"
  const path = hash.replace(/^#/, '').replace(/^\//, '').split('?')[0]
  const parts = path.split('/').filter(Boolean)
  if (parts.length === 0) return { route: 'list' }
  if (parts[0] === 'session' && parts[1]) {
    return { route: 'session', sessionID: parts[1], sub: parts[2] }
  }
  return { route: 'list' }
}

function App() {
  const [hash, setHash] = useState(window.location.hash)
  useEffect(() => {
    const onChange = () => setHash(window.location.hash)
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])

  const route = parseHash(hash)
  const sub = route.sub ?? 'timeline'

  return (
    <div className="min-h-screen p-4 font-sans">
      <header className="mb-4 flex items-baseline gap-3">
        <a href="#/" className="text-xl font-semibold text-white hover:text-blue-300">
          mcpscope
        </a>
        <span className="text-xs text-gray-500">DevTools for the Model Context Protocol</span>
        {route.route === 'session' && route.sessionID && (
          <>
            <nav className="ml-6 flex gap-3 text-xs">
              <SubLink sessionID={route.sessionID} sub={undefined} active={sub === 'timeline'} label="Timeline" />
              <SubLink sessionID={route.sessionID} sub="diff" active={sub === 'diff'} label="Diff" />
              <SubLink sessionID={route.sessionID} sub="live" active={sub === 'live'} label="Live" />
            </nav>
            <span className="ml-auto font-mono text-xs text-gray-400">{route.sessionID}</span>
          </>
        )}
      </header>

      {route.route === 'list' && <SessionsList />}
      {route.route === 'session' && route.sessionID && (
        <>
          {sub === 'timeline' && <Timeline sessionID={route.sessionID} />}
          {sub === 'diff' && <Diff sessionID={route.sessionID} />}
          {sub === 'live' && <Live sessionID={route.sessionID} />}
        </>
      )}
    </div>
  )
}

function SubLink({
  sessionID,
  sub,
  active,
  label,
}: {
  sessionID: string
  sub?: string
  active: boolean
  label: string
}) {
  const href = sub ? `#/session/${sessionID}/${sub}` : `#/session/${sessionID}`
  return (
    <a
      href={href}
      className={
        'rounded px-2 py-0.5 ' +
        (active ? 'bg-blue-900/50 text-blue-200' : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200')
      }
    >
      {label}
    </a>
  )
}

export default App
