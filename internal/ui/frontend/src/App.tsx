import { useEffect, useState } from 'react'
import { SessionsList } from './screens/SessionsList'
import { Timeline } from './screens/Timeline'

// Hash-based router: avoids react-router-dom (-50 KB) and works as a static
// asset embedded in the Go binary. Routes:
//   #/                       → SessionsList
//   #/session/<id>           → Timeline + MessageDetail split-pane
//   #/session/<id>/diff      → (Day 9) Diff selection
//   #/session/<id>/live      → (Day 10) Live SSE mode
function parseHash(hash: string): { route: string; sessionID?: string; sub?: string } {
  // strip leading "#"
  const path = hash.replace(/^#/, '').replace(/^\//, '')
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

  return (
    <div className="min-h-screen p-4 font-sans">
      <header className="mb-4 flex items-baseline gap-3">
        <a href="#/" className="text-xl font-semibold text-white hover:text-blue-300">
          mcpscope
        </a>
        <span className="text-xs text-gray-500">DevTools for the Model Context Protocol</span>
        {route.route === 'session' && (
          <span className="ml-auto font-mono text-xs text-gray-400">{route.sessionID}</span>
        )}
      </header>

      {route.route === 'list' && <SessionsList />}
      {route.route === 'session' && route.sessionID && <Timeline sessionID={route.sessionID} />}
    </div>
  )
}

export default App
