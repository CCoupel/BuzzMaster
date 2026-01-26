import { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import LogEntry from '../components/LogEntry'
import './LogsPage.css'

// Log levels for filtering
const LOG_LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR']

// Components for filtering
const LOG_COMPONENTS = ['App', 'Engine', 'HTTP', 'WebSocket', 'TCP', 'UDP']

export default function LogsPage() {
  const [logs, setLogs] = useState([])
  const [filterLevels, setFilterLevels] = useState(new Set(['INFO', 'WARN', 'ERROR']))
  const [filterComponents, setFilterComponents] = useState(new Set(LOG_COMPONENTS))
  const [searchTerm, setSearchTerm] = useState('')
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState('')
  const [autoScroll, setAutoScroll] = useState(true)
  const [connected, setConnected] = useState(false)

  const logsContainerRef = useRef(null)
  const isUserScrolling = useRef(false)
  const wsRef = useRef(null)

  // Connect to dedicated logs WebSocket
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/logs`

    console.log('[LogsPage] Connecting to', wsUrl)
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[LogsPage] Connected to logs WebSocket')
      setConnected(true)
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)

        if (message.ACTION === 'LOG_HISTORY') {
          // Initial history received
          console.log('[LogsPage] Received LOG_HISTORY with', message.MSG.entries?.length || 0, 'entries')
          setLogs(message.MSG.entries || [])
        } else if (message.ACTION === 'LOG_ENTRY') {
          // New log entry
          const entry = message.MSG.entry
          setLogs(prev => [...prev, entry])
        }
      } catch (err) {
        console.error('[LogsPage] Failed to parse message:', err)
      }
    }

    ws.onerror = (error) => {
      console.error('[LogsPage] WebSocket error:', error)
      setConnected(false)
    }

    ws.onclose = () => {
      console.log('[LogsPage] Disconnected from logs WebSocket')
      setConnected(false)
    }

    // Cleanup on unmount
    return () => {
      console.log('[LogsPage] Closing logs WebSocket')
      if (ws.readyState === WebSocket.OPEN) {
        ws.close()
      }
    }
  }, [])

  // Debounce search term
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchTerm(searchTerm)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchTerm])

  // Filter logs
  const filteredLogs = useMemo(() => {
    return logs.filter(log => {
      // Level filter
      if (!filterLevels.has(log.level)) return false

      // Component filter
      if (!filterComponents.has(log.component)) return false

      // Search filter
      if (debouncedSearchTerm && debouncedSearchTerm.length >= 2) {
        const searchLower = debouncedSearchTerm.toLowerCase()
        if (!log.message.toLowerCase().includes(searchLower)) {
          return false
        }
      }

      return true
    })
  }, [logs, filterLevels, filterComponents, debouncedSearchTerm])

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (autoScroll && logsContainerRef.current && !isUserScrolling.current) {
      logsContainerRef.current.scrollTop = logsContainerRef.current.scrollHeight
    }
  }, [filteredLogs, autoScroll])

  // Detect user scroll to pause auto-scroll
  const handleScroll = useCallback(() => {
    const container = logsContainerRef.current
    if (!container) return

    const isAtBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 50
    isUserScrolling.current = !isAtBottom

    if (isAtBottom) {
      setAutoScroll(true)
    }
  }, [])

  // Toggle level filter
  const toggleLevel = (level) => {
    setFilterLevels(prev => {
      const newSet = new Set(prev)
      if (newSet.has(level)) {
        newSet.delete(level)
      } else {
        newSet.add(level)
      }
      return newSet
    })
  }

  // Toggle component filter
  const toggleComponent = (component) => {
    setFilterComponents(prev => {
      const newSet = new Set(prev)
      if (newSet.has(component)) {
        newSet.delete(component)
      } else {
        newSet.add(component)
      }
      return newSet
    })
  }

  // Select all/none levels
  const selectAllLevels = () => setFilterLevels(new Set(LOG_LEVELS))
  const selectNoLevels = () => setFilterLevels(new Set())

  // Select all/none components
  const selectAllComponents = () => setFilterComponents(new Set(LOG_COMPONENTS))
  const selectNoComponents = () => setFilterComponents(new Set())

  // Export logs to file
  const exportLogs = () => {
    const content = filteredLogs.map(log => {
      const date = new Date(log.timestamp)
      const timestamp = date.toISOString()
      return `${timestamp} [${log.level}][${log.component}] ${log.message}`
    }).join('\n')

    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `buzzcontrol-logs-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.log`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  // Scroll to bottom manually
  const scrollToBottom = () => {
    if (logsContainerRef.current) {
      logsContainerRef.current.scrollTop = logsContainerRef.current.scrollHeight
      setAutoScroll(true)
      isUserScrolling.current = false
    }
  }

  return (
    <div className="logs-page">
      {/* Connection status */}
      {!connected && (
        <div className="logs-connection-status">
          Connexion au serveur de logs...
        </div>
      )}

      {/* Toolbar */}
      <div className="logs-toolbar">
        {/* Search */}
        <div className="logs-search">
          <input
            type="text"
            placeholder="Rechercher..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="logs-search-input"
          />
          {searchTerm && (
            <button
              className="logs-search-clear"
              onClick={() => setSearchTerm('')}
              title="Effacer la recherche"
            >
              x
            </button>
          )}
        </div>

        {/* Level filters */}
        <div className="logs-filter-group">
          <span className="logs-filter-label">Niveau:</span>
          <div className="logs-filter-buttons">
            {LOG_LEVELS.map(level => (
              <button
                key={level}
                className={`logs-filter-btn logs-filter-level-${level.toLowerCase()} ${filterLevels.has(level) ? 'active' : ''}`}
                onClick={() => toggleLevel(level)}
              >
                {level}
              </button>
            ))}
          </div>
          <button className="logs-filter-all-btn" onClick={selectAllLevels} title="Tout">+</button>
          <button className="logs-filter-all-btn" onClick={selectNoLevels} title="Aucun">-</button>
        </div>

        {/* Component filters */}
        <div className="logs-filter-group">
          <span className="logs-filter-label">Composant:</span>
          <div className="logs-filter-buttons">
            {LOG_COMPONENTS.map(component => (
              <button
                key={component}
                className={`logs-filter-btn logs-filter-component-${component.toLowerCase()} ${filterComponents.has(component) ? 'active' : ''}`}
                onClick={() => toggleComponent(component)}
              >
                {component}
              </button>
            ))}
          </div>
          <button className="logs-filter-all-btn" onClick={selectAllComponents} title="Tout">+</button>
          <button className="logs-filter-all-btn" onClick={selectNoComponents} title="Aucun">-</button>
        </div>

        {/* Auto-scroll toggle */}
        <div className="logs-actions">
          <button
            className={`logs-action-btn ${autoScroll ? 'active' : ''}`}
            onClick={() => setAutoScroll(!autoScroll)}
            title="Auto-scroll"
          >
            Auto
          </button>
          <button
            className="logs-action-btn"
            onClick={scrollToBottom}
            title="Aller en bas"
          >
            Bas
          </button>
          <button
            className="logs-action-btn"
            onClick={exportLogs}
            title="Exporter les logs"
          >
            Export
          </button>
        </div>

        {/* Stats */}
        <div className="logs-stats">
          {filteredLogs.length} / {logs.length}
        </div>
      </div>

      {/* Logs list */}
      <div
        className="logs-container"
        ref={logsContainerRef}
        onScroll={handleScroll}
      >
        {filteredLogs.length === 0 ? (
          <div className="logs-empty">
            {logs.length === 0 ? (
              <p>En attente de logs...</p>
            ) : (
              <p>Aucun log ne correspond aux filtres</p>
            )}
          </div>
        ) : (
          filteredLogs.map((log, index) => (
            <LogEntry
              key={`${log.timestamp}-${index}`}
              entry={log}
              searchTerm={debouncedSearchTerm}
            />
          ))
        )}
      </div>

      {/* Scroll indicator */}
      {!autoScroll && (
        <div className="logs-scroll-indicator" onClick={scrollToBottom}>
          Nouveaux logs - Cliquez pour descendre
        </div>
      )}
    </div>
  )
}
