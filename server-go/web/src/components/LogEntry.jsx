import './LogEntry.css'

// Color mappings for log levels
const LEVEL_COLORS = {
  DEBUG: 'log-level-debug',
  INFO: 'log-level-info',
  WARN: 'log-level-warn',
  ERROR: 'log-level-error',
}

// Color mappings for components
const COMPONENT_COLORS = {
  Engine: 'log-component-engine',
  HTTP: 'log-component-http',
  WebSocket: 'log-component-websocket',
  TCP: 'log-component-tcp',
  UDP: 'log-component-udp',
  App: 'log-component-app',
}

function formatTimestamp(timestamp) {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('fr-FR', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  })
}

function highlightSearch(text, searchTerm) {
  if (!searchTerm || searchTerm.length < 2) return text

  const regex = new RegExp(`(${searchTerm.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi')
  const parts = text.split(regex)

  return parts.map((part, i) =>
    regex.test(part) ? (
      <mark key={i} className="log-highlight">{part}</mark>
    ) : (
      part
    )
  )
}

/**
 * LogEntry component displays a single log entry
 * @param {Object} props
 * @param {Object} props.entry - Log entry object
 * @param {number} props.entry.timestamp - Unix timestamp in milliseconds
 * @param {string} props.entry.level - Log level (DEBUG, INFO, WARN, ERROR)
 * @param {string} props.entry.component - Component name
 * @param {string} props.entry.message - Log message
 * @param {string} [props.searchTerm=''] - Search term to highlight
 */
export default function LogEntry({ entry, searchTerm = '' }) {
  const { timestamp, level, component, message } = entry

  const levelClass = LEVEL_COLORS[level] || 'log-level-info'
  const componentClass = COMPONENT_COLORS[component] || 'log-component-app'

  return (
    <div className={`log-entry ${levelClass}`}>
      <span className="log-timestamp">{formatTimestamp(timestamp)}</span>
      <span className={`log-level ${levelClass}`}>{level}</span>
      <span className={`log-component ${componentClass}`}>{component}</span>
      <span className="log-message">{highlightSearch(message, searchTerm)}</span>
    </div>
  )
}
